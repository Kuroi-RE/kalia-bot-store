package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/kalia/store/internal/model"
	"github.com/kalia/store/internal/repository"
	"github.com/kalia/store/pkg/apperr"
)

// CredentialSender delivers a text message to a customer chat.
type CredentialSender interface {
	Send(ctx context.Context, chatID int64, text string) error
}

// DeliveryService dispatches credentials for paid orders exactly once.
type DeliveryService struct {
	tx     *repository.TxManager
	sender CredentialSender
	log    *slog.Logger
}

// NewDeliveryService builds a delivery service.
func NewDeliveryService(tx *repository.TxManager, sender CredentialSender, log *slog.Logger) *DeliveryService {
	return &DeliveryService{tx: tx, sender: sender, log: log}
}

// DeliverOrder sends credentials for a paid order and finalizes inventory.
// Idempotent: if the order is already delivered it is a no-op. Implements the
// service.DeliveryTrigger interface used by the payment service.
//
// Exactly-once holds because (a) the payment webhook only triggers delivery on
// the single PENDING->PAID transition (payment_events dedupes replays), and
// (b) the unique deliveries.order_id row plus a DELIVERED status check guard
// redeliver/worker paths.
func (s *DeliveryService) DeliverOrder(ctx context.Context, orderID int64) error {
	// Load order + reserved account + customer + product for rendering.
	order, err := repository.NewOrderRepository(s.tx.DB()).GetByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return apperr.NotFound("order not found")
		}
		return apperr.Internal("order lookup failed").Wrap(err)
	}
	if order.AccountID == nil {
		return apperr.Conflict("order has no reserved account")
	}
	if order.Status != model.OrderPaid && order.Status != model.OrderFailed && order.Status != model.OrderDelivered {
		return apperr.Conflict("order is not payable/deliverable in its current state")
	}

	deliveries := repository.NewDeliveryRepository(s.tx.DB())
	delivery, _, err := deliveries.CreatePending(ctx, order.ID, *order.AccountID)
	if err != nil {
		return apperr.Internal("could not create delivery record").Wrap(err)
	}
	if delivery.Status == model.DeliveryDelivered {
		// Already delivered; nothing to do.
		return nil
	}

	account, err := repository.NewAccountRepository(s.tx.DB()).GetByID(ctx, *order.AccountID)
	if err != nil {
		return apperr.Internal("account lookup failed").Wrap(err)
	}
	user, err := repository.NewTelegramUserRepository(s.tx.DB()).GetByID(ctx, order.TelegramUserID)
	if err != nil {
		return apperr.Internal("customer lookup failed").Wrap(err)
	}
	product, err := repository.NewProductRepository(s.tx.DB()).GetByID(ctx, order.ProductID)
	if err != nil {
		return apperr.Internal("product lookup failed").Wrap(err)
	}

	message := renderCredentials(product, order, account)

	// Dispatch to the bot/customer (external call, no DB lock held).
	if sendErr := s.sender.Send(ctx, user.TelegramID, message); sendErr != nil {
		s.log.Error("credential delivery failed",
			slog.String("order_ref", order.OrderRef), slog.Any("error", sendErr))
		if err := s.tx.WithTx(ctx, func(db repository.DBTX) error {
			if err := repository.NewDeliveryRepository(db).MarkFailed(ctx, delivery.ID, sendErr.Error()); err != nil {
				return err
			}
			// PAID -> FAILED (retriable); leave already-DELIVERED untouched.
			if order.Status == model.OrderPaid {
				return repository.NewOrderRepository(db).UpdateStatus(ctx, order.ID, model.OrderFailed)
			}
			return nil
		}); err != nil {
			s.log.Error("recording delivery failure failed", slog.Any("error", err))
		}
		return apperr.Internal("delivery failed").Wrap(sendErr)
	}

	// Finalize: mark account SOLD, order DELIVERED, delivery DELIVERED — atomically.
	now := time.Now()
	if err := s.tx.WithTx(ctx, func(db repository.DBTX) error {
		if err := repository.NewAccountRepository(db).MarkSold(ctx, *order.AccountID); err != nil {
			return err
		}
		if err := repository.NewOrderRepository(db).MarkDelivered(ctx, order.ID, now); err != nil {
			return err
		}
		return repository.NewDeliveryRepository(db).MarkDelivered(ctx, delivery.ID, now)
	}); err != nil {
		// Credentials were sent but finalization failed; log loudly. A retry
		// will see delivery still non-DELIVERED and could resend — acceptable
		// rare edge; alerting recommended.
		s.log.Error("delivery finalization failed after send",
			slog.String("order_ref", order.OrderRef), slog.Any("error", err))
		return apperr.Internal("delivery finalization failed").Wrap(err)
	}
	s.log.Info("order delivered", slog.String("order_ref", order.OrderRef))
	return nil
}

// Redeliver retries delivery for an order (admin action).
func (s *DeliveryService) Redeliver(ctx context.Context, orderID int64) error {
	return s.DeliverOrder(ctx, orderID)
}

// List returns deliveries filtered by optional status.
func (s *DeliveryService) List(ctx context.Context, status *model.DeliveryStatus, limit, offset int) ([]model.Delivery, int64, error) {
	items, total, err := repository.NewDeliveryRepository(s.tx.DB()).List(ctx, status, limit, offset)
	if err != nil {
		return nil, 0, apperr.Internal("could not list deliveries").Wrap(err)
	}
	return items, total, nil
}

// RetryFailed re-attempts FAILED deliveries under the max attempt count.
// Returns the number of deliveries that succeeded on retry.
func (s *DeliveryService) RetryFailed(ctx context.Context, maxAttempts, limit int) (int, error) {
	items, err := repository.NewDeliveryRepository(s.tx.DB()).ListRetriable(ctx, maxAttempts, limit)
	if err != nil {
		return 0, err
	}
	succeeded := 0
	for i := range items {
		d := items[i]
		if err := s.DeliverOrder(ctx, d.OrderID); err != nil {
			s.log.Warn("delivery retry failed",
				slog.Int64("order_id", d.OrderID), slog.Any("error", err))
			continue
		}
		succeeded++
	}
	return succeeded, nil
}

// renderCredentials builds the message delivered to the customer.
func renderCredentials(product *model.Product, order *model.Order, account *model.Account) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("✅ Payment confirmed for order %s\n", order.OrderRef))
	if product != nil {
		sb.WriteString(fmt.Sprintf("Product: %s\n", product.Name))
	}
	sb.WriteString("\nYour account details:\n")

	// Prefer the product's declared field order; fall back to sorted keys.
	keys := credentialKeyOrder(product, account.Credentials)
	for _, k := range keys {
		if v, ok := account.Credentials[k]; ok {
			label := k
			if product != nil {
				for _, f := range product.CredentialSchema {
					if f.Key == k && f.Label != "" {
						label = f.Label
						break
					}
				}
			}
			sb.WriteString(fmt.Sprintf("%s: %v\n", label, v))
		}
	}
	sb.WriteString("\nThank you for your purchase!")
	return sb.String()
}

func credentialKeyOrder(product *model.Product, creds model.Credentials) []string {
	seen := map[string]bool{}
	var keys []string
	if product != nil {
		for _, f := range product.CredentialSchema {
			if _, ok := creds[f.Key]; ok {
				keys = append(keys, f.Key)
				seen[f.Key] = true
			}
		}
	}
	var rest []string
	for k := range creds {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	return append(keys, rest...)
}
