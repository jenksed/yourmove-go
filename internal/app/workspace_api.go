// YM-004A Application Workspace JSON transport.
//
// Coherent internal/app implementation (no feature-package extraction):
// resume writes share intakeMu and the jobs table (assignResume), and
// submission/cover-letter history constrains deletion and Applied status.
// Tracing callers showed extraction would split that coordination or
// duplicate the Job/document models, failing the FEATURE_OWNERSHIP
// extraction rule. The bounded improvement here is three narrow JSON
// contracts reusing existing Go decisions and durable writes:
//
//   - GET  /api/jobs/{id}/workspace  read-only snapshot, no submission token
//   - POST /api/jobs/{id}/details    applicant-owned edits via updateJob
//   - POST /api/jobs/{id}/refresh    metadata refresh via refreshPosting
//
// Existing endpoints are reused unchanged: GET /api/jobs/{id}/resume-options
// and POST /api/jobs/{id}/resume (assignResume), GET /api/resumes, and the
// preparation summaries (prepareReadiness, summarizePreparation). The legacy
// GET /jobs/{id} page keeps the submission-review token mechanism for the
// eventual submission workflow; the React read contract MUST NOT create one.

package app

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
)

const workspaceRefreshDisclaimer = "A metadata refresh reports saved posting details only; it does not verify whether the employer is still accepting applications."

type workspaceJobJSON struct {
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
	SalaryText       string `json:"salaryText"`
	SalaryOrigin     string `json:"salaryOrigin"`
	SalaryOriginLabl string `json:"salaryOriginLabel"`
	SourceStatus     string `json:"sourceStatus"`
	SourceStatusLabl string `json:"sourceStatusLabel"`
	SourceNote       string `json:"sourceNote"`
	SourceCheckedAt  string `json:"sourceCheckedAt"`
	SelectedResumeID int64  `json:"selectedResumeId"`
	SubmissionCount  int64  `json:"submissionCount"`
	LastSubmittedAt  string `json:"lastSubmittedAt"`
	CompanyOrHost    string `json:"companyOrHost"`
	DisplayTitle     string `json:"displayTitle"`
	// Provider is the Go-owned ATS platform key ("" when unrecognized) with
	// its accessible label, identical to the Opportunities list verdict.
	// React renders these verbatim.
	Provider       string   `json:"provider"`
	ProviderLabel  string   `json:"providerLabel"`
	MissingDetails []string `json:"missingDetails"`
	TitleManual    bool     `json:"titleManual"`
	CompanyManual  bool     `json:"companyManual"`
	DescriptionMan bool     `json:"descriptionManual"`
	SalaryManual   bool     `json:"salaryManual"`
}

func toWorkspaceJob(job Job) workspaceJobJSON {
	companyOrHost := strings.TrimSpace(job.Company)
	if companyOrHost == "" {
		companyOrHost = job.Host
	}
	displayTitle := job.DisplayTitle()
	return workspaceJobJSON{
		ID: job.ID, URL: job.URL, Host: job.Host, Title: job.Title,
		Company: job.Company, Description: job.Description, Stage: job.Stage,
		StageLabel: job.StageLabel(), Priority: job.Priority, PriorityLabel: job.PriorityLabel(),
		SalaryText: job.SalaryText, SalaryOrigin: job.SalaryOrigin, SalaryOriginLabl: job.SalaryOriginLabel(),
		SourceStatus: job.SourceStatus, SourceStatusLabl: job.SourceStatusLabel(),
		SourceNote: job.SourceNote, SourceCheckedAt: job.SourceCheckedAt,
		SelectedResumeID: job.SelectedResumeID, SubmissionCount: job.SubmissionCount,
		LastSubmittedAt: job.LastSubmittedAt, CompanyOrHost: companyOrHost, DisplayTitle: displayTitle,
		Provider: job.PostingProvider(), ProviderLabel: job.PostingProviderLabel(),
		MissingDetails: job.MissingPostingDetails(),
		TitleManual:    job.TitleManual, CompanyManual: job.CompanyManual,
		DescriptionMan: job.DescriptionManual, SalaryManual: job.SalaryManual,
	}
}

