package appruntime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Brook-sys/picofarm/internal/api"
	"github.com/Brook-sys/picofarm/internal/database"
	"github.com/Brook-sys/picofarm/internal/printer"
	"github.com/Brook-sys/picofarm/internal/realtime"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/Brook-sys/picofarm/internal/storage"
	"github.com/Brook-sys/picofarm/internal/version"
)

// Runtime is a running application. All lifecycle methods are safe to call
// concurrently. Done closes only after owned resources have been cleaned up.
type Runtime struct {
	server   *http.Server
	address  net.Addr
	cancel   context.CancelFunc
	stopOnce sync.Once
	stop     chan struct{}
	done     chan struct{}
	err      error // written before closing done; read only after done
}

// Start binds the listener synchronously and initializes the application. On
// error it releases resources already acquired. A successful return means the
// listener is bound and HTTP serving has been scheduled, not that external
// printers or integrations have connected. Parent cancellation initiates cleanup.
func Start(parent context.Context, cfg Config) (_ *Runtime, err error) {
	if parent == nil {
		return nil, errors.New("runtime: nil context")
	}
	if err := parent.Err(); err != nil {
		return nil, err
	}
	if cfg.ListenAddress == "" {
		return nil, errors.New("runtime: ListenAddress is required")
	}
	if cfg.DatabasePath == "" || cfg.UploadDir == "" {
		return nil, errors.New("runtime: DatabasePath and UploadDir are required")
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 30 * time.Second
	}
	if cfg.ShutdownTimeout < 0 {
		return nil, errors.New("runtime: ShutdownTimeout must be positive")
	}
	if cfg.ReconnectConfig == (service.PrinterReconnectConfig{}) {
		cfg.ReconnectConfig = service.DefaultPrinterReconnectConfig()
	}
	if err := cfg.ReconnectConfig.Validate(); err != nil {
		return nil, err
	}

	// Reserve the address before database/service side effects. In particular a
	// second desktop instance must not run migrations before noticing a bind error.
	listener, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	ctx, cancel := context.WithCancel(parent)
	cleanups := []func(){cancel, func() { _ = listener.Close() }}
	defer func() {
		if err != nil {
			for i := len(cleanups) - 1; i >= 0; i-- {
				cleanups[i]()
			}
		}
	}()
	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	cleanups = append(cleanups, func() { _ = db.Close() })
	repos := repository.NewRepositories(db)
	hub := realtime.NewHub()
	manager := printer.NewManager()
	manager.SetBroadcaster(hub)
	services := service.NewServicesWithConfig(repos, storage.NewLocalStorage(cfg.UploadDir), manager, hub, cfg.ServicesConfig)
	cleanups = append(cleanups, services.Dispatcher.Stop)
	if err = services.Printers.ConfigureReconnect(cfg.ReconnectConfig); err != nil {
		return nil, err
	}
	cleanups = append(cleanups, services.Printers.StopReconnect)
	services.SlicerJobs, err = service.NewSlicerJobsService(ctx, services.Settings, services.Slicer)
	if err != nil {
		return nil, fmt.Errorf("initialize slicer jobs: %w", err)
	}
	cleanups = append(cleanups, services.SlicerJobs.Close)
	backup := service.NewBackupService(db, cfg.DatabasePath)
	services.SetBackupService(backup)
	backup.SetSettingsService(services.Settings)
	services.PrintJobs.Init()
	loadEtsy(ctx, services, cfg.ServicesConfig.Etsy)

	// Router configures mutable hub auth/origin fields before Run can read them.
	opts := cfg.RouterOptions
	opts.StaticDir = cfg.StaticDir
	router := api.NewRouterWithOptions(services, hub, opts)
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	hubDone := make(chan struct{})
	go func() { defer close(hubDone); hub.Run() }()
	backup.StartScheduler()
	services.Printers.ConnectAllPrinters(ctx)
	services.Printers.StartAutoReconnect(ctx)

	r := &Runtime{
		server: &http.Server{Addr: cfg.ListenAddress, Handler: router,
			ReadTimeout: 30 * time.Second, WriteTimeout: 120 * time.Second, IdleTimeout: 60 * time.Second,
			BaseContext: func(net.Listener) context.Context { return ctx },
		},
		address: listener.Addr(), cancel: cancel, stop: make(chan struct{}), done: make(chan struct{}),
	}
	serveDone := make(chan error, 1)
	go func() {
		slog.Info("starting server", "version", version.String(), "address", r.address.String())
		serveDone <- r.server.Serve(listener)
	}()
	go func() {
		defer close(r.done)
		var serveErr error
		select {
		case <-ctx.Done():
		case <-r.stop:
		case serveErr = <-serveDone:
		}
		slog.Info("shutting down server")
		// Stop new HTTP work first. Cancellation propagates to background workers
		// and request contexts; forced Close handles expired graceful shutdown.
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		shutdownErr := r.server.Shutdown(shutdownCtx)
		shutdownCancel()
		if shutdownErr != nil {
			_ = r.server.Close()
		}
		if serveErr == nil {
			serveErr = <-serveDone
		}
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		backup.StopScheduler()
		services.Printers.StopReconnect()
		manager.DisconnectAll()
		services.Dispatcher.Stop()
		services.SlicerJobs.Close()
		hub.Stop()
		<-hubDone
		dbErr := db.Close()
		r.err = errors.Join(serveErr, shutdownErr, dbErr)
		slog.Info("shutdown complete")
	}()
	return r, nil
}

