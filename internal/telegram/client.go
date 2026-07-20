// Package telegram is the backend's client for delivering messages to the
// Telegram Bot API (backend -> Telegram, distinct from the bot's REST calls
// into the backend).
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client sends messages via the Telegram Bot API.
type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

// NewClient builds a Telegram client. token is the bot token. httpClient may be nil.
func NewClient(token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{
		token:   token,
		baseURL: "https://api.telegram.org",
		http:    httpClient,
	}
}

// Send delivers a text message to a chat (the customer's telegram_id).
func (c *Client) Send(ctx context.Context, chatID int64, text string) error {
	if c.token == "" {
		return fmt.Errorf("telegram bot token not configured")
	}
	body, _ := json.Marshal(map[string]any{
		"chat_id": chatID,
		"text":    text,
	})
	url := fmt.Sprintf("%s/bot%s/sendMessage", c.baseURL, c.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("telegram send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram send: http %d: %s", resp.StatusCode, string(b))
	}
	return nil
}
