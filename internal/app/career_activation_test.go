package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	discovery "github.com/jenksed/yourmove-career-discovery"
	"github.com/jenksed/yourmove-go/internal/application/schema"
	"github.com/jenksed/yourmove-go/internal/storage"
)

// Synthetic recorded interview responses. Quotes in test traces must stay
// verbatim excerpts of these strings.
const (
	activationResponse1 = "EXPLAIN showed a full table scan for a frequently used query. I recommended adding an index and handed the change to the DBA team for review."
	activationResponse2 = "The DBA confirmed the diagnosis the next morning and scheduled the index change. I do not know whether query times improved after that."
)

const activationSecondPosting = "https://jobs.ashbyhq.com/render/a1b2c3d4-e5f6-4789-abcd-ef0123456789"

// activationEvidence creates one reviewed record, optionally bound to a
// promoted interview session holding the two synthetic recorded responses.
func activationEvidence(t *testing.T, a *App, withInterview bool) CareerEvidence {
	t.Helper()
	id, err := a.createCareerEvidenceDraft()
	if err != nil {
		t.Fatal(err)
	}
	content := careerEvidenceContent{
		Title: "Synthetic MySQL investigation", Situation: "Synthetic queries slowed down.",
		Actions:      "I reviewed EXPLAIN plans and recommended an index.",
		OutcomeState: "unknown", Technologies: "MySQL",
	}
	if err := a.updateCareerEvidence(id, 1, content, true); err != nil {
		t.Fatal(err)
	}
	if withInterview {
		state := discovery.State{
			Resume:         discovery.Source{ID: "resume", Kind: "document_excerpt", Text: "Synthetic resume excerpt about MySQL support."},
			Leads:          []discovery.Lead{{ID: "lead-1", Title: "MySQL investigation", Why: "The excerpt hints at diagnosis worth documenting."}},
			SelectedLeadID: "lead-1",
			Responses: []discovery.Response{
				{Question: "What did the slow query look like?", Source: discovery.Source{ID: "response-1", Kind: "self_report", Text: activationResponse1}},
				{Question: "What happened after the handoff?", Source: discovery.Source{ID: "response-2", Kind: "self_report", Text: activationResponse2}},
			},
		}
		raw, err := json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		sessionID, err := careerDiscoverySessionID()
		if err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		expires := time.Now().Add(careerDiscoverySessionTTL).UTC().Format(time.RFC3339Nano)
		if _, err := a.db.Exec(`INSERT INTO career_discovery_sessions
			(id,state_json,status,provider,model,promoted_evidence_id,revision,created_at,updated_at,expires_at)
			VALUES(?,?,'promoted','ollama','stub',?,1,?,?,?)`,
			sessionID, string(raw), id, now, now, expires); err != nil {
			t.Fatal(err)
		}
	}
	evidence, err := careerEvidenceByID(a.db, id)
	if err != nil {
		t.Fatal(err)
	}
	return evidence
}

func activationSecondJob(t *testing.T, a *App) (Job, string) {
	t.Helper()
	job := Job{URL: activationSecondPosting, Host: "jobs.ashbyhq.com", Title: "Customer Success Engineer", Company: "Fixture Systems"}
	if err := a.saveJob(job); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(2)
	if err != nil {
		t.Fatal(err)
	}
	field := schema.Field{ID: "browser:handoff-story", Label: "Describe how you communicated a technical finding to another team", Kind: "textarea"}
	if _, err := a.saveInspection(job.ID, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: job.URL + "/application", Fields: []schema.Field{field}}); err != nil {
		t.Fatal(err)
	}
	return job, questionKey(job.URL, "ashby", field)
}

func activationSelection(evidence CareerEvidence) string {
	return fmt.Sprintf("%d:%d:%s", evidence.ID, evidence.Revision, evidence.CreatedAt)
}

func activationForm(job Job, key string, evidence CareerEvidence) url.Values {
	return url.Values{"destination": {job.URL}, "question_key": {key}, "revision": {"0"},
		"provider": {"ollama"}, "model": {"synthetic-model"}, "career_evidence": {activationSelection(evidence)}}
}

