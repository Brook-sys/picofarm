package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Brook-sys/picofarm/internal/appruntime"
	"github.com/Brook-sys/picofarm/internal/version"
	"github.com/getsentry/sentry-go"
	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()
	cfg, err := appruntime.ConfigFromEnv()
	if err != nil {
		return err
	}

	if dsn := os.Getenv("SENTRY_DSN"); dsn != "" {
		environment := os.Getenv("ENVIRONMENT")
		if environment == "" {
			environment = "development"
		}
		if err := sentry.Init(sentry.ClientOptions{
			Dsn: dsn, Release: version.Version, Environment: environment, TracesSampleRate: 0.2,
		}); err != nil {
			slog.Error("failed to initialize Sentry", "error", err)
		} else {
			slog.Info("Sentry initialized", "environment", environment)
			defer sentry.Flush(2 * time.Second)
		}
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	app, err := appruntime.Start(ctx, cfg)
	if err != nil {
		return err
	}
	return app.Wait()
}
