// Command verify is an in-process integration harness. Because the environment
// blocks `go test` binaries (Device Guard), this normal `go build` binary
// exercises the HTTP flows against a real database using Fiber's App.Test and
// a fake payment gateway. Exit code is non-zero if any check fails.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kalia/store/internal/app"
	"github.com/kalia/store/internal/config"
	"github.com/kalia/store/internal/database"
	"github.com/kalia/store/internal/payment"
	"github.com/kalia/store/internal/repository"
	"github.com/kalia/store/internal/service"
	"github.com/kalia/store/internal/testkit"
	"github.com/kalia/store/pkg/crypto"
	"github.com/kalia/store/pkg/logger"
)

const (
	serverKey = "verify-server-key"
	botToken  = "verify-bot-token"
)

var (
	container *app.Container
	fakeGW    *testkit.FakeGateway
	fakeSender *testkit.FakeSender
	failures  int
)

func main() {
	dsn := os.Getenv("KALIA_TEST_DB")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		fmt.Println("KALIA_TEST_DB (or DATABASE_URL) must be set")
		os.Exit(2)
	}

	if err := database.Migrate(dsn); err != nil {
		fmt.Printf("migrate failed: %v\n", err)
		os.Exit(2)
	}
	pool, err := database.Connect(context.Background(), dsn)
	if err != nil {
		fmt.Printf("connect failed: %v\n", err)
		os.Exit(2)
	}
	defer pool.Close()

	cfg := &config.Config{
		AppEnv:          "test",
		JWTSecret:       "verify-secret",
		JWTTTL:          time.Hour,
		BotServiceToken: botToken,
		DatabaseURL:     dsn,
		ReservationTTL:  15 * time.Minute,
		PaymentTTL:      15 * time.Minute,
	}
	cfg.Midtrans.ServerKey = serverKey
	cfg.Midtrans.DefaultAcquirer = "gopay"

	fakeGW = testkit.NewFakeGateway(serverKey)
	fakeSender = testkit.NewFakeSender()
	container = app.Build(cfg, logger.New("error"), pool, fakeGW, fakeSender)

	adminTok := mustAdminToken()

	scenarioAuth(adminTok)
	scenarioProductAccount(adminTok)
	scenarioOrderAndWebhook(adminTok)
	scenarioDeliveryFailureRetry(adminTok)
	scenarioWorkerJobs(adminTok)
	scenarioHardening(adminTok)
	scenarioEndToEnd(adminTok)

	if failures > 0 {
		fmt.Printf("\nVERIFY FAILED: %d check(s) failed\n", failures)
		os.Exit(1)
	}
	fmt.Println("\nVERIFY OK: all checks passed")
}

// ---- helpers ----

func check(name string, cond bool, detail ...any) {
	if cond {
		fmt.Printf("  PASS %s\n", name)
		return
	}
	failures++
	fmt.Printf("  FAIL %s %v\n", name, detail)
}

