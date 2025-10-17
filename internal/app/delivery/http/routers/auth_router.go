package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachAuthRoutes(router chi.Router, middlewares *middlewares.Middlewares, authController *controllers.AuthController) {
	router.With(middlewares.RequireSuperadminAPIKey).Post("/magiclink", authController.CreateMagicLink)
	router.Post("/anonymous-session", authController.CreateAnonymousSession)
	router.With(middlewares.Authenticate).Post("/logout", authController.Logout)
}
