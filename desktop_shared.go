//go:build desktop && (darwin || windows) && !qualification

package main

import (
	"log"
	"net/http"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/jenksed/yourmove-go/internal/app"
	"github.com/jenksed/yourmove-go/internal/storage"
)

// runDesktop is the shared native entry composition for every desktop OS.
// Wails is a host for the same Go App and HTTP routes as browser mode.
// Templates and static assets are embedded; no repository working directory
// is needed. Per-OS files (desktop.go, desktop_windows.go) select this
// composition through build tags and contribute no separate behavior, so
// macOS and Windows launch identical application state over the same
// SQLite workspace.
//
// The macOS updater hooks below are per-OS no-ops outside darwin
// (see desktop_updater.go and desktop_updater_windows.go); no Windows
// updater behavior is introduced here.
func runDesktop() {
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
	// macOS only: wire the pinned updater lifecycle without blocking startup.
	// Explicit applicant action is required for download, install, restart.
	configureDesktopUpdater(a, desktop)
	startDesktopUpdateTasks(a, desktop)
	desktop.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "YourMove",
		Width:     1100,
		Height:    800,
		MinWidth:  700,
		MinHeight: 550,
		URL:       start,
	})
	log.Printf("YourMove Wails compatibility run starting at %s (version %s build %s)", start, app.CurrentAppVersion(), app.CurrentAppBuild())
	if err := desktop.Run(); err != nil {
		log.Fatal(err)
	}
}

// probeMux is the debug-only endpoint mount. It is not mounted in browser
// mode or in normal desktop runs; use YOURMOVE_WAILS_PROBE=1 explicitly.
func probeMux(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", next)
	mux.HandleFunc("GET /__wails_probe", showProbe)
	mux.HandleFunc("POST /__wails_probe", receiveProbe)
	return mux
}
