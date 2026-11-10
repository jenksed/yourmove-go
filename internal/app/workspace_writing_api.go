// YM-004B Application Writing JSON transport.
//
// Coherent internal/app implementation (no feature-package extraction): every
// contract below reuses the existing Go decisions and durable writes owned by
// the legacy Prepare flow. This file adds transport only:
//
//   - GET  /api/jobs/{id}/questions   read-only inventory (live + early), token-free.
//     The payload also carries the unified applicant-facing projection, which
//     merges both contexts into one coherent list without removing data.
//   - POST /api/jobs/{id}/answers     JSON twin of saveApplicationAnswer /
//     saveEarlyApplicationAnswer, sharing persistApplicationAnswer and the
//     early-answer upsert semantics (optimistic revision checks intact)
//   - POST /api/jobs/{id}/assistance  JSON twin of applicationAnswerHelpHandler
//     (suggestions, AI choices, providers for one exact current question)
//   - GET  /api/jobs/{id}/letters            read-only drafts + source status
//   - POST /api/jobs/{id}/letters            JSON twin of generateCoverLetterHandler
//     (new draft per generation; edits are never overwritten)
//   - GET  /api/jobs/{id}/letters/{draft}    one saved draft with staleness
//   - POST /api/jobs/{id}/letters/{draft}    JSON twin of saveCoverLetterBody
//
// AI ticket generation itself is NOT duplicated: React calls the existing
// POST /jobs/{id}/answers/ai/{review,generate,adopt-check,cancel} endpoints
// with form-encoded bodies and consumes their JSON responses directly.
//
// Ordinary reads never mint a submission-review token and never write durable
// state. No résumé bytes, credentials, paths, or browser internals enter
// these responses.

package app

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/schema"
	coverprovider "github.com/jenksed/yourmove-go/internal/coverletter/provider"
)

type writingLibrarySuggestionJSON struct {
	MatchKind string `json:"matchKind"`
	EntryID   int64  `json:"entryId"`
	Revision  int64  `json:"revision"`
	Identity  string `json:"identity"`
	Question  string `json:"question"`
	Employer  string `json:"employer"`
	JobTitle  string `json:"jobTitle"`
	Host      string `json:"host"`
	Excerpt   string `json:"excerpt"`
}

type writingEvidenceSuggestionJSON struct {
	EvidenceID     int64  `json:"evidenceId"`
	Revision       int64  `json:"revision"`
	Identity       string `json:"identity"`
	Title          string `json:"title"`
	OutcomeState   string `json:"outcomeState"`
	Outcome        string `json:"outcome"`
	ActionsExcerpt string `json:"actionsExcerpt"`
}

type writingQuestionJSON struct {
	Key              string   `json:"key"`
	Label            string   `json:"label"`
	Kind             string   `json:"kind"`
	Required         bool     `json:"required"`
	Options          []string `json:"options"`
	Value            string   `json:"value"`
	Source           string   `json:"source"`
	LocalStatus      string   `json:"localStatus"`
	Revision         int64    `json:"revision"`
	Editable         bool     `json:"editable"`
	LibraryEligible  bool     `json:"libraryEligible"`
	HelpEligible     bool     `json:"helpEligible"`
	IsEarly          bool     `json:"isEarly"`
	DiscoverySource  string   `json:"discoverySource"`
	DiscoveryPrint   string   `json:"discoveryFingerprint"`
	SourceQuestionID string   `json:"sourceQuestionId"`
	// IsManual marks an applicant-authored question that never describes an
	// observed employer control. Manual rows are always editable writing
	// tasks and never enter Fill scope.
	IsManual            bool                            `json:"isManual"`
	TriageCategory      string                          `json:"triageCategory"`
	ManualReason        string                          `json:"manualReason"`
	ReusableKey         string                          `json:"reusableKey"`
	ReusableRevision    int64                           `json:"reusableRevision"`
	ReusableEligible    bool                            `json:"reusableEligible"`
	ReusableAnswer      string                          `json:"reusableAnswer"`
	ReusableAvailable   bool                            `json:"reusableAvailable"`
	OriginEntryID       int64                           `json:"originEntryId"`
	OriginRevision      int64                           `json:"originRevision"`
	OriginQuestion      string                          `json:"originQuestion"`
	OriginEmployer      string                          `json:"originEmployer"`
	OriginRecorded      bool                            `json:"originRecorded"`
	LibraryAutoEntryID  int64                           `json:"libraryAutoEntryId"`
	LibraryAutoRevision int64                           `json:"libraryAutoRevision"`
	LibraryAutoQuestion string                          `json:"libraryAutoQuestion"`
	LibraryAutoPrepared bool                            `json:"libraryAutoPrepared"`
	LibrarySuggestions  []writingLibrarySuggestionJSON  `json:"librarySuggestions"`
	EvidenceSuggestion  []writingEvidenceSuggestionJSON `json:"evidenceSuggestions"`
	// Reconciliation is early-only presentation data; acceptance stays server-owned.
	ReconciliationStatus string `json:"reconciliationStatus"`
	ReconciliationDetail string `json:"reconciliationDetail"`
	ReconciliationAccept bool   `json:"reconciliationAcceptable"`
	Reconciled           bool   `json:"reconciled"`
}

type writingPreviousJSON struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Value    string `json:"value"`
	Revision int64  `json:"revision"`
	Early    bool   `json:"early"`
}

func toWritingLibrarySuggestions(in []answerLibrarySuggestion) []writingLibrarySuggestionJSON {
	out := make([]writingLibrarySuggestionJSON, 0, len(in))
	for _, s := range in {
		out = append(out, writingLibrarySuggestionJSON{
			EntryID: s.EntryID, Revision: s.Revision, MatchKind: s.MatchKind,
			Identity: fmt.Sprintf("%d:%d", s.EntryID, s.Revision),
			Question: s.OriginalQuestion, Employer: s.SourceEmployer,
			JobTitle: s.SourceJobTitle, Host: s.SourceHost, Excerpt: s.Excerpt,
		})
	}
	return out
}

func toWritingEvidenceSuggestions(in []careerEvidenceSuggestion) []writingEvidenceSuggestionJSON {
	out := make([]writingEvidenceSuggestionJSON, 0, len(in))
	for _, s := range in {
		e := s.Evidence
		out = append(out, writingEvidenceSuggestionJSON{
			EvidenceID: e.ID, Revision: e.Revision,
			Identity:     fmt.Sprintf("%d:%d:%s", e.ID, e.Revision, e.CreatedAt),
			Title:        e.Title,
			OutcomeState: e.OutcomeState, Outcome: e.Outcome,
			ActionsExcerpt: s.ActionsExcerpt,
		})
	}
	return out
}

func toWritingQuestion(q preparedQuestion) writingQuestionJSON {
	category, manualReason := liveWritingTriage(q)
	return writingQuestionJSON{
		Key: q.Key, Label: q.Field.Label, Kind: q.Field.Kind, Required: q.Field.Required,
		Options:             safePublishedOptions(q.Field.Kind, q.Field.Options),
		Value:               q.Value,
		Source:              q.Source,
		LocalStatus:         q.LocalStatus(),
		Revision:            q.Revision,
		Editable:            q.Editable,
		LibraryEligible:     q.Editable && answerLibraryEligible(q.Field, true),
		HelpEligible:        q.HelpEligible(),
		IsEarly:             q.IsEarly,
		DiscoverySource:     q.DiscoverySource,
		DiscoveryPrint:      q.DiscoveryFingerprint,
		SourceQuestionID:    q.SourceQuestionID,
		TriageCategory:      category,
		ManualReason:        manualReason,
		ReusableKey:         q.ReusableKey,
		ReusableRevision:    q.ReusableRevision,
		ReusableEligible:    q.ReusableKey != "",
		ReusableAnswer:      q.ReusableAnswer,
		ReusableAvailable:   q.ReusableAvailable,
		LibraryAutoEntryID:  q.LibraryAutoEntryID,
		LibraryAutoRevision: q.LibraryAutoRevision,
		LibraryAutoQuestion: q.LibraryAutoQuestion,
		LibraryAutoPrepared: q.LibraryAutoEntryID > 0,
		LibrarySuggestions:  toWritingLibrarySuggestions(q.LibrarySuggestions),
		EvidenceSuggestion:  toWritingEvidenceSuggestions(q.CareerEvidenceSuggestions),
	}
}

// liveWritingTriage mirrors questionPreparation.partitionQuestions so the React
// inventory names the same category the legacy workbench uses. It only labels
// presentation; save and transfer authority stay with Editable and the
// server-owned validators.
func liveWritingTriage(q preparedQuestion) (string, string) {
	category := writingCategory(q.Field, true, q.Editable)
	switch category {
	case "employer":
		return category, "Complete this control manually in the employer application."
	case "review":
		return category, "This control is unsupported for saved-answer transfer. Complete it manually in the employer application."
	default:
		return category, ""
	}

}

