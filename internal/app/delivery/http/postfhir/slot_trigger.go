package postfhir

import (
	"context"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/delivery/http/middlewares"
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
	} `json:"request,omitempty"`
	Resource json.RawMessage `json:"resource,omitempty"`
}

// transactionRequestBundle is the minimal shape of a FHIR transaction request.
type transactionRequestBundle struct {
	ResourceType string                    `json:"resourceType"`
	Type         string                    `json:"type"`
	Entry        []transactionRequestEntry `json:"entry"`
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
		ids := collectPractitionerRoleIDsFromMutation(req)
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
func collectPractitionerRoleIDsFromMutation(req middlewares.PostFHIRProxyUserRequestDetail) []string {
	if ids := collectPractitionerRoleIDsFromTransactionBundle(req.Body); ids != nil {
		return ids
	}
	return collectPractitionerRoleIDsFromSingleResource(req)
}

// collectPractitionerRoleIDsFromTransactionBundle extracts PractitionerRole IDs from a FHIR transaction
// bundle body. Returns nil if body is empty or not a valid transaction bundle (caller should try single-resource path).
// Returns a non-nil slice (possibly empty) when the body is a valid transaction bundle, so the caller does not fall through to single-resource parsing.
func collectPractitionerRoleIDsFromTransactionBundle(body []byte) []string {
	bundle, ok := parseTransactionBundle(body)
	if !ok {
		return nil
	}
	seen := make(map[string]struct{})
	add := func(ids []string) {
		for _, id := range ids {
			if id != "" {
				seen[id] = struct{}{}
			}
		}
	}
	for i := range bundle.Entry {
		collectPractitionerRoleIDsFromBundleEntry(&bundle.Entry[i], add)
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

// collectPractitionerRoleIDsFromBundleEntry extracts PractitionerRole IDs from one transaction entry and adds them via add.
// Skips DELETE entries and non-PUT/PATCH; skips entries that fail to unmarshal or are not PractitionerRole/Schedule.
func collectPractitionerRoleIDsFromBundleEntry(e *transactionRequestEntry, add func(ids []string)) {
	method := entryMethod(e)
	if method == "DELETE" || (method != "PUT" && method != "PATCH") {
		return
	}
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
func collectPractitionerRoleIDsFromSingleResource(req middlewares.PostFHIRProxyUserRequestDetail) []string {
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
		if len(req.Body) > 0 {
			var env resourceEnvelope
			if err := json.Unmarshal(req.Body, &env); err == nil && strings.EqualFold(env.ResourceType, resourceTypeSchedule) {
				for _, a := range env.Actor {
					if id := practitionerRoleIDFromReference(a.Reference); id != "" {
						add([]string{id})
					}
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