type workspaceResumeJSON struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Size           int64  `json:"size"`
	DisplaySize    string `json:"displaySize"`
	ImportedAt     string `json:"importedAt"`
	RoleFamilyID   int64  `json:"roleFamilyId"`
	RoleFamilyName string `json:"roleFamilyName"`
	IsPrimary      bool   `json:"isPrimary"`
	Archived       bool   `json:"archived"`
	Version        int    `json:"version"`
	Selected       bool   `json:"selected"`
	TransferReady  bool   `json:"transferReady"`
	TransferNote   string `json:"transferNote"`
}

type workspaceMatchJSON struct {
	Status          string `json:"status"`
	FamilyName      string `json:"familyName"`
	ResumeID        int64  `json:"resumeId"`
	ResumeName      string `json:"resumeName"`
	TitleEvidence   string `json:"titleEvidence"`
	ContextEvidence string `json:"contextEvidence"`
}

type workspaceReadinessJSON struct {
	ResumeReady        bool   `json:"resumeReady"`
	ResumeUnavailable  bool   `json:"resumeUnavailable"`
	ProfileApproved    bool   `json:"profileApproved"`
	QuestionsAvailable bool   `json:"questionsAvailable"`
	NeedsAnswerCount   int    `json:"needsAnswerCount"`
	PreparedCount      int    `json:"preparedCount"`
	ProfileCount       int    `json:"profileCount"`
	EmployerOnlyCount  int    `json:"employerOnlyCount"`
	ManualReviewCount  int    `json:"manualReviewCount"`
	ManualWorkCount    int    `json:"manualWorkCount"`
	QuestionSource     string `json:"questionSource"`
	QuestionPartial    bool   `json:"questionPartial"`
	QuestionAnchor     string `json:"questionAnchor"`
	CoverLetterSaved   bool   `json:"coverLetterSaved"`
	BrowserStatus      string `json:"browserStatus"`
}

type workspacePrepJSON struct {
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Recovery bool   `json:"recovery"`
	HasScope bool   `json:"hasScope"`
	External bool   `json:"external"`
}

type workspaceNextActionJSON struct {
	Activity string `json:"activity"`
	Label    string `json:"label"`
}

// chooseWorkspaceNextAction derives one optional navigation suggestion from
// already loaded facts. It grants no authority, assesses no new requirements,
// and never substitutes for queue triage or transfer-eligibility decisions.
// Precedence follows the S1 contract: recorded history outranks stage,
// browser/recovery state outranks preparation gaps, then resume validity,
// then observed unanswered questions, otherwise general application options.
func chooseWorkspaceNextAction(job Job, state browserRunState, resumeValid bool, readiness applicationReadiness, preparation HandoffPresentation) workspaceNextActionJSON {
	if job.SubmissionCount > 0 {
		return workspaceNextActionJSON{Activity: "applied", Label: "View submission history"}
	}
	if job.Stage == StageApplied {
		return workspaceNextActionJSON{Activity: "applied", Label: "Review application status"}
	}
	if job.Stage == StageClosed {
		return workspaceNextActionJSON{Activity: "job", Label: "Review opportunity"}
	}
	if state.Active || state.Phase == "opening" || preparation.Recovery || state.Phase == "interrupted" || state.Phase == "attention" {
		return workspaceNextActionJSON{Activity: "apply", Label: "Review application status"}
	}
	if job.SelectedResumeID == 0 {
		return workspaceNextActionJSON{Activity: "documents", Label: "Choose a résumé"}
	}
	if !resumeValid {
		return workspaceNextActionJSON{Activity: "documents", Label: "Review selected résumé"}
	}
	if readiness.QuestionsAvailable && readiness.NeedsAnswerCount > 0 {
		return workspaceNextActionJSON{Activity: "questions", Label: "Review available questions"}
	}
	return workspaceNextActionJSON{Activity: "apply", Label: "Review application options"}
}

