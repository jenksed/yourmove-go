//go:build desktop && darwin && !qualification

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/endpoint"

	"github.com/jenksed/yourmove-go/internal/app"
	"github.com/jenksed/yourmove-go/internal/storage"
)

// Wails is a host for the same Go App and HTTP routes as browser mode.
// Templates and static assets are embedded; no repository working directory is needed.
func main() {
	// Updater helper mode must not open the applicant workspace. The Wails
	// application constructor performs the staged swap and relaunches.
	if os.Getenv("WAILS_UPDATER_HELPER") == "1" {
		application.New(application.Options{Name: "YourMove"})
		return
	}
	dbPath, resumeDir, err := storage.ApplicationStorage()
	if err != nil {
		log.Fatal(err)
	}
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	a, err := app.New(app.Config{
		DB: db, ResumeDir: resumeDir, TemplateFiles: templateFiles,
		StaticFiles: staticFiles, FrontendDist: frontendDist, CoverLetterInstructions: coverLetterInstructions, ApplicationAnswerInstructions: applicationAnswerInstructions,
		PickResume: func() (string, error) {
			return application.Get().Dialog.OpenFile().SetTitle("Select a resume (PDF or DOCX)").AddFilter("Resumes", "*.pdf;*.docx").PromptForSingleSelection()
		}, ChooseExportPath: func(name string) (string, error) {
			return application.Get().Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
				Title: "Save a copy", Filename: name, CanCreateDirectories: true,
			}).PromptForSingleSelection()
		}, OpenPosting: func(url string) error {
			return application.Get().Browser.OpenURL(url)
		}})
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()
	handler := a.DesktopHandler()
	start := "/jobs"
	if os.Getenv("YOURMOVE_WAILS_PROBE") == "1" {
		handler = probeMux(handler)
		start = "/__wails_probe"
	}

	desktop := application.New(application.Options{
		Name:   "YourMove",
		Assets: application.AssetOptions{Handler: handler},
	})
	configureUpdater(a, desktop)
	startUpdateTestAuto(a, desktop)
	desktop.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "YourMove",
		Width:     1100,
		Height:    800,
		MinWidth:  700,
		MinHeight: 550,
		URL:       start,
	})
	startUpdateBackground(a)
	log.Printf("YourMove Wails compatibility run starting at %s (version %s build %s)", start, app.CurrentAppVersion(), app.CurrentAppBuild())
	if err := desktop.Run(); err != nil {
		log.Fatal(err)
	}
}

// Build-time pinned update trust root and channel (set via ldflags for
// release candidates; empty in normal dev runs). Env vars provide the
// controlled dev fixture channel without touching production pins.
var (
	updaterManifestURLEmbedded string
	updaterPublicKeyEmbedded   string
)

// configureUpdater wires the Wails updater lifecycle without blocking startup.
// Headless (WindowNone): discovery and progress surface through the Utilities
// page and Wails events, never an unexpected modal. Explicit applicant action
// is required for download, install, and restart.
func configureUpdater(a *app.App, desktop *application.App) {
	manifestURL := strings.TrimSpace(updaterManifestURLEmbedded)
	if manifestURL == "" {
		manifestURL = strings.TrimSpace(os.Getenv("YOURMOVE_UPDATE_MANIFEST_URL"))
	}
	if manifestURL == "" {
		a.SetUpdateController(app.UpdateController{})
		log.Printf("automatic updates disabled: YOURMOVE_UPDATE_MANIFEST_URL is not set (manual DMG remains available)")
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
	if path := strings.TrimSpace(os.Getenv("YOURMOVE_UPDATE_PUBLIC_KEY_FILE")); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("update public key file unreadable: %v", err)
			return nil
		}
		if len(data) == 0 {
			return nil
		}
		return data
	}
	if raw := strings.TrimSpace(os.Getenv("YOURMOVE_UPDATE_PUBLIC_KEY")); raw != "" {
		// Allow escaped newlines from shell-provided PEM blocks.
		raw = strings.ReplaceAll(raw, `\n`, "\n")
		return []byte(raw)
	}
	return nil
}

// startUpdateTestAuto drives the real Wails updater path for the controlled
// dev fixture only. It requires explicit YOURMOVE_UPDATE_TEST_AUTO=1 and
// performs the same Check, DownloadAndInstall, and Restart calls the
// Utilities buttons make. Production never sets this variable.
func startUpdateTestAuto(a *app.App, desktop *application.App) {
	if os.Getenv("YOURMOVE_UPDATE_TEST_AUTO") != "1" {
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
func startUpdateBackground(a *app.App) {	delay := 30 * time.Second
	if os.Getenv("YOURMOVE_UPDATE_BG_FAST") == "1" {
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

// Separate debug-only endpoint. It is not mounted in browser mode or in
// normal desktop runs; use YOURMOVE_WAILS_PROBE=1 explicitly.
func probeMux(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", next)
	mux.HandleFunc("GET /__wails_probe", showProbe)
	mux.HandleFunc("POST /__wails_probe", receiveProbe)
	return mux
}
