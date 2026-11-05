package app

// Resumes JSON transport for the YM-002 React slice.
//
// Go remains authoritative for validation, document identity, routing,
// archival, deletion refusals, and persistence. React owns presentation,
// disposable drafts, and destructive confirmations.
//
// Why this lives in internal/app instead of internal/features/resumes:
// resume writes share the single-writer intakeMu mutex, the jobs table
// (assignResume), and cross-feature transactions spanning submissions and
// cover letters (deleteResume, reviseResume, setMasterResume). Tracing
// actual callers showed extraction would split that coordination or
// duplicate the Job/document models across packages, failing the
// FEATURE_OWNERSHIP extraction rule (cohesive + traceable + reduces
// duplication now). The bounded Go improvement here is therefore a
// cohesive same-package refactor: loadResumeLibrary is the single loader
// used by both the legacy HTML library and this JSON API, storeResumeFile
// is the single private-file writer used by both import and revision, and
// importResumeData is the single import core used by both HTML and JSON
// transports.

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jenksed/yourmove-go/internal/documents"
)

// Single private-file writer shared by import and revision. Private bytes
// are exposed only after the database row commits; callers remove the file
// when their transaction fails.
func storeResumeFile(dir, kind string, data []byte) (string, error) {
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return "", err
	}
	storedName := hex.EncodeToString(idBytes) + kind
	path := filepath.Join(dir, storedName)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	writeErr, closeErr := func() (error, error) {
		_, werr := file.Write(data)
		if werr == nil {
			werr = file.Sync()
		}
		return werr, file.Close()
	}()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		return "", &resumeFileWriteError{writeErr: writeErr, closeErr: closeErr}
	}
	return storedName, nil
}

type resumeFileWriteError struct {
	writeErr error
	closeErr error
}

func (e *resumeFileWriteError) Error() string {
	return "write resume: " + errorPair(e.writeErr, e.closeErr)
}

func errorPair(a, b error) string {
	if a != nil && b != nil {
		return a.Error() + " " + b.Error()
	}
	if a != nil {
		return a.Error()
	}
	if b != nil {
		return b.Error()
	}
	return "unknown write failure"
}

// importResumeData owns the single import core used by the legacy HTML form
// and the React JSON transport. It reports whether the bytes were already
// registered so both transports can describe duplicates truthfully without
// reimplementing hash, verification, or insert logic.
func (a *App) importResumeData(name string, data []byte) (Resume, bool, error) {
	if a.resumeDir == "" {
		return Resume{}, false, errors.New("resume storage is not configured")
	}
	name = documents.SanitizeFilename(name)
	if name == "" {
		return Resume{}, false, errors.New("invalid filename")
	}
	kind, err := documents.ValidateResume(name, data)
	if err != nil {
		return Resume{}, false, err
	}
	sum := sha256Of(data)
	if existing, err := a.resumeByHash(sum); err == nil {
		verified, verr := a.verifiedImportedResume(existing)
		return verified, true, verr
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Resume{}, false, err
	}
	storedName, err := storeResumeFile(a.resumeDir, kind, data)
	if err != nil {
		return Resume{}, false, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := a.db.Exec(`INSERT INTO resumes (name,stored_name,sha256,size,kind,imported_at) VALUES (?,?,?,?,?,?)`, name, storedName, sum, len(data), kind, now)
	if err != nil {
		_ = os.Remove(filepath.Join(a.resumeDir, storedName))
		if existing, e := a.resumeByHash(sum); e == nil {
			verified, verr := a.verifiedImportedResume(existing)
			return verified, true, verr
		}
		return Resume{}, false, &resumeSaveError{err: err}
	}
	rowID, err := result.LastInsertId()
	if err != nil {
		_ = os.Remove(filepath.Join(a.resumeDir, storedName))
		return Resume{}, false, err
	}
	return Resume{ID: rowID, Name: name, StoredName: storedName, SHA256: sum, Size: int64(len(data)), Kind: kind, ImportedAt: now}, false, nil
}

type resumeSaveError struct{ err error }

func (e *resumeSaveError) Error() string { return "save resume record: " + e.err.Error() }
func (e *resumeSaveError) Unwrap() error { return e.err }

// Single library loader shared by the legacy HTML page and the JSON API so
// family-name mapping and ordering cannot drift between transports.
func (a *App) loadResumeLibrary() ([]documentRow, []RoleFamily, error) {
	resumes, err := a.allResumes()
	if err != nil {
		return nil, nil, err
	}
	families, err := a.allFamilies()
	if err != nil {
		return nil, nil, err
	}
	names := make(map[int64]string, len(families))
	for _, f := range families {
		names[f.ID] = f.Name
	}
	rows := make([]documentRow, 0, len(resumes))
	for _, r := range resumes {
		rows = append(rows, documentRow{Resume: r, FamilyName: names[r.RoleFamilyID]})
	}
	return rows, families, nil
}

// JSON shapes use lowercase keys; the React client reads them verbatim.

type resumeJSON struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Size           int64  `json:"size"`
	DisplaySize    string `json:"displaySize"`
	ImportedAt     string `json:"importedAt"`
	RoleFamilyID   int64  `json:"roleFamilyId"`
	RoleFamilyName string `json:"roleFamilyName"`
	IsPrimary      bool   `json:"isPrimary"`
	IsMaster       bool   `json:"isMaster"`
	Archived       bool   `json:"archived"`
	ArchivedAt     string `json:"archivedAt"`
	SeriesID       int64  `json:"seriesId"`
	Version        int    `json:"version"`
}

