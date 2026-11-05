package app

// Cross-experience synthesis and applicant challenges for the YM-003B React
// Studio slice.
//
// One singleton synthesis snapshot persists the applicant's selected
// interpretation across reviewed Career Evidence records. Every connection
// stays a hypothesis: challenges record the applicant's judgment without
// editing evidence or deleting the interpretation, and source corrections
// surface as visible staleness keyed by evidence hash — never silent
// rewrites and never adopted facts.

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	discovery "github.com/jenksed/yourmove-career-discovery"
	coverprovider "github.com/jenksed/yourmove-go/internal/coverletter/provider"
)

func careerSynthesisInputHash(inputs []discovery.ConnectionEvidence) string {
	inputsJSON, _ := json.Marshal(inputs)
	return careerDiscoveryHash("ym003b/v1\x00synthesis\x00" + string(inputsJSON) + "\x00" + discovery.ConnectionPolicy)
}

func careerSynthesisTransmitBytes(inputs []discovery.ConnectionEvidence) int {
	inputsJSON, _ := json.Marshal(inputs)
	bytes := len(inputsJSON) + len(discovery.ConnectionPolicy)
	for _, input := range inputs {
		bytes += len(input.Title) + len(input.Situation) + len(input.Actions)
	}
	return bytes
}

type savedSynthesis struct {
	EvidenceHash string
	Synthesis    *discovery.ConnectionSynthesis
	Notice       string
	Provider     string
	Model        string
	UpdatedAt    string
}

func loadSavedSynthesis(db *sql.DB) (savedSynthesis, error) {
	var saved savedSynthesis
	var raw string
	err := db.QueryRow(`SELECT evidence_hash,synthesis,notice,provider,model,updated_at
		FROM career_connection_synthesis WHERE id=1`).
		Scan(&saved.EvidenceHash, &raw, &saved.Notice, &saved.Provider, &saved.Model, &saved.UpdatedAt)
	if err != nil {
		return saved, err
	}
	if raw != "" {
		var synthesis discovery.ConnectionSynthesis
		if json.Unmarshal([]byte(raw), &synthesis) != nil {
			return saved, sql.ErrNoRows
		}
		saved.Synthesis = &synthesis
	}
	return saved, nil
}

func loadSynthesisChallenges(db *sql.DB, synthesisHash string) map[string][2]string {
	challenges := map[string][2]string{}
	if synthesisHash == "" {
		return challenges
	}
	rows, err := db.Query(`SELECT connection_id,state,note FROM career_connection_challenges WHERE synthesis_hash=?`, synthesisHash)
	if err != nil {
		return challenges
	}
	defer rows.Close()
	for rows.Next() {
		var id, state, note string
		if rows.Scan(&id, &state, &note) == nil {
			challenges[id] = [2]string{state, note}
		}
	}
	return challenges
}

// careerSynthesisEligible caps the reviewed set at the engine budget,
// oldest records first by ID, and reports how many reviewed records the
// cap leaves out so the Studio can say so honestly.
func careerSynthesisEligible(a *App) (eligible []CareerEvidence, inputs []discovery.ConnectionEvidence, overflow int, err error) {
	reviewed, err := careerReviewedEvidence(a)
	if err != nil {
		return nil, nil, 0, err
	}
	capped := reviewed
	if len(capped) > discovery.MaxConnectionEvidences {
		overflow = len(capped) - discovery.MaxConnectionEvidences
		capped = capped[:discovery.MaxConnectionEvidences]
	}
	for _, entry := range capped {
		eligible = append(eligible, entry)
		inputs = append(inputs, projectConnectionEvidence(entry))
	}
	return eligible, inputs, overflow, nil
}

