package routers

import (
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/services/patients"

	"github.com/gofiber/fiber/v2"
)

func attachPatientRoutes(router fiber.Router, middlewares *middlewares.Middlewares, patientController *patients.PatientController) {
	router.Get("/me", middlewares.AuthMiddleware, patientController.GetPatientProfileBySession)
	router.Put("/me", middlewares.AuthMiddleware, patientController.UpdateProfileBySession)
}