// buildWorkspaceSnapshot assembles the read-only snapshot. It never mints a
// submission-review token and never writes durable state.
func (a *App) buildWorkspaceSnapshot(jobID int64) (map[string]any, error) {
	job, err := a.jobByID(jobID)
	if err != nil {
		return nil, err
	}
	resumes, err := a.allResumes()
	if err != nil {
		return nil, err
	}
	families, err := a.allFamilies()
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(families))
	for _, f := range families {
		names[f.ID] = f.Name
	}
	match := matchResume(job, families, resumes)
	options := resumeOptionsForJob(job, resumes, families)
	optionJSON := make([]workspaceResumeJSON, 0, len(options))
	for _, row := range options {
		transferReady := row.ArchivedAt == ""
		note := ""
		if transferReady {
			if _, fileErr := a.readResumeBytes(row.Resume); fileErr != nil {
				transferReady = false
				note = "The selected file is missing or changed. Select and save an available resume, or import it again in Documents."
			}
		} else {
			note = "This archived version remains for history. Select its current version for transfer."
		}
		if !transferReady && row.ID != job.SelectedResumeID {
			note = "Unavailable for new transfer. Choose an available document."
		}
		optionJSON = append(optionJSON, workspaceResumeJSON{
			ID: row.ID, Name: row.Name, Kind: row.Kind, Size: row.Size,
			DisplaySize: row.DisplaySize(), ImportedAt: row.ImportedAt,
			RoleFamilyID: row.RoleFamilyID, RoleFamilyName: names[row.RoleFamilyID],
			IsPrimary: row.IsPrimary, Archived: row.ArchivedAt != "",
			Version: row.Version, Selected: row.ID == job.SelectedResumeID,
			TransferReady: transferReady, TransferNote: note,
		})
	}
	var selected *Resume
	for i := range resumes {
		if resumes[i].ID == job.SelectedResumeID {
			selected = &resumes[i]
			break
		}
	}
	resumeValid := false
	if selected != nil && selected.ArchivedAt == "" {
		if _, fileErr := a.readResumeBytes(*selected); fileErr == nil {
			resumeValid = true
		}
	}
	handoff := assessResumeHandoff(job, selected)
	profile, err := a.candidateProfile()
	if err != nil {
		return nil, err
	}
	live, err := a.prepareQuestions(job, profile)
	if err != nil {
		return nil, err
	}
	early, err := a.prepareEarlyQuestions(job, nil)
	if err != nil {
		return nil, err
	}
	browserState := a.browsers.state(job.ID)
	snapshot := chooseApplicationQuestionSnapshot(job, browserState, live, early)
	letter, err := a.latestCoverLetter(job.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	readiness := prepareReadiness(resumeValid, profile, browserState, snapshot, letter)
	prepWorkspace := jobWorkspace{
		Job: job, BrowserState: browserState, Handoff: handoff,
		Questions: live, EarlyQuestions: early, Letter: letter, Profile: profile,
	}
	preparation := summarizePreparation(prepWorkspace)
	nextAction := chooseWorkspaceNextAction(job, browserState, resumeValid, readiness, preparation)
	submissions, err := a.jobSubmissions(job.ID)
	if err != nil {
		return nil, err
	}
	letters, err := a.listCoverLetters(job.ID)
	if err != nil {
		return nil, err
	}
	answerEditingAllowed := job.Stage != StageApplied && job.Stage != StageClosed && job.SubmissionCount == 0
	// Masters are never offered as transfer attachments; surface their
	// presence separately so the UI can explain, not select, them.
	masterCount := 0
	for _, r := range resumes {
		if r.IsMaster && r.ArchivedAt == "" {
			masterCount++
		}
	}
	return map[string]any{
		"job":              toWorkspaceJob(job),
		"resumeOptions":    optionJSON,
		"selectedResumeId": job.SelectedResumeID,
		// Distinct family count lets the workspace distinguish "no families
		// configured" from "no confident match" without another request.
		"familyCount": len(families),
		"suggestion": workspaceMatchJSON{
			Status: match.Status, FamilyName: match.FamilyName,
			ResumeID: match.ResumeID, ResumeName: match.ResumeName,
			TitleEvidence: match.TitleEvidence, ContextEvidence: match.ContextEvidence,
		},
		"handoff":     map[string]any{"allowed": handoff.Allowed, "reason": handoff.Reason},
		"resumeValid": resumeValid,
		"readiness": workspaceReadinessJSON{
			ResumeReady: readiness.ResumeReady, ResumeUnavailable: readiness.ResumeUnavailable,
			ProfileApproved: profile.Approved, QuestionsAvailable: readiness.QuestionsAvailable,
			NeedsAnswerCount: readiness.NeedsAnswerCount, PreparedCount: readiness.PreparedCount,
			ProfileCount: readiness.ProfileCount, EmployerOnlyCount: readiness.EmployerOnlyCount,
			ManualReviewCount: readiness.ManualReviewCount, ManualWorkCount: readiness.ManualWorkCount,
			QuestionSource: readiness.QuestionSource, QuestionPartial: readiness.QuestionPartial,
			QuestionAnchor: readiness.QuestionAnchor, CoverLetterSaved: readiness.CoverLetterSaved,
			// The underlying browser message may contain Chromium/runtime errors,
			// executable paths, or other implementation detail. The workspace
			// exposes only the coarse status; troubleshooting stays in the legacy UI.
			BrowserStatus: readiness.BrowserStatus,
		},
		"preparation": workspacePrepJSON{
			Title: preparation.Title, Detail: preparation.Detail,
			Recovery: preparation.Recovery, HasScope: preparation.Action != "",
			External: preparation.OpenExternally,
		},
		"nextAction": workspaceNextActionJSON{
			Activity: nextAction.Activity, Label: nextAction.Label,
		},
		"submissions":          map[string]any{"count": len(submissions)},
		"letters":              map[string]any{"count": len(letters), "saved": letter.ID > 0},
		"masterCount":          masterCount,
		"answerEditingAllowed": answerEditingAllowed,
		"desktop":              a.openPosting != nil,
		"legacyHref":           workspaceLegacyHref(job.ID),
		"refreshDisclaimer":    workspaceRefreshDisclaimer,
	}, nil
}

