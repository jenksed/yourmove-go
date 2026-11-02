package app

// Role comparison for the YM-003B React Studio slice.
//
// A role profile is derived deterministically from a real saved Opportunity
// (title, company, description sentences) — never from synthetic demo jobs
// and never model-written. The applicant selects one to four reviewed Career
// Evidence records, reviews the redacted snapshot (names, byte counts,
// hashes, policy name), and authorizes one bounded AnalyzeRole inference
// with the same dual confirmation and input-hash binding as discovery. One
// saved analysis per role key persists; source corrections and posting edits
// surface as visible staleness, never silent rewrites.

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	discovery "github.com/jenksed/yourmove-career-discovery"
	coverprovider "github.com/jenksed/yourmove-go/internal/coverletter/provider"
)

const careerRolePolicyName = "engine role policy (pinned)"
const careerSynthesisPolicyName = "engine synthesis policy (pinned)"

// truncateBytesRune keeps s within max bytes without splitting a rune, so
// host-derived role text always satisfies the engine byte budgets.
func truncateBytesRune(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return strings.TrimSpace(s[:cut])
}

// splitRoleRequirements derives candidate requirements from posting prose.
// It splits bullet lines and sentences deterministically, drops empties and
// duplicates, and caps the list at the engine budget. The applicant reviews
// the derived list before authorizing; the model echoes unsupported entries
// verbatim from exactly this list.
func splitRoleRequirements(description string) []string {
	var out []string
	seen := map[string]bool{}
	emit := func(part string) {
		part = strings.TrimSpace(strings.Trim(part, "-*• \t"))
		part = truncateBytesRune(part, 200)
		if len(part) < 3 || seen[part] {
			return
		}
		seen[part] = true
		out = append(out, part)
	}
	for _, line := range strings.Split(description, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		start := 0
		for i := 0; i < len(line) && len(out) < discovery.MaxRoleRequirements; i++ {
			if line[i] != '.' && line[i] != '!' && line[i] != '?' && line[i] != ';' {
				continue
			}
			if i+1 < len(line) && line[i+1] != ' ' && line[i+1] != '\t' {
				continue
			}
			emit(line[start : i+1])
			start = i + 1
		}
		if len(out) < discovery.MaxRoleRequirements {
			emit(line[start:])
		}
		if len(out) >= discovery.MaxRoleRequirements {
			break
		}
	}
	return out
}

// deriveRoleProfile builds the host-authored comparison context for one
// saved Opportunity. It returns a non-empty reason when the posting has no
// usable comparison context; the applicant can add a description through the
// existing posting edit path.
func deriveRoleProfile(job Job) (discovery.RoleProfile, string) {
	title := strings.TrimSpace(job.Title)
	if title == "" {
		title = strings.TrimSpace(job.Company)
		if title != "" {
			title += " opportunity"
		}
	}
	if title == "" {
		title = strings.TrimSpace(job.Host)
	}
	if title == "" {
		title = strings.TrimSpace(job.URL)
	}
	requirements := splitRoleRequirements(job.Description)
	if title == "" || len(requirements) == 0 {
		return discovery.RoleProfile{}, "Add a title and description to this posting before comparing it against your experience."
	}
	focus := ""
	if company := strings.TrimSpace(job.Company); company != "" {
		focus = "Employer: " + company + ". "
	} else if host := strings.TrimSpace(job.Host); host != "" {
		focus = "Source: " + host + ". "
	}
	focus += strings.TrimSpace(job.Description)
	profile := discovery.RoleProfile{
		Key:          "job-" + strconv.FormatInt(job.ID, 10),
		Title:        truncateBytesRune(title, 160),
		Focus:        truncateBytesRune(focus, 600),
		Requirements: requirements,
	}
	if strings.TrimSpace(profile.Title) == "" || strings.TrimSpace(profile.Focus) == "" {
		return discovery.RoleProfile{}, "Add a title and description to this posting before comparing it against your experience."
	}
	return profile, ""
}

func careerRoleHash(profile discovery.RoleProfile) string {
	raw, _ := json.Marshal(profile)
	return careerDiscoveryHash("ym003b/v1\x00role-profile\x00" + string(raw))
}

