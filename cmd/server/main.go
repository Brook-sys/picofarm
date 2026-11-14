package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Brook-sys/picofarm/internal/api"
	"github.com/Brook-sys/picofarm/internal/database"
	"github.com/Brook-sys/picofarm/internal/etsy"
	"github.com/Brook-sys/picofarm/internal/printer"
	"github.com/Brook-sys/picofarm/internal/realtime"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/Brook-sys/picofarm/internal/storage"
	"github.com/Brook-sys/picofarm/internal/version"
	"github.com/getsentry/sentry-go"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if present
	_ = godotenv.Load()

	// Initialize Sentry for crash reporting
	if dsn := os.Getenv("SENTRY_DSN"); dsn != "" {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              dsn,
			Release:          version.Version,
			Environment:      getEnv("ENVIRONMENT", "development"),
			TracesSampleRate: 0.2,
		}); err != nil {
			slog.Error("failed to initialize Sentry", "error", err)
		} else {
			slog.Info("Sentry initialized", "environment", getEnv("ENVIRONMENT", "development"))
			defer sentry.Flush(2 * time.Second)
		}
	}

	// Configure structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Get configuration from environment
	port := getEnv("PORT", "8084")
	uploadDir := getEnv("UPLOAD_DIR", "./uploads")

	// Etsy OAuth configuration (optional)
	etsyClientID := os.Getenv("ETSY_CLIENT_ID")
	// Same single source of truth the service uses (etsy.DefaultRedirectURI):
	// two hardcoded defaults drifted apart in the past and the service shipped
	// port 8080, which is not the port this app listens on.
	etsyRedirectURI := getEnv("ETSY_REDIRECT_URI", etsy.DefaultRedirectURI)
	// Etsy webhook signing secret (optional). When set, webhook events without a
	// valid X-Etsy-Signature are rejected with 401 instead of accepted.
	etsyWebhookSecret := os.Getenv("ETSY_WEBHOOK_SECRET")

	// Open SQLite database
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		var err error
		dbPath, err = database.DefaultDBPath()
		if err != nil {
			slog.Error("failed to get default database path", "error", err)
			os.Exit(1)
		}
	}

	db, err := database.Open(dbPath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize storage
	fileStorage := storage.NewLocalStorage(uploadDir)

	// Initialize repositories
	repos := repository.NewRepositories(db)

	// Initialize WebSocket hub for real-time updates.
	// Run() starts after the router is built: the router configures the hub's
	// auth and origin policy, so those fields must be written before anything
	// can read them.
	hub := realtime.NewHub()

	// Initialize printer manager with hub for broadcasting state changes
	printerManager := printer.NewManager()
	printerManager.SetBroadcaster(hub)

	// Initialize services
	servicesConfig := service.ServicesConfig{
		Etsy: service.EtsyConfig{
			ClientID:      etsyClientID,
			RedirectURI:   etsyRedirectURI,
			WebhookSecret: etsyWebhookSecret,
		},
	}
	services := service.NewServicesWithConfig(repos, fileStorage, printerManager, hub, servicesConfig)

	// Initialize backup service with database access
	backupService := service.NewBackupService(db, dbPath)
	services.SetBackupService(backupService)
	backupService.SetSettingsService(services.Settings)
	backupService.StartScheduler()

	// Initialize PrintJobService to register for printer status changes (auto failure detection)
	services.PrintJobs.Init()

	// Reconnect all saved printers at startup
	services.Printers.ConnectAllPrinters(context.Background())

	// Keep retrying printers that are still offline. Without this call the
	// reconnect loop existed but was never started, so a printer that missed
	// its initial connection stayed offline until manually reconnected.
	services.Printers.StartAutoReconnect(context.Background())

	if etsyClientID != "" {
		slog.Info("Etsy integration enabled", "redirect_uri", etsyRedirectURI)
	} else {
		// Try loading Etsy config from database settings
		if setting, err := services.Settings.Get(context.Background(), "etsy_client_id"); err == nil && setting != nil && setting.Value != "" {
			// Older builds persisted the wrong default redirect URI (port
			// 8080). Rewrite it before the client is built, otherwise the
			// authorization keeps failing with "The requested redirect URL is
			// not permitted." and nothing in the UI can correct it.
			if err := services.Etsy.RepairLegacyRedirectURI(context.Background()); err != nil {
				slog.Warn("failed to repair legacy Etsy redirect URI", "error", err)
			}
			redirectURI := etsyRedirectURI
			if uriSetting, err := services.Settings.Get(context.Background(), service.EtsyRedirectURIKey); err == nil && uriSetting != nil && uriSetting.Value != "" {
				redirectURI = uriSetting.Value
			}
			if err := services.Etsy.Configure(context.Background(), setting.Value, redirectURI); err != nil {
				slog.Warn("failed to load Etsy config from database", "error", err)
			} else {
				slog.Info("Etsy integration enabled from stored config", "redirect_uri", services.Etsy.RedirectURI())
			}
		}
	}

	// Initialize HTTP router
	router := api.NewRouter(services, hub)

	// Start the hub now that the router has configured its auth/origin policy.
	go hub.Run()

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second, // Long timeout for network scanning
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info("starting server", "version", version.String(), "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server first (stop accepting new requests)
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}
	slog.Info("HTTP server stopped")

	// Stop backup scheduler
	backupService.StopScheduler()

	// Disconnect all printers (closes MQTT connections)
	printerManager.DisconnectAll()

	// Stop WebSocket hub (closes all client connections)
	hub.Stop()

	slog.Info("shutdown complete")
}

// getEnv returns environment variable or default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