func workspaceLegacyHref(id int64) string {
	return "/jobs/" + strconv.FormatInt(id, 10) + "?legacy=1"
}

// GET /api/jobs/{id}/workspace is strictly read-only: it assembles Go-derived
// state without minting a submission-review token. The legacy GET /jobs/{id}
// page remains the only path that creates that token.
func (a *App) workspaceAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	snapshot, err := a.buildWorkspaceSnapshot(jobID)
	if errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	if err != nil {
		log.Printf("load workspace api %d: %v", jobID, err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to load application workspace")
		return
	}
	writeResumeJSON(w, http.StatusOK, snapshot)
}

// POST /api/jobs/{id}/details edits applicant-owned fields only. The source
// URL is immutable here; Applied still requires a confirmed submission.
func (a *App) workspaceDetailsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	var payload struct {
		Title       string `json:"title"`
		Company     string `json:"company"`
		Description string `json:"description"`
		SalaryText  string `json:"salary_text"`
		Stage       string `json:"stage"`
		Priority    string `json:"priority"`
	}
	if !decodeResumeJSON(w, r, 128<<10, &payload) {
		return
	}
	title := strings.TrimSpace(payload.Title)
	company := strings.TrimSpace(payload.Company)
	description := strings.TrimSpace(payload.Description)
	salary := strings.TrimSpace(payload.SalaryText)
	if len(title) > 200 || len(company) > 200 || len(description) > 120<<10 || len(salary) > 500 ||
		!validStage(payload.Stage) || !validPriority(payload.Priority) {
		writeResumeError(w, http.StatusBadRequest, "Invalid job details, stage, or priority")
		return
	}
	if err := a.updateJob(Job{ID: jobID, Title: title, Company: company, Description: description, SalaryText: salary, Stage: payload.Stage, Priority: payload.Priority}); err != nil {
		if errors.Is(err, ErrAppliedWithoutSubmission) {
			writeResumeError(w, http.StatusBadRequest, "Applied requires a confirmed submission")
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			writeResumeError(w, http.StatusNotFound, "Unknown job")
			return
		}
		log.Printf("update workspace details %d: %v", jobID, err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to update job")
		return
	}
	job, err := a.jobByID(jobID)
	if err != nil {
		log.Printf("reload workspace details %d: %v", jobID, err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to load updated job")
		return
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{"saved": true, "job": toWorkspaceJob(job)})
}

// POST /api/jobs/{id}/refresh reuses refreshPosting: manual edits are
// preserved and a failed lookup updates evidence only. The response carries
// the standing disclaimer so the UI never presents metadata as proof the
// employer accepts applications.
func (a *App) workspaceRefreshAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	job, err := a.jobByID(jobID)
	if errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	if err != nil {
		log.Printf("load workspace refresh %d: %v", jobID, err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to load job")
		return
	}
	if err := a.refreshPosting(r.Context(), job); err != nil {
		log.Printf("refresh workspace posting %d: %v", jobID, err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to save source result")
		return
	}
	updated, err := a.jobByID(jobID)
	if err != nil {
		log.Printf("reload workspace refresh %d: %v", jobID, err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to load refreshed job")
		return
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{
		"job": toWorkspaceJob(updated), "disclaimer": workspaceRefreshDisclaimer,
	})
}
