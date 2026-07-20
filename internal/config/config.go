package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration, loaded from environment variables.
type Config struct {
	AppEnv   string
	HTTPPort string
	LogLevel string

	DatabaseURL string

	JWTSecret string
	JWTTTL    time.Duration

	// Admin bootstrap credentials (seeded on first startup only).
	AdminUsername string
	AdminPassword string

	// BotServiceToken authenticates the Telegram bot against /bot/* endpoints.
	BotServiceToken string

	Midtrans MidtransConfig

	// ReservationTTL is how long an account stays RESERVED before cleanup.
	ReservationTTL time.Duration
	// PaymentTTL is how long a QRIS charge is valid before it expires.
	PaymentTTL time.Duration

	// PaymentMode: "midtrans" (default) or "fake" (local dev only).
	PaymentMode string

	// CORSAllowedOrigins is a comma-separated list of allowed origins for the
	// admin dashboard SPA ("*" for any).
	CORSAllowedOrigins string

	// Worker (background jobs) tunables.
	Worker WorkerConfig

	// TelegramBotAPI is the base URL the backend calls to deliver credentials
	// (Telegram Bot API or an internal bot service).
	TelegramBotToken string
}

// WorkerConfig holds background-job scheduling parameters.
type WorkerConfig struct {
	PollInterval          time.Duration // reconciler cadence
	CleanupInterval       time.Duration // expiry/reservation cleanup cadence
	DeliveryRetryInterval time.Duration // failed-delivery retry cadence
	PollMinAge            time.Duration // only reconcile PENDING orders older than this
	MaxDeliveryAttempts   int           // stop retrying after this many attempts
	BatchLimit            int           // rows processed per job tick
}

// MidtransConfig holds Midtrans Core API settings.
type MidtransConfig struct {
	ServerKey       string
	BaseURL         string // https://api.sandbox.midtrans.com or https://api.midtrans.com
	DefaultAcquirer string // gopay | shopeepay
}

// PaymentMode selects the payment gateway implementation. "midtrans" (default)
// uses the real Core API; "fake" uses an in-process stub for local development
// so the bot flow can be exercised without a Midtrans account. NEVER use "fake"
// in production.
func (c *Config) PaymentFake() bool { return c.PaymentMode == "fake" }

// Load reads configuration from the environment (and an optional .env file).
func Load() (*Config, error) {
	// .env is best-effort; production supplies real env vars.
	_ = godotenv.Load()

	cfg := &Config{
		AppEnv:           getEnv("APP_ENV", "development"),
		HTTPPort:         getEnv("HTTP_PORT", "8080"),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		DatabaseURL:      getEnv("DATABASE_URL", ""),
		JWTSecret:        getEnv("JWT_SECRET", ""),
		JWTTTL:           getEnvDuration("JWT_TTL", 24*time.Hour),
		AdminUsername:    getEnv("ADMIN_USERNAME", ""),
		AdminPassword:    getEnv("ADMIN_PASSWORD", ""),
		BotServiceToken:  getEnv("BOT_SERVICE_TOKEN", ""),
		ReservationTTL:   getEnvDuration("RESERVATION_TTL", 15*time.Minute),
		PaymentTTL:       getEnvDuration("PAYMENT_TTL", 15*time.Minute),
		PaymentMode:      getEnv("PAYMENT_MODE", "midtrans"),
		CORSAllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", "*"),
		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
		Worker: WorkerConfig{
			PollInterval:          getEnvDuration("WORKER_POLL_INTERVAL", 90*time.Second),
			CleanupInterval:       getEnvDuration("WORKER_CLEANUP_INTERVAL", 2*time.Minute),
			DeliveryRetryInterval: getEnvDuration("WORKER_DELIVERY_RETRY_INTERVAL", 3*time.Minute),
			PollMinAge:            getEnvDuration("WORKER_POLL_MIN_AGE", 2*time.Minute),
			MaxDeliveryAttempts:   getEnvInt("WORKER_MAX_DELIVERY_ATTEMPTS", 5),
			BatchLimit:            getEnvInt("WORKER_BATCH_LIMIT", 100),
		},
		Midtrans: MidtransConfig{
			ServerKey:       getEnv("MIDTRANS_SERVER_KEY", ""),
			BaseURL:         getEnv("MIDTRANS_BASE_URL", "https://api.sandbox.midtrans.com"),
			DefaultAcquirer: getEnv("MIDTRANS_ACQUIRER", "gopay"),
		},
	}

	return cfg, nil
}

// Validate ensures required secrets are present. Called by binaries that need them.
func (c *Config) Validate() error {
	var missing []string
	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if c.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if c.BotServiceToken == "" {
		missing = append(missing, "BOT_SERVICE_TOKEN")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %v", missing)
	}
	return nil
}

// IsProduction reports whether the app runs in production mode.
func (c *Config) IsProduction() bool { return c.AppEnv == "production" }

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		// Support both Go duration strings ("15m") and plain seconds ("900").
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		if secs, err := strconv.Atoi(v); err == nil {
			return time.Duration(secs) * time.Second
		}
	}
	return fallback
}
