//go:build desktop && darwin && qualification

package main

import (
	"log"
	"os"

	"github.com/jenksed/yourmove-go/internal/app"
	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/storage"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func runNativeFillQualification() {
	stop, err := browser.StartNativeFillFixture()
	if err != nil {
		log.Fatal(err)
	}
	defer stop()
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
		StaticFiles: staticFiles, FrontendDist: frontendDist,
		CoverLetterInstructions: coverLetterInstructions, ApplicationAnswerInstructions: applicationAnswerInstructions,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()

	const title = "YourMove KAN-17 Synthetic Qualification"
	log.Printf("KAN-17 workspace=%s DB=%s destination=%s", os.Getenv("YOURMOVE_DATA_DIR"), dbPath, browser.NativeFillFixtureURL)
	desktop := application.New(application.Options{Name: title, Assets: application.AssetOptions{Handler: a.DesktopHandler()}})
	desktop.Window.NewWithOptions(application.WebviewWindowOptions{Title: title, Width: 1100, Height: 800, URL: "/jobs"})
	if err := desktop.Run(); err != nil {
		log.Fatal(err)
	}
}
