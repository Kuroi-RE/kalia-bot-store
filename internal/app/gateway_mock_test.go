package app

import (
	"context"
	"sync"

	"github.com/kalia/store/internal/payment"
)

// testServerKey is the shared Midtrans server key used to compute/verify
// signatures in tests.
const testServerKey = "test-server-key"

// mockGateway is a controllable payment.Gateway for tests.
type mockGateway struct {
	mu sync.Mutex

	chargeCalls int
	statusCalls int

	// Overridable behavior.
	chargeErr   error
	nextTxnID   string
	qrImageURL  string
	qrString    string
	statusByRef map[string]*payment.StatusResult
}

func newMockGateway() *mockGateway {
	return &mockGateway{
		nextTxnID:   "txn-test-1",
		qrImageURL:  "https://api.sandbox.midtrans.com/v2/qris/test/qr-code",
		qrString:    "00020101021226...QRIS",
		statusByRef: map[string]*payment.StatusResult{},
	}
}

func (m *mockGateway) Name() string { return "midtrans" }

func (m *mockGateway) CreateCharge(_ context.Context, req payment.ChargeRequest) (*payment.ChargeResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chargeCalls++
	if m.chargeErr != nil {
		return nil, m.chargeErr
	}
	return &payment.ChargeResult{
		TransactionID:     m.nextTxnID,
		TransactionStatus: "pending",
		QRString:          m.qrString,
		QRImageURL:        m.qrImageURL,
		Raw:               []byte(`{"transaction_status":"pending"}`),
	}, nil
}

func (m *mockGateway) GetStatus(_ context.Context, orderRef string) (*payment.StatusResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusCalls++
	if r, ok := m.statusByRef[orderRef]; ok {
		return r, nil
	}
	// Default: still pending.
	return &payment.StatusResult{TransactionStatus: "pending", StatusCode: "201"}, nil
}

func (m *mockGateway) VerifySignature(orderID, statusCode, grossAmount, signatureKey string) bool {
	expected := payment.ComputeSignature(orderID, statusCode, grossAmount, testServerKey)
	return expected == signatureKey
}

// setStatus configures the status returned for an order ref.
func (m *mockGateway) setStatus(ref string, r *payment.StatusResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusByRef[ref] = r
}

// mockSender is an in-memory CredentialSender for tests.
type mockSender struct {
	mu       sync.Mutex
	err      error
	messages []string
}

func newMockSender() *mockSender { return &mockSender{} }

func (s *mockSender) Send(_ context.Context, _ int64, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.messages = append(s.messages, text)
	return nil
}

func (s *mockSender) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.messages)
}
