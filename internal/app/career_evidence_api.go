package app

// Career Evidence JSON transport for the YM-003B React Studio slice.
//
// Go remains authoritative: every mutation goes through the existing
// create/update/review/delete owner in career_evidence.go (plus reopen,
// which shares that single writer and revision gate). Review still requires
// its explicit confirmation, saves stay revision-aware, and the source
// snapshot fields stay read-only here — they are written only by the
// Answer Library bridge and interview promotion.

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

func careerEvidenceJSON(evidence CareerEvidence) map[string]any {
	return map[string]any{
		"id": evidence.ID, "title": evidence.Title,
		"situation": evidence.Situation, "actions": evidence.Actions,
		"employer_context": evidence.EmployerContext, "role_context": evidence.RoleContext,
		"technologies": evidence.Technologies, "outcome": evidence.Outcome,
		"outcome_state": evidence.OutcomeState, "period": evidence.Period,
		"source_notes":            evidence.SourceNotes,
		"source_library_entry_id": evidence.SourceLibraryEntryID,
		"source_question":         evidence.SourceQuestion,
		"source_writing":          evidence.SourceWriting,
		"source_employer":         evidence.SourceEmployer,
		"source_job_title":        evidence.SourceJobTitle,
		"source_host":             evidence.SourceHost,
		"status":                  evidence.Status, "revision": evidence.Revision,
		"created_at": evidence.CreatedAt, "updated_at": evidence.UpdatedAt,
	}
}

func careerEvidencePathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_evidence", "Unknown Career Evidence record")
		return 0, false
	}
	return id, true
}

func careerEvidenceWriteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		careerDiscoveryError(w, http.StatusNotFound, "unknown_evidence", "Unknown Career Evidence record")
	case errors.Is(err, ErrCareerEvidenceStale):
		careerDiscoveryError(w, http.StatusConflict, "changed", "This record changed. Reload it before saving.")
	case errors.Is(err, ErrCareerEvidenceIncomplete):
		careerDiscoveryError(w, http.StatusUnprocessableEntity, "incomplete", "Add a title, situation, and actions before marking this record reviewed.")
	case errors.Is(err, ErrCareerEvidenceOutcomeMissing):
		careerDiscoveryError(w, http.StatusUnprocessableEntity, "outcome_missing", "Add the actual outcome or change its status to Unknown or Not recorded.")
	case errors.Is(err, ErrCareerEvidenceInvalid):
		careerDiscoveryError(w, http.StatusUnprocessableEntity, "invalid_evidence", "Career Evidence fields are invalid or too long")
	default:
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to save Career Evidence")
	}
}

// GET /api/career/evidence lists every record newest first, drafts and
// reviewed together, so the Studio library can browse both.
func (a *App) careerEvidenceListAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	entries, err := a.careerEvidenceEntries()
	if err != nil {
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to load Career Evidence")
		return
	}
	out := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		out = append(out, careerEvidenceJSON(entry))
	}
	writeCareerJSON(w, http.StatusOK, map[string]any{"evidence": out})
}

// POST /api/career/evidence creates one blank draft through the existing owner.
func (a *App) careerEvidenceCreateAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := a.createCareerEvidenceDraft()
	if err != nil {
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to create a Career Evidence draft")
		return
	}
	evidence, err := careerEvidenceByID(a.db, id)
	if err != nil {
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to reload the Career Evidence draft")
		return
	}
	writeCareerJSON(w, http.StatusCreated, map[string]any{"record": careerEvidenceJSON(evidence)})
}

// GET /api/career/evidence/{id} returns one record with its source snapshot
// for inspection in the Studio editor.
func (a *App) careerEvidenceGetAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, ok := careerEvidencePathID(w, r)
	if !ok {
		return
	}
	evidence, err := careerEvidenceByID(a.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_evidence", "Unknown Career Evidence record")
		return
	}
	if err != nil {
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to load this Career Evidence record")
		return
	}
	writeCareerJSON(w, http.StatusOK, map[string]any{"record": careerEvidenceJSON(evidence)})
}

type careerEvidenceContentPayload struct {
	Title           *string `json:"title"`
	Situation       *string `json:"situation"`
	Actions         *string `json:"actions"`
	EmployerContext *string `json:"employer_context"`
	RoleContext     *string `json:"role_context"`
	Technologies    *string `json:"technologies"`
	Outcome         *string `json:"outcome"`
	OutcomeState    *string `json:"outcome_state"`
	Period          *string `json:"period"`
	SourceNotes     *string `json:"source_notes"`
}

func normalizeCareerEvidenceText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\r", "\n")
}

