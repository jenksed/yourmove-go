package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	discovery "github.com/jenksed/yourmove-career-discovery"
	"github.com/jenksed/yourmove-go/internal/storage"
)

func careerRequest(t *testing.T, a *App, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Host = "127.0.0.1:3000"
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	return rec
}

func careerDecode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &value); err != nil {
		t.Fatalf("decode %d body %q: %v", rec.Code, rec.Body.String(), err)
	}
	return value
}

// careerDOCX builds a synthetic resume with real extractable paragraphs.
func careerDOCX(t *testing.T, paragraphs ...string) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	types, err := z.Create("[Content_Types].xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(types, `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`); err != nil {
		t.Fatal(err)
	}
	f, err := z.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	sb.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`)
	for _, p := range paragraphs {
		sb.WriteString(`<w:p><w:r><w:t>`)
		if err := xml.EscapeText(&sb, []byte(p)); err != nil {
			t.Fatal(err)
		}
		sb.WriteString(`</w:t></w:r></w:p>`)
	}
	sb.WriteString(`</w:document>`)
	if _, err := io.WriteString(f, sb.String()); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

const careerLeadsStub = `{"leads":[` +
	`{"id":"lead-1","title":"On-call triage rotation","why":"Weekly rotation with incident decisions worth documenting"},` +
	`{"id":"lead-2","title":"Junior mentoring","why":"Coaching approach and feedback may reveal methods"},` +
	`{"id":"lead-3","title":"Legacy migration","why":"Upgrade choices and handoffs may reveal judgment"}]}`

const careerQuestionStub = `{"question":"What did you decide when the pager fired?","proposal":null}`

// The quoted excerpt must stay a contiguous verbatim slice of the answer.
const careerAnswerText = "I decided to page the specialist after checking the dashboard."

const careerProposalStub = `{"question":"Did anyone confirm your diagnosis was useful?","proposal":` +
	`{"title":"Paged specialist during triage","experience":"On-call triage with dashboard review","claim":"Paged the specialist after checking the dashboard",` +
	`"source_id":"response-1","quote":"page the specialist after checking the dashboard",` +
	`"supports":[{"source_id":"response-1","quote":"page the specialist after checking the dashboard"}],` +
	`"unresolved":["Did the specialist confirm the diagnosis?"]}}`

func careerStubScript(t *testing.T, outputs []string) func(context.Context, string, string) (string, error) {
	t.Helper()
	index := 0
	return func(_ context.Context, prompt, policy string) (string, error) {
		if policy != discovery.Policy && policy != discovery.LeadsPolicy {
			t.Fatalf("unpinned policy passed to inference: %q", policy)
		}
		if index >= len(outputs) {
			t.Fatalf("unexpected inference call %d", index+1)
			return "", errors.New("no scripted output")
		}
		out := outputs[index]
		index++
		_ = prompt
		return out, nil
	}
}

func careerCreateExcerpt(t *testing.T, a *App, excerpt string) map[string]any {
	t.Helper()
	rec := careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions", map[string]any{"excerpt": excerpt})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", rec.Code, rec.Body.String())
	}
	return careerDecode(t, rec)
}

func careerAuthorize(t *testing.T, a *App, view map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	next := view["next"].(map[string]any)
	return careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+view["id"].(string)+"/authorize",
		map[string]any{"provider": "ollama", "model": "qwen3.5:4b",
			"authorize_generation": "yes", "authorize_external": "yes",
			"input_hash": next["input_hash"].(string)})
}

func TestCareerDiscoveryCreateFromExcerpt(t *testing.T) {
	a, _ := testWorkspace(t)
	view := careerCreateExcerpt(t, a, "Sam was an on-call engineer who triaged incidents and mentored juniors.")
	if view["status"] != careerDiscoveryLeads {
		t.Fatalf("status %v", view["status"])
	}
	if view["resume"] != nil {
		t.Fatalf("excerpt session should have no resume: %v", view["resume"])
	}
	next := view["next"].(map[string]any)
	if next["kind"] != "leads" || next["input_hash"] == "" || next["policy_name"] == "" || next["bytes"] == nil {
		t.Fatalf("pending leads step: %v", next)
	}
	if _, ok := next["policy"]; ok {
		t.Fatalf("next step leaks policy: %v", next)
	}
	if _, ok := next["transmit"]; ok {
		t.Fatalf("next step leaks transmit: %v", next)
	}
	if _, ok := view["excerpt"]; ok {
		t.Fatalf("view leaks excerpt bytes: %v", view["excerpt"])
	}
	if view["excerpt_hash"] == "" || view["excerpt_bytes"] == nil {
		t.Fatalf("excerpt summary missing: %v", view)
	}
	if view["revision"] != float64(1) {
		t.Fatalf("revision %v", view["revision"])
	}
}

func TestCareerDiscoveryCreateValidation(t *testing.T) {
	a, _ := testWorkspace(t)
	for _, body := range []map[string]any{
		{"excerpt": "   "},
		{"excerpt": strings.Repeat("x", 8001)},
		{"excerpt": "valid but(has\x00nul)"},
	} {
		rec := careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions", body)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("excerpt %q status %d: %s", body["excerpt"], rec.Code, rec.Body.String())
		}
	}
	rec := careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions", map[string]any{"excerpt": ""})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing input status %d", rec.Code)
	}
	rec = careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions",
		map[string]any{"resume_id": 1, "excerpt": "both"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("both inputs status %d", rec.Code)
	}
	rec = careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty status %d", rec.Code)
	}
	rec = careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions", map[string]any{"resume_id": 99})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown resume status %d", rec.Code)
	}
}

func TestCareerDiscoveryCreateFromResume(t *testing.T) {
	a, _ := testWorkspace(t)
	resume, err := a.importResume("sam.docx", bytes.NewReader(careerDOCX(t,
		"Sam Rivera, on-call engineer.",
		"Triaged incidents weekly and mentored two juniors.")))
	if err != nil {
		t.Fatal(err)
	}
	rec := careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions",
		map[string]any{"resume_id": resume.ID})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", rec.Code, rec.Body.String())
	}
	view := careerDecode(t, rec)
	resumeBlock := view["resume"].(map[string]any)
	if resumeBlock["id"] != float64(resume.ID) || resumeBlock["name"] != "sam.docx" ||
		resumeBlock["sha256"] != resume.SHA256 || resumeBlock["removed"] != false {
		t.Fatalf("resume block: %v", resumeBlock)
	}
	if view["excerpt_hash"] == "" || view["excerpt_bytes"] == nil {
		t.Fatalf("excerpt summary: %v", view)
	}
	// The extracted text persists server-side only; the API returns a
	// hash/byte summary, never the bytes.
	var storedState string
	if err := a.db.QueryRow(`SELECT state_json FROM career_discovery_sessions WHERE id=?`,
		view["id"].(string)).Scan(&storedState); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(storedState, "Triaged incidents weekly") {
		t.Fatalf("stored excerpt: %s", storedState)
	}
	if _, err := a.db.Exec(`UPDATE resumes SET archived_at='2026-01-01T00:00:00Z' WHERE id=?`, resume.ID); err != nil {
		t.Fatal(err)
	}
	rec = careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions",
		map[string]any{"resume_id": resume.ID})
	if rec.Code != http.StatusConflict {
		t.Fatalf("archived status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCareerDiscoveryLongResumeRequiresExcerpt(t *testing.T) {
	a, _ := testWorkspace(t)
	resume, err := a.importResume("long.docx", bytes.NewReader(careerDOCX(t, strings.Repeat("duty ", 2000))))
	if err != nil {
		t.Fatal(err)
	}
	rec := careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions",
		map[string]any{"resume_id": resume.ID})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "excerpt_required") {
		t.Fatalf("long resume status %d: %s", rec.Code, rec.Body.String())
	}
	rec = careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions",
		map[string]any{"resume_id": resume.ID, "excerpt_end": 8000})
	if rec.Code != http.StatusCreated {
		t.Fatalf("full-size excerpt_end status %d: %s", rec.Code, rec.Body.String())
	}
	full := careerDecode(t, rec)
	if full["excerpt_bytes"].(float64) > 8000 {
		t.Fatalf("fitted excerpt bytes: %v", full["excerpt_bytes"])
	}
	var fullState struct {
		Resume struct {
			Text string `json:"text"`
		} `json:"resume"`
	}
	var fullStateJSON string
	if err := a.db.QueryRow(`SELECT state_json FROM career_discovery_sessions WHERE id=?`,
		full["id"].(string)).Scan(&fullStateJSON); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(fullStateJSON), &fullState); err != nil {
		t.Fatal(err)
	}
	if len(fullState.Resume.Text) > 8000 || !strings.HasSuffix(fullState.Resume.Text, "\n[Excerpt truncated]") {
		t.Fatalf("fitted excerpt: %q", fullState.Resume.Text)
	}
	rec = careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions",
		map[string]any{"resume_id": resume.ID, "excerpt_end": 7000})
	if rec.Code != http.StatusCreated {
		t.Fatalf("bounded excerpt status %d: %s", rec.Code, rec.Body.String())
	}
	view := careerDecode(t, rec)
	var boundedStateJSON string
	if err := a.db.QueryRow(`SELECT state_json FROM career_discovery_sessions WHERE id=?`,
		view["id"].(string)).Scan(&boundedStateJSON); err != nil {
		t.Fatal(err)
	}
	var boundedState struct {
		Resume struct {
			Text string `json:"text"`
		} `json:"resume"`
	}
	if err := json.Unmarshal([]byte(boundedStateJSON), &boundedState); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(boundedState.Resume.Text, "\n[Excerpt truncated]") {
		t.Fatalf("truncation marker missing: %q", boundedState.Resume.Text)
	}
}

func TestCareerDiscoveryAuthorizeLeads(t *testing.T) {
	a, _ := testWorkspace(t)
	view := careerCreateExcerpt(t, a, "Sam was an on-call engineer who triaged incidents and mentored juniors.")
	id := view["id"].(string)
	next := view["next"].(map[string]any)

	denied := careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/authorize",
		map[string]any{"provider": "ollama", "model": "qwen3.5:4b",
			"authorize_generation": "yes", "input_hash": next["input_hash"].(string)})
	if denied.Code != http.StatusForbidden {
		t.Fatalf("missing external ack status %d", denied.Code)
	}
	stale := careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/authorize",
		map[string]any{"provider": "ollama", "model": "qwen3.5:4b",
			"authorize_generation": "yes", "authorize_external": "yes", "input_hash": strings.Repeat("0", 64)})
	if stale.Code != http.StatusConflict {
		t.Fatalf("wrong input hash status %d", stale.Code)
	}
	badProvider := careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/authorize",
		map[string]any{"provider": "baseten", "model": "x",
			"authorize_generation": "yes", "authorize_external": "yes", "input_hash": next["input_hash"].(string)})
	if badProvider.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown provider status %d", badProvider.Code)
	}

	a.careerDiscoveryGenerate = careerStubScript(t, []string{careerLeadsStub})
	rec := careerAuthorize(t, a, view)
	if rec.Code != http.StatusOK {
		t.Fatalf("authorize status %d: %s", rec.Code, rec.Body.String())
	}
	updated := careerDecode(t, rec)
	if updated["status"] != careerDiscoveryInterview {
		t.Fatalf("status %v", updated["status"])
	}
	leads := updated["leads"].([]any)
	if len(leads) != 3 || updated["selected_lead_id"] != "" {
		t.Fatalf("leads: %v", updated["leads"])
	}
	if updated["provider"] != "ollama" || updated["model"] != "qwen3.5:4b" {
		t.Fatalf("provider echo: %v %v", updated["provider"], updated["model"])
	}
	if updated["failure"] != nil {
		t.Fatalf("failure: %v", updated["failure"])
	}
	if updated["next"].(map[string]any)["kind"] != "interview" {
		t.Fatalf("next: %v", updated["next"])
	}
}

func TestCareerDiscoverySelectAndAnswer(t *testing.T) {
	a, _ := testWorkspace(t)
	a.careerDiscoveryGenerate = careerStubScript(t, []string{careerLeadsStub})
	view := careerCreateExcerpt(t, a, "Sam was an on-call engineer who triaged incidents and mentored juniors.")
	view = careerDecode(t, careerAuthorize(t, a, view))
	id := view["id"].(string)
	revision := view["revision"].(float64)

	rec := careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/select",
		map[string]any{"lead_id": "lead-9", "revision": revision})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown lead status %d", rec.Code)
	}
	rec = careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/select",
		map[string]any{"lead_id": "lead-2", "revision": revision})
	if rec.Code != http.StatusOK {
		t.Fatalf("select status %d: %s", rec.Code, rec.Body.String())
	}
	view = careerDecode(t, rec)
	if view["selected_lead_id"] != "lead-2" {
		t.Fatalf("selection: %v", view["selected_lead_id"])
	}

	// No pending question yet: an answer has nothing to attach to.
	rec = careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/answer",
		map[string]any{"answer": "early", "revision": view["revision"].(float64)})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("answer without question status %d", rec.Code)
	}
	rec = careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/answer",
		map[string]any{"answer": "x", "revision": float64(999)})
	if rec.Code != http.StatusConflict {
		t.Fatalf("stale revision status %d", rec.Code)
	}
}

func TestCareerDiscoveryInterviewTurnProposalAndPromote(t *testing.T) {
	a, _ := testWorkspace(t)
	a.careerDiscoveryGenerate = careerStubScript(t, []string{careerLeadsStub, careerQuestionStub, careerProposalStub})
	view := careerCreateExcerpt(t, a, "Sam was an on-call engineer who triaged incidents and mentored juniors.")
	view = careerDecode(t, careerAuthorize(t, a, view))
	id := view["id"].(string)

	rec := careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/select",
		map[string]any{"lead_id": "lead-1", "revision": view["revision"].(float64)})
	view = careerDecode(t, rec)

	rec = careerAuthorize(t, a, view)
	if rec.Code != http.StatusOK {
		t.Fatalf("interview authorize status %d: %s", rec.Code, rec.Body.String())
	}
	view = careerDecode(t, rec)
	if view["question"] != "What did you decide when the pager fired?" {
		t.Fatalf("question: %v", view["question"])
	}
	if view["proposal"] != nil {
		t.Fatalf("early proposal: %v", view["proposal"])
	}

	rec = careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/answer",
		map[string]any{"answer": careerAnswerText, "revision": view["revision"].(float64)})
	if rec.Code != http.StatusOK {
		t.Fatalf("answer status %d: %s", rec.Code, rec.Body.String())
	}
	view = careerDecode(t, rec)
	if len(view["responses"].([]any)) != 1 || view["question"] != "" {
		t.Fatalf("recorded answer: %v", view)
	}

	rec = careerAuthorize(t, a, view)
	if rec.Code != http.StatusOK {
		t.Fatalf("proposal authorize status %d: %s", rec.Code, rec.Body.String())
	}
	view = careerDecode(t, rec)
	proposal := view["proposal"].(map[string]any)
	if proposal["title"] != "Paged specialist during triage" {
		t.Fatalf("proposal: %v", proposal)
	}
	if len(proposal["supports"].([]any)) != 1 || len(proposal["unresolved"].([]any)) != 1 {
		t.Fatalf("proposal links: %v", proposal)
	}

	// Selection is closed once answers exist.
	rec = careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/select",
		map[string]any{"lead_id": "lead-2", "revision": view["revision"].(float64)})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("late select status %d", rec.Code)
	}

	rec = careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/promote",
		map[string]any{"revision": view["revision"].(float64)})
	if rec.Code != http.StatusOK {
		t.Fatalf("promote status %d: %s", rec.Code, rec.Body.String())
	}
	promoted := careerDecode(t, rec)
	evidenceID := int64(promoted["evidence_id"].(float64))
	if promoted["path"] != "/career-evidence/"+strconv.FormatInt(evidenceID, 10) {
		t.Fatalf("draft path: %v", promoted)
	}
	evidence, err := careerEvidenceByID(a.db, evidenceID)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "draft" || evidence.Revision != 2 || evidence.Title != "Paged specialist during triage" ||
		evidence.Situation != "On-call triage with dashboard review" ||
		evidence.Actions != "Paged the specialist after checking the dashboard" {
		t.Fatalf("draft: %+v", evidence)
	}
	if !strings.Contains(evidence.SourceNotes, "[response-1]") ||
		!strings.Contains(evidence.SourceNotes, "Did the specialist confirm the diagnosis?") {
		t.Fatalf("draft notes: %q", evidence.SourceNotes)
	}

	rec = careerRequest(t, a, http.MethodGet, "/api/career/discovery/sessions/"+id, nil)
	closed := careerDecode(t, rec)
	if closed["status"] != careerDiscoveryPromoted || closed["promoted_evidence_id"] != float64(evidenceID) {
		t.Fatalf("promoted session: %v", closed)
	}
	rec = careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/promote",
		map[string]any{"revision": closed["revision"].(float64)})
	if rec.Code != http.StatusConflict {
		t.Fatalf("second promote status %d", rec.Code)
	}

	// Explicit applicant review stays with the existing career_evidence owner.
	if err := a.updateCareerEvidence(evidenceID, evidence.Revision, evidence.content(), true); err != nil {
		t.Fatalf("review: %v", err)
	}
	reviewed, err := careerEvidenceByID(a.db, evidenceID)
	if err != nil || reviewed.Status != "reviewed" {
		t.Fatalf("reviewed: %+v %v", reviewed, err)
	}
}

func TestCareerDiscoveryInvalidOutputRecovery(t *testing.T) {
	a, _ := testWorkspace(t)
	view := careerCreateExcerpt(t, a, "Sam was an on-call engineer who triaged incidents and mentored juniors.")
	id := view["id"].(string)

	a.careerDiscoveryGenerate = careerStubScript(t, []string{"not json"})
	rec := careerAuthorize(t, a, view)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "invalid_output") {
		t.Fatalf("invalid output status %d: %s", rec.Code, rec.Body.String())
	}
	view = careerDecode(t, careerRequest(t, a, http.MethodGet, "/api/career/discovery/sessions/"+id, nil))
	if view["status"] != careerDiscoveryLeads || len(view["leads"].([]any)) != 0 {
		t.Fatalf("state not preserved: %v", view)
	}
	if view["failure"].(map[string]any)["code"] != "invalid_output" {
		t.Fatalf("failure: %v", view["failure"])
	}
	if view["provider"] != "" || view["model"] != "" {
		t.Fatalf("failed choice echoed: %v %v", view["provider"], view["model"])
	}

	a.careerDiscoveryGenerate = careerStubScript(t, []string{careerLeadsStub})
	rec = careerAuthorize(t, a, view)
	if rec.Code != http.StatusOK {
		t.Fatalf("retry status %d: %s", rec.Code, rec.Body.String())
	}
	recovered := careerDecode(t, rec)
	if recovered["status"] != careerDiscoveryInterview || recovered["failure"] != nil {
		t.Fatalf("recovered: %v", recovered)
	}
}

func TestCareerDiscoveryProviderFailureRecovery(t *testing.T) {
	a, _ := testWorkspace(t)
	view := careerCreateExcerpt(t, a, "Sam was an on-call engineer who triaged incidents and mentored juniors.")
	id := view["id"].(string)

	a.careerDiscoveryGenerate = func(context.Context, string, string) (string, error) {
		return "", errors.New("provider is unavailable")
	}
	rec := careerAuthorize(t, a, view)
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "provider_") {
		t.Fatalf("provider failure status %d: %s", rec.Code, rec.Body.String())
	}
	view = careerDecode(t, careerRequest(t, a, http.MethodGet, "/api/career/discovery/sessions/"+id, nil))
	if view["status"] != careerDiscoveryLeads {
		t.Fatalf("state not preserved: %v", view)
	}
	if view["provider"] != "" {
		t.Fatalf("failed provider echoed: %v", view["provider"])
	}

	a.careerDiscoveryGenerate = careerStubScript(t, []string{careerLeadsStub})
	rec = careerAuthorize(t, a, view)
	if rec.Code != http.StatusOK {
		t.Fatalf("retry status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCareerDiscoveryPromoteValidation(t *testing.T) {
	a, _ := testWorkspace(t)
	a.careerDiscoveryGenerate = careerStubScript(t, []string{careerLeadsStub})
	view := careerCreateExcerpt(t, a, "Sam was an on-call engineer who triaged incidents and mentored juniors.")
	view = careerDecode(t, careerAuthorize(t, a, view))
	rec := careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+view["id"].(string)+"/promote",
		map[string]any{"revision": view["revision"].(float64)})
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "no_proposal") {
		t.Fatalf("promote without proposal status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCareerDiscoveryTamperedProposalRejected(t *testing.T) {
	a, _ := testWorkspace(t)
	a.careerDiscoveryGenerate = careerStubScript(t, []string{careerLeadsStub, careerQuestionStub, careerProposalStub})
	view := careerCreateExcerpt(t, a, "Sam was an on-call engineer who triaged incidents and mentored juniors.")
	view = careerDecode(t, careerAuthorize(t, a, view))
	id := view["id"].(string)
	view = careerDecode(t, careerRequest(t, a, http.MethodGet, "/api/career/discovery/sessions/"+id, nil))
	rec := careerAuthorize(t, a, view)
	view = careerDecode(t, rec)
	rec = careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/answer",
		map[string]any{"answer": careerAnswerText, "revision": view["revision"].(float64)})
	view = careerDecode(t, rec)
	rec = careerAuthorize(t, a, view)
	if rec.Code != http.StatusOK {
		t.Fatalf("proposal status %d: %s", rec.Code, rec.Body.String())
	}
	var stateJSON string
	if err := a.db.QueryRow(`SELECT state_json FROM career_discovery_sessions WHERE id=?`, id).Scan(&stateJSON); err != nil {
		t.Fatal(err)
	}
	var state discovery.State
	if err := json.Unmarshal([]byte(stateJSON), &state); err != nil {
		t.Fatal(err)
	}
	state.Proposal.Quote = "fabricated words nowhere in the response"
	state.Proposal.Supports[0].Quote = "fabricated words nowhere in the response"
	tampered, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`UPDATE career_discovery_sessions SET state_json=? WHERE id=?`, string(tampered), id); err != nil {
		t.Fatal(err)
	}
	rec = careerRequest(t, a, http.MethodGet, "/api/career/discovery/sessions/"+id, nil)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "invalid_state") {
		t.Fatalf("tampered state status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCareerDiscoveryExpirySweep(t *testing.T) {
	a, _ := testWorkspace(t)
	view := careerCreateExcerpt(t, a, "Sam was an on-call engineer who triaged incidents and mentored juniors.")
	id := view["id"].(string)
	if _, err := a.db.Exec(`UPDATE career_discovery_sessions SET expires_at='2000-01-01T00:00:00Z' WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	rec := careerRequest(t, a, http.MethodGet, "/api/career/discovery/sessions/"+id, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expired status %d", rec.Code)
	}
	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM career_discovery_sessions WHERE id=?`, id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expired session not swept: %d", count)
	}
}

func TestCareerDiscoveryQuarantineFromEvidence(t *testing.T) {
	a, _ := testWorkspace(t)
	a.careerDiscoveryGenerate = careerStubScript(t, []string{careerLeadsStub})
	view := careerCreateExcerpt(t, a, "Sam was an on-call engineer who triaged incidents and mentored juniors.")
	view = careerDecode(t, careerAuthorize(t, a, view))
	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM career_evidence`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("interview wrote %d evidence rows before promotion", count)
	}
	// Sessions carry no foreign key into career_evidence.
	var sqlText string
	if err := a.db.QueryRow(`SELECT sql FROM sqlite_master WHERE name='career_discovery_sessions'`).Scan(&sqlText); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToUpper(sqlText), "REFERENCES") {
		t.Fatalf("session table has references: %s", sqlText)
	}
}

