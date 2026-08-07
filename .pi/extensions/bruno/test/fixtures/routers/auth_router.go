package routers

import (
	"github.com/go-chi/chi/v5"
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"
)

func attachAuthRoutes(router chi.Router, m *middlewares.Middlewares, authController *controllers.AuthController) {
	router.With(m.RequireSuperadminAPIKey).Post("/magiclink", authController.CreateMagicLink)
	router.Post("/anonymous-session", authController.CreateAnonymousSession)
	router.Patch("/anonymous/claim", authController.ClaimAnonymousResources)
	router.Get("/passwordless/email/exists", authController.PasswordlessEmailExists)
	router.Post("/active-role", authController.SetActiveRole)
}
