package app

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/discovery"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

const maxEarlyAnswersPerJob = 250

// An early application answer preserves the exact discovered question but is
// deliberately not a preparedQuestion and is never returned by prepareQuestions.
type earlyApplicationAnswer struct {
	ID                   int64
	JobID                int64
	Destination          string
	DiscoverySource      string
	SourceQuestionID     string
	QuestionLabel        string
	Field                schema.Field
	DiscoveryFingerprint string
	Answer               string
	Revision             int64
	ReconciliationStatus string
	LiveInspection       string
	LiveQuestionKey      string
	ReconciledAt         string
	CreatedAt            string
	UpdatedAt            string
	CurrentQuestionState string
	Reconciliation       earlyReconciliation
}

// A reconciliation candidate is presentation data only. The accept handler
// recomputes it from the current inspections before saving a live answer.
type earlyReconciliation struct {
	Status                    string
	Detail                    string
	LiveQuestionKey           string
	LiveInspectionFingerprint string
	LiveAnswerRevision        int64
	LiveField                 schema.Field
	CanAccept                 bool
	Accepted                  bool
}

type earlyApplicationQuestion struct {
	JobID                     int64
	Destination               string
	DiscoverySource           string
	DiscoveryFingerprint      string
	SourceQuestionID          string
	Key                       string
	Field                     schema.Field
	Value                     string
	ProfileValue              string
	ProfileResolved           bool
	Revision                  int64
	Editable                  bool
	IsEarly                   bool
	ManualReason              string
	TriageCategory            string
	LocalStatus               string
	AIChoices                 *answerAIChoices
	AIProviders               []answerAIProviderOption
	AIDefaultProvider         string
	AIDefaultModel            string
	LibrarySuggestions        []answerLibrarySuggestion
	CareerEvidence            []careerEvidenceSuggestion
	Reconciliation            earlyReconciliation
	EarlyAnswerID             int64
	SavedReconciliationStatus string
	SavedLiveInspection       string
	SavedLiveQuestionKey      string
}

type earlyQuestionPreparation struct {
	Item                    *Inspection
	Questions               []earlyApplicationQuestion
	Previous                []earlyApplicationAnswer
	SupportedQuestionCount  int
	ManualQuestionCount     int
	SelectedKey             string
	ReturnNotice            string
	PublicDiscoveryAllowed  bool
	PreviewDiscoveryAllowed bool
	Notice                  string
}

func (q earlyApplicationQuestion) NeedsAnswer() bool  { return q.TriageCategory == "answer" }
func (q earlyApplicationQuestion) FromProfile() bool  { return q.TriageCategory == "profile" }
func (q earlyApplicationQuestion) EmployerOnly() bool { return q.TriageCategory == "employer" }
func (q earlyApplicationQuestion) NeedsReview() bool  { return q.TriageCategory == "review" }
func (q earlyApplicationQuestion) HelpEligible() bool {
	return q.Editable && q.TriageCategory == "answer" && earlyPotentialNarrative(q.Field)
}

func (p earlyQuestionPreparation) WorkbenchQuestions() []earlyApplicationQuestion {
	var questions []earlyApplicationQuestion
	seen := make(map[string]bool)
	for _, q := range p.Questions {
		if !seen[q.Key] {
			seen[q.Key] = true
			questions = append(questions, q)
		}
	}
	return questions
}

func (p earlyQuestionPreparation) ActionableQuestions() []earlyApplicationQuestion {
	var questions []earlyApplicationQuestion
	for _, q := range p.WorkbenchQuestions() {
		if q.NeedsAnswer() {
			questions = append(questions, q)
		}
	}
	return questions
}

func (p earlyQuestionPreparation) ProfileQuestions() []earlyApplicationQuestion {
	var questions []earlyApplicationQuestion
	for _, q := range p.WorkbenchQuestions() {
		if q.FromProfile() {
			questions = append(questions, q)
		}
	}
	return questions
}

func (p earlyQuestionPreparation) EmployerSiteQuestions() []earlyApplicationQuestion {
	var questions []earlyApplicationQuestion
	for _, q := range p.WorkbenchQuestions() {
		if q.EmployerOnly() {
			questions = append(questions, q)
		}
	}
	return questions
}

func (p earlyQuestionPreparation) ManualReviewQuestions() []earlyApplicationQuestion {
	var questions []earlyApplicationQuestion
	for _, q := range p.WorkbenchQuestions() {
		if q.NeedsReview() {
			questions = append(questions, q)
		}
	}
	return questions
}