// careerReviewedEvidence returns applicant-reviewed records with complete
// comparison text, oldest selections first by ID for stable input hashes.
// Drafts, incomplete records, and reopened records never qualify.
func careerReviewedEvidence(a *App) ([]CareerEvidence, error) {
	entries, err := a.careerEvidenceEntries()
	if err != nil {
		return nil, err
	}
	var out []CareerEvidence
	for _, entry := range entries {
		if entry.Status != "reviewed" {
			continue
		}
		if strings.TrimSpace(entry.Title) == "" || strings.TrimSpace(entry.Situation) == "" || strings.TrimSpace(entry.Actions) == "" {
			continue
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func projectRoleEvidence(entry CareerEvidence) discovery.RoleEvidence {
	return discovery.RoleEvidence{ID: entry.ID, Revision: entry.Revision, Title: entry.Title,
		Situation: entry.Situation, Actions: entry.Actions, OutcomeState: entry.OutcomeState}
}

func projectConnectionEvidence(entry CareerEvidence) discovery.ConnectionEvidence {
	// SourceTexts stay empty: no interview transcript is linked to these
	// records, and quotations resolve against situation and actions.
	return discovery.ConnectionEvidence{ID: entry.ID, Revision: entry.Revision, Title: entry.Title,
		Situation: entry.Situation, Actions: entry.Actions, OutcomeState: entry.OutcomeState}
}

// careerEvidenceRef is the redacted inference reference: names, byte counts,
// and hashes only. Full text stays inspectable through the evidence API.
func careerEvidenceRef(entry CareerEvidence) map[string]any {
	projection, _ := json.Marshal(projectRoleEvidence(entry))
	return map[string]any{
		"id": entry.ID, "revision": entry.Revision, "title": entry.Title,
		"bytes":        len(entry.Title) + len(entry.Situation) + len(entry.Actions),
		"content_hash": careerDiscoveryHash(string(projection)),
	}
}

// careerSelectionHash binds a saved interpretation to the exact evidence
// revisions it read. Missing or unreviewed records hash distinctly so their
// later absence always mismatches instead of silently matching.
func careerSelectionHash(entries []CareerEvidence) string {
	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		parts = append(parts, strconv.FormatInt(entry.ID, 10)+":"+strconv.FormatInt(entry.Revision, 10))
	}
	sort.Strings(parts)
	return careerDiscoveryHash("ym003b/v1\x00selection\x00" + strings.Join(parts, "|"))
}

func careerRoleInputHash(profile discovery.RoleProfile, inputs []discovery.RoleEvidence) string {
	profileJSON, _ := json.Marshal(profile)
	inputsJSON, _ := json.Marshal(inputs)
	return careerDiscoveryHash("ym003b/v1\x00role\x00" + string(profileJSON) + "\x00" + string(inputsJSON) + "\x00" + discovery.RolePolicy)
}

func careerRoleTransmitBytes(profile discovery.RoleProfile, inputs []discovery.RoleEvidence) int {
	profileJSON, _ := json.Marshal(profile)
	bytes := len(profileJSON) + len(discovery.RolePolicy)
	for _, input := range inputs {
		bytes += len(input.Title) + len(input.Situation) + len(input.Actions)
	}
	return bytes
}

// careerStudioFailure maps an inference failure to a recoverable API error.
// It logs and persists categories only, never prompts or model output.
func careerStudioFailure(step, provider, model string, ctxErr, err error) (code string, status int, message string) {
	if errors.Is(err, discovery.ErrInvalid) {
		return "invalid_output", http.StatusUnprocessableEntity,
			"The model returned unusable output. Nothing was recorded; authorize again for a new attempt."
	}
	notice := answerAIProviderNotice(provider, model, err)
	code = "provider_" + notice
	status = http.StatusBadGateway
	if errors.Is(ctxErr, context.DeadlineExceeded) || notice == "timeout" {
		code = "provider_timeout"
		status = http.StatusGatewayTimeout
	}
	log.Printf("career studio inference: step=%s provider=%s status=%s", step, provider, code)
	return code, status, careerDiscoveryProviderMessage(provider, model, notice)
}

type savedRoleAnalysis struct {
	RoleKey      string
	JobID        int64
	RoleHash     string
	EvidenceHash string
	Analysis     *discovery.RoleAnalysis
	Notice       string
	Provider     string
	Model        string
	UpdatedAt    string
}

func loadSavedRoleAnalysis(db *sql.DB, roleKey string) (savedRoleAnalysis, error) {
	var saved savedRoleAnalysis
	var raw string
	err := db.QueryRow(`SELECT role_key,job_id,role_hash,evidence_hash,analysis,notice,provider,model,updated_at
		FROM career_role_analyses WHERE role_key=?`, roleKey).
		Scan(&saved.RoleKey, &saved.JobID, &saved.RoleHash, &saved.EvidenceHash, &raw, &saved.Notice, &saved.Provider, &saved.Model, &saved.UpdatedAt)
	if err != nil {
		return saved, err
	}
	if raw != "" {
		var analysis discovery.RoleAnalysis
		if json.Unmarshal([]byte(raw), &analysis) != nil || analysis.RoleKey != roleKey {
			return saved, sql.ErrNoRows
		}
		saved.Analysis = &analysis
	}
	return saved, nil
}

// careerRoleSavedView renders a saved analysis with staleness recomputed
// from the current posting and evidence revisions. Stale interpretations
// stay visible behind their banner; nothing refreshes without a new
// explicit authorization.
func (a *App) careerRoleSavedView(job *Job, profile discovery.RoleProfile, usable bool, saved savedRoleAnalysis) map[string]any {
	view := map[string]any{
		"role_hash": saved.RoleHash, "evidence_hash": saved.EvidenceHash,
		"notice": saved.Notice, "provider": saved.Provider, "model": saved.Model,
		"updated_at": saved.UpdatedAt, "has_analysis": saved.Analysis != nil,
		"stale": false, "stale_evidence": false, "stale_role": false,
		"analysis": nil, "missing_job": job == nil,
	}
	if saved.Analysis == nil {
		return view
	}
	view["analysis"] = saved.Analysis
	if job == nil || !usable {
		view["stale"], view["stale_role"] = true, true
	}
	if usable && careerRoleHash(profile) != saved.RoleHash {
		view["stale"], view["stale_role"] = true, true
	}
	currentByID := map[int64]CareerEvidence{}
	for _, item := range saved.Analysis.Items {
		entry, err := careerEvidenceByID(a.db, item.EvidenceID)
		if err == nil {
			currentByID[item.EvidenceID] = entry
		}
	}
	staleItems := []int64{}
	for _, item := range saved.Analysis.Items {
		entry, ok := currentByID[item.EvidenceID]
		if !ok || entry.Status != "reviewed" || entry.Revision != item.EvidenceRevision {
			view["stale"], view["stale_evidence"] = true, true
			staleItems = append(staleItems, item.EvidenceID)
		}
	}
	view["stale_evidence_ids"] = staleItems
	return view
}

// GET /api/career/roles lists every saved Opportunity with its derived role
// context (or the honest reason it cannot be compared), every reviewed
// record available for selection, and each saved analysis with staleness.
func (a *App) careerRolesAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobs, err := a.allJobs()
	if err != nil {
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to load Opportunities")
		return
	}
	reviewed, err := careerReviewedEvidence(a)
	if err != nil {
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to load reviewed evidence")
		return
	}
	roles := make([]map[string]any, 0, len(jobs))
	for _, job := range jobs {
		profile, reason := deriveRoleProfile(job)
		usable := reason == ""
		entry := map[string]any{
			"job_id": job.ID, "key": "job-" + strconv.FormatInt(job.ID, 10),
			"title": job.Title, "company": job.Company,
			"usable": usable, "unusable_reason": reason,
			"requirements": []string{}, "requirements_count": 0,
			"saved": nil,
		}
		if usable {
			entry["focus"] = profile.Focus
			entry["requirements"] = profile.Requirements
			entry["requirements_count"] = len(profile.Requirements)
			entry["role_hash"] = careerRoleHash(profile)
		}
		saved, err := loadSavedRoleAnalysis(a.db, "job-"+strconv.FormatInt(job.ID, 10))
		if err == nil {
			jobCopy := job
			entry["saved"] = a.careerRoleSavedView(&jobCopy, profile, usable, saved)
		} else if !errors.Is(err, sql.ErrNoRows) {
			careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to load saved role analyses")
			return
		}
		roles = append(roles, entry)
	}
	refs := make([]map[string]any, 0, len(reviewed))
	for _, entry := range reviewed {
		refs = append(refs, careerEvidenceRef(entry))
	}
	writeCareerJSON(w, http.StatusOK, map[string]any{
		"roles": roles, "reviewed": refs, "policy_name": careerRolePolicyName,
		"max_evidences": discovery.MaxRoleEvidences,
	})
}

func parseRoleKey(r *http.Request) (int64, string, bool) {
	key := r.PathValue("key")
	id, err := strconv.ParseInt(strings.TrimPrefix(key, "job-"), 10, 64)
	if err != nil || id < 1 || key != "job-"+strconv.FormatInt(id, 10) {
		return 0, "", false
	}
	return id, key, true
}

func parseEvidenceIDSet(values string) ([]int64, bool) {
	seen := map[int64]bool{}
	var ids []int64
	for _, part := range strings.Split(values, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id < 1 || seen[id] {
			return nil, false
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) < 1 || len(ids) > discovery.MaxRoleEvidences {
		return nil, false
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, true
}

// careerRoleSelection loads the applicant-selected records. Every ID must
// resolve to a reviewed, complete record; drafts and reopened records are
// refused by name so the applicant knows exactly what to review first.
func (a *App) careerRoleSelection(ids []int64) ([]CareerEvidence, []discovery.RoleEvidence, string, bool) {
	var entries []CareerEvidence
	var inputs []discovery.RoleEvidence
	var refused []string
	for _, id := range ids {
		entry, err := careerEvidenceByID(a.db, id)
		if err != nil {
			return nil, nil, "", false
		}
		if entry.Status != "reviewed" || strings.TrimSpace(entry.Title) == "" ||
			strings.TrimSpace(entry.Situation) == "" || strings.TrimSpace(entry.Actions) == "" {
			refused = append(refused, strconv.FormatInt(id, 10))
			continue
		}
		entries = append(entries, entry)
		inputs = append(inputs, projectRoleEvidence(entry))
	}
	if len(refused) > 0 {
		return nil, nil, "Review Career Evidence record(s) " + strings.Join(refused, ", ") + " with complete title, situation, and actions before comparing.", true
	}
	return entries, inputs, "", true
}

// GET /api/career/roles/{key}?evidence_ids=1,2 returns the redacted review
// snapshot binding one authorization: derived profile, selected evidence
// references, byte counts, and the input hash. No inference runs here.
func (a *App) careerRoleSnapshotAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, key, ok := parseRoleKey(r)
	if !ok {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_role", "Unknown role")
		return
	}
	job, err := a.jobByID(jobID)
	if errors.Is(err, sql.ErrNoRows) {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_role", "That Opportunity no longer exists")
		return
	}
	if err != nil {
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to load the Opportunity")
		return
	}
	profile, reason := deriveRoleProfile(job)
	if reason != "" {
		careerDiscoveryError(w, http.StatusUnprocessableEntity, "unusable_role", reason)
		return
	}
	ids, ok := parseEvidenceIDSet(r.URL.Query().Get("evidence_ids"))
	if !ok {
		careerDiscoveryError(w, http.StatusBadRequest, "invalid_selection",
			"Select one to "+strconv.Itoa(discovery.MaxRoleEvidences)+" reviewed Career Evidence records to compare.")
		return
	}
	entries, inputs, refusal, ok := a.careerRoleSelection(ids)
	if !ok {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_evidence", "One selected Career Evidence record no longer exists")
		return
	}
	if refusal != "" {
		careerDiscoveryError(w, http.StatusUnprocessableEntity, "not_reviewed", refusal)
		return
	}
	refs := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		refs = append(refs, careerEvidenceRef(entry))
	}
	view := map[string]any{
		"job_id": job.ID, "key": key, "title": job.Title, "company": job.Company,
		"profile": profile, "role_hash": careerRoleHash(profile),
		"evidences": refs, "policy_name": careerRolePolicyName,
		"input_hash": careerRoleInputHash(profile, inputs),
		"bytes":      careerRoleTransmitBytes(profile, inputs),
		"saved":      nil,
	}
	if saved, err := loadSavedRoleAnalysis(a.db, key); err == nil {
		view["saved"] = a.careerRoleSavedView(&job, profile, true, saved)
	} else if !errors.Is(err, sql.ErrNoRows) {
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to load the saved analysis")
		return
	}
	writeCareerJSON(w, http.StatusOK, view)
}

// POST /api/career/roles/{key}/authorize runs one role comparison after
// explicit dual authorization. The input hash must match the exact derived
// profile plus selected evidence revisions; any concurrent edit rejects the
// call and the applicant reviews a fresh snapshot before authorizing again.
func (a *App) careerRoleAuthorizeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, key, ok := parseRoleKey(r)
	if !ok {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_role", "Unknown role")
		return
	}
	var payload struct {
		Provider            string  `json:"provider"`
		Model               string  `json:"model"`
		EvidenceIDs         []int64 `json:"evidence_ids"`
		InputHash           string  `json:"input_hash"`
		AuthorizeGeneration string  `json:"authorize_generation"`
		AuthorizeExternal   string  `json:"authorize_external"`
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
	job, err := a.jobByID(jobID)
	if errors.Is(err, sql.ErrNoRows) {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_role", "That Opportunity no longer exists")
		return
	}
	if err != nil {
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to load the Opportunity")
		return
	}
	profile, reason := deriveRoleProfile(job)
	if reason != "" {
		careerDiscoveryError(w, http.StatusUnprocessableEntity, "unusable_role", reason)
		return
	}
	ids := append([]int64(nil), payload.EvidenceIDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) < 1 || len(ids) > discovery.MaxRoleEvidences {
		careerDiscoveryError(w, http.StatusBadRequest, "invalid_selection",
			"Select one to "+strconv.Itoa(discovery.MaxRoleEvidences)+" reviewed Career Evidence records to compare.")
		return
	}
	for i := 1; i < len(ids); i++ {
		if ids[i] == ids[i-1] {
			careerDiscoveryError(w, http.StatusBadRequest, "invalid_selection", "Select each Career Evidence record only once.")
			return
		}
	}
	entries, inputs, refusal, ok := a.careerRoleSelection(ids)
	if !ok {
		careerDiscoveryError(w, http.StatusNotFound, "unknown_evidence", "One selected Career Evidence record no longer exists")
		return
	}
	if refusal != "" {
		careerDiscoveryError(w, http.StatusUnprocessableEntity, "not_reviewed", refusal)
		return
	}
	if subtle.ConstantTimeCompare([]byte(payload.InputHash), []byte(careerRoleInputHash(profile, inputs))) != 1 {
		careerDiscoveryError(w, http.StatusConflict, "changed", "The role or evidence changed. Review the current snapshot before authorizing again.")
		return
	}
	if !a.careerStudio.begin("role:" + key) {
		careerDiscoveryError(w, http.StatusConflict, "generation_in_progress", "Another inference is running for this role")
		return
	}
	defer a.careerStudio.finish("role:" + key)
	generate := a.careerStudioGenerate
	if generate == nil {
		generate = runner.GenerateDiscovery
	}
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	analysis, err := discovery.AnalyzeRole(ctx, generate, profile, inputs)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err != nil {
		code, status, message := careerStudioFailure("role", payload.Provider, payload.Model, ctx.Err(), err)
		notice := "Muse could not complete this comparison. No analysis was saved."
		if code == "invalid_output" {
			notice = "Muse output did not pass evidence checks. Nothing was saved; retry the comparison."
		}
		if dbErr := a.storeRoleFailure(key, jobID, careerRoleHash(profile), careerSelectionHash(entries), notice, payload.Provider, payload.Model, now); dbErr != nil {
			log.Printf("career role failure store: %v", dbErr)
		}
		careerDiscoveryError(w, status, code, message)
		return
	}
	raw, _ := json.Marshal(analysis)
	notice := "Muse compared " + strconv.Itoa(len(entries)) + " reviewed experience(s) against this role. Explanations need your judgment; nothing was adopted."
	if _, err := a.db.Exec(`INSERT INTO career_role_analyses(role_key,job_id,role_hash,evidence_hash,analysis,notice,provider,model,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(role_key) DO UPDATE SET job_id=excluded.job_id,role_hash=excluded.role_hash,
		evidence_hash=excluded.evidence_hash,analysis=excluded.analysis,notice=excluded.notice,
		provider=excluded.provider,model=excluded.model,updated_at=excluded.updated_at`,
		key, jobID, careerRoleHash(profile), careerSelectionHash(entries), string(raw), notice, payload.Provider, payload.Model, now, now); err != nil {
		log.Printf("career role store: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to save the role analysis")
		return
	}
	saved, err := loadSavedRoleAnalysis(a.db, key)
	if err != nil {
		log.Printf("career role reload: %v", err)
		careerDiscoveryError(w, http.StatusInternalServerError, "storage", "Unable to reload the role analysis")
		return
	}
	writeCareerJSON(w, http.StatusOK, map[string]any{
		"job_id": job.ID, "key": key, "saved": a.careerRoleSavedView(&job, profile, true, saved),
	})
}

// storeRoleFailure surfaces a failed comparison without overwriting a prior
// good analysis: the notice updates, the saved interpretation stands.
func (a *App) storeRoleFailure(roleKey string, jobID int64, roleHash, evidenceHash, notice, provider, model, now string) error {
	result, err := a.db.Exec(`UPDATE career_role_analyses SET notice=?,provider=?,model=?,updated_at=? WHERE role_key=?`,
		notice, provider, model, now, roleKey)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil {
		return err
	} else if count == 1 {
		return nil
	}
	_, err = a.db.Exec(`INSERT INTO career_role_analyses(role_key,job_id,role_hash,evidence_hash,analysis,notice,provider,model,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(role_key) DO NOTHING`,
		roleKey, jobID, roleHash, evidenceHash, "", notice, provider, model, now, now)
	return err
}
