package repository

import (
	"context"
	"time"

	"github.com/kalia/store/internal/model"
)

// DeliveryRepository provides access to the deliveries table.
type DeliveryRepository struct{ db DBTX }

// NewDeliveryRepository builds a delivery repository over db.
func NewDeliveryRepository(db DBTX) *DeliveryRepository { return &DeliveryRepository{db: db} }

const deliveryColumns = `id, order_id, account_id, status, attempts, last_error, delivered_at, created_at, updated_at`

func scanDelivery(row interface{ Scan(dest ...any) error }) (*model.Delivery, error) {
	var d model.Delivery
	if err := row.Scan(&d.ID, &d.OrderID, &d.AccountID, &d.Status, &d.Attempts, &d.LastError, &d.DeliveredAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
		return nil, err
	}
	return &d, nil
}

// CreatePending inserts a PENDING delivery for an order. Returns
// (delivery, created=false) if one already exists (unique order_id), enabling
// exactly-once creation under duplicate settlement notifications.
func (r *DeliveryRepository) CreatePending(ctx context.Context, orderID, accountID int64) (*model.Delivery, bool, error) {
	const q = `
		INSERT INTO deliveries (order_id, account_id, status)
		VALUES ($1, $2, 'PENDING')
		ON CONFLICT (order_id) DO NOTHING
		RETURNING ` + deliveryColumns
	d, err := scanDelivery(r.db.QueryRow(ctx, q, orderID, accountID))
	if IsNotFound(err) {
		// Already exists.
		existing, gerr := r.GetByOrderID(ctx, orderID)
		if gerr != nil {
			return nil, false, gerr
		}
		return existing, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return d, true, nil
}

// GetByOrderID fetches a delivery by order id.
func (r *DeliveryRepository) GetByOrderID(ctx context.Context, orderID int64) (*model.Delivery, error) {
	d, err := scanDelivery(r.db.QueryRow(ctx, `SELECT `+deliveryColumns+` FROM deliveries WHERE order_id = $1`, orderID))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return d, err
}

// MarkDelivered flags a delivery as delivered.
func (r *DeliveryRepository) MarkDelivered(ctx context.Context, id int64, at time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE deliveries
		SET status = 'DELIVERED', delivered_at = $2, attempts = attempts + 1, last_error = '', updated_at = now()
		WHERE id = $1`, id, at)
	return err
}

// MarkFailed records a failed delivery attempt.
func (r *DeliveryRepository) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE deliveries
		SET status = 'FAILED', attempts = attempts + 1, last_error = $2, updated_at = now()
		WHERE id = $1`, id, errMsg)
	return err
}

// List returns deliveries filtered by optional status, most recent first.
func (r *DeliveryRepository) List(ctx context.Context, status *model.DeliveryStatus, limit, offset int) ([]model.Delivery, int64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var total int64
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM deliveries WHERE ($1::delivery_status IS NULL OR status = $1)`, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	const q = `
		SELECT ` + deliveryColumns + `
		FROM deliveries
		WHERE ($1::delivery_status IS NULL OR status = $1)
		ORDER BY id DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []model.Delivery
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *d)
	}
	return items, total, rows.Err()
}

// ListRetriable returns FAILED deliveries under the max attempt count (worker).
func (r *DeliveryRepository) ListRetriable(ctx context.Context, maxAttempts, limit int) ([]model.Delivery, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	const q = `
		SELECT ` + deliveryColumns + `
		FROM deliveries
		WHERE status = 'FAILED' AND attempts < $1
		ORDER BY updated_at
		LIMIT $2`
	rows, err := r.db.Query(ctx, q, maxAttempts, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.Delivery
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *d)
	}
	return items, rows.Err()
}
