package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/kalia/store/internal/model"
	"github.com/kalia/store/internal/service"
	"github.com/kalia/store/pkg/apperr"
)

// ProductHandler serves product catalog endpoints (admin, JWT-protected).
type ProductHandler struct {
	products *service.ProductService
}

// NewProductHandler builds a product handler.
func NewProductHandler(products *service.ProductService) *ProductHandler {
	return &ProductHandler{products: products}
}

type productRequest struct {
	Name             string                 `json:"name" validate:"required,min=1,max=200"`
	Description      string                 `json:"description" validate:"max=2000"`
	BasePrice        int64                  `json:"base_price" validate:"gte=0"`
	IsActive         *bool                  `json:"is_active"`
	CredentialSchema model.CredentialSchema `json:"credential_schema" validate:"dive"`
}

type statusRequest struct {
	IsActive *bool `json:"is_active" validate:"required"`
}

// Register mounts product routes under the given router, guarded by mw (JWT).
func (h *ProductHandler) Register(r fiber.Router, mw fiber.Handler) {
	grp := r.Group("/products", mw)
	grp.Get("/", h.List)
	grp.Post("/", h.Create)
	grp.Get("/:id", h.Get)
	grp.Put("/:id", h.Update)
	grp.Patch("/:id/status", h.SetStatus)
	grp.Delete("/:id", h.Delete)
}

// Create handles POST /products.
func (h *ProductHandler) Create(c *fiber.Ctx) error {
	var req productRequest
	if err := bindAndValidate(c, &req); err != nil {
		return respondError(c, err)
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	p, err := h.products.Create(c.Context(), service.CreateProductInput{
		Name:             req.Name,
		Description:      req.Description,
		BasePrice:        req.BasePrice,
		IsActive:         active,
		CredentialSchema: req.CredentialSchema,
	})
	if err != nil {
		return respondError(c, err)
	}
	return created(c, p)
}

// List handles GET /products?is_active=&limit=&offset=.
func (h *ProductHandler) List(c *fiber.Ctx) error {
	var isActive *bool
	if v := c.Query("is_active"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return respondError(c, apperr.BadRequest("is_active must be true or false"))
		}
		isActive = &b
	}
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	items, total, err := h.products.List(c.Context(), isActive, limit, offset)
	if err != nil {
		return respondError(c, err)
	}
	if items == nil {
		items = []model.Product{}
	}
	return ok(c, fiber.Map{"items": items, "total": total, "limit": limit, "offset": offset})
}

// Get handles GET /products/:id.
func (h *ProductHandler) Get(c *fiber.Ctx) error {
	id, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	p, err := h.products.Get(c.Context(), id)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, p)
}

// Update handles PUT /products/:id.
func (h *ProductHandler) Update(c *fiber.Ctx) error {
	id, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	var req productRequest
	if err := bindAndValidate(c, &req); err != nil {
		return respondError(c, err)
	}
	p, err := h.products.Update(c.Context(), id, service.UpdateProductInput{
		Name:             req.Name,
		Description:      req.Description,
		BasePrice:        req.BasePrice,
		CredentialSchema: req.CredentialSchema,
	})
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, p)
}

// SetStatus handles PATCH /products/:id/status.
func (h *ProductHandler) SetStatus(c *fiber.Ctx) error {
	id, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	var req statusRequest
	if err := bindAndValidate(c, &req); err != nil {
		return respondError(c, err)
	}
	p, err := h.products.SetActive(c.Context(), id, *req.IsActive)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, p)
}

// Delete handles DELETE /products/:id?force=true.
func (h *ProductHandler) Delete(c *fiber.Ctx) error {
	id, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	force := c.Query("force") == "true"
	if err := h.products.Delete(c.Context(), id, force); err != nil {
		return respondError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