type resumeFamilyJSON struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type resumeOptionJSON struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Size           int64  `json:"size"`
	DisplaySize    string `json:"displaySize"`
	ImportedAt     string `json:"importedAt"`
	RoleFamilyID   int64  `json:"roleFamilyId"`
	RoleFamilyName string `json:"roleFamilyName"`
	IsPrimary      bool   `json:"isPrimary"`
	IsMaster       bool   `json:"isMaster"`
	Archived       bool   `json:"archived"`
	ArchivedAt     string `json:"archivedAt"`
	SeriesID       int64  `json:"seriesId"`
	Version        int    `json:"version"`
	Selected       bool   `json:"selected"`
}

func toResumeOptionJSON(row documentRow, selected bool) resumeOptionJSON {
	base := toResumeJSON(row)
	return resumeOptionJSON{
		ID: base.ID, Name: base.Name, Kind: base.Kind, Size: base.Size,
		DisplaySize: base.DisplaySize, ImportedAt: base.ImportedAt,
		RoleFamilyID: base.RoleFamilyID, RoleFamilyName: base.RoleFamilyName,
		IsPrimary: base.IsPrimary, IsMaster: base.IsMaster,
		Archived: base.Archived, ArchivedAt: base.ArchivedAt,
		SeriesID: base.SeriesID, Version: base.Version, Selected: selected,
	}
}

func sha256Of(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func toResumeJSON(row documentRow) resumeJSON {
	return resumeJSON{
		ID: row.ID, Name: row.Name, Kind: row.Kind, Size: row.Size,
		DisplaySize: row.DisplaySize(), ImportedAt: row.ImportedAt,
		RoleFamilyID: row.RoleFamilyID, RoleFamilyName: row.FamilyName,
		IsPrimary: row.IsPrimary, IsMaster: row.IsMaster,
		Archived: row.ArchivedAt != "", ArchivedAt: row.ArchivedAt,
		SeriesID: row.SeriesID, Version: row.Version,
	}
}

func writeResumeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeResumeError(w http.ResponseWriter, status int, message string) {
	writeResumeJSON(w, status, map[string]string{"error": message})
}

func decodeResumeJSON(w http.ResponseWriter, r *http.Request, limit int64, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeResumeError(w, http.StatusBadRequest, "Invalid request body")
		return false
	}
	return true
}

func resumeAPIID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func resumeErrorStatus(err error) (int, string) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return http.StatusNotFound, "Unknown resume or job"
	case errors.Is(err, ErrResumeInUse):
		return http.StatusConflict, "This resume is referenced by a job, submission, or saved cover letter. Archive it instead of deleting it."
	case errors.Is(err, ErrMasterResume):
		return http.StatusConflict, "Remove the master designation before deleting this document."
	case errors.Is(err, ErrResumeArchived):
		return http.StatusConflict, "This resume version is archived. Restore it or use its current version."
	case errors.Is(err, ErrDuplicateResumeRevision):
		return http.StatusConflict, "That file is already in the document library. A revision must contain different document bytes."
	default:
		return 0, ""
	}
}