func doJSON(method, path, token string, body any) (int, map[string]any) {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := container.App.Test(req, -1)
	if err != nil {
		return 0, map[string]any{"error": err.Error()}
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func mustAdminToken() string {
	ctx := context.Background()
	username := fmt.Sprintf("verify_%d", time.Now().UnixNano())
	hash, _ := crypto.HashPassword("verifypass123")
	if _, err := repository.NewAdminRepository(container.Pool).Create(ctx, username, hash); err != nil {
		fmt.Printf("seed admin failed: %v\n", err)
		os.Exit(2)
	}
	status, body := doJSON(http.MethodPost, "/api/v1/auth/login", "",
		map[string]string{"username": username, "password": "verifypass123"})
	if status != 200 {
		fmt.Printf("admin login failed: %d\n", status)
		os.Exit(2)
	}
	return body["token"].(string)
}

func scenarioAuth(tok string) {
	fmt.Println("[auth]")
	status, body := doJSON(http.MethodGet, "/api/v1/auth/me", tok, nil)
	check("me returns 200", status == 200, status)
	check("me has username", body["username"] != nil, body)
	status, _ = doJSON(http.MethodGet, "/api/v1/auth/me", "", nil)
	check("me without token 401", status == 401, status)
}

func createProduct(tok string) int64 {
	_, body := doJSON(http.MethodPost, "/api/v1/products", tok, map[string]any{
		"name": "Verify Product", "base_price": 10000, "is_active": true,
	})
	return int64(body["id"].(float64))
}

func addAccount(tok string, pid int64, email string) {
	doJSON(http.MethodPost, fmt.Sprintf("/api/v1/products/%d/accounts", pid), tok,
		map[string]any{"credentials": map[string]any{"email": email, "password": "pw"}})
}

func scenarioProductAccount(tok string) {
	fmt.Println("[product+account]")
	pid := createProduct(tok)
	addAccount(tok, pid, "pa@x.com")
	status, sum := doJSON(http.MethodGet, fmt.Sprintf("/api/v1/products/%d/inventory-summary", pid), tok, nil)
	check("inventory summary 200", status == 200, status)
	check("1 available", sum["available"].(float64) == 1, sum)
}

func scenarioOrderAndWebhook(adminTok string) {
	fmt.Println("[order+webhook]")
	run := time.Now().UnixNano() // unique suffix so txn ids don't collide across runs
	pid := createProduct(adminTok)
	addAccount(adminTok, pid, "ow@x.com")

	status, body := doJSON(http.MethodPost, "/api/v1/bot/orders", botToken, map[string]any{
		"telegram_user": map[string]any{"id": time.Now().UnixNano() % 1_000_000_000},
		"product_id":    pid,
	})
	check("create order 201", status == 201, status, body)
	ref, _ := body["order_ref"].(string)
	check("order has ref", ref != "", body)
	check("order has qr_image", body["qr_image"] != nil && body["qr_image"] != "", body)

	// Tampered signature -> 401, still PENDING.
	st, _ := notify(ref, "settlement", "200", "10000.00", fmt.Sprintf("vt-bad-%d", run), "not-valid")
	check("tampered sig 401", st == 401, st)
	check("still PENDING after tampered", orderStatus(botToken, ref) == "PENDING")

	// Valid settlement -> PAID then auto-DELIVERED, account SOLD, message sent.
	before := fakeSender.Count()
	st, _ = notify(ref, "settlement", "200", "10000.00", fmt.Sprintf("vt-1-%d", run), "")
	check("settlement 200", st == 200, st)
	check("order DELIVERED after settlement", orderStatus(botToken, ref) == "DELIVERED")
	check("credentials sent once", fakeSender.Count() == before+1, fakeSender.Count(), before)
	_, psum := doJSON(http.MethodGet, fmt.Sprintf("/api/v1/products/%d/inventory-summary", pid), adminTok, nil)
	check("account SOLD", psum["sold"].(float64) == 1, psum)

	// Duplicate settlement (same txn+status) -> still 200, still DELIVERED, no 2nd send.
	sentBefore := fakeSender.Count()
	st, _ = notify(ref, "settlement", "200", "10000.00", fmt.Sprintf("vt-1-%d", run), "")
	check("duplicate settlement 200", st == 200, st)
	check("still DELIVERED after duplicate", orderStatus(botToken, ref) == "DELIVERED")
	check("no duplicate send", fakeSender.Count() == sentBefore, fakeSender.Count(), sentBefore)

	// Expire flow on a second order releases the reservation.
	pid2 := createProduct(adminTok)
	addAccount(adminTok, pid2, "ex@x.com")
	_, ob := doJSON(http.MethodPost, "/api/v1/bot/orders", botToken, map[string]any{
		"telegram_user": map[string]any{"id": time.Now().UnixNano() % 1_000_000_000},
		"product_id":    pid2,
	})
	ref2 := ob["order_ref"].(string)
	st, _ = notify(ref2, "expire", "202", "10000.00", fmt.Sprintf("vt-exp-%d", run), "")
	check("expire 200", st == 200, st)
	check("order EXPIRED", orderStatus(botToken, ref2) == "EXPIRED")
	_, sum := doJSON(http.MethodGet, fmt.Sprintf("/api/v1/products/%d/inventory-summary", pid2), adminTok, nil)
	check("account released to available", sum["available"].(float64) == 1 && sum["reserved"].(float64) == 0, sum)
}

// scenarioDeliveryFailureRetry verifies a failed delivery marks the order
// FAILED and that a later redeliver succeeds once the sender recovers.
func scenarioDeliveryFailureRetry(adminTok string) {
	fmt.Println("[delivery failure + redeliver]")
	run := time.Now().UnixNano()
	pid := createProduct(adminTok)
	addAccount(adminTok, pid, "fail@x.com")
	_, ob := doJSON(http.MethodPost, "/api/v1/bot/orders", botToken, map[string]any{
		"telegram_user": map[string]any{"id": time.Now().UnixNano() % 1_000_000_000},
		"product_id":    pid,
	})
	ref := ob["order_ref"].(string)

	// Force the sender to fail, then settle -> order should end FAILED.
	fakeSender.SetErr(fmt.Errorf("telegram down"))
	st, _ := notify(ref, "settlement", "200", "10000.00", fmt.Sprintf("df-1-%d", run), "")
	check("settlement accepted (200)", st == 200, st)
	check("order FAILED after send error", orderStatus(botToken, ref) == "FAILED")
	_, sum := doJSON(http.MethodGet, fmt.Sprintf("/api/v1/products/%d/inventory-summary", pid), adminTok, nil)
	check("account not sold on failure", sum["sold"].(float64) == 0, sum)

	// Recover the sender and redeliver via the admin endpoint.
	fakeSender.SetErr(nil)
	oid := int64(orderInternalID(ref))
	st, body := doJSON(http.MethodPost, fmt.Sprintf("/api/v1/orders/%d/redeliver", oid), adminTok, nil)
	check("redeliver 200", st == 200, st, body)
	check("order DELIVERED after redeliver", orderStatus(botToken, ref) == "DELIVERED")
	_, sum = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/products/%d/inventory-summary", pid), adminTok, nil)
	check("account SOLD after redeliver", sum["sold"].(float64) == 1, sum)
}

// orderInternalID resolves an order's internal id from its ref via the bot
// endpoint (admin order lookup arrives in Task 9).
func orderInternalID(ref string) float64 {
	_, body := doJSON(http.MethodGet, "/api/v1/bot/orders/"+ref, botToken, nil)
	if id, ok := body["id"].(float64); ok {
		return id
	}
	return 0
}

// scenarioWorkerJobs verifies the background jobs: reconciliation (missed
// webhook), expiry cleanup + reservation release, and delivery retry. It calls
// the worker service methods directly (sharing the container's DB pool).
func scenarioWorkerJobs(adminTok string) {
	fmt.Println("[worker jobs]")
	ctx := context.Background()
	run := time.Now().UnixNano()
	log := logger.New("error")

	tm := repository.NewTxManager(container.Pool)
	paymentSvc := service.NewPaymentService(tm, fakeGW, log)
	deliverySvc := service.NewDeliveryService(tm, fakeSender, log)
	paymentSvc.SetDeliveryTrigger(deliverySvc)
	orderSvc := service.NewOrderService(tm, fakeGW, "gopay", 15*time.Minute, 15*time.Minute, log)

	// --- Reconciliation: settle via GetStatus, not a webhook ---
	pid := createProduct(adminTok)
	addAccount(adminTok, pid, "rec@x.com")
	_, ob := doJSON(http.MethodPost, "/api/v1/bot/orders", botToken, map[string]any{
		"telegram_user": map[string]any{"id": time.Now().UnixNano() % 1_000_000_000},
		"product_id":    pid,
	})
	ref := ob["order_ref"].(string)
	fakeGW.SetStatus(ref, &payment.StatusResult{
		TransactionStatus: "settlement", StatusCode: "200", GrossAmount: "10000.00",
		TransactionID: fmt.Sprintf("rec-%d", run),
	})
	before := fakeSender.Count()
	n, err := paymentSvc.ReconcilePending(ctx, 0, 200)
	check("reconcile ran without error", err == nil, err)
	check("reconcile advanced >=1", n >= 1, n)
	check("reconciled order DELIVERED", orderStatus(botToken, ref) == "DELIVERED")
	check("reconcile delivered credentials", fakeSender.Count() >= before+1, fakeSender.Count(), before)

	// --- Expiry cleanup: force an order past expires_at, then clean up ---
	pid2 := createProduct(adminTok)
	addAccount(adminTok, pid2, "exp@x.com")
	_, ob2 := doJSON(http.MethodPost, "/api/v1/bot/orders", botToken, map[string]any{
		"telegram_user": map[string]any{"id": time.Now().UnixNano() % 1_000_000_000},
		"product_id":    pid2,
	})
	ref2 := ob2["order_ref"].(string)
	// Age the order so it is overdue (fakeGW default status stays pending).
	if _, aerr := container.Pool.Exec(ctx,
		`UPDATE orders SET expires_at = now() - interval '1 hour' WHERE order_ref = $1`, ref2); aerr != nil {
		check("age order for expiry", false, aerr)
	}
	fakeGW.SetStatus(ref2, &payment.StatusResult{TransactionStatus: "pending", StatusCode: "201"})
	expired, err := orderSvc.CleanupExpired(ctx, 200)
	check("cleanup ran without error", err == nil, err)
	check("cleanup expired >=1", expired >= 1, expired)
	check("expired order status EXPIRED", orderStatus(botToken, ref2) == "EXPIRED")
	_, sum := doJSON(http.MethodGet, fmt.Sprintf("/api/v1/products/%d/inventory-summary", pid2), adminTok, nil)
	check("expired order released account", sum["available"].(float64) == 1 && sum["reserved"].(float64) == 0, sum)

	// --- Delivery retry: fail a delivery then let the worker retry it ---
	pid3 := createProduct(adminTok)
	addAccount(adminTok, pid3, "retry@x.com")
	_, ob3 := doJSON(http.MethodPost, "/api/v1/bot/orders", botToken, map[string]any{
		"telegram_user": map[string]any{"id": time.Now().UnixNano() % 1_000_000_000},
		"product_id":    pid3,
	})
	ref3 := ob3["order_ref"].(string)
	fakeSender.SetErr(fmt.Errorf("telegram down"))
	notify(ref3, "settlement", "200", "10000.00", fmt.Sprintf("wr-%d", run), "")
	check("retry: order FAILED first", orderStatus(botToken, ref3) == "FAILED")
	fakeSender.SetErr(nil)
	retried, err := deliverySvc.RetryFailed(ctx, 5, 200)
	check("retry ran without error", err == nil, err)
	check("retry succeeded >=1", retried >= 1, retried)
	check("retried order DELIVERED", orderStatus(botToken, ref3) == "DELIVERED")
}

// scenarioHardening verifies admin read/cancel APIs, payment lookups, settings
// API, and rate limiting.
func scenarioHardening(adminTok string) {
	fmt.Println("[hardening: admin apis + settings + rate limit]")

	st, body := doJSON(http.MethodGet, "/api/v1/orders?limit=5", adminTok, nil)
	check("orders list 200", st == 200, st)
	_, hasItems := body["items"].([]any)
	check("orders list has items array", hasItems, body)

	// Create a fresh order to cancel.
	pid := createProduct(adminTok)
	addAccount(adminTok, pid, "cancel@x.com")
	_, ob := doJSON(http.MethodPost, "/api/v1/bot/orders", botToken, map[string]any{
		"telegram_user": map[string]any{"id": time.Now().UnixNano() % 1_000_000_000},
		"product_id":    pid,
	})
	ref := ob["order_ref"].(string)
	oid := int64(orderInternalID(ref))

	st, gb := doJSON(http.MethodGet, fmt.Sprintf("/api/v1/orders/%d", oid), adminTok, nil)
	check("order get 200", st == 200, st)
	check("order get matches ref", gb["order_ref"] == ref, gb)

	st, pb := doJSON(http.MethodGet, fmt.Sprintf("/api/v1/orders/%d/payment", oid), adminTok, nil)
	check("order payment 200", st == 200, st)
	payID := int64(pb["id"].(float64))
	st, _ = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/payments/%d", payID), adminTok, nil)
	check("payment get 200", st == 200, st)

	// Cancel the PENDING order -> CANCELLED + account released.
	st, cb := doJSON(http.MethodPatch, fmt.Sprintf("/api/v1/orders/%d/cancel", oid), adminTok, nil)
	check("cancel 200", st == 200, st, cb)
	check("order CANCELLED", cb["status"] == "CANCELLED", cb)
	_, sum := doJSON(http.MethodGet, fmt.Sprintf("/api/v1/products/%d/inventory-summary", pid), adminTok, nil)
	check("cancel released account", sum["available"].(float64) == 1 && sum["reserved"].(float64) == 0, sum)

	// Cancelling a non-PENDING order -> 409.
	st, _ = doJSON(http.MethodPatch, fmt.Sprintf("/api/v1/orders/%d/cancel", oid), adminTok, nil)
	check("re-cancel 409", st == 409, st)

	// Settings API.
	st, sb := doJSON(http.MethodGet, "/api/v1/settings", adminTok, nil)
	check("settings list 200", st == 200, st)
	_, hasSettings := sb["items"].([]any)
	check("settings has items", hasSettings, sb)
	st, _ = doJSON(http.MethodPut, "/api/v1/settings/payment_ttl_seconds", adminTok, map[string]any{"value": "1200"})
	check("settings put 200", st == 200, st)
	st, one := doJSON(http.MethodGet, "/api/v1/settings/payment_ttl_seconds", adminTok, nil)
	check("settings get updated value", st == 200 && one["value"] == "1200", one)

	// Rate limiting: login limiter max 10/min -> some of 13 rapid attempts 429.
	got429 := false
	for i := 0; i < 13; i++ {
		s, _ := doJSON(http.MethodPost, "/api/v1/auth/login", "",
			map[string]string{"username": "nobody", "password": "wrongpass"})
		if s == 429 {
			got429 = true
			break
		}
	}
	check("rate limiter triggers 429", got429)
}

// scenarioEndToEnd exercises the full customer flow: admin defines a menu +
// static response and a product with inventory; the bot reads catalog/menu,
// places an order, pays via a settlement webhook, and receives credentials.
func scenarioEndToEnd(adminTok string) {
	fmt.Println("[end-to-end: menu -> product -> order -> settlement -> delivery]")
	run := time.Now().UnixNano()

	// Admin defines a dynamic menu and a static response.
	st, _ := doJSON(http.MethodPost, "/api/v1/telegram/menus", adminTok, map[string]any{
		"command": fmt.Sprintf("/order%d", run), "title": "Order", "reply_text": "Pick a product", "is_enabled": true,
	})
	check("e2e create menu", st == 201, st)
	st, _ = doJSON(http.MethodPost, "/api/v1/telegram/responses", adminTok, map[string]any{
		"command": fmt.Sprintf("/help%d", run), "reply_text": "Contact support", "is_enabled": true,
	})
	check("e2e create response", st == 201, st)

	// Admin creates a product with a credential schema + one account.
	st, prod := doJSON(http.MethodPost, "/api/v1/products", adminTok, map[string]any{
		"name": "Netflix E2E", "base_price": 55000, "is_active": true,
		"credential_schema": []map[string]any{
			{"key": "email", "label": "Email", "required": true},
			{"key": "password", "label": "Password", "type": "secret", "required": true},
		},
	})
	check("e2e create product", st == 201, st)
	pid := int64(prod["id"].(float64))
	st, _ = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/products/%d/accounts", pid), adminTok,
		map[string]any{"credentials": map[string]any{"email": "e2e@netflix.com", "password": "s3cret"}})
	check("e2e add account", st == 201, st)

	// Bot reads enabled menus and the active catalog.
	st, menus := doJSON(http.MethodGet, "/api/v1/bot/menus", botToken, nil)
	check("e2e bot menus 200", st == 200, st)
	_, hasMenus := menus["items"].([]any)
	check("e2e bot menus list", hasMenus, menus)
	st, cat := doJSON(http.MethodGet, "/api/v1/bot/products", botToken, nil)
	check("e2e bot products 200", st == 200, st)
	_, hasCat := cat["items"].([]any)
	check("e2e bot catalog list", hasCat, cat)

	// Bot places an order.
	st, ob := doJSON(http.MethodPost, "/api/v1/bot/orders", botToken, map[string]any{
		"telegram_user": map[string]any{"id": run % 1_000_000_000, "username": "e2euser", "first_name": "E2E"},
		"product_id":    pid,
	})
	check("e2e order 201", st == 201, st, ob)
	ref := ob["order_ref"].(string)
	check("e2e order has qr", ob["qr_image"] != nil && ob["qr_image"] != "", ob)

	// Customer pays -> settlement webhook -> auto delivery.
	before := fakeSender.Count()
	st, _ = notify(ref, "settlement", "200", "55000.00", fmt.Sprintf("e2e-%d", run), "")
	check("e2e settlement 200", st == 200, st)
	check("e2e order DELIVERED", orderStatus(botToken, ref) == "DELIVERED")
	check("e2e credentials sent", fakeSender.Count() == before+1, fakeSender.Count(), before)

	// Inventory reflects the sale.
	_, sum := doJSON(http.MethodGet, fmt.Sprintf("/api/v1/products/%d/inventory-summary", pid), adminTok, nil)
	check("e2e account SOLD", sum["sold"].(float64) == 1 && sum["available"].(float64) == 0, sum)

	// Delivered message contains the credentials.
	if len(fakeSender.Messages) > 0 {
		last := fakeSender.Messages[len(fakeSender.Messages)-1].Text
		check("e2e message contains email", strings.Contains(last, "e2e@netflix.com"), last)
	} else {
		check("e2e message captured", false)
	}
}

func notify(ref, txnStatus, statusCode, gross, txnID, sigOverride string) (int, map[string]any) {
	sig := sigOverride
	if sig == "" {
		sig = payment.ComputeSignature(ref, statusCode, gross, serverKey)
	}
	return doJSON(http.MethodPost, "/webhooks/midtrans", "", map[string]any{
		"order_id":           ref,
		"transaction_id":     txnID,
		"transaction_status": txnStatus,
		"status_code":        statusCode,
		"gross_amount":       gross,
		"signature_key":      sig,
	})
}

func orderStatus(botTok, ref string) string {
	_, body := doJSON(http.MethodGet, "/api/v1/bot/orders/"+ref, botTok, nil)
	return fmt.Sprint(body["status"])
}