func (p earlyQuestionPreparation) PreparedCount() int {
	count := 0
	for _, q := range p.ActionableQuestions() {
		if q.Revision > 0 {
			count++
		}
	}
	return count
}

func (p earlyQuestionPreparation) NeedsAnswerCount() int {
	count := len(p.ActionableQuestions()) - p.PreparedCount()
	if count < 0 {
		return 0
	}
	return count
}

func earlyQuestionKey(destination, source, fingerprint string, field schema.Field) string {
	data, _ := json.Marshal(struct {
		Destination, Source, Fingerprint string
		Field                            schema.Field
	}{destination, source, fingerprint, field})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func earlyFieldSnapshot(field schema.Field) string {
	data, _ := json.Marshal(field)
	return string(data)
}

var earlySensitivePhraseWords = func() [][]string {
	phrases := make([][]string, 0, len(answerLibrarySensitivePhrases))
	for _, phrase := range answerLibrarySensitivePhrases {
		phrases = append(phrases, answerLibraryWords(phrase))
	}
	return phrases
}()

var earlyNarrativeMarkers = []string{
	"describe", "tell us", "share", "give an example", "provide an example", "example of",
	"how have", "how did", "what was", "what would", "what interests", "why do you want",
	"why are you", "walk us through", "discuss", "explain", "elaborate", "challenge",
	"incident", "project", "situation", "time when", "accomplishment", "achievement",
	"proud of", "motivat", "strength", "improvement", "experience with", "experience in",
}

func earlyQuestionWritable(field schema.Field, unique bool) (bool, string) {
	if !unique || strings.TrimSpace(field.ID) == "" {
		return false, "This question has no unique published identity. Write or complete it in the employer application."
	}
	if field.Sensitive || answerLibraryContainsAnySensitivePhrase(field.Label) {
		return false, "This question may involve sensitive information and must be completed manually in the employer application."
	}
	if strings.Contains(strings.ToLower(field.Label), "cover letter") {
		return false, "Use the separate cover-letter editor for this material."
	}
	switch strings.ToLower(field.Kind) {
	case "text", "input_text", "email", "tel", "url", "textarea":
		return true, ""
	case "select", "radio", "multi_value_single_select":
		if len(field.Options) > 0 {
			return true, ""
		}
	}
	return false, "This published control is unsupported for early drafting. Complete it manually in the employer application."
}

func answerLibraryContainsAnySensitivePhrase(label string) bool {
	words := answerLibraryWords(label)
	for _, phrase := range earlySensitivePhraseWords {
		if len(phrase) == 0 || len(phrase) > len(words) {
			continue
		}
		for start := 0; start <= len(words)-len(phrase); start++ {
			match := true
			for offset := range phrase {
				if words[start+offset] != phrase[offset] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

func earlyPotentialNarrative(field schema.Field) bool {
	if field.Kind != "text" && field.Kind != "textarea" {
		return false
	}
	label := strings.ToLower(strings.TrimSpace(field.Label))
	if len(label) < 15 || len(strings.Fields(label)) < 3 {
		return false
	}
	for _, marker := range earlyNarrativeMarkers {
		if strings.Contains(label, marker) {
			return true
		}
	}
	return false
}

func (q earlyApplicationQuestion) Status() string {
	if q.Reconciliation.Accepted {
		return "Accepted as a live application answer; browser filling still requires your authorization."
	}
	if !q.Editable {
		return "Manual completion required"
	}
	if q.Revision > 0 {
		return "Early answer saved locally; not checked against the employer form."
	}
	return "No early answer saved"
}

func (p *earlyQuestionPreparation) selectQuestion(r *http.Request) {
	questions := p.ActionableQuestions()
	requested := r.URL.Query().Get("early_question")
	for _, q := range questions {
		if requested != "" && requested == q.Key {
			p.SelectedKey = q.Key
			return
		}
	}
	if requested != "" {
		p.ReturnNotice = "That discovered question changed. Review the current question before continuing."
	}
	for _, q := range questions {
		if q.Revision == 0 {
			p.SelectedKey = q.Key
			return
		}
	}
	if len(questions) > 0 {
		p.SelectedKey = questions[0].Key
	}
}

func (a *App) prepareEarlyQuestions(job Job, r *http.Request) (earlyQuestionPreparation, error) {
	out := earlyQuestionPreparation{PublicDiscoveryAllowed: discovery.SupportsPublicQuestions(job.URL), PreviewDiscoveryAllowed: browser.PostingPlatform(job.URL) == "ashby"}
	profile, err := a.candidateProfile()
	if err != nil {
		return out, err
	}
	if r != nil {
		switch r.URL.Query().Get("early_discovery") {
		case "failed":
			out.Notice = "Public question discovery failed. Your saved questions and answers were not changed. Try again or use the rendered-form inspection path."
		case "unsupported":
			out.Notice = "Public question discovery is unavailable for this job. Use the rendered-form inspection path or open the employer site manually."
		}
		if r.URL.Query().Get("early_preview") == "failed" {
			out.Notice = "The read-only preview could not inspect this page. Your saved questions and answers remain unchanged. Authentication, CAPTCHA, browser availability, or a later manual step may require the normal rendered-form inspection path."
		}
	}
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 {
		return out, nil
	}
	discoverySource := "public_api"
	if !out.PublicDiscoveryAllowed && out.PreviewDiscoveryAllowed {
		discoverySource = "preview"
	}
	item, err := a.latestValidInspectionForPosting(job.ID, discoverySource, job.URL)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	if err != nil {
		history, historyErr := a.earlyAnswerHistory(job.ID)
		if historyErr != nil {
			return out, historyErr
		}
		for _, answer := range history {
			answer.CurrentQuestionState = "No current discovery is available for this posting. The original question and answer are preserved for review."
			answer.Reconciliation = earlyReconciliation{Status: "Not observed yet", Detail: "Run the available question discovery before reconciling this saved answer."}
			if answer.Destination != job.URL {
				answer.CurrentQuestionState = "This answer was saved for a different employer destination. Its original wording and answer are preserved; it cannot be reconciled with the current destination."
				answer.Reconciliation = earlyReconciliation{Status: "Manual only", Detail: "The saved employer destination changed. The original answer cannot be accepted for this destination."}
			}
			out.Previous = append(out.Previous, answer)
		}
		return out, nil
	}
	if err == nil {
		out.Item = &item
		counts := make(map[string]int, len(item.Fields))
		for _, field := range item.Fields {
			counts[field.ID]++
		}
		history, queryErr := a.earlyAnswerHistory(job.ID)
		if queryErr != nil {
			return out, queryErr
		}
		answersByIdentity := make(map[string]earlyApplicationAnswer)
		for _, answer := range history {
			identity := earlyRecordIdentity(answer.Destination, answer.DiscoverySource, answer.DiscoveryFingerprint, answer.SourceQuestionID)
			if _, exists := answersByIdentity[identity]; !exists {
				answersByIdentity[identity] = answer
			}
		}
		seenCurrent := make(map[string]bool, len(item.Fields))
		for _, field := range item.Fields {
			key := earlyQuestionKey(job.URL, item.Source, item.Fingerprint, field.Field)
			answer := answersByIdentity[earlyRecordIdentity(job.URL, item.Source, item.Fingerprint, field.ID)]
			editable, reason := earlyQuestionWritable(field.Field, counts[field.ID] == 1)
			status := "No early answer saved"
			if answer.Revision > 0 {
				status = "Early answer saved locally; not checked against the employer form."
			}
			stableIdentity := stableQuestionIdentity(field.Field, counts[field.ID] == 1)
			profileValue, profileSource := resolveApplicationAnswer(field.Field, "", false, profile, "", false)
			profileResolved := stableIdentity && profileSource == "Approved candidate information" && strings.TrimSpace(profileValue) != ""
			category := "answer"
			switch {
			case profileResolved && !(answer.Revision > 0 && editable):
				category = "profile"
			case employerOnlyQuestion(field.Field):
				category = "employer"
				if reason == "" {
					reason = "This question is excluded from early preparation by existing privacy or legal policy. Complete it in the employer application."
				}
			case answer.Revision > 0 && editable:
				category = "answer" // Preserve explicit early work over a profile fact.
			case !editable || !stableIdentity && answer.Revision == 0:
				if reason == "" {
					reason = "This question has no unique, stable identity. Review and complete it manually in the employer application."
				}
				category = "review"
			}
			q := earlyApplicationQuestion{JobID: job.ID, Destination: job.URL, DiscoverySource: item.Source, DiscoveryFingerprint: item.Fingerprint, SourceQuestionID: field.ID, Key: key, Field: field.Field, Value: answer.Answer, ProfileValue: profileValue, ProfileResolved: profileResolved, Revision: answer.Revision, Editable: editable, IsEarly: true, ManualReason: reason, TriageCategory: category, LocalStatus: status, EarlyAnswerID: answer.ID, SavedReconciliationStatus: answer.ReconciliationStatus, SavedLiveInspection: answer.LiveInspection, SavedLiveQuestionKey: answer.LiveQuestionKey}
			out.Questions = append(out.Questions, q)
			seenCurrent[earlyRecordIdentity(job.URL, item.Source, item.Fingerprint, field.ID)] = true
		}
		for _, question := range out.Questions {
			if question.Editable {
				out.SupportedQuestionCount++
			} else {
				out.ManualQuestionCount++
			}
		}
		for _, answer := range answersByIdentity {
			identity := earlyRecordIdentity(answer.Destination, answer.DiscoverySource, answer.DiscoveryFingerprint, answer.SourceQuestionID)
			if seenCurrent[identity] {
				continue
			}
			answer.CurrentQuestionState = "The saved question is not in the latest discovery. Its original wording and answer are preserved."
			if answer.Destination != job.URL {
				answer.CurrentQuestionState = "This answer was saved for a different employer destination. Its original wording and answer are preserved; it cannot be reconciled with the current destination."
			} else if answer.DiscoverySource == item.Source {
				if current, exists := fieldByID(item.Fields, answer.SourceQuestionID); exists {
					if earlyFieldSnapshot(current) == earlyFieldSnapshot(answer.Field) {
						answer.CurrentQuestionState = "The question itself is unchanged; the discovery schema was refreshed. Review and copy this earlier draft if needed."
					} else {
						answer.CurrentQuestionState = "The question or its control changed since this answer was saved. Review and adapt it manually."
					}
				}
			}
			out.Previous = append(out.Previous, answer)
		}
		sort.SliceStable(out.Previous, func(i, j int) bool {
			if out.Previous[i].UpdatedAt != out.Previous[j].UpdatedAt {
				return out.Previous[i].UpdatedAt > out.Previous[j].UpdatedAt
			}
			return out.Previous[i].ID > out.Previous[j].ID
		})
		if r != nil {
			out.selectQuestion(r)
		}
		if err := a.reconcileEarlyPreparation(job, &item, &out); err != nil {
			return out, err
		}
	}
	return out, nil
}

func (a *App) earlyAnswerHistory(jobID int64) ([]earlyApplicationAnswer, error) {
	rows, err := a.db.Query(`SELECT id,job_id,destination,discovery_source,source_question_id,question_label,field_snapshot_json,discovery_fingerprint,answer,revision,reconciliation_status,live_inspection_fingerprint,live_question_key,reconciled_at,created_at,updated_at
		FROM early_application_answers WHERE job_id=? ORDER BY updated_at DESC,id DESC LIMIT ?`, jobID, maxEarlyAnswersPerJob)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var history []earlyApplicationAnswer
	seen := make(map[string]bool)
	for rows.Next() {
		var answer earlyApplicationAnswer
		var snapshot string
		if err := rows.Scan(&answer.ID, &answer.JobID, &answer.Destination, &answer.DiscoverySource, &answer.SourceQuestionID, &answer.QuestionLabel, &snapshot, &answer.DiscoveryFingerprint, &answer.Answer, &answer.Revision, &answer.ReconciliationStatus, &answer.LiveInspection, &answer.LiveQuestionKey, &answer.ReconciledAt, &answer.CreatedAt, &answer.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(snapshot), &answer.Field); err != nil {
			return nil, fmt.Errorf("decode early answer question snapshot: %w", err)
		}
		identity := earlyRecordIdentity(answer.Destination, answer.DiscoverySource, answer.DiscoveryFingerprint, answer.SourceQuestionID)
		if !seen[identity] {
			history = append(history, answer)
			seen[identity] = true
		}
	}
	return history, rows.Err()
}

func earlyRecordIdentity(destination, source, fingerprint, questionID string) string {
	return destination + "\x00" + source + "\x00" + fingerprint + "\x00" + questionID
}

func fieldByID(fields []InspectedField, id string) (schema.Field, bool) {
	var found schema.Field
	count := 0
	for _, field := range fields {
		if field.ID == id {
			found = field.Field
			count++
		}
	}
	return found, count == 1
}

func (a *App) earlyQuestionForGeneration(job Job, source, fingerprint, questionID string) (earlyApplicationQuestion, error) {
	if source != "public_api" && source != "preview" {
		return earlyApplicationQuestion{}, errors.New("source")
	}
	item, err := a.latestInspectionBySource(job.ID, source)
	if err != nil || item.Fingerprint != fingerprint || !browser.OwnsPostingURL(job.URL, item.ApplicationURL) {
		return earlyApplicationQuestion{}, errors.New("question")
	}
	field, unique := fieldByID(item.Fields, questionID)
	if !unique {
		return earlyApplicationQuestion{}, errors.New("question")
	}
	key := earlyQuestionKey(job.URL, source, fingerprint, field)
	answer, err := a.earlyAnswer(job.ID, job.URL, source, fingerprint, questionID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return earlyApplicationQuestion{}, err
	}
	editable, reason := earlyQuestionWritable(field, true)
	return earlyApplicationQuestion{JobID: job.ID, Destination: job.URL, DiscoverySource: source, DiscoveryFingerprint: fingerprint, SourceQuestionID: questionID, Key: key, Field: field, Value: answer.Answer, Revision: answer.Revision, Editable: editable, IsEarly: true, ManualReason: reason}, nil
}

func (a *App) earlyAnswer(jobID int64, destination, source, fingerprint, questionID string) (earlyApplicationAnswer, error) {
	var answer earlyApplicationAnswer
	var snapshot string
	err := a.db.QueryRow(`SELECT id,job_id,destination,discovery_source,source_question_id,question_label,field_snapshot_json,discovery_fingerprint,answer,revision,reconciliation_status,live_inspection_fingerprint,live_question_key,reconciled_at,created_at,updated_at
		FROM early_application_answers WHERE job_id=? AND destination=? AND discovery_source=? AND discovery_fingerprint=? AND source_question_id=?`, jobID, destination, source, fingerprint, questionID).Scan(
		&answer.ID, &answer.JobID, &answer.Destination, &answer.DiscoverySource, &answer.SourceQuestionID, &answer.QuestionLabel, &snapshot, &answer.DiscoveryFingerprint, &answer.Answer, &answer.Revision, &answer.ReconciliationStatus, &answer.LiveInspection, &answer.LiveQuestionKey, &answer.ReconciledAt, &answer.CreatedAt, &answer.UpdatedAt)
	if err != nil {
		return answer, err
	}
	if err := json.Unmarshal([]byte(snapshot), &answer.Field); err != nil {
		return answer, err
	}
	return answer, nil
}

func (a *App) saveEarlyApplicationAnswer(w http.ResponseWriter, r *http.Request) {
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
		_ = a.resources.jobDetailTemplate.ExecuteTemplate(w, "early-answer-save-error", struct {
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
		fail("This application is no longer active.", 409)
		return
	}
	source, fingerprint, questionID := r.PostForm.Get("discovery_source"), r.PostForm.Get("discovery_fingerprint"), r.PostForm.Get("source_question_id")
	question, err := a.earlyQuestionForGeneration(job, source, fingerprint, questionID)
	if err != nil || question.Key != r.PostForm.Get("question_key") || r.PostForm.Get("destination") != job.URL {
		fail("Not saved because this discovered question changed. Review the current question and copy your text before continuing.", 409)
		return
	}
	if !question.Editable {
		fail(question.ManualReason, 409)
		return
	}
	revision, revisionErr := strconv.ParseInt(r.PostForm.Get("revision"), 10, 64)
	if revisionErr != nil || revision < 0 || revision != question.Revision {
		fail("This early answer changed in another window. Review the saved version before editing.", 409)
		return
	}
	value := r.PostForm.Get("answer")
	if len(value) > 12000 || !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
		fail("Answer is too long or invalid.", 422)
		return
	}
	switch strings.ToLower(question.Field.Kind) {
	case "select", "radio", "multi_value_single_select":
		valid := value == ""
		for _, option := range question.Field.Options {
			if option == value {
				valid = true
			}
		}
		if !valid {
			fail("Choose one of the published options.", 422)
			return
		}
	}
	now := answerLibraryTimestamp()
	result, err := a.db.Exec(`INSERT INTO early_application_answers(job_id,destination,discovery_source,source_question_id,question_label,field_snapshot_json,discovery_fingerprint,answer,revision,reconciliation_status,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,1,'unreconciled',?,?)
		ON CONFLICT(job_id,destination,discovery_source,discovery_fingerprint,source_question_id)
		DO UPDATE SET question_label=excluded.question_label,field_snapshot_json=excluded.field_snapshot_json,answer=excluded.answer,revision=early_application_answers.revision+1,reconciliation_status='unreconciled',live_inspection_fingerprint='',live_question_key='',reconciled_at='',updated_at=excluded.updated_at
		WHERE early_application_answers.revision=?`, id, job.URL, source, questionID, question.Field.Label, earlyFieldSnapshot(question.Field), fingerprint, value, now, now, revision)
	if err != nil {
		fail("Unable to save this early answer.", 500)
		return
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		fail("This early answer changed in another window. Reload Prepare and review it.", 409)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/jobs/%d?early_question=%s#prepare", id, url.QueryEscape(question.Key)), http.StatusSeeOther)
}