// careerEvidencePayloadContent requires the full editable content so a
// partial request can never silently wipe fields the editor did not send.
func careerEvidencePayloadContent(payload careerEvidenceContentPayload) (careerEvidenceContent, bool) {
	fields := []*string{
		payload.Title, payload.Situation, payload.Actions,
		payload.EmployerContext, payload.RoleContext, payload.Technologies,
		payload.Outcome, payload.OutcomeState, payload.Period, payload.SourceNotes,
	}
	for _, field := range fields {
		if field == nil {
			return careerEvidenceContent{}, false
		}
	}
	content := careerEvidenceContent{
		Title:           normalizeCareerEvidenceText(*payload.Title),
		Situation:       normalizeCareerEvidenceText(*payload.Situation),
		Actions:         normalizeCareerEvidenceText(*payload.Actions),
		EmployerContext: normalizeCareerEvidenceText(*payload.EmployerContext),
		RoleContext:     normalizeCareerEvidenceText(*payload.RoleContext),
		Technologies:    normalizeCareerEvidenceText(*payload.Technologies),
		Outcome:         normalizeCareerEvidenceText(*payload.Outcome),
		OutcomeState:    strings.TrimSpace(*payload.OutcomeState),
		Period:          normalizeCareerEvidenceText(*payload.Period),
		SourceNotes:     normalizeCareerEvidenceText(*payload.SourceNotes),
	}
	if !content.valid() {
		return careerEvidenceContent{}, false
	}
	return content, true
}

func (a *App) careerEvidenceSaveAPI(w http.ResponseWriter, r *http.Request, review bool) {
	id, ok := careerEvidencePathID(w, r)
	if !ok {
		return
	}
	var payload struct {
		careerEvidenceContentPayload
		Revision      int64  `json:"revision"`
		ConfirmReview string `json:"confirm_review"`
	}
	if !decodeCareerJSON(w, r, 96<<10, &payload) {
		return
	}
	if review && payload.ConfirmReview != "yes" {
		careerDiscoveryError(w, http.StatusBadRequest, "confirmation_required", "Confirm that this record accurately describes your experience before marking it reviewed")
		return
	}
	content, ok := careerEvidencePayloadContent(payload.careerEvidenceContentPayload)
	if !ok {
		careerDiscoveryError(w, http.StatusUnprocessableEntity, "invalid_evidence", "Career Evidence fields are invalid or too long")
		return
	}
	if err := a.updateCareerEvidence(id, payload.Revision, content, review); err != nil {
		careerEvidenceWriteError(w, err)
		return
	}
	evidence, err := careerEvidenceByID(a.db, id)
	if err != nil {
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to reload this Career Evidence record")
		return
	}
	writeCareerJSON(w, http.StatusOK, map[string]any{"record": careerEvidenceJSON(evidence)})
}

// POST /api/career/evidence/{id}/save persists an edit; saving changes to a
// reviewed record returns it to draft through the existing owner.
func (a *App) careerEvidenceSaveHandlerAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.careerEvidenceSaveAPI(w, r, false)
}

// POST /api/career/evidence/{id}/review marks the record reviewed only with
// its explicit confirmation and complete title, situation, and actions.
func (a *App) careerEvidenceReviewAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.careerEvidenceSaveAPI(w, r, true)
}

// POST /api/career/evidence/{id}/reopen returns a reviewed record to draft
// with explicit confirmation, without touching its content.
func (a *App) careerEvidenceReopenAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, ok := careerEvidencePathID(w, r)
	if !ok {
		return
	}
	var payload struct {
		Revision      int64  `json:"revision"`
		ConfirmReopen string `json:"confirm_reopen"`
	}
	if !decodeCareerJSON(w, r, 2<<10, &payload) {
		return
	}
	if payload.ConfirmReopen != "yes" {
		careerDiscoveryError(w, http.StatusBadRequest, "confirmation_required", "Confirm that this record needs more work before reopening it as a draft")
		return
	}
	if err := a.reopenCareerEvidence(id, payload.Revision); err != nil {
		if errors.Is(err, ErrCareerEvidenceInvalid) && payload.Revision > 0 {
			if current, loadErr := careerEvidenceByID(a.db, id); loadErr == nil && current.Status != "reviewed" {
				careerDiscoveryError(w, http.StatusConflict, "already_draft", "This record is already a draft")
				return
			}
		}
		careerEvidenceWriteError(w, err)
		return
	}
	evidence, err := careerEvidenceByID(a.db, id)
	if err != nil {
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to reload this Career Evidence record")
		return
	}
	writeCareerJSON(w, http.StatusOK, map[string]any{"record": careerEvidenceJSON(evidence)})
}

// POST /api/career/evidence/{id}/delete removes the record only with its
// explicit confirmation and current revision.
func (a *App) careerEvidenceDeleteAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, ok := careerEvidencePathID(w, r)
	if !ok {
		return
	}
	var payload struct {
		Revision int64  `json:"revision"`
		Confirm  string `json:"confirm"`
	}
	if !decodeCareerJSON(w, r, 2<<10, &payload) {
		return
	}
	if payload.Confirm != "yes" {
		careerDiscoveryError(w, http.StatusBadRequest, "confirmation_required", "Check the confirmation to delete this Career Evidence record")
		return
	}
	if err := a.deleteCareerEvidence(id, payload.Revision); err != nil {
		careerEvidenceWriteError(w, err)
		return
	}
	writeCareerJSON(w, http.StatusOK, map[string]any{"deleted": id})
}
