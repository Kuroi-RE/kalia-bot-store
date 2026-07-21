package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kalia/store/internal/app"
	"github.com/kalia/store/internal/config"
	"github.com/kalia/store/internal/database"
	"github.com/kalia/store/internal/payment"
	"github.com/kalia/store/internal/testkit"
	"github.com/kalia/store/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logger.New(cfg.LogLevel)

	if err := cfg.Validate(); err != nil {
		log.Error("invalid configuration", slog.Any("error", err))
		os.Exit(1)
	}

	ctx := context.Background()

	// Apply pending migrations before serving traffic.
	if err := database.Migrate(cfg.DatabaseURL); err != nil {
		log.Error("migrations failed", slog.Any("error", err))
		os.Exit(1)
	}

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database connection failed", slog.Any("error", err))
		os.Exit(1)
	}

	// Payment gateway: fake (dev), or the configured provider (midtrans/temanqris).
	var gateway payment.Gateway
	if cfg.PaymentFake() {
		log.Warn("PAYMENT_MODE=fake — using in-process fake payment gateway (DEV ONLY)")
		gateway = testkit.NewFakeGateway(cfg.Midtrans.ServerKey)
	} else {
		gateway = payment.NewGateway(payment.ProviderConfig{
			Provider:        cfg.PaymentProvider,
			MidtransKey:     cfg.Midtrans.ServerKey,
			MidtransBaseURL: cfg.Midtrans.BaseURL,
			Acquirer:        cfg.Midtrans.DefaultAcquirer,
			QRISStatic:      cfg.QRISStatic,
		}, nil)
		log.Info("payment provider", slog.String("provider", gateway.Name()))
	}

	container := app.Build(cfg, log, pool, gateway, nil)

	if err := container.Bootstrap(ctx); err != nil {
		log.Error("bootstrap failed", slog.Any("error", err))
		os.Exit(1)
	}

	// Start server.
	go func() {
		addr := ":" + cfg.HTTPPort
		log.Info("api server starting", slog.String("addr", addr))
		if err := container.App.Listen(addr); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("server stopped", slog.Any("error", err))
		}
	}()

	// Graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_ = container.App.ShutdownWithContext(shutdownCtx)
	container.Close(shutdownCtx)
	log.Info("shutdown complete")
}
