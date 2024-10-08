package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachUserRoutes(router chi.Router, middlewares *middlewares.Middlewares, userController *controllers.UserController) {
	router.With(middlewares.Authenticate).Get("/profile", userController.GetUserProfileBySession)
	router.With(middlewares.Authenticate).Put("/profile", userController.UpdateUserBySession)
	router.With(middlewares.Authenticate).Delete("/me", userController.DeactivateUserBySession)
}
