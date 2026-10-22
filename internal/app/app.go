package app

import (
	"database/sql"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
)

// Config supplies workspace dependencies, embedded resources, and native callbacks.
// The caller owns the database and closes it after the application.
type Config struct {
	DB                      *sql.DB
	ResumeDir               string
	TemplateFiles           fs.FS
	StaticFiles             fs.FS
	CoverLetterInstructions string
	PickResume              func() (string, error)
	ChooseExportPath        func(string) (string, error)
	OpenPosting             func(string) error
}

type appResources struct {
	staticFiles             fs.FS
	coverLetterInstructions string
	jobsTemplate            *template.Template
	jobDetailTemplate       *template.Template
	resumesTemplate         *template.Template
	applicationsTemplate    *template.Template
	candidateTemplate       *template.Template
	inspectionTemplate      *template.Template
	aiConnectionsTemplate   *template.Template
	requirementsTemplate    *template.Template
	batchResultsTemplate    *template.Template
	familiesTemplate        *template.Template
	utilitiesTemplate       *template.Template
	enrichmentTemplate      *template.Template
	answerLibraryTemplate   *template.Template
}

// New parses each page with the shared partials once, before serving requests.
// Resource paths are relative to the supplied filesystems, never the working directory.
func New(config Config) (*App, error) {
	if config.DB == nil || config.TemplateFiles == nil || config.StaticFiles == nil ||
		strings.TrimSpace(config.CoverLetterInstructions) == "" {
		return nil, fmt.Errorf("application requires a database, templates, static assets, and cover-letter instructions")
	}
	resources := &appResources{
		staticFiles: config.StaticFiles, coverLetterInstructions: config.CoverLetterInstructions,
	}
	for _, page := range []struct {
		path   string
		target **template.Template
	}{
		{"templates/jobs.html", &resources.jobsTemplate},
		{"templates/job.html", &resources.jobDetailTemplate},
		{"templates/resumes.html", &resources.resumesTemplate},
		{"templates/applications.html", &resources.applicationsTemplate},
		{"templates/candidate.html", &resources.candidateTemplate},
		{"templates/inspection.html", &resources.inspectionTemplate},
		{"templates/ai.html", &resources.aiConnectionsTemplate},
		{"templates/requirements.html", &resources.requirementsTemplate},
		{"templates/import_results.html", &resources.batchResultsTemplate},
		{"templates/families.html", &resources.familiesTemplate},
		{"templates/utilities.html", &resources.utilitiesTemplate},
		{"templates/enrichment.html", &resources.enrichmentTemplate},
		{"templates/answer_library.html", &resources.answerLibraryTemplate},
	} {
		parsed, err := template.ParseFS(config.TemplateFiles, page.path, "templates/partials.html")
		if err != nil {
			return nil, fmt.Errorf("parse application template %s: %w", page.path, err)
		}
		*page.target = parsed
	}
	if err := indexUnindexedAnswerLibraryEntries(config.DB); err != nil {
		return nil, fmt.Errorf("initialize Answer Library search index: %w", err)
	}
	return &App{
		db: config.DB, resumeDir: config.ResumeDir, resources: resources,
		pickResume: config.PickResume, chooseExportPath: config.ChooseExportPath,
		openPosting: config.OpenPosting,
	}, nil
}

// Handler serves the localhost browser application.
func (a *App) Handler() http.Handler { return newMux(a) }

// DesktopHandler grants native request context and adapts POST redirects.
// Mount it only in Wails' in-process asset handler, never on a TCP server.
func (a *App) DesktopHandler() http.Handler { return newDesktopMux(a) }

// Close terminates all managed browser sessions.
func (a *App) Close() { a.browsers.closeAll() }