// displayDocumentControl keeps résumé and cover-letter controls in the
// Documents workflow: they never appear as applicant writing tasks. This
// mirrors the live workbench filter, which already excludes them, for the
// early inventory that preserves them. File kinds are documents whatever
// their label says, and the provider's own résumé identities (including the
// Greenhouse paste-text control) are documents whatever their control kind
// is. Anything else keeps its own role: narrative wording that merely
// mentions a résumé stays an applicant-written question.
func displayDocumentControl(f schema.Field) bool {
	if f.Kind == "file" || f.Kind == "input_file" {
		return true
	}
	if strings.Contains(strings.ToLower(f.Label), "cover letter") {
		return true
	}
	switch resumeDocumentIdentity(profileQuestionIdentity(f.ID)) {
	case "resume", "resumetext", "cv":
		return true
	}
	return false
}

func resumeDocumentIdentity(id string) string {
	return strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(id))
}

// displayUnidentifiedControl reports generic buckets without a stable field
// identity. They are condensed into one inspection notice instead of being
// presented as questions.
func displayUnidentifiedControl(id string) bool {
	base := ashbyStableFieldSuffix(id)
	base = strings.TrimPrefix(base, "application:")
	return base == "unidentified-controls" || strings.HasPrefix(base, "unidentified-")
}

// displayQuestionIdentity keys the live and early observations of one employer
// control for the applicant-facing projection only. It never grants save or
// transfer authority: every write still revalidates the exact underlying
// record. Empty means the row must never merge with another context, so two
// genuinely distinct controls with identical wording stay separate.
// Applicant-authored manual questions never merge: they are appended after
// unification with their own application-owned keys.
func displayQuestionIdentity(id string) string {
	base := ashbyStableFieldSuffix(id)
	base = strings.TrimPrefix(base, "application:")
	if base == "" || base == "unidentified-controls" || strings.HasPrefix(base, "unidentified-") || positionalQuestionID.MatchString(base) {
		return ""
	}
	if strings.HasPrefix(base, "manual-") {
		return ""
	}
	return base
}

// displayQuestionsCompatible reports whether the live and early observations
// describe the same control closely enough to present once. Any drift in
// wording, control type, required status, or options keeps both rows so the
// applicant sees the truth instead of a silent collapse.
func displayQuestionsCompatible(live, early schema.Field, earlySource string) bool {
	if reconciliationLabel(live.Label) != reconciliationLabel(early.Label) {
		return false
	}
	kindsMatch := live.Kind == early.Kind
	if earlySource == "public_api" {
		kindsMatch = greenhouseControlTypesMatch(early.Kind, live.Kind)
	}
	if !kindsMatch {
		return false
	}
	return live.Required == early.Required && sameOptions(live.Options, early.Options)
}

func liveRowAnswered(q preparedQuestion) bool {
	return q.Revision > 0 && strings.TrimSpace(q.Value) != ""
}

func earlyRowDrafted(q earlyApplicationQuestion) bool {
	return q.Revision > 0 && strings.TrimSpace(q.Value) != ""
}

// unifyQuestionInventories presents one coherent applicant-facing list from
// the live and early inventories without removing previously discovered data.
// Correspondence uses qualified field identities plus compatible field
// details; wording alone never merges rows, and merging never confers save or
// transfer authority. A saved early draft keeps its editor identity until it
// is reconciled or saved as a live answer, so it is never shown as
// simultaneously unanswered and prepared. Unmatched rows from either context
// are preserved, and a stale live inspection never displaces current preview
// information: without drafts, the preferred context wins. Rows that are both
// outside the applicant-written tasks may additionally merge on identity and
// wording alone, so the current valid observation describes one employer-side
// requirement once; applicant-written questions never merge that way. It also
// returns the unidentified-control count from the preferred context only, so
// the notice reflects the current observation instead of adding both
// inventories together.
func unifyQuestionInventories(liveRows []preparedQuestion, earlyRows []earlyApplicationQuestion, preferLive bool) ([]writingQuestionJSON, int) {
	liveUnidentified, earlyUnidentified := 0, 0
	liveKept := make([]preparedQuestion, 0, len(liveRows))
	for _, q := range liveRows {
		if displayDocumentControl(q.Field) || isProfileField(q.Field) {
			continue
		}
		if displayUnidentifiedControl(q.Field.ID) {
			liveUnidentified++
			continue
		}
		liveKept = append(liveKept, q)
	}
	earlyKept := make([]earlyApplicationQuestion, 0, len(earlyRows))
	for _, q := range earlyRows {
		if displayDocumentControl(q.Field) || isProfileField(q.Field) {
			continue
		}
		if displayUnidentifiedControl(q.SourceQuestionID) {
			earlyUnidentified++
			continue
		}
		earlyKept = append(earlyKept, q)
	}
	liveUsed := make([]bool, len(liveKept))
	earlyUsed := make([]bool, len(earlyKept))
	liveCategory := func(i int) string {
		category, _ := liveWritingTriage(liveKept[i])
		return category
	}
	// findCounterpart locates the first unused row with the same qualified
	// identity. Strict mode additionally requires compatible field details;
	// relaxed mode is display-only for pairs that are both outside the
	// applicant-written tasks and share the same wording.
	findEarly := func(liveIdx int, strict bool) int {
		want := displayQuestionIdentity(liveKept[liveIdx].Field.ID)
		if want == "" {
			return -1
		}
		for j, early := range earlyKept {
			if earlyUsed[j] || displayQuestionIdentity(early.SourceQuestionID) != want {
				continue
			}
			if strict {
				if !displayQuestionsCompatible(liveKept[liveIdx].Field, early.Field, early.DiscoverySource) {
					continue
				}
			} else if liveCategory(liveIdx) == "answer" || early.TriageCategory == "answer" {
				continue
			} else if reconciliationLabel(liveKept[liveIdx].Field.Label) != reconciliationLabel(early.Field.Label) {
				continue
			}
			return j
		}
		return -1
	}
	findLive := func(earlyIdx int, strict bool) int {
		want := displayQuestionIdentity(earlyKept[earlyIdx].SourceQuestionID)
		if want == "" {
			return -1
		}
		for i, live := range liveKept {
			if liveUsed[i] || displayQuestionIdentity(live.Field.ID) != want {
				continue
			}
			if strict {
				if !displayQuestionsCompatible(live.Field, earlyKept[earlyIdx].Field, earlyKept[earlyIdx].DiscoverySource) {
					continue
				}
			} else if liveCategory(i) == "answer" || earlyKept[earlyIdx].TriageCategory == "answer" {
				continue
			} else if reconciliationLabel(live.Field.Label) != reconciliationLabel(earlyKept[earlyIdx].Field.Label) {
				continue
			}
			return i
		}
		return -1
	}
	matchEarly := func(live preparedQuestion) int {
		for i := range liveKept {
			if liveKept[i].Key == live.Key {
				if j := findEarly(i, true); j >= 0 {
					return j
				}
				return findEarly(i, false)
			}
		}
		return -1
	}
	matchLive := func(early earlyApplicationQuestion) int {
		for j := range earlyKept {
			if earlyKept[j].Key == early.Key {
				if i := findLive(j, true); i >= 0 {
					return i
				}
				return findLive(j, false)
			}
		}
		return -1
	}
	pick := func(live *preparedQuestion, early *earlyApplicationQuestion) writingQuestionJSON {
		switch {
		case early != nil && earlyRowDrafted(*early) && (live == nil || !liveRowAnswered(*live)):
			return toWritingEarlyQuestion(*early)
		case live != nil && liveRowAnswered(*live):
			return toWritingQuestion(*live)
		case live != nil && early != nil:
			if preferLive {
				return toWritingQuestion(*live)
			}
			return toWritingEarlyQuestion(*early)
		case live != nil:
			return toWritingQuestion(*live)
		default:
			return toWritingEarlyQuestion(*early)
		}
	}
	var out []writingQuestionJSON
	if preferLive {
		for i := range liveKept {
			if j := matchEarly(liveKept[i]); j >= 0 {
				earlyUsed[j] = true
				live, early := liveKept[i], earlyKept[j]
				out = append(out, pick(&live, &early))
			} else {
				live := liveKept[i]
				out = append(out, pick(&live, nil))
			}
		}
		for j := range earlyKept {
			if !earlyUsed[j] {
				early := earlyKept[j]
				out = append(out, pick(nil, &early))
			}
		}
	} else {
		for j := range earlyKept {
			if i := matchLive(earlyKept[j]); i >= 0 {
				liveUsed[i] = true
				live, early := liveKept[i], earlyKept[j]
				out = append(out, pick(&live, &early))
			} else {
				early := earlyKept[j]
				out = append(out, pick(nil, &early))
			}
		}
		for i := range liveKept {
			if !liveUsed[i] {
				live := liveKept[i]
				out = append(out, pick(&live, nil))
			}
		}
	}
	if preferLive {
		return out, liveUnidentified
	}
	return out, earlyUnidentified
}

