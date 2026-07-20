package model

import "time"

// Payment tracks the gateway state for an order, independently of order state.
type Payment struct {
	ID             int64         `json:"id"`
	OrderID        int64         `json:"order_id"`
	Gateway        string        `json:"gateway"`
	GatewayTxnID   string        `json:"gateway_txn_id"`
	Status         PaymentStatus `json:"status"`
	GrossAmount    int64         `json:"gross_amount"`
	Acquirer       string        `json:"acquirer"`
	QRString       string        `json:"qr_string"`
	QRImageURL     string        `json:"qr_image_url"`
	ExpiresAt      *time.Time    `json:"expires_at,omitempty"`
	SettledAt      *time.Time    `json:"settled_at,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}
