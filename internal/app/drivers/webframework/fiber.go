package webframework

import (
	"errors"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"time"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/sirupsen/logrus"
)

func NewFiber(driverConfig *config.DriverConfig) *fiber.App {
	fiberConfig := fiber.Config{
		// Prefork:       true,
		CaseSensitive: true,
		StrictRouting: true,
		JSONEncoder:   json.Marshal,
		JSONDecoder:   json.Unmarshal,
		AppName:       fmt.Sprintf("Konsulin Service %s", driverConfig.App.Version),
		BodyLimit:     driverConfig.App.RequestBodyLimitInMegabyte * 1024 * 1024,
		ErrorHandler:  ErrorHandler,
	}
	app := fiber.New(fiberConfig)

	app.Use(cors.New())
	app.Use(limiter.New(limiter.Config{
		Next: func(c *fiber.Ctx) bool {
			return c.IP() == "127.0.0.1"
		},
		Max:        10,
		Expiration: 30 * time.Second,
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"success": false,
				"error":   "Too many requests on single time-frame",
			})
		},
	}))
	app.Use(logger.New(logger.Config{
		Format:     "${time} | ${ip} | ${status} ==> ${latency} | ${method} | ${path}\n",
		TimeFormat: time.RFC850,
		TimeZone:   driverConfig.App.Timezone,
	}))
	return app
}

func ErrorHandler(ctx *fiber.Ctx, err error) error {
	code := constvars.StatusInternalServerError
	clientMessage := constvars.ErrClientSomethingWrongWithApplication

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		code = fiberErr.Code
		clientMessage = fiberErr.Message
		logrus.WithFields(logrus.Fields{
			"location": logrus.Fields{
				"file":          constvars.ErrFileLocationUnknown,
				"line":          constvars.ErrLineLocationUnknown,
				"function_name": constvars.ErrFunctionNameUnknown,
			},
		}).Error(fiberErr.Message)
	}

	var customErr *exceptions.CustomError
	if errors.As(err, &customErr) {
		code = customErr.StatusCode
		clientMessage = customErr.ClientMessage
		logrus.WithFields(logrus.Fields{
			"location": logrus.Fields{
				"file":          customErr.Location.File,
				"line":          customErr.Location.Line,
				"function_name": customErr.Location.FunctionName,
			},
		}).Error(customErr.DevMessage)
	} else {
		logrus.Error(err)
	}

	ctx.Set(constvars.HeaderContentType, constvars.MIMEApplicationJSON)
	return ctx.Status(code).JSON(fiber.Map{
		"status_code": code,
		"success":     false,
		"message":     clientMessage,
	})
}
