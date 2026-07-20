package app

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/kalia/store/internal/payment"
)

// placeOrder creates a product+account and an order, returning its order_ref.
func placeOrder(t *testing.T, c *Container, adminTok, botTok string, tgID int64) (string, int64) {
	t.Helper()
	pid := createProduct(t, c, adminTok, nil) // base_price 10000
	addAccount(t, c, adminTok, pid, map[string]any{"email": "w@x.com", "password": "pw"})
	status, body := doJSON(t, c, http.MethodPost, "/api/v1/bot/orders", botTok, map[string]any{
		"telegram_user": map[string]any{"id": tgID},
		"product_id":    pid,
	})
	if status != http.StatusCreated {
		t.Fatalf("place order: status %d (%v)", status, body)
	}
	return body["order_ref"].(string), pid
}

// notify posts a signed midtrans notification for the given status.
func notify(t *testing.T, c *Container, ref, txnStatus, statusCode, gross, txnID, sigOverride string) (int, map[string]any) {
	t.Helper()
	sig := sigOverride
	if sig == "" {
		sig = payment.ComputeSignature(ref, statusCode, gross, testServerKey)
	}
	payload := map[string]any{
		"order_id":           ref,
		"transaction_id":     txnID,
		"transaction_status": txnStatus,
		"status_code":        statusCode,
		"gross_amount":       gross,
		"signature_key":      sig,
	}
	return doJSON(t, c, http.MethodPost, "/webhooks/midtrans", "", payload)
}

func orderStatus(t *testing.T, c *Container, botTok, ref string) string {
	t.Helper()
	_, body := doJSON(t, c, http.MethodGet, "/api/v1/bot/orders/"+ref, botTok, nil)
	return fmt.Sprint(body["status"])
}

func TestWebhookSettlementIdempotent(t *testing.T) {
	c := newTestContainer(t)
	adminTok := adminToken(t, c)
	const botTok = "test-bot-token"

	ref, _ := placeOrder(t, c, adminTok, botTok, 500001)

	// Settlement notification -> order becomes PAID.
	status, _ := notify(t, c, ref, "settlement", "200", "10000.00", "txn-1", "")
	if status != http.StatusOK {
		t.Fatalf("settlement: expected 200, got %d", status)
	}
	if s := orderStatus(t, c, botTok, ref); s != "PAID" {
		t.Fatalf("after settlement: expected PAID, got %s", s)
	}

	// Duplicate settlement (same txn+status) -> still 200, still PAID.
	status, _ = notify(t, c, ref, "settlement", "200", "10000.00", "txn-1", "")
	if status != http.StatusOK {
		t.Fatalf("duplicate settlement: expected 200, got %d", status)
	}
	if s := orderStatus(t, c, botTok, ref); s != "PAID" {
		t.Fatalf("after duplicate: expected PAID, got %s", s)
	}
}

func TestWebhookTamperedSignature(t *testing.T) {
	c := newTestContainer(t)
	adminTok := adminToken(t, c)
	const botTok = "test-bot-token"

	ref, _ := placeOrder(t, c, adminTok, botTok, 500002)

	status, _ := notify(t, c, ref, "settlement", "200", "10000.00", "txn-bad", "deadbeef-not-a-valid-sig")
	if status != http.StatusUnauthorized {
		t.Fatalf("tampered signature: expected 401, got %d", status)
	}
	// Order must remain PENDING.
	if s := orderStatus(t, c, botTok, ref); s != "PENDING" {
		t.Fatalf("after tampered: expected PENDING, got %s", s)
	}
}

func TestWebhookExpireReleasesReservation(t *testing.T) {
	c := newTestContainer(t)
	adminTok := adminToken(t, c)
	const botTok = "test-bot-token"

	ref, pid := placeOrder(t, c, adminTok, botTok, 500003)

	// Before expiry: 1 reserved.
	_, sum := doJSON(t, c, http.MethodGet, fmt.Sprintf("/api/v1/products/%d/inventory-summary", pid), adminTok, nil)
	if sum["reserved"].(float64) != 1 {
		t.Fatalf("expected 1 reserved, got %v", sum)
	}

	status, _ := notify(t, c, ref, "expire", "202", "10000.00", "txn-exp", "")
	if status != http.StatusOK {
		t.Fatalf("expire: expected 200, got %d", status)
	}
	if s := orderStatus(t, c, botTok, ref); s != "EXPIRED" {
		t.Fatalf("after expire: expected EXPIRED, got %s", s)
	}
	// Account returned to AVAILABLE.
	_, sum = doJSON(t, c, http.MethodGet, fmt.Sprintf("/api/v1/products/%d/inventory-summary", pid), adminTok, nil)
	if sum["available"].(float64) != 1 || sum["reserved"].(float64) != 0 {
		t.Fatalf("after expire: expected 1 available/0 reserved, got %v", sum)
	}
}

func TestWebhookDenyCancels(t *testing.T) {
	c := newTestContainer(t)
	adminTok := adminToken(t, c)
	const botTok = "test-bot-token"

	ref, _ := placeOrder(t, c, adminTok, botTok, 500004)

	status, _ := notify(t, c, ref, "deny", "202", "10000.00", "txn-deny", "")
	if status != http.StatusOK {
		t.Fatalf("deny: expected 200, got %d", status)
	}
	if s := orderStatus(t, c, botTok, ref); s != "CANCELLED" {
		t.Fatalf("after deny: expected CANCELLED, got %s", s)
	}
}