func TestCareerDiscoverySurvivesResumeRemoval(t *testing.T) {
	a, _ := testWorkspace(t)
	resume, err := a.importResume("sam.docx", bytes.NewReader(careerDOCX(t, "Sam Rivera triaged incidents weekly.")))
	if err != nil {
		t.Fatal(err)
	}
	rec := careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions",
		map[string]any{"resume_id": resume.ID})
	view := careerDecode(t, rec)
	id := view["id"].(string)
	if _, err := a.db.Exec(`DELETE FROM resumes WHERE id=?`, resume.ID); err != nil {
		t.Fatal(err)
	}
	view = careerDecode(t, careerRequest(t, a, http.MethodGet, "/api/career/discovery/sessions/"+id, nil))
	if view["resume"].(map[string]any)["removed"] != true {
		t.Fatalf("removed flag: %v", view["resume"])
	}
	if view["excerpt_hash"] == "" || view["excerpt_bytes"] == nil {
		t.Fatalf("excerpt summary lost: %v", view)
	}
	var storedState string
	if err := a.db.QueryRow(`SELECT state_json FROM career_discovery_sessions WHERE id=?`, id).Scan(&storedState); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(storedState, "triaged incidents") {
		t.Fatalf("stored excerpt lost: %s", storedState)
	}
	a.careerDiscoveryGenerate = careerStubScript(t, []string{careerLeadsStub})
	if rec := careerAuthorize(t, a, view); rec.Code != http.StatusOK {
		t.Fatalf("authorize after removal status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCareerDiscoveryRestartPersistence(t *testing.T) {
	a, dir := testWorkspace(t)
	a.careerDiscoveryGenerate = careerStubScript(t, []string{careerLeadsStub, careerQuestionStub, careerProposalStub})
	view := careerCreateExcerpt(t, a, "Sam was an on-call engineer who triaged incidents and mentored juniors.")
	view = careerDecode(t, careerAuthorize(t, a, view))
	id := view["id"].(string)
	view = careerDecode(t, careerAuthorize(t, a, view))
	rec := careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/answer",
		map[string]any{"answer": careerAnswerText, "revision": view["revision"].(float64)})
	view = careerDecode(t, rec)
	view = careerDecode(t, careerAuthorize(t, a, view))
	rec = careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/promote",
		map[string]any{"revision": view["revision"].(float64)})
	evidenceID := int64(careerDecode(t, rec)["evidence_id"].(float64))
	evidence, err := careerEvidenceByID(a.db, evidenceID)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.updateCareerEvidence(evidenceID, evidence.Revision, evidence.content(), true); err != nil {
		t.Fatal(err)
	}
	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.Close() })
	b := newTestApp(t, reopened, filepath.Join(dir, "resumes"))
	view = careerDecode(t, careerRequest(t, b, http.MethodGet, "/api/career/discovery/sessions/"+id, nil))
	if view["status"] != careerDiscoveryPromoted || len(view["responses"].([]any)) != 1 ||
		view["proposal"] == nil || view["promoted_evidence_id"] != float64(evidenceID) {
		t.Fatalf("reopened session: %v", view)
	}
	reviewed, err := careerEvidenceByID(reopened, evidenceID)
	if err != nil || reviewed.Status != "reviewed" || reviewed.Revision != 3 {
		t.Fatalf("reopened evidence: %+v %v", reviewed, err)
	}
	listed := careerDecode(t, careerRequest(t, b, http.MethodGet, "/api/career/discovery/sessions", nil))
	sessions := listed["sessions"].([]any)
	if len(sessions) != 1 || sessions[0].(map[string]any)["id"] != id {
		t.Fatalf("reopened list: %v", listed)
	}
}

