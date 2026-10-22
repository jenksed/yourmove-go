package main

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/jenksed/yourmove-go/internal/platforms"
	"github.com/jenksed/yourmove-go/internal/storage"
)

func TestOpenDBFresh(t *testing.T) {
	db, err := storage.OpenDB(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	app := &App{db: db}
	if err := app.saveJob(Job{
		URL: "https://example.com/job", Host: "example.com",
		Title: "Example", Description: "Description from API",
	}); err != nil {
		t.Fatal(err)
	}
	got, err := app.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Example" || got.Description != "Description from API" {
		t.Fatalf("persisted job = %+v", got)
	}
}

func TestOpenDBMigrations(t *testing.T) {
	tests := []struct {
		name, ddl, insert, title string
	}{
		{
			name:   "before title",
			ddl:    `CREATE TABLE jobs (id INTEGER PRIMARY KEY, url TEXT NOT NULL, host TEXT NOT NULL)`,
			insert: `INSERT INTO jobs (url, host) VALUES ('https://example.com/legacy', 'example.com')`,
		},
		{
			name:   "before description",
			ddl:    `CREATE TABLE jobs (id INTEGER PRIMARY KEY, url TEXT NOT NULL, host TEXT NOT NULL, title TEXT NOT NULL DEFAULT '')`,
			insert: `INSERT INTO jobs (url, host, title) VALUES ('https://example.com/legacy', 'example.com', 'Existing title')`,
			title:  "Existing title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "legacy.db")
			legacy, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := legacy.Exec(tt.ddl); err != nil {
				t.Fatal(err)
			}
			if _, err := legacy.Exec(tt.insert); err != nil {
				t.Fatal(err)
			}
			if err := legacy.Close(); err != nil {
				t.Fatal(err)
			}

			// Opening twice proves schema changes are safe to repeat.
			for i := 0; i < 2; i++ {
				db, err := storage.OpenDB(path)
				if err != nil {
					t.Fatal(err)
				}
				got, err := (&App{db: db}).jobByID(1)
				if err != nil {
					t.Fatal(err)
				}
				if got.URL != "https://example.com/legacy" || got.Title != tt.title || got.Description != "" || got.Stage != StageReview || got.Priority != PriorityNormal || got.Company != "" || !got.TitleManual || !got.DescriptionManual || !got.CompanyManual {
					t.Fatalf("after migration: %+v", got)
				}
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestSourceRefreshPreservesManualEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.db")
	db, err := storage.OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{db: db}
	if err := app.saveJob(Job{
		URL: "https://jobs.lever.co/acme/a1", Host: "jobs.lever.co",
		Title: "From API", Company: "first-board", Description: "Old description", SalaryText: "USD 80000–90000 / year", SalaryOrigin: "employer",
		SourceStatus: "retrieved",
	}); err != nil {
		t.Fatal(err)
	}
	original, err := app.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	// A user changes title and salary. A later source refresh can update the description,
	// but neither overwrite manually edited fields nor invent a closure on lookup failure.
	original.Title = "Preferred title"
	original.SalaryText = "USD 95000 base / year (confirmed manually)"
	if err := app.updateJob(original); err != nil {
		t.Fatal(err)
	}
	meta := platforms.Metadata{Title: "Updated API title", Company: "new-board", Description: "Updated description", SalaryText: "USD 100000–110000 / year"}
	if err := app.applyPostingRefresh(1, meta, "retrieved", "fresh", "2026-10-19T12:00:00Z"); err != nil {
		t.Fatal(err)
	}
	got, err := app.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != original.Title || got.Company != "new-board" || got.Description != "Updated description" || got.SalaryText != original.SalaryText || got.SalaryOrigin != "manual" || !got.TitleManual || !got.SalaryManual || got.DescriptionManual {
		t.Fatalf("refresh overwritten manual values or failed to update imported values: %+v", got)
	}
	if err := app.applyPostingRefresh(1, platforms.Metadata{}, "failed", "HTTP 404", "2026-10-19T12:01:00Z"); err != nil {
		t.Fatal(err)
	}
	got, err = app.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != original.Title || got.Company != "new-board" || got.Description != "Updated description" || got.SalaryText != original.SalaryText || got.SourceStatus != "failed" || got.SourceNote != "HTTP 404" {
		t.Fatalf("failed refresh changed job content: %+v", got)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	persisted, err := (&App{db: reopened}).jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Title != got.Title || persisted.Description != got.Description || persisted.SalaryOrigin != got.SalaryOrigin || persisted.SourceStatus != got.SourceStatus {
		t.Fatalf("source state not durable: %+v", persisted)
	}
}

func TestRefreshPreservesManualCompany(t *testing.T) {
	db, err := storage.OpenDB(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app := &App{db: db}
	if err := app.saveJob(Job{URL: "https://jobs.ashbyhq.com/openai/c7f26e3d-7345-44a6-8e5a-3599679bfe9e", Host: "jobs.ashbyhq.com", Company: "openai"}); err != nil {
		t.Fatal(err)
	}
	job, err := app.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	job.Company = "OpenAI"
	if err := app.updateJob(job); err != nil {
		t.Fatal(err)
	}
	if err := app.applyPostingRefresh(1, platforms.Metadata{Company: "openai", Title: "Support Operations Lead"}, "retrieved", "", "2026-10-19T12:00:00Z"); err != nil {
		t.Fatal(err)
	}
	got, err := app.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Company != "OpenAI" || !got.CompanyManual || got.Title != "Support Operations Lead" {
		t.Fatalf("manual employer edit was overwritten: %+v", got)
	}
}

func TestRefreshDoesNotEraseUnreturnedSalary(t *testing.T) {
	db, err := storage.OpenDB(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app := &App{db: db}
	if err := app.saveJob(Job{URL: "https://jobs.lever.co/acme/test", Host: "jobs.lever.co", SalaryText: "USD 90000–100000 / year", SalaryOrigin: "employer"}); err != nil {
		t.Fatal(err)
	}
	if err := app.applyPostingRefresh(1, platforms.Metadata{Title: "New title"}, "retrieved", "", "2026-10-19T12:00:00Z"); err != nil {
		t.Fatal(err)
	}
	got, err := app.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if got.SalaryText != "USD 90000–100000 / year" || got.SalaryOrigin != "employer" || got.Title != "New title" {
		t.Fatalf("partial source refresh erased known values: %+v", got)
	}
}