func toWritingEarlyQuestion(q earlyApplicationQuestion) writingQuestionJSON {
	out := writingQuestionJSON{
		Key: q.Key, Label: q.Field.Label, Kind: q.Field.Kind, Required: q.Field.Required,
		Options:              safePublishedOptions(q.Field.Kind, q.Field.Options),
		Value:                q.Value,
		Revision:             q.Revision,
		Editable:             q.Editable,
		LibraryEligible:      q.Editable && q.TriageCategory == "answer" && earlyPotentialNarrative(q.Field),
		HelpEligible:         q.HelpEligible(),
		IsEarly:              true,
		DiscoverySource:      q.DiscoverySource,
		DiscoveryPrint:       q.DiscoveryFingerprint,
		SourceQuestionID:     q.SourceQuestionID,
		TriageCategory:       q.TriageCategory,
		ManualReason:         q.ManualReason,
		ReusableEligible:     false,
		ReusableAvailable:    false,
		LibraryAutoEntryID:   q.LibraryAutoEntryID,
		LibraryAutoRevision:  q.LibraryAutoRevision,
		LibraryAutoQuestion:  q.LibraryAutoQuestion,
		LibraryAutoPrepared:  q.LibraryAutoEntryID > 0,
		LibrarySuggestions:   toWritingLibrarySuggestions(q.LibrarySuggestions),
		EvidenceSuggestion:   toWritingEvidenceSuggestions(q.CareerEvidence),
		ReconciliationStatus: q.Reconciliation.Status,
		ReconciliationDetail: q.Reconciliation.Detail,
		ReconciliationAccept: q.Reconciliation.CanAccept,
		Reconciled:           q.Reconciliation.Accepted,
	}
	if q.ProfileResolved {
		out.Source = "Approved candidate information"
		out.LocalStatus = "Approved candidate information; transfer requires your authorization"
	} else if q.LibraryAutoEntryID > 0 {
		out.Source = libraryAutoSource
		out.LocalStatus = "Automatically prepared from Answer Library; review and save locally. It has not been checked against the employer form."
	} else if q.Value != "" {
		out.Source = "Saved early answer"
		out.LocalStatus = "Saved locally. It has not been verified against the employer form."
	} else if q.Editable {
		out.LocalStatus = "Needs your answer"
	} else {
		out.LocalStatus = q.ManualReason
		if out.LocalStatus == "" {
			out.LocalStatus = "Requires manual completion in Chromium"
		}
	}
	return out
}

// buildWritingQuestions assembles the read-only inventory. It never mints a
// submission-review token and never writes durable state.
func (a *App) buildWritingQuestions(jobID int64) (map[string]any, error) {
	job, err := a.jobByID(jobID)
	if err != nil {
		return nil, err
	}
	profile, err := a.candidateProfile()
	if err != nil {
		return nil, err
	}
	live, err := a.prepareQuestions(job, profile)
	if err != nil {
		return nil, err
	}
	early, err := a.prepareEarlyQuestions(job, nil)
	if err != nil {
		return nil, err
	}
	snapshot := chooseApplicationQuestionSnapshot(job, a.browsers.state(job.ID), live, early)
	activeContext := "early"
	if snapshot.Anchor == "#application-questions" {
		activeContext = "live"
	}

	liveActionable := live.ActionableQuestions()
	if len(liveActionable) > 0 {
		library, err := a.answerLibrarySuggestionsForQuestions(liveActionable)
		if err != nil {
			return nil, err
		}
		evidence, err := a.careerEvidenceSuggestionsForQuestions(liveActionable)
		if err != nil {
			return nil, err
		}
		for i := range liveActionable {
			liveActionable[i].LibrarySuggestions = library[liveActionable[i].Key]
			liveActionable[i].CareerEvidenceSuggestions = evidence[liveActionable[i].Key]
		}
	}
	// The inventory is the full discovered set: classification determines
	// available actions, never hides observed questions. Suggestion lookups
	// above stay keyed on the actionable subset.
	liveByKey := make(map[string]preparedQuestion, len(liveActionable))
	for _, q := range liveActionable {
		liveByKey[q.Key] = q
	}
	liveWorkbench := live.WorkbenchQuestions()
	for i := range liveWorkbench {
		if enriched, ok := liveByKey[liveWorkbench[i].Key]; ok {
			liveWorkbench[i].LibrarySuggestions = enriched.LibrarySuggestions
			liveWorkbench[i].CareerEvidenceSuggestions = enriched.CareerEvidenceSuggestions
		}
	}
	liveJSON := make([]writingQuestionJSON, 0, len(liveWorkbench))
	for _, q := range liveWorkbench {
		liveJSON = append(liveJSON, toWritingQuestion(q))
	}

	earlyActionable := early.ActionableQuestions()
	// Suggestion lookups key on preparedQuestion.Key, so convert early
	// questions without granting them live authority.
	earlyConverted := make([]preparedQuestion, 0, len(earlyActionable))
	for _, q := range earlyActionable {
		earlyConverted = append(earlyConverted, preparedQuestion{
			JobID: job.ID, Destination: q.Destination, Field: q.Field, Key: q.Key,
			Value: q.Value, Revision: q.Revision, Editable: q.Editable,
		})
	}
	if len(earlyConverted) > 0 {
		library, err := a.answerLibrarySuggestionsForQuestions(earlyConverted)
		if err != nil {
			return nil, err
		}
		evidence, err := a.careerEvidenceSuggestionsForQuestions(earlyConverted)
		if err != nil {
			return nil, err
		}
		for i := range earlyActionable {
			earlyActionable[i].LibrarySuggestions = library[earlyActionable[i].Key]
			earlyActionable[i].CareerEvidence = evidence[earlyActionable[i].Key]
		}
	}
	// As with live questions, the early inventory is the full discovered set.
	earlyByKey := make(map[string]earlyApplicationQuestion, len(earlyActionable))
	for _, q := range earlyActionable {
		earlyByKey[q.Key] = q
	}
	earlyWorkbench := early.WorkbenchQuestions()
	for i := range earlyWorkbench {
		if enriched, ok := earlyByKey[earlyWorkbench[i].Key]; ok {
			earlyWorkbench[i].LibrarySuggestions = enriched.LibrarySuggestions
			earlyWorkbench[i].CareerEvidence = enriched.CareerEvidence
		}
	}
	earlyJSON := make([]writingQuestionJSON, 0, len(earlyWorkbench))
	for _, q := range earlyWorkbench {
		earlyJSON = append(earlyJSON, toWritingEarlyQuestion(q))
	}

	// The applicant-facing projection merges the two inventories into one
	// coherent list. The live and early arrays above keep the full
	// discovered data; only this projection decides what the applicant sees.
	unified, unidentified := unifyQuestionInventories(liveWorkbench, earlyWorkbench, snapshot.Anchor == "#application-questions")
	// Attach durable starting-source provenance without rereading source
	// bodies. Stored origin is display-only; it never confers authority.
	func() {
		liveOrigins := make(map[string]answerReuseOrigin)
		if provider := browser.PostingPlatform(job.URL); provider != "" {
			if rows, err := a.db.Query(`SELECT question_key,reuse_source_json FROM application_answers WHERE job_id=? AND destination=? AND provider=?`, job.ID, job.URL, provider); err == nil {
				func() {
					defer rows.Close()
					for rows.Next() {
						var key, raw string
						if err := rows.Scan(&key, &raw); err != nil || strings.TrimSpace(raw) == "" {
							continue
						}
						var origin answerReuseOrigin
						if err := json.Unmarshal([]byte(raw), &origin); err == nil && origin.LibraryEntryID > 0 {
							liveOrigins[key] = origin
						}
					}
				}()
			}
		}
		type earlyIdentity struct {
			source, fingerprint, question string
		}
		earlyOrigins := make(map[earlyIdentity]answerReuseOrigin)
		if rows, err := a.db.Query(`SELECT discovery_source,discovery_fingerprint,source_question_id,reuse_source_json FROM early_application_answers WHERE job_id=? AND destination=?`, job.ID, job.URL); err == nil {
			func() {
				defer rows.Close()
				for rows.Next() {
					var source, fingerprint, questionID, raw string
					if err := rows.Scan(&source, &fingerprint, &questionID, &raw); err != nil || strings.TrimSpace(raw) == "" {
						continue
					}
					var origin answerReuseOrigin
					if err := json.Unmarshal([]byte(raw), &origin); err == nil && origin.LibraryEntryID > 0 {
						earlyOrigins[earlyIdentity{source, fingerprint, questionID}] = origin
					}
				}
			}()
		}
		apply := func(list []writingQuestionJSON) {
			for i := range list {
				if list[i].IsEarly {
					origin, ok := earlyOrigins[earlyIdentity{list[i].DiscoverySource, list[i].DiscoveryPrint, list[i].SourceQuestionID}]
					if !ok {
						continue
					}
					list[i].OriginEntryID = origin.LibraryEntryID
					list[i].OriginRevision = origin.LibraryRevision
					list[i].OriginQuestion = origin.SourceQuestion
					list[i].OriginEmployer = origin.SourceEmployer
					list[i].OriginRecorded = true
					continue
				}
				origin, ok := liveOrigins[list[i].Key]
				if !ok {
					continue
				}
				list[i].OriginEntryID = origin.LibraryEntryID
				list[i].OriginRevision = origin.LibraryRevision
				list[i].OriginQuestion = origin.SourceQuestion
				list[i].OriginEmployer = origin.SourceEmployer
				list[i].OriginRecorded = true
			}
		}
		apply(liveJSON)
		apply(earlyJSON)
		apply(unified)
	}()

	// Applicant-authored questions are appended after unification with their
	// own application-owned keys. They never merge with observed controls, so
	// later capture cannot silently merge, overwrite, or authorize them.
	manual, err := a.manualQuestionProjection(job)
	if err != nil {
		return nil, err
	}
	unified = append(unified, manual...)

	previous := make([]writingPreviousJSON, 0, len(live.Previous)+len(early.Previous))
	for _, p := range live.Previous {
		previous = append(previous, writingPreviousJSON{Key: p.Key, Label: p.Label, Value: p.Value, Revision: p.Revision})
	}
	for _, p := range early.Previous {
		previous = append(previous, writingPreviousJSON{Key: p.SourceQuestionID, Label: p.QuestionLabel, Value: p.Answer, Revision: p.Revision, Early: true})
	}

	captureState := a.browsers.state(job.ID)
	editingAllowed := job.Stage != StageApplied && job.Stage != StageClosed && job.SubmissionCount == 0
	return map[string]any{
		"browserActive": captureState.Active,
		"captureState":  map[string]any{"phase": captureState.Phase, "message": captureState.Message},
		"available":     snapshot.Available, "source": snapshot.Source,
		"partial": snapshot.Partial, "anchor": snapshot.Anchor,
		"activeContext": activeContext,
		"inspection": map[string]any{
			"liveId": live.InspectionID, "liveSource": live.InspectionSource,
			"liveCompleteness": live.InspectionCompleteness, "liveUrl": live.InspectionURL,
			"earlySource": func() string {
				if early.Item != nil {
					return early.Item.Source
				}
				return ""
			}(),
			"earlyCompleteness": func() string {
				if early.Item != nil {
					return early.Item.Completeness
				}
				return ""
			}(),
			"earlyFingerprint": func() string {
				if early.Item != nil {
					return early.Item.Fingerprint
				}
				return ""
			}(),
		},
		"live":                 liveJSON,
		"early":                earlyJSON,
		"questions":            unified,
		"unidentifiedCount":    unidentified,
		"previous":             previous,
		"profileCount":         len(live.ProfileQuestions()) + len(early.ProfileQuestions()),
		"employerOnlyCount":    len(live.EmployerSiteQuestions()),
		"manualReviewCount":    len(live.ManualReviewQuestions()),
		"earlyNotice":          early.Notice,
		"returnNotice":         live.ReturnNotice,
		"editingAllowed":       editingAllowed,
		"answerEditingAllowed": editingAllowed,
		"ownsPosting":          browser.OwnsPostingURL(job.URL, job.URL),
	}, nil
}