// Addr is the actual bound address, including an OS-selected ephemeral port.
func (r *Runtime) Addr() net.Addr { return r.address }

// URL returns the local HTTP base URL. Wildcard listeners use loopback as a
// client destination; this does not make such listeners local-only.
func (r *Runtime) URL() string {
	host, port, _ := net.SplitHostPort(r.address.String())
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func (r *Runtime) Done() <-chan struct{} { return r.done }

// Wait waits for complete cleanup and returns serving/shutdown/close errors.
// Normal parent cancellation is a successful shutdown, not a serving error.
func (r *Runtime) Wait() error { <-r.done; return r.err }

// Shutdown initiates cleanup exactly once and waits up to ctx's deadline. A
// timed-out caller does not abort cleanup; Wait can be used to observe its result.
// ShutdownTimeout bounds HTTP draining, not third-party device disconnects.
func (r *Runtime) Shutdown(ctx context.Context) error {
	r.stopOnce.Do(func() { close(r.stop) })
	select {
	case <-r.done:
		return r.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close is an alias for Shutdown for embedded hosts.
func (r *Runtime) Close(ctx context.Context) error { return r.Shutdown(ctx) }

// loadEtsy retains the standalone bootstrap's stored-config fallback and legacy
// callback repair. An incomplete environment pair intentionally does not fall
// back to stored credentials.
func loadEtsy(ctx context.Context, services *service.Services, cfg service.EtsyConfig) {
	if cfg.ClientID != "" && cfg.SharedSecret != "" {
		slog.Info("Etsy integration enabled", "redirect_uri", cfg.RedirectURI)
	} else if cfg.ClientID != "" || cfg.SharedSecret != "" {
		slog.Warn("Etsy environment configuration incomplete; both ETSY_CLIENT_ID and ETSY_SHARED_SECRET are required")
	} else if setting, err := services.Settings.Get(ctx, "etsy_client_id"); err == nil && setting != nil && setting.Value != "" {
		if err := services.Etsy.RepairLegacyRedirectURI(ctx); err != nil {
			slog.Warn("failed to repair legacy Etsy redirect URI", "error", err)
		}
		redirectURI := cfg.RedirectURI
		if setting, err := services.Settings.Get(ctx, service.EtsyRedirectURIKey); err == nil && setting != nil && setting.Value != "" {
			redirectURI = setting.Value
		}
		if err := services.Etsy.Configure(ctx, setting.Value, "", redirectURI); err != nil {
			slog.Warn("failed to load Etsy config from database", "error", err)
		} else {
			slog.Info("Etsy integration enabled from stored config", "redirect_uri", services.Etsy.RedirectURI())
		}
	}
}
