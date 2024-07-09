package routers

import (
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/services/auth"

	"github.com/go-chi/chi/v5"
)

func attachAuthRoutes(router chi.Router, middlewares *middlewares.Middlewares, authController *auth.AuthController) {
	router.Post("/register", authController.RegisterPatient)
	router.Post("/login", authController.Login)
	router.With(middlewares.AuthMiddleware).Post("/logout", authController.Logout)
}
