package app

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// An answer belongs to the exact destination and question definition. Identity
// changes create a separate answer, leaving earlier user work intact.
type preparedQuestion struct {
	ReusableRevision   int64
	ReusableKey        string
	JobID              int64
	Destination        string
	Field              schema.Field
	Key, Value, Source string
	Revision           int64
	Editable           bool
}
type questionPreparation struct {
	InspectionID int64
	Questions    []preparedQuestion
	Previous     []savedApplicationAnswer
	SelectedKey  string // Presentation only; never used as save/transfer authority.
	ReturnNotice string
}
type savedApplicationAnswer struct {
	Key, Label, Value string
	Revision          int64
}

func questionKey(destination, provider string, f schema.Field) string {
	data, _ := json.Marshal(struct {
		Destination, Provider string
		Field                 schema.Field
	}{destination, provider, f})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

var questionReturnKey = regexp.MustCompile(`^[a-f0-9]{64}$`)

// LocalStatus describes preparation, not the contents of the employer's field.
func (q preparedQuestion) LocalStatus() string {
	if !q.Editable {
		return "Requires manual completion in Chromium"
	}
	if q.Value == "" {
		return "Needs local answer"
	}
	return "Saved locally or provided by an approved source"
}

func (p questionPreparation) WorkbenchQuestions() []preparedQuestion {
	var questions []preparedQuestion
	seen := make(map[string]bool)
	for _, q := range p.Questions {
		if q.Field.Kind != "file" && !strings.Contains(strings.ToLower(q.Field.Label), "cover letter") && !seen[q.Key] {
			// Identical ambiguous controls share one manual instruction, never duplicate
			// DOM identities. The authoritative preparation and eligibility stay intact.
			seen[q.Key] = true
			questions = append(questions, q)
		}
	}
	return questions
}

// ProfileQuestions are ready, safely editable facts, not work left to answer.
// Keep Questions intact: presentation must never change transfer scope or hashes.
func (p questionPreparation) ProfileQuestions() []preparedQuestion {
	return p.partitionQuestions(true)
}

func (p questionPreparation) EmployerQuestions() []preparedQuestion {
	return p.partitionQuestions(false)
}

func (p questionPreparation) partitionQuestions(profile bool) []preparedQuestion {
	var questions []preparedQuestion
	for _, q := range p.WorkbenchQuestions() {
		readyProfile := q.Editable && q.Value != "" && q.Source == "Approved candidate information"
		if readyProfile == profile {
			questions = append(questions, q)
		}
	}
	return questions
}

func (p *questionPreparation) selectQuestion(r *http.Request) {
	questions := p.EmployerQuestions()
	query, err := url.ParseQuery(r.URL.RawQuery)
	keys := query["question"]
	if err == nil && len(keys) == 1 && questionReturnKey.MatchString(keys[0]) {
		for _, q := range questions {
			if q.Key == keys[0] {
				p.SelectedKey = q.Key
				return
			}
		}
	}
	if len(keys) > 0 {
		p.ReturnNotice = "That question is no longer available in this observed form. Review the current questions below."
	}
	for _, q := range questions {
		if q.Editable && q.Value == "" {
			p.SelectedKey = q.Key
			return
		}
	}
	if len(questions) > 0 {
		p.SelectedKey = questions[0].Key
	}
}

func (q preparedQuestion) profileContact() bool {
	return q.Source == "Approved candidate information" && profileContactKey(q.Field) != ""
}

func (a *App) prepareQuestions(job Job, p CandidateProfile) (questionPreparation, error) {
	var out questionPreparation
	if !browser.OwnsPostingURL(job.URL, job.URL) {
		return out, nil
	}
	item, err := a.latestInspectionBySource(job.ID, "browser")
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if !browser.OwnsPostingURL(job.URL, item.ApplicationURL) {
		return out, nil
	}
	out.InspectionID = item.ID
	rows, err := a.db.Query(`SELECT question_key,label,answer,revision FROM application_answers WHERE job_id=? AND destination=? AND provider=? ORDER BY question_key`, job.ID, job.URL, item.Platform)
	if err != nil {
		return out, err
	}
	saved := map[string]savedApplicationAnswer{}
	var ordered []savedApplicationAnswer
	for rows.Next() {
		var v savedApplicationAnswer
		if err := rows.Scan(&v.Key, &v.Label, &v.Value, &v.Revision); err != nil {
			rows.Close()
			return out, err
		}
		saved[v.Key] = v
		ordered = append(ordered, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	memory, err := a.reusableAnswers(job.URL)
	if err != nil {
		return out, err
	}
	counts := map[string]int{}
	for _, f := range item.Fields {
		counts[f.ID]++
	}
	used := map[string]bool{}
	for _, f := range item.Fields {
		key := questionKey(job.URL, item.Platform, f.Field)
		answer, exists := saved[key]
		memoryKey := reusableAnswerKey(job.URL, f.Field)
		reusable := memory[memoryKey]
		value, source := resolveApplicationAnswer(f.Field, answer.Value, exists, p, reusable.Value, reusable.Revision > 0)
		editable := editableQuestion(f.Field, counts[f.ID] == 1)
		if !editable {
			value = ""
			source = ""
		}
		out.Questions = append(out.Questions, preparedQuestion{ReusableKey: memoryKey, ReusableRevision: reusable.Revision, JobID: job.ID, Destination: job.URL, Field: f.Field, Key: key, Value: value, Source: source, Revision: answer.Revision, Editable: editable})
		if exists && editable {
			used[key] = true
		}
	}
	for _, v := range ordered {
		if !used[v.Key] {
			out.Previous = append(out.Previous, v)
		}
	}
	return out, nil
}

func (a *App) saveApplicationAnswer(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 24<<10) {
		return
	}
	fail := func(message string, status int) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "private, no-store")
		w.WriteHeader(status)
		// This recovery value never enters questionPreparation or any transfer scope.
		_ = a.resources.jobDetailTemplate.ExecuteTemplate(w, "answer-save-error", struct {
			ID             int64
			Message, Value string
		}{id, message, strings.ReplaceAll(strings.ToValidUTF8(r.PostForm.Get("answer"), "�"), "\x00", "�")})
	}
	job, err := a.jobByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 {
		fail("This application is no longer active", 409)
		return
	}
	p, err := a.candidateProfile()
	if err != nil {
		fail("Unable to read candidate information", 500)
		return
	}
	prepared, err := a.prepareQuestions(job, p)
	if err != nil {
		fail("Unable to read questions", 500)
		return
	}
	key := r.PostForm.Get("question_key")
	var question *preparedQuestion
	for i := range prepared.Questions {
		if prepared.Questions[i].Key == key {
			question = &prepared.Questions[i]
		}
	}
	if question != nil && !question.Editable {
		fail("This question requires manual completion in Chromium. It cannot be saved as a supported local answer.", 409)
		return
	}
	revision, revErr := strconv.ParseInt(r.PostForm.Get("revision"), 10, 64)
	if question == nil || r.PostForm.Get("destination") != job.URL {
		fail("Not saved because the observed question changed. Review the current form before editing.", 409)
		return
	}
	if revErr != nil || question.Revision != revision {
		fail("The saved answer changed in another window. Review the current answer before editing.", 409)
		return
	}
	value := r.PostForm.Get("answer")
	if len(value) > 12000 || !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
		fail("Answer is too long or invalid", 422)
		return
	}
	if question.Field.Kind == "select" || question.Field.Kind == "radio" {
		valid := value == ""
		for _, option := range question.Field.Options {
			if option == value {
				valid = true
			}
		}
		if !valid {
			fail("Choose an observed option", 422)
			return
		}
	}
	if question.Field.Kind == "checkbox" && value != "" && value != "true" && value != "false" {
		fail("Choose Yes or No", 422)
		return
	}
	// Optimistic revision checking prevents a stale tab from overwriting edits.
	reuse := r.PostForm.Get("reusable") == "yes"
	if reuse && (question.ReusableKey == "" || strings.TrimSpace(value) == "") {
		fail("Only a qualified nonempty answer can be reusable", 422)
		return
	}
	memoryRevision, memoryErr := strconv.ParseInt(r.PostForm.Get("reusable_revision"), 10, 64)
	if reuse && (memoryErr != nil || memoryRevision != question.ReusableRevision) {
		fail("The reusable original changed. Review it before replacing it.", 409)
		return
	}
	tx, err := a.db.Begin()
	if err != nil {
		fail("Unable to save the answer", 500)
		return
	}
	defer tx.Rollback()
	result, err := tx.Exec(`INSERT INTO application_answers(job_id,destination,provider,question_key,label,answer,revision) VALUES(?,?,?,?,?,?,1)
 ON CONFLICT(job_id,destination,provider,question_key) DO UPDATE SET answer=excluded.answer,revision=application_answers.revision+1 WHERE application_answers.revision=?`, id, job.URL, browser.PostingPlatform(job.URL), key, question.Field.Label, value, revision)
	if err != nil {
		fail("Unable to save the answer", 500)
		return
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		fail("This answer changed in another window. Reload Prepare.", 409)
		return
	}
	if reuse {
		result, err = tx.Exec(`INSERT INTO reusable_answers(question_key,employer_scope,label,answer,revision) VALUES(?,?,?,?,1)
 ON CONFLICT(question_key) DO UPDATE SET answer=excluded.answer,revision=reusable_answers.revision+1 WHERE reusable_answers.revision=?`, question.ReusableKey, answerEmployerScope(job.URL), question.Field.Label, value, memoryRevision)
		if err != nil {
			fail("Unable to save the reusable answer", 500)
			return
		}
		n, err = result.RowsAffected()
		if err != nil || n != 1 {
			fail("The reusable original changed. Reload Prepare.", 409)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		fail("Unable to save the answer", 500)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/jobs/%d?question=%s#prepare", id, key), http.StatusSeeOther)
}
