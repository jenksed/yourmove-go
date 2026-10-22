package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func (a *App) reconcileEarlyPreparation(job Job, public *Inspection, preparation *earlyQuestionPreparation) error {
	hasDraft := len(preparation.Previous) > 0
	for _, question := range preparation.Questions {
		hasDraft = hasDraft || question.Revision > 0
	}
	if !hasDraft {
		return nil
	}
	live, err := a.latestInspectionBySource(job.ID, "browser")
	liveAvailable := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	for i := range preparation.Questions {
		q := &preparation.Questions[i]
		if q.Revision == 0 {
			continue
		}
		answer := earlyApplicationAnswer{
			ID: q.EarlyAnswerID, JobID: q.JobID, Destination: q.Destination,
			DiscoverySource: q.DiscoverySource, SourceQuestionID: q.SourceQuestionID,
			QuestionLabel: q.Field.Label, Field: q.Field, DiscoveryFingerprint: q.DiscoveryFingerprint,
			Answer: q.Value, Revision: q.Revision, ReconciliationStatus: q.SavedReconciliationStatus,
			LiveInspection: q.SavedLiveInspection, LiveQuestionKey: q.SavedLiveQuestionKey,
		}
		q.Reconciliation, err = a.reconcileSavedEarlyAnswer(job, answer, public, live, liveAvailable)
		if err != nil {
			return err
		}
		if q.Reconciliation.Accepted {
			q.LocalStatus = q.Status()
		}
	}
	for i := range preparation.Previous {
		preparation.Previous[i].Reconciliation, err = a.reconcileSavedEarlyAnswer(job, preparation.Previous[i], public, live, liveAvailable)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *App) reconcileSavedEarlyAnswer(job Job, answer earlyApplicationAnswer, public *Inspection, live Inspection, liveAvailable bool) (earlyReconciliation, error) {
	result := reconcileEarlyAnswer(job, answer, public, live, liveAvailable)
	if !result.CanAccept {
		return result, nil
	}
	current, err := a.savedLiveAnswer(job, result.LiveQuestionKey)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return earlyReconciliation{}, err
	}
	if err == nil {
		result.LiveAnswerRevision = current.Revision
	}
	if answer.ReconciliationStatus != "accepted" || answer.LiveQuestionKey == "" || result.LiveQuestionKey != answer.LiveQuestionKey {
		return result, nil
	}
	if err == nil && current.Value == answer.Answer {
		result.Status = "Accepted as a live answer"
		result.Detail = "The current saved live answer still matches this early draft. Browser filling still requires separate authorization."
		result.Accepted = true
		result.CanAccept = false
		return result, nil
	}
	result.Accepted = false
	result.CanAccept = false
	switch {
	case errors.Is(err, sql.ErrNoRows):
		result.Status = "Live answer removed"
		result.Detail = "This early answer was accepted before, but its live saved answer has since been removed. The current live workbench is unchanged; this early copy remains saved for review."
	case current.Value == "":
		result.Status = "Live answer cleared"
		result.Detail = "This early answer was accepted before, but its live saved answer has since been cleared. The cleared live answer stays unchanged; this early copy remains saved for review."
	default:
		result.Status = "Live answer changed"
		result.Detail = "This early answer was accepted before, but its live saved answer has since changed. The newer live answer stays unchanged; this early copy remains saved for review."
	}
	return result, nil
}

func (a *App) savedLiveAnswer(job Job, questionKey string) (savedApplicationAnswer, error) {
	var answer savedApplicationAnswer
	err := a.db.QueryRow(`SELECT question_key,label,answer,revision FROM application_answers
		WHERE job_id=? AND destination=? AND provider=? AND question_key=?`,
		job.ID, job.URL, browser.PostingPlatform(job.URL), questionKey).Scan(&answer.Key, &answer.Label, &answer.Value, &answer.Revision)
	return answer, err
}

