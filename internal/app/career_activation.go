package app

// Career Evidence activation for YM-003C application preparation.
//
// A reviewed record that came from a career discovery interview stays bound
// to its recorded interview responses through the promoting session
// (career_discovery_sessions.promoted_evidence_id). Answer generation rebinds
// those responses to the exact reviewed revision as engine TraceSources, so
// generated claims carry inspectable evidence/revision/response references
// that discovery.ValidateAnswerTrace accepts or rejects. Records without an
// interview stay usable for ordinary drafting but cannot advertise a trace.
//
// Saved answers keep their existing authority untouched: an additive
// ai_answer_provenance sidecar pins the generation basis (evidence
// id/revision/created plus answer hash) for staleness flags only. Cover
// letter drafts pin reviewed evidence revisions in evidence_refs. Corrections
// surface as visible staleness; saved text is never rewritten.

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	discovery "github.com/jenksed/yourmove-career-discovery"
)

// careerEvidenceTraceSources rebinds recorded interview responses to the
// exact reviewed record revision. A record without a promoting session
// returns no sources and stays usable for ordinary prose drafting. A
// tampered or unreadable session is an error: generation is refused rather
// than advertising a trace that no longer resolves.
func (a *App) careerEvidenceTraceSources(e CareerEvidence) ([]discovery.TraceSource, error) {
	if err := a.careerDiscoverySweepExpired(time.Now()); err != nil {
		return nil, err
	}
	var raw string
	err := a.db.QueryRow(`SELECT state_json FROM career_discovery_sessions
		WHERE promoted_evidence_id=? ORDER BY updated_at DESC,id DESC LIMIT 1`, e.ID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var state discovery.State
	if json.Unmarshal([]byte(raw), &state) != nil || careerDiscoveryCheckState(state) != nil || len(state.Responses) == 0 {
		return nil, discovery.ErrInvalid
	}
	sources := make([]discovery.TraceSource, 0, len(state.Responses))
	for i, response := range state.Responses {
		if response.Source.ID != fmt.Sprintf("response-%d", i+1) || response.Source.Kind != "self_report" ||
			!validNarrativeText(response.Question) || !validNarrativeText(response.Source.Text) || len(response.Source.Text) > 2000 {
			return nil, discovery.ErrInvalid
		}
		sources = append(sources, discovery.TraceSource{EvidenceID: e.ID, EvidenceRevision: e.Revision,
			ResponseID: response.Source.ID, ResponseText: response.Source.Text})
	}
	return sources, nil
}

// AI-answer provenance is a read-only flag source. It never rewrites a saved
// answer, never describes submitted history, and treats missing rows as
// unknown rather than an error.

// recordAnswerProvenance replaces the proposed basis for one question with
// the completed generation's validated evidence selection. It runs after a
// generation completes and never blocks serving the answer.
func (a *App) recordAnswerProvenance(snapshot answerGenerationSnapshot, answerHash, now string) {
	if snapshot.JobID < 1 || snapshot.QuestionKey == "" || answerHash == "" {
		return
	}
	tx, err := a.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM ai_answer_provenance WHERE job_id=? AND destination=? AND question_key=?`, snapshot.JobID, snapshot.Destination, snapshot.QuestionKey); err != nil {
		return
	}
	for _, source := range snapshot.Sources {
		if !strings.HasPrefix(source.Category, "Applicant-reviewed Career Evidence") || source.ID < 1 || source.Revision < 1 || source.CreatedAt == "" {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO ai_answer_provenance(job_id,destination,question_key,evidence_id,evidence_rev,evidence_created,answer_hash,created_at) VALUES(?,?,?,?,?,?,?,?)`,
			snapshot.JobID, snapshot.Destination, snapshot.QuestionKey, source.ID, source.Revision, source.CreatedAt, answerHash, now); err != nil {
			return
		}
	}
	_ = tx.Commit()
}

type answerProvenanceFlag struct {
	QuestionKey, Label string
	State              string // grounded, needs-review, edited, no-provenance
	Detail             string
}

