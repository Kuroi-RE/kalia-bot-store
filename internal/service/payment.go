package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/kalia/store/internal/model"
	"github.com/kalia/store/internal/payment"
	"github.com/kalia/store/internal/repository"
	"github.com/kalia/store/pkg/apperr"
)

// DeliveryTrigger is invoked when an order becomes PAID so credentials can be
// dispatched. Wired in Task 7; may be nil.
type DeliveryTrigger interface {
	DeliverOrder(ctx context.Context, orderID int64) error
}

// PaymentService handles gateway notifications and reconciliation.
type PaymentService struct {
	tx       *repository.TxManager
	gateway  payment.Gateway
	log      *slog.Logger
	delivery DeliveryTrigger
}

// NewPaymentService builds a payment service.
func NewPaymentService(tx *repository.TxManager, gateway payment.Gateway, log *slog.Logger) *PaymentService {
	return &PaymentService{tx: tx, gateway: gateway, log: log}
}

// SetDeliveryTrigger wires the delivery hook (Task 7).
func (s *PaymentService) SetDeliveryTrigger(d DeliveryTrigger) { s.delivery = d }

// Notification is the normalized gateway webhook payload.
type Notification struct {
	OrderRef          string
	TransactionID     string
	TransactionStatus string
	StatusCode        string
	GrossAmount       string
	FraudStatus       string
	SignatureKey      string
	Raw               []byte
}

// HandleNotification verifies, records (idempotently), and applies a gateway
// notification. Returns nil on success (including safe duplicate replays).
func (s *PaymentService) HandleNotification(ctx context.Context, n Notification) error {
	if n.OrderRef == "" {
		return apperr.BadRequest("missing order_id")
	}

	// 1. Verify signature first; reject tampered notifications.
	sigValid := s.gateway.VerifySignature(n.OrderRef, n.StatusCode, n.GrossAmount, n.SignatureKey)

	// 2. Record the event (idempotent). Duplicate (same txn+status) -> no-op.
	events := repository.NewPaymentEventRepository(s.tx.DB())
	inserted, eventID, err := events.Insert(ctx, repository.InsertEventParams{
		OrderRef:       n.OrderRef,
		GatewayTxnID:   n.TransactionID,
		EventStatus:    n.TransactionStatus,
		StatusCode:     n.StatusCode,
		SignatureValid: sigValid,
		Payload:        n.Raw,
	})
	if err != nil {
		return apperr.Internal("could not record payment event").Wrap(err)
	}
	if !inserted {
		// Duplicate webhook; already handled. Ack with success.
		s.log.Info("duplicate payment notification ignored",
			slog.String("order_ref", n.OrderRef), slog.String("status", n.TransactionStatus))
		return nil
	}

	if !sigValid {
		s.log.Warn("payment notification signature invalid",
			slog.String("order_ref", n.OrderRef))
		return apperr.Unauthorized("invalid signature")
	}

	// 3. Apply the state transition (forward-only) inside a transaction.
	newlyPaid, err := s.applyTransition(ctx, n)
	if err != nil {
		return err
	}

	// Mark event processed (best effort).
	if err := repository.NewPaymentEventRepository(s.tx.DB()).MarkProcessed(ctx, eventID); err != nil {
		s.log.Warn("could not mark payment event processed", slog.Any("error", err))
	}

	// 4. Trigger delivery outside the transition tx when newly paid.
	if newlyPaid && s.delivery != nil {
		if orderID, ok := s.orderIDForRef(ctx, n.OrderRef); ok {
			if derr := s.delivery.DeliverOrder(ctx, orderID); derr != nil {
				// Delivery has its own retry/FAILED handling; just log here.
				s.log.Error("delivery after settlement failed",
					slog.String("order_ref", n.OrderRef), slog.Any("error", derr))
			}
		}
	}
	return nil
}

// applyTransition maps the notification to order/payment state changes.
// Returns newlyPaid=true when this notification transitions the order to PAID.
func (s *PaymentService) applyTransition(ctx context.Context, n Notification) (bool, error) {
	var newlyPaid bool
	err := s.tx.WithTx(ctx, func(db repository.DBTX) error {
		orders := repository.NewOrderRepository(db)
		payments := repository.NewPaymentRepository(db)
		accounts := repository.NewAccountRepository(db)

		order, err := orders.GetByRefForUpdate(ctx, n.OrderRef)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return apperr.NotFound("order not found")
			}
			return apperr.Internal("order lookup failed").Wrap(err)
		}
		pay, err := payments.GetByOrderID(ctx, order.ID)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return apperr.Internal("payment lookup failed").Wrap(err)
		}

		mapped := mapTransactionStatus(n.TransactionStatus, n.FraudStatus)
		switch mapped {
		case model.PaymentSettlement:
			// Forward-only: only settle a PENDING order.
			if order.Status == model.OrderPending {
				if err := orders.MarkPaid(ctx, order.ID, time.Now()); err != nil {
					return apperr.Internal("could not mark order paid").Wrap(err)
				}
				if pay != nil {
					if err := payments.SetStatus(ctx, pay.ID, model.PaymentSettlement, n.TransactionID); err != nil {
						return apperr.Internal("could not update payment").Wrap(err)
					}
				}
				newlyPaid = true
			}
		case model.PaymentExpired:
			if order.Status == model.OrderPending {
				if err := orders.UpdateStatus(ctx, order.ID, model.OrderExpired); err != nil {
					return apperr.Internal("could not expire order").Wrap(err)
				}
				if pay != nil {
					_ = payments.SetStatus(ctx, pay.ID, model.PaymentExpired, n.TransactionID)
				}
				if order.AccountID != nil {
					_ = accounts.ReleaseReservation(ctx, *order.AccountID)
				}
			}
		case model.PaymentDenied, model.PaymentCancelled:
			if order.Status == model.OrderPending {
				if err := orders.UpdateStatus(ctx, order.ID, model.OrderCancelled); err != nil {
					return apperr.Internal("could not cancel order").Wrap(err)
				}
				if pay != nil {
					_ = payments.SetStatus(ctx, pay.ID, mapped, n.TransactionID)
				}
				if order.AccountID != nil {
					_ = accounts.ReleaseReservation(ctx, *order.AccountID)
				}
			}
		default:
			// pending / unknown -> no state change.
		}
		return nil
	})
	return newlyPaid, err
}

