package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kalia/store/internal/model"
	"github.com/kalia/store/internal/service"
)

// BotHandler serves bot-facing catalog and order endpoints (bot-token-protected).
type BotHandler struct {
	products *service.ProductService
	accounts *service.AccountService
	orders   *service.OrderService
}

// NewBotHandler builds a bot handler.
func NewBotHandler(products *service.ProductService, accounts *service.AccountService, orders *service.OrderService) *BotHandler {
	return &BotHandler{products: products, accounts: accounts, orders: orders}
}

type botUser struct {
	ID        int64  `json:"id" validate:"required"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type createOrderRequest struct {
	TelegramUser  botUser `json:"telegram_user" validate:"required"`
	ProductID     int64   `json:"product_id"`
	AccountID     *int64  `json:"account_id"`
	Type          *string `json:"type"`
	PriceOverride *int64  `json:"price_override"`
}

// Register mounts bot catalog and order routes on the bot-token group.
// orderLimiter rate-limits order creation to blunt spam.
func (h *BotHandler) Register(r fiber.Router, orderLimiter fiber.Handler) {
	r.Get("/products", h.Products)
	r.Get("/accounts", h.AvailableAccounts)
	r.Get("/catalog", h.Catalog)
	if orderLimiter != nil {
		r.Post("/orders", orderLimiter, h.CreateOrder)
	} else {
		r.Post("/orders", h.CreateOrder)
	}
	r.Get("/orders/:order_ref", h.GetOrder)
}

// Catalog handles GET /bot/catalog — available accounts grouped by type
// (product_name / type / price / available count).
func (h *BotHandler) Catalog(c *fiber.Ctx) error {
	items, err := h.accounts.ListCatalogForBot(c.Context())
	if err != nil {
		return respondError(c, err)
	}
	if items == nil {
		items = []model.BotCatalogItem{}
	}
	return ok(c, fiber.Map{"items": items})
}

// AvailableAccounts handles GET /bot/accounts — available accounts as a safe
// public list (label/username only, never secret credentials).
func (h *BotHandler) AvailableAccounts(c *fiber.Ctx) error {
	items, err := h.accounts.ListAvailableForBot(c.Context(), 200)
	if err != nil {
		return respondError(c, err)
	}
	if items == nil {
		items = []model.BotAccountListing{}
	}
	return ok(c, fiber.Map{"items": items})
}

// Products handles GET /bot/products — active, in-stock products with
// name/description/price for the catalog.
func (h *BotHandler) Products(c *fiber.Ctx) error {
	items, err := h.products.ListForBot(c.Context())
	if err != nil {
		return respondError(c, err)
	}
	if items == nil {
		items = []model.BotProductListing{}
	}
	return ok(c, fiber.Map{"items": items})
}

// CreateOrder handles POST /bot/orders — reserves an account and creates a
// PENDING order.
func (h *BotHandler) CreateOrder(c *fiber.Ctx) error {
	var req createOrderRequest
	if err := bindAndValidate(c, &req); err != nil {
		return respondError(c, err)
	}
	res, err := h.orders.CreateOrder(c.Context(), service.CreateOrderInput{
		TelegramID:    req.TelegramUser.ID,
		Username:      req.TelegramUser.Username,
		FirstName:     req.TelegramUser.FirstName,
		ProductID:     req.ProductID,
		AccountID:     req.AccountID,
		Type:          req.Type,
		PriceOverride: req.PriceOverride,
	})
	if err != nil {
		return respondError(c, err)
	}
	return created(c, fiber.Map{
		"order_ref":  res.Order.OrderRef,
		"amount":     res.Order.Amount,
		"status":     res.Order.Status,
		"expires_at": res.Order.ExpiresAt,
		"qr_string":  res.Payment.QRString,
		"qr_image":   res.Payment.QRImageURL,
		"payment_status": res.Payment.Status,
		"product": fiber.Map{
			"id":   res.Product.ID,
			"name": res.Product.Name,
		},
	})
}

// GetOrder handles GET /bot/orders/:order_ref — order status for bot polling.
func (h *BotHandler) GetOrder(c *fiber.Ctx) error {
	ref := c.Params("order_ref")
	o, err := h.orders.GetByRef(c.Context(), ref)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, o)
}
