package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/konsulin-care/gateway-sdk-go/envelope"
	"go.uber.org/zap"
)

// requestStatusCompleted is the FHIR ServiceRequest status a finished
// invocation receipt is marked with when its result arrives.
const requestStatusCompleted = "completed"

// RequestResult is the owner-visible view of one invocation request: its
// status and the stored result.
type RequestResult struct {
	// Status is the ServiceRequest status ("active" until the result
	// arrives, "completed" afterwards).
	Status string `json:"status"`
	// Result is the opened result stored on the request ("" before the
	// result arrives).
	Result string `json:"result"`
}

// ResultUsecase handles inbound service-result federation events and
// owner-only reads of stored invocation results over the ServiceRequest
// FHIR client. It is immutable after construction and safe for concurrent
// use.
type ResultUsecase struct {
	fhir          contracts.ServiceRequestFhirClient
	partyID       string
	encPrivateKey string
	log           *zap.Logger
}

// NewResultUsecase builds the service-result usecase over the ServiceRequest
// FHIR client.
//
// Parameters:
//   - client is the FHIR ServiceRequest client the flow reads and writes.
//   - gw carries the party configuration: EncPrivateKey (base64 X25519 seed)
//     opens sealed results and PartyID labels logs; nil is tolerated and
//     yields a usecase that fails closed at open time.
//   - logger reports handling; nil is replaced by a no-op logger.
//
// Returns: a usecase ready to bind. Never panics.
func NewResultUsecase(client contracts.ServiceRequestFhirClient, gw *config.AppGateway, logger *zap.Logger) *ResultUsecase {
	if logger == nil {
		logger = zap.NewNop()
	}
	u := &ResultUsecase{fhir: client, log: logger}
	if gw != nil {
		u.partyID = strings.TrimSpace(gw.PartyID)
		u.encPrivateKey = strings.TrimSpace(gw.EncPrivateKey)
	}
	return u
}

// HandleServiceResultEvent processes one verified service-result event: it
// finds the ServiceRequest receipt identified by the event's webhook id,
// opens the sealed result with the party's use:enc key
// (APP_PARTY_ENC_PRIVATE_KEY) and appends the opened bytes as a note,
// marking the request completed while stamping the webhook id identifier
// back onto it. An unknown webhook id is acknowledged with nil and no
// write; a failed open or write returns the error so the federation
// controller answers 500 for a retry.
//
// Parameters:
//   - ctx bounds the FHIR calls.
//   - webhookID is the event's verified webhook_id claim.
//   - sealed is the exact event body: the compact JWE carrying the result.
//
// Returns: nil when the result was stored or the webhook id is unknown, or
// the open/write error. Never panics.
func (u *ResultUsecase) HandleServiceResultEvent(ctx context.Context, webhookID string, sealed []byte) error {
	if u == nil || u.fhir == nil {
		return errors.New("results: no service request client configured")
	}
	found, err := u.findByWebhookID(ctx, webhookID)
	if err != nil {
		return err
	}
	if found == nil {
		// Unknown webhook id: acknowledge without writing, so the gateway
		// does not retry a result this backend never asked for.
		u.log.Info("results: service-result for unknown webhook id acknowledged",
			zap.String("webhook_id", webhookID))
		return nil
	}
	opened, err := u.openResult(sealed)
	if err != nil {
		return err
	}
	notes := append(append([]fhir_dto.Annotation{}, found.Note...), fhir_dto.Annotation{
		Text: string(opened),
		Time: time.Now().UTC().Format(time.RFC3339),
	})
	upd := &fhir_dto.UpdateServiceRequestInput{
		ResourceType:       resourceTypeOr(found.ResourceType),
		ID:                 found.ID,
		Meta:               found.Meta,
		Status:             requestStatusCompleted,
		Intent:             found.Intent,
		Subject:            found.Subject,
		OccurrenceDateTime: found.OccurrenceDateTime,
		AuthoredOn:         found.AuthoredOn,
		InstantiatesUri:    found.InstantiatesUri,
		Identifier: []fhir_dto.Identifier{
			{System: WebhookIdentifierSystem, Value: strings.TrimSpace(webhookID)},
		},
		Note: notes,
	}
	if found.Requester.Reference != "" {
		requester := found.Requester
		upd.Requester = &requester
	}
	if _, err := u.fhir.Update(ctx, found.ID, upd); err != nil {
		return fmt.Errorf("results: store result on %s: %w", found.ID, err)
	}
	u.log.Info("results: service result stored",
		zap.String("webhook_id", webhookID),
		zap.String("service_request_id", found.ID),
		zap.String("party_id", u.partyID))
	return nil
}

