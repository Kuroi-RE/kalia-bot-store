package app

import (
	"fmt"
	"net/http"
	"testing"
)

// addAccount adds a single account to a product via the admin API.
func addAccount(t *testing.T, c *Container, tok string, productID int64, creds map[string]any) {
	t.Helper()
	status, body := doJSON(t, c, http.MethodPost, fmt.Sprintf("/api/v1/products/%d/accounts", productID), tok,
		map[string]any{"credentials": creds})
	if status != http.StatusCreated {
		t.Fatalf("add account: status %d (%v)", status, body)
	}
}

func TestBotOrderCreation(t *testing.T) {
	c := newTestContainer(t)
	tok := adminToken(t, c)
	const botToken = "test-bot-token"

	// Product with a single account (schema-less for simplicity).
	pid := createProduct(t, c, tok, nil)
	addAccount(t, c, tok, pid, map[string]any{"email": "acc1@x.com", "password": "p1"})

	orderReq := map[string]any{
		"telegram_user": map[string]any{"id": 12345, "username": "buyer", "first_name": "Bee"},
		"product_id":    pid,
	}

	// Happy path.
	status, body := doJSON(t, c, http.MethodPost, "/api/v1/bot/orders", botToken, orderReq)
	if status != http.StatusCreated {
		t.Fatalf("create order: expected 201, got %d (%v)", status, body)
	}
	ref, _ := body["order_ref"].(string)
	if ref == "" {
		t.Fatalf("create order: missing order_ref (%v)", body)
	}
	if body["amount"].(float64) != 10000 { // createProduct sets base_price 10000
		t.Fatalf("create order: expected amount 10000, got %v", body["amount"])
	}
	if body["status"] != "PENDING" {
		t.Fatalf("create order: expected PENDING, got %v", body["status"])
	}
	if body["expires_at"] == nil {
		t.Fatalf("create order: expected expires_at set")
	}
	if body["qr_image"] == nil || body["qr_image"] == "" {
		t.Fatalf("create order: expected qr_image in response (%v)", body)
	}
	if body["payment_status"] != "PENDING" {
		t.Fatalf("create order: expected payment_status PENDING, got %v", body["payment_status"])
	}

	// The account is now reserved -> inventory summary shows 1 reserved / 0 available.
	_, sum := doJSON(t, c, http.MethodGet, fmt.Sprintf("/api/v1/products/%d/inventory-summary", pid), tok, nil)
	if sum["reserved"].(float64) != 1 || sum["available"].(float64) != 0 {
		t.Fatalf("after order: expected 1 reserved/0 available, got %v", sum)
	}

	// Second order for the same product is out of stock (only 1 account).
	status, _ = doJSON(t, c, http.MethodPost, "/api/v1/bot/orders", botToken, orderReq)
	if status != http.StatusConflict {
		t.Fatalf("out of stock: expected 409, got %d", status)
	}

	// Poll the order by ref.
	status, body = doJSON(t, c, http.MethodGet, "/api/v1/bot/orders/"+ref, botToken, nil)
	if status != http.StatusOK || body["order_ref"] != ref {
		t.Fatalf("get order: status %d body %v", status, body)
	}
	if body["account_id"] == nil {
		t.Fatalf("get order: expected account_id linked, got %v", body)
	}
}

func TestBotOrderPriceOverride(t *testing.T) {
	c := newTestContainer(t)
	tok := adminToken(t, c)
	const botToken = "test-bot-token"

	pid := createProduct(t, c, tok, nil)
	addAccount(t, c, tok, pid, map[string]any{"email": "o@x.com"})

	status, body := doJSON(t, c, http.MethodPost, "/api/v1/bot/orders", botToken, map[string]any{
		"telegram_user":  map[string]any{"id": 999},
		"product_id":     pid,
		"price_override": 25000,
	})
	if status != http.StatusCreated {
		t.Fatalf("override order: expected 201, got %d (%v)", status, body)
	}
	if body["amount"].(float64) != 25000 {
		t.Fatalf("override order: expected amount 25000, got %v", body["amount"])
	}
}

func TestBotOrderInactiveProduct(t *testing.T) {
	c := newTestContainer(t)
	tok := adminToken(t, c)
	const botToken = "test-bot-token"

	pid := createProduct(t, c, tok, nil)
	addAccount(t, c, tok, pid, map[string]any{"email": "z@x.com"})
	// Disable the product.
	doJSON(t, c, http.MethodPatch, fmt.Sprintf("/api/v1/products/%d/status", pid), tok, map[string]any{"is_active": false})

	status, _ := doJSON(t, c, http.MethodPost, "/api/v1/bot/orders", botToken, map[string]any{
		"telegram_user": map[string]any{"id": 111},
		"product_id":    pid,
	})
	if status != http.StatusConflict {
		t.Fatalf("inactive product order: expected 409, got %d", status)
	}
}
