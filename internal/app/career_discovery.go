package app

// Career discovery JSON transport for the YM-003A React slice.
//
// Go remains authoritative for input validation, provider policy,
// authorization binding, interview state, and promotion into Career
// Evidence. React owns presentation, selection, and disposable drafts.
//
// Flow: an existing resume (or a pasted excerpt) becomes a persisted
// interview session; each external inference (lead finding, each interview
// turn) requires explicit authorization bound to the exact persisted state
// via an input hash (the ticket snapshot→authorize→generate→adopt-check
// pattern, with the GET view as the review snapshot). Candidate answers
// persist before any subsequent inference, failed or invalid inference
// preserves prior state for recovery, and a source-linked proposal can be
// saved as a Career Evidence draft for correction and explicit applicant
// review through the existing career_evidence owner.

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
	"unicode/utf8"

	discovery "github.com/jenksed/yourmove-career-discovery"
	coverprovider "github.com/jenksed/yourmove-go/internal/coverletter/provider"
	"github.com/jenksed/yourmove-go/internal/documents"
)

func writeCareerJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func careerDiscoveryError(w http.ResponseWriter, status int, code, message string) {
	writeCareerJSON(w, status, map[string]string{"code": code, "message": message})
}

func decodeCareerJSON(w http.ResponseWriter, r *http.Request, limit int64, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		careerDiscoveryError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return false
	}
	return true
}

func careerDiscoveryPathID(r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if len(id) != 64 {
		return "", false
	}
	for _, c := range id {
		if c < '0' || (c > '9' && c < 'a') || c > 'f' {
			return "", false
		}
	}
	return id, true
}

// serveCareerDiscovery prefers the React bundle when embedded for /career
// and /career/studio. Unlike the migrated library routes there is no
// Go-rendered equivalent, so a missing bundle is an honest 404 rather than
// a silent fallback.
func (a *App) serveCareerDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && len(a.resources.frontendIndex) > 0 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(a.resources.frontendIndex)
		return
	}
	http.NotFound(w, r)
}

// GET /api/career/discovery/providers describes the configured inference
// destinations an authorization may name. It renders no credentials and
// contacts no model.
func (a *App) careerDiscoveryProvidersAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ids := []string{"ollama", "openai", "anthropic", "openrouter", "compatible", "codex", "claude-cli"}
	options := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		configured := coverprovider.Configured(id)
		if id == "codex" || id == "claude-cli" {
			state, _ := coverprovider.SubscriptionReadiness(id)
			configured = state == "ready"
		}
		execution := answerAIExecution(id)
		if runner, err := coverprovider.Configure(id, coverprovider.DefaultModel(id)); err == nil {
			execution += " Transport: " + runner.ExecutionLocality() + "."
		}
		options = append(options, map[string]any{
			"id": id, "default_model": coverprovider.DefaultModel(id),
			"execution": execution, "configured": configured,
		})
	}
	writeCareerJSON(w, http.StatusOK, map[string]any{"providers": options})
}

// careerDiscoveryNextStep describes the pending external inference, if any.
// The input hash binds an authorization to the exact persisted state plus
// the pinned engine policy; any concurrent change invalidates it. Prompt
// and excerpt bytes never leave the server: callers expose only the opaque
// kind, policy name, input hash, and byte count.
func careerDiscoveryNextStep(session careerDiscoverySession) (kind, policyName, inputHash string, bytes int) {
	stateJSON, _ := json.Marshal(session.State)
	switch {
	case session.Status == careerDiscoveryPromoted:
		return "none", "", "", 0
	case len(session.State.Leads) == 0:
		policy := discovery.LeadsPolicy
		bytes = len(session.State.Resume.Text) + len(policy)
		inputHash = careerDiscoveryHash("ym003a/v1\x00leads\x00" + string(stateJSON) + "\x00" + policy)
		return "leads", "engine lead-finding policy", inputHash, bytes
	default:
		policy := discovery.Policy
		bytes = len(session.State.Resume.Text) + len(policy)
		for _, response := range session.State.Responses {
			bytes += len(response.Question) + len(response.Source.Text)
		}
		inputHash = careerDiscoveryHash("ym003a/v1\x00interview\x00" + string(stateJSON) + "\x00" + policy)
		return "interview", "engine interview policy (pinned)", inputHash, bytes
	}
}

