package app

import (
	"database/sql"
	"errors"
	"fmt"
	"io"

	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	browser "github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/documents"
)

type jobWorkspace struct {
	OtherBrowser        *Job
	SavedBrowser        *Inspection
	NextJob             *NextAction
	Preparation         HandoffPresentation
	QuestionSnapshot    applicationQuestionSnapshot
	Readiness           applicationReadiness
	TransferNotice      string
	SubmissionNotice    string
	SubmissionUncertain bool
	Questions           questionPreparation
	EarlyQuestions      earlyQuestionPreparation
	Job
	Resumes              []Resume
	ResumeOptions        []documentRow
	Submissions          []Submission
	Desktop              bool
	SubmissionToken      string
	SelectedResume       *Resume
	ResumeSelectionValid bool // Presentation only; commands still validate the file and hash.
	AnswerEditingAllowed bool
	Handoff              HandoffEligibility
	Profile              CandidateProfile
	BrowserState         browserRunState
	Sources              coverLetterSourcePreview
	Letter               CoverLetter
	Letters              []CoverLetterSummary
	LetterNotice         string
	LetterSaved          bool
	LetterOpen           bool   // Expand when entering from generation, editing, or a specific draft.
	WritingAnswerContext string // Presentation context: "early" or "live"; does not change answer authority.
	LetterModel          string
	ProviderReady        map[string]bool
	LetterResumes        []documentRow
	Match                MatchRecommendation
	HasFamilies          bool
}

type HandoffEligibility struct {
	Allowed bool
	Reason  string
}

func assessResumeHandoff(job Job, resume *Resume) HandoffEligibility {
	switch {
	case job.SubmissionCount > 0 || job.Stage == StageApplied:
		return HandoffEligibility{
			Reason: "This opportunity is marked applied. Review its history.",
		}

	case job.Stage == StageClosed:
		return HandoffEligibility{
			Reason: "This opportunity is closed in YourMove.",
		}

	case job.SelectedResumeID == 0:
		return HandoffEligibility{
			Reason: "Select a resume before starting a handoff.",
		}

	case resume == nil || resume.ID != job.SelectedResumeID:
		return HandoffEligibility{
			Reason: "The selected resume is unavailable. Choose a valid document.",
		}
	case resume.ArchivedAt != "":
		return HandoffEligibility{
			Reason: "This resume is archived. Select its current version before starting another handoff.",
		}

	case !browser.OwnsPostingURL(job.URL, job.URL):
		return HandoffEligibility{
			Reason: "Automatic filling supports Ashby, Greenhouse, and Lever postings. Use Open application for other sites.",
		}

	default:
		return HandoffEligibility{
			Allowed: true,
			Reason:  "Resume handoff is available. Explicit approval is required.",
		}
	}
}

// coverLetterSources is a read-only inspection of verified career evidence.
// This previews sources; it does not generate text, call a model, or upload files.
type resumeSourcePreview struct {
	Name       string
	Version    int
	Excerpt    string
	Characters int
	Truncated  bool
	Message    string
	Ready      bool
}

type coverLetterSourcePreview struct {
	Master              resumeSourcePreview
	Role                resumeSourcePreview
	Ready               bool
	JobDescriptionReady bool
}

func (a *App) previewResumeSource(resume Resume) resumeSourcePreview {
	out := resumeSourcePreview{Name: resume.Name, Version: resume.Version}
	if resume.ArchivedAt != "" {
		out.Message = "This version is archived. Select an active version for new cover letters."
		return out
	}
	text, err := a.extractResumeText(resume)
	if err != nil {
		// Show a useful explanation without revealing filesystem paths or
		// unexpected internal errors in the desktop application.
		if resume.Kind == ".pdf" {
			out.Message = "This PDF has no readable text or could not be parsed. Choose a DOCX resume below for this letter."
		} else {
			out.Message = "This document's text could not be read. Use another DOCX or PDF for this letter."
		}
		return out
	}
	runes := []rune(text)
	out.Characters = len(runes)
	const excerptLimit = 32000
	if len(runes) > excerptLimit {
		runes = runes[:excerptLimit]
		out.Truncated = true
	}
	out.Excerpt = string(runes)
	out.Ready = true
	return out
}

func (a *App) coverLetterSources(job Job, selected *Resume) coverLetterSourcePreview {
	sources := coverLetterSourcePreview{JobDescriptionReady: strings.TrimSpace(job.Description) != ""}
	sources.Master.Message = "Designate an active master resume in the document library."
	master, err := a.masterResume()
	if err == nil {
		sources.Master = a.previewResumeSource(master)
	} else if !errors.Is(err, sql.ErrNoRows) {
		sources.Master.Message = "Unable to load the master resume."
	}
	sources.Role.Message = "Choose an active role-specific resume for this job."
	if selected != nil {
		sources.Role = a.previewResumeSource(*selected)
	}
	sources.Ready = sources.Master.Ready && sources.Role.Ready && sources.JobDescriptionReady
	return sources
}

type documentRow struct {
	Resume
	FamilyName string
}

