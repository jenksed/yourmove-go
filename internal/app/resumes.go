package app

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/jenksed/yourmove-go/internal/documents"
)

type Resume struct {
	ID           int64
	Name         string
	StoredName   string
	SHA256       string
	Size         int64
	Kind         string
	ImportedAt   string
	RoleFamilyID int64
	IsPrimary    bool
	IsMaster     bool
	ArchivedAt   string
	SeriesID     int64
	Version      int
}

type Submission struct {
	ID           int64
	JobID        int64
	ResumeID     int64
	ResumeName   string
	ResumeSHA256 string
	SubmittedAt  string
	Notes        string
}

func (a *App) importResume(name string, body io.Reader) (Resume, error) {
	if a.resumeDir == "" {
		return Resume{}, errors.New("resume storage is not configured")
	}
	name = documents.SanitizeFilename(name)
	if name == "" {
		return Resume{}, fmt.Errorf("invalid filename")
	}
	data, err := io.ReadAll(io.LimitReader(body, documents.MaxResumeSize+1))
	if err != nil {
		return Resume{}, fmt.Errorf("read resume: %w", err)
	}
	kind, err := documents.ValidateResume(name, data)
	if err != nil {
		return Resume{}, err
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	if r, err := a.resumeByHash(hash); err == nil {
		return a.verifiedImportedResume(r)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Resume{}, err
	}
	idBytes := make([]byte, 16)
	if _, err = rand.Read(idBytes); err != nil {
		return Resume{}, err
	}
	storedName := hex.EncodeToString(idBytes) + kind
	path := filepath.Join(a.resumeDir, storedName)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return Resume{}, err
	}
	_, writeErr := f.Write(data)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		os.Remove(path)
		return Resume{}, fmt.Errorf("write resume: %v %v", writeErr, closeErr)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := a.db.Exec(`INSERT INTO resumes (name,stored_name,sha256,size,kind,imported_at) VALUES (?,?,?,?,?,?)`, name, storedName, hash, len(data), kind, now)
	if err != nil {
		os.Remove(path)
		if r, e := a.resumeByHash(hash); e == nil {
			return a.verifiedImportedResume(r)
		}
		return Resume{}, fmt.Errorf("save resume record: %w", err)
	}
	rowID, err := result.LastInsertId()
	if err != nil {
		os.Remove(path)
		return Resume{}, err
	}
	return Resume{ID: rowID, Name: name, StoredName: storedName, SHA256: hash, Size: int64(len(data)), Kind: kind, ImportedAt: now}, nil
}

// A matching registry entry alone does not establish that the document exists.
func (a *App) verifiedImportedResume(r Resume) (Resume, error) {
	if _, err := a.readResumeBytes(r); err != nil {
		return Resume{}, fmt.Errorf("this resume is already registered, but its stored file is unavailable or damaged; restore the complete workspace from backup before retrying: %w", err)
	}
	return r, nil
}

