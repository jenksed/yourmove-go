package app

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	discovery "github.com/jenksed/yourmove-career-discovery"
	"github.com/jenksed/yourmove-go/internal/storage"
)

func studioEvidence(t *testing.T, a *App, title, situation, actions string, review bool) CareerEvidence {
	t.Helper()
	id, err := a.createCareerEvidenceDraft()
	if err != nil {
		t.Fatal(err)
	}
	content := careerEvidenceContent{
		Title: title, Situation: situation, Actions: actions,
		OutcomeState: "unknown", Technologies: "Linux and MySQL",
	}
	if err := a.updateCareerEvidence(id, 1, content, review); err != nil {
		t.Fatal(err)
	}
	evidence, err := careerEvidenceByID(a.db, id)
	if err != nil {
		t.Fatal(err)
	}
	return evidence
}

func studioJob(t *testing.T, a *App, url, title, company, description string) int64 {
	t.Helper()
	result, err := insertJob(a.db, Job{URL: url, Host: "example.test", Title: title, Company: company,
		Description: description, SourceStatus: "synthetic", SourceNote: "Synthetic studio fixture."})
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func studioContentPayload(revision int64, title, situation, actions string) map[string]any {
	return map[string]any{
		"revision": revision, "title": title, "situation": situation, "actions": actions,
		"employer_context": "", "role_context": "", "technologies": "Linux",
		"outcome": "", "outcome_state": "unknown", "period": "", "source_notes": "",
	}
}

// studioRoleStub answers AnalyzeRole deterministically: the first selected
// record is relevant, the rest are not, and the last requirement is unmet.
func studioRoleStub(t *testing.T) func(context.Context, string, string) (string, error) {
	t.Helper()
	return func(_ context.Context, payload, policy string) (string, error) {
		if policy != discovery.RolePolicy {
			t.Errorf("role stub received unexpected policy")
		}
		var decoded struct {
			Role      discovery.RoleProfile    `json:"role"`
			Evidences []discovery.RoleEvidence `json:"evidences"`
		}
		if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
			t.Fatal(err)
		}
		items := make([]discovery.EvidenceRelevance, 0, len(decoded.Evidences))
		for i, evidence := range decoded.Evidences {
			item := discovery.EvidenceRelevance{
				EvidenceID: evidence.ID, EvidenceRevision: evidence.Revision,
				Explanation: "Stub reading for the studio fixture.",
			}
			if i == 0 {
				item.Relevant = true
				item.Capabilities = []string{"Diagnoses production issues"}
				item.Gaps = []string{"No renewal ownership shown"}
			} else {
				item.Gaps = []string{"Record does not address this role"}
			}
			items = append(items, item)
		}
		out, _ := json.Marshal(map[string]any{"items": items,
			"unmet": []string{decoded.Role.Requirements[len(decoded.Role.Requirements)-1]}})
		return string(out), nil
	}
}

// studioSynthesisStub answers SynthesizeConnections with one connection
// quoting each cited record's full situation verbatim.
func studioSynthesisStub() func(context.Context, string, string) (string, error) {
	return func(_ context.Context, payload, _ string) (string, error) {
		var decoded struct {
			Evidences []discovery.ConnectionEvidence `json:"evidences"`
		}
		if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
			return "", err
		}
		supports := make([]discovery.ConnectionSupport, 0, len(decoded.Evidences))
		for _, evidence := range decoded.Evidences {
			supports = append(supports, discovery.ConnectionSupport{
				EvidenceID: evidence.ID, EvidenceRevision: evidence.Revision,
				Quote: evidence.Situation, Action: "Investigated and documented the incident.",
			})
		}
		out, _ := json.Marshal(discovery.ConnectionSynthesis{Connections: []discovery.Connection{{
			ID: "conn-1", Capability: "Production diagnosis",
			Interpretation: "The candidate diagnoses production issues across different systems.",
			Relationship:   "Both records show the same diagnose-then-document pattern in different contexts.",
			Supports:       supports, Limitations: []string{"No fix ownership is established."},
			CandidateUse: "I can explain how I diagnose production issues before escalating.",
		}}})
		return string(out), nil
	}
}