// GET /api/jobs/{id}/questions is strictly read-only.
func (a *App) writingQuestionsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	payload, err := a.buildWritingQuestions(jobID)
	if errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	if err != nil {
		log.Printf("load writing questions %d: %v", jobID, err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to load application questions")
		return
	}
	writeResumeJSON(w, http.StatusOK, payload)
}

type writingSaveInput struct {
	SaveToLibrary    bool   `json:"save_to_library"`
	QuestionKey      string `json:"question_key"`
	Revision         int64  `json:"revision"`
	Destination      string `json:"destination"`
	Answer           string `json:"answer"`
	Reusable         bool   `json:"reusable"`
	ReusableRevision int64  `json:"reusable_revision"`
	Early            bool   `json:"early"`
	DiscoverySource  string `json:"discovery_source"`
	DiscoveryPrint   string `json:"discovery_fingerprint"`
	SourceQuestionID string `json:"source_question_id"`
	OriginEntryID    int64  `json:"origin_entry_id"`
	OriginRevision   int64  `json:"origin_revision"`
	OriginClear      bool   `json:"origin_clear"`
}

// answerReuseOrigin is durable starting-source provenance for a manually
// adapted answer. It records a starting point, never endorsement or proof the
// edited text still matches the source.
type answerReuseOrigin struct {
	LibraryEntryID    int64  `json:"library_entry_id"`
	LibraryRevision   int64  `json:"library_revision"`
	SourceQuestion    string `json:"source_question"`
	SourceEmployer    string `json:"source_employer"`
	SourceJobTitle    string `json:"source_job_title"`
	SourceDestination string `json:"source_destination"`
	SourceHash        string `json:"source_hash"`
	DestinationRev    int64  `json:"destination_revision"`
}

func buildAnswerReuseOrigin(entry AnswerLibraryEntry, destinationRevision int64) (string, error) {
	origin := answerReuseOrigin{
		LibraryEntryID: entry.ID, LibraryRevision: entry.Revision,
		SourceQuestion: entry.OriginalQuestion, SourceEmployer: entry.SourceEmployer,
		SourceJobTitle: entry.SourceJobTitle, SourceDestination: entry.SourceDestination,
		SourceHash:     fmt.Sprintf("%x", sha256.Sum256([]byte(entry.Answer))),
		DestinationRev: destinationRevision,
	}
	data, err := json.Marshal(origin)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func validAnswerReuseOriginPayload(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return true
	}
	var origin answerReuseOrigin
	if err := json.Unmarshal([]byte(raw), &origin); err != nil {
		return false
	}
	return origin.LibraryEntryID > 0 && origin.LibraryRevision > 0 && origin.SourceHash != ""
}

// POST /api/jobs/{id}/answers saves one applicant answer. Live saves share
// persistApplicationAnswer with the legacy form handler, so revision semantics
// stay identical; early saves share the legacy early-answer upsert.
func (a *App) writingSaveAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	var input writingSaveInput
	if !decodeResumeJSON(w, r, 24<<10, &input) {
		return
	}
	job, err := a.jobByID(jobID)
	if errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	if err != nil {
		log.Printf("load writing save job %d: %v", jobID, err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to save the answer")
		return
	}
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 {
		writeResumeError(w, http.StatusConflict, "This application is no longer active")
		return
	}
	if input.Destination != job.URL {
		writeResumeError(w, http.StatusConflict, "Not saved because the observed question changed. Review the current form before editing.")
		return
	}
	if input.Early {
		a.writingSaveEarly(w, job, input)
		return
	}
	profile, err := a.candidateProfile()
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to read candidate information")
		return
	}
	prepared, err := a.prepareQuestions(job, profile)
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to read questions")
		return
	}
	var question *preparedQuestion
	for i := range prepared.Questions {
		if prepared.Questions[i].Key == input.QuestionKey {
			question = &prepared.Questions[i]
		}
	}
	if question == nil {
		writeResumeError(w, http.StatusConflict, "Not saved because the observed question changed. Review the current form before editing.")
		return
	}
	if !question.Editable {
		writeResumeError(w, http.StatusConflict, "This question requires manual completion in Chromium. It cannot be saved as a supported local answer.")
		return
	}
	if question.Revision != input.Revision {
		writeResumeError(w, http.StatusConflict, "The saved answer changed in another window. Review the current answer before editing.")
		return
	}
	if message := validateApplicationAnswerValue(*question, input.Answer); message != "" {
		writeResumeError(w, http.StatusUnprocessableEntity, message)
		return
	}
	if input.Reusable && (question.ReusableKey == "" || strings.TrimSpace(input.Answer) == "") {
		writeResumeError(w, http.StatusUnprocessableEntity, "Only a qualified nonempty answer can be reusable")
		return
	}
	if input.Reusable && input.ReusableRevision != question.ReusableRevision {
		writeResumeError(w, http.StatusConflict, "The reusable original changed. Review it before replacing it.")
		return
	}
	var originJSON string
	var originRequested bool
	if input.OriginEntryID > 0 || input.OriginClear {
		originRequested = true
	}
	if input.OriginEntryID > 0 {
		if input.OriginRevision < 1 {
			writeResumeError(w, http.StatusBadRequest, "Review the selected source before saving with provenance.")
			return
		}
		entry, err := a.answerLibraryBySourceID(a.db, input.OriginEntryID)
		if errors.Is(err, sql.ErrNoRows) {
			writeResumeError(w, http.StatusConflict, "The selected source changed or was deleted. Review the updated example or save as independent writing.")
			return
		}
		if err != nil {
			writeResumeError(w, http.StatusInternalServerError, "Unable to verify the selected source")
			return
		}
		if entry.Revision != input.OriginRevision {
			writeResumeError(w, http.StatusConflict, "The selected source changed. Review the updated example or save as independent writing.")
			return
		}
		if !answerLibraryEligible(question.Field, true) {
			writeResumeError(w, http.StatusConflict, "The current question changed. Review it before saving with this source.")
			return
		}
		originJSON, err = buildAnswerReuseOrigin(entry, question.Revision+1)
		if err != nil {
			writeResumeError(w, http.StatusInternalServerError, "Unable to save the answer")
			return
		}
	} else if input.OriginClear {
		originJSON = ""
	}
	tx, err := a.db.Begin()
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to save the answer")
		return
	}
	defer tx.Rollback()
	if err := persistApplicationAnswer(tx, job, *question, input.Revision, input.Answer, input.Reusable, input.ReusableRevision); err != nil {
		switch {
		case errors.Is(err, errApplicationAnswerConflict):
			writeResumeError(w, http.StatusConflict, "This answer changed in another window. Review the current answer before editing.")
		case errors.Is(err, errReusableAnswerConflict):
			writeResumeError(w, http.StatusConflict, "The reusable original changed. Review it before replacing it.")
		case errors.Is(err, errReusableAnswerInvalid):
			writeResumeError(w, http.StatusUnprocessableEntity, "Only a qualified nonempty answer can be reusable")
		default:
			writeResumeError(w, http.StatusInternalServerError, "Unable to save the answer")
		}
		return
	}
	if originRequested {
		if !validAnswerReuseOriginPayload(originJSON) {
			writeResumeError(w, http.StatusInternalServerError, "Unable to save the answer")
			return
		}
		if _, err := tx.Exec(`UPDATE application_answers SET reuse_source_json=? WHERE job_id=? AND destination=? AND provider=? AND question_key=?`,
			originJSON, job.ID, job.URL, browser.PostingPlatform(job.URL), input.QuestionKey); err != nil {
			writeResumeError(w, http.StatusInternalServerError, "Unable to save the answer")
			return
		}
	}
	if input.SaveToLibrary {
		if err := saveWritingLibrary(tx, job, input, question.Field.Label, question.Field.Kind, answerLibraryEligible(question.Field, true)); err != nil {
			writeResumeError(w, 409, "Not saved. This question is ineligible or matches conflicting library entries. Review the library, or save for this application only.")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to save the answer")
		return
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{"saved": true, "key": input.QuestionKey, "revision": question.Revision + 1})
}

