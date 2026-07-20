package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/kalia/store/pkg/apperr"
)

// idParam parses the :id path parameter as an int64.
func idParam(c *fiber.Ctx) (int64, error) {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.BadRequest("invalid id parameter")
	}
	return id, nil
}

// queryInt parses an integer query param, returning fallback when absent/invalid.
func queryInt(c *fiber.Ctx, key string, fallback int) int {
	v := c.Query(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}
