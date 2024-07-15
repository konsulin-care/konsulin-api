package routers

import (
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/services/core/auth"

	"github.com/go-chi/chi/v5"
)

func attachAuthRoutes(router chi.Router, middlewares *middlewares.Middlewares, authController *auth.AuthController) {
	router.Post("/register/patient", authController.RegisterPatient)
	router.Post("/register/clinician", authController.RegisterClinician)
	router.Post("/login/patient", authController.LoginPatient)
	router.Post("/login/clinician", authController.LoginClinician)
	router.Post("/forgot-password", authController.Logout)
	router.With(middlewares.Authenticate).Post("/logout", authController.Logout)
}