// savedAnswerProvenance compares saved workspace answers against the pinned
// generation basis. Missing rows report no-provenance, never an error.
func (a *App) savedAnswerProvenance(job Job) []answerProvenanceFlag {
	type saved struct {
		key, label, answer string
	}
	var answers []saved
	rows, err := a.db.Query(`SELECT question_key,label,answer FROM application_answers WHERE job_id=? AND destination=?`, job.ID, job.URL)
	if err == nil {
		for rows.Next() {
			var s saved
			if rows.Scan(&s.key, &s.label, &s.answer) == nil {
				answers = append(answers, s)
			}
		}
		rows.Close()
	}
	early, err := a.db.Query(`SELECT COALESCE(live_question_key,''),question_label,answer FROM early_application_answers WHERE job_id=? AND destination=?`, job.ID, job.URL)
	if err == nil {
		for early.Next() {
			var s saved
			if early.Scan(&s.key, &s.label, &s.answer) == nil {
				answers = append(answers, s)
			}
		}
		early.Close()
	}
	var flags []answerProvenanceFlag
	for _, s := range answers {
		flag := answerProvenanceFlag{QuestionKey: s.key, Label: s.label, State: "no-provenance", Detail: "Saved without recorded evidence basis."}
		hash := answerAIHash(s.answer)
		type pin struct {
			evidenceID, evidenceRev int64
			evidenceCreated         string
			answerHash              string
		}
		var pins []pin
		prows, err := a.db.Query(`SELECT evidence_id,evidence_rev,evidence_created,answer_hash FROM ai_answer_provenance WHERE job_id=? AND destination=? AND question_key=?`, job.ID, job.URL, s.key)
		if err == nil {
			for prows.Next() {
				var p pin
				if prows.Scan(&p.evidenceID, &p.evidenceRev, &p.evidenceCreated, &p.answerHash) == nil {
					pins = append(pins, p)
				}
			}
			prows.Close()
		}
		if len(pins) == 0 {
			flags = append(flags, flag)
			continue
		}
		matched := false
		for _, p := range pins {
			if p.answerHash == hash {
				matched = true
			}
		}
		if !matched {
			flag.State = "edited"
			flag.Detail = "Edited after generation. References below describe the generation basis, not the current text."
			flags = append(flags, flag)
			continue
		}
		stale := []string{}
		for _, p := range pins {
			if p.answerHash != hash {
				continue
			}
			current, err := careerEvidenceByID(a.db, p.evidenceID)
			if err != nil || current.Revision != p.evidenceRev || current.CreatedAt != p.evidenceCreated {
				stale = append(stale, current.Title)
			}
		}
		if len(stale) > 0 {
			flag.State = "needs-review"
			flag.Detail = "Supporting evidence changed since generation: " + strings.Join(stale, "; ") + ". Text unchanged."
		} else {
			flag.State = "grounded"
			flag.Detail = "Matches the generated text; supporting evidence unchanged."
		}
		flags = append(flags, flag)
	}
	return flags
}