func activationEnvelope(t *testing.T, trace discovery.AnswerTrace) string {
	t.Helper()
	raw, err := json.Marshal(trace)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestActivationTraceSourcesBindReviewedRevision(t *testing.T) {
	a, _, _, _, _ := newAnswerLibraryFixture(t)
	interviewed := activationEvidence(t, a, true)
	plain := activationEvidence(t, a, false)

	sources, err := a.careerEvidenceTraceSources(interviewed)
	if err != nil || len(sources) != 2 {
		t.Fatalf("trace sources: %v %+v", err, sources)
	}
	for i, source := range sources {
		if source.EvidenceID != interviewed.ID || source.EvidenceRevision != interviewed.Revision || source.ResponseID != fmt.Sprintf("response-%d", i+1) {
			t.Fatalf("source %d binds %+v", i, source)
		}
	}
	if sources[0].ResponseText != activationResponse1 || sources[1].ResponseText != activationResponse2 {
		t.Fatal("sources do not carry the recorded responses")
	}
	if sources, err := a.careerEvidenceTraceSources(plain); err != nil || sources != nil {
		t.Fatalf("record without interview: %v %+v", err, sources)
	}

	// A corrected and re-reviewed record rebinds the same interview to the
	// new revision; the old revision no longer resolves.
	corrected := interviewed.content()
	corrected.Situation += " Corrected detail."
	if err := a.updateCareerEvidence(interviewed.ID, interviewed.Revision, corrected, true); err != nil {
		t.Fatal(err)
	}
	updated, err := careerEvidenceByID(a.db, interviewed.ID)
	if err != nil {
		t.Fatal(err)
	}
	sources, err = a.careerEvidenceTraceSources(updated)
	if err != nil || len(sources) != 2 || sources[0].EvidenceRevision != updated.Revision {
		t.Fatalf("rebind after correction: %v %+v", err, sources)
	}

	// A tampered session refuses generation instead of advertising a trace
	// that no longer resolves.
	if _, err := a.db.Exec(`UPDATE career_discovery_sessions SET state_json='{"responses":[]}' WHERE promoted_evidence_id=?`, interviewed.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.careerEvidenceTraceSources(updated); err == nil {
		t.Fatal("tampered session accepted")
	}
}

func TestActivationAnswerTraceRoundTrip(t *testing.T) {
	a, _, job, _, key := newAnswerLibraryFixture(t)
	evidence := activationEvidence(t, a, true)
	v := activationForm(job, key, evidence)

	rec := postApp(newMux(a), "/jobs/1/answers/ai/review", v, "", "")
	if rec.Code != 200 {
		t.Fatalf("review: %d %s", rec.Code, rec.Body.String())
	}
	var reviewed struct {
		Ticket       string
		Instructions string
		Prompt       string
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &reviewed); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Return one JSON object only", "interview_responses", activationResponse1} {
		if !strings.Contains(reviewed.Prompt, want) && !strings.Contains(reviewed.Instructions, want) {
			t.Fatalf("review hides %q", want)
		}
	}
	if strings.Contains(reviewed.Instructions, "Return only the answer, or exactly") {
		t.Fatal("trace snapshot kept the prose-only return directive")
	}

	answer := "I used EXPLAIN to find a full table scan and recommended adding an index for DBA review."
	a.applicationAnswerGenerate = func(context.Context, string, string) (string, error) {
		return activationEnvelope(t, discovery.AnswerTrace{Answer: answer,
			Claims: []discovery.AnswerClaim{
				{Claim: "used EXPLAIN to find a full table scan", EvidenceID: evidence.ID, EvidenceRevision: evidence.Revision, ResponseID: "response-1", Quote: "EXPLAIN showed a full table scan for a frequently used query"},
				{Claim: "recommended adding an index", EvidenceID: evidence.ID, EvidenceRevision: evidence.Revision, ResponseID: "response-1", Quote: "I recommended adding an index"},
			},
			Unresolved: []string{"High-availability failover experience", "Measured query improvement after the index change"}}), nil
	}
	generated := postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(reviewed.Ticket), "", "")
	if generated.Code != 200 {
		t.Fatalf("generate: %d %s", generated.Code, generated.Body.String())
	}
	var result struct {
		State     string
		Answer    string
		Adoptable bool
		Trace     *discovery.AnswerTrace
	}
	if err := json.Unmarshal(generated.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.State != "complete" || !result.Adoptable || result.Answer != answer {
		t.Fatalf("result: %+v", result)
	}
	if result.Trace == nil || len(result.Trace.Claims) != 2 || len(result.Trace.Unresolved) != 2 {
		t.Fatalf("trace missing: %+v", result.Trace)
	}
	claim := result.Trace.Claims[0]
	if claim.EvidenceID != evidence.ID || claim.EvidenceRevision != evidence.Revision || claim.ResponseID != "response-1" {
		t.Fatalf("claim binding: %+v", claim)
	}

	checked := postApp(newMux(a), "/jobs/1/answers/ai/adopt-check",
		url.Values{"ticket": {reviewed.Ticket}, "question_key": {key}, "revision": {"0"}}, "", "")
	if checked.Code != 200 {
		t.Fatalf("adopt-check: %d %s", checked.Code, checked.Body.String())
	}
	var adoption struct {
		Answer    string
		Adoptable bool
		Trace     *discovery.AnswerTrace
	}
	if err := json.Unmarshal(checked.Body.Bytes(), &adoption); err != nil {
		t.Fatal(err)
	}
	if !adoption.Adoptable || adoption.Answer != answer || adoption.Trace == nil || len(adoption.Trace.Unresolved) != 2 {
		t.Fatalf("adoption: %s", checked.Body.String())
	}

	// Explicit adoption saves through the unchanged answer owner.
	saveLibrarySourceAnswer(t, a, job, key, answer, 0)
	flags := a.savedAnswerProvenance(job)
	if len(flags) != 1 || flags[0].State != "grounded" {
		t.Fatalf("provenance: %+v", flags)
	}
}

func TestActivationCanceledTicketCompletionPreservesProvenance(t *testing.T) {
	for _, priorPins := range []bool{false, true} {
		t.Run(fmt.Sprintf("prior_pins=%t", priorPins), func(t *testing.T) {
			a, _, job, _, key := newAnswerLibraryFixture(t)
			evidence := activationEvidence(t, a, true)
			ticket := reviewNarrative(t, a, activationForm(job, key, evidence))
			snapshot := a.answerAI.tickets[ticket].snapshot
			if priorPins {
				a.recordAnswerProvenance(snapshot, answerAIHash("Previously retained answer"), "prior-pin-time")
			}
			pins := func() string {
				t.Helper()
				var raw string
				if err := a.db.QueryRow(`SELECT json_group_array(json_array(job_id,destination,question_key,evidence_id,evidence_rev,evidence_created,answer_hash,created_at)) FROM ai_answer_provenance`).Scan(&raw); err != nil {
					t.Fatal(err)
				}
				return raw
			}
			before := pins()
			if priorPins == (before == "[]") {
				t.Fatalf("unexpected initial pins: %s", before)
			}
			a.applicationAnswerGenerate = func(context.Context, string, string) (string, error) {
				// Model a cancellation that wins the ticket lock after output
				// acceptance, without relying on context cancellation to set final.
				a.answerAI.mu.Lock()
				a.answerAI.tickets[ticket].state = "canceled"
				a.answerAI.mu.Unlock()
				return activationEnvelope(t, discovery.AnswerTrace{
					Answer: "I used EXPLAIN to find a full table scan.",
					Claims: []discovery.AnswerClaim{{Claim: "used EXPLAIN to find a full table scan", EvidenceID: evidence.ID, EvidenceRevision: evidence.Revision, ResponseID: "response-1", Quote: "EXPLAIN showed a full table scan for a frequently used query"}},
				}), nil
			}
			generated := postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(ticket), "", "")
			if generated.Code != 200 || !strings.Contains(generated.Body.String(), `"state":"canceled"`) {
				t.Fatalf("canceled completion: %d %s", generated.Code, generated.Body.String())
			}
			if after := pins(); after != before {
				t.Fatalf("canceled completion changed pins: before %s; after %s", before, after)
			}
			if got := a.answerAI.tickets[ticket]; got.answer != "" || got.trace != nil || got.active {
				t.Fatalf("canceled ticket retained output: %+v", got)
			}
		})
	}
}

