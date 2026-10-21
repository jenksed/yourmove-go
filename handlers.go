package main

import (
	"context"
	"database/sql"
	"errors"
	browser "example.com/yourmove-lite/internal/application/browser"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"example.com/yourmove-lite/internal/platforms"
)

var jobsTemplate = template.Must(
	template.ParseFS(
		templateFiles,
		"templates/jobs.html",
		"templates/partials.html",
	),
)

var jobDetailTemplate = template.Must(
	template.ParseFS(
		templateFiles,
		"templates/job.html",
		"templates/partials.html",
	),
)

func home(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/jobs", http.StatusSeeOther)
}

func (a *App) jobForm(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Load jobs from SQLite before writing HTML.
		jobs, err := a.allJobs()
		if err != nil {
			log.Printf("load jobs: %v", err)
			http.Error(w, "Unable to load jobs", http.StatusInternalServerError)
			return
		}

		page := summarizeJobs(jobs)
		page.RecordedToday, err = a.applicationsRecordedToday(time.Now())
		if err != nil {
			log.Printf("load today's application progress: %v", err)
			http.Error(w, "Unable to load application progress", http.StatusInternalServerError)
			return
		}
		page.Desktop = a.openPosting != nil
		families, err := a.allFamilies()
		if err != nil {
			http.Error(w, "Unable to load role families", 500)
			return
		}
		if len(families) > 0 {
			resumes, err := a.allResumes()
			if err != nil {
				http.Error(w, "Unable to load resumes", 500)
				return
			}
			page.Matches = make(map[int64]MatchRecommendation, len(jobs))
			for _, job := range jobs {
				if job.SelectedResumeID == 0 && job.SubmissionCount == 0 {
					page.Matches[job.ID] = matchResume(job, families, resumes)
				}
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		if err := jobsTemplate.Execute(w, page); err != nil {
			log.Printf("render jobs: %v", err)
		}

		return

	}

	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid job form", http.StatusBadRequest)
			return
		}
		jobURL, host, err := normalizePostingURL(r.PostForm.Get("job_url"))
		jobTitle := strings.TrimSpace(r.PostForm.Get("job_title"))
		if err != nil || len(jobTitle) > 200 {
			http.Error(w, "Invalid job URL or title", http.StatusBadRequest)
			return
		}
		// Duplicates must not cause a new employer fetch or overwrite manual work.
		if exists, err := a.hasJobURL(jobURL); err != nil {
			log.Printf("check existing job: %v", err)
			http.Error(w, "Unable to check saved jobs", 500)
			return
		} else if exists {
			http.Redirect(w, r, "/jobs", http.StatusSeeOther)
			return
		}
		metadata, status, note := fetchPostingDetails(r.Context(), jobURL)
		manualTitle := jobTitle != ""
		if jobTitle == "" {
			jobTitle = metadata.Title
		}
		salaryOrigin := ""
		if metadata.SalaryText != "" {
			salaryOrigin = "employer"
		}
		job := Job{
			URL: jobURL, Host: host, Title: jobTitle, Company: metadata.Company,
			Description: metadata.Description, SalaryText: metadata.SalaryText,
			SalaryOrigin: salaryOrigin, TitleManual: manualTitle,
			SourceStatus: status, SourceNote: note,
			SourceCheckedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if _, _, err := a.storeJobIfNew(job); err != nil {
			log.Printf("save job: %v", err)
			http.Error(w, "Unable to save job", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/jobs", http.StatusSeeOther)
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (a *App) jobDetail(w http.ResponseWriter, r *http.Request) {
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
		log.Printf("load job detail: %v", err)
		http.Error(w, "Unable to load job", http.StatusInternalServerError)
		return
	}

	resumes, err := a.allResumes()
	if err != nil {
		log.Printf("list resumes for job: %v", err)
		http.Error(w, "Unable to load resume list", 500)
		return
	}
	families, err := a.allFamilies()
	if err != nil {
		http.Error(w, "Unable to load role families", 500)
		return
	}
	match := matchResume(job, families, resumes)
	familyNames := make(map[int64]string, len(families))
	for _, f := range families {
		familyNames[f.ID] = f.Name
	}
	resumeOptions := make([]documentRow, 0, len(resumes))
	for _, resume := range resumes {
		if resume.IsMaster || (resume.ArchivedAt != "" && resume.ID != job.SelectedResumeID) {
			continue
		}
		resumeOptions = append(resumeOptions, documentRow{Resume: resume, FamilyName: familyNames[resume.RoleFamilyID]})
	}
	showMatch := len(families) > 0 && job.SubmissionCount == 0 && job.Stage != StageApplied && job.Stage != StageClosed
	history, err := a.jobSubmissions(id)
	if err != nil {
		log.Printf("load submission history: %v", err)
		http.Error(w, "Unable to load applications", 500)
		return
	}
	var selected *Resume
	for i := range resumes {
		if resumes[i].ID == job.SelectedResumeID {
			selected = &resumes[i]
			break
		}
	}
	token, err := newSubmissionToken()
	if err != nil {
		http.Error(w, "Unable to prepare submission confirmation", http.StatusInternalServerError)
		return
	}
	reviewedResumeHash := ""
	if selected != nil {
		reviewedResumeHash = selected.SHA256
	}
	if err := a.recordSubmissionReview(id, token, job.SelectedResumeID, reviewedResumeHash); err != nil {
		if errors.Is(err, ErrSubmissionReviewStale) {
			http.Error(w, "The selected resume changed while this page was loading. Reload and review it again.", http.StatusConflict)
			return
		}
		log.Printf("record submission review: %v", err)
		http.Error(w, "Unable to prepare submission confirmation", http.StatusInternalServerError)
		return
	}
	letters, err := a.listCoverLetters(id)
	if err != nil {
		log.Printf("list cover letters: %v", err)
		http.Error(w, "Unable to list cover letter drafts", 500)
		return
	}
	var letter CoverLetter
	if selectedDraft := r.URL.Query().Get("draft"); selectedDraft != "" {
		draftID, parseErr := strconv.ParseInt(selectedDraft, 10, 64)
		if parseErr != nil || draftID <= 0 {
			http.NotFound(w, r)
			return
		}
		letter, err = a.coverLetterByID(id, draftID)
	} else {
		letter, err = a.latestCoverLetter(id)
	}
	if errors.Is(err, sql.ErrNoRows) && r.URL.Query().Has("draft") {
		http.NotFound(w, r)
		return
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("load cover letter: %v", err)
		http.Error(w, "Unable to load saved cover letter", 500)
		return
	}
	notice := map[string]string{
		"transferring":             "Cover-letter transfer authorized. Review its result below.",
		"generated":                "Cover letter generated and saved locally. Review the text before using it.",
		"saved":                    "Your edits were saved locally. Review the current scope before continuing.",
		"source-description":       "No job description is saved. Open Overview → Edit job and tracking, save the description, then generate again.",
		"source-master-missing":    "No master resume is selected. Open Documents → Manage document on your master resume → Set as master resume, then return and generate again. A filename containing Master does not select it automatically.",
		"source-master-unreadable": "The designated master resume is archived, missing, changed, or has no readable text. Open Documents to repair it or designate a readable DOCX/PDF as master. Changing Resume to emphasize alone will not fix this source.",
		"source-resume-missing":    "Choose an available application resume under Resume to emphasize, then generate again.",
		"source-resume-invalid":    "The writing resume is archived or is the master resume. Choose an active application resume under Resume to emphasize; the master is used separately as career evidence.",
		"source-resume-unreadable": "The resume selected under Resume to emphasize could not be read. Choose another readable DOCX/PDF there, or repair that document in Documents. Your application resume selection is unchanged.",
		"sources":                  "The selected letter source could not be read. Check the job description and master resume, or choose a different DOCX/PDF resume below.",
		"model":                    "Choose a provider and enter a valid model name. For custom providers, set YOURMOVE_COMPAT_URL and YOURMOVE_COMPAT_MODEL.",
		"key":                      "This API provider needs a key. Set the appropriate environment variable, then restart YourMove. API usage is billed separately from subscriptions.",
		"cli":                      "The subscription CLI is not installed or not in YourMove's PATH. Open AI connections for setup instructions.",
		"login":                    "The CLI is not signed in with a subscription account. Open AI connections, then sign in using the official CLI.",
		"cli-failed":               "The subscription CLI could not finish writing. Try it from Terminal, verify your subscription and model availability, and retry.",
		"auth":                     "The provider rejected its API key or permissions. Check the key and account access.",
		"rate":                     "The provider is rate-limiting requests. Try again later or choose another provider.",
		"credits":                  "The provider reports insufficient credits or billing access. Check the provider account.",
		"timeout":                  "Generation timed out. Try a smaller/faster model or a different provider.",
		"output":                   "The model returned no usable letter. Try another model or generate again; existing drafts are unchanged.",
		"provider-unavailable":     "The provider could not complete the request. Check service status and model availability.",
		"model-missing":            "That Ollama model is not installed. Run ollama list, then select an installed model.",
		"offline":                  "Cannot reach Ollama at 127.0.0.1:11434. Start Ollama and try again.",
		"failed":                   "YourMove could not generate or save a cover letter. The existing draft is unchanged. Check the application log.",
		"invalid":                  "The draft could not be saved. Check its length and try again.",
	}[r.URL.Query().Get("cover")]
	letterModel := coverDefaultModel("ollama")
	providerReady := make(map[string]bool)
	for _, id := range []string{"codex", "claude-cli", "ollama", "openai", "anthropic", "openrouter", "compatible"} {
		providerReady[id] = coverProviderConfigured(id)
	}
	letterResumes := make([]documentRow, 0, len(resumeOptions))
	for _, r := range resumeOptions {
		if r.ArchivedAt == "" && !r.IsMaster {
			letterResumes = append(letterResumes, r)
		}
	}
	profile, err := a.candidateProfile()
	if err != nil {
		http.Error(w, "Unable to read candidate information", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	questions, err := a.prepareQuestions(job, profile)
	if err != nil {
		http.Error(w, "Unable to load saved application answers", 500)
		return
	}
	questions.selectQuestion(r)
	workspace := jobWorkspace{Questions: questions, Job: job, Profile: profile, BrowserState: a.browsers.state(id), Resumes: resumes, ResumeOptions: resumeOptions, LetterResumes: letterResumes, Submissions: history, Desktop: a.openPosting != nil, SubmissionToken: token, SelectedResume: selected, Handoff: assessResumeHandoff(job, selected), Sources: a.coverLetterSources(job, selected), Letter: letter, Letters: letters, LetterNotice: notice, LetterOpen: notice != "" || r.URL.Query().Has("draft"), LetterModel: letterModel, ProviderReady: providerReady, Match: match, HasFamilies: showMatch}
	workspace.TransferNotice = r.URL.Query().Get("transfer_notice")
	workspace.LetterSaved = r.URL.Query().Get("cover") == "saved" && letter.ID > 0
	// Keep the canonical saved-draft URL, but return to the action rather than
	// reopening the editor. Generation and deliberate draft selection still open it.
	if r.URL.Query().Get("cover") == "saved" || r.URL.Query().Get("cover") == "transferring" {
		workspace.LetterOpen = false
	}
	if selected != nil && selected.ArchivedAt == "" {
		_, fileErr := a.readResumeBytes(*selected)
		workspace.ResumeSelectionValid = fileErr == nil
		if fileErr != nil {
			workspace.Handoff = HandoffEligibility{Reason: "The selected resume file is missing or changed. Select and save an available resume below, or import it again in Documents."}
		}
	}
	if job.Stage == StageApplied || job.SubmissionCount > 0 {
		jobs, loadErr := a.allJobs()
		if loadErr != nil {
			log.Printf("load next opportunity: %v", loadErr)
		} else {
			workspace.NextJob = recommendNext(jobs)
		}
	}
	if otherID := a.browsers.otherApplication(id); otherID > 0 {
		if other, loadErr := a.jobByID(otherID); loadErr == nil {
			workspace.OtherBrowser = &other
		}
	}
	if !workspace.BrowserState.Active {
		if saved, loadErr := a.latestInspectionBySource(id, "browser"); loadErr == nil && browser.OwnsPostingURL(job.URL, saved.ApplicationURL) {
			workspace.SavedBrowser = &saved
		}
	}
	workspace.Preparation = summarizePreparation(workspace)
	if err := jobDetailTemplate.Execute(w, workspace); err != nil {
		log.Printf("render job detail: %v", err)
	}
}

// editJob updates only applicant-owned job details and tracking decisions.
// The source URL is immutable in this first edit path.
func (a *App) editJob(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}

	// A bounded local form should not accept arbitrary multi-megabyte payloads.
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid job form", http.StatusBadRequest)
		return
	}
	title := strings.TrimSpace(r.PostForm.Get("title"))
	company := strings.TrimSpace(r.PostForm.Get("company"))
	description := strings.TrimSpace(r.PostForm.Get("description"))
	salary := strings.TrimSpace(r.PostForm.Get("salary_text"))
	stage := r.PostForm.Get("stage")
	priority := r.PostForm.Get("priority")
	if len(title) > 200 || len(company) > 200 || len(description) > 120<<10 || len(salary) > 500 || !validStage(stage) || !validPriority(priority) {
		http.Error(w, "Invalid job details, stage, or priority", http.StatusBadRequest)
		return
	}

	err = a.updateJob(Job{ID: id, Title: title, Company: company, Description: description, SalaryText: salary, Stage: stage, Priority: priority})
	if errors.Is(err, ErrAppliedWithoutSubmission) {
		http.Error(w, "Applied requires a confirmed submission", http.StatusBadRequest)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("update job: %v", err)
		http.Error(w, "Unable to update job", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/jobs/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// refreshJob reads public source metadata only, without transmitting candidate data.
func (a *App) refreshJob(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Unable to load job", http.StatusInternalServerError)
		return
	}
	if err := a.refreshPosting(r.Context(), job); err != nil {
		log.Printf("refresh job: %v", err)
		http.Error(w, "Unable to save source result", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/jobs/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// A successful public API response indicates retrieved metadata, not that the
// application is still open or accepts submissions.
func fetchPostingDetails(ctx context.Context, rawURL string) (platforms.Metadata, string, string) {
	metadata, err := platforms.FetchJob(ctx, rawURL)
	switch {
	case errors.Is(err, platforms.ErrClosed):
		return platforms.Metadata{}, "closed", "This job is closed; the employer is no longer hiring for this posting."
	case errors.Is(err, platforms.ErrUnsupported):
		return platforms.Metadata{}, "unsupported", "This platform has no metadata extractor yet."
	case err != nil:
		log.Printf("extract job metadata: %v", err)
		return platforms.Metadata{}, "failed", err.Error()
	default:
		return metadata, "retrieved", "Posting details returned by a supported source; application availability not verified."
	}
}
