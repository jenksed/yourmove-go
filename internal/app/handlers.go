package app

import (
	"context"
	"database/sql"
	"errors"
	browser "github.com/jenksed/yourmove-go/internal/application/browser"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	coverprovider "github.com/jenksed/yourmove-go/internal/coverletter/provider"
	"github.com/jenksed/yourmove-go/internal/platforms"
)

func home(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/jobs", http.StatusSeeOther)
}

func (a *App) renderOpportunityPage(w http.ResponseWriter, formURL, formTitle, urlError, titleError string, status int) {
	jobs, err := a.allJobs()
	if err != nil {
		log.Printf("load jobs: %v", err)
		http.Error(w, "Unable to load jobs", http.StatusInternalServerError)
		return
	}
	page := summarizeJobs(jobs)
	page.NewJobURL = formURL
	page.NewJobTitle = formTitle
	page.JobURLError = urlError
	page.JobTitleError = titleError
	page.CreateFormOpen = urlError != "" || titleError != ""
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
	w.WriteHeader(status)
	if err := a.resources.jobsTemplate.Execute(w, page); err != nil {
		log.Printf("render jobs: %v", err)
	}
}

func (a *App) jobForm(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		a.renderOpportunityPage(w, "", "", "", "", http.StatusOK)
		return
	}

	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid job form", http.StatusBadRequest)
			return
		}
		formURL := r.PostForm.Get("job_url")
		formTitle := r.PostForm.Get("job_title")
		jobURL, host, err := normalizePostingURL(formURL)
		jobTitle := strings.TrimSpace(formTitle)
		if err != nil || len(jobTitle) > 200 {
			if r.PostForm.Get("save_first") == "1" {
				urlError, titleError := "", ""
				if err != nil {
				urlError = "Enter a valid job posting URL."
				}
				if len(jobTitle) > 200 {
				titleError = "Use a job title of 200 bytes or fewer."
				}
				a.renderOpportunityPage(w, formURL, formTitle, urlError, titleError, http.StatusBadRequest)
				return
			}
			http.Error(w, "Invalid job URL or title", http.StatusBadRequest)
			return
		}
		// The first-party intake saves locally before any optional source check.
		// Saving a URL grants no network or employer-browser authority.
		if r.PostForm.Get("save_first") == "1" {
			id, _, err := a.storeJobIfNew(Job{URL: jobURL, Host: host, Title: jobTitle, TitleManual: jobTitle != "", SourceStatus: "not_checked"})
			if err != nil {
				http.Error(w, "Unable to save job", http.StatusInternalServerError)
				return
			}
			http.Redirect(w, r, "/jobs/"+strconv.FormatInt(id, 10)+"#overview", http.StatusSeeOther)
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
		fetch := a.enrichmentFetch
		if fetch == nil {
			fetch = fetchPostingDetails
		}
		metadata, status, note := fetch(r.Context(), jobURL)
		manualTitle := jobTitle != ""
		if jobTitle == "" {
			jobTitle = metadata.Title
		}
		salaryOrigin := metadata.SalaryOrigin
		if metadata.SalaryText != "" && salaryOrigin == "" {
			salaryOrigin = "employer"
		}
		checkedAt := time.Now().UTC().Format(time.RFC3339Nano)
		job := Job{
			URL: jobURL, Host: host, Title: jobTitle, Company: metadata.Company,
			Description: metadata.Description, SalaryText: metadata.SalaryText,
			SalaryOrigin: salaryOrigin, TitleManual: manualTitle,
			SourceStatus: status, SourceNote: note,
			SourceCheckedAt: checkedAt,
		}
		latestSuccessAt := ""
		descriptionConfirmed := false
		salaryConfirmed := false
		if status == "retrieved" {
			latestSuccessAt = checkedAt
			descriptionConfirmed = metadata.Description != ""
			salaryConfirmed = metadata.SalaryText != ""
		}
		id, _, err := a.storeJobIfNewWithPostingSource(job, metadata.Source, latestSuccessAt, descriptionConfirmed, salaryConfirmed)
		if err != nil {
			log.Printf("save job: %v", err)
			http.Error(w, "Unable to save job", http.StatusInternalServerError)
			return
		}
		destination := "/jobs"
		if strings.TrimSpace(job.Description) == "" {
			destination = "/jobs/" + strconv.FormatInt(id, 10) + "#overview"
		}
		http.Redirect(w, r, destination, http.StatusSeeOther)
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
	sourceState, sourceFound, err := loadJobPostingSource(a.db, job)
	if err != nil {
		log.Printf("load posting source for job %d: %v", id, err)
		http.Error(w, "Unable to load saved posting source", http.StatusInternalServerError)
		return
	}
	reading := buildJobReadingView(job, sourceState, sourceFound)

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
	resumeOptions := resumeOptionsForJob(job, resumes, families)
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
	notice := coverLetterNotice(r.URL.Query().Get("cover"))
	letterModel := coverprovider.DefaultModel("ollama")
	providerReady := make(map[string]bool)
	for _, id := range []string{"codex", "claude-cli", "ollama", "openai", "anthropic", "openrouter", "compatible"} {
		providerReady[id] = coverprovider.Configured(id)
	}
	letterResumes := make([]documentRow, 0, len(resumeOptions))
	for _, option := range resumeOptions {
		if option.ArchivedAt == "" && !option.IsMaster {
			letterResumes = append(letterResumes, option)
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
	earlyQuestions, err := a.prepareEarlyQuestions(job, r)
	if err != nil {
		http.Error(w, "Unable to load discovered application questions", 500)
		return
	}

	workspace := jobWorkspace{
		Questions:            questions,
		EarlyQuestions:       earlyQuestions,
		Reading:              reading,
		Job:                  job,
		Profile:              profile,
		BrowserState:         a.browsers.state(id),
		Resumes:              resumes,
		ResumeOptions:        resumeOptions,
		LetterResumes:        letterResumes,
		Submissions:          history,
		Desktop:              a.openPosting != nil,
		SubmissionToken:      token,
		SelectedResume:       selected,
		AnswerEditingAllowed: job.Stage != StageApplied && job.Stage != StageClosed && job.SubmissionCount == 0,
		Handoff:              assessResumeHandoff(job, selected),
		Sources:              a.coverLetterSources(job, selected),
		Letter:               letter,
		Letters:              letters,
		LetterNotice:         notice,
		LetterOpen:           notice != "" || r.URL.Query().Has("draft"),
		LetterModel:          letterModel,
		ProviderReady:        providerReady,
		Match:                match,
		HasFamilies:          showMatch,
	}
	workspace.SubmissionNotice, workspace.SubmissionUncertain = submissionRecoveryNotice(r.URL.Query().Get("submission"))
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
	workspace.QuestionSnapshot = chooseApplicationQuestionSnapshot(job, workspace.BrowserState, workspace.Questions, workspace.EarlyQuestions)
	workspace.WritingAnswerContext = writingAnswerContext(r, workspace.QuestionSnapshot)
	// A disconnected browser is not current readiness, but its saved writing is
	// still the first useful artifact when no early question inventory exists.
	// This changes presentation only; transfer scope stays server-authoritative.
	if !workspace.QuestionSnapshot.Available && workspace.SavedBrowser != nil && !r.URL.Query().Has("early_question") {
		workspace.WritingAnswerContext = "live"
	}
	workspace.Readiness = prepareReadiness(workspace.ResumeSelectionValid, profile, workspace.BrowserState, workspace.QuestionSnapshot, letter)
	if err := a.resources.jobDetailTemplate.Execute(w, workspace); err != nil {
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
	r.Body = http.MaxBytesReader(w, r.Body, 3*(120<<10)+8192)
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
	if len(title) > 200 || len(company) > 200 || len(description) > 120<<10 || !utf8.ValidString(description) || strings.ContainsRune(description, 0) || len(salary) > 500 || !validStage(stage) || !validPriority(priority) {
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
		return platforms.Metadata{}, "unsupported", "Your job is saved. Copy the description from the employer page and paste it into Edit details."
	case err != nil:
		log.Printf("extract job metadata: %v", err)
		return platforms.Metadata{}, "failed", "Posting details could not be checked. Your saved content is retained; paste the description or try again later."
	default:
		return metadata, "retrieved", "Posting details returned by a supported source; application availability not verified."
	}
}
