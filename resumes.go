package main

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxResumeSize = 10 << 20

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

func validateResume(name string, data []byte) (string, error) {
	if len(data) == 0 || len(data) > maxResumeSize {
		return "", fmt.Errorf("resume must be between 1 byte and 10 MiB")
	}
	kind := strings.ToLower(filepath.Ext(name))
	switch kind {
	case ".pdf":
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			return "", fmt.Errorf("file is not a PDF")
		}
	case ".docx":
		z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return "", fmt.Errorf("file is not a DOCX: %w", err)
		}
		document, contentTypes := false, false
		for _, f := range z.File {
			if f.Name == "word/document.xml" {
				document = true
			}
			if f.Name == "[Content_Types].xml" {
				contentTypes = true
			}
		}
		if !document || !contentTypes {
			return "", fmt.Errorf("DOCX missing required package entries")
		}
	default:
		return "", fmt.Errorf("only .pdf and .docx resumes are supported")
	}
	return kind, nil
}

func cleanResumeName(name string) string {
	// An uploaded filename is display-only. Never use it in a filesystem path.
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.TrimSpace(name)
	if len(name) > 180 {
		return ""
	}
	if name == "." || name == "/" || strings.ContainsAny(name, "\x00\r\n") {
		return ""
	}
	for _, char := range name {
		if char < 0x20 || char == 0x7f {
			return ""
		}
	}
	return name
}

func (a *App) importResume(name string, body io.Reader) (Resume, error) {
	if a.resumeDir == "" {
		return Resume{}, errors.New("resume storage is not configured")
	}
	name = cleanResumeName(name)
	if name == "" {
		return Resume{}, fmt.Errorf("invalid filename")
	}
	data, err := io.ReadAll(io.LimitReader(body, maxResumeSize+1))
	if err != nil {
		return Resume{}, fmt.Errorf("read resume: %w", err)
	}
	kind, err := validateResume(name, data)
	if err != nil {
		return Resume{}, err
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	if r, err := a.resumeByHash(hash); err == nil {
		return r, nil
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
			return r, nil
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

func (a *App) resumeByHash(hash string) (Resume, error) {
	var r Resume
	err := a.db.QueryRow(`SELECT id,name,stored_name,sha256,size,kind,imported_at,COALESCE(role_family_id,0),is_primary FROM resumes WHERE sha256=?`, hash).Scan(&r.ID, &r.Name, &r.StoredName, &r.SHA256, &r.Size, &r.Kind, &r.ImportedAt, &r.RoleFamilyID, &r.IsPrimary)
	return r, err
}

func (a *App) resumeByID(id int64) (Resume, error) {
	var r Resume
	err := a.db.QueryRow(`SELECT id,name,stored_name,sha256,size,kind,imported_at,COALESCE(role_family_id,0),is_primary FROM resumes WHERE id=?`, id).Scan(&r.ID, &r.Name, &r.StoredName, &r.SHA256, &r.Size, &r.Kind, &r.ImportedAt, &r.RoleFamilyID, &r.IsPrimary)
	return r, err
}

func (a *App) allResumes() ([]Resume, error) {
	rows, err := a.db.Query(`SELECT id,name,stored_name,sha256,size,kind,imported_at,COALESCE(role_family_id,0),is_primary FROM resumes ORDER BY imported_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Resume{}
	for rows.Next() {
		var r Resume
		if err := rows.Scan(&r.ID, &r.Name, &r.StoredName, &r.SHA256, &r.Size, &r.Kind, &r.ImportedAt, &r.RoleFamilyID, &r.IsPrimary); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (a *App) assignResume(jobID, resumeID int64) error {
	// The EXISTS condition ensures a fabricated resume ID cannot be assigned.
	result, err := a.db.Exec(`UPDATE jobs SET selected_resume_id=NULLIF(?,0) WHERE id=? AND (?=0 OR EXISTS (SELECT 1 FROM resumes WHERE id=?))`, resumeID, jobID, resumeID, resumeID)
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
	var resumeID sql.NullInt64
	var name, hash string
	if err := tx.QueryRow(`SELECT j.selected_resume_id, COALESCE(r.name,''), COALESCE(r.sha256,'')
            FROM jobs j LEFT JOIN resumes r ON r.id=j.selected_resume_id WHERE j.id=?`, jobID).Scan(&resumeID, &name, &hash); err != nil {
		return err
	}
	var selected any
	if resumeID.Valid {
		selected = resumeID.Int64
	}
	_, err = tx.Exec(`INSERT INTO submissions (job_id,resume_id,resume_name,resume_sha256,submitted_at,notes,token)
        VALUES (?,?,?,?,?,?,?)`, jobID, selected, name, hash, time.Now().UTC().Format(time.RFC3339), note, token)
	if err != nil {
		return fmt.Errorf("record submission: %w", err)
	}
	if _, err := tx.Exec(`UPDATE jobs SET stage='applied' WHERE id=?`, jobID); err != nil {
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