func (a *App) careerDiscoveryView(session careerDiscoverySession, resumeName string) map[string]any {
	kind, policyName, inputHash, bytes := careerDiscoveryNextStep(session)
	// Normalize absent collections to [] so the JSON contract never emits
	// null for a list the client iterates.
	leads := session.State.Leads
	if leads == nil {
		leads = []discovery.Lead{}
	}
	responses := session.State.Responses
	if responses == nil {
		responses = []discovery.Response{}
	}
	var proposal any
	if session.State.Proposal != nil {
		normalized := *session.State.Proposal
		if normalized.Supports == nil {
			normalized.Supports = []discovery.ProposalSupport{}
		}
		if normalized.Unresolved == nil {
			normalized.Unresolved = []string{}
		}
		proposal = &normalized
	}
	var resume any
	if session.ResumeID > 0 {
		resume = map[string]any{"id": session.ResumeID, "name": resumeName, "removed": resumeName == "",
			"sha256": session.ResumeSHA256, "excerpt_hash": session.ResumeExcerptHash}
	}
	var failure any
	if session.FailureCode != "" {
		failure = map[string]string{"code": session.FailureCode, "detail": session.FailureDetail}
	}
	var promoted any
	if session.PromotedEvidence > 0 {
		promoted = session.PromotedEvidence
	}
	return map[string]any{
		"id": session.ID, "status": session.Status, "revision": session.Revision,
		"created_at": session.CreatedAt, "updated_at": session.UpdatedAt, "expires_at": session.ExpiresAt,
		"resume": resume, "excerpt_hash": session.ResumeExcerptHash,
		"excerpt_bytes": len(session.State.Resume.Text),
		"leads": leads, "selected_lead_id": session.State.SelectedLeadID,
		"question": session.State.Question, "responses": responses,
		"proposal": proposal,
		"next": map[string]any{"kind": kind, "input_hash": inputHash, "policy_name": policyName,
			"bytes": bytes},
		"provider": session.Provider, "model": session.Model, "failure": failure,
		"promoted_evidence_id": promoted,
	}
}

// GET /api/career/discovery/sessions lists resumable interviews newest
// first. It carries counts, not transcripts; open one session to inspect it.
func (a *App) careerDiscoveryListAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := a.careerDiscoverySweepExpired(time.Now()); err != nil {
		log.Printf("career discovery sweep: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to list interview sessions")
		return
	}
	rows, err := a.db.Query(`SELECT s.id,s.resume_id,s.state_json,s.status,s.provider,s.model,
		s.failure_code,s.promoted_evidence_id,s.revision,s.created_at,s.updated_at,s.expires_at,
		(SELECT name FROM resumes r WHERE r.id=s.resume_id)
		FROM career_discovery_sessions s ORDER BY s.updated_at DESC,s.id DESC LIMIT 50`)
	if err != nil {
		log.Printf("career discovery list: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to list interview sessions")
		return
	}
	defer rows.Close()
	sessions := []map[string]any{}
	for rows.Next() {
		var id, stateJSON, status, provider, model, failureCode string
		var resumeID sql.NullInt64
		var promoted sql.NullInt64
		var resumeName sql.NullString
		var revision int64
		var createdAt, updatedAt, expiresAt string
		if err := rows.Scan(&id, &resumeID, &stateJSON, &status, &provider, &model,
			&failureCode, &promoted, &revision, &createdAt, &updatedAt, &expiresAt, &resumeName); err != nil {
			log.Printf("career discovery list scan: %v", err)
			careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to list interview sessions")
			return
		}
		var state discovery.State
		if err := json.Unmarshal([]byte(stateJSON), &state); err != nil {
			log.Printf("career discovery list state: invalid stored state")
			continue
		}
		name := ""
		if resumeName.Valid {
			name = resumeName.String
		}
		resume := resumeID.Valid && resumeID.Int64 > 0
		var promotedID any
		if promoted.Valid {
			promotedID = promoted.Int64
		}
		sessions = append(sessions, map[string]any{
			"id": id, "status": status, "resume_id": resumeID.Int64, "resume_name": name,
			"resume_removed": resume && name == "", "leads_count": len(state.Leads),
			"selected_lead_id": state.SelectedLeadID, "responses_count": len(state.Responses),
			"question_pending": strings.TrimSpace(state.Question) != "", "has_proposal": state.Proposal != nil,
			"provider": provider, "model": model, "failure_code": failureCode,
			"promoted_evidence_id": promotedID, "revision": revision,
			"created_at": createdAt, "updated_at": updatedAt, "expires_at": expiresAt,
		})
	}
	if err := rows.Err(); err != nil {
		log.Printf("career discovery list rows: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to list interview sessions")
		return
	}
	writeCareerJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