// GetRequest returns the owner-visible view of one invocation request: the
// ServiceRequest status and the stored result. The requester recorded on
// the request is the only caller that gets the view; any other caller (and
// an unknown id) gets a 404 not found, leaking nothing about the request's
// existence.
//
// Parameters:
//   - ctx bounds the FHIR read.
//   - caller is the requesting FHIR reference (the session's subject).
//   - id is the ServiceRequest id.
//
// Returns: the view and nil for the owner, or nil and a 404 custom error
// for anyone else; a FHIR read failure passes through raw. Never panics.
func (u *ResultUsecase) GetRequest(ctx context.Context, caller, id string) (*RequestResult, error) {
	if u == nil || u.fhir == nil {
		return nil, errors.New("results: no service request client configured")
	}
	if strings.TrimSpace(id) == "" {
		return nil, requestNotFound()
	}
	found, err := u.fhir.Search(ctx, &fhir_dto.SearchServiceRequestInput{ID: id})
	if err != nil {
		return nil, err
	}
	if len(found) != 1 {
		return nil, requestNotFound()
	}
	sr := found[0]
	requester := strings.TrimSpace(sr.Requester.Reference)
	if requester == "" || requester != strings.TrimSpace(caller) {
		return nil, requestNotFound()
	}
	var notes []string
	for _, note := range sr.Note {
		if text := strings.TrimSpace(note.Text); text != "" {
			notes = append(notes, note.Text)
		}
	}
	return &RequestResult{
		Status: sr.Status,
		Result: strings.Join(notes, " "),
	}, nil
}

// findByWebhookID locates the ServiceRequest receipt for one webhook id.
// The read model (GetServiceRequestOutput) exposes no identifier member and
// the search input filters by _id only, so the webhook id is looked up as
// the receipt's _id — the value the invoke receipt records under
// urn:konsulin:webhook-id is carried back on every write this flow makes.
//
// Parameters:
//   - ctx bounds the FHIR read, webhookID is the event's webhook id.
//
// Returns: the matching resource and nil, or nil and nil when no receipt
// carries the id; a FHIR read failure passes through raw. Never panics.
func (u *ResultUsecase) findByWebhookID(ctx context.Context, webhookID string) (*fhir_dto.GetServiceRequestOutput, error) {
	want := strings.TrimSpace(webhookID)
	if want == "" {
		return nil, nil
	}
	found, err := u.fhir.Search(ctx, &fhir_dto.SearchServiceRequestInput{ID: want})
	if err != nil {
		return nil, err
	}
	switch len(found) {
	case 0:
		return nil, nil
	case 1:
		return &found[0], nil
	default:
		return nil, exceptions.ErrResultFetchedNotUniqueFhirResource(nil, constvars.ResourceServiceRequest)
	}
}

// openResult opens the compact JWE body of one service-result event with
// the party's use:enc private key (APP_PARTY_ENC_PRIVATE_KEY, base64 of the
// 32-byte X25519 seed). A missing or unparseable key fails closed and never
// yields plaintext.
//
// Parameters:
//   - sealed is the exact event body bytes.
//
// Returns: the opened result bytes and nil, or nil and an error naming only
// the missing key or envelope failure. Never panics, never logs key
// material.
func (u *ResultUsecase) openResult(sealed []byte) ([]byte, error) {
	compact := strings.TrimSpace(string(sealed))
	if compact == "" {
		return nil, errors.New("results: sealed result body is empty")
	}
	if u.encPrivateKey == "" {
		return nil, errors.New("results: APP_PARTY_ENC_PRIVATE_KEY is required to open the sealed result")
	}
	key, err := envelope.ParseX25519PrivateKey(u.encPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("results: open sealed result: %w", err)
	}
	opened, err := envelope.Open(compact, key)
	if err != nil {
		return nil, fmt.Errorf("results: open sealed result: %w", err)
	}
	return opened, nil
}

// resourceTypeOr falls back to the FHIR resource name when a search result
// carries none, so the update body always names its resource.
//
// Parameters:
//   - value is the resource type found on the resource.
//
// Returns: the value, or ServiceRequest when blank. Never panics.
func resourceTypeOr(value string) string {
	if strings.TrimSpace(value) == "" {
		return constvars.ResourceServiceRequest
	}
	return value
}

// requestNotFound renders the owner-check refusal: a 404 custom error that
// never distinguishes "no such request" from "not your request".
//
// Returns: the 404 custom error. Never panics.
func requestNotFound() error {
	return exceptions.BuildNewCustomError(nil, constvars.StatusNotFound,
		"service request not found", "SERVICE_REQUEST_NOT_FOUND")
}
