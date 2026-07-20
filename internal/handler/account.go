package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kalia/store/internal/model"
	"github.com/kalia/store/internal/service"
	"github.com/kalia/store/pkg/apperr"
)

// AccountHandler serves inventory/account endpoints (admin, JWT-protected).
type AccountHandler struct {
	accounts *service.AccountService
}

// NewAccountHandler builds an account handler.
func NewAccountHandler(accounts *service.AccountService) *AccountHandler {
	return &AccountHandler{accounts: accounts}
}

// createAccountsRequest accepts either a single account or a bulk list.
type createAccountsRequest struct {
	// Single: {"credentials": {...}}
	Credentials model.Credentials `json:"credentials"`
	// Bulk: {"accounts": [{"credentials": {...}}, ...]}
	Accounts []struct {
		Credentials model.Credentials `json:"credentials"`
	} `json:"accounts"`
}

type updateAccountRequest struct {
	Credentials model.Credentials   `json:"credentials"`
	Status      model.AccountStatus `json:"status"`
}

// Register mounts account routes, guarded by mw (JWT).
func (h *AccountHandler) Register(r fiber.Router, mw fiber.Handler) {
	r.Get("/products/:id/accounts", mw, h.ListByProduct)
	r.Post("/products/:id/accounts", mw, h.Create)
	r.Get("/products/:id/inventory-summary", mw, h.Summary)

	r.Get("/accounts/:id", mw, h.Get)
	r.Put("/accounts/:id", mw, h.Update)
	r.Delete("/accounts/:id", mw, h.Delete)
}

// Create handles POST /products/:id/accounts (single or bulk).
func (h *AccountHandler) Create(c *fiber.Ctx) error {
	productID, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	var req createAccountsRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, apperr.BadRequest("invalid request body"))
	}

	var credsList []model.Credentials
	if len(req.Accounts) > 0 {
		for _, a := range req.Accounts {
			credsList = append(credsList, a.Credentials)
		}
	} else if req.Credentials != nil {
		credsList = append(credsList, req.Credentials)
	} else {
		return respondError(c, apperr.BadRequest("provide 'credentials' or a non-empty 'accounts' array"))
	}

	accounts, err := h.accounts.CreateAccounts(c.Context(), productID, credsList)
	if err != nil {
		return respondError(c, err)
	}
	return created(c, fiber.Map{"accounts": accounts, "count": len(accounts)})
}

// ListByProduct handles GET /products/:id/accounts?status=&limit=&offset=.
func (h *AccountHandler) ListByProduct(c *fiber.Ctx) error {
	productID, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	var status *model.AccountStatus
	if v := c.Query("status"); v != "" {
		st := model.AccountStatus(v)
		if !isValidAccountStatusStr(st) {
			return respondError(c, apperr.BadRequest("invalid status filter"))
		}
		status = &st
	}
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	items, total, err := h.accounts.ListByProduct(c.Context(), productID, status, limit, offset)
	if err != nil {
		return respondError(c, err)
	}
	if items == nil {
		items = []model.Account{}
	}
	return ok(c, fiber.Map{"items": items, "total": total, "limit": limit, "offset": offset})
}

// Get handles GET /accounts/:id.
func (h *AccountHandler) Get(c *fiber.Ctx) error {
	id, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	a, err := h.accounts.Get(c.Context(), id)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, a)
}

// Update handles PUT /accounts/:id.
func (h *AccountHandler) Update(c *fiber.Ctx) error {
	id, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	var req updateAccountRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, apperr.BadRequest("invalid request body"))
	}
	a, err := h.accounts.Update(c.Context(), id, req.Credentials, req.Status)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, a)
}

// Delete handles DELETE /accounts/:id.
func (h *AccountHandler) Delete(c *fiber.Ctx) error {
	id, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	if err := h.accounts.Delete(c.Context(), id); err != nil {
		return respondError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// Summary handles GET /products/:id/inventory-summary.
func (h *AccountHandler) Summary(c *fiber.Ctx) error {
	productID, err := idParam(c)
	if err != nil {
		return respondError(c, err)
	}
	sum, err := h.accounts.Summary(c.Context(), productID)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, sum)
}

func isValidAccountStatusStr(s model.AccountStatus) bool {
	switch s {
	case model.AccountAvailable, model.AccountReserved, model.AccountSold:
		return true
	}
	return false
}