// careerActivationUsage reports, per Career Evidence record, the recorded
// interview behind it and the saved answers and letter drafts that cite it.
// Staleness is recomputed from current revisions on every read. The view is
// redacted: identities, titles, counts, and hashes only — no answer, letter,
// policy, or excerpt bytes.
func (a *App) careerActivationUsage() ([]map[string]any, error) {
	entries, err := a.careerEvidenceEntries()
	if err != nil {
		return nil, err
	}
	interviews := map[int64]map[string]any{}
	rows, err := a.db.Query(`SELECT id,promoted_evidence_id,state_json FROM career_discovery_sessions WHERE promoted_evidence_id IS NOT NULL`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, raw string
			var evidenceID int64
			if rows.Scan(&id, &evidenceID, &raw) != nil {
				continue
			}
			if _, seen := interviews[evidenceID]; seen {
				continue
			}
			var state discovery.State
			if json.Unmarshal([]byte(raw), &state) != nil {
				continue
			}
			interviews[evidenceID] = map[string]any{"session_id": id, "responses": len(state.Responses)}
		}
	}
	type answerUse struct {
		jobID              int64
		jobTitle           string
		questionKey, label string
		state, detail      string
	}
	// The workspace holds a single SQLite connection: drain each result set
	// fully before running the follow-up lookups below.
	type provenancePin struct {
		jobID, evidenceID, evidenceRev int64
		destination, questionKey       string
		evidenceCreated, answerHash    string
	}
	var pins []provenancePin
	if prows, err := a.db.Query(`SELECT job_id,destination,question_key,evidence_id,evidence_rev,evidence_created,answer_hash FROM ai_answer_provenance`); err == nil {
		for prows.Next() {
			var pin provenancePin
			if prows.Scan(&pin.jobID, &pin.destination, &pin.questionKey, &pin.evidenceID, &pin.evidenceRev, &pin.evidenceCreated, &pin.answerHash) == nil {
				pins = append(pins, pin)
			}
		}
		prows.Close()
	}
	answersByEvidence := map[int64][]answerUse{}
	jobs := map[int64]Job{}
	for _, pin := range pins {
		job, ok := jobs[pin.jobID]
		if !ok {
			job, err = a.jobByID(pin.jobID)
			if err != nil {
				continue
			}
			jobs[pin.jobID] = job
		}
		use := answerUse{jobID: pin.jobID, jobTitle: job.Title, questionKey: pin.questionKey, state: "edited",
			detail: "Edited after generation. The pin below describes the generation basis, not the current text."}
		var text, label string
		err := a.db.QueryRow(`SELECT answer,label FROM application_answers WHERE job_id=? AND destination=? AND question_key=?`,
			pin.jobID, pin.destination, pin.questionKey).Scan(&text, &label)
		if errors.Is(err, sql.ErrNoRows) && pin.questionKey != "" {
			err = a.db.QueryRow(`SELECT answer,question_label FROM early_application_answers WHERE job_id=? AND destination=? AND live_question_key=?`,
				pin.jobID, pin.destination, pin.questionKey).Scan(&text, &label)
		}
		if err != nil {
			use.state, use.detail = "removed", "The saved answer is no longer present. The pin below describes the generation basis."
			use.label = label
			answersByEvidence[pin.evidenceID] = append(answersByEvidence[pin.evidenceID], use)
			continue
		}
		use.label = label
		if answerAIHash(text) != pin.answerHash {
			answersByEvidence[pin.evidenceID] = append(answersByEvidence[pin.evidenceID], use)
			continue
		}
		current, err := careerEvidenceByID(a.db, pin.evidenceID)
		if err != nil || current.Revision != pin.evidenceRev || current.CreatedAt != pin.evidenceCreated {
			use.state, use.detail = "needs-review", "Supporting evidence changed since generation. Text unchanged."
		} else {
			use.state, use.detail = "grounded", "Matches the generated text; supporting evidence unchanged."
		}
		answersByEvidence[pin.evidenceID] = append(answersByEvidence[pin.evidenceID], use)
	}
	type letterUse struct {
		jobID          int64
		jobTitle       string
		draftID        int64
		stale          bool
		pinnedRevision int64
	}
	type letterDraft struct {
		jobID, draftID int64
		raw            string
	}
	var drafts []letterDraft
	if lrows, err := a.db.Query(`SELECT job_id,id,evidence_refs FROM cover_letters WHERE COALESCE(evidence_refs,'') != ''`); err == nil {
		for lrows.Next() {
			var draft letterDraft
			if lrows.Scan(&draft.jobID, &draft.draftID, &draft.raw) == nil {
				drafts = append(drafts, draft)
			}
		}
		lrows.Close()
	}
	lettersByEvidence := map[int64][]letterUse{}
	for _, draft := range drafts {
		refs, err := parseLetterEvidenceRefs(draft.raw)
		if err != nil {
			continue
		}
		job, err := a.jobByID(draft.jobID)
		if err != nil {
			continue
		}
		for _, ref := range refs {
			current, err := careerEvidenceByID(a.db, ref.ID)
			stale := err != nil || current.Revision != ref.Revision || current.CreatedAt != ref.CreatedAt
			lettersByEvidence[ref.ID] = append(lettersByEvidence[ref.ID], letterUse{
				jobID: draft.jobID, jobTitle: job.Title, draftID: draft.draftID,
				stale: stale, pinnedRevision: ref.Revision,
			})
		}
	}
	out := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		var interview any
		if link, ok := interviews[entry.ID]; ok {
			interview = link
		}
		answers := make([]map[string]any, 0)
		for _, use := range answersByEvidence[entry.ID] {
			answers = append(answers, map[string]any{
				"job_id": use.jobID, "job_title": use.jobTitle,
				"question_key": use.questionKey, "label": use.label,
				"state": use.state, "detail": use.detail,
			})
		}
		letters := make([]map[string]any, 0)
		for _, use := range lettersByEvidence[entry.ID] {
			letters = append(letters, map[string]any{
				"job_id": use.jobID, "job_title": use.jobTitle,
				"draft_id": use.draftID, "stale": use.stale,
				"pinned_revision": use.pinnedRevision,
			})
		}
		out = append(out, map[string]any{
			"id": entry.ID, "revision": entry.Revision, "title": entry.Title,
			"status": entry.Status, "updated_at": entry.UpdatedAt,
			"interview": interview, "answers": answers, "letters": letters,
		})
	}
	return out, nil
}

// GET /api/career/activation reports per-record application use with
// recomputed staleness. Nothing here rewrites saved answers or letters.
func (a *App) careerActivationAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	usage, err := a.careerActivationUsage()
	if err != nil {
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to load application use")
		return
	}
	writeCareerJSON(w, http.StatusOK, map[string]any{"evidence": usage})
}