func TestCareerEvidenceAPILifecycle(t *testing.T) {
	a, _ := testWorkspace(t)

	created := careerRequest(t, a, http.MethodPost, "/api/career/evidence", nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("evidence create: %d %s", created.Code, created.Body.String())
	}
	record := careerDecode(t, created)["record"].(map[string]any)
	id := int64(record["id"].(float64))

	getPath := "/api/career/evidence/" + strconv.FormatInt(id, 10)
	payload := studioContentPayload(1, "Night shift diagnosis", "The database slowed at night.", "I inspected slow queries.")
	saved := careerRequest(t, a, http.MethodPost, getPath+"/save", payload)
	if saved.Code != http.StatusOK {
		t.Fatalf("evidence save: %d %s", saved.Code, saved.Body.String())
	}
	if revision := careerDecode(t, saved)["record"].(map[string]any)["revision"].(float64); revision != 2 {
		t.Fatalf("expected revision 2 after save, got %v", revision)
	}

	review := studioContentPayload(2, "Night shift diagnosis", "The database slowed at night.", "I inspected slow queries.")
	if rec := careerRequest(t, a, http.MethodPost, getPath+"/review", review); rec.Code != http.StatusBadRequest {
		t.Fatalf("review without confirmation accepted: %d", rec.Code)
	}
	incomplete := studioContentPayload(2, "", "", "")
	incomplete["confirm_review"] = "yes"
	if rec := careerRequest(t, a, http.MethodPost, getPath+"/review", incomplete); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("incomplete review accepted: %d %s", rec.Code, rec.Body.String())
	}
	review["confirm_review"] = "yes"
	reviewed := careerRequest(t, a, http.MethodPost, getPath+"/review", review)
	if reviewed.Code != http.StatusOK {
		t.Fatalf("evidence review: %d %s", reviewed.Code, reviewed.Body.String())
	}
	if status := careerDecode(t, reviewed)["record"].(map[string]any)["status"].(string); status != "reviewed" {
		t.Fatalf("expected reviewed status, got %q", status)
	}

	if rec := careerRequest(t, a, http.MethodPost, getPath+"/reopen", map[string]any{"revision": 3}); rec.Code != http.StatusBadRequest {
		t.Fatalf("reopen without confirmation accepted: %d", rec.Code)
	}
	reopened := careerRequest(t, a, http.MethodPost, getPath+"/reopen", map[string]any{"revision": 3, "confirm_reopen": "yes"})
	if reopened.Code != http.StatusOK {
		t.Fatalf("evidence reopen: %d %s", reopened.Code, reopened.Body.String())
	}
	reopenedRecord := careerDecode(t, reopened)["record"].(map[string]any)
	if reopenedRecord["status"].(string) != "draft" || reopenedRecord["revision"].(float64) != 4 {
		t.Fatalf("reopen did not return a bumped draft: %v", reopenedRecord)
	}
	if rec := careerRequest(t, a, http.MethodPost, getPath+"/reopen", map[string]any{"revision": 4, "confirm_reopen": "yes"}); rec.Code != http.StatusConflict {
		t.Fatalf("second reopen accepted: %d", rec.Code)
	}

	stale := studioContentPayload(2, "Night shift diagnosis", "Changed.", "I inspected slow queries.")
	if rec := careerRequest(t, a, http.MethodPost, getPath+"/save", stale); rec.Code != http.StatusConflict {
		t.Fatalf("stale save accepted: %d", rec.Code)
	}
	if rec := careerRequest(t, a, http.MethodPost, getPath+"/delete", map[string]any{"revision": 4}); rec.Code != http.StatusBadRequest {
		t.Fatalf("delete without confirmation accepted: %d", rec.Code)
	}
	if rec := careerRequest(t, a, http.MethodPost, getPath+"/delete", map[string]any{"revision": 3, "confirm": "yes"}); rec.Code != http.StatusConflict {
		t.Fatalf("stale delete accepted: %d", rec.Code)
	}
	deleted := careerRequest(t, a, http.MethodPost, getPath+"/delete", map[string]any{"revision": 4, "confirm": "yes"})
	if deleted.Code != http.StatusOK {
		t.Fatalf("evidence delete: %d %s", deleted.Code, deleted.Body.String())
	}
	if rec := careerRequest(t, a, http.MethodGet, getPath, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("deleted record still readable: %d", rec.Code)
	}
}