func TestCareerDiscoveryMigrationPreservesEvidence(t *testing.T) {
	a, dir := testWorkspace(t)
	id, err := a.createCareerEvidenceDraft()
	if err != nil {
		t.Fatal(err)
	}
	content := careerEvidenceContent{Title: "Prior record", Situation: "s", Actions: "a", OutcomeState: "unknown"}
	if err := a.updateCareerEvidence(id, 1, content, false); err != nil {
		t.Fatal(err)
	}
	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.Close() })
	evidence, err := careerEvidenceByID(reopened, id)
	if err != nil || evidence.Title != "Prior record" || evidence.Revision != 2 {
		t.Fatalf("prior evidence: %+v %v", evidence, err)
	}
	var name string
	if err := reopened.QueryRow(`SELECT name FROM sqlite_master WHERE name='career_discovery_sessions'`).Scan(&name); err != nil {
		t.Fatalf("session table missing after reopen: %v", err)
	}
}

func TestCareerDiscoveryProvidersAPI(t *testing.T) {
	a, _ := testWorkspace(t)
	rec := careerRequest(t, a, http.MethodGet, "/api/career/discovery/providers", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("providers status %d", rec.Code)
	}
	providers := careerDecode(t, rec)["providers"].([]any)
	if len(providers) != 7 {
		t.Fatalf("providers: %v", providers)
	}
	first := providers[0].(map[string]any)
	if first["id"] != "ollama" || first["default_model"] == "" || first["execution"] == "" {
		t.Fatalf("ollama entry: %v", first)
	}
}

