package auth

import (
	"context"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"time"

	"github.com/gofiber/fiber/v2"
)

type AuthController struct {
	AuthUsecase AuthUsecase
}

func NewAuthController(authUsecase AuthUsecase) *AuthController {
	return &AuthController{
		AuthUsecase: authUsecase,
	}
}

func (ctrl *AuthController) RegisterPatient(c *fiber.Ctx) error {
	request := new(requests.RegisterPatient)
	err := c.BodyParser(&request)
	if err != nil {
		return exceptions.WrapWithError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, constvars.ErrDevCannotParseJSON)
	}
	// Sanitize register patient request
	utils.SanitizeCreatePatientRequest(request)

	// Validate register patient request
	err = utils.ValidateStruct(request)
	if err != nil {
		return exceptions.WrapWithError(err, constvars.StatusBadRequest, utils.FormatFirstValidationError(err), constvars.ErrDevValidationFailed)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AuthUsecase.RegisterPatient(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			return exceptions.WrapWithoutError(constvars.StatusRequestTimeout, constvars.ErrClientServerLongRespond, constvars.ErrDevServerDeadlineExceeded)
		}
		return err
	}

	return c.Status(constvars.StatusCreated).JSON(utils.BuildSuccessResponse(constvars.UserCreatedSuccess, response))
}

func (ctrl *AuthController) Login(c *fiber.Ctx) error {
	request := new(requests.LoginPatient)
	err := c.BodyParser(&request)
	if err != nil {
		return exceptions.WrapWithError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, constvars.ErrDevCannotParseJSON)
	}

	// Validate request
	err = utils.ValidateStruct(request)
	if err != nil {
		return exceptions.WrapWithError(err, constvars.StatusBadRequest, utils.FormatFirstValidationError(err), constvars.ErrDevValidationFailed)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AuthUsecase.LoginPatient(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			return exceptions.WrapWithoutError(constvars.StatusRequestTimeout, constvars.ErrClientServerLongRespond, constvars.ErrDevServerDeadlineExceeded)
		}
		return err
	}

	return c.Status(constvars.StatusOK).JSON(utils.BuildSuccessResponse(constvars.LoginSuccess, response))
}

func (ctrl *AuthController) Logout(c *fiber.Ctx) error {
	// sessionData := c.Locals("sessionData").(string)
	sessionID := c.Locals("sessionID").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := ctrl.AuthUsecase.LogoutPatient(ctx, sessionID)
	if err != nil {
		if err == context.DeadlineExceeded {
			return exceptions.WrapWithoutError(constvars.StatusRequestTimeout, constvars.ErrClientServerLongRespond, constvars.ErrDevServerDeadlineExceeded)
		}
		return err
	}
	return c.Status(constvars.StatusOK).JSON(utils.BuildSuccessResponse(constvars.LogoutSuccess, nil))
}
