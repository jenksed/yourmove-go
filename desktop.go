//go:build desktop && darwin

package main

import (
	"log"
	"net/http"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/jenksed/yourmove-go/internal/storage"
)

// Wails is a host for the same Go App and HTTP routes as browser mode.
// Templates and static assets are embedded; no repository working directory is needed.
func main() {
	dbPath, resumeDir, err := storage.ApplicationStorage()
	if err != nil {
		log.Fatal(err)
	}
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	app := &App{db: db, resumeDir: resumeDir, pickResume: func() (string, error) {
		return application.Get().Dialog.OpenFile().SetTitle("Select a resume (PDF or DOCX)").AddFilter("Resumes", "*.pdf;*.docx").PromptForSingleSelection()
	}, chooseExportPath: func(name string) (string, error) {
		return application.Get().Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
			Title: "Save a copy", Filename: name, CanCreateDirectories: true,
		}).PromptForSingleSelection()
	}, openPosting: func(url string) error {
		return application.Get().Browser.OpenURL(url)
	}}
	defer app.browsers.closeAll()
	handler := newDesktopMux(app)
	start := "/jobs"
	if os.Getenv("YOURMOVE_WAILS_PROBE") == "1" {
		handler = probeMux(handler)
		start = "/__wails_probe"
	}

	desktop := application.New(application.Options{
		Name:   "YourMove",
		Assets: application.AssetOptions{Handler: handler},
	})
	desktop.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "YourMove",
		Width:     1100,
		Height:    800,
		MinWidth:  700,
		MinHeight: 550,
		URL:       start,
	})
	log.Printf("YourMove Wails compatibility run starting at %s", start)
	if err := desktop.Run(); err != nil {
		log.Fatal(err)
	}
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