func TestCareerDiscoveryServeCareer(t *testing.T) {
	a, _ := testWorkspace(t)
	rec := careerRequest(t, a, http.MethodGet, "/career", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing bundle status %d", rec.Code)
	}
	a.resources.frontendIndex = []byte("<!doctype html><title>Career</title>")
	rec = careerRequest(t, a, http.MethodGet, "/career", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Career") {
		t.Fatalf("bundle status %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cache header: %v", rec.Header())
	}
}

func TestCareerDiscoveryLocalBoundary(t *testing.T) {
	a, _ := testWorkspace(t)
	req := httptest.NewRequest(http.MethodGet, "/api/career/discovery/sessions", nil)
	req.Host = "evil.example"
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign host status %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/career/discovery/sessions", nil)
	req.Host = "127.0.0.1:3000"
	req.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign origin status %d", rec.Code)
	}
}

func TestCareerDiscoveryGuard(t *testing.T) {
	var guard careerDiscoveryGuard
	if !guard.begin("a") || guard.begin("a") {
		t.Fatal("guard must serialize one session")
	}
	if !guard.begin("b") {
		t.Fatal("guard must allow distinct sessions")
	}
	guard.finish("a")
	if !guard.begin("a") {
		t.Fatal("guard must release after finish")
	}
}

