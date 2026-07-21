package payment

import (
	"context"
	"fmt"
)

// TemanQRIS is a "dynamic QRIS" gateway that converts a merchant's static QRIS
// into a per-order dynamic QRIS (amount-locked). Funds settle directly into the
// merchant's account, so there is no automatic settlement signal — payment is
// confirmed manually by an admin (see PaymentService.ConfirmPayment and the
// dashboard Confirmations page).
type TemanQRIS struct {
	staticPayload string
}

// NewTemanQRIS builds a TemanQRIS gateway from the merchant's static QRIS.
func NewTemanQRIS(staticPayload string) *TemanQRIS {
	return &TemanQRIS{staticPayload: staticPayload}
}

// Name returns the provider name.
func (t *TemanQRIS) Name() string { return "temanqris" }

// RequiresManualConfirmation reports that settlement is admin-confirmed.
func (t *TemanQRIS) RequiresManualConfirmation() bool { return true }

// CreateCharge produces a dynamic QRIS string for the order amount. The QR
// image is rendered by the client (bot) from QRString; no external image URL is
// used. There is no gateway-side transaction, so TransactionID is the order ref.
func (t *TemanQRIS) CreateCharge(_ context.Context, req ChargeRequest) (*ChargeResult, error) {
	if t.staticPayload == "" {
		return nil, fmt.Errorf("temanqris: QRIS_STATIC_PAYLOAD is not configured")
	}
	dynamic, err := StaticToDynamicQRIS(t.staticPayload, req.GrossAmount)
	if err != nil {
		return nil, fmt.Errorf("temanqris: %w", err)
	}
	return &ChargeResult{
		TransactionID:     req.OrderRef,
		TransactionStatus: "pending",
		QRString:          dynamic,
		QRImageURL:        "", // client renders from QRString
		Raw:               []byte(`{"provider":"temanqris","transaction_status":"pending"}`),
	}, nil
}

// GetStatus always reports pending: there is no acquirer to query. Settlement
// happens via manual admin confirmation, so the poller never auto-settles these.
func (t *TemanQRIS) GetStatus(_ context.Context, _ string) (*StatusResult, error) {
	return &StatusResult{TransactionStatus: "pending"}, nil
}

// VerifySignature is unused (no webhook); always false so no auto-settle path
// can be triggered for this provider.
func (t *TemanQRIS) VerifySignature(_, _, _, _ string) bool { return false }
