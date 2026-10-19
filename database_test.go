package main

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenDBFresh(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "jobs.db"))
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
				db, err := openDB(path)
				if err != nil {
					t.Fatal(err)
				}
				got, err := (&App{db: db}).jobByID(1)
				if err != nil {
					t.Fatal(err)
				}
				if got.URL != "https://example.com/legacy" || got.Title != tt.title || got.Description != "" {
					t.Fatalf("after migration: %+v", got)
				}
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
