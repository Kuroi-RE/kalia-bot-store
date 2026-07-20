// Package payment defines the payment gateway abstraction and its Midtrans
// implementation. The interface is intentionally thin (single-provider MVP)
// and exists mainly for testability/mocking.
package payment

import "context"

// ChargeRequest is a request to create a QRIS charge.
type ChargeRequest struct {
	OrderRef    string // used as Midtrans order_id (must be unique per merchant)
	GrossAmount int64  // IDR, integer
	Acquirer    string // gopay | shopeepay
	// CustomExpirySeconds sets the charge TTL; 0 uses the gateway default.
	CustomExpirySeconds int
}

// ChargeResult is the normalized outcome of a charge creation.
type ChargeResult struct {
	TransactionID     string // Midtrans transaction_id
	TransactionStatus string // e.g. "pending"
	QRString          string // raw QR payload (if provided)
	QRImageURL        string // URL to the QR PNG
	ExpiresAtRFC3339  string // gateway-provided expiry, when available
	Raw               []byte // raw JSON charge response for auditing
}

// StatusResult is the normalized outcome of a status query.
type StatusResult struct {
	TransactionID     string
	TransactionStatus string // pending | settlement | expire | deny | cancel
	StatusCode        string
	GrossAmount       string
	SignatureKey      string
	FraudStatus       string
	Raw               []byte
}

// Gateway abstracts the payment provider (Midtrans in MVP).
type Gateway interface {
	// CreateCharge creates a QRIS charge and returns QR details.
	CreateCharge(ctx context.Context, req ChargeRequest) (*ChargeResult, error)
	// GetStatus queries the current transaction status for reconciliation.
	GetStatus(ctx context.Context, orderRef string) (*StatusResult, error)
	// VerifySignature validates a webhook notification signature.
	VerifySignature(orderID, statusCode, grossAmount, signatureKey string) bool
	// Name returns the provider name (e.g. "midtrans").
	Name() string
}
