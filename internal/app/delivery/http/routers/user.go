package routers

import (
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/services/core/users"

	"github.com/go-chi/chi/v5"
)

func attachUserRoutes(router chi.Router, middlewares *middlewares.Middlewares, userController *users.UserController) {
	router.With(middlewares.Authenticate).Get("/profile", userController.GetUserProfileBySession)
	router.With(middlewares.Authenticate).Put("/profile", userController.UpdateUserBySession)
	router.With(middlewares.Authenticate).Delete("/me", userController.DeleteUserBySession)
}
