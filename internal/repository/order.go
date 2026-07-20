package repository

import (
	"context"
	"time"

	"github.com/kalia/store/internal/model"
)

// OrderRepository provides access to the orders table.
type OrderRepository struct{ db DBTX }

// NewOrderRepository builds an order repository over db.
func NewOrderRepository(db DBTX) *OrderRepository { return &OrderRepository{db: db} }

const orderColumns = `id, order_ref, telegram_user_id, product_id, account_id, amount, status, expires_at, paid_at, delivered_at, created_at, updated_at`

func scanOrder(row interface{ Scan(dest ...any) error }) (*model.Order, error) {
	var o model.Order
	if err := row.Scan(&o.ID, &o.OrderRef, &o.TelegramUserID, &o.ProductID, &o.AccountID, &o.Amount, &o.Status, &o.ExpiresAt, &o.PaidAt, &o.DeliveredAt, &o.CreatedAt, &o.UpdatedAt); err != nil {
		return nil, err
	}
	return &o, nil
}

// Create inserts an order.
func (r *OrderRepository) Create(ctx context.Context, o *model.Order) (*model.Order, error) {
	status := o.Status
	if status == "" {
		status = model.OrderPending
	}
	const q = `
		INSERT INTO orders (order_ref, telegram_user_id, product_id, account_id, amount, status, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING ` + orderColumns
	return scanOrder(r.db.QueryRow(ctx, q, o.OrderRef, o.TelegramUserID, o.ProductID, o.AccountID, o.Amount, status, o.ExpiresAt))
}

// SetAccount links an account to an order.
func (r *OrderRepository) SetAccount(ctx context.Context, orderID, accountID int64) error {
	_, err := r.db.Exec(ctx, `UPDATE orders SET account_id = $2, updated_at = now() WHERE id = $1`, orderID, accountID)
	return err
}

// DeleteByProduct deletes all orders for a product, returning the count.
// Cascades to their payments and deliveries (FK ON DELETE CASCADE) and nulls
// any accounts.reserved_order_id references (FK ON DELETE SET NULL).
func (r *OrderRepository) DeleteByProduct(ctx context.Context, productID int64) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM orders WHERE product_id = $1`, productID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// GetByID fetches an order by id.
func (r *OrderRepository) GetByID(ctx context.Context, id int64) (*model.Order, error) {
	o, err := scanOrder(r.db.QueryRow(ctx, `SELECT `+orderColumns+` FROM orders WHERE id = $1`, id))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return o, err
}

// GetByRef fetches an order by its order_ref.
func (r *OrderRepository) GetByRef(ctx context.Context, ref string) (*model.Order, error) {
	o, err := scanOrder(r.db.QueryRow(ctx, `SELECT `+orderColumns+` FROM orders WHERE order_ref = $1`, ref))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return o, err
}

// GetByRefForUpdate locks the order row for a state transition.
func (r *OrderRepository) GetByRefForUpdate(ctx context.Context, ref string) (*model.Order, error) {
	o, err := scanOrder(r.db.QueryRow(ctx, `SELECT `+orderColumns+` FROM orders WHERE order_ref = $1 FOR UPDATE`, ref))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return o, err
}

// OrderListParams filters and paginates orders.
type OrderListParams struct {
	Status         *model.OrderStatus
	TelegramUserID *int64
	Limit          int
	Offset         int
}

// List returns orders matching params plus the total count.
func (r *OrderRepository) List(ctx context.Context, params OrderListParams) ([]model.Order, int64, error) {
	if params.Limit <= 0 || params.Limit > 200 {
		params.Limit = 50
	}
	var total int64
	countQ := `
		SELECT count(*) FROM orders
		WHERE ($1::order_status IS NULL OR status = $1)
		  AND ($2::bigint IS NULL OR telegram_user_id = $2)`
	if err := r.db.QueryRow(ctx, countQ, params.Status, params.TelegramUserID).Scan(&total); err != nil {
		return nil, 0, err
	}

	const q = `
		SELECT ` + orderColumns + `
		FROM orders
		WHERE ($1::order_status IS NULL OR status = $1)
		  AND ($2::bigint IS NULL OR telegram_user_id = $2)
		ORDER BY id DESC
		LIMIT $3 OFFSET $4`
	rows, err := r.db.Query(ctx, q, params.Status, params.TelegramUserID, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []model.Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *o)
	}
	return items, total, rows.Err()
}

// UpdateStatus sets a new status unconditionally (callers enforce transition rules).
func (r *OrderRepository) UpdateStatus(ctx context.Context, id int64, status model.OrderStatus) error {
	_, err := r.db.Exec(ctx, `UPDATE orders SET status = $2, updated_at = now() WHERE id = $1`, id, status)
	return err
}

// MarkPaid transitions an order to PAID and records paid_at.
func (r *OrderRepository) MarkPaid(ctx context.Context, id int64, at time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE orders SET status = 'PAID', paid_at = $2, updated_at = now() WHERE id = $1`, id, at)
	return err
}

// MarkDelivered transitions an order to DELIVERED and records delivered_at.
func (r *OrderRepository) MarkDelivered(ctx context.Context, id int64, at time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE orders SET status = 'DELIVERED', delivered_at = $2, updated_at = now() WHERE id = $1`, id, at)
	return err
}

// ListExpiredPending returns PENDING orders whose expires_at has passed.
func (r *OrderRepository) ListExpiredPending(ctx context.Context, now time.Time, limit int) ([]model.Order, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	const q = `
		SELECT ` + orderColumns + `
		FROM orders
		WHERE status = 'PENDING' AND expires_at IS NOT NULL AND expires_at < $1
		ORDER BY expires_at
		LIMIT $2`
	rows, err := r.db.Query(ctx, q, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *o)
	}
	return items, rows.Err()
}

// ListPendingOlderThan returns PENDING orders created before the cutoff (poller).
func (r *OrderRepository) ListPendingOlderThan(ctx context.Context, cutoff time.Time, limit int) ([]model.Order, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	const q = `
		SELECT ` + orderColumns + `
		FROM orders
		WHERE status = 'PENDING' AND created_at < $1
		ORDER BY created_at
		LIMIT $2`
	rows, err := r.db.Query(ctx, q, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *o)
	}
	return items, rows.Err()
}
