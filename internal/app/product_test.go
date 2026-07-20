package app

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/kalia/store/internal/repository"
	"github.com/kalia/store/pkg/crypto"
)

// adminToken seeds a fresh admin and returns a valid bearer token.
func adminToken(t *testing.T, c *Container) string {
	t.Helper()
	ctx := context.Background()
	username := fmt.Sprintf("padmin_%d", time.Now().UnixNano())
	password := "testpass123"
	hash, err := crypto.HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if _, err := repository.NewAdminRepository(c.Pool).Create(ctx, username, hash); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	status, body := doJSON(t, c, http.MethodPost, "/api/v1/auth/login", "",
		map[string]string{"username": username, "password": password})
	if status != http.StatusOK {
		t.Fatalf("login for token: status %d", status)
	}
	return body["token"].(string)
}

func TestProductCRUD(t *testing.T) {
	c := newTestContainer(t)
	tok := adminToken(t, c)

	// Requires auth.
	status, _ := doJSON(t, c, http.MethodGet, "/api/v1/products", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("list without token: expected 401, got %d", status)
	}

	// Create.
	createBody := map[string]any{
		"name":        "Netflix Premium",
		"description": "4K UHD shared",
		"base_price":  50000,
		"is_active":   true,
		"credential_schema": []map[string]any{
			{"key": "email", "label": "Email", "type": "string", "required": true},
			{"key": "password", "label": "Password", "type": "secret", "required": true},
		},
	}
	status, body := doJSON(t, c, http.MethodPost, "/api/v1/products", tok, createBody)
	if status != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d (%v)", status, body)
	}
	id := int64(body["id"].(float64))
	if body["base_price"].(float64) != 50000 {
		t.Fatalf("create: unexpected base_price %v", body["base_price"])
	}
	schema := body["credential_schema"].([]any)
	if len(schema) != 2 {
		t.Fatalf("create: expected 2 credential fields, got %d", len(schema))
	}

	// Get.
	status, body = doJSON(t, c, http.MethodGet, fmt.Sprintf("/api/v1/products/%d", id), tok, nil)
	if status != http.StatusOK || body["name"] != "Netflix Premium" {
		t.Fatalf("get: status %d body %v", status, body)
	}

	// Update (price + name).
	updateBody := map[string]any{
		"name":        "Netflix Premium 1P1U",
		"description": "Private profile",
		"base_price":  65000,
		"credential_schema": []map[string]any{
			{"key": "email", "required": true},
		},
	}
	status, body = doJSON(t, c, http.MethodPut, fmt.Sprintf("/api/v1/products/%d", id), tok, updateBody)
	if status != http.StatusOK {
		t.Fatalf("update: expected 200, got %d (%v)", status, body)
	}
	if body["base_price"].(float64) != 65000 || body["name"] != "Netflix Premium 1P1U" {
		t.Fatalf("update: fields not applied: %v", body)
	}

	// Disable via status patch.
	status, body = doJSON(t, c, http.MethodPatch, fmt.Sprintf("/api/v1/products/%d/status", id), tok,
		map[string]any{"is_active": false})
	if status != http.StatusOK || body["is_active"] != false {
		t.Fatalf("disable: status %d body %v", status, body)
	}

	// List filtered by is_active=false should include our product.
	status, body = doJSON(t, c, http.MethodGet, "/api/v1/products?is_active=false", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("list: status %d", status)
	}
	if _, ok := body["items"].([]any); !ok {
		t.Fatalf("list: missing items array: %v", body)
	}

	// Delete.
	req := httptest(http.MethodDelete, fmt.Sprintf("/api/v1/products/%d", id), emptyBody())
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := c.App.Test(req, -1)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", resp.StatusCode)
	}

	// Confirm gone.
	status, _ = doJSON(t, c, http.MethodGet, fmt.Sprintf("/api/v1/products/%d", id), tok, nil)
	if status != http.StatusNotFound {
		t.Fatalf("get after delete: expected 404, got %d", status)
	}
}

func TestProductValidation(t *testing.T) {
	c := newTestContainer(t)
	tok := adminToken(t, c)

	// Missing name.
	status, _ := doJSON(t, c, http.MethodPost, "/api/v1/products", tok,
		map[string]any{"base_price": 1000})
	if status != http.StatusBadRequest {
		t.Fatalf("missing name: expected 400, got %d", status)
	}

	// Duplicate credential keys.
	status, _ = doJSON(t, c, http.MethodPost, "/api/v1/products", tok, map[string]any{
		"name":       "Dup",
		"base_price": 1000,
		"credential_schema": []map[string]any{
			{"key": "email"}, {"key": "email"},
		},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("dup keys: expected 400, got %d", status)
	}
}
