//go:build desktop && darwin && !qualification

package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/endpoint"

	"github.com/jenksed/yourmove-go/internal/app"
)

// macOS updater wiring for the shared desktop composition. Windows stays a
// no-op (see desktop_updater_windows.go); no Windows updater is in scope.

// Build-time pinned update trust root and channel (set via ldflags for
// release candidates; empty in normal dev runs). Env vars provide the
// controlled dev fixture channel without touching production pins.
var (
	updaterManifestURLEmbedded string
	updaterPublicKeyEmbedded   string
)

// configureDesktopUpdater wires the Wails updater lifecycle for macOS
// without blocking startup. Called once from runDesktop after the Wails
// application exists.
func configureDesktopUpdater(a *app.App, desktop *application.App) {
	configureUpdater(a, desktop)
}

// startDesktopUpdateTasks starts the macOS-only updater tasks: the explicit
// dev-fixture auto trigger (build-tag gated, never production) and the
// download-never background availability check.
func startDesktopUpdateTasks(a *app.App, desktop *application.App) {
	startUpdateTestAuto(a, desktop)
	startUpdateBackground(a)
}

// configureUpdater wires the Wails updater lifecycle without blocking startup.
// Headless (WindowNone): discovery and progress surface through the Utilities
// page and Wails events, never an unexpected modal. Explicit applicant action
// is required for download, install, and restart.
func configureUpdater(a *app.App, desktop *application.App) {
	manifestURL := strings.TrimSpace(updaterManifestURLEmbedded)
	if manifestURL == "" {
		manifestURL = updaterEnvManifestURL()
	}
	if manifestURL == "" {
		a.SetUpdateController(app.UpdateController{})
		log.Printf("automatic updates disabled: no update manifest URL pinned (manual DMG remains available)")
		return
	}
	publicKey := loadUpdatePublicKey()
	if len(publicKey) == 0 {
		a.SetUpdateController(app.UpdateController{})
		log.Printf("automatic updates disabled: no update public key configured (manual DMG remains available)")
		return
	}
	provider, err := endpoint.New(endpoint.Config{URL: manifestURL, Channel: "stable"})
	if err != nil {
		a.SetUpdateController(app.UpdateController{})
		log.Printf("automatic updates disabled: invalid manifest URL: %v", err)
		return
	}
	if err := desktop.Updater.Init(updater.Config{
		CurrentVersion: app.CurrentAppVersion(),
		Providers:      []updater.Provider{provider},
		PublicKey:      publicKey,
		Window:         updater.WindowNone,
	}); err != nil {
		a.SetUpdateController(app.UpdateController{})
		log.Printf("automatic updates disabled: updater init failed: %v", err)
		return
	}
	// Surface lifecycle transitions without applicant material.
	desktop.Event.On(updater.EventError, func(e *application.CustomEvent) {
		if info, ok := e.Data.(updater.ErrorInfo); ok {
			log.Printf("update error stage=%s message=%.300s", info.Stage, info.Message)
		} else {
			log.Printf("update error (untyped payload)")
		}
	})
	desktop.Event.On(updater.EventUpdateAvailable, func(e *application.CustomEvent) {
		if rel, ok := e.Data.(*updater.Release); ok && rel != nil {
			log.Printf("update available: current %s latest %s", app.CurrentAppVersion(), rel.Version)
		}
	})
	a.SetUpdateController(app.UpdateController{
		Check: func(ctx context.Context) (app.UpdateCheckResult, error) {
			rel, err := desktop.Updater.Check(ctx)
			if err != nil {
				return app.UpdateCheckResult{}, err
			}
			if rel == nil {
				return app.UpdateCheckResult{Available: false, CurrentVersion: app.CurrentAppVersion()}, nil
			}
			return app.UpdateCheckResult{
				Available:      true,
				CurrentVersion: app.CurrentAppVersion(),
				LatestVersion:  rel.Version,
				Name:           rel.Name,
				Notes:          rel.Notes,
				Size:           rel.Artifact.Size,
			}, nil
		},
		Install: func(ctx context.Context) error { return desktop.Updater.DownloadAndInstall(ctx) },
		Restart: func(ctx context.Context) error { return desktop.Updater.Restart(ctx) },
	})
	log.Printf("automatic updates enabled: manifest configured, current %s (channel stable, headless UI)", app.CurrentAppVersion())
}

func loadUpdatePublicKey() []byte {
	if trimmed := strings.TrimSpace(updaterPublicKeyEmbedded); trimmed != "" {
		trimmed = strings.ReplaceAll(trimmed, `\n`, "\n")
		return []byte(trimmed)
	}
	return updaterEnvPublicKey()
}

// startUpdateTestAuto drives the real Wails updater path for the controlled
// dev fixture only. It requires explicit YOURMOVE_UPDATE_TEST_AUTO=1 and
// performs the same Check, DownloadAndInstall, and Restart calls the
// Utilities buttons make. Production builds ignore the variable entirely
// (see desktop_updater_env_prod.go), so it can never trigger there.
func startUpdateTestAuto(a *app.App, desktop *application.App) {
	if !updateTestAutoEnabled() {
		return
	}
	go func() {
		time.Sleep(5 * time.Second)
		log.Printf("update test auto: explicit fixture trigger, checking for update")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		rel, err := desktop.Updater.Check(ctx)
		cancel()
		if err != nil {
			log.Printf("update test auto: check failed: %v", err)
			return
		}
		if rel == nil {
			log.Printf("update test auto: no update available")
			return
		}
		log.Printf("update test auto: found version %s, installing (explicit fixture approval)", rel.Version)
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Minute)
		if err := desktop.Updater.DownloadAndInstall(ctx2); err != nil {
			cancel2()
			log.Printf("update test auto: install failed: %v", err)
			return
		}
		cancel2()
		log.Printf("update test auto: staged ready, restarting into new version")
		ctx3, cancel3 := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel3()
		if err := desktop.Updater.Restart(ctx3); err != nil {
			log.Printf("update test auto: restart failed: %v", err)
			return
		}
	}()
}

// startUpdateBackground performs one delayed nonblocking check after startup,
// then polls daily. It never opens a window and never downloads.
func startUpdateBackground(a *app.App) {
	delay := 30 * time.Second
	if updateBGFastEnabled() {
		delay = 3 * time.Second
	}
	go func() {
		select {
		case <-time.After(delay):
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		a.BackgroundUpdateCheck(ctx)
		cancel()
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			a.BackgroundUpdateCheck(ctx)
			cancel()
		}
	}()
}