func (a *App) writingSaveEarly(w http.ResponseWriter, job Job, input writingSaveInput) {
	question, err := a.earlyQuestionForGeneration(job, input.DiscoverySource, input.DiscoveryPrint, input.SourceQuestionID)
	if err != nil || question.Key != input.QuestionKey {
		writeResumeError(w, http.StatusConflict, "Not saved because this discovered question changed. Review the current question before continuing.")
		return
	}
	if !question.Editable {
		reason := question.ManualReason
		if reason == "" {
			reason = "This discovered question requires manual completion."
		}
		writeResumeError(w, http.StatusConflict, reason)
		return
	}
	if input.Revision < 0 || input.Revision != question.Revision {
		writeResumeError(w, http.StatusConflict, "This early answer changed in another window. Review the saved version before editing.")
		return
	}
	if len(input.Answer) > 12000 || !utf8.ValidString(input.Answer) || strings.ContainsRune(input.Answer, 0) {
		writeResumeError(w, http.StatusUnprocessableEntity, "Answer is too long or invalid.")
		return
	}
	switch strings.ToLower(question.Field.Kind) {
	case "select", "radio", "multi_value_single_select":
		valid := input.Answer == ""
		for _, option := range question.Field.Options {
			if option == input.Answer {
				valid = true
			}
		}
		if !valid {
			writeResumeError(w, http.StatusUnprocessableEntity, "Choose one of the published options.")
			return
		}
	}
	var earlyOriginJSON string
	earlyOriginRequested := input.OriginEntryID > 0 || input.OriginClear
	if input.OriginEntryID > 0 {
		if input.OriginRevision < 1 {
			writeResumeError(w, http.StatusBadRequest, "Review the selected source before saving with provenance.")
			return
		}
		entry, err := a.answerLibraryBySourceID(a.db, input.OriginEntryID)
		if errors.Is(err, sql.ErrNoRows) {
			writeResumeError(w, http.StatusConflict, "The selected source changed or was deleted. Review the updated example or save as independent writing.")
			return
		}
		if err != nil {
			writeResumeError(w, http.StatusInternalServerError, "Unable to verify the selected source")
			return
		}
		if entry.Revision != input.OriginRevision {
			writeResumeError(w, http.StatusConflict, "The selected source changed. Review the updated example or save as independent writing.")
			return
		}
		if !earlyPotentialNarrative(question.Field) {
			writeResumeError(w, http.StatusConflict, "The current discovered question changed. Review it before saving with this source.")
			return
		}
		earlyOriginJSON, err = buildAnswerReuseOrigin(entry, question.Revision+1)
		if err != nil {
			writeResumeError(w, http.StatusInternalServerError, "Unable to save this early answer.")
			return
		}
	}
	libraryEligible := false
	if input.SaveToLibrary {
		prepared, err := a.prepareEarlyQuestions(job, nil)
		if err != nil {
			writeResumeError(w, 500, "Unable to verify this question")
			return
		}
		for _, q := range prepared.Questions {
			if q.Key == question.Key {
				libraryEligible = q.Editable && q.TriageCategory == "answer" && earlyPotentialNarrative(q.Field)
			}
		}
	}
	tx, err := a.db.Begin()
	if err != nil {
		writeResumeError(w, 500, "Unable to save answer")
		return
	}
	defer tx.Rollback()
	now := answerLibraryTimestamp()
	result, err := tx.Exec(`INSERT INTO early_application_answers(job_id,destination,discovery_source,source_question_id,question_label,field_snapshot_json,discovery_fingerprint,answer,revision,reconciliation_status,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,1,'unreconciled',?,?)
		ON CONFLICT(job_id,destination,discovery_source,discovery_fingerprint,source_question_id)
		DO UPDATE SET question_label=excluded.question_label,field_snapshot_json=excluded.field_snapshot_json,answer=excluded.answer,revision=early_application_answers.revision+1,reconciliation_status='unreconciled',live_inspection_fingerprint='',live_question_key='',reconciled_at='',updated_at=excluded.updated_at
		WHERE early_application_answers.revision=?`, job.ID, job.URL, input.DiscoverySource, input.SourceQuestionID, question.Field.Label, earlyFieldSnapshot(question.Field), input.DiscoveryPrint, input.Answer, now, now, input.Revision)
	if err != nil {
		log.Printf("save early writing answer %d: %v", job.ID, err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to save this early answer.")
		return
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		writeResumeError(w, http.StatusConflict, "This early answer changed in another window. Review it before continuing.")
		return
	}
	if earlyOriginRequested {
		if !validAnswerReuseOriginPayload(earlyOriginJSON) {
			writeResumeError(w, http.StatusInternalServerError, "Unable to save this early answer.")
			return
		}
		if _, err := tx.Exec(`UPDATE early_application_answers SET reuse_source_json=? WHERE job_id=? AND destination=? AND discovery_source=? AND discovery_fingerprint=? AND source_question_id=?`,
			earlyOriginJSON, job.ID, job.URL, input.DiscoverySource, input.DiscoveryPrint, input.SourceQuestionID); err != nil {
			writeResumeError(w, http.StatusInternalServerError, "Unable to save this early answer.")
			return
		}
	}
	if input.SaveToLibrary {
		if err := saveWritingLibrary(tx, job, input, question.Field.Label, question.Field.Kind, libraryEligible); err != nil {
			writeResumeError(w, 409, "Not saved. This question is ineligible or matches conflicting library entries. Review the library, or save for this application only.")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeResumeError(w, 500, "Unable to save answer")
		return
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{"saved": true, "key": input.QuestionKey, "revision": question.Revision + 1, "early": true})
}

type writingAssistInput struct {
	QuestionKey      string `json:"question_key"`
	Revision         int64  `json:"revision"`
	Destination      string `json:"destination"`
	Early            bool   `json:"early"`
	DiscoverySource  string `json:"discovery_source"`
	DiscoveryPrint   string `json:"discovery_fingerprint"`
	SourceQuestionID string `json:"source_question_id"`
}

// POST /api/jobs/{id}/assistance returns suggestions, AI source choices, and
// provider options for one exact current question. All authoritative reads
// happen again at review/generation time; nothing here grants authority.
func (a *App) writingAssistAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	var input writingAssistInput
	if !decodeResumeJSON(w, r, 12<<10, &input) {
		return
	}
	job, err := a.jobByID(jobID)
	if errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to read the current question")
		return
	}
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 {
		writeResumeError(w, http.StatusConflict, "Completed applications are read-only")
		return
	}
	if !questionReturnKey.MatchString(input.QuestionKey) || input.Destination != job.URL {
		writeResumeError(w, http.StatusConflict, "Review the current question")
		return
	}
	var q preparedQuestion
	if input.Early {
		if input.DiscoverySource == "" || input.DiscoveryPrint == "" || input.SourceQuestionID == "" {
			writeResumeError(w, http.StatusConflict, "The discovered question changed")
			return
		}
		early, err := a.prepareEarlyQuestions(job, nil)
		if err != nil {
			writeResumeError(w, http.StatusInternalServerError, "Unable to read the current discovered question")
			return
		}
		if early.Item == nil || early.Item.Source != input.DiscoverySource || early.Item.Fingerprint != input.DiscoveryPrint {
			writeResumeError(w, http.StatusConflict, "The discovered question changed")
			return
		}
		found := false
		for _, candidate := range early.Questions {
			if candidate.Key == input.QuestionKey && candidate.SourceQuestionID == input.SourceQuestionID && candidate.Revision == input.Revision && candidate.Destination == input.Destination && candidate.Editable && candidate.TriageCategory == "answer" {
				q = preparedQuestion{JobID: jobID, Destination: input.Destination, Field: candidate.Field, Key: input.QuestionKey, Value: candidate.Value, Revision: input.Revision, Editable: true, IsEarly: true, DiscoverySource: input.DiscoverySource, DiscoveryFingerprint: input.DiscoveryPrint, SourceQuestionID: input.SourceQuestionID}
				found = true
				break
			}
		}
		if !found {
			writeResumeError(w, http.StatusConflict, "This discovered question is no longer eligible for assistance")
			return
		}
	} else {
		profile, err := a.candidateProfile()
		if err != nil {
			writeResumeError(w, http.StatusInternalServerError, "Unable to read candidate information")
			return
		}
		prepared, err := a.prepareQuestions(job, profile)
		if err != nil {
			writeResumeError(w, http.StatusInternalServerError, "Unable to read current questions")
			return
		}
		for _, candidate := range prepared.Questions {
			if candidate.Key == input.QuestionKey && candidate.Revision == input.Revision && candidate.Destination == input.Destination && candidate.Editable {
				q = candidate
				break
			}
		}
		if q.Key == "" {
			writeResumeError(w, http.StatusConflict, "This question is no longer eligible for assistance")
			return
		}
	}
	if !narrativeAnswerEligible(q) {
		writeResumeError(w, http.StatusUnprocessableEntity, "This question does not support writing assistance")
		return
	}
	library, err := a.answerLibrarySuggestionsForQuestions([]preparedQuestion{q})
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to retrieve previous writing for this question")
		return
	}
	q.LibrarySuggestions = library[q.Key]
	evidence, err := a.careerEvidenceSuggestionsForQuestions([]preparedQuestion{q})
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to retrieve reviewed experience for this question")
		return
	}
	q.CareerEvidenceSuggestions = evidence[q.Key]
	choices := a.answerAIChoices(job, q)
	if choices == nil {
		writeResumeError(w, http.StatusUnprocessableEntity, "AI drafting is unavailable for this question")
		return
	}
	configured := make(map[string]bool)
	for _, id := range []string{"codex", "claude-cli", "ollama", "openai", "anthropic", "openrouter", "compatible"} {
		configured[id] = coverprovider.Configured(id)
	}
	var libraryCount, evidenceCount int
	_ = a.db.QueryRow(`SELECT count(*) FROM answer_library`).Scan(&libraryCount)
	_ = a.db.QueryRow(`SELECT count(*) FROM career_evidence WHERE status='reviewed'`).Scan(&evidenceCount)
	writeResumeJSON(w, http.StatusOK, map[string]any{
		"question":        toWritingQuestion(q),
		"libraryEmpty":    libraryCount == 0,
		"evidenceEmpty":   evidenceCount == 0,
		"sources":         choices,
		"providers":       applicationAnswerAIProviders(configured),
		"defaultProvider": "ollama",
		"defaultModel":    coverprovider.DefaultModel("ollama"),
	})
}

