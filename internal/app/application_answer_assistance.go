package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jenksed/yourmove-go/internal/coverletter/provider"
)

type answerHelpPresentation struct {
	Question      preparedQuestion
	LibraryEmpty  bool
	EvidenceEmpty bool
}

// applicationAnswerHelpHandler loads source previews and AI controls only when
// the applicant opens assistance for one exact current question.
func (a *App) applicationAnswerHelpHandler(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil || !parseLocalForm(w, r, 12<<10) {
		http.Error(w, "Review the current question before opening assistance", http.StatusBadRequest)
		return
	}
	job, err := a.jobByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 {
		http.Error(w, "Completed applications are read-only", http.StatusConflict)
		return
	}
	values := r.PostForm
	one := func(name string) (string, bool) {
		v := values[name]
		return func() (string, bool) {
			if len(v) != 1 {
				return "", false
			}
			return v[0], true
		}()
	}
	key, ok := one("question_key")
	if !ok || !questionReturnKey.MatchString(key) {
		http.Error(w, "Review the current question", 400)
		return
	}
	revText, ok := one("revision")
	rev, revErr := strconv.ParseInt(revText, 10, 64)
	destination, destOK := one("destination")
	if !ok || revErr != nil || rev < 0 || !destOK || destination != job.URL {
		http.Error(w, "The current question changed", 409)
		return
	}
	var q preparedQuestion
	early := values.Get("early") == "yes"
	if early {
		source, sourceOK := one("discovery_source")
		fingerprint, fpOK := one("discovery_fingerprint")
		sourceID, sourceIDOK := one("source_question_id")
		if !sourceOK || !fpOK || !sourceIDOK || source == "" || fingerprint == "" || sourceID == "" {
			http.Error(w, "The discovered question changed", 409)
			return
		}
		p, err := a.prepareEarlyQuestions(job, r)
		if err != nil {
			http.Error(w, "Unable to read the current discovered question", 500)
			return
		}
		if p.Item == nil || p.Item.Source != source || p.Item.Fingerprint != fingerprint {
			http.Error(w, "The discovered question changed", 409)
			return
		}
		found := false
		for _, candidate := range p.Questions {
			if candidate.Key == key && candidate.SourceQuestionID == sourceID && candidate.Revision == rev && candidate.Destination == destination && candidate.Editable && candidate.TriageCategory == "answer" {
				q = preparedQuestion{JobID: id, Destination: destination, Field: candidate.Field, Key: key, Value: candidate.Value, Revision: rev, Editable: true, IsEarly: true, DiscoverySource: source, DiscoveryFingerprint: fingerprint, SourceQuestionID: sourceID, LibrarySuggestions: candidate.LibrarySuggestions, CareerEvidenceSuggestions: candidate.CareerEvidence}
				found = true
				break
			}
		}
		if !found {
			http.Error(w, "This discovered question is no longer eligible for assistance", 409)
			return
		}
	} else {
		profile, err := a.candidateProfile()
		if err != nil {
			http.Error(w, "Unable to read candidate information", 500)
			return
		}
		p, err := a.prepareQuestions(job, profile)
		if err != nil {
			http.Error(w, "Unable to read current questions", 500)
			return
		}
		for _, candidate := range p.Questions {
			if candidate.Key == key && candidate.Revision == rev && candidate.Destination == destination && candidate.Editable {
				q = candidate
				break
			}
		}
		if q.Key == "" {
			http.Error(w, "This question is no longer eligible for assistance", 409)
			return
		}
	}
	if !narrativeAnswerEligible(q) {
		http.Error(w, "This question does not support writing assistance", 422)
		return
	}
	librarySuggestions, err := a.answerLibrarySuggestionsForQuestions([]preparedQuestion{q})
	if err != nil {
		http.Error(w, "Unable to retrieve previous writing for this question", 500)
		return
	}
	q.LibrarySuggestions = librarySuggestions[q.Key]
	evidenceSuggestions, err := a.careerEvidenceSuggestionsForQuestions([]preparedQuestion{q})
	if err != nil {
		http.Error(w, "Unable to retrieve reviewed experience for this question", 500)
		return
	}
	q.CareerEvidenceSuggestions = evidenceSuggestions[q.Key]
	q.AIChoices = a.answerAIChoices(job, q)
	if q.AIChoices == nil {
		http.Error(w, "AI drafting is unavailable for this question", 422)
		return
	}
	configured := make(map[string]bool)
	for _, id := range []string{"codex", "claude-cli", "ollama", "openai", "anthropic", "openrouter", "compatible"} {
		configured[id] = provider.Configured(id)
	}
	q.AIProviders = applicationAnswerAIProviders(configured)
	q.AIDefaultProvider = "ollama"
	q.AIDefaultModel = provider.DefaultModel("ollama")
	var libraryCount, evidenceCount int
	_ = a.db.QueryRow(`SELECT count(*) FROM answer_library`).Scan(&libraryCount)
	_ = a.db.QueryRow(`SELECT count(*) FROM career_evidence WHERE status='reviewed'`).Scan(&evidenceCount)
	var body bytes.Buffer
	if err := a.resources.jobDetailTemplate.ExecuteTemplate(&body, "question-help-content", answerHelpPresentation{Question: q, LibraryEmpty: libraryCount == 0, EvidenceEmpty: evidenceCount == 0}); err != nil {
		http.Error(w, "Unable to render question assistance", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	_ = json.NewEncoder(w).Encode(struct {
		HTML string `json:"html"`
	}{body.String()})
}