func TestDeriveRoleProfileFromSavedOpportunity(t *testing.T) {
	job := Job{ID: 7, Title: "Technical Support Engineer", Company: "Example Co.",
		Description: "Diagnose production Linux and MySQL issues.\nDocument incidents and hand off clearly to engineering.\nOwn customer accounts and renewal targets."}
	profile, reason := deriveRoleProfile(job)
	if reason != "" {
		t.Fatalf("usable posting rejected: %s", reason)
	}
	if profile.Key != "job-7" || profile.Title != job.Title || len(profile.Requirements) != 3 {
		t.Fatalf("unexpected derived profile: %+v", profile)
	}
	if profile.Requirements[2] != "Own customer accounts and renewal targets." {
		t.Fatalf("requirements not echoed verbatim: %q", profile.Requirements)
	}
	again, _ := deriveRoleProfile(job)
	if careerRoleHash(profile) != careerRoleHash(again) {
		t.Fatal("role derivation is not deterministic")
	}

	if _, reason := deriveRoleProfile(Job{ID: 8, Title: "Support Engineer"}); reason == "" {
		t.Fatal("description-less posting accepted as comparable")
	}
	if _, reason := deriveRoleProfile(Job{ID: 9}); reason == "" {
		t.Fatal("empty posting accepted as comparable")
	}

	long := Job{ID: 10, Title: strings.Repeat("T", 300), Company: "Co",
		Description: strings.Repeat("Fix production systems carefully. ", 40)}
	capped, reason := deriveRoleProfile(long)
	if reason != "" {
		t.Fatalf("long posting rejected: %s", reason)
	}
	if len(capped.Title) > 160 || len(capped.Focus) > 600 || len(capped.Requirements) > discovery.MaxRoleRequirements {
		t.Fatalf("derived profile exceeds engine budgets: %+v", capped)
	}
	for _, requirement := range capped.Requirements {
		if len(requirement) > 200 {
			t.Fatalf("requirement exceeds budget: %q", requirement)
		}
	}
}