// ---- Cover letters ----

type writingLetterJSON struct {
	ID                   int64               `json:"id"`
	JobID                int64               `json:"jobId"`
	Provider             string              `json:"provider"`
	Model                string              `json:"model"`
	Body                 string              `json:"body"`
	CreatedAt            string              `json:"createdAt"`
	EditedAt             string              `json:"editedAt"`
	EvidenceRefs         []letterEvidenceRef `json:"evidenceRefs"`
	EvidenceStale        bool                `json:"evidenceStale"`
	ContentHash          string              `json:"contentHash"`
	MasterResumeID       int64               `json:"masterResumeId"`
	MasterSHA            string              `json:"masterSha"`
	MasterName           string              `json:"masterName"`
	MasterVersion        int                 `json:"masterVersion"`
	RoleResumeID         int64               `json:"roleResumeId"`
	RoleSHA              string              `json:"roleSha"`
	RoleName             string              `json:"roleName"`
	RoleVersion          int                 `json:"roleVersion"`
	ManualSource         bool                `json:"manualSource"`
	OriginRecorded       bool                `json:"originRecorded"`
	OriginSourceLetterID int64               `json:"originSourceLetterId"`
	OriginSourceJobID    int64               `json:"originSourceJobId"`
	OriginEmployer       string              `json:"originEmployer"`
	OriginJobTitle       string              `json:"originJobTitle"`
}

func (a *App) toWritingLetter(d CoverLetter) writingLetterJSON {
	refs, stale := a.coverLetterEvidenceStatus(d)
	if refs == nil {
		refs = []letterEvidenceRef{}
	}
	out := writingLetterJSON{
		ID: d.ID, JobID: d.JobID, Provider: d.Provider, Model: d.Model, Body: d.Body,
		CreatedAt: d.CreatedAt, EditedAt: d.EditedAt,
		EvidenceRefs: refs, EvidenceStale: stale, ContentHash: d.ContentHash(),
		MasterResumeID: d.MasterResumeID, MasterSHA: d.MasterSHA256,
		RoleResumeID: d.RoleResumeID, RoleSHA: d.RoleSHA256,
		ManualSource: d.Provider == "manual",
	}
	if d.MasterResumeID > 0 {
		if resume, err := a.resumeByID(d.MasterResumeID); err == nil {
			out.MasterName = resume.Name
			out.MasterVersion = resume.Version
		}
	}
	if d.RoleResumeID > 0 {
		if resume, err := a.resumeByID(d.RoleResumeID); err == nil {
			out.RoleName = resume.Name
			out.RoleVersion = resume.Version
		}
	}
	var raw string
	if err := a.db.QueryRow(`SELECT COALESCE(reuse_source_json,'') FROM cover_letters WHERE id=?`, d.ID).Scan(&raw); err == nil && strings.TrimSpace(raw) != "" {
		var origin letterReuseOrigin
		if err := json.Unmarshal([]byte(raw), &origin); err == nil && origin.SourceLetterID > 0 {
			out.OriginRecorded = true
			out.OriginSourceLetterID = origin.SourceLetterID
			out.OriginSourceJobID = origin.SourceJobID
			out.OriginEmployer = origin.SourceEmployer
			out.OriginJobTitle = origin.SourceJobTitle
		}
	}
	return out
}

type writingLetterSourceJSON struct {
	Name      string `json:"name"`
	Version   int    `json:"version"`
	Ready     bool   `json:"ready"`
	Message   string `json:"message"`
	Excerpt   string `json:"excerpt"`
	Truncated bool   `json:"truncated"`
}

func toWritingLetterSource(in resumeSourcePreview) writingLetterSourceJSON {
	excerpt := in.Excerpt
	truncated := in.Truncated
	if runes := []rune(excerpt); len(runes) > 2000 {
		excerpt = string(runes[:2000])
		truncated = true
	}
	return writingLetterSourceJSON{
		Name: in.Name, Version: in.Version, Ready: in.Ready,
		Message: in.Message, Excerpt: excerpt, Truncated: truncated,
	}
}

// buildWritingLetters assembles the read-only cover-letter state.
func (a *App) buildWritingLetters(jobID int64, draftID int64) (map[string]any, error) {
	job, err := a.jobByID(jobID)
	if err != nil {
		return nil, err
	}
	summaries, err := a.listCoverLetters(jobID)
	if err != nil {
		return nil, err
	}
	var current CoverLetter
	if draftID > 0 {
		current, err = a.coverLetterByID(jobID, draftID)
	} else {
		current, err = a.selectedCoverLetter(jobID)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var selected *Resume
	if job.SelectedResumeID > 0 {
		if resume, resumeErr := a.resumeByID(job.SelectedResumeID); resumeErr == nil {
			selected = &resume
		}
	}
	sources := a.coverLetterSources(job, selected)
	draftJSON := make([]map[string]any, 0, len(summaries))
	for _, s := range summaries {
		draftJSON = append(draftJSON, map[string]any{
			"id": s.ID, "provider": s.Provider, "model": s.Model,
			"createdAt": s.CreatedAt, "editedAt": s.EditedAt,
			"current": current.ID == s.ID,
		})
	}
	var currentJSON *writingLetterJSON
	if current.ID > 0 {
		converted := a.toWritingLetter(current)
		currentJSON = &converted
	}
	configured := make(map[string]bool)
	for _, id := range []string{"codex", "claude-cli", "ollama", "openai", "anthropic", "openrouter", "compatible"} {
		configured[id] = coverprovider.Configured(id)
	}
	evidenceChoices := a.reviewedCareerEvidenceChoices()
	if evidenceChoices == nil {
		evidenceChoices = []letterEvidenceChoice{}
	}
	return map[string]any{
		"drafts":  draftJSON,
		"current": currentJSON,
		"sources": map[string]any{
			"master":              toWritingLetterSource(sources.Master),
			"role":                toWritingLetterSource(sources.Role),
			"jobDescriptionReady": sources.JobDescriptionReady,
			"ready":               sources.Ready,
		},
		"evidenceChoices": evidenceChoices,
		"providers":       applicationAnswerAIProviders(configured),
		"defaultProvider": "ollama",
		"defaultModel":    coverprovider.DefaultModel("ollama"),
		"editingAllowed":  job.Stage != StageApplied && job.Stage != StageClosed && job.SubmissionCount == 0,
	}, nil
}

// GET /api/jobs/{id}/letters lists drafts, the latest draft, and source state.
func (a *App) writingLettersAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	payload, err := a.buildWritingLetters(jobID, 0)
	if errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	if err != nil {
		log.Printf("load writing letters %d: %v", jobID, err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to load cover letters")
		return
	}
	writeResumeJSON(w, http.StatusOK, payload)
}

func parseLetterDraftID(r *http.Request) (int64, error) {
	draft, err := strconv.ParseInt(r.PathValue("draft"), 10, 64)
	if err != nil || draft <= 0 {
		return 0, fmt.Errorf("bad draft id")
	}
	return draft, nil
}

// GET /api/jobs/{id}/letters/{draft} returns one saved draft with staleness.
func (a *App) writingLetterAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	draftID, err := parseLetterDraftID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown draft")
		return
	}
	draft, err := a.coverLetterByID(jobID, draftID)
	if errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusNotFound, "Unknown draft")
		return
	}
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to load cover letter")
		return
	}
	writeResumeJSON(w, http.StatusOK, a.toWritingLetter(draft))
}

