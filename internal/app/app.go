package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalia/store/internal/config"
	"github.com/kalia/store/internal/handler"
	"github.com/kalia/store/internal/middleware"
	"github.com/kalia/store/internal/payment"
	"github.com/kalia/store/internal/repository"
	"github.com/kalia/store/internal/server"
	"github.com/kalia/store/internal/service"
	"github.com/kalia/store/internal/telegram"
	"github.com/kalia/store/pkg/token"
)

// Container holds long-lived application dependencies.
type Container struct {
	Cfg     *config.Config
	Log     *slog.Logger
	Pool    *pgxpool.Pool
	App     *fiber.App
	Tokens  *token.Manager
	Gateway payment.Gateway

	tx       *repository.TxManager
	services *services
}

// services groups the business-logic layer.
type services struct {
	auth     *service.AuthService
	product  *service.ProductService
	account  *service.AccountService
	telegram *service.TelegramService
	order    *service.OrderService
	payment  *service.PaymentService
	delivery *service.DeliveryService
	settings *service.SettingsService
}

// Build wires repositories, services, and handlers into a ready Fiber app.
// gateway/sender may be nil, in which case real Midtrans/Telegram clients are
// built from cfg.
func Build(cfg *config.Config, log *slog.Logger, pool *pgxpool.Pool, gateway payment.Gateway, sender service.CredentialSender) *Container {
	app := server.New(log, cfg.IsProduction(), cfg.CORSAllowedOrigins)
	tm := repository.NewTxManager(pool)
	tokens := token.NewManager(cfg.JWTSecret, cfg.JWTTTL)

	if gateway == nil {
		gateway = payment.NewMidtrans(cfg.Midtrans.ServerKey, cfg.Midtrans.BaseURL, cfg.Midtrans.DefaultAcquirer, nil)
	}
	if sender == nil {
		sender = telegram.NewClient(cfg.TelegramBotToken, nil)
	}

	productRepo := repository.NewProductRepository(tm.DB())
	accountRepo := repository.NewAccountRepository(tm.DB())

	paymentSvc := service.NewPaymentService(tm, gateway, log)
	deliverySvc := service.NewDeliveryService(tm, sender, log)
	// Wire automatic delivery on settlement.
	paymentSvc.SetDeliveryTrigger(deliverySvc)

	svcs := &services{
		auth:    service.NewAuthService(repository.NewAdminRepository(tm.DB()), tokens, log),
		product: service.NewProductService(productRepo, tm),
		account: service.NewAccountService(accountRepo, productRepo),
		telegram: service.NewTelegramService(
			repository.NewMenuRepository(tm.DB()),
			repository.NewResponseRepository(tm.DB()),
		),
		order:    service.NewOrderService(tm, gateway, cfg.Midtrans.DefaultAcquirer, cfg.ReservationTTL, cfg.PaymentTTL, log),
		payment:  paymentSvc,
		delivery: deliverySvc,
		settings: service.NewSettingsService(repository.NewSettingRepository(tm.DB())),
	}

	c := &Container{
		Cfg:      cfg,
		Log:      log,
		Pool:     pool,
		App:      app,
		Tokens:   tokens,
		Gateway:  gateway,
		tx:       tm,
		services: svcs,
	}
	c.registerRoutes()
	return c
}

// Bootstrap performs one-time startup tasks (e.g. seeding the initial admin).
func (c *Container) Bootstrap(ctx context.Context) error {
	return c.services.auth.Bootstrap(ctx, c.Cfg.AdminUsername, c.Cfg.AdminPassword)
}

// registerRoutes mounts all HTTP routes. Extended as modules are added.
func (c *Container) registerRoutes() {
	// Health probes (no auth).
	handler.NewHealthHandler(c.Pool).Register(c.App)

	// Public webhook (no JWT; signature-gated inside the service).
	handler.NewWebhookHandler(c.services.payment).Register(c.App)

	// API v1 root group; module routes attach here.
	api := c.App.Group("/api/v1")

	// Rate limiters: blunt brute force on login and order spam.
	loginLimiter := middleware.RateLimit(10, time.Minute)
	orderLimiter := middleware.RateLimit(20, time.Minute)

	handler.NewAuthHandler(c.services.auth).Register(api, c.Tokens, loginLimiter)

	// Admin routes: JWT applied per group (never at the /api/v1 root, so /bot
	// stays outside the JWT umbrella).
	jwt := middleware.JWTAuth(c.Tokens)
	handler.NewProductHandler(c.services.product).Register(api, jwt)
	handler.NewAccountHandler(c.services.account).Register(api, jwt)

	tgHandler := handler.NewTelegramHandler(c.services.telegram)
	tgHandler.RegisterAdmin(api, jwt)
	handler.NewDeliveryHandler(c.services.delivery).Register(api, jwt)
	handler.NewOrderHandler(c.services.order, c.services.payment).Register(api, jwt)
	handler.NewSettingsHandler(c.services.settings).Register(api, jwt)

	// Bot routes protected by the static service token.
	bot := api.Group("/bot", middleware.BotAuth(c.Cfg.BotServiceToken))
	tgHandler.RegisterBot(bot)
	handler.NewBotHandler(c.services.product, c.services.account, c.services.order).Register(bot, orderLimiter)

	// Local-dev only: manual settlement helper when using the fake gateway.
	if c.Cfg.PaymentFake() {
		handler.NewDevHandler(c.services.payment).Register(api)
		c.Log.Warn("PAYMENT_MODE=fake: dev settlement endpoint enabled at /api/v1/dev/settle/:order_ref — DO NOT use in production")
	}
}

// Close releases resources held by the container.
func (c *Container) Close(_ context.Context) {
	if c.Pool != nil {
		c.Pool.Close()
	}
}
