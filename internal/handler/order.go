package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kalia/store/internal/model"
	"github.com/kalia/store/internal/service"
	"github.com/kalia/store/pkg/apperr"
)

// OrderHandler serves admin order + payment read endpoints (JWT-protected).
type OrderHandler struct {
	orders   *service.OrderService
	payments *service.PaymentService
}

// NewOrderHandler builds an order handler.
func NewOrderHandler(orders *service.OrderService, payments *service.PaymentService) *OrderHandler {
	return &OrderHandler{orders: orders, payments: payments}
}

// Register mounts order/payment admin routes guarded by mw (JWT).
func (h *OrderHandler) Register(r fiber.Router, mw fiber.Handler) {
	r.Get("/orders", mw, h.List)
	r.Get("/orders/:id", mw, h.Get)
	r.Patch("/orders/:id/cancel", mw, h.Cancel)
	r.Get("/orders/:id/payment", mw, h.OrderPayment)
	r.Get("/payments/:id", mw, h.Payment)
}

// List handles GET /orders?status=&limit=&offset=.
func (h *OrderHandler) List(c *fiber.Ctx) error {
	var status *model.OrderStatus
	if v := c.Query("status"); v != "" {
		st := model.OrderStatus(v)
		if !validOrderStatus(st) {
			return respondError(c, apperr.BadRequest("invalid status filter"))
		}
		status = &st
	}
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)
	items, total, err := h.orders.List(c.Context(), status, limit, offset)
	if err != nil {
		return respondError(c, err)
	}
	if items == nil {
		items = []model.Order{}
	}
	return ok(c, fiber.Map{"items": items, "total": total, "limit": limit, "offset": offset})
}

// Get handles GET /orders/:id.
func (h *OrderHandler) Get(c *fiber.Ctx) error {
	id, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	o, err := h.orders.Get(c.Context(), id)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, o)
}

// Cancel handles PATCH /orders/:id/cancel.
func (h *OrderHandler) Cancel(c *fiber.Ctx) error {
	id, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	o, err := h.orders.Cancel(c.Context(), id)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, o)
}

// OrderPayment handles GET /orders/:id/payment.
func (h *OrderHandler) OrderPayment(c *fiber.Ctx) error {
	id, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	p, err := h.payments.GetPaymentByOrder(c.Context(), id)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, p)
}

// Payment handles GET /payments/:id.
func (h *OrderHandler) Payment(c *fiber.Ctx) error {
	id, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	p, err := h.payments.GetPayment(c.Context(), id)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, p)
}

func validOrderStatus(s model.OrderStatus) bool {
	switch s {
	case model.OrderPending, model.OrderPaid, model.OrderDelivered,
		model.OrderExpired, model.OrderCancelled, model.OrderFailed:
		return true
	}
	return false
}
