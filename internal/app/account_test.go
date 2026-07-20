package app

import (
	"fmt"
	"net/http"
	"testing"
)

// createProduct is a helper that creates a product and returns its id.
func createProduct(t *testing.T, c *Container, tok string, schema []map[string]any) int64 {
	t.Helper()
	body := map[string]any{
		"name":              "Prod " + fmt.Sprint(len(schema)),
		"base_price":        10000,
		"is_active":         true,
		"credential_schema": schema,
	}
	status, resp := doJSON(t, c, http.MethodPost, "/api/v1/products", tok, body)
	if status != http.StatusCreated {
		t.Fatalf("create product: status %d (%v)", status, resp)
	}
	return int64(resp["id"].(float64))
}

func TestAccountInventory(t *testing.T) {
	c := newTestContainer(t)
	tok := adminToken(t, c)

	pid := createProduct(t, c, tok, []map[string]any{
		{"key": "email", "required": true},
		{"key": "password", "required": true},
	})

	// Single create.
	status, body := doJSON(t, c, http.MethodPost, fmt.Sprintf("/api/v1/products/%d/accounts", pid), tok,
		map[string]any{"credentials": map[string]any{"email": "a@x.com", "password": "p1"}})
	if status != http.StatusCreated || body["count"].(float64) != 1 {
		t.Fatalf("single create: status %d body %v", status, body)
	}

	// Bulk create.
	status, body = doJSON(t, c, http.MethodPost, fmt.Sprintf("/api/v1/products/%d/accounts", pid), tok,
		map[string]any{"accounts": []map[string]any{
			{"credentials": map[string]any{"email": "b@x.com", "password": "p2"}},
			{"credentials": map[string]any{"email": "c@x.com", "password": "p3"}},
		}})
	if status != http.StatusCreated || body["count"].(float64) != 2 {
		t.Fatalf("bulk create: status %d body %v", status, body)
	}

	// Missing required field is rejected.
	status, _ = doJSON(t, c, http.MethodPost, fmt.Sprintf("/api/v1/products/%d/accounts", pid), tok,
		map[string]any{"credentials": map[string]any{"email": "d@x.com"}})
	if status != http.StatusBadRequest {
		t.Fatalf("missing field: expected 400, got %d", status)
	}

	// Inventory summary: 3 available.
	status, body = doJSON(t, c, http.MethodGet, fmt.Sprintf("/api/v1/products/%d/inventory-summary", pid), tok, nil)
	if status != http.StatusOK {
		t.Fatalf("summary: status %d", status)
	}
	if body["available"].(float64) != 3 || body["total"].(float64) != 3 {
		t.Fatalf("summary: expected 3 available/total, got %v", body)
	}

	// List available accounts.
	status, body = doJSON(t, c, http.MethodGet, fmt.Sprintf("/api/v1/products/%d/accounts?status=AVAILABLE", pid), tok, nil)
	if status != http.StatusOK || body["total"].(float64) != 3 {
		t.Fatalf("list available: status %d body %v", status, body)
	}
	items := body["items"].([]any)
	first := items[0].(map[string]any)
	accID := int64(first["id"].(float64))

	// Update account status to SOLD.
	status, body = doJSON(t, c, http.MethodPut, fmt.Sprintf("/api/v1/accounts/%d", accID), tok,
		map[string]any{"credentials": map[string]any{"email": "b@x.com", "password": "pX"}, "status": "SOLD"})
	if status != http.StatusOK || body["status"] != "SOLD" {
		t.Fatalf("update: status %d body %v", status, body)
	}

	// Summary now: 2 available, 1 sold.
	_, body = doJSON(t, c, http.MethodGet, fmt.Sprintf("/api/v1/products/%d/inventory-summary", pid), tok, nil)
	if body["available"].(float64) != 2 || body["sold"].(float64) != 1 {
		t.Fatalf("summary after sold: got %v", body)
	}

	// Delete the sold account.
	req := httptest(http.MethodDelete, fmt.Sprintf("/api/v1/accounts/%d", accID), emptyBody())
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := c.App.Test(req, -1)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", resp.StatusCode)
	}
}
