package app

import (
	"database/sql"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

// Tests use the same editable repository sources that assets.go embeds in production.
func testConfig(t *testing.T, db *sql.DB, resumeDir string) Config {
	t.Helper()
	files := os.DirFS("../..")
	instructions, err := fs.ReadFile(files, "prompts/cover_letter.md")
	if err != nil {
		t.Fatal(err)
	}
	narrative, err := fs.ReadFile(files, "prompts/application_answer.md")
	if err != nil {
		t.Fatal(err)
	}
	return Config{
		DB: db, ResumeDir: resumeDir, TemplateFiles: files, StaticFiles: files,
		CoverLetterInstructions: string(instructions), ApplicationAnswerInstructions: string(narrative),
	}
}

func newTestApp(t *testing.T, db *sql.DB, resumeDir string) *App {
	t.Helper()
	a, err := New(testConfig(t, db, resumeDir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a
}

func TestNewRejectsInvalidTemplates(t *testing.T) {
	a, _ := testWorkspace(t)
	config := testConfig(t, a.db, a.resumeDir)
	files := fstest.MapFS{}
	paths, err := fs.Glob(config.TemplateFiles, "templates/*.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		data, err := fs.ReadFile(config.TemplateFiles, path)
		if err != nil {
			t.Fatal(err)
		}
		files[path] = &fstest.MapFile{Data: data}
	}
	config.TemplateFiles = files
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			original := files[path]
			files[path] = &fstest.MapFile{Data: []byte("{{")}
			defer func() { files[path] = original }()
			if app, err := New(config); err == nil || app != nil || !strings.Contains(err.Error(), "parse application template") {
				t.Fatalf("invalid template accepted: app=%v err=%v", app, err)
			}
		})
	}
}

func TestApplicationResourcesBelongToEachInstance(t *testing.T) {
	a, _ := testWorkspace(t)
	config := testConfig(t, a.db, a.resumeDir)
	config.CoverLetterInstructions = "Independent instructions for a second instance."
	b, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Close)
	if a.resources.coverLetterInstructions == b.resources.coverLetterInstructions ||
		a.resources.jobsTemplate == b.resources.jobsTemplate {
		t.Fatal("application instances share initialized resources")
	}
}
