package app

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	nethttptest "net/http/httptest"
	"strings"
	"testing"

	"github.com/kalia/store/internal/payment"
)

// These tests exercise the real Midtrans adapter's HTTP parsing and signature
// logic using an in-process httptest server. They live in the app test package
// because the Device Guard policy blocks the standalone payment test binary.

func TestMidtransComputeAndVerifySignature(t *testing.T) {
	sig := payment.ComputeSignature("ORDER1", "200", "50000", "server-key")
	if len(sig) != 128 {
		t.Fatalf("expected 128-char hex signature, got %d", len(sig))
	}
	if sig == payment.ComputeSignature("ORDER1", "200", "50001", "server-key") {
		t.Fatal("signature should change with input")
	}

	m := payment.NewMidtrans("my-server-key", "https://example.com", "gopay", nil)
	good := payment.ComputeSignature("ORD9", "200", "10000", "my-server-key")
	if !m.VerifySignature("ORD9", "200", "10000", good) {
		t.Fatal("valid signature should verify")
	}
	if m.VerifySignature("ORD9", "200", "10000", "deadbeef") {
		t.Fatal("tampered signature must not verify")
	}
}

func TestMidtransCreateChargeParsesQR(t *testing.T) {
	var gotAuth, gotPath string
	srv := nethttptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"payment_type":"qris"`) {
			t.Errorf("expected qris payment_type in body, got %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status_code": "201",
			"transaction_id": "abc-123",
			"order_id": "ORDER-XYZ",
			"gross_amount": "50000.00",
			"transaction_status": "pending",
			"qr_string": "QRPAYLOAD",
			"actions": [{"name": "generate-qr-code", "method": "GET", "url": "https://mid/qr.png"}],
			"expiry_time": "2026-01-01 00:15:00"
		}`))
	}))
	defer srv.Close()

	m := payment.NewMidtrans("sk-test", srv.URL, "gopay", srv.Client())
	res, err := m.CreateCharge(context.Background(), payment.ChargeRequest{
		OrderRef: "ORDER-XYZ", GrossAmount: 50000, Acquirer: "gopay", CustomExpirySeconds: 900,
	})
	if err != nil {
		t.Fatalf("charge: %v", err)
	}
	if res.TransactionID != "abc-123" || res.QRImageURL != "https://mid/qr.png" || res.QRString != "QRPAYLOAD" {
		t.Fatalf("unexpected charge result: %+v", res)
	}
	if gotPath != "/v2/charge" {
		t.Fatalf("path: got %s", gotPath)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("sk-test:"))
	if gotAuth != wantAuth {
		t.Fatalf("auth header: got %q want %q", gotAuth, wantAuth)
	}
}

func TestMidtransCreateChargeRejectsError(t *testing.T) {
	srv := nethttptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status_code":"400","status_message":"bad request"}`))
	}))
	defer srv.Close()
	m := payment.NewMidtrans("sk", srv.URL, "gopay", srv.Client())
	if _, err := m.CreateCharge(context.Background(), payment.ChargeRequest{OrderRef: "X", GrossAmount: 1}); err == nil {
		t.Fatal("expected error for non-201 status_code")
	}
}

func TestMidtransGetStatusParses(t *testing.T) {
	srv := nethttptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/status") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status_code":"200","transaction_id":"t1","order_id":"O1",
			"gross_amount":"50000.00","transaction_status":"settlement",
			"fraud_status":"accept","signature_key":"sig"
		}`))
	}))
	defer srv.Close()
	m := payment.NewMidtrans("sk", srv.URL, "gopay", srv.Client())
	st, err := m.GetStatus(context.Background(), "O1")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.TransactionStatus != "settlement" || st.StatusCode != "200" {
		t.Fatalf("unexpected status: %+v", st)
	}
}
