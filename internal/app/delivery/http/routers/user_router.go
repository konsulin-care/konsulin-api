package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/session/claims"
	"github.com/supertokens/supertokens-golang/recipe/session/sessmodels"
	"github.com/supertokens/supertokens-golang/recipe/userroles/userrolesclaims"
	"github.com/supertokens/supertokens-golang/supertokens"
)

func attachUserRoutes(router chi.Router, middlewares *middlewares.Middlewares, userController *controllers.UserController) {
	router.Get("/profile", session.VerifySession(&sessmodels.VerifySessionOptions{
		OverrideGlobalClaimValidators: func(globalClaimValidators []claims.SessionClaimValidator, sessionContainer sessmodels.SessionContainer, userContext supertokens.UserContext) ([]claims.SessionClaimValidator, error) {
			globalClaimValidators = append(globalClaimValidators, userrolesclaims.UserRoleClaimValidators.Includes("clinician", nil, nil))
			return globalClaimValidators, nil
		},
	}, userController.GetUserProfileBySession))

	router.Put("/profile", session.VerifySession(&sessmodels.VerifySessionOptions{
		OverrideGlobalClaimValidators: func(globalClaimValidators []claims.SessionClaimValidator, sessionContainer sessmodels.SessionContainer, userContext supertokens.UserContext) ([]claims.SessionClaimValidator, error) {
			globalClaimValidators = append(globalClaimValidators, userrolesclaims.UserRoleClaimValidators.Includes("clinician", nil, nil))
			return globalClaimValidators, nil
		},
	}, userController.UpdateUserBySession))

	router.Delete("/me", session.VerifySession(&sessmodels.VerifySessionOptions{
		OverrideGlobalClaimValidators: func(globalClaimValidators []claims.SessionClaimValidator, sessionContainer sessmodels.SessionContainer, userContext supertokens.UserContext) ([]claims.SessionClaimValidator, error) {
			globalClaimValidators = append(globalClaimValidators, userrolesclaims.UserRoleClaimValidators.Includes("clinician", nil, nil))
			return globalClaimValidators, nil
		},
	}, userController.DeactivateUserBySession))
}