// readResumeBytes returns stored document bytes only when they match
// the recorded size and SHA-256 digest.
func (a *App) readResumeBytes(resume Resume) ([]byte, error) {
	if a.resumeDir == "" ||
		resume.StoredName == "" ||
		filepath.Base(resume.StoredName) != resume.StoredName ||
		resume.Size <= 0 ||
		resume.Size > documents.MaxResumeSize {
		return nil, errors.New("invalid stored resume")
	}

	file, err := os.Open(filepath.Join(a.resumeDir, resume.StoredName))
	if err != nil {
		return nil, fmt.Errorf("open stored resume: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, documents.MaxResumeSize+1))
	if err != nil {
		return nil, fmt.Errorf("read stored resume: %w", err)
	}

	if int64(len(data)) != resume.Size {
		return nil, errors.New("stored resume size mismatch")
	}

	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != resume.SHA256 {
		return nil, errors.New("stored resume integrity check failed")
	}

	return data, nil
}

// extractResumeText reads only verified document bytes.
func (a *App) extractResumeText(resume Resume) (string, error) {
	data, err := a.readResumeBytes(resume)
	if err != nil {
		return "", err
	}
	return documents.ExtractText(resume.Kind, data)
}

func (a *App) resumeByHash(hash string) (Resume, error) {
	var r Resume
	err := a.db.QueryRow(`SELECT id,name,stored_name,sha256,size,kind,imported_at,COALESCE(role_family_id,0),is_primary,is_master,archived_at,series_id,version FROM resumes WHERE sha256=?`, hash).Scan(&r.ID, &r.Name, &r.StoredName, &r.SHA256, &r.Size, &r.Kind, &r.ImportedAt, &r.RoleFamilyID, &r.IsPrimary, &r.IsMaster, &r.ArchivedAt, &r.SeriesID, &r.Version)
	return r, err
}

func (a *App) resumeByID(id int64) (Resume, error) {
	var r Resume
	err := a.db.QueryRow(`SELECT id,name,stored_name,sha256,size,kind,imported_at,COALESCE(role_family_id,0),is_primary,is_master,archived_at,series_id,version FROM resumes WHERE id=?`, id).Scan(&r.ID, &r.Name, &r.StoredName, &r.SHA256, &r.Size, &r.Kind, &r.ImportedAt, &r.RoleFamilyID, &r.IsPrimary, &r.IsMaster, &r.ArchivedAt, &r.SeriesID, &r.Version)
	return r, err
}

func (a *App) allResumes() ([]Resume, error) {
	rows, err := a.db.Query(`SELECT id,name,stored_name,sha256,size,kind,imported_at,COALESCE(role_family_id,0),is_primary,is_master,archived_at,series_id,version FROM resumes ORDER BY (archived_at != '') ASC, imported_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Resume{}
	for rows.Next() {
		var r Resume
		if err := rows.Scan(&r.ID, &r.Name, &r.StoredName, &r.SHA256, &r.Size, &r.Kind, &r.ImportedAt, &r.RoleFamilyID, &r.IsPrimary, &r.IsMaster, &r.ArchivedAt, &r.SeriesID, &r.Version); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (a *App) assignResume(jobID, resumeID int64) error {
	// The EXISTS condition ensures a fabricated resume ID cannot be assigned.
	result, err := a.db.Exec(`UPDATE jobs SET selected_resume_id=NULLIF(?,0) WHERE id=? AND (?=0 OR EXISTS (SELECT 1 FROM resumes WHERE id=? AND is_master=0 AND archived_at=''))`, resumeID, jobID, resumeID, resumeID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// A single master resume is a source of career evidence, never an automatic
// employer attachment. The master and role-specific primaries are independent.
var ErrResumeInUse = errors.New("resume is referenced by a job, submission, or cover letter")
var ErrMasterResume = errors.New("remove the master designation before deleting this resume")

func (a *App) masterResume() (Resume, error) {
	var r Resume
	err := a.db.QueryRow(`SELECT id,name,stored_name,sha256,size,kind,imported_at,COALESCE(role_family_id,0),is_primary,is_master,archived_at,series_id,version
        FROM resumes WHERE is_master=1`).Scan(&r.ID, &r.Name, &r.StoredName, &r.SHA256,
		&r.Size, &r.Kind, &r.ImportedAt, &r.RoleFamilyID, &r.IsPrimary, &r.IsMaster, &r.ArchivedAt, &r.SeriesID, &r.Version)
	return r, err
}

func (a *App) setMasterResume(id int64, enabled bool) error {
	a.intakeMu.Lock()
	defer a.intakeMu.Unlock()
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM resumes WHERE id=?)`, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return sql.ErrNoRows
	}
	if enabled {
		var archived string
		if err := tx.QueryRow(`SELECT archived_at FROM resumes WHERE id=?`, id).Scan(&archived); err != nil {
			return err
		}
		if archived != "" {
			return ErrResumeArchived
		}
		var used bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM jobs WHERE selected_resume_id=?)
            OR EXISTS(SELECT 1 FROM submissions WHERE resume_id=?)`, id, id).Scan(&used); err != nil {
			return err
		}
		if used {
			return ErrResumeInUse
		}
		if _, err := tx.Exec(`UPDATE resumes SET is_master=0 WHERE is_master=1`); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE resumes SET is_master=1,role_family_id=NULL,is_primary=0 WHERE id=?`, id); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(`UPDATE resumes SET is_master=0 WHERE id=?`, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Refuse deletion when any opportunity or submission refers to the document.
// Commit the metadata removal first: a failure to unlink leaves an orphan
// private file, never a broken database reference to a missing document.
func (a *App) deleteResume(id int64) error {
	a.intakeMu.Lock()
	defer a.intakeMu.Unlock()
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var filename string
	var master bool
	if err := tx.QueryRow(`SELECT stored_name,is_master FROM resumes WHERE id=?`, id).Scan(&filename, &master); err != nil {
		return err
	}
	if master {
		return ErrMasterResume
	}
	var used bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM jobs WHERE selected_resume_id=?)
        OR EXISTS(SELECT 1 FROM submissions WHERE resume_id=?)
        OR EXISTS(SELECT 1 FROM cover_letters WHERE master_resume_id=? OR role_resume_id=?)`, id, id, id, id).Scan(&used); err != nil {
		return err
	}
	if used {
		return ErrResumeInUse
	}
	if a.resumeDir == "" || filename == "" || filename == "." || filename == ".." || filepath.Base(filename) != filename {
		return errors.New("invalid stored resume path")
	}
	if _, err := tx.Exec(`DELETE FROM resumes WHERE id=?`, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(a.resumeDir, filename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("resume record deleted but private file could not be removed: %w", err)
	}
	return nil
}

// Archived resumes remain available for application and submission evidence.
// Only the unreferenced artifacts can be physically deleted.
var ErrResumeArchived = errors.New("resume is archived")
var ErrDuplicateResumeRevision = errors.New("this document is already in the library; upload different content")

func (a *App) setResumeArchived(id int64, archived bool) error {
	a.intakeMu.Lock()
	defer a.intakeMu.Unlock()
	at := ""
	if archived {
		at = time.Now().UTC().Format(time.RFC3339Nano)
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var result sql.Result
	if archived {
		result, err = tx.Exec(`UPDATE resumes SET archived_at=?,is_primary=0,is_master=0 WHERE id=?`, at, id)
	} else {
		// Restoring a historical version never makes it master or primary again.
		result, err = tx.Exec(`UPDATE resumes SET archived_at='' WHERE id=?`, id)
	}
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

// reviseResume keeps immutable bytes for every version. Existing job selections
// and confirmed submission snapshots continue to refer to the old resume ID.
// A new revision inherits role routing or master designation, not job selections.
// reviseResume publishes a new immutable version only after its routing and
// archival metadata commit together. A failed transaction leaves the previous
// version active and removes the newly written private file.
func (a *App) reviseResume(previousID int64, filename string, body io.Reader) (Resume, error) {
	a.intakeMu.Lock()
	defer a.intakeMu.Unlock()

	previous, err := a.resumeByID(previousID)
	if err != nil {
		return Resume{}, err
	}
	if previous.ArchivedAt != "" {
		return Resume{}, ErrResumeArchived
	}
	if a.resumeDir == "" {
		return Resume{}, errors.New("resume storage is not configured")
	}
	filename = documents.SanitizeFilename(filename)
	if filename == "" {
		return Resume{}, errors.New("invalid revision filename")
	}
	data, err := io.ReadAll(io.LimitReader(body, documents.MaxResumeSize+1))
	if err != nil {
		return Resume{}, err
	}
	kind, err := documents.ValidateResume(filename, data)
	if err != nil {
		return Resume{}, err
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	if _, err := a.resumeByHash(hash); err == nil {
		return Resume{}, ErrDuplicateResumeRevision
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Resume{}, err
	}

	series := previous.SeriesID
	if series == 0 {
		series = previous.ID
	}

	// Prepare private bytes first, but do not expose a database resume row
	// until the version/archival transition can be committed atomically.
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return Resume{}, err
	}
	storedName := hex.EncodeToString(idBytes) + kind
	path := filepath.Join(a.resumeDir, storedName)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return Resume{}, err
	}
	_, writeErr := file.Write(data)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		return Resume{}, fmt.Errorf("write revised resume: %v %v", writeErr, closeErr)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(path)
		}
	}()

	tx, err := a.db.Begin()
	if err != nil {
		return Resume{}, err
	}
	defer tx.Rollback()

	var highest int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(version),1) FROM resumes WHERE id=? OR series_id=?`, series, series).Scan(&highest); err != nil {
		return Resume{}, err
	}
	result, err := tx.Exec(`UPDATE resumes SET archived_at=?,is_master=0,is_primary=0 WHERE id=? AND archived_at=''`,
		time.Now().UTC().Format(time.RFC3339Nano), previous.ID)
	if err != nil {
		return Resume{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Resume{}, err
	}
	if affected != 1 {
		return Resume{}, ErrResumeArchived
	}
	now := time.Now().UTC().Format(time.RFC3339)
	result, err = tx.Exec(`INSERT INTO resumes
        (name,stored_name,sha256,size,kind,imported_at,series_id,version,role_family_id,is_primary,is_master)
        VALUES(?,?,?,?,?,?,?,?,NULLIF(?,0),?,?)`,
		filename, storedName, hash, len(data), kind, now,
		series, highest+1, previous.RoleFamilyID, previous.IsPrimary, previous.IsMaster)
	if err != nil {
		return Resume{}, err
	}
	newID, err := result.LastInsertId()
	if err != nil {
		return Resume{}, err
	}
	if err := tx.Commit(); err != nil {
		return Resume{}, err
	}
	committed = true
	return a.resumeByID(newID)
}

func newSubmissionToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func validSubmissionToken(token string) bool {
	if len(token) != 32 {
		return false
	}
	_, err := hex.DecodeString(token)
	return err == nil
}

var ErrSubmissionReviewStale = errors.New("submission review is stale")

func (a *App) recordSubmissionReview(jobID int64, token string, resumeID int64, resumeHash string) error {
	if !validSubmissionToken(token) || jobID <= 0 || resumeID < 0 {
		return fmt.Errorf("invalid submission review")
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var selectedResumeID sql.NullInt64
	var selectedResumeHash string
	if err := tx.QueryRow(`SELECT j.selected_resume_id, COALESCE(r.sha256,'')
		FROM jobs j LEFT JOIN resumes r ON r.id=j.selected_resume_id WHERE j.id=?`, jobID).
		Scan(&selectedResumeID, &selectedResumeHash); err != nil {
		return err
	}
	if selectedResumeID.Int64 != resumeID || selectedResumeHash != resumeHash {
		return ErrSubmissionReviewStale
	}
	var reviewedResume any
	if resumeID > 0 {
		reviewedResume = resumeID
	}
	if _, err := tx.Exec(`INSERT INTO submission_reviews(token,job_id,resume_id,resume_sha256) VALUES(?,?,?,?)`, token, jobID, reviewedResume, resumeHash); err != nil {
		return fmt.Errorf("save submission review: %w", err)
	}
	return tx.Commit()
}

func (a *App) recordSubmission(jobID int64, note, token string) error {
	// Keep explicit submission creation and destructive deletion serialized.
	a.intakeMu.Lock()
	defer a.intakeMu.Unlock()
	if !validSubmissionToken(token) {
		return fmt.Errorf("invalid submission token")
	}
	if len(note) > 1000 {
		return fmt.Errorf("submission note too long")
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Retrying the exact same form submission must not create a second record.
	var tokenExists bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM submissions WHERE token=?)`, token).Scan(&tokenExists); err != nil {
		return err
	}
	if tokenExists {
		return nil
	}
	staleReview := func() error {
		if _, err := tx.Exec(`DELETE FROM submission_reviews WHERE token=?`, token); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return ErrSubmissionReviewStale
	}
	var reviewedJobID int64
	var reviewedResumeID sql.NullInt64
	var reviewedResumeHash string
	if err := tx.QueryRow(`SELECT job_id,resume_id,resume_sha256 FROM submission_reviews WHERE token=?`, token).
		Scan(&reviewedJobID, &reviewedResumeID, &reviewedResumeHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			var jobExists bool
			if checkErr := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM jobs WHERE id=?)`, jobID).Scan(&jobExists); checkErr != nil {
				return checkErr
			}
			if !jobExists {
				return sql.ErrNoRows
			}
			return staleReview()
		}
		return err
	}
	if reviewedJobID != jobID {
		return staleReview()
	}
	var currentResumeID sql.NullInt64
	if err := tx.QueryRow(`SELECT selected_resume_id FROM jobs WHERE id=?`, jobID).Scan(&currentResumeID); err != nil {
		return err
	}
	if currentResumeID.Int64 != reviewedResumeID.Int64 || currentResumeID.Valid != reviewedResumeID.Valid {
		return staleReview()
	}
	var name, hash string
	if reviewedResumeID.Valid {
		var resume Resume
		resume.ID = reviewedResumeID.Int64
		if err := tx.QueryRow(`SELECT name,stored_name,sha256,size,kind FROM resumes WHERE id=?`, resume.ID).
			Scan(&resume.Name, &resume.StoredName, &resume.SHA256, &resume.Size, &resume.Kind); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return staleReview()
			}
			return err
		}
		if resume.SHA256 != reviewedResumeHash {
			return staleReview()
		}
		if _, err := a.readResumeBytes(resume); err != nil {
			return staleReview()
		}
		name, hash = resume.Name, resume.SHA256
	} else if reviewedResumeHash != "" {
		return staleReview()
	}
	var selected any
	if reviewedResumeID.Valid {
		selected = reviewedResumeID.Int64
	}
	_, err = tx.Exec(`INSERT INTO submissions (job_id,resume_id,resume_name,resume_sha256,submitted_at,notes,token)
        VALUES (?,?,?,?,?,?,?)`, jobID, selected, name, hash, time.Now().UTC().Format(time.RFC3339), note, token)
	if err != nil {
		return fmt.Errorf("record submission: %w", err)
	}
	if _, err := tx.Exec(`UPDATE jobs SET stage='applied' WHERE id=?`, jobID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM submission_reviews WHERE token=?`, token); err != nil {
		return err
	}
	return tx.Commit()
}

func (a *App) jobSubmissions(jobID int64) ([]Submission, error) {
	rows, err := a.db.Query(`SELECT s.id,s.job_id,COALESCE(s.resume_id,0),COALESCE(NULLIF(s.resume_name,''),'No resume assigned'),s.resume_sha256,s.submitted_at,s.notes FROM submissions s WHERE s.job_id=? ORDER BY s.id DESC`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Submission{}
	for rows.Next() {
		var s Submission
		if err := rows.Scan(&s.ID, &s.JobID, &s.ResumeID, &s.ResumeName, &s.ResumeSHA256, &s.SubmittedAt, &s.Notes); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}
