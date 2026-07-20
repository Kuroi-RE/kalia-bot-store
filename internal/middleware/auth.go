package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/kalia/store/pkg/token"
)

// Context keys for values stashed by middleware.
const (
	CtxAdminID  = "admin_id"
	CtxUsername = "admin_username"
)

// JWTAuth verifies a Bearer token and stashes admin identity in the context.
func JWTAuth(tm *token.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authz := c.Get(fiber.HeaderAuthorization)
		if authz == "" || !strings.HasPrefix(authz, "Bearer ") {
			return unauthorized(c, "missing bearer token")
		}
		raw := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
		claims, err := tm.Verify(raw)
		if err != nil {
			return unauthorized(c, "invalid or expired token")
		}
		c.Locals(CtxAdminID, claims.AdminID)
		c.Locals(CtxUsername, claims.Username)
		return c.Next()
	}
}

// BotAuth guards /bot/* endpoints with a static service token.
func BotAuth(serviceToken string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Accept either "X-Bot-Token" header or "Bearer <token>".
		provided := c.Get("X-Bot-Token")
		if provided == "" {
			authz := c.Get(fiber.HeaderAuthorization)
			if strings.HasPrefix(authz, "Bearer ") {
				provided = strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
			}
		}
		if provided == "" || subtleCompare(provided, serviceToken) == false {
			return unauthorized(c, "invalid bot service token")
		}
		return c.Next()
	}
}

// AdminIDFromCtx extracts the authenticated admin id (0 if absent).
func AdminIDFromCtx(c *fiber.Ctx) int64 {
	if v, ok := c.Locals(CtxAdminID).(int64); ok {
		return v
	}
	return 0
}

func unauthorized(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"error": fiber.Map{"code": "unauthorized", "message": msg},
	})
}
