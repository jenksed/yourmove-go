package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	browser "example.com/yourmove-lite/internal/application/browser"
)

var resumesTemplate = template.Must(template.ParseFiles("templates/resumes.html", "templates/partials.html"))

type jobWorkspace struct {
	Job
	Resumes         []Resume
	ResumeOptions   []documentRow
	Submissions     []Submission
	Desktop         bool
	SubmissionToken string
	SelectedResume  *Resume
	Handoff         HandoffEligibility
	Match           MatchRecommendation
	HasFamilies     bool
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

	case !browser.OwnsAshbyURL(job.URL, job.URL):
		return HandoffEligibility{
			Reason: "Managed resume handoff currently supports Ashby applications.",
		}

	default:
		return HandoffEligibility{
			Allowed: true,
			Reason:  "Resume handoff is available. Explicit approval is required.",
		}
	}
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
}

func (a *App) resumeLibrary(w http.ResponseWriter, r *http.Request) {
	resumes, err := a.allResumes()
	if err != nil {
		http.Error(w, "Unable to load resumes", 500)
		return
	}
	families, err := a.allFamilies()
	if err != nil {
		http.Error(w, "Unable to load role families", 500)
		return
	}
	names := make(map[int64]string, len(families))
	for _, f := range families {
		names[f.ID] = f.Name
	}
	rows := make([]documentRow, 0, len(resumes))
	for _, r := range resumes {
		rows = append(rows, documentRow{Resume: r, FamilyName: names[r.RoleFamilyID]})
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := resumesTemplate.Execute(w, resumeLibrary{Resumes: resumes, Rows: rows, Families: families, Desktop: a.pickResume != nil}); err != nil {
		log.Printf("render resume library: %v", err)
	}
}

func (a *App) uploadResume(w http.ResponseWriter, r *http.Request) {
	if a.resumeDir == "" {
		http.Error(w, "Storage unavailable", 500)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxResumeSize+(1<<20))
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
	if r.Host != "localhost" || r.Header.Get("X-Wails-Window-Id") == "" || r.Header.Get("Origin") != "http://localhost" {
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
	file, err := os.Open(filepath.Join(a.resumeDir, resume.StoredName))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	// Never download altered or truncated artifacts under a verified resume identity.
	body, err := io.ReadAll(io.LimitReader(file, maxResumeSize+1))
	if err != nil || int64(len(body)) != resume.Size || len(body) > maxResumeSize {
		http.Error(w, "Stored resume failed integrity verification", http.StatusInternalServerError)
		return
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != resume.SHA256 {
		http.Error(w, "Stored resume failed integrity verification", http.StatusInternalServerError)
		return
	}
	typ := "application/pdf"
	if resume.Kind == ".docx" {
		typ = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}
	w.Header().Set("Content-Type", typ)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", resume.Name))
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
	http.Redirect(w, r, fmt.Sprintf("/jobs/%d", id), http.StatusSeeOther)
}

func (a *App) submitJob(w http.ResponseWriter, r *http.Request) {
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
	if r.PostForm.Get("confirm") != "yes" {
		http.Error(w, "Confirm submission completed on employer website", 400)
		return
	}
	notes := strings.TrimSpace(r.PostForm.Get("notes"))
	if len(notes) > 1000 {
		http.Error(w, "Notes too long", 400)
		return
	}
	token := r.PostForm.Get("submission_token")
	if err := a.recordSubmission(id, notes, token); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		log.Printf("record application: %v", err)
		http.Error(w, "Unable to record application", 500)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/jobs/%d", id), http.StatusSeeOther)
}

func parseJobID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("bad job id")
	}
	return id, nil
}