func TestCareerDiscoveryResponsesExposeNoPromptOrExcerptBytes(t *testing.T) {
	a, _ := testWorkspace(t)
	const excerpt = "Zebra-Quasar sentinel excerpt bytes 7f3a9c-unique-never-echoed"
	a.careerDiscoveryGenerate = careerStubScript(t, []string{careerLeadsStub, careerQuestionStub, careerProposalStub})

	var bodies []string
	rec := careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions", map[string]any{"excerpt": excerpt})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", rec.Code, rec.Body.String())
	}
	bodies = append(bodies, rec.Body.String())
	view := careerDecode(t, rec)
	id := view["id"].(string)

	rec = careerRequest(t, a, http.MethodGet, "/api/career/discovery/sessions/"+id, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status %d: %s", rec.Code, rec.Body.String())
	}
	bodies = append(bodies, rec.Body.String())

	rec = careerAuthorize(t, a, view)
	if rec.Code != http.StatusOK {
		t.Fatalf("authorize leads status %d: %s", rec.Code, rec.Body.String())
	}
	bodies = append(bodies, rec.Body.String())
	view = careerDecode(t, rec)

	rec = careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/select",
		map[string]any{"lead_id": "lead-1", "revision": view["revision"].(float64)})
	if rec.Code != http.StatusOK {
		t.Fatalf("select status %d: %s", rec.Code, rec.Body.String())
	}
	bodies = append(bodies, rec.Body.String())
	view = careerDecode(t, rec)

	rec = careerAuthorize(t, a, view)
	if rec.Code != http.StatusOK {
		t.Fatalf("authorize question status %d: %s", rec.Code, rec.Body.String())
	}
	bodies = append(bodies, rec.Body.String())
	view = careerDecode(t, rec)

	rec = careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/answer",
		map[string]any{"answer": careerAnswerText, "revision": view["revision"].(float64)})
	if rec.Code != http.StatusOK {
		t.Fatalf("answer status %d: %s", rec.Code, rec.Body.String())
	}
	bodies = append(bodies, rec.Body.String())
	view = careerDecode(t, rec)

	rec = careerAuthorize(t, a, view)
	if rec.Code != http.StatusOK {
		t.Fatalf("authorize proposal status %d: %s", rec.Code, rec.Body.String())
	}
	bodies = append(bodies, rec.Body.String())

	rec = careerRequest(t, a, http.MethodGet, "/api/career/discovery/sessions", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d: %s", rec.Code, rec.Body.String())
	}
	bodies = append(bodies, rec.Body.String())

	for i, body := range bodies {
		if strings.Contains(body, excerpt) {
			t.Fatalf("body %d echoes excerpt bytes", i)
		}
		if strings.Contains(body, discovery.Policy) || strings.Contains(body, discovery.LeadsPolicy) {
			t.Fatalf("body %d echoes engine policy bytes", i)
		}
		for _, key := range []string{`"policy":`, `"transmit":`, `"excerpt":`} {
			if strings.Contains(body, key) {
				t.Fatalf("body %d contains forbidden key %s", i, key)
			}
		}
	}
	// The redacted contract keeps the opaque authorization binding and the
	// applicant-facing question; the client already holds its own excerpt.
	latest := careerDecode(t, careerRequest(t, a, http.MethodGet, "/api/career/discovery/sessions/"+id, nil))
	next := latest["next"].(map[string]any)
	if next["kind"] == "" || next["input_hash"] == "" || next["policy_name"] == "" {
		t.Fatalf("redacted next step lost opaque binding: %v", next)
	}
	if _, ok := next["policy"]; ok {
		t.Fatalf("redacted next step still carries policy: %v", next)
	}
	if _, ok := next["transmit"]; ok {
		t.Fatalf("redacted next step still carries transmit: %v", next)
	}
	if _, ok := latest["excerpt"]; ok {
		t.Fatalf("redacted view still carries excerpt")
	}
	if latest["excerpt_hash"] == "" || latest["excerpt_bytes"] == nil {
		t.Fatalf("redacted view lost excerpt summary: %v", latest)
	}
}

