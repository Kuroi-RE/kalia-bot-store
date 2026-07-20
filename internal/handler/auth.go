package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kalia/store/internal/middleware"
	"github.com/kalia/store/internal/service"
	"github.com/kalia/store/pkg/token"
)

// AuthHandler serves authentication endpoints.
type AuthHandler struct {
	auth *service.AuthService
}

// NewAuthHandler builds an auth handler.
func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

type loginRequest struct {
	Username string `json:"username" validate:"required,min=3,max=64"`
	Password string `json:"password" validate:"required,min=6,max=128"`
}

// Register mounts auth routes. Public routes on api; protected under jwt.
// loginLimiter rate-limits the login endpoint to blunt brute force.
func (h *AuthHandler) Register(api fiber.Router, tm *token.Manager, loginLimiter fiber.Handler) {
	grp := api.Group("/auth")
	if loginLimiter != nil {
		grp.Post("/login", loginLimiter, h.Login)
	} else {
		grp.Post("/login", h.Login)
	}
	grp.Get("/me", middleware.JWTAuth(tm), h.Me)
}

// Login authenticates an admin and returns a JWT.
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req loginRequest
	if err := bindAndValidate(c, &req); err != nil {
		return respondError(c, err)
	}
	res, err := h.auth.Login(c.Context(), req.Username, req.Password)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, res)
}

// Me returns the currently authenticated admin.
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	adminID := middleware.AdminIDFromCtx(c)
	admin, err := h.auth.Me(c.Context(), adminID)
	if err != nil {
		return respondError(c, err)
	}
	return ok(c, admin)
}
