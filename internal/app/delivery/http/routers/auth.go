package routers

import (
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/services/core/auth"

	"github.com/go-chi/chi/v5"
)

func attachAuthRoutes(router chi.Router, middlewares *middlewares.Middlewares, authController *auth.AuthController) {
	router.Post("/register", authController.RegisterUser)
	router.Post("/login", authController.LoginUser)
	router.With(middlewares.Authenticate).Post("/logout", authController.Logout)
}