type resumeLibrary struct {
	Resumes  []Resume
	Rows     []documentRow
	Families []RoleFamily
	Desktop  bool
	Notice   string
}

func (a *App) resumeLibrary(w http.ResponseWriter, r *http.Request) {
	// loadResumeLibrary is shared with GET /api/resumes so the legacy page
	// and the React slice render the same ordering and family mapping.
	rows, families, err := a.loadResumeLibrary()
	if err != nil {
		http.Error(w, "Unable to load resumes", 500)
		return
	}
	resumes := make([]Resume, 0, len(rows))
	for _, row := range rows {
		resumes = append(resumes, row.Resume)
	}
	notice := map[string]string{
		"in-use":    "This resume is referenced by a job, submission, or saved cover letter. Archive it to remove it from active choices without losing application evidence.",
		"master":    "Remove the master designation or upload a new master revision before deleting this document.",
		"archived":  "Resume archived. Existing job assignments and submission history are unchanged.",
		"restored":  "Resume restored. Assign it to a role family or designate it as master if needed.",
		"revised":   "New revision saved. Existing job assignments still point to the original document until you update them.",
		"duplicate": "That file is already in your document library. A revision must contain different document bytes.",
		"failed":    "YourMove could not complete that document change. The current library remains available.",
	}[r.URL.Query().Get("notice")]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.resources.resumesTemplate.Execute(w, resumeLibrary{Resumes: resumes, Rows: rows, Families: families, Desktop: a.pickResume != nil, Notice: notice}); err != nil {
		log.Printf("render resume library: %v", err)
	}
}

func (a *App) uploadResume(w http.ResponseWriter, r *http.Request) {
	if a.resumeDir == "" {
		http.Error(w, "Storage unavailable", 500)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, documents.MaxResumeSize+(1<<20))
	if err := r.ParseMultipartForm(64 << 10); err != nil {
		http.Error(w, "Invalid or oversized upload", http.StatusBadRequest)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, h, err := r.FormFile("resume")
	if err != nil {
		http.Error(w, "Choose a PDF or DOCX", http.StatusBadRequest)
		return
	}
	defer file.Close()
	if _, err = a.importResume(h.Filename, file); err != nil {
		http.Error(w, "Cannot import resume: "+err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/resumes", http.StatusSeeOther)
}

// nativeResumeImport is enabled only by the Wails entry point. It does not
// accept arbitrary filesystem paths from the browser or from form fields.
func (a *App) nativeResumeImport(w http.ResponseWriter, r *http.Request) {
	if a.pickResume == nil {
		http.NotFound(w, r)
		return
	}
	if !isDesktopRequest(r) {
		http.Error(w, "Desktop window required", http.StatusForbidden)
		return
	}
	path, err := a.pickResume()
	if err != nil {
		log.Printf("native resume picker: %v", err)
		http.Error(w, "File selection failed", 500)
		return
	}
	if path == "" {
		http.Redirect(w, r, "/resumes", http.StatusSeeOther)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "Unable to open selected file", 400)
		return
	}
	defer f.Close()
	if _, err = a.importResume(filepath.Base(path), f); err != nil {
		http.Error(w, "Cannot import resume: "+err.Error(), 400)
		return
	}
	http.Redirect(w, r, "/resumes", http.StatusSeeOther)
}

func (a *App) chooseMasterResume(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	raw := r.PostForm.Get("master")
	if raw != "yes" && raw != "no" {
		http.Redirect(w, r, "/resumes?notice=failed", http.StatusSeeOther)
		return
	}
	err = a.setMasterResume(id, raw == "yes")
	switch {
	case errors.Is(err, sql.ErrNoRows):
		http.NotFound(w, r)
	case errors.Is(err, ErrResumeInUse):
		http.Redirect(w, r, "/resumes?notice=in-use", http.StatusSeeOther)
	case errors.Is(err, ErrResumeArchived):
		http.Redirect(w, r, "/resumes?notice=archived", http.StatusSeeOther)
	case err != nil:
		log.Printf("set master resume: %v", err)
		http.Error(w, "Unable to update master resume", 500)
	default:
		http.Redirect(w, r, "/resumes", http.StatusSeeOther)
	}
}

func (a *App) deleteResumeHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	if r.PostForm.Get("confirm") != "yes" || r.PostForm.Get("confirm_phrase") != "DELETE" {
		http.Redirect(w, r, "/resumes?notice=failed", http.StatusSeeOther)
		return
	}
	err = a.deleteResume(id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		http.NotFound(w, r)
	case errors.Is(err, ErrResumeInUse):
		http.Redirect(w, r, "/resumes?notice=in-use", http.StatusSeeOther)
	case errors.Is(err, ErrMasterResume):
		http.Redirect(w, r, "/resumes?notice=master", http.StatusSeeOther)
	case err != nil:
		log.Printf("delete resume: %v", err)
		http.Error(w, "Unable to delete resume", 500)
	default:
		http.Redirect(w, r, "/resumes", http.StatusSeeOther)
	}
}

func (a *App) changeResumeArchive(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	action := r.PostForm.Get("action")
	if action != "archive" && action != "restore" {
		http.Redirect(w, r, "/resumes?notice=failed", http.StatusSeeOther)
		return
	}
	if action == "archive" && r.PostForm.Get("confirm") != "yes" {
		http.Redirect(w, r, "/resumes?notice=failed", http.StatusSeeOther)
		return
	}
	err = a.setResumeArchived(id, action == "archive")
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("archive resume: %v", err)
		http.Redirect(w, r, "/resumes?notice=failed", http.StatusSeeOther)
		return
	}
	notice := "archived"
	if action == "restore" {
		notice = "restored"
	}
	http.Redirect(w, r, "/resumes?notice="+notice, http.StatusSeeOther)
}

