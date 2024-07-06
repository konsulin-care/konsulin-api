package patients

import (
	"context"
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"time"

	"github.com/gofiber/fiber/v2"
)

type PatientController struct {
	PatientUsecase PatientUsecase
}

func NewPatientController(patientUsecase PatientUsecase) *PatientController {
	return &PatientController{
		PatientUsecase: patientUsecase,
	}
}

func (ctrl *PatientController) GetPatientProfileBySession(c *fiber.Ctx) error {
	sessionData := c.Locals("sessionData").(string)
	fmt.Println(sessionData)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := ctrl.PatientUsecase.GetPatientProfileBySession(ctx, sessionData)
	if err != nil {
		if err == context.DeadlineExceeded {
			return exceptions.WrapWithoutError(constvars.StatusRequestTimeout, constvars.ErrClientServerLongRespond, constvars.ErrDevServerDeadlineExceeded)
		}
		return err
	}

	return c.Status(constvars.StatusOK).JSON(utils.BuildSuccessResponse(constvars.ProfileGetSuccess, result))
}

func (ctrl *PatientController) UpdateProfileBySession(c *fiber.Ctx) error {
	request := new(requests.UpdateProfile)
	err := c.BodyParser(&request)
	if err != nil {
		return exceptions.WrapWithError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, constvars.ErrDevCannotParseJSON)
	}

	// Validate request
	err = utils.ValidateStruct(request)
	if err != nil {
		return exceptions.WrapWithError(err, constvars.StatusBadRequest, utils.FormatFirstValidationError(err), constvars.ErrDevValidationFailed)
	}

	sessionData := c.Locals("sessionData").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.PatientUsecase.UpdatePatientProfileBySession(ctx, sessionData, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			return exceptions.WrapWithoutError(constvars.StatusRequestTimeout, constvars.ErrClientServerLongRespond, constvars.ErrDevServerDeadlineExceeded)
		}
		return err
	}

	return c.Status(fiber.StatusOK).JSON(utils.BuildSuccessResponse(constvars.UserUpdatedSuccess, response))
}
