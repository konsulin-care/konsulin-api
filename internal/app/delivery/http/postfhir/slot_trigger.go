package postfhir

import (
	"context"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"net/url"
	"strings"

	"slices"

	"go.uber.org/zap"
)

const (
	resourceTypePractitionerRole = "PractitionerRole"
	resourceTypeSchedule         = "Schedule"
	fhirPathPrefix               = "/fhir/"
)

// transactionRequestEntry represents one entry in a FHIR transaction request bundle.
type transactionRequestEntry struct {
	Request *struct {
		Method string `json:"method"`
		URL    string `json:"url"`
	} `json:"request,omitempty"`
	Resource json.RawMessage `json:"resource,omitempty"`
}

// transactionRequestBundle is the minimal shape of a FHIR transaction request.
type transactionRequestBundle struct {
	ResourceType string                    `json:"resourceType"`
	Type         string                    `json:"type"`
	Entry        []transactionRequestEntry `json:"entry"`
}

// transactionResponseBundle is the minimal shape of a FHIR transaction-response bundle.
// Some FHIR servers include entry.resource depending on Prefer/return settings; we use it
// as a best-effort source of the updated resource (especially for PATCH).
type transactionResponseBundle struct {
	ResourceType string `json:"resourceType"`
	Type         string `json:"type"`
	Entry        []struct {
		Resource json.RawMessage `json:"resource,omitempty"`
	} `json:"entry"`
}

// resourceEnvelope is used to detect resourceType and id/actor from a raw resource.
type resourceEnvelope struct {
	ResourceType string     `json:"resourceType"`
	ID           string     `json:"id,omitempty"`
	Actor        []actorRef `json:"actor,omitempty"`
}

type actorRef struct {
	Reference string `json:"reference,omitempty"`
}

// NewSlotRegenerationHook returns a PostFHIRProxyHook that detects PractitionerRole/Schedule
// mutations and calls HandleOnDemandSlotRegeneration for each affected practitioner role.
// DELETE operations trigger no-op for now.
func NewSlotRegenerationHook(log *zap.Logger, slotUsecase contracts.SlotUsecaseIface) middlewares.PostFHIRProxyHook {
	return func(req middlewares.PostFHIRProxyUserRequestDetail, resp middlewares.PostFHIRProxyFHIRServerResponse) error {
		if resp.StatusCode >= 400 {
			return nil
		}
		ids := collectPractitionerRoleIDsFromMutation(req, resp)
		if len(ids) == 0 {
			return nil
		}
		ctx := req.Context
		if ctx == nil {
			ctx = context.Background()
		}
		var errMsgs []string
		for _, id := range ids {
			if err := slotUsecase.HandleOnDemandSlotRegeneration(ctx, id); err != nil {
				log.Warn("HandleOnDemandSlotRegeneration failed", zap.String("practitioner_role_id", id), zap.Error(err))
				errMsgs = append(errMsgs, err.Error())
			}
		}
		if len(errMsgs) > 0 {
			return fmt.Errorf("%s", strings.Join(errMsgs, "; "))
		}
		return nil
	}
}

// collectPractitionerRoleIDsFromMutation returns deduplicated PractitionerRole IDs affected by the request.
func collectPractitionerRoleIDsFromMutation(req middlewares.PostFHIRProxyUserRequestDetail, resp middlewares.PostFHIRProxyFHIRServerResponse) []string {
	if ids := collectPractitionerRoleIDsFromTransactionBundle(req.Body, resp.Body); ids != nil {
		return ids
	}
	return collectPractitionerRoleIDsFromSingleResource(req, resp)
}

// collectPractitionerRoleIDsFromTransactionBundle extracts PractitionerRole IDs from a FHIR transaction
// bundle body. Returns nil if body is empty or not a valid transaction bundle (caller should try single-resource path).
// Returns a non-nil slice (possibly empty) when the body is a valid transaction bundle, so the caller does not fall through to single-resource parsing.
func collectPractitionerRoleIDsFromTransactionBundle(reqBody []byte, respBody []byte) []string {
	bundle, ok := parseTransactionBundle(reqBody)
	if !ok {
		return nil
	}
	respBundle, _ := parseTransactionResponseBundle(respBody)
	seen := make(map[string]struct{})
	add := func(ids []string) {
		for _, id := range ids {
			if id != "" {
				seen[id] = struct{}{}
			}
		}
	}
	for i := range bundle.Entry {
		var respResource json.RawMessage
		if respBundle != nil && i < len(respBundle.Entry) {
			respResource = respBundle.Entry[i].Resource
		}
		collectPractitionerRoleIDsFromBundleEntry(&bundle.Entry[i], respResource, add)
	}
	if len(seen) == 0 {
		return []string{}
	}
	return mapKeysToSlice(seen)
}

// parseTransactionBundle unmarshals body and returns the bundle and true only if it is a valid FHIR transaction bundle.
func parseTransactionBundle(body []byte) (*transactionRequestBundle, bool) {
	if len(body) == 0 {
		return nil, false
	}
	var bundle transactionRequestBundle
	if err := json.Unmarshal(body, &bundle); err != nil {
		return nil, false
	}
	if !strings.EqualFold(bundle.ResourceType, "Bundle") || !strings.EqualFold(bundle.Type, "transaction") {
		return nil, false
	}
	return &bundle, true
}

func parseTransactionResponseBundle(body []byte) (*transactionResponseBundle, bool) {
	if len(body) == 0 {
		return nil, false
	}
	var bundle transactionResponseBundle
	if err := json.Unmarshal(body, &bundle); err != nil {
		return nil, false
	}
	if !strings.EqualFold(bundle.ResourceType, "Bundle") || !strings.EqualFold(bundle.Type, "transaction-response") {
		return nil, false
	}
	return &bundle, true
}

