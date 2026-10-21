package main

import (
	"context"
	"database/sql"
	"example.com/yourmove-lite/internal/application/schema"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

type App struct {
	intakeMu            sync.Mutex
	enrichmentMu        sync.Mutex
	enrichment          *enrichmentRun
	enrichmentFetch     postingFetcher
	db                  *sql.DB
	resumeDir           string
	pickResume          func() (string, error)
	chooseExportPath    func(string) (string, error) // trusted desktop Save dialog only
	openPosting         func(string) error           // desktop-only native default-browser action
	inspectApplication  func(context.Context, string) (schema.Result, error)
	browsers            sessionHost
	coverLetterGenerate func(context.Context, string) (string, error) // test override; production uses local Ollama
}

func newMux(app *App) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/",
		http.FileServer(http.FS(staticFiles)))
	mux.HandleFunc("/", home)
	mux.HandleFunc("/jobs", app.jobForm)
	mux.HandleFunc("POST /jobs/import", app.importJobs)
	mux.HandleFunc("POST /jobs/delete", app.deleteSelectedJobs)
	mux.HandleFunc("GET /utilities", app.utilitiesPage)
	mux.HandleFunc("GET /ai", app.aiConnectionsPage)
	mux.HandleFunc("GET /candidate", app.candidatePage)
	mux.HandleFunc("GET /requirements", app.requirementsPage)
	mux.HandleFunc("POST /candidate", app.updateCandidate)
	mux.HandleFunc("GET /families", app.familyPage)
	mux.HandleFunc("POST /families", app.createFamily)
	mux.HandleFunc("POST /families/examples", app.addExamples)
	mux.HandleFunc("POST /families/test", app.testFamilyMatch)
	mux.HandleFunc("POST /families/{id}/edit", app.editFamily)
	mux.HandleFunc("POST /families/{id}/delete", app.removeFamily)
	mux.HandleFunc("POST /resumes/{id}/family", app.setResumeFamily)
	mux.HandleFunc("POST /resumes/{id}/master", app.chooseMasterResume)
	mux.HandleFunc("POST /resumes/{id}/delete", app.deleteResumeHandler)
	mux.HandleFunc("GET /enrichment", app.enrichmentPage)
	mux.HandleFunc("POST /enrichment/{action}", app.enrichmentAction)
	mux.HandleFunc("GET /jobs/{id}", app.jobDetail)
	mux.HandleFunc("GET /jobs/{id}/inspection", app.inspectionPage)
	mux.HandleFunc("POST /jobs/{id}/inspect", app.inspectJob)
	mux.HandleFunc("POST /jobs/{id}/continue", app.continueApplication)
	mux.HandleFunc("POST /jobs/{id}/handoff", app.startResumeHandoff)
	mux.HandleFunc("POST /jobs/{id}/browser/{action}", app.browserOperation)
	mux.HandleFunc("GET /jobs/{id}/browser-status", app.browserStatus)
	mux.HandleFunc("POST /jobs/{id}/edit", app.editJob)
	mux.HandleFunc("POST /jobs/{id}/open", app.openJobPosting)
	mux.HandleFunc("POST /jobs/{id}/delete", app.deleteJobHandler)
	mux.HandleFunc("POST /jobs/{id}/refresh", app.refreshJob)
	mux.HandleFunc("POST /jobs/{id}/cover-letter/generate", app.generateCoverLetterHandler)
	mux.HandleFunc("POST /jobs/{id}/cover-letter/save", app.saveCoverLetterHandler)
	mux.HandleFunc("GET /jobs/{id}/cover-letter/{draft}/download", app.downloadCoverLetter)
	mux.HandleFunc("POST /jobs/{id}/cover-letter/transfer", app.transferCoverLetter)
	mux.HandleFunc("POST /jobs/{id}/answers/save", app.saveApplicationAnswer)
	mux.HandleFunc("POST /jobs/{id}/answers/transfer", app.transferApplicationAnswers)
	mux.HandleFunc("GET /resumes", app.resumeLibrary)
	mux.HandleFunc("GET /applications", app.applicationsPage)
	mux.HandleFunc("POST /resumes/import", app.uploadResume)
	mux.HandleFunc("POST /resumes/import-native", app.nativeResumeImport)
	mux.HandleFunc("GET /resumes/{id}/download", app.downloadResume)
	mux.HandleFunc("POST /jobs/{id}/resume", app.setJobResume)
	mux.HandleFunc("POST /jobs/{id}/submit", app.submitJob)
	mux.HandleFunc("POST /resumes/{id}/archive", app.changeResumeArchive)
	mux.HandleFunc("POST /resumes/{id}/revise", app.reviseResumeBrowser)
	mux.HandleFunc("POST /resumes/{id}/revise-native", app.reviseResumeNative)

	return protectLocalRequests(mux)
}

// Protect reads as well as writes against unexpected hosts and browser origins.
// Comparing Origin to Host alone does not stop DNS rebinding: both can name
// the attacker's domain while the connection reaches our loopback listener.
func protectLocalRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !localRequestHost(r.Host) || r.Header.Get("Sec-Fetch-Site") == "cross-site" ||
			len(r.Header.Values("Origin")) > 1 {
			http.Error(w, "Local application request required", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
			http.Error(w, "Cross-origin requests are not allowed", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func localRequestHost(host string) bool {
	u, err := url.Parse("http://" + host)
	if err != nil || u.Host != host || u.User != nil || u.Path != "" ||
		u.RawQuery != "" || u.Fragment != "" || strings.HasSuffix(host, ":") {
		return false
	}
	hostname := strings.ToLower(u.Hostname())
	switch hostname {
	case "localhost", "127.0.0.1", "::1":
	default:
		return false
	}
	if hostname == "::1" {
		hostname = "[::1]"
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n <= 0 || n > 65535 {
			return false
		}
		hostname += ":" + port
	}
	return strings.EqualFold(host, hostname)
}
