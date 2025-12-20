package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/pkg/constvars"

	"github.com/go-chi/chi/v5"
	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/session/sessmodels"
)

func attachUserRoutes(router chi.Router, middlewares *middlewares.Middlewares, userController *controllers.UserController) {
	router.Get("/profile", session.VerifySession(&sessmodels.VerifySessionOptions{
		OverrideGlobalClaimValidators: middlewares.RequirePermission(constvars.ResourcePatient, constvars.MethodGet),
	}, userController.GetUserProfileBySession))

	router.Put("/profile", session.VerifySession(&sessmodels.VerifySessionOptions{
		OverrideGlobalClaimValidators: middlewares.RequirePermission(constvars.ResourcePatient, constvars.MethodPut),
	}, userController.UpdateUserBySession))

	router.Delete("/me", session.VerifySession(&sessmodels.VerifySessionOptions{
		OverrideGlobalClaimValidators: middlewares.RequirePermission(constvars.ResourcePatient, constvars.MethodDelete),
	}, userController.DeactivateUserBySession))
}