func reconcileEarlyAnswer(job Job, answer earlyApplicationAnswer, public *Inspection, live Inspection, liveAvailable bool) earlyReconciliation {
	result := earlyReconciliation{Status: "Not observed yet", Detail: "This saved question has not been matched to a current rendered application control."}
	if answer.Destination != job.URL {
		return earlyReconciliation{Status: "Manual only", Detail: "The saved employer destination changed. The original answer is preserved and cannot be used for this destination."}
	}
	if answer.DiscoverySource != "public_api" && answer.DiscoverySource != "preview" {
		return earlyReconciliation{Status: "Manual only", Detail: "This discovery source has no qualified identity relationship with live controls."}
	}
	if public != nil && answer.DiscoverySource == public.Source && answer.Destination == public.ApplicationURL {
		current, found := fieldByID(public.Fields, answer.SourceQuestionID)
		if !found {
			return earlyReconciliation{Status: "Question changed", Detail: "The latest published schema no longer contains this question identity. Its original wording and answer are preserved for manual review."}
		}
		if earlyFieldSnapshot(current) != earlyFieldSnapshot(answer.Field) {
			return earlyReconciliation{Status: "Question changed", Detail: "The published question or its options changed after this answer was written. Review and adapt the saved answer before using it."}
		}
	}
	if !liveAvailable {
		return result
	}
	if live.Source != "browser" || live.Platform != browser.PostingPlatform(job.URL) || !browser.OwnsPostingURL(job.URL, live.ApplicationURL) {
		return earlyReconciliation{Status: "Manual only", Detail: "The current browser inspection does not belong to the saved employer posting."}
	}
	var name string
	switch answer.DiscoverySource {
	case "public_api":
		if live.Platform != "greenhouse" || answer.Field.Group != "application" || !strings.HasPrefix(answer.SourceQuestionID, "application:") {
			return earlyReconciliation{Status: "Manual only", Detail: "No provider-specific identity rule supports mapping this published field to the live form."}
		}
		name = strings.TrimPrefix(answer.SourceQuestionID, "application:")
	case "preview":
		if live.Platform != "ashby" || !strings.HasPrefix(answer.SourceQuestionID, "preview:") {
			return earlyReconciliation{Status: "Manual only", Detail: "No provider-specific identity rule supports mapping this preview field to the live form."}
		}
		name = strings.TrimPrefix(answer.SourceQuestionID, "preview:")
	default:
		return earlyReconciliation{Status: "Manual only", Detail: "No provider-specific identity rule supports mapping this question to the live form."}
	}
	if name == "" || strings.Contains(name, ":") {
		return earlyReconciliation{Status: "Manual only", Detail: "This discovered question does not have a qualified field identity."}
	}
	liveID := "browser:" + name
	count := 0
	var candidate schema.Field
	labelCount := 0
	for _, field := range live.Fields {
		if field.ID == liveID {
			count++
			candidate = field.Field
		}
		if reconciliationLabel(field.Label) == reconciliationLabel(answer.Field.Label) {
			labelCount++
		}
	}
	if count > 1 {
		return earlyReconciliation{Status: "Ambiguous", Detail: "More than one visible live control uses the published field name. No answer was mapped."}
	}
	if count == 1 && labelCount > 1 {
		return earlyReconciliation{Status: "Ambiguous", Detail: "Several visible live controls share this question wording. Review the controls manually before using the prepared answer."}
	}
	if count == 0 {
		if labelCount > 1 {
			return earlyReconciliation{Status: "Ambiguous", Detail: "Several visible controls have the same wording, and none has the published field identity. Review them manually."}
		}
		if labelCount == 1 {
			return earlyReconciliation{Status: "Question changed", Detail: "The wording appears on the live form, but the live control has a different field identity. Wording alone cannot authorize a mapping."}
		}
		if live.Completeness == "rendered_complete" {
			return earlyReconciliation{Status: "No longer available", Detail: "A complete observation of the relevant form contains no control with this published field identity. The saved answer remains available for reference."}
		}
		return result
	}
	result.LiveField = candidate
	result.LiveQuestionKey = questionKey(job.URL, live.Platform, candidate)
	result.LiveInspectionFingerprint = live.Fingerprint
	if candidate.Sensitive || answer.Field.Sensitive || answerLibraryContainsAnySensitivePhrase(candidate.Label) {
		result.Status = "Manual only"
		result.Detail = "This live field may contain sensitive information. Complete it manually on the employer form."
		return result
	}
	if !editableQuestion(candidate, true) {
		result.Status = "Manual only"
		result.Detail = "The live control is unsupported for saved-answer transfer and must be completed manually."
		return result
	}
	var differences []string
	if reconciliationLabel(answer.Field.Label) != reconciliationLabel(candidate.Label) {
		differences = append(differences, "question wording")
	}
	typesMatch := answer.Field.Kind == candidate.Kind
	if answer.DiscoverySource == "public_api" {
		typesMatch = greenhouseControlTypesMatch(answer.Field.Kind, candidate.Kind)
	}
	if !typesMatch {
		differences = append(differences, "control type")
	}
	if answer.Field.Required != candidate.Required {
		differences = append(differences, "required status")
	}
	if !sameOptions(answer.Field.Options, candidate.Options) {
		differences = append(differences, "options")
	}
	if len(differences) > 0 {
		result.Status = "Question changed"
		result.Detail = "The live " + strings.Join(differences, ", ") + " differs from the published question. Review and adapt the answer manually."
		return result
	}
	result.Status = "Ready for explicit review"
	if answer.DiscoverySource == "preview" {
		result.Detail = "The same Ashby control identity is present and the visible question details match. Review both questions and choose whether to use the saved answer."
	} else {
		result.Detail = "The unique Greenhouse field identity and published question details match this rendered control. Review both questions and choose whether to use the saved answer."
	}
	result.CanAccept = strings.TrimSpace(answer.Answer) != ""
	if !result.CanAccept {
		result.Detail += " Save a nonempty early answer before accepting it."
	}
	return result
}