func TestCareerRoleComparisonAgainstRealOpportunities(t *testing.T) {
	a, _ := testWorkspace(t)
	first := studioEvidence(t, a, "Night shift diagnosis",
		"The production database slowed every night shift.", "I inspected slow queries with EXPLAIN and documented the pattern.", true)
	second := studioEvidence(t, a, "Checkout outage",
		"The checkout page returned errors during a sale.", "I traced the errors to a pool limit and paged the owning team.", true)
	studioEvidence(t, a, "Unfinished draft", "", "", false)

	applicable := studioJob(t, a, "https://example.test/jobs/support-1", "Technical Support Engineer", "Example Platform Co.",
		"Diagnose production Linux and MySQL issues.\nDocument incidents and hand off clearly to engineering.")
	inapplicable := studioJob(t, a, "https://example.test/jobs/accounts-1", "Technical Account Manager", "Example Customer Co.",
		"Explain technical issues clearly to customers.\nOwn customer accounts and renewal targets.")
	studioJob(t, a, "https://example.test/jobs/empty-1", "Mystery Role", "", "")

	listed := careerRequest(t, a, http.MethodGet, "/api/career/roles", nil)
	if listed.Code != http.StatusOK {
		t.Fatalf("roles list: %d %s", listed.Code, listed.Body.String())
	}
	list := careerDecode(t, listed)
	if len(list["reviewed"].([]any)) != 2 {
		t.Fatalf("draft leaked into reviewed selection: %v", list["reviewed"])
	}
	roles := list["roles"].([]any)
	byID := map[float64]map[string]any{}
	for _, item := range roles {
		role := item.(map[string]any)
		byID[role["job_id"].(float64)] = role
	}
	if !byID[float64(applicable)]["usable"].(bool) || !byID[float64(inapplicable)]["usable"].(bool) {
		t.Fatalf("real postings not comparable: %v", roles)
	}
	if byID[float64(applicable)]["requirements_count"].(float64) != 2 {
		t.Fatalf("unexpected derived requirements: %v", byID[float64(applicable)])
	}

	a.careerStudioGenerate = studioRoleStub(t)
	snapshotPath := "/api/career/roles/job-" + strconv.FormatInt(applicable, 10) + "?evidence_ids=" + strconv.FormatInt(first.ID, 10)
	snapshot := careerRequest(t, a, http.MethodGet, snapshotPath, nil)
	if snapshot.Code != http.StatusOK {
		t.Fatalf("role snapshot: %d %s", snapshot.Code, snapshot.Body.String())
	}
	if strings.Contains(snapshot.Body.String(), first.Situation) || strings.Contains(snapshot.Body.String(), "You compare reviewed") {
		t.Fatal("role snapshot leaks source text or policy bytes")
	}
	view := careerDecode(t, snapshot)
	inputHash := view["input_hash"].(string)
	if inputHash == "" || view["bytes"].(float64) < 1 {
		t.Fatalf("snapshot missing binding: %v", view)
	}

	authorizePath := "/api/career/roles/job-" + strconv.FormatInt(applicable, 10) + "/authorize"
	denied := careerRequest(t, a, http.MethodPost, authorizePath, map[string]any{
		"provider": "ollama", "model": "stub", "evidence_ids": []int64{first.ID}, "input_hash": inputHash,
		"authorize_generation": "yes",
	})
	if denied.Code != http.StatusForbidden {
		t.Fatalf("single confirmation authorized inference: %d", denied.Code)
	}
	stale := careerRequest(t, a, http.MethodPost, authorizePath, map[string]any{
		"provider": "ollama", "model": "stub", "evidence_ids": []int64{first.ID}, "input_hash": "deadbeef",
		"authorize_generation": "yes", "authorize_external": "yes",
	})
	if stale.Code != http.StatusConflict {
		t.Fatalf("mismatched input hash accepted: %d", stale.Code)
	}

	single := careerRequest(t, a, http.MethodPost, authorizePath, map[string]any{
		"provider": "ollama", "model": "stub", "evidence_ids": []int64{first.ID}, "input_hash": inputHash,
		"authorize_generation": "yes", "authorize_external": "yes",
	})
	if single.Code != http.StatusOK {
		t.Fatalf("single-record authorize: %d %s", single.Code, single.Body.String())
	}
	saved := careerDecode(t, single)["saved"].(map[string]any)
	if saved["stale"].(bool) || saved["has_analysis"].(bool) != true {
		t.Fatalf("fresh analysis reported stale: %v", saved)
	}
	analysis := saved["analysis"].(map[string]any)
	if analysis["role_key"].(string) != "job-"+strconv.FormatInt(applicable, 10) || len(analysis["items"].([]any)) != 1 {
		t.Fatalf("unexpected saved analysis: %v", analysis)
	}
	if unmet := analysis["unmet"].([]any); len(unmet) != 1 || unmet[0].(string) != "Document incidents and hand off clearly to engineering." {
		t.Fatalf("unsupported requirements not identified: %v", unmet)
	}

	// One reviewed experience against the inapplicable role: the same stub
	// reading marks it relevant to its single record while the renewal
	// requirement stays explicitly unmet.
	inappSnapshot := careerRequest(t, a, http.MethodGet,
		"/api/career/roles/job-"+strconv.FormatInt(inapplicable, 10)+"?evidence_ids="+strconv.FormatInt(first.ID, 10), nil)
	inappHash := careerDecode(t, inappSnapshot)["input_hash"].(string)
	inapp := careerRequest(t, a, http.MethodPost,
		"/api/career/roles/job-"+strconv.FormatInt(inapplicable, 10)+"/authorize", map[string]any{
			"provider": "ollama", "model": "stub", "evidence_ids": []int64{first.ID}, "input_hash": inappHash,
			"authorize_generation": "yes", "authorize_external": "yes",
		})
	if inapp.Code != http.StatusOK {
		t.Fatalf("inapplicable role authorize: %d %s", inapp.Code, inapp.Body.String())
	}
	inappAnalysis := careerDecode(t, inapp)["saved"].(map[string]any)["analysis"].(map[string]any)
	if unmet := inappAnalysis["unmet"].([]any); len(unmet) != 1 || unmet[0].(string) != "Own customer accounts and renewal targets." {
		t.Fatalf("inapplicable unmet requirements not identified: %v", unmet)
	}

	// Multi-record comparison against the applicable role.
	multiSnapshot := careerRequest(t, a, http.MethodGet,
		"/api/career/roles/job-"+strconv.FormatInt(applicable, 10)+"?evidence_ids="+strconv.FormatInt(first.ID, 10)+","+strconv.FormatInt(second.ID, 10), nil)
	if multiSnapshot.Code != http.StatusOK {
		t.Fatalf("multi snapshot: %d %s", multiSnapshot.Code, multiSnapshot.Body.String())
	}
	multi := careerRequest(t, a, http.MethodPost, authorizePath, map[string]any{
		"provider": "ollama", "model": "stub",
		"evidence_ids": []int64{first.ID, second.ID}, "input_hash": careerDecode(t, multiSnapshot)["input_hash"].(string),
		"authorize_generation": "yes", "authorize_external": "yes",
	})
	if multi.Code != http.StatusOK {
		t.Fatalf("multi-record authorize: %d %s", multi.Code, multi.Body.String())
	}
	if items := careerDecode(t, multi)["saved"].(map[string]any)["analysis"].(map[string]any)["items"].([]any); len(items) != 2 {
		t.Fatalf("expected two-item analysis, got %v", items)
	}

	// Invalid model output saves nothing and preserves the prior analysis.
	a.careerStudioGenerate = func(context.Context, string, string) (string, error) { return "not json", nil }
	fresh := careerRequest(t, a, http.MethodGet,
		"/api/career/roles/job-"+strconv.FormatInt(applicable, 10)+"?evidence_ids="+strconv.FormatInt(first.ID, 10), nil)
	failed := careerRequest(t, a, http.MethodPost, authorizePath, map[string]any{
		"provider": "ollama", "model": "stub", "evidence_ids": []int64{first.ID},
		"input_hash":           careerDecode(t, fresh)["input_hash"].(string),
		"authorize_generation": "yes", "authorize_external": "yes",
	})
	if failed.Code != http.StatusUnprocessableEntity || careerDecode(t, failed)["code"].(string) != "invalid_output" {
		t.Fatalf("invalid output not reported: %d %s", failed.Code, failed.Body.String())
	}
	after := careerRequest(t, a, http.MethodGet,
		"/api/career/roles/job-"+strconv.FormatInt(applicable, 10)+"?evidence_ids="+strconv.FormatInt(first.ID, 10), nil)
	preserved := careerDecode(t, after)["saved"].(map[string]any)
	if preserved["has_analysis"].(bool) != true || len(preserved["analysis"].(map[string]any)["items"].([]any)) != 2 {
		t.Fatalf("prior analysis not preserved: %v", preserved)
	}
}