// GET /api/resumes lists the same library the Go page renders, with family
// routing and a desktop capability flag for the native file picker.
func (a *App) resumesAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rows, families, err := a.loadResumeLibrary()
	if err != nil {
		log.Printf("load resumes api: %v", err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to load resumes")
		return
	}
	resumes := make([]resumeJSON, 0, len(rows))
	for _, row := range rows {
		resumes = append(resumes, toResumeJSON(row))
	}
	familyJSON := make([]resumeFamilyJSON, 0, len(families))
	for _, f := range families {
		familyJSON = append(familyJSON, resumeFamilyJSON{ID: f.ID, Name: f.Name})
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{
		"resumes": resumes, "families": familyJSON,
		"desktop": a.pickResume != nil, "total": len(resumes),
	})
}

// GET /api/jobs/{id}/resume-options returns the same selectable documents
// the Prepare activity offers: masters are never employer attachments and
// archived versions appear only when already selected for that job.
func (a *App) resumeOptionsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	job, err := a.jobByID(jobID)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	resumes, err := a.allResumes()
	if err != nil {
		log.Printf("load resume options api: %v", err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to load resumes")
		return
	}
	families, err := a.allFamilies()
	if err != nil {
		log.Printf("load resume families api: %v", err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to load role families")
		return
	}
	options := resumeOptionsForJob(job, resumes, families)
	names := make(map[int64]string, len(families))
	for _, f := range families {
		names[f.ID] = f.Name
	}
	out := make([]resumeOptionJSON, 0, len(options))
	for _, row := range options {
		out = append(out, toResumeOptionJSON(
			documentRow{Resume: row.Resume, FamilyName: names[row.RoleFamilyID]},
			row.ID == job.SelectedResumeID,
		))
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{
		"job": map[string]any{
			"id": job.ID, "title": job.Title, "company": job.Company,
			"selectedResumeId": job.SelectedResumeID,
		},
		"options": out, "selectedResumeId": job.SelectedResumeID,
	})
}

// POST /api/jobs/{id}/resume accepts {resume_id} and reuses assignResume so
// fabricated IDs and master/archived documents are refused exactly like the
// legacy form handler.
func (a *App) setJobResumeAPI(w http.ResponseWriter, r *http.Request) {
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
		ResumeID int64 `json:"resume_id"`
	}
	if !decodeResumeJSON(w, r, 4<<10, &payload) {
		return
	}
	if payload.ResumeID < 0 {
		writeResumeError(w, http.StatusBadRequest, "Invalid resume")
		return
	}
	if err := a.assignResume(jobID, payload.ResumeID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeResumeError(w, http.StatusNotFound, "Unknown job or resume")
			return
		}
		log.Printf("assign resume api: %v", err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to assign resume")
		return
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{"jobId": jobID, "selectedResumeId": payload.ResumeID})
}

func resumeMultipartFile(w http.ResponseWriter, r *http.Request) (string, []byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, documents.MaxResumeSize+(1<<20))
	if err := r.ParseMultipartForm(64 << 10); err != nil {
		writeResumeError(w, http.StatusBadRequest, "Invalid or oversized upload")
		return "", nil, false
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("resume")
	if err != nil {
		writeResumeError(w, http.StatusBadRequest, "Choose a PDF or DOCX")
		return "", nil, false
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, documents.MaxResumeSize+1))
	if err != nil {
		writeResumeError(w, http.StatusBadRequest, "Unable to read upload")
		return "", nil, false
	}
	return header.Filename, data, true
}

// POST /api/resumes/import accepts multipart form field "resume" (PDF/DOCX,
// 10 MiB) and stores exact bytes under a private random name.
func (a *App) importResumeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name, data, ok := resumeMultipartFile(w, r)
	if !ok {
		return
	}
	resume, duplicate, err := a.importResumeData(name, data)
	if err != nil {
		writeResumeError(w, http.StatusBadRequest, "Cannot import resume: "+err.Error())
		return
	}
	status := http.StatusCreated
	if duplicate {
		status = http.StatusOK
	}
	writeResumeJSON(w, status, map[string]any{"id": resume.ID, "name": resume.Name, "duplicate": duplicate})
}

