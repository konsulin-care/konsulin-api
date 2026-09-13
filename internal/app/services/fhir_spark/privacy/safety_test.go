package privacy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsDeletable_SelfOnlyReferences(t *testing.T) {
	body := []byte(`{
		"resourceType":"Observation",
		"id":"obs-1",
		"status":"final",
		"subject":{"reference":"Patient/pat-1"}
	}`)
	require.True(t, isDeletable(body, "pat-1"),
		"a resource referencing only the purged patient must be deletable")
}

func TestIsDeletable_OtherPatientReferenceBlocks(t *testing.T) {
	body := []byte(`{
		"resourceType":"Communication",
		"id":"comm-1",
		"status":"completed",
		"sender":{"reference":"Patient/pat-1"},
		"recipient":[{"reference":"Patient/pat-2"}]
	}`)
	require.False(t, isDeletable(body, "pat-1"),
		"a reference to another patient must block deletion")
}

func TestIsDeletable_PractitionerReferenceBlocks(t *testing.T) {
	body := []byte(`{
		"resourceType":"Observation",
		"id":"obs-1",
		"status":"final",
		"subject":{"reference":"Patient/pat-1"},
		"performer":[{"reference":"Practitioner/prac-1"}]
	}`)
	require.False(t, isDeletable(body, "pat-1"),
		"a practitioner reference must block deletion")
}

func TestIsDeletable_NestedExtensionReferenceFound(t *testing.T) {
	body := []byte(`{
		"resourceType":"Communication",
		"id":"comm-1",
		"status":"completed",
		"sender":{"reference":"Patient/pat-1"},
		"extension":[{
			"url":"http://example.org/related",
			"valueReference":{"reference":"Patient/pat-2"}
		}]
	}`)
	require.False(t, isDeletable(body, "pat-1"),
		"nested references inside extensions must be detected")
}

func TestIsDeletable_NoReferences(t *testing.T) {
	body := []byte(`{"resourceType":"Consent","id":"c-1","status":"active"}`)
	require.True(t, isDeletable(body, "pat-1"),
		"a body with no references must be deletable")
}

func TestIsDeletable_NonUserReferencesAllowed(t *testing.T) {
	body := []byte(`{
		"resourceType":"ResearchSubject",
		"id":"rs-1",
		"status":"on-study",
		"individual":{"reference":"Patient/pat-1"},
		"study":{"reference":"ResearchStudy/study-1"},
		"consent":{"reference":"Consent/c-1"}
	}`)
	require.True(t, isDeletable(body, "pat-1"),
		"references to non-user resources (study, consent) must not block deletion")
}

func TestIsDeletable_PractitionerRoleReferenceDoesNotBlock(t *testing.T) {
	body := []byte(`{
		"resourceType":"Communication",
		"id":"comm-1",
		"status":"completed",
		"sender":{"reference":"Patient/pat-1"},
		"recipient":[{"reference":"PractitionerRole/role-1"}]
	}`)
	require.True(t, isDeletable(body, "pat-1"),
		"PractitionerRole references are not user identities and must not block deletion")
}