func TestActivationRejectsFabricatedReferences(t *testing.T) {
	a, _, job, _, key := newAnswerLibraryFixture(t)
	evidence := activationEvidence(t, a, true)
	answer := "I used EXPLAIN to find a full table scan and recommended adding an index for DBA review."
	good := discovery.AnswerTrace{Answer: answer,
		Claims: []discovery.AnswerClaim{
			{Claim: "used EXPLAIN to find a full table scan", EvidenceID: evidence.ID, EvidenceRevision: evidence.Revision, ResponseID: "response-1", Quote: "EXPLAIN showed a full table scan for a frequently used query"},
		},
		Unresolved: []string{"High-availability failover experience"}}
	cases := []struct {
		name  string
		raw   string
		stage string
	}{
		{"prose", "I used EXPLAIN and found a full table scan.", "trace_parse"},
		{"truncated", `{"answer":"I used EXPLAIN`, "trace_parse"},
		{"unknown-field", `{"answer":"a","claims":[],"unresolved":[],"reasoning":"private"}`, "trace_parse"},
	}
	mutate := []struct {
		name  string
		edit  func(*discovery.AnswerTrace)
		stage string
	}{
		{"stale-revision", func(trace *discovery.AnswerTrace) { trace.Claims[0].EvidenceRevision++ }, "trace_identity"},
		{"missing-response", func(trace *discovery.AnswerTrace) { trace.Claims[0].ResponseID = "response-9" }, "trace_identity"},
		{"wrong-evidence", func(trace *discovery.AnswerTrace) { trace.Claims[0].EvidenceID++ }, "trace_identity"},
		{"altered-quote", func(trace *discovery.AnswerTrace) {
			trace.Claims[0].Quote = "EXPLAIN showed a full table scan for every query"
		}, "trace_quote"},
		{"claim-not-in-answer", func(trace *discovery.AnswerTrace) { trace.Claims[0].Claim = "I implemented the fix" }, "trace_claim"},
		{"empty-claims", func(trace *discovery.AnswerTrace) { trace.Claims = nil }, "trace_shape"},
		{"duplicate", func(trace *discovery.AnswerTrace) { trace.Claims = append(trace.Claims, trace.Claims[0]) }, "trace_shape"},
		{"unresolved-overflow", func(trace *discovery.AnswerTrace) {
			trace.Unresolved = []string{"a", "b", "c", "d", "e", "f", "g"}
		}, "trace_unresolved"},
		{"disallowed-answer", func(trace *discovery.AnswerTrace) {
			trace.Answer = "Assistant: " + trace.Answer
			trace.Claims[0].Claim = "Assistant: used EXPLAIN to find a full table scan"
		}, "answer_output"},
	}
	for _, tc := range mutate {
		cloned := good
		cloned.Claims = append([]discovery.AnswerClaim(nil), good.Claims...)
		cloned.Unresolved = append([]string(nil), good.Unresolved...)
		tc.edit(&cloned)
		raw, err := json.Marshal(cloned)
		if err != nil {
			t.Fatal(err)
		}
		cases = append(cases, struct {
			name  string
			raw   string
			stage string
		}{tc.name, string(raw), tc.stage})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticket := reviewNarrative(t, a, activationForm(job, key, evidence))
			a.applicationAnswerGenerate = func(context.Context, string, string) (string, error) { return tc.raw, nil }
			rec := postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(ticket), "", "")
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			var failure struct {
				Code          string
				FailureStage  string `json:"failure_stage"`
				FailureDetail string `json:"failure_detail"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &failure); err != nil {
				t.Fatal(err)
			}
			if failure.Code != "output" || failure.FailureStage != tc.stage || failure.FailureDetail == "" {
				t.Fatalf("failure: %s", rec.Body.String())
			}
			// Rejected model output is never retained: neither the claim nor
			// the recorded quote may appear in the failure response.
			for _, leaked := range []string{"full table scan", "recommended adding", "EXPLAIN showed", "Assistant:"} {
				if strings.Contains(rec.Body.String(), leaked) {
					t.Fatalf("failure response retains model text: %s", rec.Body.String())
				}
			}
			a.answerAI.mu.Lock()
			stored := a.answerAI.tickets[ticket]
			a.answerAI.mu.Unlock()
			if stored.answer != "" || stored.trace != nil {
				t.Fatal("rejected output retained on ticket")
			}
		})
	}
}

func TestActivationTwoApplicationsShareHistory(t *testing.T) {
	a, _, first, _, firstKey := newAnswerLibraryFixture(t)
	second, secondKey := activationSecondJob(t, a)
	evidence := activationEvidence(t, a, true)

	firstAnswer := "I used EXPLAIN to find a full table scan and recommended adding an index for DBA review."
	a.applicationAnswerGenerate = func(context.Context, string, string) (string, error) {
		return activationEnvelope(t, discovery.AnswerTrace{Answer: firstAnswer,
			Claims: []discovery.AnswerClaim{
				{Claim: "used EXPLAIN to find a full table scan", EvidenceID: evidence.ID, EvidenceRevision: evidence.Revision, ResponseID: "response-1", Quote: "EXPLAIN showed a full table scan for a frequently used query"},
			},
			Unresolved: []string{"Measured query improvement after the index change"}}), nil
	}
	firstTicket := reviewNarrative(t, a, activationForm(first, firstKey, evidence))
	if rec := postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(firstTicket), "", ""); rec.Code != 200 {
		t.Fatalf("first generate: %d %s", rec.Code, rec.Body.String())
	}
	saveLibrarySourceAnswer(t, a, first, firstKey, firstAnswer, 0)

	secondAnswer := "I handed the change to the DBA team for review, and the DBA confirmed the diagnosis the next morning."
	a.applicationAnswerGenerate = func(context.Context, string, string) (string, error) {
		return activationEnvelope(t, discovery.AnswerTrace{Answer: secondAnswer,
			Claims: []discovery.AnswerClaim{
				{Claim: "handed the change to the DBA team for review", EvidenceID: evidence.ID, EvidenceRevision: evidence.Revision, ResponseID: "response-1", Quote: "handed the change to the DBA team for review"},
				{Claim: "the DBA confirmed the diagnosis the next morning", EvidenceID: evidence.ID, EvidenceRevision: evidence.Revision, ResponseID: "response-2", Quote: "The DBA confirmed the diagnosis the next morning"},
			},
			Unresolved: []string{"Whether query times improved after the index change"}}), nil
	}
	reviewed := postApp(newMux(a), "/jobs/2/answers/ai/review", activationForm(second, secondKey, evidence), "", "")
	if reviewed.Code != 200 {
		t.Fatalf("second review: %d %s", reviewed.Code, reviewed.Body.String())
	}
	var secondReview struct {
		Ticket string
	}
	if err := json.Unmarshal(reviewed.Body.Bytes(), &secondReview); err != nil {
		t.Fatal(err)
	}
	if rec := postApp(newMux(a), "/jobs/2/answers/ai/generate", generationForm(secondReview.Ticket), "", ""); rec.Code != 200 {
		t.Fatalf("second generate: %d %s", rec.Code, rec.Body.String())
	}
	saveLibrarySourceAnswer(t, a, second, secondKey, secondAnswer, 0)

	if firstAnswer == secondAnswer {
		t.Fatal("applications did not use the history differently")
	}
	for _, job := range []Job{first, second} {
		flags := a.savedAnswerProvenance(job)
		if len(flags) != 1 || flags[0].State != "grounded" {
			t.Fatalf("job %d provenance: %+v", job.ID, flags)
		}
	}
	usage, err := a.careerActivationUsage()
	if err != nil {
		t.Fatal(err)
	}
	if len(usage) != 1 || len(usage[0]["answers"].([]map[string]any)) != 2 {
		t.Fatalf("activation usage: %+v", usage)
	}
	if usage[0]["interview"] == nil {
		t.Fatal("activation hides the recorded interview")
	}
}

func TestActivationCorrectionFlagsWithoutMutation(t *testing.T) {
	a, _, job, _, key := newAnswerLibraryFixture(t)
	evidence := activationEvidence(t, a, true)
	answer := "I used EXPLAIN to find a full table scan and recommended adding an index for DBA review."
	a.applicationAnswerGenerate = func(context.Context, string, string) (string, error) {
		return activationEnvelope(t, discovery.AnswerTrace{Answer: answer,
			Claims: []discovery.AnswerClaim{
				{Claim: "used EXPLAIN to find a full table scan", EvidenceID: evidence.ID, EvidenceRevision: evidence.Revision, ResponseID: "response-1", Quote: "EXPLAIN showed a full table scan for a frequently used query"},
			}}), nil
	}
	ticket := reviewNarrative(t, a, activationForm(job, key, evidence))
	if rec := postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(ticket), "", ""); rec.Code != 200 {
		t.Fatalf("generate: %d %s", rec.Code, rec.Body.String())
	}
	saveLibrarySourceAnswer(t, a, job, key, answer, 0)

	corrected := evidence.content()
	corrected.Actions += " Corrected wording."
	if err := a.updateCareerEvidence(evidence.ID, evidence.Revision, corrected, true); err != nil {
		t.Fatal(err)
	}

	var saved string
	if err := a.db.QueryRow(`SELECT answer FROM application_answers WHERE job_id=1 AND question_key=?`, key).Scan(&saved); err != nil || saved != answer {
		t.Fatalf("saved answer mutated: %q %v", saved, err)
	}
	flags := a.savedAnswerProvenance(job)
	if len(flags) != 1 || flags[0].State != "needs-review" || !strings.Contains(flags[0].Detail, "Text unchanged") {
		t.Fatalf("provenance: %+v", flags)
	}
	// The completed ticket no longer adopts: its basis changed.
	adopt := postApp(newMux(a), "/jobs/1/answers/ai/adopt-check",
		url.Values{"ticket": {ticket}, "question_key": {key}, "revision": {"0"}}, "", "")
	if adopt.Code != http.StatusConflict {
		t.Fatalf("stale adopt-check: %d %s", adopt.Code, adopt.Body.String())
	}
	// The old selection identity no longer reviews.
	stale := activationForm(job, key, evidence)
	stale.Set("revision", "1")
	if rec := postApp(newMux(a), "/jobs/1/answers/ai/review", stale, "", ""); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("old selection: %d %s", rec.Code, rec.Body.String())
	}

	// Editing the saved answer reports edited, not grounded.
	saveLibrarySourceAnswer(t, a, job, key, answer+" Edited.", 1)
	if flags := a.savedAnswerProvenance(job); len(flags) != 1 || flags[0].State != "edited" {
		t.Fatalf("edited provenance: %+v", flags)
	}
}

func TestActivationLetterEvidenceGroundsAndPinsRefs(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/role", Host: "example.com", Title: "Support Engineer", Company: "Example", Description: "Work on Linux incidents and MySQL queries"}); err != nil {
		t.Fatal(err)
	}
	_, role := activationResumes(t, a)
	evidence := activationEvidence(t, a, true)
	restrained := "Dear Hiring Manager, I investigated slow MySQL queries with EXPLAIN and handed a recommended index to the DBA team for review. The eventual outcome is unknown to me. I would bring the same careful diagnosis to this role. Sincerely, Applicant"
	var prompt string
	a.coverLetterGenerate = func(_ context.Context, received string) (string, error) {
		prompt = received
		return restrained, nil
	}
	rec := postApp(newMux(a), "/jobs/1/cover-letter/generate", url.Values{
		"provider": {"ollama"}, "model": {"synthetic-model"}, "letter_resume_id": {strconv.FormatInt(role.ID, 10)},
		"confirm_local": {"yes"}, "career_evidence": {activationSelection(evidence)},
	}, "", "")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != coverNoticeURL(1, "generated") {
		t.Fatalf("generate: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	for _, want := range []string{"APPLICANT-REVIEWED CAREER EVIDENCE", "Outcome: unknown (not established as a result)",
		"Candidate statement 1 (verbatim excerpt)", "EXPLAIN showed a full table scan", "Synthetic MySQL investigation"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt hides %q", want)
		}
	}
	if !strings.HasSuffix(prompt, a.resources.coverLetterInstructions) {
		t.Fatal("evidence prompt broke the canonical instruction suffix")
	}
	draft, err := a.latestCoverLetter(1)
	if err != nil || draft.Body != restrained {
		t.Fatalf("draft: %+v %v", draft, err)
	}
	for _, claimed := range []string{"improved", "reduced", "saved", "cut ", "%", "owned the fix"} {
		if strings.Contains(strings.ToLower(draft.Body), claimed) {
			t.Fatalf("letter claims an unestablished result: %q", claimed)
		}
	}
	refs, stale := a.coverLetterEvidenceStatus(draft)
	if stale || len(refs) != 1 || refs[0].ID != evidence.ID || refs[0].Revision != evidence.Revision || refs[0].CreatedAt != evidence.CreatedAt {
		t.Fatalf("refs: %+v stale=%v", refs, stale)
	}

	// A supporting-evidence correction flags the draft without rewriting it.
	corrected := evidence.content()
	corrected.Actions += " Corrected wording."
	if err := a.updateCareerEvidence(evidence.ID, evidence.Revision, corrected, true); err != nil {
		t.Fatal(err)
	}
	draft, err = a.latestCoverLetter(1)
	if err != nil || draft.Body != restrained {
		t.Fatalf("draft mutated: %+v %v", draft, err)
	}
	if _, stale := a.coverLetterEvidenceStatus(draft); !stale {
		t.Fatal("corrected evidence left the draft current")
	}
}

func TestActivationLetterRejectsBadSelections(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/role", Host: "example.com", Title: "Support Engineer", Company: "Example", Description: "Work on Linux incidents and MySQL queries"}); err != nil {
		t.Fatal(err)
	}
	_, role := activationResumes(t, a)
	reviewed := activationEvidence(t, a, false)
	draftID, err := a.createCareerEvidenceDraft()
	if err != nil {
		t.Fatal(err)
	}
	draft, err := careerEvidenceByID(a.db, draftID)
	if err != nil {
		t.Fatal(err)
	}
	second := activationEvidence(t, a, false)
	third := activationEvidence(t, a, false)
	form := func(selections ...string) url.Values {
		return url.Values{"provider": {"ollama"}, "model": {"synthetic-model"},
			"letter_resume_id": {strconv.FormatInt(role.ID, 10)}, "confirm_local": {"yes"}, "career_evidence": selections}
	}
	for _, selections := range [][]string{
		{activationSelection(draft)},
		{fmt.Sprintf("%d:%d:%s", reviewed.ID, reviewed.Revision+1, reviewed.CreatedAt)},
		{activationSelection(reviewed), activationSelection(second), activationSelection(third)},
		{activationSelection(reviewed), activationSelection(reviewed)},
		{"not-an-identity"},
	} {
		rec := postApp(newMux(a), "/jobs/1/cover-letter/generate", form(selections...), "", "")
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != coverNoticeURL(1, "sources") {
			t.Fatalf("selections %v: %d %s", selections, rec.Code, rec.Header().Get("Location"))
		}
	}
	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM cover_letters WHERE job_id=1`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rejected selections saved %d drafts: %v", count, err)
	}

	// Letters without evidence keep the legacy contract: empty refs, current.
	a.coverLetterGenerate = func(context.Context, string) (string, error) {
		return "Dear Hiring Manager, I support production services and document reliable troubleshooting steps. Sincerely, Applicant", nil
	}
	rec := postApp(newMux(a), "/jobs/1/cover-letter/generate", url.Values{
		"provider": {"ollama"}, "model": {"synthetic-model"},
		"letter_resume_id": {strconv.FormatInt(role.ID, 10)}, "confirm_local": {"yes"},
	}, "", "")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != coverNoticeURL(1, "generated") {
		t.Fatalf("legacy generate: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	legacy, err := a.latestCoverLetter(1)
	if err != nil {
		t.Fatal(err)
	}
	if refs, stale := a.coverLetterEvidenceStatus(legacy); refs != nil || stale {
		t.Fatalf("legacy refs: %+v stale=%v", refs, stale)
	}
}

func activationResumes(t *testing.T, a *App) (Resume, Resume) {
	t.Helper()
	master, err := a.importResume("master.docx", bytes.NewReader(sampleDOCXForPreview(t, "Managed production incidents")))
	if err != nil {
		t.Fatal(err)
	}
	role, err := a.importResume("role.docx", bytes.NewReader(sampleDOCXForPreview(t, "Linux and MySQL troubleshooting")))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.setMasterResume(master.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, role.ID); err != nil {
		t.Fatal(err)
	}
	return master, role
}

func TestActivationSurvivesRestart(t *testing.T) {
	a, dir := testWorkspace(t)
	job := Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Support Engineer", Company: "Example", Description: "Work on Linux incidents and MySQL queries"}
	if err := a.saveJob(job); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	_, roleResume := activationResumes(t, a)
	evidence := activationEvidence(t, a, true)
	answer := "I used EXPLAIN to find a full table scan and recommended adding an index for DBA review."
	a.applicationAnswerGenerate = func(context.Context, string, string) (string, error) {
		return activationEnvelope(t, discovery.AnswerTrace{Answer: answer,
			Claims: []discovery.AnswerClaim{
				{Claim: "used EXPLAIN to find a full table scan", EvidenceID: evidence.ID, EvidenceRevision: evidence.Revision, ResponseID: "response-1", Quote: "EXPLAIN showed a full table scan for a frequently used query"},
			}}), nil
	}
	field := schema.Field{ID: "browser:incident-story", Label: answerLibraryStory, Kind: "textarea"}
	if _, err := a.saveInspection(job.ID, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: job.URL + "/application", Fields: []schema.Field{field}}); err != nil {
		t.Fatal(err)
	}
	_ = field
	key := questionKey(job.URL, "ashby", schema.Field{ID: "browser:incident-story", Label: answerLibraryStory, Kind: "textarea"})
	ticket := reviewNarrative(t, a, activationForm(job, key, evidence))
	if rec := postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(ticket), "", ""); rec.Code != 200 {
		t.Fatalf("generate: %d %s", rec.Code, rec.Body.String())
	}
	saveLibrarySourceAnswer(t, a, job, key, answer, 0)
	letter := "Dear Hiring Manager, I investigated slow MySQL queries with EXPLAIN and handed a recommended index to the DBA team. Sincerely, Applicant"
	a.coverLetterGenerate = func(context.Context, string) (string, error) { return letter, nil }
	if rec := postApp(newMux(a), "/jobs/1/cover-letter/generate", url.Values{
		"provider": {"ollama"}, "model": {"synthetic-model"},
		"letter_resume_id": {strconv.FormatInt(roleResume.ID, 10)}, "confirm_local": {"yes"},
		"career_evidence": {activationSelection(evidence)},
	}, "", ""); rec.Code != http.StatusSeeOther {
		t.Fatalf("letter: %d %s", rec.Code, rec.Body.String())
	}

	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	restartedDB, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	restarted := newTestApp(t, restartedDB, filepath.Join(dir, "resumes"))
	restartedJob, err := restarted.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	flags := restarted.savedAnswerProvenance(restartedJob)
	if len(flags) != 1 || flags[0].State != "grounded" {
		t.Fatalf("provenance after restart: %+v", flags)
	}
	draft, err := restarted.latestCoverLetter(1)
	if err != nil || draft.Body != letter {
		t.Fatalf("letter after restart: %+v %v", draft, err)
	}
	if refs, stale := restarted.coverLetterEvidenceStatus(draft); stale || len(refs) != 1 {
		t.Fatalf("letter refs after restart: %+v stale=%v", refs, stale)
	}
	usage, err := restarted.careerActivationUsage()
	if err != nil {
		t.Fatal(err)
	}
	if len(usage) != 1 || usage[0]["interview"] == nil {
		t.Fatalf("activation after restart: %+v", usage)
	}
	if answers := usage[0]["answers"].([]map[string]any); len(answers) != 1 || answers[0]["state"] != "grounded" {
		t.Fatalf("activation answers: %+v", answers)
	}
	if letters := usage[0]["letters"].([]map[string]any); len(letters) != 1 || letters[0]["stale"] != false {
		t.Fatalf("activation letters: %+v", letters)
	}
}

