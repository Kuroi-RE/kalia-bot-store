package service

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/kalia/store/internal/inventory"
	"github.com/kalia/store/internal/model"
	"github.com/kalia/store/internal/payment"
	"github.com/kalia/store/internal/repository"
	"github.com/kalia/store/pkg/apperr"
)

// OrderService coordinates order creation with inventory reservation and
// payment charge creation.
type OrderService struct {
	tx             *repository.TxManager
	gateway        payment.Gateway
	acquirer       string
	log            *slog.Logger
	reservationTTL time.Duration
	paymentTTL     time.Duration
}

// NewOrderService builds an order service.
func NewOrderService(tx *repository.TxManager, gateway payment.Gateway, acquirer string, reservationTTL, paymentTTL time.Duration, log *slog.Logger) *OrderService {
	return &OrderService{
		tx:             tx,
		gateway:        gateway,
		acquirer:       acquirer,
		log:            log,
		reservationTTL: reservationTTL,
		paymentTTL:     paymentTTL,
	}
}

// CreateOrderInput carries the data needed to place an order from the bot.
// Provide AccountID to buy a specific account (e.g. a chosen Twitter username),
// or ProductID to reserve any available account of a product.
type CreateOrderInput struct {
	TelegramID    int64
	Username      string
	FirstName     string
	ProductID     int64
	AccountID     *int64 // optional: reserve this specific account
	PriceOverride *int64 // optional per-order price
}

// OrderResult is the outcome of creating an order.
type OrderResult struct {
	Order   *model.Order   `json:"order"`
	Account *model.Account `json:"-"` // credentials never returned at order time
	Product *model.Product `json:"product"`
	Payment *model.Payment `json:"payment"`
}