func TestCareerDiscoveryPromoteGuardSerializes(t *testing.T) {
	a, _ := testWorkspace(t)
	a.careerDiscoveryGenerate = careerStubScript(t, []string{careerLeadsStub, careerQuestionStub, careerProposalStub})
	view := careerCreateExcerpt(t, a, "Sam was an on-call engineer who triaged incidents and mentored juniors.")
	view = careerDecode(t, careerAuthorize(t, a, view))
	id := view["id"].(string)
	view = careerDecode(t, careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/select",
		map[string]any{"lead_id": "lead-1", "revision": view["revision"].(float64)}))
	view = careerDecode(t, careerAuthorize(t, a, view))
	view = careerDecode(t, careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/answer",
		map[string]any{"answer": careerAnswerText, "revision": view["revision"].(float64)}))
	view = careerDecode(t, careerAuthorize(t, a, view))

	// A second operator (or tab) promoting while one promotion is
	// in-flight gets 409 and creates no draft.
	if !a.careerDiscovery.begin(id) {
		t.Fatal("setup guard not acquired")
	}
	rec := careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/promote",
		map[string]any{"revision": view["revision"].(float64)})
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "promotion_in_progress") {
		a.careerDiscovery.finish(id)
		t.Fatalf("guarded promote status %d: %s", rec.Code, rec.Body.String())
	}
	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM career_evidence`).Scan(&count); err != nil {
		a.careerDiscovery.finish(id)
		t.Fatal(err)
	}
	if count != 0 {
		a.careerDiscovery.finish(id)
		t.Fatalf("guarded promote created %d drafts", count)
	}
	a.careerDiscovery.finish(id)

	rec = careerRequest(t, a, http.MethodPost, "/api/career/discovery/sessions/"+id+"/promote",
		map[string]any{"revision": view["revision"].(float64)})
	if rec.Code != http.StatusOK {
		t.Fatalf("promote status %d: %s", rec.Code, rec.Body.String())
	}
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM career_evidence`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("promote created %d drafts", count)
	}
}

func TestCareerDiscoveryUnknownSession(t *testing.T) {
	a, _ := testWorkspace(t)
	rec := careerRequest(t, a, http.MethodGet, "/api/career/discovery/sessions/zzz", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("malformed id status %d", rec.Code)
	}
	rec = careerRequest(t, a, http.MethodGet, "/api/career/discovery/sessions/"+strings.Repeat("a", 64), nil)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "unknown_session") {
		t.Fatalf("absent id status %d: %s", rec.Code, rec.Body.String())
	}
}