func TestActivationRedactsBodiesAndExcerpts(t *testing.T) {
	a, _, job, _, key := newAnswerLibraryFixture(t)
	evidence := activationEvidence(t, a, true)
	answer := "I used EXPLAIN to find a full table scan and recommended adding an index for DBA review."
	a.applicationAnswerGenerate = func(context.Context, string, string) (string, error) {
		return activationEnvelope(t, discovery.AnswerTrace{Answer: answer,
			Claims: []discovery.AnswerClaim{
				{Claim: "used EXPLAIN to find a full table scan", EvidenceID: evidence.ID, EvidenceRevision: evidence.Revision, ResponseID: "response-1", Quote: "EXPLAIN showed a full table scan for a frequently used query"},
			}}), nil
	}
	ticket := reviewNarrative(t, a, activationForm(job, key, evidence))
	if rec := postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(ticket), "", ""); rec.Code != 200 {
		t.Fatalf("generate: %d %s", rec.Code, rec.Body.String())
	}
	saveLibrarySourceAnswer(t, a, job, key, answer, 0)

	rec := careerRequest(t, a, http.MethodGet, "/api/career/activation", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("activation: %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, leaked := range []string{answer, activationResponse1, activationResponse2, "interview_responses", "response_text", "YourMove application-answer policy"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("activation leaks %q", leaked)
		}
	}
	var decoded struct {
		Evidence []struct {
			ID        int64
			Interview *struct {
				SessionID string `json:"session_id"`
				Responses int    `json:"responses"`
			} `json:"interview"`
			Answers []struct {
				State string `json:"state"`
			} `json:"answers"`
		} `json:"evidence"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Evidence) != 1 || decoded.Evidence[0].Interview == nil || decoded.Evidence[0].Interview.Responses != 2 {
		t.Fatalf("activation: %s", body)
	}
	if len(decoded.Evidence[0].Interview.SessionID) != 64 {
		t.Fatalf("session link: %s", body)
	}
	if len(decoded.Evidence[0].Answers) != 1 || decoded.Evidence[0].Answers[0].State != "grounded" {
		t.Fatalf("activation answers: %s", body)
	}
}

func TestActivationSubmissionAuthorityPreserved(t *testing.T) {
	a, _, job, _, key := newAnswerLibraryFixture(t)
	evidence := activationEvidence(t, a, true)
	answer := "I used EXPLAIN to find a full table scan and recommended adding an index for DBA review."
	a.applicationAnswerGenerate = func(context.Context, string, string) (string, error) {
		return activationEnvelope(t, discovery.AnswerTrace{Answer: answer,
			Claims: []discovery.AnswerClaim{
				{Claim: "used EXPLAIN to find a full table scan", EvidenceID: evidence.ID, EvidenceRevision: evidence.Revision, ResponseID: "response-1", Quote: "EXPLAIN showed a full table scan for a frequently used query"},
			}}), nil
	}
	ticket := reviewNarrative(t, a, activationForm(job, key, evidence))
	if rec := postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(ticket), "", ""); rec.Code != 200 {
		t.Fatalf("generate: %d %s", rec.Code, rec.Body.String())
	}
	saveLibrarySourceAnswer(t, a, job, key, answer, 0)

	token := reviewedSubmissionToken(t, a, job.ID)
	completed := postApp(newMux(a), "/jobs/1/submit", url.Values{"confirm": {"yes"}, "submission_token": {token}}, "", "")
	if completed.Code != http.StatusSeeOther {
		t.Fatalf("submit: %d %s", completed.Code, completed.Body.String())
	}
	recorded, err := a.jobByID(job.ID)
	if err != nil || recorded.Stage != StageApplied || recorded.SubmissionCount != 1 {
		t.Fatalf("submission not recorded: %+v %v", recorded, err)
	}
	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE job_id=1`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("submissions: %d %v", count, err)
	}
	// The manual declaration changes no saved preparation.
	var saved string
	if err := a.db.QueryRow(`SELECT answer FROM application_answers WHERE job_id=1 AND question_key=?`, key).Scan(&saved); err != nil || saved != answer {
		t.Fatalf("saved answer after submit: %q %v", saved, err)
	}
}

func TestActivationDeletionClearsProvenance(t *testing.T) {
	a, _, job, _, key := newAnswerLibraryFixture(t)
	evidence := activationEvidence(t, a, true)
	answer := "I used EXPLAIN to find a full table scan and recommended adding an index for DBA review."
	a.applicationAnswerGenerate = func(context.Context, string, string) (string, error) {
		return activationEnvelope(t, discovery.AnswerTrace{Answer: answer,
			Claims: []discovery.AnswerClaim{
				{Claim: "used EXPLAIN to find a full table scan", EvidenceID: evidence.ID, EvidenceRevision: evidence.Revision, ResponseID: "response-1", Quote: "EXPLAIN showed a full table scan for a frequently used query"},
			}}), nil
	}
	ticket := reviewNarrative(t, a, activationForm(job, key, evidence))
	if rec := postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(ticket), "", ""); rec.Code != 200 {
		t.Fatalf("generate: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := a.deleteJobs([]int64{job.ID}, false); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM ai_answer_provenance WHERE job_id=?`, job.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("provenance rows after deletion: %d %v", count, err)
	}
}