// careerSynthesisView renders the eligible set, the pending-step binding,
// and the saved snapshot with staleness recomputed from current evidence.
// Saved interpretations stay visible behind their banner; only a new
// explicit authorization replaces them.
func (a *App) careerSynthesisView() (map[string]any, error) {
	eligible, inputs, overflow, err := careerSynthesisEligible(a)
	if err != nil {
		return nil, err
	}
	refs := make([]map[string]any, 0, len(eligible))
	for _, entry := range eligible {
		refs = append(refs, careerEvidenceRef(entry))
	}
	currentHash := ""
	inputHash := ""
	bytes := 0
	if len(eligible) >= 2 {
		currentHash = careerSelectionHash(eligible)
		inputHash = careerSynthesisInputHash(inputs)
		bytes = careerSynthesisTransmitBytes(inputs)
	}
	view := map[string]any{
		"eligible": refs, "eligible_overflow": overflow,
		"policy_name": careerSynthesisPolicyName,
		"input_hash":  inputHash, "bytes": bytes, "current_hash": currentHash,
		"max_evidences":   discovery.MaxConnectionEvidences,
		"max_connections": discovery.MaxConnections,
		"saved":           nil,
	}
	saved, err := loadSavedSynthesis(a.db)
	if errors.Is(err, sql.ErrNoRows) {
		return view, nil
	}
	if err != nil {
		return nil, err
	}
	view["saved"] = a.careerSavedSynthesisView(saved, currentHash)
	return view, nil
}

func (a *App) careerSavedSynthesisView(saved savedSynthesis, currentHash string) map[string]any {
	view := map[string]any{
		"evidence_hash": saved.EvidenceHash, "notice": saved.Notice,
		"provider": saved.Provider, "model": saved.Model, "updated_at": saved.UpdatedAt,
		"has_synthesis": saved.Synthesis != nil, "stale": false, "integrity_ok": true,
		"synthesis": nil, "connections": []any{}, "challenges_count": 0,
	}
	if saved.Synthesis == nil {
		return view
	}
	view["synthesis"] = saved.Synthesis
	if saved.EvidenceHash != currentHash {
		view["stale"] = true
	}
	current := map[int64]CareerEvidence{}
	for _, connection := range saved.Synthesis.Connections {
		for _, support := range connection.Supports {
			if _, seen := current[support.EvidenceID]; seen {
				continue
			}
			if entry, err := careerEvidenceByID(a.db, support.EvidenceID); err == nil {
				current[support.EvidenceID] = entry
			}
		}
	}
	// Rebind the saved synthesis to current source material when every
	// cited record is still present at its cited revision: persisted or
	// tampered results cannot advertise support that no longer resolves.
	rebind := make([]discovery.ConnectionEvidence, 0, len(current))
	rebindable := true
	for _, entry := range current {
		if entry.Status != "reviewed" {
			rebindable = false
			break
		}
		rebind = append(rebind, projectConnectionEvidence(entry))
	}
	if rebindable {
		for _, connection := range saved.Synthesis.Connections {
			for _, support := range connection.Supports {
				if entry, ok := current[support.EvidenceID]; !ok || entry.Revision != support.EvidenceRevision {
					rebindable = false
					break
				}
			}
		}
	}
	if rebindable && discovery.ValidateSynthesis(*saved.Synthesis, rebind) != nil {
		view["stale"], view["integrity_ok"] = true, false
	}
	challenges := loadSynthesisChallenges(a.db, saved.EvidenceHash)
	connections := make([]map[string]any, 0, len(saved.Synthesis.Connections))
	count := 0
	for _, connection := range saved.Synthesis.Connections {
		item := map[string]any{
			"connection": connection, "challenge": "", "note": "", "stale": false,
		}
		if challenge, ok := challenges[connection.ID]; ok {
			item["challenge"], item["note"] = challenge[0], challenge[1]
			if challenge[0] != "" {
				count++
			}
		}
		for _, support := range connection.Supports {
			entry, ok := current[support.EvidenceID]
			if !ok || entry.Status != "reviewed" || entry.Revision != support.EvidenceRevision {
				item["stale"] = true
				view["stale"] = true
				break
			}
		}
		connections = append(connections, item)
	}
	view["connections"] = connections
	view["challenges_count"] = count
	return view
}

