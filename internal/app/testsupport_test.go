package app

import (
	"context"
	"os"
	"testing"

	"github.com/kalia/store/internal/config"
	"github.com/kalia/store/internal/database"
	"github.com/kalia/store/pkg/logger"
)

// newTestContainer builds a fully-wired Container against the test database.
// The database DSN is taken from KALIA_TEST_DB; the test is skipped when unset
// so unit-only CI runs stay green without a database.
func newTestContainer(t *testing.T) *Container {
	t.Helper()

	dsn := os.Getenv("KALIA_TEST_DB")
	if dsn == "" {
		t.Skip("KALIA_TEST_DB not set; skipping database integration test")
	}

	if err := database.Migrate(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	cfg := &config.Config{
		AppEnv:          "test",
		JWTSecret:       "integration-test-secret",
		JWTTTL:          3600_000_000_000, // 1h in ns
		BotServiceToken: "test-bot-token",
		DatabaseURL:     dsn,
		ReservationTTL:  15 * 60_000_000_000, // 15m in ns
		PaymentTTL:      15 * 60_000_000_000, // 15m in ns
	}
	cfg.Midtrans.ServerKey = testServerKey
	cfg.Midtrans.DefaultAcquirer = "gopay"
	log := logger.New("error")
	return Build(cfg, log, pool, newMockGateway(), newMockSender())
}
