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

var positionalQuestionID = regexp.MustCompile(`(?i)^(?:[0-9]+|(?:field|question|input)[_-]?[0-9]+)$`)
var questionReturnKey = regexp.MustCompile(`^[a-f0-9]{64}$`)

// Ashby renders stable field identities under a per-form UUID namespace.
const formUUID = `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`

var namespacedSystemField = regexp.MustCompile(`(?i)^` + formUUID + `__(systemfield_[a-z_]+)$`)
var namespacedCustomField = regexp.MustCompile(`(?i)^` + formUUID + `_` + formUUID + `$`)

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

func editableQuestion(f schema.Field, unique bool) bool {
	id := strings.TrimPrefix(f.ID, "browser:")
	if !unique || f.Sensitive || id == "" || positionalQuestionID.MatchString(id) || strings.HasPrefix(id, "unidentified-") || strings.Contains(strings.ToLower(f.Label), "cover letter") {
		return false
	}
	switch f.Kind {
	case "text", "email", "tel", "url", "textarea", "select", "checkbox", "radio":
		return true
	}
	return false
}

// resolveApplicationAnswer is pure and uses explicit identity plus compatible
// labels. Label similarity alone never supplies an answer.
func resolveApplicationAnswer(f schema.Field, explicit string, hasExplicit bool, p CandidateProfile, memory string, hasMemory bool) (string, string) {
	if hasExplicit {
		return explicit, "Saved for this application"
	}
	if f.Sensitive {
		return "", ""
	}

	if p.Approved {
		if key := profileContactKey(f); key != "" {
			value := strings.TrimSpace((browser.ContactValues{FirstName: p.FirstName, LastName: p.LastName, Email: p.Email, Phone: p.Phone}).Value(key))
			if value != "" {
				return value, "Approved candidate information"
			}
		}
	}
	if value := resolveAdditionalProfileFact(f, p); value != "" {
		return value, "Approved candidate information"
	}
	if hasMemory && !f.Sensitive {
		return memory, "Reusable answer approved for this employer"
	}
	return "", ""
}

// These exact labels and identities intentionally exclude eligibility, consent,
// demographic questions and general narrative prompts. No value is inferred.
func resolveAdditionalProfileFact(f schema.Field, p CandidateProfile) string {
	id := strings.TrimPrefix(strings.TrimPrefix(f.ID, "browser:"), "application:")
	if !p.Approved || f.Sensitive || id == "" || positionalQuestionID.MatchString(id) || strings.HasPrefix(id, "unidentified-") {
		return ""
	}
	label := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(f.Label), "*")))
	// Ordinary profile fields may have employer-generated opaque identities.
	label = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(label, " (optional)"), " (required)"))
	linkedin := label == "linkedin" || label == "linkedin url" || label == "linkedin profile" || label == "linkedin profile url" || label == "linkedin profile link" || label == "linkedin link" || label == "your linkedin profile" || label == "your linkedin url"
	if linkedin {
		id = "linkedin"
	}
	location := label == "location" || label == "current location" || label == "your location"
	if location && (f.Kind == "text" || f.Kind == "combobox") {
		parts := []string{}
		for _, part := range []string{p.City, p.Region, p.Country} {
			if part = strings.TrimSpace(part); part != "" {
				parts = append(parts, part)
			}
		}
		return strings.Join(parts, ", ")
	}

	pronouns := label == "pronouns" || label == "preferred pronouns" || label == "what are your pronouns?" || label == "what are your pronouns"
	var value string
	urlField := false
	if pronouns {
		value = p.Pronouns
	} else {
		switch id {
		case "city":
			if label == "city" {
				value = p.City
			}
		case "region", "state", "state_province":
			if label == "state" || label == "region" || label == "state / region" || label == "state/region" || label == "state/province" {
				value = p.Region
			}
		case "country":
			if label == "country" {
				value = p.Country
			}
		case "linkedin", "linkedin_url":
			urlField = true
			if linkedin {
				value = p.LinkedIn
			}
		case "github", "github_url":
			urlField = true
			if label == "github" || label == "github url" || label == "github profile" || label == "github profile url" {
				value = p.GitHub
			}
		case "portfolio", "portfolio_url":
			urlField = true
			if label == "portfolio" || label == "portfolio url" || label == "portfolio website" || label == "portfolio or work sample url" {
				value = p.Portfolio
			}
		}
	}
	if value == "" {
		return ""
	}
	if f.Kind == "text" || f.Kind == "textarea" || (urlField && f.Kind == "url") {
		return value
	}
	if urlField || (f.Kind != "select" && !(pronouns && f.Kind == "radio")) {
		return ""
	}
	// Preserve exact employer text. Even two equivalent options are ambiguous.
	match := ""
	for _, option := range f.Options {
		equal := option == value
		if pronouns {
			equal = pronounOption(option, value)
		}
		if equal {
			if match != "" {
				return ""
			}
			match = option
		}
	}
	return match
}

func pronounOption(option, preference string) bool {
	// Only case and spacing around slashes vary; no fuzzy or substring matching.
	parts := strings.Split(strings.ToLower(strings.TrimSpace(option)), "/")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	normalized := strings.Join(parts, "/")
	switch preference {
	case "He/him":
		return normalized == "he/him" || normalized == "he/him/his"
	case "She/her":
		return normalized == "she/her" || normalized == "she/her/hers"
	case "They/them":
		return normalized == "they/them" || normalized == "they/them/theirs"
	}
	return false
}

// Match the same explicit contact identities and compatible labels as the
// browser contact filler. A name belonging to a manager or reference is excluded.
func profileContactKey(f schema.Field) string {
	if f.Sensitive {
		return ""
	}
	id := strings.TrimPrefix(strings.TrimPrefix(f.ID, "browser:"), "application:")
	if match := namespacedSystemField.FindStringSubmatch(id); match != nil {
		id = match[1]
	}
	// A telephone input supplies machine semantics even with an opaque ID.
	// The exact compatible label below still excludes references and managers.
	if f.Kind == "tel" && namespacedCustomField.MatchString(id) {
		id = "phone"
	}
	id = strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(id))
	label := strings.ToLower(f.Label)
	label = strings.NewReplacer("(required)", "", "(optional)", "", "*", "", "✱", "").Replace(label)
	label = strings.Join(strings.Fields(label), " ")
	key := ""
	switch id {
	case "name", "fullname", "systemfieldname":
		if label == "name" || label == "full name" || label == "full legal name" || label == "your name" {
			key = "name"
		}
	case "firstname", "givenname", "systemfieldfirstname":
		if label == "first name" || label == "given name" {
			key = "first_name"
		}
	case "lastname", "familyname", "surname", "systemfieldlastname":
		if label == "last name" || label == "family name" || label == "surname" {
			key = "last_name"
		}
	case "email", "emailaddress", "systemfieldemail":
		if label == "email" || label == "email address" {
			key = "email"
		}
	case "phone", "phonenumber", "mobilephone", "systemfieldphone":
		if label == "phone" || label == "phone number" || label == "mobile phone" || label == "mobile number" {
			key = "phone"
		}
	}
	switch f.Kind {
	case "text", "input_text", "textarea":
		return key
	case "email":
		if key == "email" {
			return key
		}
	case "tel":
		if key == "phone" {
			return key
		}
	}
	return ""
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