// POST /api/resumes/import-native is desktop-only and opens the trusted
// native file picker; it never accepts browser-supplied filesystem paths.
func (a *App) importResumeNativeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if a.pickResume == nil {
		writeResumeError(w, http.StatusNotFound, "Native file selection is unavailable")
		return
	}
	if !isDesktopRequest(r) {
		writeResumeError(w, http.StatusForbidden, "Desktop window required")
		return
	}
	path, err := a.pickResume()
	if err != nil {
		log.Printf("native resume picker api: %v", err)
		writeResumeError(w, http.StatusInternalServerError, "File selection failed")
		return
	}
	if strings.TrimSpace(path) == "" {
		writeResumeError(w, http.StatusBadRequest, "No file selected")
		return
	}
	file, err := os.Open(path)
	if err != nil {
		writeResumeError(w, http.StatusBadRequest, "Unable to open selected file")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, documents.MaxResumeSize+1))
	if err != nil {
		writeResumeError(w, http.StatusBadRequest, "Unable to read selected file")
		return
	}
	resume, duplicate, err := a.importResumeData(filepath.Base(path), data)
	if err != nil {
		writeResumeError(w, http.StatusBadRequest, "Cannot import resume: "+err.Error())
		return
	}
	status := http.StatusCreated
	if duplicate {
		status = http.StatusOK
	}
	writeResumeJSON(w, status, map[string]any{"id": resume.ID, "name": resume.Name, "duplicate": duplicate})
}

// POST /api/resumes/{id}/family accepts {family_id, primary} and updates
// routing only: explicit job selections and immutable submitted snapshots
// never change.
func (a *App) resumeFamilyAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, ok := resumeAPIID(r)
	if !ok {
		writeResumeError(w, http.StatusNotFound, "Unknown resume")
		return
	}
	var payload struct {
		FamilyID int64 `json:"family_id"`
		Primary  bool  `json:"primary"`
	}
	if !decodeResumeJSON(w, r, 4<<10, &payload) {
		return
	}
	if payload.FamilyID < 0 {
		writeResumeError(w, http.StatusBadRequest, "Invalid family")
		return
	}
	if err := a.assignResumeFamily(id, payload.FamilyID, payload.Primary); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeResumeError(w, http.StatusNotFound, "Unknown resume or family")
			return
		}
		log.Printf("assign resume family api: %v", err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to update resume routing")
		return
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{"id": id, "familyId": payload.FamilyID, "primary": payload.Primary})
}

// POST /api/resumes/{id}/master accepts {master: bool}. A master resume is
// career evidence for cover letters, never an automatic employer upload.
func (a *App) resumeMasterAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, ok := resumeAPIID(r)
	if !ok {
		writeResumeError(w, http.StatusNotFound, "Unknown resume")
		return
	}
	var payload struct {
		Master bool `json:"master"`
	}
	if !decodeResumeJSON(w, r, 4<<10, &payload) {
		return
	}
	if err := a.setMasterResume(id, payload.Master); err != nil {
		if status, message := resumeErrorStatus(err); status != 0 {
			writeResumeError(w, status, message)
			return
		}
		log.Printf("set master resume api: %v", err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to update master resume")
		return
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{"id": id, "master": payload.Master})
}

// POST /api/resumes/{id}/archive accepts {action: archive|restore, confirm}
// and preserves job assignments and submission history either way.
func (a *App) resumeArchiveAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, ok := resumeAPIID(r)
	if !ok {
		writeResumeError(w, http.StatusNotFound, "Unknown resume")
		return
	}
	var payload struct {
		Action  string `json:"action"`
		Confirm bool   `json:"confirm"`
	}
	if !decodeResumeJSON(w, r, 4<<10, &payload) {
		return
	}
	archived := false
	switch payload.Action {
	case "archive":
		archived = true
		if !payload.Confirm {
			writeResumeError(w, http.StatusBadRequest, "Archiving requires confirmation")
			return
		}
	case "restore":
		archived = false
	default:
		writeResumeError(w, http.StatusBadRequest, "Invalid archive action")
		return
	}
	if err := a.setResumeArchived(id, archived); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeResumeError(w, http.StatusNotFound, "Unknown resume")
			return
		}
		log.Printf("archive resume api: %v", err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to update resume archive")
		return
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{"id": id, "archived": archived})
}

