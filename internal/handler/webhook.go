package handler

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"

	"github.com/kalia/store/internal/service"
	"github.com/kalia/store/pkg/apperr"
)

// WebhookHandler serves the public Midtrans notification endpoint.
type WebhookHandler struct {
	payments *service.PaymentService
}

// NewWebhookHandler builds a webhook handler.
func NewWebhookHandler(payments *service.PaymentService) *WebhookHandler {
	return &WebhookHandler{payments: payments}
}

// midtransNotification is the subset of the Core API notification we consume.
type midtransNotification struct {
	OrderID           string `json:"order_id"`
	TransactionID     string `json:"transaction_id"`
	TransactionStatus string `json:"transaction_status"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	FraudStatus       string `json:"fraud_status"`
	SignatureKey      string `json:"signature_key"`
}

// Register mounts the public webhook route (no JWT; signature-gated).
func (h *WebhookHandler) Register(app *fiber.App) {
	app.Post("/webhooks/midtrans", h.Midtrans)
}

// Midtrans handles POST /webhooks/midtrans.
func (h *WebhookHandler) Midtrans(c *fiber.Ctx) error {
	raw := c.Body()
	var n midtransNotification
	if err := json.Unmarshal(raw, &n); err != nil {
		return respondError(c, apperr.BadRequest("invalid notification body"))
	}

	err := h.payments.HandleNotification(c.Context(), service.Notification{
		OrderRef:          n.OrderID,
		TransactionID:     n.TransactionID,
		TransactionStatus: n.TransactionStatus,
		StatusCode:        n.StatusCode,
		GrossAmount:       n.GrossAmount,
		FraudStatus:       n.FraudStatus,
		SignatureKey:      n.SignatureKey,
		Raw:               raw,
	})
	if err != nil {
		return respondError(c, err)
	}
	// Fast 2xx so Midtrans does not needlessly retry.
	return ok(c, fiber.Map{"status": "ok"})
}