// GET /api/career/synthesis returns the eligible reviewed set, the
// pending-step binding, and the saved snapshot with staleness.
func (a *App) careerSynthesisAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	view, err := a.careerSynthesisView()
	if err != nil {
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to load the synthesis")
		return
	}
	writeCareerJSON(w, http.StatusOK, view)
}

// POST /api/career/synthesis/authorize runs one cross-experience synthesis
// after explicit dual authorization. The input hash must match the exact
// eligible evidence revisions; any concurrent correction rejects the call
// and the applicant reviews the fresh set before authorizing again.
func (a *App) careerSynthesisAuthorizeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		Provider            string `json:"provider"`
		Model               string `json:"model"`
		InputHash           string `json:"input_hash"`
		AuthorizeGeneration string `json:"authorize_generation"`
		AuthorizeExternal   string `json:"authorize_external"`
	}
	if !decodeCareerJSON(w, r, 4<<10, &payload) {
		return
	}
	if payload.AuthorizeGeneration != "yes" || payload.AuthorizeExternal != "yes" {
		careerDiscoveryError(w, http.StatusForbidden, "authorization_required", "Authorize this inference and acknowledge external transmission for this specific request")
		return
	}
	if !coverprovider.ValidProvider(payload.Provider) || !coverprovider.ValidModel(payload.Model) {
		careerDiscoveryError(w, http.StatusUnprocessableEntity, "provider_configuration",
			careerDiscoveryProviderMessage(payload.Provider, payload.Model, answerAIProviderNotice(payload.Provider, payload.Model, nil)))
		return
	}
	var runner coverprovider.Runner
	var err error
	if a.careerStudioGenerate == nil {
		runner, err = coverprovider.Configure(payload.Provider, payload.Model)
		if err != nil {
			careerDiscoveryError(w, http.StatusUnprocessableEntity, "provider_configuration",
				careerDiscoveryProviderMessage(payload.Provider, payload.Model, answerAIProviderNotice(payload.Provider, payload.Model, err)))
			return
		}
	}
	eligible, inputs, _, err := careerSynthesisEligible(a)
	if err != nil {
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to load reviewed evidence")
		return
	}
	if len(eligible) < 2 {
		careerDiscoveryError(w, http.StatusUnprocessableEntity, "not_enough_reviewed", "Review at least two Career Evidence records with complete title, situation, and actions before looking for connections.")
		return
	}
	if subtle.ConstantTimeCompare([]byte(payload.InputHash), []byte(careerSynthesisInputHash(inputs))) != 1 {
		careerDiscoveryError(w, http.StatusConflict, "changed", "The reviewed set changed. Review the current set before authorizing again.")
		return
	}
	if !a.careerStudio.begin("synthesis") {
		careerDiscoveryError(w, http.StatusConflict, "generation_in_progress", "Another synthesis is running")
		return
	}
	defer a.careerStudio.finish("synthesis")
	generate := a.careerStudioGenerate
	if generate == nil {
		generate = runner.GenerateDiscovery
	}
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	synthesis, err := discovery.SynthesizeConnections(ctx, generate, inputs)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	hash := careerSelectionHash(eligible)
	if err != nil {
		code, status, message := careerStudioFailure("synthesis", payload.Provider, payload.Model, ctx.Err(), err)
		notice := "Muse could not complete this synthesis. No interpretation was saved."
		if code == "invalid_output" {
			notice = "Muse output did not pass source or format checks. Nothing was saved; retry the synthesis."
		}
		if dbErr := a.storeSynthesisFailure(hash, notice, payload.Provider, payload.Model, now); dbErr != nil {
			log.Printf("career synthesis failure store: %v", dbErr)
		}
		careerDiscoveryError(w, status, code, message)
		return
	}
	raw, _ := json.Marshal(synthesis)
	notice := "Muse compared " + strconv.Itoa(len(eligible)) + " reviewed experience(s) together. These connections are hypotheses for your judgment; nothing was approved or adopted."
	if _, err := a.db.Exec(`INSERT INTO career_connection_synthesis(id,evidence_hash,synthesis,notice,provider,model,created_at,updated_at)
		VALUES(1,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET evidence_hash=excluded.evidence_hash,synthesis=excluded.synthesis,
		notice=excluded.notice,provider=excluded.provider,model=excluded.model,updated_at=excluded.updated_at`,
		hash, string(raw), notice, payload.Provider, payload.Model, now, now); err != nil {
		log.Printf("career synthesis store: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to save the synthesis")
		return
	}
	view, err := a.careerSynthesisView()
	if err != nil {
		log.Printf("career synthesis reload: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to reload the synthesis")
		return
	}
	writeCareerJSON(w, http.StatusOK, view)
}

