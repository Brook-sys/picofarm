// Package appruntime owns the shared HTTP application lifecycle. It has no
// dependency on a desktop framework and never exits the hosting process.
package appruntime

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/Brook-sys/picofarm/internal/api"
	"github.com/Brook-sys/picofarm/internal/database"
	"github.com/Brook-sys/picofarm/internal/etsy"
	"github.com/Brook-sys/picofarm/internal/service"
)

// Config is a snapshot of startup configuration. Start does not load .env files
// or change process environment. Other services still read their documented
// environment overrides (for example encryption and slicer connection settings).
// Desktop hosts should use an explicit loopback ListenAddress, normally
// "127.0.0.1:0", and persistent absolute DatabasePath and UploadDir paths.
type Config struct {
	ListenAddress   string
	DatabasePath    string
	UploadDir       string
	StaticDir       string
	RouterOptions   api.RouterOptions
	ServicesConfig  service.ServicesConfig
	ReconnectConfig service.PrinterReconnectConfig
	ShutdownTimeout time.Duration
}

// ConfigFromEnv preserves the standalone web server's environment defaults.
// Loading an optional .env file remains the responsibility of the entry point.
func ConfigFromEnv() (Config, error) {
	reconnect, err := service.PrinterReconnectConfigFromEnv()
	if err != nil {
		return Config{}, err
	}
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath, err = database.DefaultDBPath()
		if err != nil {
			return Config{}, fmt.Errorf("default database path: %w", err)
		}
	}
	return Config{
		ListenAddress: net.JoinHostPort("", env("PORT", "8084")),
		DatabasePath:  dbPath,
		UploadDir:     env("UPLOAD_DIR", "./uploads"),
		StaticDir:     env("STATIC_DIR", "./web/dist"),
		RouterOptions: api.RouterOptionsFromEnv(),
		ServicesConfig: service.ServicesConfig{Etsy: service.EtsyConfig{
			ClientID: os.Getenv("ETSY_CLIENT_ID"), SharedSecret: os.Getenv("ETSY_SHARED_SECRET"),
			RedirectURI: env("ETSY_REDIRECT_URI", etsy.DefaultRedirectURI), WebhookSecret: os.Getenv("ETSY_WEBHOOK_SECRET"),
		}},
		ReconnectConfig: reconnect,
		ShutdownTimeout: 30 * time.Second,
	}, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
