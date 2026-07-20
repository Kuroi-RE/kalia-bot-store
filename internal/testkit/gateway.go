// Package testkit provides in-memory fakes and helpers for integration
// verification (used by tests and the cmd/verify harness). It is not used by
// production binaries.
package testkit

import (
	"context"
	"net/url"
	"sync"

	"github.com/kalia/store/internal/payment"
)

// FakeGateway is a controllable payment.Gateway for tests / verification.
type FakeGateway struct {
	mu sync.Mutex

	ServerKey string

	ChargeCalls int
	StatusCalls int

	ChargeErr   error
	NextTxnID   string
	QRImageURL  string
	QRString    string
	statusByRef map[string]*payment.StatusResult
}

// NewFakeGateway builds a fake gateway with sane defaults.
func NewFakeGateway(serverKey string) *FakeGateway {
	return &FakeGateway{
		ServerKey:   serverKey,
		NextTxnID:   "txn-fake-1",
		QRString:    "00020101021226FAKEQRIS5920Kalia Store Dev Mode",
		statusByRef: map[string]*payment.StatusResult{},
	}
}

// Name returns the provider name.
func (m *FakeGateway) Name() string { return "midtrans" }

// CreateCharge returns a fake pending charge. The QR image is a real,
// renderable PNG (generated from the fake QR payload) so the bot can actually
// display it during local testing — it just isn't payable (use the dev settle
// endpoint to simulate payment).
func (m *FakeGateway) CreateCharge(_ context.Context, _ payment.ChargeRequest) (*payment.ChargeResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ChargeCalls++
	if m.ChargeErr != nil {
		return nil, m.ChargeErr
	}
	img := m.QRImageURL
	if img == "" {
		img = "https://api.qrserver.com/v1/create-qr-code/?size=320x320&data=" + url.QueryEscape(m.QRString)
	}
	return &payment.ChargeResult{
		TransactionID:     m.NextTxnID,
		TransactionStatus: "pending",
		QRString:          m.QRString,
		QRImageURL:        img,
		Raw:               []byte(`{"transaction_status":"pending"}`),
	}, nil
}

// GetStatus returns a configured status or pending by default.
func (m *FakeGateway) GetStatus(_ context.Context, orderRef string) (*payment.StatusResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StatusCalls++
	if r, ok := m.statusByRef[orderRef]; ok {
		return r, nil
	}
	return &payment.StatusResult{TransactionStatus: "pending", StatusCode: "201"}, nil
}

// VerifySignature validates against the configured server key.
func (m *FakeGateway) VerifySignature(orderID, statusCode, grossAmount, signatureKey string) bool {
	return payment.ComputeSignature(orderID, statusCode, grossAmount, m.ServerKey) == signatureKey
}

// SetStatus configures the status returned for an order ref (reconciliation).
func (m *FakeGateway) SetStatus(ref string, r *payment.StatusResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusByRef[ref] = r
}
