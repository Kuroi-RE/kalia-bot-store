package payment

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Midtrans implements Gateway against the Midtrans Core API.
type Midtrans struct {
	serverKey       string
	baseURL         string
	defaultAcquirer string
	http            *http.Client
}

// NewMidtrans builds a Midtrans gateway. baseURL is the Core API root
// (sandbox or production). httpClient may be nil (a sane default is used).
func NewMidtrans(serverKey, baseURL, defaultAcquirer string, httpClient *http.Client) *Midtrans {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Midtrans{
		serverKey:       serverKey,
		baseURL:         strings.TrimRight(baseURL, "/"),
		defaultAcquirer: defaultAcquirer,
		http:            httpClient,
	}
}

// Name returns the provider name.
func (m *Midtrans) Name() string { return "midtrans" }

// ---- request/response payloads ----

type chargeAction struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	URL    string `json:"url"`
}

type chargeResponse struct {
	StatusCode        string         `json:"status_code"`
	StatusMessage     string         `json:"status_message"`
	TransactionID     string         `json:"transaction_id"`
	OrderID           string         `json:"order_id"`
	GrossAmount       string         `json:"gross_amount"`
	TransactionStatus string         `json:"transaction_status"`
	QRString          string         `json:"qr_string"`
	Actions           []chargeAction `json:"actions"`
	ExpiryTime        string         `json:"expiry_time"`
}

type statusResponse struct {
	StatusCode        string `json:"status_code"`
	TransactionID     string `json:"transaction_id"`
	OrderID           string `json:"order_id"`
	GrossAmount       string `json:"gross_amount"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
	SignatureKey      string `json:"signature_key"`
}

// CreateCharge creates a QRIS charge.
func (m *Midtrans) CreateCharge(ctx context.Context, req ChargeRequest) (*ChargeResult, error) {
	acquirer := req.Acquirer
	if acquirer == "" {
		acquirer = m.defaultAcquirer
	}

	body := map[string]any{
		"payment_type": "qris",
		"transaction_details": map[string]any{
			"order_id":     req.OrderRef,
			"gross_amount": req.GrossAmount,
		},
		"qris": map[string]any{"acquirer": acquirer},
	}
	if req.CustomExpirySeconds > 0 {
		body["custom_expiry"] = map[string]any{
			"expiry_duration": req.CustomExpirySeconds,
			"unit":            "second",
		}
	}

	raw, err := m.do(ctx, http.MethodPost, "/v2/charge", body)
	if err != nil {
		return nil, err
	}

	var cr chargeResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return nil, fmt.Errorf("decode charge response: %w", err)
	}
	// Midtrans returns 2xx-style status codes in the body; 201 = created.
	if cr.StatusCode != "201" && cr.StatusCode != "200" {
		return nil, fmt.Errorf("midtrans charge rejected: status_code=%s message=%s", cr.StatusCode, cr.StatusMessage)
	}

	result := &ChargeResult{
		TransactionID:     cr.TransactionID,
		TransactionStatus: cr.TransactionStatus,
		QRString:          cr.QRString,
		ExpiresAtRFC3339:  cr.ExpiryTime,
		Raw:               raw,
	}
	result.QRImageURL = pickQRURL(cr.Actions)
	return result, nil
}

// GetStatus queries transaction status.
func (m *Midtrans) GetStatus(ctx context.Context, orderRef string) (*StatusResult, error) {
	raw, err := m.do(ctx, http.MethodGet, "/v2/"+orderRef+"/status", nil)
	if err != nil {
		return nil, err
	}
	var sr statusResponse
	if err := json.Unmarshal(raw, &sr); err != nil {
		return nil, fmt.Errorf("decode status response: %w", err)
	}
	return &StatusResult{
		TransactionID:     sr.TransactionID,
		TransactionStatus: sr.TransactionStatus,
		StatusCode:        sr.StatusCode,
		GrossAmount:       sr.GrossAmount,
		SignatureKey:      sr.SignatureKey,
		FraudStatus:       sr.FraudStatus,
		Raw:               raw,
	}, nil
}

// VerifySignature validates the webhook signature:
// SHA512(order_id + status_code + gross_amount + ServerKey).
func (m *Midtrans) VerifySignature(orderID, statusCode, grossAmount, signatureKey string) bool {
	expected := ComputeSignature(orderID, statusCode, grossAmount, m.serverKey)
	// Case-insensitive hex comparison; both are hex strings.
	return strings.EqualFold(expected, signatureKey)
}

// ComputeSignature returns the Midtrans SHA512 signature hex string.
func ComputeSignature(orderID, statusCode, grossAmount, serverKey string) string {
	sum := sha512.Sum512([]byte(orderID + statusCode + grossAmount + serverKey))
	return hex.EncodeToString(sum[:])
}

// pickQRURL selects the QR image URL from the actions list.
func pickQRURL(actions []chargeAction) string {
	var v1 string
	for _, a := range actions {
		switch a.Name {
		case "generate-qr-code":
			v1 = a.URL
		case "generate-qr-code-v2":
			if v1 == "" {
				v1 = a.URL
			}
		}
	}
	return v1
}

// do performs an authenticated Core API request with a small retry on 5xx.
func (m *Midtrans) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = b
	}

	auth := base64.StdEncoding.EncodeToString([]byte(m.serverKey + ":"))

	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		httpReq, err := http.NewRequestWithContext(ctx, method, m.baseURL+path, reader)
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Accept", "application/json")
		httpReq.Header.Set("Authorization", "Basic "+auth)
		if payload != nil {
			httpReq.Header.Set("Content-Type", "application/json")
		}

		resp, err := m.http.Do(httpReq)
		if err != nil {
			lastErr = err
			backoff(attempt)
			continue
		}
		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			backoff(attempt)
			continue
		}
		// Retry only on transient 5xx; 4xx are returned to caller for handling.
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("midtrans %s %s: http %d: %s", method, path, resp.StatusCode, truncate(respBody))
			backoff(attempt)
			continue
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("midtrans auth error: http %d: %s", resp.StatusCode, truncate(respBody))
		}
		return respBody, nil
	}
	return nil, fmt.Errorf("midtrans request failed after %d attempts: %w", maxAttempts, lastErr)
}

func backoff(attempt int) {
	time.Sleep(time.Duration(attempt*attempt) * 100 * time.Millisecond)
}

func truncate(b []byte) string {
	const max = 300
	if len(b) > max {
		return string(b[:max])
	}
	return string(b)
}
