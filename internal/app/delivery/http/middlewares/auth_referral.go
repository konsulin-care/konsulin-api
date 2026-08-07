package middlewares

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"konsulin-service/internal/pkg/constvars"

	"github.com/tidwall/gjson"
)

// Referral Communication constants. These mirror the frontend share/referral
// write (src/utils/referral-communication.ts) so the backend can recompute the
// deterministic id and validate the schema before any Communication write is
// proxied to the FHIR server.
const (
	ReferralIDPrefix          = "referral-"
	ReferralTopicSystem       = "http://konsulin.care/fhir/CodeSystem/research-referral"
	ReferralTopicCode         = "research-referral"
	ReferralBatchExtensionURL = "http://konsulin.care/fhir/StructureDefinition/referralBatch"
)

// referralIDFor computes the deterministic Communication id for a referral:
// `referral-` + sha256 hex of the pipe-joined recipient|sender|batch.
func referralIDFor(recipient, sender, batch string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s", recipient, sender, batch)))
	return ReferralIDPrefix + hex.EncodeToString(sum[:])
}

// isReferralID reports whether an id carries the reserved referral- prefix.
func isReferralID(id string) bool {
	return strings.HasPrefix(id, ReferralIDPrefix)
}

// referralForbiddenError marks a referral validation rejection so the Auth
// middleware can map it to 403 Forbidden instead of the default 401.
type referralForbiddenError struct {
	msg string
}

func (e *referralForbiddenError) Error() string { return e.msg }

// referralForbidden builds a *referralForbiddenError with a formatted message.
func referralForbidden(format string, args ...any) error {
	return &referralForbiddenError{msg: fmt.Sprintf(format, args...)}
}

// rejectReferralPOST rejects POST creates that carry a referral- id. Referral
// Communications are PUT-only, deterministic resources; a POST create with a
// referral- id would let a caller forge an edge id (the hash would be ignored
// by Blaze and the id would pass through unvalidated).
func rejectReferralPOST(method string, body []byte, bodyID string) error {
	if method == constvars.MethodPost && isReferralID(bodyID) {
		return referralForbidden("forbidden: referral Communications cannot be created via POST")
	}
	return nil
}

// validateReferralCommunication enforces the B3 referral Communication write
// rules. It is called from the PUT chokepoint for Communication resources with
// a referral- id. Only Patient sessions may write referral Communications
// (guest and practitioner referral writes are rejected); the body must be a
// deterministic referral Communication whose recipient equals the session
// patient id; the sender must resolve to a registered Patient; and the batch
// extension must reference an existing PlanDefinition. Any failure returns an
// error, which the Auth middleware turns into a 403.
func (m *Middlewares) validateReferralCommunication(ctx context.Context, r *http.Request, roles []string, fhirID, urlID string, body []byte) error {
	if !isReferralID(urlID) {
		return referralForbidden("forbidden: non-referral- ids are not validated as referral Communications")
	}

	// Patient session only: guests (ephemeral, untracked) and practitioners
	// must not create referral edges in the research network graph.
	if !hasRole(roles, constvars.KonsulinRolePatient) || hasRole(roles, constvars.KonsulinRolePractitioner) {
		return referralForbidden("forbidden: referral Communication writes are patient-only")
	}
	if fhirID == "" {
		return referralForbidden("forbidden: referral Communication writes require a patient identity")
	}

	senderID, _, batchID, ok := validateReferralCommunicationBody(body, urlID, fhirID)
	if !ok {
		return referralForbidden("forbidden: invalid referral Communication")
	}

	// Live checks: the sender must be a registered Patient, the batch must exist.
	if _, err := m.PatientFhirClient.FindPatientByID(ctx, senderID); err != nil {
		return referralForbidden("forbidden: referral sender is not a registered Patient")
	}
	if _, err := m.PlanDefinitionFinder.FindPlanDefinitionByID(ctx, batchID); err != nil {
		return referralForbidden("forbidden: referral batch PlanDefinition not found")
	}

	return nil
}

// validateReferralCommunicationBody validates a referral Communication PUT.
//
// It enforces the deterministic, content-derived rules that do not require
// server-side lookups: the id must be a referral- id equal to
// sha256(recipient|sender|batch), the topic must be research-referral, the
// batch extension must be a well-formed PlanDefinition reference, and the
// recipient must equal the session identity. On success it returns the sender
// patient id, recipient id, and batch id for any further (live) checks.
func validateReferralCommunicationBody(body []byte, urlID, sessionIdentity string) (senderID, recipientID, batchID string, ok bool) {
	if !strings.HasPrefix(urlID, ReferralIDPrefix) {
		return "", "", "", false
	}

	// The body id must equal the url id; the frontend PUTs both as the
	// deterministic referral id.
	if gjson.GetBytes(body, "id").String() != urlID {
		return "", "", "", false
	}

	topicCode := gjson.GetBytes(body, "topic.coding.0.code").String()
	if topicCode != ReferralTopicCode {
		return "", "", "", false
	}

	sender := gjson.GetBytes(body, "sender.reference").String()
	if !strings.HasPrefix(sender, constvars.FHIRRefPrefixPatient) {
		return "", "", "", false
	}
	senderID = strings.TrimPrefix(sender, constvars.FHIRRefPrefixPatient)
	if senderID == "" {
		return "", "", "", false
	}

	recipient := gjson.GetBytes(body, "recipient.0.reference").String()
	recipientID = strings.TrimPrefix(recipient, constvars.FHIRRefPrefixPatient)
	if recipientID == "" || recipientID != sessionIdentity {
		return "", "", "", false
	}

	batchRef := gjson.GetBytes(
		body,
		fmt.Sprintf("extension.#(url=%q).valueReference.reference", ReferralBatchExtensionURL),
	).String()
	if !strings.HasPrefix(batchRef, "PlanDefinition/") {
		return "", "", "", false
	}
	batchID = strings.TrimPrefix(batchRef, "PlanDefinition/")
	if batchID == "" {
		return "", "", "", false
	}

	expectedID := referralIDFor(recipientID, senderID, batchID)
	if expectedID != urlID {
		return "", "", "", false
	}

	return senderID, recipientID, batchID, true
}
