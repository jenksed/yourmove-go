package app

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// Manually created application questions preserve applicant preparation where
// managed capture is unavailable or incomplete. They are applicant-authored
// records, never observed employer controls: their keys are application-owned
// ("manual-" prefix), they never match a browser or preview field identity,
// and no preparation, save, or transfer path for observed controls accepts
// them. Final employer-side completion stays manual.

var manualQuestionKeyPattern = regexp.MustCompile(`^manual-[0-9a-f]{32}$`)

const (
	maxManualQuestionText = 500
	maxManualAnswerText   = 12000
)

var (
	errManualQuestionConflict = errors.New("manual question revision conflict")
	errManualQuestionInvalid  = errors.New("invalid manual question")
)

type manualApplicationQuestion struct {
	ID          int64
	JobID       int64
	Destination string
	Key         string
	Question    string
	Answer      string
	Revision    int64
	ReuseJSON   string
	CreatedAt   string
	UpdatedAt   string
}

func newManualQuestionKey() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "manual-" + hex.EncodeToString(raw[:]), nil
}

func validManualQuestionKey(key string) bool {
	return manualQuestionKeyPattern.MatchString(key)
}

// manualQuestionField projects a manually created question onto the shared
// narrative-text eligibility rules (Answer Library suggestions, automatic
// preparation offers, promotion). The synthetic kind is presentation-only for
// those checks; the stored record keeps no employer control identity.
func manualQuestionField(key, question string) schema.Field {
	return schema.Field{ID: key, Label: question, Kind: "text"}
}

func validManualQuestionText(question string) bool {
	trimmed := strings.TrimSpace(question)
	return trimmed != "" && len(trimmed) <= maxManualQuestionText && utf8.ValidString(trimmed) && !strings.ContainsRune(trimmed, 0)
}

func validManualAnswerText(answer string) bool {
	return len(answer) <= maxManualAnswerText && utf8.ValidString(answer) && !strings.ContainsRune(answer, 0)
}

func manualQuestionTimestamp() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z07:00")
}

func (a *App) manualQuestions(job Job) ([]manualApplicationQuestion, error) {
	rows, err := a.db.Query(`SELECT id,job_id,destination,question_key,question,answer,revision,reuse_source_json,created_at,updated_at FROM manual_application_questions WHERE job_id=? AND destination=? ORDER BY updated_at DESC,id DESC`, job.ID, job.URL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []manualApplicationQuestion
	for rows.Next() {
		var q manualApplicationQuestion
		if err := rows.Scan(&q.ID, &q.JobID, &q.Destination, &q.Key, &q.Question, &q.Answer, &q.Revision, &q.ReuseJSON, &q.CreatedAt, &q.UpdatedAt); err != nil {
			return nil, err
		}
		if !validManualQuestionKey(q.Key) || q.Destination != job.URL {
			continue
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (a *App) manualQuestionByKey(job Job, key string) (manualApplicationQuestion, error) {
	var q manualApplicationQuestion
	if !validManualQuestionKey(key) {
		return q, sql.ErrNoRows
	}
	err := a.db.QueryRow(`SELECT id,job_id,destination,question_key,question,answer,revision,reuse_source_json,created_at,updated_at FROM manual_application_questions WHERE job_id=? AND destination=? AND question_key=?`, job.ID, job.URL, key).Scan(&q.ID, &q.JobID, &q.Destination, &q.Key, &q.Question, &q.Answer, &q.Revision, &q.ReuseJSON, &q.CreatedAt, &q.UpdatedAt)
	return q, err
}

func (a *App) createManualQuestion(job Job, question string) (manualApplicationQuestion, error) {
	var out manualApplicationQuestion
	if !validManualQuestionText(question) {
		return out, errManualQuestionInvalid
	}
	key, err := newManualQuestionKey()
	if err != nil {
		return out, err
	}
	now := manualQuestionTimestamp()
	trimmed := strings.TrimSpace(question)
	result, err := a.db.Exec(`INSERT INTO manual_application_questions(job_id,destination,question_key,question,answer,revision,created_at,updated_at) VALUES(?,?,?,?,?,1,?,?)`, job.ID, job.URL, key, trimmed, "", now, now)
	if err != nil {
		return out, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return out, err
	}
	out = manualApplicationQuestion{ID: id, JobID: job.ID, Destination: job.URL, Key: key, Question: trimmed, Revision: 1, CreatedAt: now, UpdatedAt: now}
	return out, nil
}

// saveManualQuestion updates the applicant-authored question and/or prepared
// answer with an optimistic revision check. A stale revision never overwrites
// newer applicant work.
func (a *App) saveManualQuestion(job Job, key, question, answer string, revision int64, reuseJSON string) (manualApplicationQuestion, error) {
	var out manualApplicationQuestion
	current, err := a.manualQuestionByKey(job, key)
	if err != nil {
		return out, err
	}
	if current.Revision != revision {
		return out, errManualQuestionConflict
	}
	nextQuestion := current.Question
	if strings.TrimSpace(question) != "" {
		if !validManualQuestionText(question) {
			return out, errManualQuestionInvalid
		}
		nextQuestion = strings.TrimSpace(question)
	}
	if !validManualAnswerText(answer) {
		return out, errManualQuestionInvalid
	}
	if reuseJSON != "" && !validAnswerReuseOriginPayload(reuseJSON) {
		return out, errManualQuestionInvalid
	}
	now := manualQuestionTimestamp()
	result, err := a.db.Exec(`UPDATE manual_application_questions SET question=?,answer=?,revision=revision+1,reuse_source_json=?,updated_at=? WHERE job_id=? AND destination=? AND question_key=? AND revision=?`, nextQuestion, answer, reuseJSON, now, job.ID, job.URL, key, revision)
	if err != nil {
		return out, err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return out, errManualQuestionConflict
	}
	return a.manualQuestionByKey(job, key)
}

func (a *App) deleteManualQuestion(job Job, key string, revision int64) error {
	if !validManualQuestionKey(key) {
		return sql.ErrNoRows
	}
	var existing int64
	if err := a.db.QueryRow(`SELECT revision FROM manual_application_questions WHERE job_id=? AND destination=? AND question_key=?`, job.ID, job.URL, key).Scan(&existing); err != nil {
		return err
	}
	if existing != revision {
		return errManualQuestionConflict
	}
	result, err := a.db.Exec(`DELETE FROM manual_application_questions WHERE job_id=? AND destination=? AND question_key=? AND revision=?`, job.ID, job.URL, key, revision)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return errManualQuestionConflict
	}
	return nil
}