// collectPractitionerRoleIDsFromBundleEntry extracts PractitionerRole IDs from one transaction entry and adds them via add.
// Skips DELETE entries and non-PUT/PATCH; skips entries that fail to parse or are not PractitionerRole/Schedule.
//
// For PATCH entries, the patch document usually does not include resourceType/id/actor, so we parse entry.request.url
// (e.g. "PractitionerRole/123" or "Schedule/456"). For Schedule PATCH, we best-effort parse respResource as a full
// Schedule to get actor references if the server returned it.
func collectPractitionerRoleIDsFromBundleEntry(e *transactionRequestEntry, respResource json.RawMessage, add func(ids []string)) {
	method := entryMethod(e)
	if method == "DELETE" || (method != "PUT" && method != "PATCH") {
		return
	}
	if method == "PATCH" {
		resourceType, id := entryResourceTypeAndIDFromURL(e)
		switch resourceType {
		case resourceTypePractitionerRole:
			if id != "" {
				add([]string{id})
			}
		case resourceTypeSchedule:
			// PATCH payload won't have actors; best-effort: parse updated Schedule from response entry.resource
			if len(respResource) == 0 {
				return
			}
			var env resourceEnvelope
			if err := json.Unmarshal(respResource, &env); err != nil {
				return
			}
			if strings.EqualFold(env.ResourceType, resourceTypeSchedule) {
				add(practitionerRoleIDsFromEnvelope(env))
			}
		}
		return
	}

	// PUT: entry.resource should be a full resource representation
	if len(e.Resource) == 0 {
		return
	}
	var env resourceEnvelope
	if err := json.Unmarshal(e.Resource, &env); err != nil {
		return
	}
	add(practitionerRoleIDsFromEnvelope(env))
}

// entryMethod returns the uppercased request method for the entry, or "" if missing.
func entryMethod(e *transactionRequestEntry) string {
	if e == nil || e.Request == nil {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(e.Request.Method))
}

func entryResourceTypeAndIDFromURL(e *transactionRequestEntry) (resourceType string, id string) {
	if e == nil || e.Request == nil {
		return "", ""
	}
	return parseResourceTypeAndIDFromRequestURL(e.Request.URL)
}

func parseResourceTypeAndIDFromRequestURL(u string) (resourceType string, id string) {
	u = strings.TrimSpace(u)
	if u == "" {
		return "", ""
	}
	parsed, err := url.Parse(u)
	if err == nil && parsed.Path != "" {
		u = parsed.Path
	}
	u = strings.Trim(u, "/")
	// Some systems may include a base proxy path in request.url (e.g. "/fhir/PractitionerRole/123").
	// Strip it so we can consistently read "ResourceType/id".
	if strings.HasPrefix(strings.ToLower(u), "fhir/") {
		u = strings.TrimPrefix(u, "fhir/")
	}
	parts := strings.Split(u, "/")
	if len(parts) < 2 {
		return "", ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

// practitionerRoleIDsFromEnvelope returns PractitionerRole IDs from a resource envelope (PractitionerRole ID or Schedule actors).
func practitionerRoleIDsFromEnvelope(env resourceEnvelope) []string {
	resourceType := strings.TrimSpace(env.ResourceType)
	switch resourceType {
	case resourceTypePractitionerRole:
		if env.ID != "" {
			return []string{env.ID}
		}
		return nil
	case resourceTypeSchedule:
		var ids []string
		for _, a := range env.Actor {
			if id := practitionerRoleIDFromReference(a.Reference); id != "" {
				ids = append(ids, id)
			}
		}
		return ids
	default:
		return nil
	}
}

// collectPractitionerRoleIDsFromSingleResource extracts PractitionerRole IDs from a single-resource
// request (path and optional body). Assumes request is not a transaction bundle.
func collectPractitionerRoleIDsFromSingleResource(req middlewares.PostFHIRProxyUserRequestDetail, resp middlewares.PostFHIRProxyFHIRServerResponse) []string {
	path := strings.TrimPrefix(req.Path, fhirPathPrefix)
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return nil
	}
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "DELETE" || (method != "PUT" && method != "PATCH") {
		return nil
	}
	resourceType := strings.TrimSpace(parts[0])
	resID := strings.TrimSpace(parts[1])
	seen := make(map[string]struct{})
	add := func(ids []string) {
		for _, id := range ids {
			if id != "" {
				seen[id] = struct{}{}
			}
		}
	}
	switch resourceType {
	case resourceTypePractitionerRole:
		if resID != "" {
			add([]string{resID})
		}
	case resourceTypeSchedule:
		// PUT typically sends a full Schedule in the request body, but PATCH usually doesn't.
		// For PATCH, best-effort parse the server response as a full Schedule to get actor refs.
		var candidate []byte
		if method == "PATCH" {
			candidate = resp.Body
		} else {
			candidate = req.Body
		}
		if len(candidate) == 0 {
			break
		}
		var env resourceEnvelope
		if err := json.Unmarshal(candidate, &env); err == nil && strings.EqualFold(env.ResourceType, resourceTypeSchedule) {
			for _, a := range env.Actor {
				if id := practitionerRoleIDFromReference(a.Reference); id != "" {
					add([]string{id})
				}
			}
		}
	}
	return mapKeysToSlice(seen)
}

func practitionerRoleIDFromReference(ref string) string {
	ref = strings.TrimSpace(ref)
	const prefix = "PractitionerRole/"
	if strings.HasPrefix(ref, prefix) {
		return strings.TrimSpace(ref[len(prefix):])
	}
	return ""
}

func mapKeysToSlice(m map[string]struct{}) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
