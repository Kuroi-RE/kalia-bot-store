package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/kalia/store/internal/repository"
	"github.com/kalia/store/pkg/crypto"
)

// doJSON issues a request through the Fiber test harness and decodes the body.
func doJSON(t *testing.T, c *Container, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.App.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func httptest(method, path string, body *bytes.Buffer) *http.Request {
	req, _ := http.NewRequest(method, path, body)
	return req
}

// emptyBody returns an empty buffer for bodyless requests.
func emptyBody() *bytes.Buffer { return &bytes.Buffer{} }

func TestAuthFlow(t *testing.T) {
	c := newTestContainer(t)
	ctx := context.Background()

	// Seed a dedicated admin with known credentials (unique username per run).
	username := fmt.Sprintf("itest_%d", time.Now().UnixNano())
	password := "testpass123"
	hash, err := crypto.HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	repo := repository.NewAdminRepository(c.Pool)
	if _, err := repo.Create(ctx, username, hash); err != nil {
		t.Fatalf("create admin: %v", err)
	}

	// 1. Login success returns a token.
	status, body := doJSON(t, c, http.MethodPost, "/api/v1/auth/login", "",
		map[string]string{"username": username, "password": password})
	if status != http.StatusOK {
		t.Fatalf("login: expected 200, got %d (%v)", status, body)
	}
	token, _ := body["token"].(string)
	if token == "" {
		t.Fatalf("login: expected token in response, got %v", body)
	}

	// 2. Login with wrong password is rejected.
	status, _ = doJSON(t, c, http.MethodPost, "/api/v1/auth/login", "",
		map[string]string{"username": username, "password": "wrongpass"})
	if status != http.StatusUnauthorized {
		t.Fatalf("bad login: expected 401, got %d", status)
	}

	// 3. Protected route without a token is 401.
	status, _ = doJSON(t, c, http.MethodGet, "/api/v1/auth/me", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("me without token: expected 401, got %d", status)
	}

	// 4. Protected route with a valid token returns the admin.
	status, body = doJSON(t, c, http.MethodGet, "/api/v1/auth/me", token, nil)
	if status != http.StatusOK {
		t.Fatalf("me with token: expected 200, got %d (%v)", status, body)
	}
	if body["username"] != username {
		t.Fatalf("me: expected username %q, got %v", username, body["username"])
	}
}
