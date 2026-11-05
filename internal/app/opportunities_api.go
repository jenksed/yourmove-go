package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Opportunities JSON transport: narrow, protected by the existing local-only
// mux. React owns presentation and disposable drafts; Go owns validation,
// identity, stages, and persistence.

type opportunityJobJSON struct {
	ID               int64  `json:"id"`
	URL              string `json:"url"`
	Host             string `json:"host"`
	Title            string `json:"title"`
	Company          string `json:"company"`
	Description      string `json:"description"`
	Stage            string `json:"stage"`
	StageLabel       string `json:"stageLabel"`
	Priority         string `json:"priority"`
	PriorityLabel    string `json:"priorityLabel"`
	ActionLabel      string `json:"actionLabel"`
	ActionHref       string `json:"actionHref"`
	ActionReason     string `json:"actionReason"`
	SelectedResumeID int64  `json:"selectedResumeId"`
	SubmissionCount  int64  `json:"submissionCount"`
	SourceStatus     string `json:"sourceStatus"`
	CompanyOrHost    string `json:"companyOrHost"`
	DisplayTitle     string `json:"displayTitle"`
}

func toOpportunityJSON(job Job) opportunityJobJSON {
	companyOrHost := job.Company
	if strings.TrimSpace(companyOrHost) == "" {
		companyOrHost = job.Host
	}
	displayTitle := job.Title
	if strings.TrimSpace(displayTitle) == "" {
		displayTitle = job.URL
	}
	return opportunityJobJSON{
		ID: job.ID, URL: job.URL, Host: job.Host, Title: job.Title,
		Company: job.Company, Description: job.Description, Stage: job.Stage,
		StageLabel: job.StageLabel(), Priority: job.Priority, PriorityLabel: job.PriorityLabel(),
		ActionLabel: job.ActionLabel(), ActionHref: job.ActionHref(), ActionReason: job.ActionReason(),
		SelectedResumeID: job.SelectedResumeID, SubmissionCount: job.SubmissionCount,
		SourceStatus: job.SourceStatus, CompanyOrHost: companyOrHost, DisplayTitle: displayTitle,
	}
}

func writeOpportunityError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func decodeOpportunityJSON(w http.ResponseWriter, r *http.Request, limit int64, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeOpportunityError(w, http.StatusBadRequest, "Invalid request body")
		return false
	}
	return true
}

