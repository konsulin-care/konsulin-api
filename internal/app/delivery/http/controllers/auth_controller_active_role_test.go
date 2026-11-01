package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"konsulin-service/internal/pkg/constvars"

	"github.com/stretchr/testify/require"
	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/session/claims"
	"github.com/supertokens/supertokens-golang/recipe/session/sessmodels"
	"github.com/supertokens/supertokens-golang/supertokens"
	"go.uber.org/zap"
)

func TestSetActiveRoleInvalidatesPreviousIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, previous, selected string
		wantCleared              bool
	}{
		{"switch to patient", constvars.KonsulinRolePractitioner, constvars.KonsulinRolePatient, true},
		{"switch to practitioner", constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner, true},
		{"same selection keeps identity", constvars.KonsulinRolePatient, constvars.KonsulinRolePatient, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			supertokens.ResetForTest()
			t.Cleanup(supertokens.ResetForTest)
			payload := map[string]interface{}{
				constvars.SupertokenPayloadActiveRoleKey: tc.previous,
				constvars.SupertokenPayloadRolesKey: map[string]interface{}{
					constvars.SupertokenPayloadRolesValueKey: []interface{}{constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner},
				},
			}
			var update map[string]interface{}
			sess := &sessmodels.TypeSessionContainer{
				GetUserIDWithContext:               func(_ supertokens.UserContext) string { return "user" },
				GetTenantIdWithContext:             func(_ supertokens.UserContext) string { return "public" },
				GetAccessTokenPayload:              func() map[string]interface{} { return payload },
				AssertClaimsWithContext:            func(_ []claims.SessionClaimValidator, _ supertokens.UserContext) error { return nil },
				AttachToRequestResponseWithContext: func(_ sessmodels.RequestResponseInfo, _ supertokens.UserContext) error { return nil },
				MergeIntoAccessTokenPayload: func(value map[string]interface{}) error {
					update = value
					return nil
				},
			}
			telemetry := false
			require.NoError(t, supertokens.Init(supertokens.TypeInput{
				Supertokens: &supertokens.ConnectionInfo{ConnectionURI: "http://localhost:9999"},
				AppInfo:     supertokens.AppInfo{AppName: "role-switch-test", APIDomain: "http://localhost:3200", WebsiteDomain: "http://localhost:3000"},
				Telemetry:   &telemetry,
				RecipeList: []supertokens.Recipe{session.Init(&sessmodels.TypeInput{Override: &sessmodels.OverrideStruct{Functions: func(original sessmodels.RecipeInterface) sessmodels.RecipeInterface {
					*original.GetSession = func(_, _ *string, _ *sessmodels.VerifySessionOptions, _ supertokens.UserContext) (sessmodels.SessionContainer, error) {
						return sess, nil
					}
					return original
				}}})},
			}))
			ctx := context.WithValue(context.Background(), constvars.CONTEXT_REQUEST_ID_KEY, "role-switch-test")
			request := httptest.NewRequest(http.MethodPost, "/auth/active-role", strings.NewReader(`{"role":"`+tc.selected+`"}`)).WithContext(ctx)
			request.Header.Set("Authorization", "Bearer test-token")
			recorder := httptest.NewRecorder()
			controller := &AuthController{Log: zap.NewNop()}
			controller.SetActiveRole(recorder, request)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, tc.selected, update[constvars.SupertokenPayloadActiveRoleKey])
			value, cleared := update[constvars.SupertokenPayloadFhirResourceIDKey]
			require.Equal(t, tc.wantCleared, cleared)
			if tc.wantCleared {
				require.Equal(t, "", value)
			}
		})
	}
}
