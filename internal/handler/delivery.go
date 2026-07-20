package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kalia/store/internal/model"
	"github.com/kalia/store/internal/service"
	"github.com/kalia/store/pkg/apperr"
)

// DeliveryHandler serves admin delivery endpoints (JWT-protected).
type DeliveryHandler struct {
	deliveries *service.DeliveryService
}

// NewDeliveryHandler builds a delivery handler.
func NewDeliveryHandler(deliveries *service.DeliveryService) *DeliveryHandler {
	return &DeliveryHandler{deliveries: deliveries}
}

// Register mounts delivery routes guarded by mw (JWT).
func (h *DeliveryHandler) Register(r fiber.Router, mw fiber.Handler) {
	r.Get("/deliveries", mw, h.List)
	r.Post("/orders/:id/redeliver", mw, h.Redeliver)
}

// List handles GET /deliveries?status=&limit=&offset=.
func (h *DeliveryHandler) List(c *fiber.Ctx) error {
	var status *model.DeliveryStatus
	if v := c.Query("status"); v != "" {
		st := model.DeliveryStatus(v)
		switch st {
		case model.DeliveryPending, model.DeliveryDelivered, model.DeliveryFailed:
			status = &st
		default:
			return respondError(c, apperr.BadRequest("invalid status filter"))
		}
	}
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)
	items, total, err := h.deliveries.List(c.Context(), status, limit, offset)
	if err != nil {
		return respondError(c, err)
	}
	if items == nil {
		items = []model.Delivery{}
	}
	return ok(c, fiber.Map{"items": items, "total": total, "limit": limit, "offset": offset})
}

// Redeliver handles POST /orders/:id/redeliver.
func (h *DeliveryHandler) Redeliver(c *fiber.Ctx) error {
	id, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	if err := h.deliveries.Redeliver(c.Context(), id); err != nil {
		return respondError(c, err)
	}
	return ok(c, fiber.Map{"status": "redelivered", "order_id": id})
}