// POST /api/career/discovery/sessions starts a persisted interview from an
// existing resume or a pasted excerpt. It performs no external inference;
// the first authorization runs lead finding.
func (a *App) careerDiscoveryCreateAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		ResumeID   int64  `json:"resume_id"`
		Excerpt    string `json:"excerpt"`
		ExcerptEnd int    `json:"excerpt_end"`
	}
	if !decodeCareerJSON(w, r, 16<<10, &payload) {
		return
	}
	var resumeID int64
	var resumeSHA string
	var text string
	switch {
	case payload.ResumeID > 0 && payload.Excerpt != "":
		careerDiscoveryError(w, http.StatusBadRequest, "invalid_request", "Provide a resume or an excerpt, not both")
		return
	case payload.ResumeID > 0:
		resume, err := a.resumeByID(payload.ResumeID)
		if errors.Is(err, sql.ErrNoRows) {
			careerDiscoveryError(w, http.StatusNotFound, "unknown_resume", "Unknown resume")
			return
		}
		if err != nil {
			log.Printf("career discovery resume: %v", err)
			careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to load the resume")
			return
		}
		if resume.ArchivedAt != "" {
			careerDiscoveryError(w, http.StatusConflict, "archived", "That resume version is archived. Restore it or use its current version.")
			return
		}
		data, err := a.readResumeBytes(resume)
		if err != nil {
			log.Printf("career discovery resume bytes: %v", err)
			careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to read the resume")
			return
		}
		if _, err := documents.ValidateResume(resume.Name, data); err != nil {
			careerDiscoveryError(w, http.StatusUnprocessableEntity, "invalid_resume", "That resume cannot be validated for text extraction")
			return
		}
		text, err = documents.ExtractText(resume.Kind, data)
		if err != nil || strings.TrimSpace(text) == "" {
			careerDiscoveryError(w, http.StatusUnprocessableEntity, "invalid_resume", "That resume has no extractable text; paste an excerpt instead")
			return
		}
		if payload.ExcerptEnd > 0 {
			end := payload.ExcerptEnd
			if end > len(text) || (end < len(text) && !utf8.RuneStart(text[end])) {
				careerDiscoveryError(w, http.StatusUnprocessableEntity, "excerpt", "The excerpt end splits a character or exceeds the text")
				return
			}
			if end < len(text) {
				const marker = "\n[Excerpt truncated]"
				cut := end
				if cut+len(marker) > 8000 {
					// Fit the explicit marker within the engine budget
					// rather than rejecting a full-size excerpt request.
					cut = 8000 - len(marker)
					for cut > 0 && !utf8.RuneStart(text[cut]) {
						cut--
					}
				}
				if cut < 1 {
					careerDiscoveryError(w, http.StatusUnprocessableEntity, "excerpt", "The excerpt end splits a character or exceeds the text")
					return
				}
				text = text[:cut] + marker
			}
		}
		if len(text) > 8000 {
			careerDiscoveryError(w, http.StatusUnprocessableEntity, "excerpt_required",
				"The resume text is longer than one interview excerpt. Paste the part to investigate, or limit it to the first 8000 bytes.")
			return
		}
		resumeID, resumeSHA = resume.ID, resume.SHA256
	case payload.Excerpt != "":
		if payload.ExcerptEnd != 0 {
			careerDiscoveryError(w, http.StatusBadRequest, "invalid_request", "An excerpt end applies only to a resume")
			return
		}
		text = payload.Excerpt
	default:
		careerDiscoveryError(w, http.StatusBadRequest, "invalid_request", "Provide a resume or an excerpt")
		return
	}
	state, err := discovery.New(text)
	if err != nil {
		careerDiscoveryError(w, http.StatusUnprocessableEntity, "invalid_input", "The excerpt must be non-empty text within 8000 bytes")
		return
	}
	if err := a.careerDiscoverySweepExpired(time.Now()); err != nil {
		log.Printf("career discovery sweep: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to start the interview")
		return
	}
	id, err := careerDiscoverySessionID()
	if err != nil {
		log.Printf("career discovery session id: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to start the interview")
		return
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to start the interview")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	expires := time.Now().Add(careerDiscoverySessionTTL).UTC().Format(time.RFC3339Nano)
	if _, err := a.db.Exec(`INSERT INTO career_discovery_sessions
		(id,resume_id,resume_sha256,resume_excerpt_hash,state_json,status,revision,created_at,updated_at,expires_at)
		VALUES(?,?,?,?,?,'leads',1,?,?,?)`,
		id, resumeID, resumeSHA, careerDiscoveryHash(text), string(stateJSON), now, now, expires); err != nil {
		log.Printf("career discovery create: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to start the interview")
		return
	}
	session, name, err := a.careerDiscoveryLoad(id)
	if err != nil {
		log.Printf("career discovery reload: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to start the interview")
		return
	}
	writeCareerJSON(w, http.StatusCreated, a.careerDiscoveryView(session, name))
}