// storeSynthesisFailure surfaces a failed synthesis without overwriting a
// prior good snapshot: the notice updates, the saved interpretation stands.
func (a *App) storeSynthesisFailure(hash, notice, provider, model, now string) error {
	result, err := a.db.Exec(`UPDATE career_connection_synthesis SET notice=?,provider=?,model=?,updated_at=? WHERE id=1`,
		notice, provider, model, now)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil {
		return err
	} else if count == 1 {
		return nil
	}
	_, err = a.db.Exec(`INSERT INTO career_connection_synthesis(id,evidence_hash,synthesis,notice,provider,model,created_at,updated_at)
		VALUES(1,?,'',?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, hash, notice, provider, model, now, now)
	return err
}

// POST /api/career/synthesis/challenges/{id} records the applicant's
// judgment of one saved connection. It never edits evidence, never deletes
// the interpretation, and survives restart keyed to the synthesis judged.
func (a *App) careerChallengeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	raw := strings.TrimPrefix(r.PathValue("id"), "conn-")
	number, err := strconv.Atoi(raw)
	if err != nil || number < 1 || number > discovery.MaxConnections || r.PathValue("id") != "conn-"+strconv.Itoa(number) {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_connection", "Unknown connection")
		return
	}
	var payload struct {
		State string `json:"state"`
		Note  string `json:"note"`
	}
	if !decodeCareerJSON(w, r, 4<<10, &payload) {
		return
	}
	if payload.State != "" && payload.State != "questioned" && payload.State != "unsupported" {
		careerDiscoveryError(w, http.StatusBadRequest, "invalid_challenge", "Choose a challenge state")
		return
	}
	note := strings.TrimSpace(payload.Note)
	if len(note) > 500 {
		careerDiscoveryError(w, http.StatusBadRequest, "invalid_challenge", "Keep the note under 500 bytes")
		return
	}
	saved, err := loadSavedSynthesis(a.db)
	if err != nil || saved.Synthesis == nil {
		careerDiscoveryError(w, http.StatusConflict, "no_synthesis", "No saved synthesis to challenge")
		return
	}
	found := false
	for _, connection := range saved.Synthesis.Connections {
		if connection.ID == r.PathValue("id") {
			found = true
		}
	}
	if !found {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_connection", "That connection is not in the saved synthesis")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := a.db.Exec(`INSERT INTO career_connection_challenges(synthesis_hash,connection_id,state,note,updated_at)
		VALUES(?,?,?,?,?) ON CONFLICT(synthesis_hash,connection_id) DO UPDATE SET state=excluded.state,note=excluded.note,updated_at=excluded.updated_at`,
		saved.EvidenceHash, r.PathValue("id"), payload.State, note, now); err != nil {
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to save the challenge")
		return
	}
	view, err := a.careerSynthesisView()
	if err != nil {
		log.Printf("career synthesis reload: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to reload the synthesis")
		return
	}
	writeCareerJSON(w, http.StatusOK, view)
}
