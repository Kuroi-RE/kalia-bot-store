package server

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
)

// New builds a configured Fiber app with baseline middleware.
func New(log *slog.Logger, prod bool, corsOrigins string) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:               "kalia-store",
		DisableStartupMessage: prod,
		ReadTimeout:           15 * time.Second,
		WriteTimeout:          20 * time.Second,
		ErrorHandler:          errorHandler(log),
	})

	app.Use(requestid.New())
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: corsOrigins,
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization,X-Bot-Token",
	}))
	app.Use(requestLogger(log))

	return app
}

// errorHandler is the last-resort handler for panics / unhandled errors.
func errorHandler(log *slog.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError
		if fe, ok := err.(*fiber.Error); ok {
			code = fe.Code
		}
		log.Error("unhandled request error",
			slog.String("path", c.Path()),
			slog.String("method", c.Method()),
			slog.Int("status", code),
			slog.Any("error", err),
		)
		return c.Status(code).JSON(fiber.Map{
			"error": fiber.Map{"code": "internal", "message": "internal server error"},
		})
	}
}

// requestLogger logs each request with method, path, status and latency.
func requestLogger(log *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		log.Info("request",
			slog.String("id", c.GetRespHeader(fiber.HeaderXRequestID)),
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", c.Response().StatusCode()),
			slog.Duration("latency", time.Since(start)),
		)
		return err
	}
}