// GET /api/career/discovery/sessions/{id} returns the redacted review
// snapshot: interview progress plus the opaque pending inference awaiting
// authorization. Prompt and excerpt bytes never leave the server; the
// applicant authorizes from kind, input hash, policy name, and byte count.
func (a *App) careerDiscoveryGetAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, ok := careerDiscoveryPathID(r)
	if !ok {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_session", "Unknown interview session")
		return
	}
	session, name, err := a.careerDiscoveryLoad(id)
	if errors.Is(err, sql.ErrNoRows) {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_session", "Unknown or expired interview session")
		return
	}
	if errors.Is(err, ErrCareerDiscoveryState) {
		careerDiscoveryError(w, http.StatusUnprocessableEntity, "invalid_state", "That interview session holds state the engine no longer accepts")
		return
	}
	if err != nil {
		log.Printf("career discovery load: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to load the interview session")
		return
	}
	writeCareerJSON(w, http.StatusOK, a.careerDiscoveryView(session, name))
}

// careerDiscoveryProviderMessage reuses the single-sourced provider setup
// guidance and names this flow's retry action: explicit re-authorization.
func careerDiscoveryProviderMessage(provider, model, notice string) string {
	return strings.ReplaceAll(answerAIProviderMessage(provider, model, notice), "review sources again", "authorize again")
}