func (a *App) reviseResumeFromData(id int64, filename string, data []byte) (Resume, error) {
	return a.reviseResume(id, filename, bytes.NewReader(data))
}

// POST /api/resumes/{id}/revise publishes a new immutable version from
// multipart field "resume"; the previous copy archives and job selections
// stay pinned to the old document until explicitly changed.
func (a *App) resumeReviseAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, ok := resumeAPIID(r)
	if !ok {
		writeResumeError(w, http.StatusNotFound, "Unknown resume")
		return
	}
	name, data, valid := resumeMultipartFile(w, r)
	if !valid {
		return
	}
	revised, err := a.reviseResumeFromData(id, name, data)
	if err != nil {
		if status, message := resumeErrorStatus(err); status != 0 {
			writeResumeError(w, status, message)
			return
		}
		log.Printf("revise resume api: %v", err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to revise resume")
		return
	}
	writeResumeJSON(w, http.StatusCreated, map[string]any{"id": revised.ID, "previousId": id, "version": revised.Version})
}

// POST /api/resumes/{id}/revise-native is desktop-only and sources the
// revision through the trusted native picker.
func (a *App) resumeReviseNativeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, ok := resumeAPIID(r)
	if !ok {
		writeResumeError(w, http.StatusNotFound, "Unknown resume")
		return
	}
	if a.pickResume == nil || !isDesktopRequest(r) {
		writeResumeError(w, http.StatusForbidden, "Desktop window required")
		return
	}
	path, err := a.pickResume()
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "File selection failed")
		return
	}
	if strings.TrimSpace(path) == "" {
		writeResumeError(w, http.StatusBadRequest, "No file selected")
		return
	}
	file, err := os.Open(path)
	if err != nil {
		writeResumeError(w, http.StatusBadRequest, "Unable to open selected file")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, documents.MaxResumeSize+1))
	if err != nil {
		writeResumeError(w, http.StatusBadRequest, "Unable to read selected file")
		return
	}
	revised, err := a.reviseResumeFromData(id, filepath.Base(path), data)
	if err != nil {
		if status, message := resumeErrorStatus(err); status != 0 {
			writeResumeError(w, status, message)
			return
		}
		writeResumeError(w, http.StatusInternalServerError, "Unable to revise resume")
		return
	}
	writeResumeJSON(w, http.StatusCreated, map[string]any{"id": revised.ID, "previousId": id, "version": revised.Version})
}

// POST /api/resumes/{id}/delete accepts {confirm, confirmPhrase: "DELETE"}.
// Referenced or master documents are refused; archiving preserves evidence.
func (a *App) resumeDeleteAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, ok := resumeAPIID(r)
	if !ok {
		writeResumeError(w, http.StatusNotFound, "Unknown resume")
		return
	}
	var payload struct {
		Confirm       bool   `json:"confirm"`
		ConfirmPhrase string `json:"confirmPhrase"`
	}
	if !decodeResumeJSON(w, r, 4<<10, &payload) {
		return
	}
	if !payload.Confirm || payload.ConfirmPhrase != "DELETE" {
		writeResumeError(w, http.StatusBadRequest, "Type DELETE and confirm to delete")
		return
	}
	if err := a.deleteResume(id); err != nil {
		if status, message := resumeErrorStatus(err); status != 0 {
			writeResumeError(w, status, message)
			return
		}
		log.Printf("delete resume api: %v", err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to delete resume")
		return
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

// serveResumes prefers the React bundle when embedded, keeping the existing
// Go document library for compatibility and no-JS fallback.
func (a *App) serveResumes(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && len(a.resources.frontendIndex) > 0 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(a.resources.frontendIndex)
		return
	}
	a.resumeLibrary(w, r)
}
