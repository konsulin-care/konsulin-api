package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachAuthRoutes(router chi.Router, middlewares *middlewares.Middlewares, authController *controllers.AuthController) {
	router.Post("/register/patient", authController.RegisterPatient)
	router.Post("/register/clinician", authController.RegisterClinician)
	router.Post("/register/whatsapp", authController.RegisterViaWhatsApp)
	router.Post("/register/whatsapp/verify", authController.VerifyRegisterWhatsAppOTP)
	router.Post("/login/whatsapp", authController.RegisterViaWhatsApp)
	router.Post("/login/whatsapp/verify", authController.VerifyRegisterWhatsAppOTP)
	router.Post("/login/patient", authController.LoginPatient)
	router.Post("/login/clinician", authController.LoginClinician)
	router.Post("/forgot-password", authController.ForgotPassword)
	router.Post("/reset-password", authController.ResetPassword)
	router.With(middlewares.Authenticate).Post("/logout", authController.Logout)
}