// CreateOrder reserves an available account and creates a PENDING order in a
// single transaction. If no stock is available the whole thing rolls back and
// an out-of-stock error is returned (we never take payment for unfulfillable
// stock). Payment charge creation happens after commit (Task 5).
func (s *OrderService) CreateOrder(ctx context.Context, in CreateOrderInput) (*OrderResult, error) {
	if in.TelegramID == 0 {
		return nil, apperr.BadRequest("telegram_id is required")
	}
	if in.AccountID == nil && in.ProductID == 0 {
		return nil, apperr.BadRequest("either account_id or product_id is required")
	}
	if in.PriceOverride != nil && *in.PriceOverride < 0 {
		return nil, apperr.BadRequest("price override must be >= 0")
	}

	var result OrderResult
	now := time.Now()
	reservedUntil := now.Add(s.reservationTTL)
	expiresAt := now.Add(s.paymentTTL)

	err := s.tx.WithTx(ctx, func(db repository.DBTX) error {
		products := repository.NewProductRepository(db)
		users := repository.NewTelegramUserRepository(db)
		orders := repository.NewOrderRepository(db)
		accounts := repository.NewAccountRepository(db)

		// Resolve the product. When a specific account is requested, derive the
		// product from it; otherwise use the provided product id.
		var product *model.Product
		if in.AccountID != nil {
			acc, err := accounts.GetByID(ctx, *in.AccountID)
			if err != nil {
				if errors.Is(err, repository.ErrNotFound) {
					return apperr.NotFound("account not found")
				}
				return apperr.Internal("account lookup failed").Wrap(err)
			}
			if acc.Status != model.AccountAvailable {
				return apperr.Conflict("that account is no longer available")
			}
			product, err = products.GetByID(ctx, acc.ProductID)
			if err != nil {
				return apperr.Internal("product lookup failed").Wrap(err)
			}
		} else {
			p, err := products.GetByID(ctx, in.ProductID)
			if err != nil {
				if errors.Is(err, repository.ErrNotFound) {
					return apperr.NotFound("product not found")
				}
				return apperr.Internal("product lookup failed").Wrap(err)
			}
			product = p
		}
		if !product.IsActive {
			return apperr.Conflict("product is not available")
		}

		// Record/refresh the customer.
		user, err := users.Upsert(ctx, &model.TelegramUser{
			TelegramID: in.TelegramID,
			Username:   in.Username,
			FirstName:  in.FirstName,
		})
		if err != nil {
			return apperr.Internal("could not record customer").Wrap(err)
		}

		amount := product.BasePrice
		if in.PriceOverride != nil {
			amount = *in.PriceOverride
		}

		// Insert the order first (account linked after reservation) so we have
		// an order id to stamp on the reserved account.
		orderRef, err := generateOrderRef()
		if err != nil {
			return apperr.Internal("could not generate order reference").Wrap(err)
		}
		order, err := orders.Create(ctx, &model.Order{
			OrderRef:       orderRef,
			TelegramUserID: user.ID,
			ProductID:      product.ID,
			Amount:         amount,
			Status:         model.OrderPending,
			ExpiresAt:      &expiresAt,
		})
		if err != nil {
			if repository.IsUniqueViolation(err) {
				return apperr.Conflict("order reference collision, please retry")
			}
			return apperr.Internal("could not create order").Wrap(err)
		}

		// Reserve the account: a specific one if requested, else any available.
		var account *model.Account
		if in.AccountID != nil {
			account, err = accounts.ReserveSpecificAvailable(ctx, *in.AccountID, &order.ID, reservedUntil)
		} else {
			account, err = inventory.ReserveInTx(ctx, db, product.ID, &order.ID, reservedUntil)
		}
		if err != nil {
			if errors.Is(err, repository.ErrNoStock) {
				if in.AccountID != nil {
					return apperr.Conflict("that account was just taken by someone else")
				}
				return apperr.Conflict("out of stock for this product")
			}
			return apperr.Internal("could not reserve account").Wrap(err)
		}

		// Link the reserved account to the order.
		if err := orders.SetAccount(ctx, order.ID, account.ID); err != nil {
			return apperr.Internal("could not link account to order").Wrap(err)
		}
		order.AccountID = &account.ID

		result.Order = order
		result.Account = account
		result.Product = product
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Create the payment charge OUTSIDE the reservation transaction (an external
	// call must never hold DB locks). On failure we release the reservation and
	// cancel the order so no customer is charged for unfulfillable stock.
	charge, chargeErr := s.gateway.CreateCharge(ctx, payment.ChargeRequest{
		OrderRef:            result.Order.OrderRef,
		GrossAmount:         result.Order.Amount,
		Acquirer:            s.acquirer,
		CustomExpirySeconds: int(s.paymentTTL.Seconds()),
	})
	if chargeErr != nil {
		s.log.Error("charge creation failed; rolling back order",
			slog.String("order_ref", result.Order.OrderRef), slog.Any("error", chargeErr))
		s.cancelAndRelease(ctx, result.Order.ID, result.Account.ID)
		return nil, apperr.Internal("payment initialization failed").Wrap(chargeErr)
	}

	// Persist the payment record (PENDING) with QR details.
	pay, err := repository.NewPaymentRepository(s.tx.DB()).Create(ctx, repository.CreatePaymentParams{
		OrderID:           result.Order.ID,
		Gateway:           s.gateway.Name(),
		GatewayTxnID:      charge.TransactionID,
		Status:            model.PaymentPending,
		GrossAmount:       result.Order.Amount,
		Acquirer:          s.acquirer,
		QRString:          charge.QRString,
		QRImageURL:        charge.QRImageURL,
		ExpiresAt:         result.Order.ExpiresAt,
		RawChargeResponse: charge.Raw,
	})
	if err != nil {
		s.log.Error("persisting payment failed; rolling back order",
			slog.String("order_ref", result.Order.OrderRef), slog.Any("error", err))
		s.cancelAndRelease(ctx, result.Order.ID, result.Account.ID)
		return nil, apperr.Internal("could not persist payment").Wrap(err)
	}
	result.Payment = pay
	return &result, nil
}

// cancelAndRelease best-effort cancels an order and returns its account to stock.
func (s *OrderService) cancelAndRelease(ctx context.Context, orderID, accountID int64) {
	err := s.tx.WithTx(ctx, func(db repository.DBTX) error {
		if err := repository.NewOrderRepository(db).UpdateStatus(ctx, orderID, model.OrderCancelled); err != nil {
			return err
		}
		return repository.NewAccountRepository(db).ReleaseReservation(ctx, accountID)
	})
	if err != nil {
		s.log.Error("cleanup after charge failure failed",
			slog.Int64("order_id", orderID), slog.Any("error", err))
	}
}

// GetByRef returns an order by its reference (bot polling / status).
func (s *OrderService) GetByRef(ctx context.Context, ref string) (*model.Order, error) {
	o, err := repository.NewOrderRepository(s.tx.DB()).GetByRef(ctx, ref)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("order not found")
		}
		return nil, apperr.Internal("lookup failed").Wrap(err)
	}
	return o, nil
}

// Get returns an order by internal id (admin).
func (s *OrderService) Get(ctx context.Context, id int64) (*model.Order, error) {
	o, err := repository.NewOrderRepository(s.tx.DB()).GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("order not found")
		}
		return nil, apperr.Internal("lookup failed").Wrap(err)
	}
	return o, nil
}