// POST /api/career/discovery/sessions/{id}/authorize runs the pending
// inference after explicit authorization. The input hash must match the
// current persisted state; any concurrent change rejects the call and the
// applicant reviews the fresh snapshot before authorizing again.
func (a *App) careerDiscoveryAuthorizeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, ok := careerDiscoveryPathID(r)
	if !ok {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_session", "Unknown interview session")
		return
	}
	var payload struct {
		Provider             string `json:"provider"`
		Model                string `json:"model"`
		AuthorizeGeneration  string `json:"authorize_generation"`
		AuthorizeExternal    string `json:"authorize_external"`
		InputHash            string `json:"input_hash"`
	}
	if !decodeCareerJSON(w, r, 4<<10, &payload) {
		return
	}
	session, _, err := a.careerDiscoveryLoad(id)
	if errors.Is(err, sql.ErrNoRows) {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_session", "Unknown or expired interview session")
		return
	}
	if errors.Is(err, ErrCareerDiscoveryState) {
		careerDiscoveryError(w, http.StatusUnprocessableEntity, "invalid_state", "That interview session holds state the engine no longer accepts")
		return
	}
	if err != nil {
		log.Printf("career discovery load: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to load the interview session")
		return
	}
	if session.Status == careerDiscoveryPromoted {
		careerDiscoveryError(w, http.StatusConflict, "promoted", "That interview already promoted a draft and is closed")
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
	if a.careerDiscoveryGenerate == nil {
		runner, err = coverprovider.Configure(payload.Provider, payload.Model)
		if err != nil {
			careerDiscoveryError(w, http.StatusUnprocessableEntity, "provider_configuration",
				careerDiscoveryProviderMessage(payload.Provider, payload.Model, answerAIProviderNotice(payload.Provider, payload.Model, err)))
			return
		}
	}
	kind, _, expectedHash, _ := careerDiscoveryNextStep(session)
	if kind == "none" || subtle.ConstantTimeCompare([]byte(payload.InputHash), []byte(expectedHash)) != 1 {
		careerDiscoveryError(w, http.StatusConflict, "changed", "The interview changed. Review the current snapshot before authorizing again.")
		return
	}
	if !a.careerDiscovery.begin(id) {
		careerDiscoveryError(w, http.StatusConflict, "generation_in_progress", "Another inference is running for this interview")
		return
	}
	defer a.careerDiscovery.finish(id)
	generate := a.careerDiscoveryGenerate
	if generate == nil {
		generate = runner.GenerateNarrative
	}
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	next := session.State
	switch kind {
	case "leads":
		var leads []discovery.Lead
		leads, err = discovery.FindLeads(ctx, generate, session.State.Resume.Text)
		if err == nil {
			next.Leads = leads
		}
	case "interview":
		// The product path pins the engine interview policy explicitly.
		// Caller-supplied policies are never accepted.
		next, err = discovery.NextWithPolicy(ctx, generate, session.State, discovery.Policy)
	}
	if err != nil {
		a.careerDiscoveryRecordFailure(w, session, payload.Provider, payload.Model, ctx.Err(), err)
		return
	}
	updated := session
	updated.State = next
	if kind == "leads" {
		updated.Status = careerDiscoveryInterview
	}
	updated.Provider, updated.Model = payload.Provider, payload.Model
	updated.FailureCode, updated.FailureDetail = "", ""
	if err := a.careerDiscoveryStore(updated, session.Revision); err != nil {
		if errors.Is(err, ErrCareerDiscoveryStale) {
			careerDiscoveryError(w, http.StatusConflict, "changed", "The interview changed during inference. The result was discarded; review the current snapshot before authorizing again.")
			return
		}
		log.Printf("career discovery store: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to save the interview result")
		return
	}
	loaded, name, err := a.careerDiscoveryLoad(id)
	if err != nil {
		log.Printf("career discovery reload: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to reload the interview session")
		return
	}
	writeCareerJSON(w, http.StatusOK, a.careerDiscoveryView(loaded, name))
}