func (a *App) reviseResumeBrowser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, documents.MaxResumeSize+(1<<20))
	if err := r.ParseMultipartForm(64 << 10); err != nil {
		http.Redirect(w, r, "/resumes?notice=failed", http.StatusSeeOther)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	f, h, err := r.FormFile("resume")
	if err != nil {
		http.Redirect(w, r, "/resumes?notice=failed", http.StatusSeeOther)
		return
	}
	defer f.Close()
	a.completeResumeRevision(w, r, id, h.Filename, f)
}

func (a *App) reviseResumeNative(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	if a.pickResume == nil || !isDesktopRequest(r) {
		http.Error(w, "Desktop window required", http.StatusForbidden)
		return
	}
	path, err := a.pickResume()
	if err != nil {
		http.Redirect(w, r, "/resumes?notice=failed", http.StatusSeeOther)
		return
	}
	if path == "" {
		http.Redirect(w, r, "/resumes", http.StatusSeeOther)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		http.Redirect(w, r, "/resumes?notice=failed", http.StatusSeeOther)
		return
	}
	defer f.Close()
	a.completeResumeRevision(w, r, id, filepath.Base(path), f)
}

func (a *App) completeResumeRevision(w http.ResponseWriter, r *http.Request, id int64, filename string, reader io.Reader) {
	_, err := a.reviseResume(id, filename, reader)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		http.NotFound(w, r)
	case errors.Is(err, ErrDuplicateResumeRevision):
		http.Redirect(w, r, "/resumes?notice=duplicate", http.StatusSeeOther)
	case errors.Is(err, ErrResumeArchived):
		http.Redirect(w, r, "/resumes?notice=archived", http.StatusSeeOther)
	case err != nil:
		log.Printf("revise resume: %v", err)
		http.Redirect(w, r, "/resumes?notice=failed", http.StatusSeeOther)
	default:
		http.Redirect(w, r, "/resumes?notice=revised", http.StatusSeeOther)
	}
}

func (a *App) downloadResume(w http.ResponseWriter, r *http.Request) {
	// Read-only private user files should not be usable as cross-site resources.
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "Forbidden", 403)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	resume, err := a.resumeByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if filepath.Base(resume.StoredName) != resume.StoredName || resume.StoredName == "" {
		http.NotFound(w, r)
		return
	}
	body, err := a.readResumeBytes(resume)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Stored resume failed integrity verification", 500)
		return
	}
	typ := "application/pdf"
	if resume.Kind == ".docx" {
		typ = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}
	profile, err := a.candidateProfile()
	if err != nil {
		http.Error(w, "Unable to read candidate information", 500)
		return
	}
	var attachmentJob Job
	if rawJobID := r.URL.Query().Get("job_id"); rawJobID != "" {
		jobID, parseErr := strconv.ParseInt(rawJobID, 10, 64)
		if parseErr != nil || jobID <= 0 {
			http.NotFound(w, r)
			return
		}
		attachmentJob, err = a.jobByID(jobID)
		if err != nil || attachmentJob.SelectedResumeID != resume.ID {
			http.NotFound(w, r)
			return
		}
	} else if resume.RoleFamilyID != 0 {
		families, err := a.allFamilies()
		if err != nil {
			http.Error(w, "Unable to read resume role", 500)
			return
		}
		for _, family := range families {
			if family.ID == resume.RoleFamilyID {
				attachmentJob.Title = family.Name
				break
			}
		}
	}
	filename := attachmentFilename(profile, attachmentJob, "Resume", resume.Kind)
	if a.exportDesktopDocument(w, r, filename, body, "/resumes") {
		return
	}
	w.Header().Set("Content-Type", typ)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(body)
}

func (a *App) setJobResume(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", 400)
		return
	}
	resumeID := int64(0)
	if raw := r.PostForm.Get("resume_id"); raw != "" {
		resumeID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || resumeID < 0 {
			http.Error(w, "Invalid resume", 400)
			return
		}
	}
	if err := a.assignResume(id, resumeID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Unknown job or resume", 404)
			return
		}
		http.Error(w, "Unable to assign resume", 500)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/jobs/%d#prepare", id), http.StatusSeeOther)
}

func parseJobID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("bad job id")
	}
	return id, nil
}
