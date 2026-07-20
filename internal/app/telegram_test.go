package app

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestTelegramContent(t *testing.T) {
	c := newTestContainer(t)
	tok := adminToken(t, c)
	const botToken = "test-bot-token" // matches newTestContainer config

	// Bot endpoint requires the service token.
	status, _ := doJSON(t, c, http.MethodGet, "/api/v1/bot/menus", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("bot menus without token: expected 401, got %d", status)
	}

	// Unique command per run to avoid clashes across repeated test runs.
	menuCmd := fmt.Sprintf("/order%d", time.Now().UnixNano())
	respCmd := fmt.Sprintf("/testimoni%d", time.Now().UnixNano())

	// Admin creates a menu (JWT required).
	status, menu := doJSON(t, c, http.MethodPost, "/api/v1/telegram/menus", tok, map[string]any{
		"command":    menuCmd,
		"title":      "Order",
		"reply_text": "Choose a product",
		"is_enabled": true,
		"sort_order": 1,
	})
	if status != http.StatusCreated {
		t.Fatalf("create menu: expected 201, got %d (%v)", status, menu)
	}
	menuID := int64(menu["id"].(float64))

	// Duplicate command -> 409.
	status, _ = doJSON(t, c, http.MethodPost, "/api/v1/telegram/menus", tok, map[string]any{
		"command": menuCmd, "title": "dup",
	})
	if status != http.StatusConflict {
		t.Fatalf("duplicate menu: expected 409, got %d", status)
	}

	// Admin creates a static response.
	status, _ = doJSON(t, c, http.MethodPost, "/api/v1/telegram/responses", tok, map[string]any{
		"command":    respCmd,
		"reply_text": "Great service!",
		"is_enabled": true,
	})
	if status != http.StatusCreated {
		t.Fatalf("create response: expected 201, got %d", status)
	}

	// Bot fetches enabled menus (using bot token via Bearer).
	status, body := doJSON(t, c, http.MethodGet, "/api/v1/bot/menus", botToken, nil)
	if status != http.StatusOK {
		t.Fatalf("bot menus: expected 200, got %d", status)
	}
	if !menuPresent(body, menuCmd) {
		t.Fatalf("bot menus: expected to find %s in %v", menuCmd, body)
	}

	// Disable the menu; bot list should drop it.
	status, _ = doJSON(t, c, http.MethodPatch, fmt.Sprintf("/api/v1/telegram/menus/%d/status", menuID), tok,
		map[string]any{"is_enabled": false})
	if status != http.StatusOK {
		t.Fatalf("disable menu: got %d", status)
	}
	_, body = doJSON(t, c, http.MethodGet, "/api/v1/bot/menus", botToken, nil)
	if menuPresent(body, menuCmd) {
		t.Fatalf("bot menus: disabled menu %s should not appear", menuCmd)
	}

	// Bot resolves the response by command (without leading slash -> normalized).
	trimmed := respCmd[1:] // drop leading slash
	status, body = doJSON(t, c, http.MethodGet, "/api/v1/bot/responses/"+trimmed, botToken, nil)
	if status != http.StatusOK {
		t.Fatalf("bot response: expected 200, got %d (%v)", status, body)
	}
	if body["reply_text"] != "Great service!" {
		t.Fatalf("bot response: unexpected reply %v", body["reply_text"])
	}

	// Unknown command -> 404.
	status, _ = doJSON(t, c, http.MethodGet, "/api/v1/bot/responses/doesnotexist999", botToken, nil)
	if status != http.StatusNotFound {
		t.Fatalf("unknown response: expected 404, got %d", status)
	}
}

func menuPresent(body map[string]any, command string) bool {
	items, ok := body["items"].([]any)
	if !ok {
		return false
	}
	for _, it := range items {
		m, ok := it.(map[string]any)
		if ok && m["command"] == command {
			return true
		}
	}
	return false
}