// careerDiscoveryRecordFailure preserves the pre-inference state and records
// a recoverable failure: the applicant authorizes the same step again after
// fixing the provider, with no silent substitution. It logs and persists
// categories only, never prompts or model output.
func (a *App) careerDiscoveryRecordFailure(w http.ResponseWriter, session careerDiscoverySession, provider, model string, ctxErr, err error) {
	code := ""
	status := http.StatusBadGateway
	message := ""
	if errors.Is(err, discovery.ErrInvalid) {
		code = "invalid_output"
		status = http.StatusUnprocessableEntity
		message = "The model returned unusable output. Nothing was recorded; authorize again for a new attempt."
	} else {
		notice := answerAIProviderNotice(provider, model, err)
		code = "provider_" + notice
		if errors.Is(ctxErr, context.DeadlineExceeded) || notice == "timeout" {
			code = "provider_timeout"
			status = http.StatusGatewayTimeout
		}
		message = careerDiscoveryProviderMessage(provider, model, notice)
	}
	log.Printf("career discovery inference: step=%s provider=%s status=%s", careerDiscoveryFailureStep(session), provider, code)
	updated := session
	updated.FailureCode = code
	updated.FailureDetail = "provider=" + provider + " model=" + model
	if err := a.careerDiscoveryStore(updated, session.Revision); err != nil {
		if errors.Is(err, ErrCareerDiscoveryStale) {
			careerDiscoveryError(w, http.StatusConflict, "changed", "The interview changed. Review the current snapshot before authorizing again.")
			return
		}
		log.Printf("career discovery failure store: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to save the interview result")
		return
	}
	careerDiscoveryError(w, status, code, message)
}

func careerDiscoveryFailureStep(session careerDiscoverySession) string {
	if len(session.State.Leads) == 0 {
		return "leads"
	}
	return "interview"
}

// POST /api/career/discovery/sessions/{id}/select records the applicant's
// chosen lead. Selection is pure engine logic and needs no inference.
func (a *App) careerDiscoverySelectAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, ok := careerDiscoveryPathID(r)
	if !ok {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_session", "Unknown interview session")
		return
	}
	var payload struct {
		LeadID   string `json:"lead_id"`
		Revision int64  `json:"revision"`
	}
	if !decodeCareerJSON(w, r, 2<<10, &payload) {
		return
	}
	session, _, err := a.careerDiscoveryLoad(id)
	if errors.Is(err, sql.ErrNoRows) {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_session", "Unknown or expired interview session")
		return
	}
	if errors.Is(err, ErrCareerDiscoveryState) {
		careerDiscoveryError(w, http.StatusUnprocessableEntity, "invalid_state", "That interview session holds state the engine no longer accepts")
		return
	}
	if err != nil {
		log.Printf("career discovery load: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to load the interview session")
		return
	}
	if session.Status == careerDiscoveryPromoted {
		careerDiscoveryError(w, http.StatusConflict, "promoted", "That interview already promoted a draft and is closed")
		return
	}
	if payload.Revision != session.Revision {
		careerDiscoveryError(w, http.StatusConflict, "changed", "The interview changed. Reload before selecting.")
		return
	}
	next, err := discovery.SelectLead(session.State, payload.LeadID)
	if err != nil {
		careerDiscoveryError(w, http.StatusUnprocessableEntity, "invalid_lead", "Choose one of the listed leads before answering")
		return
	}
	updated := session
	updated.State = next
	updated.FailureCode, updated.FailureDetail = "", ""
	if err := a.careerDiscoveryStore(updated, session.Revision); err != nil {
		if errors.Is(err, ErrCareerDiscoveryStale) {
			careerDiscoveryError(w, http.StatusConflict, "changed", "The interview changed. Reload before selecting.")
			return
		}
		log.Printf("career discovery store: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to save the selection")
		return
	}
	loaded, name, err := a.careerDiscoveryLoad(id)
	if err != nil {
		log.Printf("career discovery reload: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to reload the interview session")
		return
	}
	writeCareerJSON(w, http.StatusOK, a.careerDiscoveryView(loaded, name))
}