// GET /api/opportunities returns the same queue Go renders for /jobs.
func (a *App) opportunitiesAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		jobs, err := a.allJobs()
		if err != nil {
			log.Printf("load opportunities api: %v", err)
			writeOpportunityError(w, http.StatusInternalServerError, "Unable to load jobs")
			return
		}
		if jobs == nil {
			jobs = []Job{}
		}
		page := summarizeJobs(jobs)
		recorded, err := a.applicationsRecordedToday(time.Now())
		if err != nil {
			log.Printf("load application progress api: %v", err)
			writeOpportunityError(w, http.StatusInternalServerError, "Unable to load application progress")
			return
		}
		out := make([]opportunityJobJSON, 0, len(jobs))
		for _, job := range jobs {
			out = append(out, toOpportunityJSON(job))
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jobs": out, "total": page.Total, "remaining": page.Remaining,
			"applied": page.Applied, "recordedToday": recorded,
			"queueTotal": page.QueueTotal(), "recordedPercent": page.RecordedPercent(),
			"next": page.Next, "desktop": a.openPosting != nil,
		})
		return
	}

	if r.Method == http.MethodPost {
		var payload struct {
			URL   string `json:"url"`
			Title string `json:"title"`
		}
		if !decodeOpportunityJSON(w, r, 8<<10, &payload) {
			return
		}
		id, duplicate, err := a.savePostingURL(r.Context(), payload.URL, payload.Title)
		if err != nil {
			status, message := http.StatusInternalServerError, "Unable to save job"
			switch {
			case errors.Is(err, errPostingInput):
				status, message = http.StatusBadRequest, "Invalid job URL or title"
			case errors.Is(err, errPostingLookup):
				message = "Unable to check saved jobs"
			}
			log.Printf("single posting intake api: %v", err)
			writeOpportunityError(w, status, message)
			return
		}
		if duplicate {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "added": false, "duplicate": true})
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "added": true, "duplicate": false})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// POST /api/opportunities/import accepts {urls: "<one per line>"} and reuses
// the batch writer so duplicates never overwrite existing work.
func (a *App) opportunitiesImportAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		URLs any `json:"urls"`
	}
	if !decodeOpportunityJSON(w, r, 128<<10, &payload) {
		return
	}
	var raw string
	switch value := payload.URLs.(type) {
	case string:
		raw = value
	case []any:
		lines := make([]string, 0, len(value))
		for _, entry := range value {
			text, ok := entry.(string)
			if !ok {
				writeOpportunityError(w, http.StatusBadRequest, "Invalid batch input: URLs must be strings")
				return
			}
			lines = append(lines, text)
		}
		raw = strings.Join(lines, "\n")
	default:
		writeOpportunityError(w, http.StatusBadRequest, "Invalid batch input: provide URLs text")
		return
	}
	if len(raw) > 125<<10 {
		writeOpportunityError(w, http.StatusRequestEntityTooLarge, "Too much input")
		return
	}
	results, err := a.importJobBatch(raw)
	if err != nil {
		if errors.Is(err, errBatchInput) {
			writeOpportunityError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("batch intake api: %v", err)
		writeOpportunityError(w, http.StatusInternalServerError, "Unable to import jobs; no changes from this batch were saved")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(results)
}

// POST /api/opportunities/delete accepts {ids: [...], confirm: true}.
// History-bearing jobs are refused, mirroring the Go form behavior.
func (a *App) opportunitiesDeleteAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		IDs     []int64 `json:"ids"`
		Confirm bool    `json:"confirm"`
	}
	if !decodeOpportunityJSON(w, r, 8<<10, &payload) {
		return
	}
	if !payload.Confirm {
		writeOpportunityError(w, http.StatusBadRequest, "Check confirmation and type DELETE")
		return
	}
	if len(payload.IDs) == 0 || len(payload.IDs) > maxDeleteBatch {
		writeOpportunityError(w, http.StatusBadRequest, "Select up to 50 jobs to delete")
		return
	}
	deleted, err := a.deleteJobs(payload.IDs, false)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		writeOpportunityError(w, http.StatusConflict, "A selected job no longer exists. Refresh the list.")
	case errors.Is(err, ErrJobHasHistory):
		writeOpportunityError(w, http.StatusConflict, "Selection includes a job with confirmed submissions. Uncheck that job; remove it individually if intended.")
	case err != nil:
		log.Printf("bulk deletion api: %v", err)
		writeOpportunityError(w, http.StatusBadRequest, "Unable to delete selected jobs; none were removed")
	default:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"deleted": deleted})
	}
}

// POST /api/opportunities/{id}/open is desktop-only and opens the saved URL
// in the system browser without trusting client-provided URLs.
func (a *App) opportunityOpenAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if a.openPosting == nil {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	job, err := a.jobByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("load posting api %d: %v", id, err)
		writeOpportunityError(w, http.StatusInternalServerError, "Unable to load saved posting")
		return
	}
	if _, _, err := normalizePostingURL(job.URL); err != nil {
		writeOpportunityError(w, http.StatusUnprocessableEntity, "Saved job URL is invalid; edit your job data before opening")
		return
	}
	if err := a.openPosting(job.URL); err != nil {
		log.Printf("open posting api in default browser: %v", err)
		writeOpportunityError(w, http.StatusBadGateway, "Could not launch default browser; copy the saved posting URL to open it manually")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"opened": true, "id": id})
}
