package repository

import (
	"context"
)

// PaymentEventRepository provides access to payment_events (idempotency + audit).
type PaymentEventRepository struct{ db DBTX }

// NewPaymentEventRepository builds a payment event repository over db.
func NewPaymentEventRepository(db DBTX) *PaymentEventRepository {
	return &PaymentEventRepository{db: db}
}

// InsertEventParams carries a notification event to record.
type InsertEventParams struct {
	OrderRef       string
	GatewayTxnID   string
	EventStatus    string
	StatusCode     string
	SignatureValid bool
	Payload        []byte
}

// Insert records a notification event. Returns inserted=false when the event is
// a duplicate (same gateway_txn_id + event_status), enabling idempotency.
func (r *PaymentEventRepository) Insert(ctx context.Context, p InsertEventParams) (inserted bool, id int64, err error) {
	const q = `
		INSERT INTO payment_events (order_ref, gateway_txn_id, event_status, status_code, signature_valid, payload)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (gateway_txn_id, event_status) DO NOTHING
		RETURNING id`
	err = r.db.QueryRow(ctx, q, p.OrderRef, p.GatewayTxnID, p.EventStatus, p.StatusCode, p.SignatureValid, p.Payload).Scan(&id)
	if IsNotFound(err) {
		// ON CONFLICT DO NOTHING returned no row -> duplicate.
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	return true, id, nil
}

// MarkProcessed flags an event as processed.
func (r *PaymentEventRepository) MarkProcessed(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `UPDATE payment_events SET processed = TRUE WHERE id = $1`, id)
	return err
}
