package utils

import (
	"konsulin-service/internal/pkg/dto/requests"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeCreateMagicLinkRequest(t *testing.T) {
	t.Run("Email Sanitization", func(t *testing.T) {
		request := &requests.SupertokenPasswordlessCreateMagicLink{
			Email: "  TEST@EXAMPLE.COM  ",
			Roles: []string{"Practitioner"},
		}

		SanitizeCreateMagicLinkRequest(request)

		assert.Equal(t, "test@example.com", request.Email, "email should be lowercase and trimmed")
	})

	t.Run("Roles Sanitization", func(t *testing.T) {
		request := &requests.SupertokenPasswordlessCreateMagicLink{
			Email: "test@example.com",
			Roles: []string{"  Practitioner  ", "  Researcher  ", "  Clinic Admin  "},
		}

		SanitizeCreateMagicLinkRequest(request)

		expectedRoles := []string{"Practitioner", "Researcher", "Clinic Admin"}
		assert.Equal(t, expectedRoles, request.Roles, "roles should be trimmed")
	})

	t.Run("Mixed Sanitization", func(t *testing.T) {
		request := &requests.SupertokenPasswordlessCreateMagicLink{
			Email: "  USER@DOMAIN.ORG  ",
			Roles: []string{"  Patient  ", "  Practitioner  "},
		}

		SanitizeCreateMagicLinkRequest(request)

		assert.Equal(t, "user@domain.org", request.Email, "email should be lowercase and trimmed")
		expectedRoles := []string{"Patient", "Practitioner"}
		assert.Equal(t, expectedRoles, request.Roles, "roles should be trimmed")
	})

	t.Run("Empty Roles Array", func(t *testing.T) {
		request := &requests.SupertokenPasswordlessCreateMagicLink{
			Email: "test@example.com",
			Roles: []string{},
		}

		SanitizeCreateMagicLinkRequest(request)

		assert.Equal(t, "test@example.com", request.Email, "email should be sanitized")
		assert.Equal(t, []string{}, request.Roles, "empty roles array should remain empty")
	})

	t.Run("Single Role with Whitespace", func(t *testing.T) {
		request := &requests.SupertokenPasswordlessCreateMagicLink{
			Email: "  doctor@clinic.com  ",
			Roles: []string{"  Researcher  "},
		}

		SanitizeCreateMagicLinkRequest(request)

		assert.Equal(t, "doctor@clinic.com", request.Email, "email should be sanitized")
		assert.Equal(t, []string{"Researcher"}, request.Roles, "single role should be trimmed")
	})
}
