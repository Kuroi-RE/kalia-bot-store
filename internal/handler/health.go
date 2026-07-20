package handler

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthHandler serves liveness and readiness probes.
type HealthHandler struct {
	pool *pgxpool.Pool
}

// NewHealthHandler builds a health handler. pool may be nil (liveness still works).
func NewHealthHandler(pool *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{pool: pool}
}

// Register mounts health routes.
func (h *HealthHandler) Register(app *fiber.App) {
	grp := app.Group("/health")
	grp.Get("/live", h.Live)
	grp.Get("/ready", h.Ready)
}

// Live reports process liveness.
func (h *HealthHandler) Live(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

// Ready reports readiness including dependency checks (database).
func (h *HealthHandler) Ready(c *fiber.Ctx) error {
	checks := fiber.Map{}
	healthy := true

	if h.pool != nil {
		ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
		defer cancel()
		if err := h.pool.Ping(ctx); err != nil {
			checks["database"] = "down"
			healthy = false
		} else {
			checks["database"] = "up"
		}
	} else {
		checks["database"] = "not_configured"
	}

	status := fiber.StatusOK
	overall := "ready"
	if !healthy {
		status = fiber.StatusServiceUnavailable
		overall = "not_ready"
	}
	return c.Status(status).JSON(fiber.Map{"status": overall, "checks": checks})
}
