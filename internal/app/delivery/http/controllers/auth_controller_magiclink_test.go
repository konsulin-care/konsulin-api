package controllers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNormalizeAndValidateMagicLinkRequest(t *testing.T) {
	ctrl := &AuthController{Log: zap.NewNop()}

	t.Run("valid email-only passes", func(t *testing.T) {
		req := &requests.SupertokenPasswordlessCreateMagicLink{Email: "user@test.com"}
		phoneDigits, err := ctrl.normalizeAndValidateMagicLinkRequest(req, "req-1")
		assert.NoError(t, err)
		assert.Equal(t, "", phoneDigits)
	})

	t.Run("valid phone is normalized to digits", func(t *testing.T) {
		req := &requests.SupertokenPasswordlessCreateMagicLink{Phone: "+62 812-3456-7890"}
		phoneDigits, err := ctrl.normalizeAndValidateMagicLinkRequest(req, "req-1")
		assert.NoError(t, err)
		assert.Equal(t, "6281234567890", phoneDigits)
		assert.Equal(t, "6281234567890", req.Phone, "request.Phone must be persisted normalized")
	})

	t.Run("email and phone mutually exclusive", func(t *testing.T) {
		req := &requests.SupertokenPasswordlessCreateMagicLink{Email: "user@test.com", Phone: "+6281234567890"}
		_, err := ctrl.normalizeAndValidateMagicLinkRequest(req, "req-1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mutually exclusive")
	})

	t.Run("neither email nor phone rejected", func(t *testing.T) {
		req := &requests.SupertokenPasswordlessCreateMagicLink{}
		_, err := ctrl.normalizeAndValidateMagicLinkRequest(req, "req-1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "either email or phone is required")
	})

	t.Run("invalid redirect path rejected", func(t *testing.T) {
		req := &requests.SupertokenPasswordlessCreateMagicLink{Email: "user@test.com", RedirectToPath: "https://evil.com"}
		_, err := ctrl.normalizeAndValidateMagicLinkRequest(req, "req-1")
		assert.Error(t, err)
	})

	t.Run("too-short phone rejected", func(t *testing.T) {
		req := &requests.SupertokenPasswordlessCreateMagicLink{Phone: "123"}
		_, err := ctrl.normalizeAndValidateMagicLinkRequest(req, "req-1")
		assert.Error(t, err)
	})

	t.Run("invalid email rejected by struct validation", func(t *testing.T) {
		req := &requests.SupertokenPasswordlessCreateMagicLink{Email: "not-an-email"}
		_, err := ctrl.normalizeAndValidateMagicLinkRequest(req, "req-1")
		assert.Error(t, err)
	})
}

func TestNormalizeAndValidateMagicLinkRequest_OrganizationRequiredForAdminRoles(t *testing.T) {
	ctrl := &AuthController{Log: zap.NewNop()}

	t.Run("clinic admin without organizationId rejected", func(t *testing.T) {
		req := &requests.SupertokenPasswordlessCreateMagicLink{
			Email: "admin@test.com",
			Roles: []string{constvars.KonsulinRoleClinicAdmin},
		}
		_, err := ctrl.normalizeAndValidateMagicLinkRequest(req, "req-1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "organizationId")
	})

	t.Run("researcher without organizationId rejected", func(t *testing.T) {
		req := &requests.SupertokenPasswordlessCreateMagicLink{
			Email: "researcher@test.com",
			Roles: []string{constvars.KonsulinRoleResearcher},
		}
		_, err := ctrl.normalizeAndValidateMagicLinkRequest(req, "req-1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "organizationId")
	})

	t.Run("clinic admin with organizationId passes", func(t *testing.T) {
		req := &requests.SupertokenPasswordlessCreateMagicLink{
			Email:          "admin@test.com",
			Roles:          []string{constvars.KonsulinRoleClinicAdmin},
			OrganizationID: "org-1",
		}
		_, err := ctrl.normalizeAndValidateMagicLinkRequest(req, "req-1")
		assert.NoError(t, err)
	})

	t.Run("researcher with organizationId passes", func(t *testing.T) {
		req := &requests.SupertokenPasswordlessCreateMagicLink{
			Email:          "researcher@test.com",
			Roles:          []string{constvars.KonsulinRoleResearcher},
			OrganizationID: "org-1",
		}
		_, err := ctrl.normalizeAndValidateMagicLinkRequest(req, "req-1")
		assert.NoError(t, err)
	})

	t.Run("patient without organizationId passes", func(t *testing.T) {
		req := &requests.SupertokenPasswordlessCreateMagicLink{
			Email: "patient@test.com",
			Roles: []string{constvars.KonsulinRolePatient},
		}
		_, err := ctrl.normalizeAndValidateMagicLinkRequest(req, "req-1")
		assert.NoError(t, err)
	})
}

func TestValidateMagicLinkRoles(t *testing.T) {
	ctrl := &AuthController{Log: zap.NewNop()}

	t.Run("valid roles pass", func(t *testing.T) {
		err := ctrl.validateMagicLinkRoles("req-1", "user@test.com", []string{constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner})
		assert.NoError(t, err)
	})

	t.Run("empty roles pass", func(t *testing.T) {
		assert.NoError(t, ctrl.validateMagicLinkRoles("req-1", "user@test.com", nil))
	})

	t.Run("unknown role rejected", func(t *testing.T) {
		err := ctrl.validateMagicLinkRoles("req-1", "user@test.com", []string{"Admin"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid role")
	})
}

func TestWriteUsecaseError(t *testing.T) {
	ctrl := &AuthController{Log: zap.NewNop()}

	t.Run("deadline error maps to gateway timeout", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctrl.writeUsecaseError(rec, "req-1", "test op", context.DeadlineExceeded)
		assert.Equal(t, http.StatusGatewayTimeout, rec.Code)
	})

	t.Run("plain error writes internal error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctrl.writeUsecaseError(rec, "req-1", "test op", errors.New("boom"))
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("custom error preserves its status", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctrl.writeUsecaseError(rec, "req-1", "test op", exceptions.ErrInputValidation(errors.New("bad")))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
