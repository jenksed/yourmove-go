package app

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jenksed/yourmove-go/internal/platforms"
)

var ErrAppliedWithoutSubmission = errors.New("cannot mark applied without a confirmed submission")

// Stage tracks a human's work. "Applied" is deliberately omitted until a
// separate manual-submission record can establish that outcome.
const (
	StageReview    = "review"
	StageReady     = "ready"
	StageClosed    = "closed"
	StageApplied   = "applied"
	PriorityNormal = "normal"
	PriorityHigh   = "high"
	PriorityLow    = "low"
)

type Job struct {
	ID                int64
	Title             string
	URL               string
	Host              string
	Description       string
	Company           string
	Stage             string
	Priority          string
	SalaryText        string
	SalaryOrigin      string
	TitleManual       bool
	DescriptionManual bool
	CompanyManual     bool
	SalaryManual      bool
	SourceStatus      string
	SourceNote        string
	SourceCheckedAt   string
	SelectedResumeID  int64
	SubmissionCount   int64
	LastSubmittedAt   string
}

func (j Job) StageLabel() string {
	if j.SubmissionCount > 0 {
		return "Applied"
	}
	switch j.Stage {
	case StageApplied:
		return "Applied"
	case StageReady:
		return "Marked ready"
	case StageClosed:
		return "Closed"
	default:
		return "Review"
	}
}

func (j Job) PriorityLabel() string {
	switch j.Priority {
	case PriorityHigh:
		return "High"
	case PriorityLow:
		return "Low"
	default:
		return "Normal"
	}
}

func validStage(s string) bool {
	return s == StageReview || s == StageReady || s == StageClosed || s == StageApplied
}
func validPriority(s string) bool {
	return s == PriorityNormal || s == PriorityHigh || s == PriorityLow
}

func (j Job) DisplayDescription() string { return platforms.NormalizeDescription(j.Description) }

func (j Job) SalaryOriginLabel() string {
	switch j.SalaryOrigin {
	case "employer":
		return "Employer posting"
	case "description":
		return "Employer description"
	case "manual":
		return "Manually entered"
	default:
		return "Not available"
	}
}

func (j Job) SourceStatusLabel() string {
	switch j.SourceStatus {
	case "closed":
		return "Closed — no longer hiring"
	case "retrieved":
		return "Source details retrieved"
	case "unsupported":
		return "No automatic extractor for this source"
	case "failed":
		return "Source lookup failed; saved values retained"
	default:
		return "Not checked"
	}
}

func (j Job) MissingPostingDetails() []string {
	var missing []string

	if strings.TrimSpace(j.Title) == "" {
		missing = append(missing, "Job title")
	}

	if strings.TrimSpace(j.Company) == "" {
		missing = append(missing, "Employer name")
	}

	if strings.TrimSpace(j.Description) == "" {
		missing = append(missing, "Job description")
	}

	return missing
}

func (a *App) saveJob(job Job) error {
	_, err := insertJob(a.db, job)
	return err
}