type writingLetterSaveInput struct {
	Body        string `json:"body"`
	BodyHash    string `json:"expected_hash"`
	PriorLetter int64  `json:"prior_letter_id"`
	PriorJob    int64  `json:"prior_job_id"`
	PriorHash   string `json:"prior_body_hash"`
}

// POST /api/jobs/{id}/letters/{draft} edits an existing draft in place.
// Generating another letter creates a new draft instead of overwriting an edit.
func (a *App) writingLetterSaveAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	draftID, err := parseLetterDraftID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown draft")
		return
	}
	job, err := a.jobByID(jobID)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 {
		writeResumeError(w, http.StatusConflict, "This application is no longer active. Drafts are read-only.")
		return
	}
	var input writingLetterSaveInput
	if !decodeResumeJSON(w, r, 3*maxLetterBytes+4096, &input) {
		return
	}
	if err := a.saveCoverLetterBodyChecked(jobID, draftID, input.Body, strings.TrimSpace(input.BodyHash)); errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusNotFound, "This saved draft is unavailable for this job. Your submitted text has not been saved.")
		return
	} else if err != nil && err.Error() == "concurrent edit" {
		writeResumeError(w, http.StatusConflict, "This draft changed in another window. Review the current text before saving.")
		return
	} else if err != nil {
		writeResumeError(w, http.StatusUnprocessableEntity, "The letter could not be saved. Use 30–12000 bytes of valid text and try again.")
		return
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{"saved": true, "draftId": draftID})
}

// Select only an existing draft belonging to this editable application.
func (a *App) writingLetterSelectAPI(w http.ResponseWriter, r *http.Request) {
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	var input struct {
		DraftID int64 `json:"draftId"`
	}
	if !decodeResumeJSON(w, r, 4096, &input) {
		return
	}
	result, err := a.db.Exec(`UPDATE jobs SET selected_cover_letter_id=? WHERE id=? AND stage NOT IN ('applied','closed') AND NOT EXISTS(SELECT 1 FROM submissions WHERE job_id=jobs.id) AND EXISTS(SELECT 1 FROM cover_letters WHERE id=? AND job_id=jobs.id)`, input.DraftID, jobID, input.DraftID)
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to select the letter.")
		return
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		writeResumeError(w, http.StatusConflict, "This letter cannot be selected for this application.")
		return
	}
	writeResumeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

// A manual draft has no AI or résumé source. It shares the existing letter
// writer, validation, export, and transfer identity with generated drafts.
func (a *App) writingLetterManualAPI(w http.ResponseWriter, r *http.Request) {
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
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 {
		writeResumeError(w, http.StatusConflict, "This application is not editable.")
		return
	}
	var input writingLetterSaveInput
	if !decodeResumeJSON(w, r, 3*maxLetterBytes+4096, &input) {
		return
	}
	if !validLetterBody(input.Body) {
		writeResumeError(w, http.StatusUnprocessableEntity, "Use 30–12000 bytes of valid text and try again.")
		return
	}
	var originJSON string
	if input.PriorLetter > 0 {
		priorJobID := input.PriorJob
		if priorJobID <= 0 {
			priorJobID = jobID
		}
		source, err := a.coverLetterByID(priorJobID, input.PriorLetter)
		if errors.Is(err, sql.ErrNoRows) {
			writeResumeError(w, http.StatusConflict, "The selected prior letter changed or was deleted. Review the catalog or save as independent writing.")
			return
		}
		if err != nil {
			writeResumeError(w, http.StatusInternalServerError, "Unable to verify the selected prior letter")
			return
		}
		if input.PriorHash != "" && source.ContentHash() != input.PriorHash {
			writeResumeError(w, http.StatusConflict, "The selected prior letter changed. Review the updated text or save as independent writing.")
			return
		}
		sourceJob, err := a.jobByID(source.JobID)
		if err != nil {
			writeResumeError(w, http.StatusConflict, "The selected prior letter changed or was deleted. Review the catalog or save as independent writing.")
			return
		}
		originJSON, err = a.buildLetterReuseOrigin(source, sourceJob)
		if err != nil || !validLetterReuseOriginPayload(originJSON) {
			writeResumeError(w, http.StatusInternalServerError, "Unable to save the letter.")
			return
		}
	}
	tx, err := a.db.Begin()
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to save the letter.")
		return
	}
	defer tx.Rollback()
	result, err := tx.Exec(`INSERT INTO cover_letters(job_id,body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at,reuse_source_json) VALUES(?,?,'manual','',NULL,'',NULL,'','',?,?)`, jobID, input.Body, time.Now().UTC().Format(time.RFC3339), originJSON)
	if err != nil {
		log.Printf("save manual letter: %v", err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to save the letter.")
		return
	}
	id, err := result.LastInsertId()
	if err == nil {
		var selection sql.Result
		selection, err = tx.Exec(`UPDATE jobs SET selected_cover_letter_id=? WHERE id=? AND stage NOT IN ('applied','closed') AND NOT EXISTS(SELECT 1 FROM submissions WHERE job_id=jobs.id)`, id, jobID)
		if err == nil {
			var n int64
			n, err = selection.RowsAffected()
			if err == nil && n != 1 {
				err = errors.New("application no longer editable")
			}
		}
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to save the letter.")
		return
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{"saved": true, "draftId": id})
}

type writingLetterGenerateInput struct {
	Provider        string   `json:"provider"`
	Model           string   `json:"model"`
	LetterResumeID  int64    `json:"letter_resume_id"`
	CareerEvidence  []string `json:"career_evidence"`
	ConfirmExternal string   `json:"confirm_external"`
	ConfirmLocal    string   `json:"confirm_local"`
}

func writingLetterFail(w http.ResponseWriter, code string) {
	messages := map[string]string{
		"model":                    "Enter a model supported by this provider. See AI connections for its default and setup.",
		"sources":                  "Select current reviewed Career Evidence or a readable resume. Shorten oversized evidence and review it again.",
		"source-description":       "Add a saved job description before generating a cover letter.",
		"source-resume-missing":    "Choose an active role-specific resume for this job.",
		"source-master-missing":    "Designate an active master resume in the document library.",
		"source-master-unreadable": "The master resume text could not be read. Use another DOCX or PDF master.",
		"source-resume-invalid":    "Choose an active non-master role resume for this letter.",
		"source-resume-unreadable": "The role resume text could not be read. Use another DOCX or PDF resume.",
		"failed":                   "The provider could not complete this request. Prior drafts are unchanged; try again.",
		"invalid":                  "Confirm external transmission, or local-only handling for Ollama, before generating.",
	}
	message, ok := messages[code]
	if !ok {
		message = "Cover letter generation is unavailable. Prior drafts are unchanged."
		if coverprovider.ValidProvider(code) {
			message = "Check this provider and model in AI connections. Prior drafts are unchanged."
		}
	}
	writeResumeJSON(w, http.StatusUnprocessableEntity, map[string]string{"code": code, "message": message})
}