// POST /api/career/discovery/sessions/{id}/answer persists the applicant's
// response before any subsequent inference, so a failed request can never
// lose the candidate's statement.
func (a *App) careerDiscoveryAnswerAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, ok := careerDiscoveryPathID(r)
	if !ok {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_session", "Unknown interview session")
		return
	}
	var payload struct {
		Answer   string `json:"answer"`
		Revision int64  `json:"revision"`
	}
	if !decodeCareerJSON(w, r, 8<<10, &payload) {
		return
	}
	session, _, err := a.careerDiscoveryLoad(id)
	if errors.Is(err, sql.ErrNoRows) {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_session", "Unknown or expired interview session")
		return
	}
	if errors.Is(err, ErrCareerDiscoveryState) {
		careerDiscoveryError(w, http.StatusUnprocessableEntity, "invalid_state", "That interview session holds state the engine no longer accepts")
		return
	}
	if err != nil {
		log.Printf("career discovery load: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to load the interview session")
		return
	}
	if session.Status == careerDiscoveryPromoted {
		careerDiscoveryError(w, http.StatusConflict, "promoted", "That interview already promoted a draft and is closed")
		return
	}
	if payload.Revision != session.Revision {
		careerDiscoveryError(w, http.StatusConflict, "changed", "The interview changed. Reload before answering.")
		return
	}
	next, err := discovery.RecordAnswer(session.State, payload.Answer)
	if err != nil {
		careerDiscoveryError(w, http.StatusUnprocessableEntity, "invalid_answer", "Answer the pending question with non-empty text within 2000 bytes")
		return
	}
	updated := session
	updated.State = next
	updated.FailureCode, updated.FailureDetail = "", ""
	if err := a.careerDiscoveryStore(updated, session.Revision); err != nil {
		if errors.Is(err, ErrCareerDiscoveryStale) {
			careerDiscoveryError(w, http.StatusConflict, "changed", "The interview changed. Reload before answering.")
			return
		}
		log.Printf("career discovery store: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to save the answer")
		return
	}
	loaded, name, err := a.careerDiscoveryLoad(id)
	if err != nil {
		log.Printf("career discovery reload: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to reload the interview session")
		return
	}
	writeCareerJSON(w, http.StatusOK, a.careerDiscoveryView(loaded, name))
}

// careerDiscoveryDraftNotes fits proposal citations and unresolved items
// into the evidence notes budget at line granularity: quotes stay verbatim
// or are omitted behind an explicit marker naming the interview session,
// which keeps the full transcript.
func careerDiscoveryDraftNotes(sessionID string, proposal *discovery.Proposal, responses int) string {
	short := sessionID
	if len(short) > 8 {
		short = short[:8]
	}
	header := "Career discovery interview " + short + " (" + strconv.Itoa(responses) + " recorded response(s)). Model-suggested draft; verify every claim before review."
	lines := []string{"Cited responses:"}
	for _, support := range proposal.Supports {
		lines = append(lines, "- ["+support.SourceID+"] \""+support.Quote+"\"")
	}
	if len(proposal.Supports) == 0 {
		lines = append(lines, "- ["+proposal.SourceID+"] \""+proposal.Quote+"\"")
	}
	lines = append(lines, "Unresolved:")
	for _, item := range proposal.Unresolved {
		lines = append(lines, "- "+item)
	}
	lines = append(lines, "Full transcript remains in the interview session.")
	notes := header + "\n" + strings.Join(lines, "\n")
	if utf8.RuneCountInString(notes) <= careerEvidenceSourceNotesRunes {
		return notes
	}
	marker := "[further citations and unresolved items kept in the interview session]"
	kept := []string{lines[0]}
	dropped := 0
	for _, line := range lines[1:] {
		candidate := header + "\n" + strings.Join(append(append([]string{}, kept...), line, marker), "\n")
		if utf8.RuneCountInString(candidate) > careerEvidenceSourceNotesRunes {
			dropped++
			continue
		}
		kept = append(kept, line)
	}
	return header + "\n" + strings.Join(append(kept, marker), "\n")
}

