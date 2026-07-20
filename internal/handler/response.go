package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kalia/store/pkg/apperr"
)

// errorBody is the standard error envelope returned to clients.
type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// respondError maps a domain/unknown error to a JSON response.
func respondError(c *fiber.Ctx, err error) error {
	var body errorBody
	status := fiber.StatusInternalServerError
	body.Error.Code = "internal"
	body.Error.Message = "internal server error"

	if e, ok := apperr.As(err); ok {
		status = e.Status
		body.Error.Code = e.Code
		body.Error.Message = e.Message
	}
	return c.Status(status).JSON(body)
}

// ok writes a 200 JSON payload.
func ok(c *fiber.Ctx, data any) error {
	return c.Status(fiber.StatusOK).JSON(data)
}

// created writes a 201 JSON payload.
func created(c *fiber.Ctx, data any) error {
	return c.Status(fiber.StatusCreated).JSON(data)
}
