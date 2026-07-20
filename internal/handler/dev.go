package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kalia/store/internal/service"
)

// DevHandler exposes local-development helpers. It is mounted ONLY when the
// payment mode is "fake" and must never be enabled in production.
type DevHandler struct {
	payments *service.PaymentService
}

// NewDevHandler builds a dev handler.
func NewDevHandler(payments *service.PaymentService) *DevHandler {
	return &DevHandler{payments: payments}
}

// Register mounts dev routes under /dev.
func (h *DevHandler) Register(r fiber.Router) {
	grp := r.Group("/dev")
	grp.Post("/settle/:order_ref", h.Settle)
}

// Settle simulates a settlement for an order (fake payment mode only).
func (h *DevHandler) Settle(c *fiber.Ctx) error {
	ref := c.Params("order_ref")
	paid, err := h.payments.ForceSettle(c.Context(), ref)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, fiber.Map{"order_ref": ref, "newly_paid": paid, "mode": "fake"})
}