// GetPayment returns a payment by id (admin).
func (s *PaymentService) GetPayment(ctx context.Context, id int64) (*model.Payment, error) {
	p, err := repository.NewPaymentRepository(s.tx.DB()).GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("payment not found")
		}
		return nil, apperr.Internal("lookup failed").Wrap(err)
	}
	return p, nil
}

// GetPaymentByOrder returns the payment for an order (admin).
func (s *PaymentService) GetPaymentByOrder(ctx context.Context, orderID int64) (*model.Payment, error) {
	p, err := repository.NewPaymentRepository(s.tx.DB()).GetByOrderID(ctx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("payment not found")
		}
		return nil, apperr.Internal("lookup failed").Wrap(err)
	}
	return p, nil
}

func (s *PaymentService) orderIDForRef(ctx context.Context, ref string) (int64, bool) {
	order, err := repository.NewOrderRepository(s.tx.DB()).GetByRef(ctx, ref)
	if err != nil {
		return 0, false
	}
	return order.ID, true
}

// ReconcilePending polls the gateway for PENDING orders older than the cutoff
// and applies any resulting state transitions (safety-net for missed webhooks).
// Returns the number of orders whose state advanced.
func (s *PaymentService) ReconcilePending(ctx context.Context, minAge time.Duration, limit int) (int, error) {
	cutoff := time.Now().Add(-minAge)
	orders, err := repository.NewOrderRepository(s.tx.DB()).ListPendingOlderThan(ctx, cutoff, limit)
	if err != nil {
		return 0, err
	}
	advanced := 0
	for i := range orders {
		order := orders[i]
		st, err := s.gateway.GetStatus(ctx, order.OrderRef)
		if err != nil {
			s.log.Warn("reconcile: status query failed",
				slog.String("order_ref", order.OrderRef), slog.Any("error", err))
			continue
		}
		n := Notification{
			OrderRef:          order.OrderRef,
			TransactionID:     st.TransactionID,
			TransactionStatus: st.TransactionStatus,
			StatusCode:        st.StatusCode,
			GrossAmount:       st.GrossAmount,
			FraudStatus:       st.FraudStatus,
		}
		newlyPaid, err := s.applyTransition(ctx, n)
		if err != nil {
			s.log.Warn("reconcile: transition failed",
				slog.String("order_ref", order.OrderRef), slog.Any("error", err))
			continue
		}
		if newlyPaid {
			advanced++
			if s.delivery != nil {
				if derr := s.delivery.DeliverOrder(ctx, order.ID); derr != nil {
					s.log.Error("reconcile: delivery failed",
						slog.String("order_ref", order.OrderRef), slog.Any("error", derr))
				}
			}
		}
	}
	return advanced, nil
}

// ForceSettle manually settles an order (DEV ONLY — used by the fake payment
// mode so the bot flow can be exercised without a real gateway/webhook). It
// mirrors a settlement notification: applies the transition and triggers
// delivery. Returns whether the order newly became PAID.
func (s *PaymentService) ForceSettle(ctx context.Context, orderRef string) (bool, error) {
	order, err := repository.NewOrderRepository(s.tx.DB()).GetByRef(ctx, orderRef)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, apperr.NotFound("order not found")
		}
		return false, apperr.Internal("lookup failed").Wrap(err)
	}
	n := Notification{
		OrderRef:          orderRef,
		TransactionStatus: "settlement",
		StatusCode:        "200",
	}
	newlyPaid, err := s.applyTransition(ctx, n)
	if err != nil {
		return false, err
	}
	if newlyPaid && s.delivery != nil {
		if derr := s.delivery.DeliverOrder(ctx, order.ID); derr != nil {
			s.log.Error("force-settle delivery failed",
				slog.String("order_ref", orderRef), slog.Any("error", derr))
		}
	}
	return newlyPaid, nil
}

// mapTransactionStatus maps a Midtrans transaction_status to a payment status.
func mapTransactionStatus(txnStatus, fraudStatus string) model.PaymentStatus {
	switch txnStatus {
	case "settlement":
		return model.PaymentSettlement
	case "capture":
		// capture is settled only when fraud check accepts.
		if fraudStatus == "accept" || fraudStatus == "" {
			return model.PaymentSettlement
		}
		return model.PaymentPending
	case "expire":
		return model.PaymentExpired
	case "deny":
		return model.PaymentDenied
	case "cancel":
		return model.PaymentCancelled
	default:
		return model.PaymentPending
	}
}
