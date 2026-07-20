package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/kalia/store/internal/config"
	"github.com/kalia/store/internal/database"
	"github.com/kalia/store/internal/payment"
	"github.com/kalia/store/internal/repository"
	"github.com/kalia/store/internal/service"
	"github.com/kalia/store/internal/telegram"
	"github.com/kalia/store/internal/worker"
	"github.com/kalia/store/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logger.New(cfg.LogLevel)

	if cfg.DatabaseURL == "" {
		log.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database connection failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Close()

	tm := repository.NewTxManager(pool)
	gateway := payment.NewMidtrans(cfg.Midtrans.ServerKey, cfg.Midtrans.BaseURL, cfg.Midtrans.DefaultAcquirer, nil)
	sender := telegram.NewClient(cfg.TelegramBotToken, nil)

	paymentSvc := service.NewPaymentService(tm, gateway, log)
	deliverySvc := service.NewDeliveryService(tm, sender, log)
	paymentSvc.SetDeliveryTrigger(deliverySvc)
	orderSvc := service.NewOrderService(tm, gateway, cfg.Midtrans.DefaultAcquirer, cfg.ReservationTTL, cfg.PaymentTTL, log)

	sched := worker.NewScheduler(log)
	worker.Register(sched, worker.Deps{
		Payment:  paymentSvc,
		Order:    orderSvc,
		Delivery: deliverySvc,
		Cfg:      cfg,
		Log:      log,
	})

	log.Info("worker starting",
		slog.Duration("poll", cfg.Worker.PollInterval),
		slog.Duration("cleanup", cfg.Worker.CleanupInterval),
		slog.Duration("delivery_retry", cfg.Worker.DeliveryRetryInterval))

	sched.Run(ctx)
	log.Info("worker shutdown complete")
}
