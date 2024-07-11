package routers

import (
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/services/users"

	"github.com/go-chi/chi/v5"
)

func attachUserRoutes(router chi.Router, middlewares *middlewares.Middlewares, userController *users.UserController) {
	router.With(middlewares.AuthMiddleware).Get("/me", userController.GetUserProfileBySession)
	router.With(middlewares.AuthMiddleware).Put("/me", userController.GetUserProfileBySession)
}
