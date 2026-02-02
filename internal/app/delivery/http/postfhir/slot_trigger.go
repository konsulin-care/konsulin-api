package postfhir

import (
	"context"
	"encoding/json"
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
		ids := collectPractitionerRoleIDsFromMutation(req, resp)
		if len(ids) == 0 {
			return nil
		}
		ctx := req.Context
		if ctx == nil {
			ctx = context.Background()
		}
		for _, id := range ids {
			if err := slotUsecase.HandleOnDemandSlotRegeneration(ctx, id); err != nil {
				log.Warn("HandleOnDemandSlotRegeneration failed", zap.String("practitioner_role_id", id), zap.Error(err))
			}
		}
		return nil
	}
}

// collectPractitionerRoleIDsFromMutation returns deduplicated PractitionerRole IDs affected by the request.
func collectPractitionerRoleIDsFromMutation(req middlewares.PostFHIRProxyUserRequestDetail, resp middlewares.PostFHIRProxyFHIRServerResponse) []string {
	seen := make(map[string]struct{})
	var add func(ids []string)
	add = func(ids []string) {
		for _, id := range ids {
			if id == "" {
				continue
			}
			seen[id] = struct{}{}
		}
	}

	// Try request body as transaction bundle first.
	if len(req.Body) > 0 {
		var bundle transactionRequestBundle
		if err := json.Unmarshal(req.Body, &bundle); err == nil &&
			strings.EqualFold(bundle.ResourceType, "Bundle") &&
			strings.EqualFold(bundle.Type, "transaction") {
			for i := range bundle.Entry {
				e := &bundle.Entry[i]
				method := ""
				if e.Request != nil {
					method = strings.ToUpper(strings.TrimSpace(e.Request.Method))
				}
				if method == "DELETE" {
					continue
				}
				if method != "POST" && method != "PUT" && method != "PATCH" {
					continue
				}
				if len(e.Resource) == 0 {
					continue
				}
				var env resourceEnvelope
				if err := json.Unmarshal(e.Resource, &env); err != nil {
					continue
				}
				switch strings.TrimSpace(env.ResourceType) {
				case resourceTypePractitionerRole:
					if env.ID != "" {
						add([]string{env.ID})
					}
				case resourceTypeSchedule:
					for _, a := range env.Actor {
						if id := practitionerRoleIDFromReference(a.Reference); id != "" {
							add([]string{id})
						}
					}
				}
			}
			return mapKeysToSlice(seen)
		}
	}

	// Single resource: parse path and optionally body.
	path := strings.TrimPrefix(req.Path, fhirPathPrefix)
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return mapKeysToSlice(seen)
	}
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "DELETE" {
		return nil
	}
	if method != "POST" && method != "PUT" && method != "PATCH" {
		return mapKeysToSlice(seen)
	}

	resourceType := strings.TrimSpace(parts[0])
	resID := ""
	if len(parts) >= 2 {
		resID = strings.TrimSpace(parts[1])
	}

	switch resourceType {
	case resourceTypePractitionerRole:
		if resID != "" {
			add([]string{resID})
		}
	case resourceTypeSchedule:
		// Parse body for actor references (request body for PUT/PATCH, response for POST create).
		body := req.Body
		if method == "POST" && len(resp.Body) > 0 {
			body = resp.Body
		}
		if len(body) > 0 {
			var env resourceEnvelope
			if err := json.Unmarshal(body, &env); err == nil && strings.EqualFold(env.ResourceType, resourceTypeSchedule) {
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