// jobInserter supports ordinary database writes and transactional batch intake.
type jobInserter interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func insertJob(db jobInserter, job Job) (sql.Result, error) {
	stage := StageReview
	if job.SourceStatus == "closed" {
		stage = StageClosed
	}
	result, err := db.Exec(
		`INSERT INTO jobs (url, host, title, description, company, stage, priority,
            salary_text, salary_origin, title_manual, description_manual, company_manual, salary_manual,
            source_status, source_note, source_checked_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.URL, job.Host, job.Title, job.Description, job.Company,
		stage, PriorityNormal, job.SalaryText, job.SalaryOrigin,
		job.TitleManual, job.DescriptionManual, job.CompanyManual, job.SalaryManual,
		job.SourceStatus, job.SourceNote, job.SourceCheckedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("save job: %w", err)
	}
	return result, nil
}

func (a *App) allJobs() ([]Job, error) {
	rows, err := a.db.Query(
		"SELECT id, url, host, title, description, company, stage, priority, salary_text, salary_origin, title_manual, description_manual, company_manual, salary_manual, source_status, source_note, source_checked_at, COALESCE(selected_resume_id,0), (SELECT COUNT(*) FROM submissions WHERE job_id=jobs.id), COALESCE((SELECT MAX(submitted_at) FROM submissions WHERE job_id=jobs.id),'') FROM jobs ORDER BY id",
	)
	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job
		if err := rows.Scan(&job.ID, &job.URL, &job.Host, &job.Title, &job.Description, &job.Company, &job.Stage, &job.Priority, &job.SalaryText, &job.SalaryOrigin, &job.TitleManual, &job.DescriptionManual, &job.CompanyManual, &job.SalaryManual, &job.SourceStatus, &job.SourceNote, &job.SourceCheckedAt, &job.SelectedResumeID, &job.SubmissionCount, &job.LastSubmittedAt); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs: %w", err)
	}
	return jobs, nil
}

func (a *App) jobByID(id int64) (Job, error) {
	var job Job
	err := a.db.QueryRow(
		"SELECT id, url, host, title, description, company, stage, priority, salary_text, salary_origin, title_manual, description_manual, company_manual, salary_manual, source_status, source_note, source_checked_at, COALESCE(selected_resume_id,0), (SELECT COUNT(*) FROM submissions WHERE job_id=jobs.id), COALESCE((SELECT MAX(submitted_at) FROM submissions WHERE job_id=jobs.id),'') FROM jobs WHERE id = ?", id,
	).Scan(&job.ID, &job.URL, &job.Host, &job.Title, &job.Description, &job.Company, &job.Stage, &job.Priority, &job.SalaryText, &job.SalaryOrigin, &job.TitleManual, &job.DescriptionManual, &job.CompanyManual, &job.SalaryManual, &job.SourceStatus, &job.SourceNote, &job.SourceCheckedAt, &job.SelectedResumeID, &job.SubmissionCount, &job.LastSubmittedAt)
	if err != nil {
		return Job{}, fmt.Errorf("load job %d: %w", id, err)
	}
	return job, nil
}

func (a *App) updateJob(job Job) error {
	if !validStage(job.Stage) || !validPriority(job.Priority) {
		return fmt.Errorf("invalid job stage or priority")
	}
	if job.Stage == StageApplied {
		var exists bool
		if err := a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM submissions WHERE job_id=?)`, job.ID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrAppliedWithoutSubmission
		}
	}
	result, err := a.db.Exec(
		`UPDATE jobs SET
            title_manual = CASE WHEN title <> ? THEN 1 ELSE title_manual END,
            company_manual = CASE WHEN company <> ? THEN 1 ELSE company_manual END,
            description_manual = CASE WHEN description <> ? THEN 1 ELSE description_manual END,
            salary_manual = CASE WHEN salary_text <> ? THEN 1 ELSE salary_manual END,
            salary_origin = CASE WHEN salary_text <> ? THEN CASE WHEN ? = '' THEN '' ELSE 'manual' END ELSE salary_origin END,
            title = ?, company = ?, description = ?, salary_text = ?, stage = CASE WHEN EXISTS (SELECT 1 FROM submissions WHERE job_id=jobs.id) THEN 'applied' ELSE ? END, priority = ?
            WHERE id = ?`,
		job.Title, job.Company, job.Description, job.SalaryText, job.SalaryText, job.SalaryText,
		job.Title, job.Company, job.Description, job.SalaryText, job.Stage, job.Priority, job.ID,
	)
	if err != nil {
		return fmt.Errorf("update job %d: %w", job.ID, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check job update: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// applyPostingRefresh atomically applies an employer observation without touching manually
// edited values. A failed lookup updates evidence only, never the saved job content.
func (a *App) applyPostingRefresh(id int64, metadata platforms.Metadata, status, note, checkedAt string) error {
	salaryOrigin := metadata.SalaryOrigin
	if salaryOrigin == "" {
		salaryOrigin = "employer"
	}
	tx, err := a.db.Begin()
	if err != nil {
		return fmt.Errorf("begin posting refresh for job %d: %w", id, err)
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE jobs SET
			stage = CASE WHEN ? = 'closed' THEN 'closed' ELSE stage END,
            title = CASE WHEN ? = 'retrieved' AND title_manual = 0 AND ? <> '' THEN ? ELSE title END,
            company = CASE WHEN ? = 'retrieved' AND company_manual = 0 AND ? <> '' THEN ? ELSE company END,
            description = CASE WHEN ? = 'retrieved' AND description_manual = 0 AND ? <> '' THEN ? ELSE description END,
            salary_text = CASE WHEN ? = 'retrieved' AND salary_manual = 0 AND ? <> '' THEN ? ELSE salary_text END,
            salary_origin = CASE WHEN ? = 'retrieved' AND salary_manual = 0 AND ? <> '' THEN ? ELSE salary_origin END,
            source_status = ?, source_note = ?, source_checked_at = ?
            WHERE id = ?`,
		status,
		status, metadata.Title, metadata.Title,
		status, metadata.Company, metadata.Company,
		status, metadata.Description, metadata.Description,
		status, metadata.SalaryText, metadata.SalaryText,
		status, metadata.SalaryText, salaryOrigin,
		status, note, checkedAt, id)
	if err != nil {
		return fmt.Errorf("refresh job %d: %w", id, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check refreshed job: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	if status == "retrieved" {
		var savedURL string
		var descriptionManual, salaryManual bool
		if err := tx.QueryRow(`SELECT url, description_manual, salary_manual FROM jobs WHERE id=?`, id).Scan(&savedURL, &descriptionManual, &salaryManual); err != nil {
			return fmt.Errorf("load refreshed job ownership for %d: %w", id, err)
		}
		source := usablePostingSource(metadata.Source, savedURL, metadata.Description)
		descriptionConfirmed := metadata.Description != "" && !descriptionManual
		salaryConfirmed := metadata.SalaryText != "" && !salaryManual
		if err := saveJobPostingSource(tx, id, savedURL, metadata.Description, checkedAt, source, descriptionConfirmed, salaryConfirmed); err != nil {
			return fmt.Errorf("save posting source during refresh: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit posting refresh for job %d: %w", id, err)
	}
	return nil
}
