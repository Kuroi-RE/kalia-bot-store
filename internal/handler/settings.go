package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kalia/store/internal/model"
	"github.com/kalia/store/internal/service"
)

// SettingsHandler serves admin settings endpoints (JWT-protected).
type SettingsHandler struct {
	settings *service.SettingsService
}

// NewSettingsHandler builds a settings handler.
func NewSettingsHandler(settings *service.SettingsService) *SettingsHandler {
	return &SettingsHandler{settings: settings}
}

type setSettingRequest struct {
	Value string `json:"value"`
}

// Register mounts settings routes guarded by mw (JWT).
func (h *SettingsHandler) Register(r fiber.Router, mw fiber.Handler) {
	grp := r.Group("/settings", mw)
	grp.Get("/", h.List)
	grp.Get("/:key", h.Get)
	grp.Put("/:key", h.Set)
}

// List handles GET /settings.
func (h *SettingsHandler) List(c *fiber.Ctx) error {
	items, err := h.settings.List(c.Context())
	if err != nil {
		return respondError(c, err)
	}
	if items == nil {
		items = []model.Setting{}
	}
	return ok(c, fiber.Map{"items": items})
}

// Get handles GET /settings/:key.
func (h *SettingsHandler) Get(c *fiber.Ctx) error {
	setting, err := h.settings.Get(c.Context(), c.Params("key"))
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, setting)
}

// Set handles PUT /settings/:key.
func (h *SettingsHandler) Set(c *fiber.Ctx) error {
	var req setSettingRequest
	if err := bindAndValidate(c, &req); err != nil {
		return respondError(c, err)
	}
	setting, err := h.settings.Set(c.Context(), c.Params("key"), req.Value)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, setting)
}