// POST /api/jobs/{id}/letters generates a new draft through the Go-owned
// provider path. A failure never touches saved drafts.
func (a *App) writingLetterGenerateAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	var input writingLetterGenerateInput
	if !decodeResumeJSON(w, r, 8<<10, &input) {
		return
	}
	provider := strings.TrimSpace(input.Provider)
	if provider == "" {
		provider = "ollama"
	}
	if !coverprovider.ValidProvider(provider) {
		writingLetterFail(w, "model")
		return
	}
	if input.ConfirmExternal != "yes" && (provider != "ollama" || input.ConfirmLocal != "yes") {
		writingLetterFail(w, "invalid")
		return
	}
	model := strings.TrimSpace(input.Model)
	if model == "" && a.coverLetterGenerate == nil {
		model = coverprovider.DefaultModel(provider)
	}
	if model != "" && !coverprovider.ValidModel(model) {
		writingLetterFail(w, "model")
		return
	}
	if model == "" && a.coverLetterGenerate == nil {
		writingLetterFail(w, "model")
		return
	}
	var runner coverprovider.Runner
	if a.coverLetterGenerate == nil {
		runner, err = coverprovider.Configure(provider, model)
		if err != nil {
			writingLetterFail(w, coverErrorNotice(err))
			return
		}
	}
	evidence, err := a.letterEvidenceForJob(jobID, input.LetterResumeID)
	if errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	if errors.Is(err, errCoverSources) {
		code := "sources"
		var source coverSourceError
		if errors.As(err, &source) {
			code = string(source)
		}
		writingLetterFail(w, code)
		return
	}
	if err != nil {
		log.Printf("cover letter source preparation failed")
		writingLetterFail(w, "failed")
		return
	}
	if len(input.CareerEvidence) > 0 {
		if err := a.attachCareerEvidence(&evidence, input.CareerEvidence); err != nil {
			writingLetterFail(w, "sources")
			return
		}
	}
	var body string
	ctx := r.Context()
	if a.coverLetterGenerate != nil {
		body, err = a.coverLetterGenerate(ctx, evidence.prompt)
	} else if len(evidence.careerRefs) > 0 {
		body, err = runner.GenerateLetter(ctx, evidence.prompt, a.resources.coverLetterInstructions)
	} else {
		body, err = runner.Generate(ctx, evidence.prompt, a.resources.coverLetterInstructions)
	}
	if err == nil {
		err = validateGeneratedLetter(body)
	}
	if err != nil {
		code := coverErrorNotice(err)
		stage, detail := "", ""
		if errors.Is(err, errCoverOutput) {
			stage, detail = classifyLetterOutput(body)
		}
		if stage == "" {
			log.Printf("cover letter generation: provider=%s status=%s", provider, code)
		} else {
			log.Printf("cover letter generation: provider=%s status=%s stage=%s %s", provider, code, stage, detail)
		}
		writingLetterFail(w, code)
		return
	}
	if model == "" {
		model = "test-model"
	}
	refsJSON, _ := json.Marshal(evidence.careerRefs)
	if len(evidence.careerRefs) == 0 {
		refsJSON = []byte("")
	}
	result, err := a.db.Exec(`INSERT INTO cover_letters
        (job_id,body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at,evidence_refs)
        VALUES(?,?,?,?,?,?,?,?,?,?,?)`, jobID, body, provider, model, evidence.master.ID, evidence.master.SHA256,
		evidence.role.ID, evidence.role.SHA256, evidence.jobDescriptionHash, time.Now().UTC().Format(time.RFC3339), string(refsJSON))
	if err != nil {
		log.Printf("save generated cover letter: %v", err)
		writingLetterFail(w, "failed")
		return
	}
	draftID, err := result.LastInsertId()
	if err != nil {
		log.Printf("cover letter draft id: %v", err)
		writingLetterFail(w, "failed")
		return
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{"generated": true, "draftId": draftID})
}

type coverLetterCatalogItem struct {
	ID          int64  `json:"id"`
	JobID       int64  `json:"jobId"`
	Employer    string `json:"employer"`
	JobTitle    string `json:"jobTitle"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	CreatedAt   string `json:"createdAt"`
	EditedAt    string `json:"editedAt"`
	Preview     string `json:"preview"`
	MasterName  string `json:"masterName"`
	MasterVer   int    `json:"masterVersion"`
	RoleName    string `json:"roleName"`
	RoleVersion int    `json:"roleVersion"`
	Manual      bool   `json:"manual"`
	Current     bool   `json:"current,omitempty"`
}

func coverLetterPreview(body string) string {
	runes := []rune(strings.TrimSpace(body))
	if len(runes) <= 320 {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:320])) + "…"
}

func (a *App) coverLetterHistoryAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 1 && parsed <= 50 {
			limit = parsed
		} else {
			writeResumeError(w, http.StatusBadRequest, "Browse limit must be between 1 and 50")
			return
		}
	}
	cursor := int64(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("cursor")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			writeResumeError(w, http.StatusBadRequest, "Invalid browse cursor")
			return
		}
		cursor = parsed
	}
	summaries, hasMore, err := a.listCoverLettersPage(jobID, cursor, limit)
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to browse saved letters")
		return
	}
	selected, _ := a.selectedCoverLetter(jobID)
	items := make([]coverLetterCatalogItem, 0, len(summaries))
	var lastID int64
	for _, s := range summaries {
		draft, err := a.coverLetterByID(jobID, s.ID)
		if err != nil {
			continue
		}
		item := a.toCatalogItem(draft, selected.ID == draft.ID)
		items = append(items, item)
		lastID = s.ID
	}
	if items == nil {
		items = []coverLetterCatalogItem{}
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{
		"entries": items, "nextCursor": lastID, "hasMore": hasMore, "selectedId": selected.ID,
	})
}

func (a *App) toCatalogItem(draft CoverLetter, current bool) coverLetterCatalogItem {
	job, _ := a.jobByID(draft.JobID)
	item := coverLetterCatalogItem{
		ID: draft.ID, JobID: draft.JobID, Provider: draft.Provider, Model: draft.Model,
		CreatedAt: draft.CreatedAt, EditedAt: draft.EditedAt,
		Preview: coverLetterPreview(draft.Body), Manual: draft.Provider == "manual", Current: current,
	}
	if job.ID > 0 {
		item.Employer = job.Company
		if item.Employer == "" {
			item.Employer = job.Host
		}
		item.JobTitle = job.Title
	}
	if draft.MasterResumeID > 0 {
		if resume, err := a.resumeByID(draft.MasterResumeID); err == nil {
			item.MasterName = resume.Name
			item.MasterVer = resume.Version
		}
	}
	if draft.RoleResumeID > 0 {
		if resume, err := a.resumeByID(draft.RoleResumeID); err == nil {
			item.RoleName = resume.Name
			item.RoleVersion = resume.Version
		}
	}
	return item
}

// GET /api/cover-letters browses retained letters across opportunities with
// bounded descending-ID paging and optional employer/role search.
func (a *App) coverLetterCatalogAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 1 && parsed <= 50 {
			limit = parsed
		} else {
			writeResumeError(w, http.StatusBadRequest, "Browse limit must be between 1 and 50")
			return
		}
	}
	cursor := int64(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("cursor")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			writeResumeError(w, http.StatusBadRequest, "Invalid browse cursor")
			return
		}
		cursor = parsed
	}
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if len(query) > 200 {
		writeResumeError(w, http.StatusBadRequest, "Search text is too long")
		return
	}
	excludeJob := int64(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("exclude_job")); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			excludeJob = parsed
		}
	}
	var rows *sql.Rows
	var err error
	if query == "" {
		rows, err = a.db.Query(`SELECT id,job_id FROM cover_letters WHERE (?=0 OR id<?) AND (?=0 OR job_id<>?) ORDER BY id DESC LIMIT ?`, cursor, cursor, excludeJob, excludeJob, limit+1)
	} else {
		like := "%" + query + "%"
		rows, err = a.db.Query(`SELECT c.id,c.job_id FROM cover_letters c JOIN jobs j ON j.id=c.job_id
		 WHERE (?=0 OR c.id<?) AND (?=0 OR c.job_id<>?) AND (j.company LIKE ? OR j.title LIKE ? OR j.host LIKE ?)
		 ORDER BY c.id DESC LIMIT ?`, cursor, cursor, excludeJob, excludeJob, like, like, like, limit+1)
	}
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to browse saved letters")
		return
	}
	defer rows.Close()
	type ref struct{ id, jobID int64 }
	var refs []ref
	for rows.Next() {
		var item ref
		if err := rows.Scan(&item.id, &item.jobID); err != nil {
			writeResumeError(w, http.StatusInternalServerError, "Unable to browse saved letters")
			return
		}
		refs = append(refs, item)
	}
	if err := rows.Err(); err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to browse saved letters")
		return
	}
	hasMore := len(refs) > limit
	if hasMore {
		refs = refs[:limit]
	}
	items := make([]coverLetterCatalogItem, 0, len(refs))
	var lastID int64
	var total int64
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM cover_letters`).Scan(&total)
	for _, item := range refs {
		draft, err := a.coverLetterByID(item.jobID, item.id)
		if err != nil {
			continue
		}
		items = append(items, a.toCatalogItem(draft, false))
		lastID = item.id
	}
	if items == nil {
		items = []coverLetterCatalogItem{}
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{
		"entries": items, "nextCursor": lastID, "hasMore": hasMore,
		"libraryEmpty": total == 0, "query": query, "bounded": true,
	})
}

// GET /api/cover-letters/{id} reads one retained letter with source job and
// pinned résumé metadata. Read-only: it never changes selection.
func (a *App) coverLetterCatalogDetailAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeResumeError(w, http.StatusNotFound, "Unknown letter")
		return
	}
	var jobID int64
	if err := a.db.QueryRow(`SELECT job_id FROM cover_letters WHERE id=?`, id).Scan(&jobID); err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown letter")
		return
	}
	draft, err := a.coverLetterByID(jobID, id)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown letter")
		return
	}
	detail := a.toWritingLetter(draft)
	job, _ := a.jobByID(jobID)
	writeResumeJSON(w, http.StatusOK, map[string]any{
		"letter": detail,
		"job":    map[string]any{"id": job.ID, "employer": job.Company, "host": job.Host, "title": job.Title},
	})
}