// List returns orders with optional status filter and pagination (admin).
func (s *OrderService) List(ctx context.Context, status *model.OrderStatus, limit, offset int) ([]model.Order, int64, error) {
	items, total, err := repository.NewOrderRepository(s.tx.DB()).List(ctx, repository.OrderListParams{
		Status: status,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, apperr.Internal("could not list orders").Wrap(err)
	}
	return items, total, nil
}

// Cancel cancels a PENDING order and releases its reserved account (admin).
func (s *OrderService) Cancel(ctx context.Context, id int64) (*model.Order, error) {
	var result *model.Order
	err := s.tx.WithTx(ctx, func(db repository.DBTX) error {
		orders := repository.NewOrderRepository(db)
		order, err := orders.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return apperr.NotFound("order not found")
			}
			return apperr.Internal("lookup failed").Wrap(err)
		}
		if order.Status != model.OrderPending {
			return apperr.Conflict("only PENDING orders can be cancelled")
		}
		if err := orders.UpdateStatus(ctx, order.ID, model.OrderCancelled); err != nil {
			return apperr.Internal("could not cancel order").Wrap(err)
		}
		if pay, perr := repository.NewPaymentRepository(db).GetByOrderID(ctx, order.ID); perr == nil {
			_ = repository.NewPaymentRepository(db).SetStatus(ctx, pay.ID, model.PaymentCancelled, "")
		}
		if order.AccountID != nil {
			if err := repository.NewAccountRepository(db).ReleaseReservation(ctx, *order.AccountID); err != nil {
				return apperr.Internal("could not release reservation").Wrap(err)
			}
		}
		order.Status = model.OrderCancelled
		result = order
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// CleanupExpired marks overdue PENDING orders EXPIRED, expires their payment,
// and releases the reserved account. Returns the number of orders expired.
func (s *OrderService) CleanupExpired(ctx context.Context, limit int) (int, error) {
	now := time.Now()
	orders, err := repository.NewOrderRepository(s.tx.DB()).ListExpiredPending(ctx, now, limit)
	if err != nil {
		return 0, err
	}
	count := 0
	for i := range orders {
		order := orders[i]
		err := s.tx.WithTx(ctx, func(db repository.DBTX) error {
			or := repository.NewOrderRepository(db)
			// Re-check under no lock is fine; UpdateStatus is idempotent forward.
			if err := or.UpdateStatus(ctx, order.ID, model.OrderExpired); err != nil {
				return err
			}
			if pay, perr := repository.NewPaymentRepository(db).GetByOrderID(ctx, order.ID); perr == nil {
				_ = repository.NewPaymentRepository(db).SetStatus(ctx, pay.ID, model.PaymentExpired, "")
			}
			if order.AccountID != nil {
				return repository.NewAccountRepository(db).ReleaseReservation(ctx, *order.AccountID)
			}
			return nil
		})
		if err != nil {
			s.log.Warn("cleanup: expiring order failed",
				slog.String("order_ref", order.OrderRef), slog.Any("error", err))
			continue
		}
		count++
	}
	return count, nil
}

// generateOrderRef builds a unique, gateway-safe order reference (<=30 chars).
// Format: KAL-<base36 seconds>-<6 random base32 chars>.
func generateOrderRef() (string, error) {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ" // Crockford-ish, no ambiguous chars
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, b := range buf {
		sb.WriteByte(alphabet[int(b)%len(alphabet)])
	}
	ref := "KAL-" + strconv.FormatInt(time.Now().Unix(), 36) + "-" + sb.String()
	return strings.ToUpper(ref), nil
}