func TestCareerRoleStalenessAfterCorrection(t *testing.T) {
	a, _ := testWorkspace(t)
	first := studioEvidence(t, a, "Night shift diagnosis",
		"The production database slowed every night shift.", "I inspected slow queries with EXPLAIN.", true)
	jobID := studioJob(t, a, "https://example.test/jobs/support-2", "Support Engineer", "Example Co.",
		"Diagnose production issues.\nDocument incidents clearly.")
	a.careerStudioGenerate = studioRoleStub(t)

	snapshot := careerRequest(t, a, http.MethodGet,
		"/api/career/roles/job-"+strconv.FormatInt(jobID, 10)+"?evidence_ids="+strconv.FormatInt(first.ID, 10), nil)
	authorized := careerRequest(t, a, http.MethodPost,
		"/api/career/roles/job-"+strconv.FormatInt(jobID, 10)+"/authorize", map[string]any{
			"provider": "ollama", "model": "stub", "evidence_ids": []int64{first.ID},
			"input_hash":           careerDecode(t, snapshot)["input_hash"].(string),
			"authorize_generation": "yes", "authorize_external": "yes",
		})
	if authorized.Code != http.StatusOK {
		t.Fatalf("authorize: %d %s", authorized.Code, authorized.Body.String())
	}

	// A source correction bumps the revision and returns the record to draft
	// through the existing owner; the saved interpretation stays visible but
	// is flagged stale with the corrected record named.
	roleSaved := func() map[string]any {
		t.Helper()
		listed := careerRequest(t, a, http.MethodGet, "/api/career/roles", nil)
		if listed.Code != http.StatusOK {
			t.Fatalf("roles list: %d %s", listed.Code, listed.Body.String())
		}
		for _, item := range careerDecode(t, listed)["roles"].([]any) {
			role := item.(map[string]any)
			if role["job_id"].(float64) == float64(jobID) {
				return role["saved"].(map[string]any)
			}
		}
		t.Fatal("role missing from list")
		return nil
	}
	corrected := first
	corrected.Actions = "I inspected slow queries with EXPLAIN and confirmed the pattern twice."
	if err := a.updateCareerEvidence(first.ID, first.Revision, corrected.content(), false); err != nil {
		t.Fatal(err)
	}
	saved := roleSaved()
	if saved["stale"].(bool) != true || saved["stale_evidence"].(bool) != true {
		t.Fatalf("corrected source not flagged stale: %v", saved)
	}
	if ids := saved["stale_evidence_ids"].([]any); len(ids) != 1 || int64(ids[0].(float64)) != first.ID {
		t.Fatalf("stale record not named: %v", saved)
	}
	if saved["has_analysis"].(bool) != true {
		t.Fatal("stale analysis hidden instead of flagged")
	}

	// A posting edit flags the role side independently.
	job, err := a.jobByID(jobID)
	if err != nil {
		t.Fatal(err)
	}
	job.Description = "Diagnose production issues.\nDocument incidents clearly.\nCoach junior engineers."
	if err := a.updateJob(job); err != nil {
		t.Fatal(err)
	}
	if resaved := roleSaved(); resaved["stale_role"].(bool) != true {
		t.Fatalf("edited posting not flagged stale: %v", resaved)
	}

	// Re-reviewing the corrected record and re-authorizing against the fresh
	// snapshot clears both flags.
	current, err := careerEvidenceByID(a.db, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.updateCareerEvidence(first.ID, current.Revision, current.content(), true); err != nil {
		t.Fatal(err)
	}
	fresh := careerDecode(t, careerRequest(t, a, http.MethodGet,
		"/api/career/roles/job-"+strconv.FormatInt(jobID, 10)+"?evidence_ids="+strconv.FormatInt(first.ID, 10), nil))
	refreshed := careerRequest(t, a, http.MethodPost,
		"/api/career/roles/job-"+strconv.FormatInt(jobID, 10)+"/authorize", map[string]any{
			"provider": "ollama", "model": "stub", "evidence_ids": []int64{first.ID},
			"input_hash":           fresh["input_hash"].(string),
			"authorize_generation": "yes", "authorize_external": "yes",
		})
	if refreshed.Code != http.StatusOK {
		t.Fatalf("re-authorize: %d %s", refreshed.Code, refreshed.Body.String())
	}
	if careerDecode(t, refreshed)["saved"].(map[string]any)["stale"].(bool) {
		t.Fatal("refreshed analysis still stale")
	}
}

func TestCareerSynthesisChallengeAndStaleness(t *testing.T) {
	a, _ := testWorkspace(t)
	var jobsBefore, libraryBefore, submissionsBefore int
	if err := a.db.QueryRow(`SELECT (SELECT COUNT(*) FROM jobs),(SELECT COUNT(*) FROM answer_library),(SELECT COUNT(*) FROM submissions)`).
		Scan(&jobsBefore, &libraryBefore, &submissionsBefore); err != nil {
		t.Fatal(err)
	}

	first := studioEvidence(t, a, "Night shift diagnosis",
		"The production database slowed every night shift.", "I inspected slow queries with EXPLAIN.", true)
	only := careerRequest(t, a, http.MethodGet, "/api/career/synthesis", nil)
	if only.Code != http.StatusOK {
		t.Fatalf("synthesis view: %d %s", only.Code, only.Body.String())
	}
	if hash := careerDecode(t, only)["input_hash"].(string); hash != "" {
		t.Fatal("single reviewed record offered a synthesis binding")
	}
	a.careerStudioGenerate = studioSynthesisStub()
	denied := careerRequest(t, a, http.MethodPost, "/api/career/synthesis/authorize", map[string]any{
		"provider": "ollama", "model": "stub", "input_hash": "x",
		"authorize_generation": "yes", "authorize_external": "yes",
	})
	if denied.Code != http.StatusUnprocessableEntity {
		t.Fatalf("single-record synthesis accepted: %d", denied.Code)
	}

	second := studioEvidence(t, a, "Checkout outage",
		"The checkout page returned errors during a sale.", "I traced the errors to a pool limit.", true)
	ready := careerDecode(t, careerRequest(t, a, http.MethodGet, "/api/career/synthesis", nil))
	if len(ready["eligible"].([]any)) != 2 || ready["input_hash"].(string) == "" {
		t.Fatalf("eligible pair not bound: %v", ready)
	}
	authorized := careerRequest(t, a, http.MethodPost, "/api/career/synthesis/authorize", map[string]any{
		"provider": "ollama", "model": "stub", "input_hash": ready["input_hash"].(string),
		"authorize_generation": "yes", "authorize_external": "yes",
	})
	if authorized.Code != http.StatusOK {
		t.Fatalf("synthesis authorize: %d %s", authorized.Code, authorized.Body.String())
	}
	saved := careerDecode(t, authorized)["saved"].(map[string]any)
	if saved["stale"].(bool) || saved["has_synthesis"].(bool) != true {
		t.Fatalf("fresh synthesis reported stale: %v", saved)
	}
	connections := saved["connections"].([]any)
	if len(connections) != 1 {
		t.Fatalf("expected one connection, got %v", connections)
	}
	supports := connections[0].(map[string]any)["connection"].(map[string]any)["supports"].([]any)
	if len(supports) != 2 {
		t.Fatalf("connection not grounded in both records: %v", supports)
	}

	challenged := careerRequest(t, a, http.MethodPost, "/api/career/synthesis/challenges/conn-1", map[string]any{
		"state": "questioned", "note": "The checkout record shows paging, not diagnosis.",
	})
	if challenged.Code != http.StatusOK {
		t.Fatalf("challenge: %d %s", challenged.Code, challenged.Body.String())
	}
	afterChallenge := careerDecode(t, challenged)["saved"].(map[string]any)
	if afterChallenge["challenges_count"].(float64) != 1 {
		t.Fatalf("challenge not counted: %v", afterChallenge)
	}
	item := afterChallenge["connections"].([]any)[0].(map[string]any)
	if item["challenge"].(string) != "questioned" || !strings.Contains(item["note"].(string), "paging") {
		t.Fatalf("challenge not recorded: %v", item)
	}
	if bad := careerRequest(t, a, http.MethodPost, "/api/career/synthesis/challenges/conn-1", map[string]any{"state": "loved"}); bad.Code != http.StatusBadRequest {
		t.Fatalf("invalid challenge state accepted: %d", bad.Code)
	}
	if missing := careerRequest(t, a, http.MethodPost, "/api/career/synthesis/challenges/conn-2", map[string]any{"state": "questioned"}); missing.Code != http.StatusNotFound {
		t.Fatalf("challenge to absent connection accepted: %d", missing.Code)
	}

	// A source correction flags the saved synthesis and its connection
	// stale while the prior interpretation and challenge stay visible.
	corrected := second
	corrected.Situation = "The checkout page returned errors during a sale I did not observe."
	if err := a.updateCareerEvidence(second.ID, second.Revision, corrected.content(), false); err != nil {
		t.Fatal(err)
	}
	afterCorrection := careerDecode(t, careerRequest(t, a, http.MethodGet, "/api/career/synthesis", nil))["saved"].(map[string]any)
	if afterCorrection["stale"].(bool) != true {
		t.Fatalf("corrected source left synthesis current: %v", afterCorrection)
	}
	staleItem := afterCorrection["connections"].([]any)[0].(map[string]any)
	if staleItem["stale"].(bool) != true || staleItem["challenge"].(string) != "questioned" {
		t.Fatalf("stale connection lost its flag or challenge: %v", staleItem)
	}
	if afterCorrection["has_synthesis"].(bool) != true {
		t.Fatal("stale synthesis hidden instead of flagged")
	}

	var jobsAfter, libraryAfter, submissionsAfter int
	if err := a.db.QueryRow(`SELECT (SELECT COUNT(*) FROM jobs),(SELECT COUNT(*) FROM answer_library),(SELECT COUNT(*) FROM submissions)`).
		Scan(&jobsAfter, &libraryAfter, &submissionsAfter); err != nil {
		t.Fatal(err)
	}
	if jobsBefore != jobsAfter || libraryBefore != libraryAfter || submissionsBefore != submissionsAfter {
		t.Fatal("studio flow touched jobs, answers, or submissions")
	}
	_ = first
}

func TestCareerStudioSelectionsSurviveRestart(t *testing.T) {
	a, dir := testWorkspace(t)
	first := studioEvidence(t, a, "Night shift diagnosis",
		"The production database slowed every night shift.", "I inspected slow queries with EXPLAIN.", true)
	second := studioEvidence(t, a, "Checkout outage",
		"The checkout page returned errors during a sale.", "I traced the errors to a pool limit.", true)
	jobID := studioJob(t, a, "https://example.test/jobs/support-3", "Support Engineer", "Example Co.",
		"Diagnose production issues.\nDocument incidents clearly.")
	a.careerStudioGenerate = studioRoleStub(t)
	snapshot := careerDecode(t, careerRequest(t, a, http.MethodGet,
		"/api/career/roles/job-"+strconv.FormatInt(jobID, 10)+"?evidence_ids="+strconv.FormatInt(first.ID, 10), nil))
	if rec := careerRequest(t, a, http.MethodPost,
		"/api/career/roles/job-"+strconv.FormatInt(jobID, 10)+"/authorize", map[string]any{
			"provider": "ollama", "model": "stub", "evidence_ids": []int64{first.ID},
			"input_hash":           snapshot["input_hash"].(string),
			"authorize_generation": "yes", "authorize_external": "yes",
		}); rec.Code != http.StatusOK {
		t.Fatalf("role authorize: %d %s", rec.Code, rec.Body.String())
	}
	a.careerStudioGenerate = studioSynthesisStub()
	ready := careerDecode(t, careerRequest(t, a, http.MethodGet, "/api/career/synthesis", nil))
	if rec := careerRequest(t, a, http.MethodPost, "/api/career/synthesis/authorize", map[string]any{
		"provider": "ollama", "model": "stub", "input_hash": ready["input_hash"].(string),
		"authorize_generation": "yes", "authorize_external": "yes",
	}); rec.Code != http.StatusOK {
		t.Fatalf("synthesis authorize: %d %s", rec.Code, rec.Body.String())
	}
	if rec := careerRequest(t, a, http.MethodPost, "/api/career/synthesis/challenges/conn-1", map[string]any{
		"state": "unsupported", "note": "Checking restart durability.",
	}); rec.Code != http.StatusOK {
		t.Fatalf("challenge: %d %s", rec.Code, rec.Body.String())
	}

	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	restartedDB, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	restarted := newTestApp(t, restartedDB, filepath.Join(dir, "resumes"))

	roles := careerDecode(t, careerRequest(t, restarted, http.MethodGet, "/api/career/roles", nil))
	found := false
	for _, item := range roles["roles"].([]any) {
		role := item.(map[string]any)
		if role["job_id"].(float64) != float64(jobID) {
			continue
		}
		found = true
		saved, ok := role["saved"].(map[string]any)
		if !ok || saved["has_analysis"].(bool) != true || saved["stale"].(bool) {
			t.Fatalf("role analysis did not survive restart current: %v", role["saved"])
		}
	}
	if !found {
		t.Fatal("role missing after restart")
	}
	synthesis := careerDecode(t, careerRequest(t, restarted, http.MethodGet, "/api/career/synthesis", nil))["saved"].(map[string]any)
	if synthesis["has_synthesis"].(bool) != true || synthesis["stale"].(bool) {
		t.Fatalf("synthesis did not survive restart current: %v", synthesis)
	}
	if synthesis["challenges_count"].(float64) != 1 {
		t.Fatalf("challenge did not survive restart: %v", synthesis)
	}
	if _, err := careerEvidenceByID(restarted.db, first.ID); err != nil {
		t.Fatalf("first evidence missing after restart: %v", err)
	}
	if _, err := careerEvidenceByID(restarted.db, second.ID); err != nil {
		t.Fatalf("second evidence missing after restart: %v", err)
	}
}

func TestCareerRoleSnapshotRejectsDraftSelection(t *testing.T) {
	a, _ := testWorkspace(t)
	draft := studioEvidence(t, a, "Unfinished draft", "Some situation.", "Some actions.", false)
	jobID := studioJob(t, a, "https://example.test/jobs/support-4", "Support Engineer", "Example Co.",
		"Diagnose production issues.")
	rec := careerRequest(t, a, http.MethodGet,
		"/api/career/roles/job-"+strconv.FormatInt(jobID, 10)+"?evidence_ids="+strconv.FormatInt(draft.ID, 10), nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("draft selection snapshot: %d %s", rec.Code, rec.Body.String())
	}
	if code := careerDecode(t, rec)["code"].(string); code != "not_reviewed" {
		t.Fatalf("expected not_reviewed, got %q", code)
	}
}
