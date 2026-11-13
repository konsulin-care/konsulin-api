package research

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// study builds a ResearchStudy carrying the given status and meta.tag codes.
func study(status string, tags ...string) map[string]any {
	tagList := make([]any, 0, len(tags))
	for _, code := range tags {
		tagList = append(tagList, map[string]any{"code": code})
	}
	return map[string]any{
		"resourceType": "ResearchStudy",
		"status":       status,
		"meta":         map[string]any{"tag": tagList},
	}
}

// TestIsPublished_AutoPublishOff covers the autopublish-off contract: an
// active study tagged publishable is exportable, an untagged active study
// is not, and an unpublishable-tagged study is not.
func TestIsPublished_AutoPublishOff(t *testing.T) {
	cases := []struct {
		name  string
		want  bool
		study map[string]any
	}{
		{
			name:  "active tagged publishable is exportable",
			want:  true,
			study: study("active", "publishable"),
		},
		{
			name:  "active untagged is not exportable",
			want:  false,
			study: study("active"),
		},
		{
			name:  "active tagged unpublishable is not exportable",
			want:  false,
			study: study("active", "unpublishable"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsPublished(tc.study, false))
		})
	}
}

// TestIsPublished_AutoPublishOn covers the autopublish-on contract: an
// untagged active study becomes exportable while one tagged unpublishable
// stays excluded, and a publishable-tagged study remains exportable.
func TestIsPublished_AutoPublishOn(t *testing.T) {
	cases := []struct {
		name  string
		want  bool
		study map[string]any
	}{
		{
			name:  "active untagged is exportable",
			want:  true,
			study: study("active"),
		},
		{
			name:  "active tagged unpublishable is not exportable",
			want:  false,
			study: study("active", "unpublishable"),
		},
		{
			name:  "active tagged publishable is exportable",
			want:  true,
			study: study("active", "publishable"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsPublished(tc.study, true))
		})
	}
}

// TestIsPublished_RequiresActiveStatus pins the activity gate: a withdrawn
// or draft study is never exportable regardless of tags or autopublish.
func TestIsPublished_RequiresActiveStatus(t *testing.T) {
	assert.False(t, IsPublished(study("withdrawn", "publishable"), true))
	assert.False(t, IsPublished(study("draft", "publishable"), false))
	assert.False(t, IsPublished(nil, true))
}
