//go:build desktop

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Brook-sys/picofarm/desktop/internal/autostart"
	shell "github.com/Brook-sys/picofarm/desktop/internal/shell"
	"github.com/Brook-sys/picofarm/desktop/internal/updates"
	"github.com/Brook-sys/picofarm/internal/api"
	"github.com/Brook-sys/picofarm/internal/appruntime"
	"github.com/Brook-sys/picofarm/internal/version"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/icons"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

func main() {
	if err := run(); err != nil {
		log.Print("Picofarm desktop: ", err)
		os.Exit(1)
	}
}
func run() error {
	var window *application.WebviewWindow
	// New acquires Wails' OS-backed single-instance lock synchronously, before
	// touching the database, encryption key or services. Secondary launches exit.
	app := application.New(application.Options{
		Name: "Picofarm", Description: "Picofarm desktop",
		SingleInstance: &application.SingleInstanceOptions{UniqueID: "com.picofarm.desktop", OnSecondInstanceLaunch: func(application.SecondInstanceData) {
			if window != nil {
				window.Show()
				window.Focus()
			}
		}},
		Mac: application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: false},
	})
	dir, err := shell.ConfigDir()
	if err != nil {
		return err
	}
	cfg, err := shell.LoadConfig(dir)
	if err != nil {
		return fmt.Errorf("load desktop config: %w", err)
	}
	key, err := shell.EncryptionKey(dir)
	if err != nil {
		return err
	}
	if err = os.Setenv("ENCRYPTION_KEY", key); err != nil {
		return err
	}
	secret, err := shell.RandomSecret()
	if err != nil {
		return err
	}
	bootstrap, err := shell.RandomSecret()
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	origin := "http://" + listener.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var target *url.URL
	token := cfg.RemoteToken
	if cfg.Mode == "local" {
		token, err = shell.RandomSecret()
		if err != nil {
			return err
		}
		backend, err := appruntime.Start(ctx, appruntime.Config{ListenAddress: "127.0.0.1:0", DatabasePath: filepath.Join(dir, "picofarm.db"), UploadDir: filepath.Join(dir, "uploads"), RouterOptions: api.RouterOptions{ApiToken: token, RequireApiToken: true, AllowedOrigins: []string{origin}}, ShutdownTimeout: 15 * time.Second})
		if err != nil {
			return fmt.Errorf("start local runtime: %w", err)
		}
		defer func() {
			shutdown, stop := context.WithTimeout(context.Background(), 20*time.Second)
			defer stop()
			if err := backend.Shutdown(shutdown); err != nil {
				log.Print("runtime shutdown: ", err)
			}
		}()
		target, _ = url.Parse(backend.URL())
	} else {
		target, err = shell.ValidateRemote(cfg.RemoteURL, cfg.AllowInsecureLoopback)
		if err != nil {
			return err
		}
	}
	ns := notifications.New()
	// A service is registered for lifecycle only; the loopback server does not
	// expose /wails bindings. Native access is the narrower desktop endpoints.
	app.RegisterService(application.NewService(ns))
	host := &shell.Host{Config: cfg, Assets: FrontendAssets(), Target: target, Token: token, Secret: secret, Bootstrap: bootstrap, Origin: origin,
		Callbacks: shell.Callbacks{Save: func(c shell.Config) error {
			executable, err := os.Executable()
			if err != nil {
				return err
			}
			previous, err := autostart.Enabled()
			if err != nil {
				return err
			}
			if err = autostart.Set(c.Autostart, executable); err != nil {
				return err
			}
			if err = shell.SaveConfig(dir, c); err != nil {
				_ = autostart.Set(previous, executable)
				return err
			}
			return nil
		}, OpenExternal: app.Browser.OpenURL, Notify: func(n shell.Notification) error {
			authorized, err := ns.RequestNotificationAuthorization()
			if err != nil {
				return err
			}
			if !authorized {
				return fmt.Errorf("notifications not authorized")
			}
			id, err := shell.RandomSecret()
			if err != nil {
				return err
			}
			return ns.SendNotification(notifications.NotificationOptions{ID: id, Title: n.Title, Body: n.Body})
		}},
	}
	defer host.CloseIdleConnections()
	server := &http.Server{Handler: host.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	defer func() {
		shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = server.Shutdown(shutdown)
	}()
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Print("desktop gateway stopped")
			app.Quit()
		}
	}()
	// Expire the one-shot bootstrap even when the webview fails to load.
	time.AfterFunc(time.Minute, host.ExpireBootstrap)
	window = app.Window.NewWithOptions(application.WebviewWindowOptions{Title: "Picofarm", Width: 1280, Height: 850, URL: origin + "/desktop/bootstrap?key=" + bootstrap, DevToolsEnabled: false})
	tray := app.SystemTray.New()
	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(icons.SystrayMacTemplate)
	} else {
		tray.SetIcon(icons.SystrayLight)
	}
	menu := app.NewMenu()
	menu.Add("Open Picofarm").OnClick(func(*application.Context) { window.Show(); window.Focus() })
	menu.Add("Desktop settings").OnClick(func(*application.Context) { window.SetURL(origin + "/desktop/setup"); window.Show(); window.Focus() })
	menu.Add("Check for updates").OnClick(func(*application.Context) {
		go func() {
			release, err := updates.Check(context.Background())
			message := "Current version: " + version.String() + "\n"
			if err != nil {
				message += "Release check failed. Try again later."
			} else if release == nil {
				message += "No published stable release found."
			} else {
				message += "Latest stable release: " + release.Tag + "\n" + release.URL + "\nUpdates must be downloaded and installed manually."
			}
			app.Dialog.Info().SetTitle("Picofarm releases").SetMessage(message).Show()
		}()
	})
	menu.Add("Quit").OnClick(func(*application.Context) { app.Quit() })
	tray.SetMenu(menu)
	tray.OnClick(func() { window.Show(); window.Focus() })
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		// Linux desktops may have no status-notifier host. Until availability can
		// be detected reliably, never make the application inaccessible there.
		if cfg.CloseToTray && runtime.GOOS != "linux" {
			window.Hide()
			e.Cancel()
		} else {
			app.Quit()
		}
	})
	return app.Run()
}
