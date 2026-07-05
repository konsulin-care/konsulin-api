package controllers

import (
	"testing"

	"konsulin-service/internal/pkg/constvars"

	"github.com/supertokens/supertokens-golang/recipe/session/sessmodels"
)

// mockAccessTokenPayload builds a SessionContainer whose GetAccessTokenPayload
// returns the given payload. Only the GetAccessTokenPayload field is populated.
func mockSession(payload map[string]interface{}) sessmodels.SessionContainer {
	return &sessmodels.TypeSessionContainer{
		GetAccessTokenPayload: func() map[string]interface{} {
			return payload
		},
	}
}

// buildRolesPayload builds the SuperTokens access token payload with the given
// roles value. rolesValue can be []string, []interface{}, or nil for testing.
func buildRolesPayload(rolesValue interface{}) map[string]interface{} {
	return map[string]interface{}{
		constvars.SupertokenPayloadRolesKey: map[string]interface{}{
			constvars.SupertokenPayloadRolesValueKey: rolesValue,
		},
	}
}

func TestGetUserRolesFromSession(t *testing.T) {
	t.Run("returns error on nil payload", func(t *testing.T) {
		sess := mockSession(nil)
		_, err := getUserRolesFromSession(sess)
		if err == nil {
			t.Error("expected error for nil payload")
		}
	})

	t.Run("returns nil when st-role not in payload", func(t *testing.T) {
		sess := mockSession(map[string]interface{}{
			"sub": "user-123",
		})
		roles, err := getUserRolesFromSession(sess)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if roles != nil {
			t.Errorf("expected nil roles, got %v", roles)
		}
	})

	t.Run("parses roles from []interface{} format", func(t *testing.T) {
		sess := mockSession(buildRolesPayload([]interface{}{"Patient", "Researcher"}))
		roles, err := getUserRolesFromSession(sess)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(roles) != 2 {
			t.Fatalf("expected 2 roles, got %d: %v", len(roles), roles)
		}
		if roles[0] != "Patient" {
			t.Errorf("expected roles[0]=Patient, got %s", roles[0])
		}
		if roles[1] != "Researcher" {
			t.Errorf("expected roles[1]=Researcher, got %s", roles[1])
		}
	})

	t.Run("parses roles from []string format", func(t *testing.T) {
		sess := mockSession(buildRolesPayload([]string{"Patient", "Clinic Admin"}))
		roles, err := getUserRolesFromSession(sess)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(roles) != 2 {
			t.Fatalf("expected 2 roles, got %d: %v", len(roles), roles)
		}
		if roles[0] != "Patient" {
			t.Errorf("expected roles[0]=Patient, got %s", roles[0])
		}
		if roles[1] != "Clinic Admin" {
			t.Errorf("expected roles[1]=Clinic Admin, got %s", roles[1])
		}
	})

	t.Run("returns empty slice when roles value is missing", func(t *testing.T) {
		sess := mockSession(map[string]interface{}{
			constvars.SupertokenPayloadRolesKey: map[string]interface{}{},
		})
		roles, err := getUserRolesFromSession(sess)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if roles != nil {
			t.Errorf("expected nil roles, got %v", roles)
		}
	})

	t.Run("returns error when roles value is unexpected type", func(t *testing.T) {
		sess := mockSession(buildRolesPayload("not-a-list"))
		_, err := getUserRolesFromSession(sess)
		if err == nil {
			t.Error("expected error for unexpected type")
		}
	})
}
