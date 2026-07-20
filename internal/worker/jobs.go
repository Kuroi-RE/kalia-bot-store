package worker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kalia/store/internal/config"
	"github.com/kalia/store/internal/service"
)

// Deps are the services the jobs operate on.
type Deps struct {
	Payment  *service.PaymentService
	Order    *service.OrderService
	Delivery *service.DeliveryService
	Cfg      *config.Config
	Log      *slog.Logger
}

// Register wires the standard background jobs onto the scheduler.
func Register(s *Scheduler, d Deps) {
	w := d.Cfg.Worker

	// Payment poller / reconciler: catch missed webhooks.
	s.Register("payment-reconciler", w.PollInterval, func(ctx context.Context) (string, error) {
		n, err := d.Payment.ReconcilePending(ctx, w.PollMinAge, w.BatchLimit)
		if err != nil {
			return "", err
		}
		if n == 0 {
			return "", nil
		}
		return fmt.Sprintf("reconciled %d order(s)", n), nil
	})

	// Expired-payment cleanup + reservation release.
	s.Register("expiry-cleanup", w.CleanupInterval, func(ctx context.Context) (string, error) {
		n, err := d.Order.CleanupExpired(ctx, w.BatchLimit)
		if err != nil {
			return "", err
		}
		if n == 0 {
			return "", nil
		}
		return fmt.Sprintf("expired %d order(s)", n), nil
	})

	// Failed-delivery retry.
	s.Register("delivery-retry", w.DeliveryRetryInterval, func(ctx context.Context) (string, error) {
		n, err := d.Delivery.RetryFailed(ctx, w.MaxDeliveryAttempts, w.BatchLimit)
		if err != nil {
			return "", err
		}
		if n == 0 {
			return "", nil
		}
		return fmt.Sprintf("redelivered %d order(s)", n), nil
	})
}
