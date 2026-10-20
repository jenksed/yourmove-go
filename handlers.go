package main

import (
	"context"
	"database/sql"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"example.com/yourmove-lite/internal/platforms"
)

var jobsTemplate = template.Must(
	template.ParseFiles("templates/jobs.html", "templates/partials.html"),
)

var jobDetailTemplate = template.Must(
	template.ParseFiles("templates/job.html", "templates/partials.html"),
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := jobDetailTemplate.Execute(w, jobWorkspace{Job: job, Resumes: resumes, ResumeOptions: resumeOptions, Submissions: history, Desktop: a.openPosting != nil, SubmissionToken: token, SelectedResume: selected, Handoff: assessResumeHandoff(job, selected), Match: match, HasFamilies: showMatch}); err != nil {
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
	case errors.Is(err, platforms.ErrUnsupported):
		return platforms.Metadata{}, "unsupported", "This platform has no metadata extractor yet."
	case err != nil:
		log.Printf("extract job metadata: %v", err)
		return platforms.Metadata{}, "failed", err.Error()
	default:
		return metadata, "retrieved", "Posting details returned by a supported source; application availability not verified."
	}
}
