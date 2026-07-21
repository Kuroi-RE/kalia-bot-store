package handler

import "github.com/gofiber/fiber/v2"

// ConfigHandler exposes non-secret runtime config the dashboard needs.
type ConfigHandler struct {
	paymentProvider    string
	manualConfirmation bool
}

// NewConfigHandler builds a config handler.
func NewConfigHandler(paymentProvider string, manualConfirmation bool) *ConfigHandler {
	return &ConfigHandler{paymentProvider: paymentProvider, manualConfirmation: manualConfirmation}
}

// Register mounts the config route guarded by mw (JWT).
func (h *ConfigHandler) Register(r fiber.Router, mw fiber.Handler) {
	r.Get("/config", mw, h.Get)
}

// Get handles GET /config.
func (h *ConfigHandler) Get(c *fiber.Ctx) error {
	return ok(c, fiber.Map{
		"payment_provider":            h.paymentProvider,
		"requires_manual_confirmation": h.manualConfirmation,
	})
}
