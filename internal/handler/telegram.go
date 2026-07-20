package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kalia/store/internal/model"
	"github.com/kalia/store/internal/service"
)

// TelegramHandler serves admin-managed menu/response CRUD and bot-read endpoints.
type TelegramHandler struct {
	tg *service.TelegramService
}

// NewTelegramHandler builds a telegram handler.
func NewTelegramHandler(tg *service.TelegramService) *TelegramHandler {
	return &TelegramHandler{tg: tg}
}

// nilSafeMenus returns a non-nil slice so JSON renders [] instead of null.
func nilSafeMenus(items []model.TelegramMenu) []model.TelegramMenu {
	if items == nil {
		return []model.TelegramMenu{}
	}
	return items
}

// nilSafeResponses returns a non-nil slice for JSON rendering.
func nilSafeResponses(items []model.TelegramResponse) []model.TelegramResponse {
	if items == nil {
		return []model.TelegramResponse{}
	}
	return items
}

type menuRequest struct {
	Command   string `json:"command" validate:"required"`
	Title     string `json:"title" validate:"max=200"`
	ReplyText string `json:"reply_text" validate:"max=4000"`
	IsEnabled *bool  `json:"is_enabled"`
	SortOrder int    `json:"sort_order"`
}

type responseRequest struct {
	Command   string `json:"command" validate:"required"`
	ReplyText string `json:"reply_text" validate:"max=4000"`
	IsEnabled *bool  `json:"is_enabled"`
}

type enabledRequest struct {
	IsEnabled *bool `json:"is_enabled" validate:"required"`
}

// RegisterAdmin mounts admin CRUD routes, guarded by mw (JWT).
func (h *TelegramHandler) RegisterAdmin(r fiber.Router, mw fiber.Handler) {
	menus := r.Group("/telegram/menus", mw)
	menus.Get("/", h.ListMenus)
	menus.Post("/", h.CreateMenu)
	menus.Put("/:id", h.UpdateMenu)
	menus.Patch("/:id/status", h.SetMenuStatus)
	menus.Delete("/:id", h.DeleteMenu)

	resp := r.Group("/telegram/responses", mw)
	resp.Get("/", h.ListResponses)
	resp.Post("/", h.CreateResponse)
	resp.Put("/:id", h.UpdateResponse)
	resp.Patch("/:id/status", h.SetResponseStatus)
	resp.Delete("/:id", h.DeleteResponse)
}

// RegisterBot mounts bot-read routes (bot-token-protected group).
func (h *TelegramHandler) RegisterBot(r fiber.Router) {
	r.Get("/menus", h.BotMenus)
	r.Get("/responses/:command", h.BotResponse)
}

// ---- Menus (admin) ----

func (h *TelegramHandler) CreateMenu(c *fiber.Ctx) error {
	var req menuRequest
	if err := bindAndValidate(c, &req); err != nil {
		return respondError(c, err)
	}
	enabled := true
	if req.IsEnabled != nil {
		enabled = *req.IsEnabled
	}
	m, err := h.tg.CreateMenu(c.Context(), service.MenuInput{
		Command:   req.Command,
		Title:     req.Title,
		ReplyText: req.ReplyText,
		IsEnabled: enabled,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		return respondError(c, err)
	}
	return created(c, m)
}

func (h *TelegramHandler) ListMenus(c *fiber.Ctx) error {
	items, err := h.tg.ListMenus(c.Context(), false)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, fiber.Map{"items": nilSafeMenus(items)})
}

func (h *TelegramHandler) UpdateMenu(c *fiber.Ctx) error {
	id, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	var req menuRequest
	if err := bindAndValidate(c, &req); err != nil {
		return respondError(c, err)
	}
	m, err := h.tg.UpdateMenu(c.Context(), id, service.MenuInput{
		Command:   req.Command,
		Title:     req.Title,
		ReplyText: req.ReplyText,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, m)
}

func (h *TelegramHandler) SetMenuStatus(c *fiber.Ctx) error {
	id, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	var req enabledRequest
	if err := bindAndValidate(c, &req); err != nil {
		return respondError(c, err)
	}
	m, err := h.tg.SetMenuEnabled(c.Context(), id, *req.IsEnabled)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, m)
}

func (h *TelegramHandler) DeleteMenu(c *fiber.Ctx) error {
	id, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	if err := h.tg.DeleteMenu(c.Context(), id); err != nil {
		return respondError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ---- Responses (admin) ----

func (h *TelegramHandler) CreateResponse(c *fiber.Ctx) error {
	var req responseRequest
	if err := bindAndValidate(c, &req); err != nil {
		return respondError(c, err)
	}
	enabled := true
	if req.IsEnabled != nil {
		enabled = *req.IsEnabled
	}
	r, err := h.tg.CreateResponse(c.Context(), service.ResponseInput{
		Command:   req.Command,
		ReplyText: req.ReplyText,
		IsEnabled: enabled,
	})
	if err != nil {
		return respondError(c, err)
	}
	return created(c, r)
}

func (h *TelegramHandler) ListResponses(c *fiber.Ctx) error {
	items, err := h.tg.ListResponses(c.Context(), false)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, fiber.Map{"items": nilSafeResponses(items)})
}

func (h *TelegramHandler) UpdateResponse(c *fiber.Ctx) error {
	id, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	var req responseRequest
	if err := bindAndValidate(c, &req); err != nil {
		return respondError(c, err)
	}
	r, err := h.tg.UpdateResponse(c.Context(), id, service.ResponseInput{
		Command:   req.Command,
		ReplyText: req.ReplyText,
	})
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, r)
}

func (h *TelegramHandler) SetResponseStatus(c *fiber.Ctx) error {
	id, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	var req enabledRequest
	if err := bindAndValidate(c, &req); err != nil {
		return respondError(c, err)
	}
	r, err := h.tg.SetResponseEnabled(c.Context(), id, *req.IsEnabled)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, r)
}

func (h *TelegramHandler) DeleteResponse(c *fiber.Ctx) error {
	id, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	if err := h.tg.DeleteResponse(c.Context(), id); err != nil {
		return respondError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ---- Bot-read ----

func (h *TelegramHandler) BotMenus(c *fiber.Ctx) error {
	items, err := h.tg.ListMenus(c.Context(), true)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, fiber.Map{"items": nilSafeMenus(items)})
}

func (h *TelegramHandler) BotResponse(c *fiber.Ctx) error {
	command := c.Params("command")
	r, err := h.tg.ResolveResponse(c.Context(), command)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, r)
}