func sameOptions(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func greenhouseControlTypesMatch(published, observed string) bool {
	switch strings.ToLower(published) {
	case "input_text":
		return observed == "text"
	case "textarea":
		return observed == "textarea"
	case "input_file":
		return observed == "file"
	case "email":
		return observed == "email"
	case "tel":
		return observed == "tel"
	case "url":
		return observed == "url"
	case "multi_value_single_select", "select":
		return observed == "select"
	case "radio":
		return observed == "radio"
	default:
		return published == observed
	}
}

// Keep comparison deliberately narrow: collapse presentation whitespace and
// remove the asterisk used by the live inspector for required labels.
func reconciliationLabel(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, "*")
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func (a *App) acceptEarlyApplicationAnswer(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil || !parseLocalForm(w, r, 24<<10) {
		http.Error(w, "Invalid reconciliation request", http.StatusBadRequest)
		return
	}
	fail := func(message string, status int) { http.Error(w, message, status) }
	job, err := a.jobByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 {
		fail("This application is no longer active.", http.StatusConflict)
		return
	}
	answerID, idErr := strconv.ParseInt(r.PostForm.Get("early_answer_id"), 10, 64)
	revision, revErr := strconv.ParseInt(r.PostForm.Get("early_revision"), 10, 64)
	liveAnswerRevision, liveAnswerRevErr := strconv.ParseInt(r.PostForm.Get("live_answer_revision"), 10, 64)
	if idErr != nil || answerID <= 0 || revErr != nil || revision <= 0 || liveAnswerRevErr != nil || liveAnswerRevision < 0 || r.PostForm.Get("destination") != job.URL {
		fail("This prepared answer changed. Review the current question before continuing.", http.StatusConflict)
		return
	}
	answer, err := earlyApplicationAnswerByID(a, id, answerID)
	if err != nil || answer.Revision != revision || answer.Destination != job.URL {
		fail("This prepared answer changed or belongs to a different employer destination.", http.StatusConflict)
		return
	}
	public, publicErr := a.latestInspectionBySource(id, answer.DiscoverySource)
	if publicErr != nil {
		fail("The original question discovery is no longer available. The prepared answer remains saved.", http.StatusConflict)
		return
	}
	live, err := a.latestInspectionBySource(id, "browser")
	if err != nil || live.Fingerprint != r.PostForm.Get("live_inspection_fingerprint") {
		fail("The employer form changed after this review. Inspect the current form and review the prepared answer again.", http.StatusConflict)
		return
	}
	current := reconcileEarlyAnswer(job, answer, &public, live, true)
	if !current.CanAccept || current.Status != "Ready for explicit review" || current.LiveQuestionKey != r.PostForm.Get("live_question_key") || current.LiveInspectionFingerprint != live.Fingerprint {
		fail("This question no longer has a unique, unchanged live match. Your early answer remains saved for manual review.", http.StatusConflict)
		return
	}
	profile, err := a.candidateProfile()
	if err != nil {
		fail("Unable to read candidate information.", http.StatusInternalServerError)
		return
	}
	prepared, err := a.prepareQuestions(job, profile)
	if err != nil {
		fail("Unable to read current employer questions.", http.StatusInternalServerError)
		return
	}
	var liveQuestion *preparedQuestion
	for i := range prepared.Questions {
		if prepared.Questions[i].Key == current.LiveQuestionKey {
			liveQuestion = &prepared.Questions[i]
			break
		}
	}
	if liveQuestion == nil || !liveQuestion.Editable {
		fail("The current employer question is not eligible for a saved local answer. Complete it manually.", http.StatusConflict)
		return
	}
	if liveQuestion.Revision != liveAnswerRevision {
		fail("The live saved answer changed after this review. Your current live answer was kept; review it before using the early draft.", http.StatusConflict)
		return
	}
	if message := validateApplicationAnswerValue(*liveQuestion, answer.Answer); message != "" {
		fail("The prepared answer does not meet the current live question rules: "+message+". Adapt it in the live question workbench.", http.StatusConflict)
		return
	}
	// Serialize acceptance with both early-answer edits and new inspections. The
	// normal live question validator and persistence function are reused below.
	tx, err := a.db.Begin()
	if err != nil {
		fail("Unable to save the live application answer.", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	var earlyRevision int64
	if err := tx.QueryRow(`SELECT revision FROM early_application_answers WHERE id=? AND job_id=? AND destination=?`, answerID, id, job.URL).Scan(&earlyRevision); err != nil || earlyRevision != revision {
		fail("This prepared answer changed during review. Review it again before accepting.", http.StatusConflict)
		return
	}
	var currentLiveFingerprint string
	if err := tx.QueryRow(`SELECT fingerprint FROM application_inspections WHERE job_id=? AND source='browser' ORDER BY checked_at DESC,id DESC LIMIT 1`, id).Scan(&currentLiveFingerprint); err != nil || currentLiveFingerprint != live.Fingerprint {
		fail("The employer form changed during review. Inspect it and reconcile again.", http.StatusConflict)
		return
	}
	var currentPublicFingerprint string
	if err := tx.QueryRow(`SELECT fingerprint FROM application_inspections WHERE job_id=? AND source=? ORDER BY checked_at DESC,id DESC LIMIT 1`, id, answer.DiscoverySource).Scan(&currentPublicFingerprint); err != nil || currentPublicFingerprint != public.Fingerprint {
		fail("The discovered question changed during review. Review the current discovery before accepting.", http.StatusConflict)
		return
	}
	if err := persistApplicationAnswer(tx, job, *liveQuestion, liveQuestion.Revision, answer.Answer, false, 0); err != nil {
		if errors.Is(err, errApplicationAnswerConflict) {
			fail("The live saved answer changed in another window. Review it before accepting this draft.", http.StatusConflict)
		} else {
			fail("Unable to save the live application answer.", http.StatusInternalServerError)
		}
		return
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z07:00")
	updated, err := tx.Exec(`UPDATE early_application_answers SET reconciliation_status='accepted',live_inspection_fingerprint=?,live_question_key=?,reconciled_at=?,updated_at=? WHERE id=? AND revision=?`, live.Fingerprint, current.LiveQuestionKey, now, now, answerID, revision)
	if err != nil {
		fail("Unable to record the reviewed reconciliation.", http.StatusInternalServerError)
		return
	}
	if changed, countErr := updated.RowsAffected(); countErr != nil || changed != 1 {
		fail("This prepared answer changed during review. Review it again before accepting.", http.StatusConflict)
		return
	}
	if err := tx.Commit(); err != nil {
		fail("Unable to save the live application answer.", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/jobs/%d?question=%s#prepare", id, url.QueryEscape(current.LiveQuestionKey)), http.StatusSeeOther)
}

func earlyApplicationAnswerByID(a *App, jobID, answerID int64) (earlyApplicationAnswer, error) {
	var answer earlyApplicationAnswer
	var snapshot string
	err := a.db.QueryRow(`SELECT id,job_id,destination,discovery_source,source_question_id,question_label,field_snapshot_json,discovery_fingerprint,answer,revision,reconciliation_status,live_inspection_fingerprint,live_question_key,reconciled_at,created_at,updated_at
		FROM early_application_answers WHERE id=? AND job_id=?`, answerID, jobID).Scan(&answer.ID, &answer.JobID, &answer.Destination, &answer.DiscoverySource, &answer.SourceQuestionID, &answer.QuestionLabel, &snapshot, &answer.DiscoveryFingerprint, &answer.Answer, &answer.Revision, &answer.ReconciliationStatus, &answer.LiveInspection, &answer.LiveQuestionKey, &answer.ReconciledAt, &answer.CreatedAt, &answer.UpdatedAt)
	if err != nil {
		return answer, err
	}
	if err := json.Unmarshal([]byte(snapshot), &answer.Field); err != nil {
		return answer, err
	}
	return answer, nil
}