// POST /api/career/discovery/sessions/{id}/promote saves the current
// proposal as a Career Evidence draft through the existing owner. Model
// output never approves evidence: the draft stays a draft until the
// applicant corrects it and completes explicit review.
func (a *App) careerDiscoveryPromoteAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, ok := careerDiscoveryPathID(r)
	if !ok {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_session", "Unknown interview session")
		return
	}
	// Serialize promotion with inference per session: the load, revision
	// check, and draft link below all run under the guard, so a second
	// concurrent promoter gets 409 instead of creating an orphan draft.
	if !a.careerDiscovery.begin(id) {
		careerDiscoveryError(w, http.StatusConflict, "promotion_in_progress", "Another operation is running for this interview. Reload and retry.")
		return
	}
	defer a.careerDiscovery.finish(id)
	var payload struct {
		Revision int64 `json:"revision"`
	}
	if !decodeCareerJSON(w, r, 2<<10, &payload) {
		return
	}
	session, _, err := a.careerDiscoveryLoad(id)
	if errors.Is(err, sql.ErrNoRows) {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_session", "Unknown or expired interview session")
		return
	}
	if errors.Is(err, ErrCareerDiscoveryState) {
		careerDiscoveryError(w, http.StatusUnprocessableEntity, "invalid_state", "That interview session holds state the engine no longer accepts")
		return
	}
	if err != nil {
		log.Printf("career discovery load: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to load the interview session")
		return
	}
	if session.Status == careerDiscoveryPromoted {
		careerDiscoveryError(w, http.StatusConflict, "promoted", "That interview already promoted a draft and is closed")
		return
	}
	if payload.Revision != session.Revision {
		careerDiscoveryError(w, http.StatusConflict, "changed", "The interview changed. Reload before saving a draft.")
		return
	}
	proposal := session.State.Proposal
	if proposal == nil {
		careerDiscoveryError(w, http.StatusConflict, "no_proposal", "The interview has no proposal yet. Continue answering first.")
		return
	}
	// The load already rebound the proposal to the recorded responses.
	content := careerEvidenceContent{
		Title: proposal.Title, Situation: proposal.Experience, Actions: proposal.Claim,
		OutcomeState: "not_recorded",
		SourceNotes:  careerDiscoveryDraftNotes(session.ID, proposal, len(session.State.Responses)),
	}
	if !content.valid() {
		log.Printf("career discovery draft: proposal exceeds evidence bounds")
		careerDiscoveryError(w, http.StatusInternalServerError, "draft", "The proposal does not fit a Career Evidence draft")
		return
	}
	evidenceID, err := a.createCareerEvidenceDraft()
	if err != nil {
		log.Printf("career discovery draft: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to create the Career Evidence draft")
		return
	}
	if err := a.updateCareerEvidence(evidenceID, 1, content, false); err != nil {
		log.Printf("career discovery draft save: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to save the Career Evidence draft")
		return
	}
	evidence, err := careerEvidenceByID(a.db, evidenceID)
	if err != nil {
		log.Printf("career discovery draft reload: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to reload the Career Evidence draft")
		return
	}
	updated := session
	updated.Status = careerDiscoveryPromoted
	updated.PromotedEvidence = evidenceID
	updated.FailureCode, updated.FailureDetail = "", ""
	if err := a.careerDiscoveryStore(updated, session.Revision); err != nil {
		// The draft is saved; only the session link raced. Name the draft
		// so the applicant is never stranded by a concurrent edit.
		path := "/career-evidence/" + evidencePathID(evidenceID)
		if errors.Is(err, ErrCareerDiscoveryStale) {
			writeCareerJSON(w, http.StatusConflict, map[string]any{"code": "changed",
				"message":     "The draft was saved but the interview changed. Open the draft directly.",
				"evidence_id": evidenceID, "path": path})
			return
		}
		log.Printf("career discovery promote store: %v", err)
		writeCareerJSON(w, http.StatusInternalServerError, map[string]any{"code": "storage",
			"message": "The draft was saved but the interview link failed. Open the draft directly.",
			"evidence_id": evidenceID, "path": path})
		return
	}
	writeCareerJSON(w, http.StatusOK, map[string]any{
		"evidence_id": evidenceID, "evidence_revision": evidence.Revision,
		"path": "/career-evidence/" + evidencePathID(evidenceID),
		"notice": "Draft saved. Correct it and complete your explicit review; nothing is approved yet.",
	})
}

func evidencePathID(id int64) string {
	return strconv.FormatInt(id, 10)
}
