package repository

import (
	"context"
	"time"

	"github.com/kalia/store/internal/model"
)

// PaymentRepository provides access to the payments table.
type PaymentRepository struct{ db DBTX }

// NewPaymentRepository builds a payment repository over db.
func NewPaymentRepository(db DBTX) *PaymentRepository { return &PaymentRepository{db: db} }

const paymentColumns = `id, order_id, gateway, gateway_txn_id, status, gross_amount, acquirer, qr_string, qr_image_url, expires_at, settled_at, created_at, updated_at`

func scanPayment(row interface{ Scan(dest ...any) error }) (*model.Payment, error) {
	var p model.Payment
	if err := row.Scan(&p.ID, &p.OrderID, &p.Gateway, &p.GatewayTxnID, &p.Status, &p.GrossAmount, &p.Acquirer, &p.QRString, &p.QRImageURL, &p.ExpiresAt, &p.SettledAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

// CreateParams carries fields for inserting a payment.
type CreatePaymentParams struct {
	OrderID           int64
	Gateway           string
	GatewayTxnID      string
	Status            model.PaymentStatus
	GrossAmount       int64
	Acquirer          string
	QRString          string
	QRImageURL        string
	ExpiresAt         *time.Time
	RawChargeResponse []byte
}

// Create inserts a payment row.
func (r *PaymentRepository) Create(ctx context.Context, p CreatePaymentParams) (*model.Payment, error) {
	if p.Gateway == "" {
		p.Gateway = "midtrans"
	}
	if p.Status == "" {
		p.Status = model.PaymentPending
	}
	const q = `
		INSERT INTO payments (order_id, gateway, gateway_txn_id, status, gross_amount, acquirer, qr_string, qr_image_url, expires_at, raw_charge_response)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING ` + paymentColumns
	return scanPayment(r.db.QueryRow(ctx, q,
		p.OrderID, p.Gateway, p.GatewayTxnID, p.Status, p.GrossAmount, p.Acquirer, p.QRString, p.QRImageURL, p.ExpiresAt, p.RawChargeResponse))
}

// GetByID fetches a payment by id.
func (r *PaymentRepository) GetByID(ctx context.Context, id int64) (*model.Payment, error) {
	p, err := scanPayment(r.db.QueryRow(ctx, `SELECT `+paymentColumns+` FROM payments WHERE id = $1`, id))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return p, err
}

// GetByOrderID fetches the payment for an order.
func (r *PaymentRepository) GetByOrderID(ctx context.Context, orderID int64) (*model.Payment, error) {
	p, err := scanPayment(r.db.QueryRow(ctx, `SELECT `+paymentColumns+` FROM payments WHERE order_id = $1`, orderID))
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	return p, err
}

// SetStatus updates a payment's status and (optionally) gateway txn id.
func (r *PaymentRepository) SetStatus(ctx context.Context, id int64, status model.PaymentStatus, gatewayTxnID string) error {
	settled := status == model.PaymentSettlement
	const q = `
		UPDATE payments
		SET status = $2,
		    gateway_txn_id = CASE WHEN $3 <> '' THEN $3 ELSE gateway_txn_id END,
		    settled_at = CASE WHEN $4 THEN now() ELSE settled_at END,
		    updated_at = now()
		WHERE id = $1`
	_, err := r.db.Exec(ctx, q, id, status, gatewayTxnID, settled)
	return err
}
