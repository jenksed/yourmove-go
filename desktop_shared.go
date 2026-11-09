//go:build desktop && (darwin || windows) && !qualification

package main

import (
	"log"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

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

	// Ordinary window closing (red X) and Cmd+Q / Dock quit are guarded
	// separately: both consult the React-reported unsaved flag. A clean
	// workspace closes without prompting. allowDesktopClose lets the
	// confirmed second close proceed without re-prompting.
	var allowDesktopClose atomic.Bool
	var desktop *application.App
	desktop = application.New(application.Options{
		Name:   "YourMove",
		Assets: application.AssetOptions{Handler: handler},
		// Cmd+Q and Dock quit arrive here. A clean workspace quits
		// directly; dirty answer, letter, detail, or evidence drafts
		// get a native confirmation instead of silent loss.
		ShouldQuit: func() bool {
			if allowDesktopClose.Load() || !a.HasDesktopUnsaved() {
				return true
			}
			confirmDesktopUnsavedClose(desktop.Window.Current(), func() {
				allowDesktopClose.Store(true)
				application.Get().Quit()
			})
			return false
		},
	})
	// macOS only: wire the pinned updater lifecycle without blocking startup.
	// Explicit applicant action is required for download, install, restart.
	configureDesktopUpdater(a, desktop)
	startDesktopUpdateTasks(a, desktop)
	window := desktop.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "YourMove",
		Width:     1100,
		Height:    800,
		MinWidth:  700,
		MinHeight: 550,
		URL:       start,
	})
	// Ordinary window closing is cancellable: a clean workspace closes
	// directly, while dirty drafts get a native confirmation.
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if allowDesktopClose.Load() || !a.HasDesktopUnsaved() {
			return
		}
		event.Cancel()
		confirmDesktopUnsavedClose(window, func() {
			allowDesktopClose.Store(true)
			window.Close()
		})
	})
	log.Printf("YourMove Wails compatibility run starting at %s (version %s build %s)", start, app.CurrentAppVersion(), app.CurrentAppBuild())
	if err := desktop.Run(); err != nil {
		log.Fatal(err)
	}
}

// confirmDesktopUnsavedClose asks natively whether to abandon dirty React
// drafts. Stay is the default and cancel action; quitting runs quit, which
// sets the one-shot allow flag before closing or quitting again.
func confirmDesktopUnsavedClose(host application.Window, quit func()) {
	dialog := application.Get().Dialog.Question()
	dialog.SetTitle("Unsaved writing")
	dialog.SetMessage("You have unsaved answer or letter edits. Quit without saving them?")
	quitButton := dialog.AddButton("Quit without saving")
	quitButton.OnClick(func() { quit() })
	stay := dialog.AddButton("Stay")
	dialog.SetDefaultButton(stay)
	dialog.SetCancelButton(stay)
	if host != nil {
		dialog.AttachToWindow(host)
	}
	dialog.Show()
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
