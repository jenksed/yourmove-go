package app

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type Submission struct {
	ID           int64
	JobID        int64
	ResumeID     int64
	ResumeName   string
	ResumeSHA256 string
	SubmittedAt  string
	Notes        string
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
