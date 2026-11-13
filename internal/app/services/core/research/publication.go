package research

import "konsulin-service/internal/pkg/constvars"

// Publication tag codes recognized on a ResearchStudy's meta.tag.
const (
	studyTagPublishable   = "publishable"
	studyTagUnpublishable = "unpublishable"
)

// studyStatusActive is the only ResearchStudy status eligible for export.
const studyStatusActive = "active"

// IsPublished reports whether a ResearchStudy's derived resources may be
// exported (BK-DATABANK publication registry). A study qualifies when it is
// active AND either tagged "publishable" in meta.tag, or — when autoPublish
// (APP_AUTOPUBLISH) is true — not tagged "unpublishable". A study tagged
// unpublishable is never exportable, and a non-active or nil study is never
// exportable regardless of tags or autoPublish.
// Parameters: study is the raw FHIR ResearchStudy map (nil-safe); autoPublish
// is the APP_AUTOPUBLISH flag widening the default from deny to allow.
// Returns true when the study's resources are exportable; false otherwise.
func IsPublished(study map[string]any, autoPublish bool) bool {
	if status, _ := study[constvars.FhirFieldStatus].(string); status != studyStatusActive {
		return false
	}
	publishable := studyTagged(study, studyTagPublishable)
	unpublishable := studyTagged(study, studyTagUnpublishable)
	if unpublishable {
		return false
	}
	return publishable || autoPublish
}

// studyTagged reports whether the study carries code among its meta.tag
// codings. Missing meta/tag structures report false.
func studyTagged(study map[string]any, code string) bool {
	meta, ok := study[constvars.FhirFieldMeta].(map[string]any)
	if !ok {
		return false
	}
	tags, ok := meta[constvars.FhirFieldTag].([]any)
	if !ok {
		return false
	}
	for _, raw := range tags {
		coding, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if tagCode, _ := coding[constvars.FhirFieldCode].(string); tagCode == code {
			return true
		}
	}
	return false
}
