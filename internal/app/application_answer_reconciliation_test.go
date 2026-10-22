package app

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func TestGreenhouseEarlyAnswerNeedsExplicitLiveAcceptance(t *testing.T) {
	a, _ := testWorkspace(t)
	job, publicResult := earlyGreenhouseFixture(t, a)
	if _, err := a.saveInspection(job.ID, publicResult); err != nil {
		t.Fatal(err)
	}
	q := earlyQuestionByID(t, a, job, "application:story", "")
	answer := "I reviewed the alert history and documented the incident response."
	if rec := postApp(newMux(a), "/jobs/1/answers/early-save", earlySaveValues(job, q, "0", answer), "", ""); rec.Code != 303 {
		t.Fatalf("save early answer: %d %s", rec.Code, rec.Body.String())
	}
	liveResult := schema.Result{Platform: "greenhouse", Source: "browser", Completeness: "rendered_partial", ApplicationURL: job.URL, Fields: []schema.Field{
		{ID: "browser:story", Label: "Describe a time you improved a system", Kind: "textarea", Required: true, Group: "rendered form"},
	}}
	if _, err := a.saveInspection(job.ID, liveResult); err != nil {
		t.Fatal(err)
	}
	preparation, err := a.prepareEarlyQuestions(job, httptest.NewRequest("GET", "http://localhost/jobs/1?early_question="+q.Key, nil))
	if err != nil {
		t.Fatal(err)
	}
	var prepared earlyApplicationQuestion
	for _, candidate := range preparation.Questions {
		if candidate.SourceQuestionID == "application:story" {
			prepared = candidate
		}
	}
	if prepared.Reconciliation.Status != "Ready for explicit review" || !prepared.Reconciliation.CanAccept || prepared.Reconciliation.LiveField.ID != "browser:story" {
		t.Fatalf("unique exact match not available for explicit review: %+v", prepared.Reconciliation)
	}
	page := httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1?early_question="+q.Key, nil))
	for _, expected := range []string{"Live form review", "Current employer question", "Use prepared answer for this question", "do not authorize browser filling"} {
		if !strings.Contains(page.Body.String(), expected) {
			t.Fatalf("Prepare omitted reconciliation context %q", expected)
		}
	}
	var answerCount int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM application_answers`).Scan(&answerCount); err != nil || answerCount != 0 {
		t.Fatalf("reconciliation prepared live answer without acceptance: count=%d err=%v", answerCount, err)
	}
	values := url.Values{
		"early_answer_id":             {fmt.Sprint(prepared.EarlyAnswerID)},
		"early_revision":              {fmt.Sprint(prepared.Revision)},
		"destination":                 {job.URL},
		"live_inspection_fingerprint": {prepared.Reconciliation.LiveInspectionFingerprint},
		"live_question_key":           {prepared.Reconciliation.LiveQuestionKey},
	}
	accepted := postApp(newDesktopMux(a), "/jobs/1/answers/early-accept", values, "wails://localhost", "1")
	if accepted.Code != 200 || !strings.Contains(accepted.Body.String(), `window.location.replace("/jobs/1?question=`) {
		t.Fatalf("explicit accept: %d %s", accepted.Code, accepted.Body.String())
	}
	var saved, state string
	if err := a.db.QueryRow(`SELECT answer FROM application_answers WHERE job_id=1`).Scan(&saved); err != nil || saved != answer {
		t.Fatalf("accepted answer did not use live answer persistence: %q %v", saved, err)
	}
	if err := a.db.QueryRow(`SELECT reconciliation_status FROM early_application_answers WHERE id=?`, prepared.EarlyAnswerID).Scan(&state); err != nil || state != "accepted" {
		t.Fatalf("acceptance state not recorded: %q %v", state, err)
	}
	profile, _ := a.candidateProfile()
	live, err := a.prepareQuestions(job, profile)
	if err != nil || len(live.ReadyAnswers()) != 1 || live.ReadyAnswers()[0].Value != answer {
		t.Fatalf("accepted answer did not enter current live workbench: %+v %v", live.Questions, err)
	}
	// Acceptance saves a live answer but does not attach a browser or arm filling.
	transfer := url.Values{"review_hash": {live.ReviewHash()}, "destination": {job.URL}}
	transferRequest := httptest.NewRequest("POST", "http://localhost", strings.NewReader(transfer.Encode()))
	transferRequest.PostForm = transfer
	if status, _ := a.answerTransferCommand(transferRequest, job.ID); status != 409 {
		t.Fatalf("acceptance bypassed explicit transfer authorization: status=%d", status)
	}
	state = ""
	if err := a.db.QueryRow(`SELECT reconciliation_status FROM early_application_answers WHERE id=?`, prepared.EarlyAnswerID).Scan(&state); err != nil || state != "accepted" {
		t.Fatalf("accepted state was lost: %q %v", state, err)
	}
}

func TestEarlyReconciliationConservativeCases(t *testing.T) {
	job := Job{URL: earlyGreenhousePosting}
	basePublic := schema.Field{ID: "application:story", Label: "Describe a time you improved a system", Kind: "textarea", Required: true, Group: "application"}
	baseAnswer := earlyApplicationAnswer{ID: 5, JobID: 1, Destination: job.URL, DiscoverySource: "public_api", SourceQuestionID: basePublic.ID, QuestionLabel: basePublic.Label, Field: basePublic, DiscoveryFingerprint: "published-v1", Answer: "Prepared response", Revision: 1}
	baseLive := Inspection{ID: 2, JobID: 1, Platform: "greenhouse", Source: "browser", Completeness: "rendered_partial", ApplicationURL: job.URL, Fingerprint: "live-v1", Fields: []InspectedField{{Field: schema.Field{ID: "browser:story", Label: basePublic.Label, Kind: "textarea", Required: true}}}}
	basePublicInspection := Inspection{ID: 1, JobID: 1, Platform: "greenhouse", Source: "public_api", Completeness: "public_schema", ApplicationURL: job.URL, Fingerprint: "published-v1", Fields: []InspectedField{{Field: basePublic}}}
	assert := func(name string, answer earlyApplicationAnswer, public Inspection, live Inspection, liveAvailable bool, expected string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			got := reconcileEarlyAnswer(job, answer, &public, live, liveAvailable)
			if got.Status != expected {
				t.Fatalf("status = %q, want %q; detail: %s", got.Status, expected, got.Detail)
			}
			if expected != "Ready for explicit review" && got.CanAccept {
				t.Fatal("unsafe reconciliation was offered for acceptance")
			}
		})
	}
	assert("exact identity and definition", baseAnswer, basePublicInspection, baseLive, true, "Ready for explicit review")

	changed := baseLive
	changed.Fields = append([]InspectedField(nil), baseLive.Fields...)
	changed.Fields[0].Label = "Describe how you improved a system"
	assert("same identity changed wording", baseAnswer, basePublicInspection, changed, true, "Question changed")
	changed = baseLive
	changed.Fields = []InspectedField{{Field: schema.Field{ID: "browser:different", Label: basePublic.Label, Kind: "textarea", Required: true}}}
	assert("same wording different identity", baseAnswer, basePublicInspection, changed, true, "Question changed")
	changed = baseLive
	changed.Fields = []InspectedField{{Field: schema.Field{ID: "browser:story", Label: basePublic.Label, Kind: "text", Required: true}}}
	assert("different control type", baseAnswer, basePublicInspection, changed, true, "Question changed")
	changed = baseLive
	changed.Fields = []InspectedField{
		{Field: schema.Field{ID: "browser:story", Label: basePublic.Label, Kind: "textarea", Required: true}},
		{Field: schema.Field{ID: "browser:story-copy", Label: basePublic.Label, Kind: "textarea", Required: true}},
	}
	assert("duplicate labels with exact identity", baseAnswer, basePublicInspection, changed, true, "Ambiguous")
	changed.Fields = []InspectedField{
		{Field: schema.Field{ID: "browser:story", Label: basePublic.Label, Kind: "textarea", Required: true}},
		{Field: schema.Field{ID: "browser:story", Label: basePublic.Label, Kind: "textarea", Required: true}},
	}
	assert("duplicate observed identity", baseAnswer, basePublicInspection, changed, true, "Ambiguous")
	changed = baseLive
	changed.Fields = []InspectedField{
		{Field: schema.Field{ID: "browser:other-one", Label: basePublic.Label, Kind: "textarea", Required: true}},
		{Field: schema.Field{ID: "browser:other-two", Label: basePublic.Label, Kind: "textarea", Required: true}},
	}
	assert("duplicate labels", baseAnswer, basePublicInspection, changed, true, "Ambiguous")
	changed = baseLive
	changed.Fields[0].Sensitive = true
	assert("sensitive live field", baseAnswer, basePublicInspection, changed, true, "Manual only")
	changed = baseLive
	changed.Fields[0].Kind = "file"
	assert("unsupported live control", baseAnswer, basePublicInspection, changed, true, "Manual only")
	assert("partial later step absent", baseAnswer, basePublicInspection, Inspection{Platform: "greenhouse", Source: "browser", Completeness: "rendered_partial", ApplicationURL: job.URL}, true, "Not observed yet")
	assert("complete form proves absence", baseAnswer, basePublicInspection, Inspection{Platform: "greenhouse", Source: "browser", Completeness: "rendered_complete", ApplicationURL: job.URL}, true, "No longer available")
	assert("no live inspection", baseAnswer, basePublicInspection, Inspection{}, false, "Not observed yet")
	changedPublic := basePublicInspection
	changedPublic.Fields = append([]InspectedField(nil), basePublicInspection.Fields...)
	changedPublic.Fields[0].Label = "Describe a different system improvement"
	assert("published schema changed", baseAnswer, changedPublic, baseLive, true, "Question changed")
	changedJob := Job{URL: "https://job-boards.greenhouse.io/synthetic/jobs/98765"}
	if got := reconcileEarlyAnswer(changedJob, baseAnswer, &basePublicInspection, baseLive, true); got.Status != "Manual only" {
		t.Fatalf("changed posting destination status = %q", got.Status)
	}
	options := basePublic
	options.ID = "application:work_type"
	options.Label = "Preferred schedule"
	options.Kind = "multi_value_single_select"
	options.Options = []string{"Remote", "Hybrid"}
	answer := baseAnswer
	answer.SourceQuestionID, answer.QuestionLabel, answer.Field = options.ID, options.Label, options
	answer.DiscoveryFingerprint = "published-v2"
	public := basePublicInspection
	public.Fingerprint = "published-v2"
	public.Fields = []InspectedField{{Field: options}}
	live := baseLive
	live.Fields = []InspectedField{{Field: schema.Field{ID: "browser:work_type", Label: options.Label, Kind: "select", Required: true, Options: []string{"Remote", "Hybrid"}}}}
	if got := reconcileEarlyAnswer(job, answer, &public, live, true); got.Status != "Ready for explicit review" {
		t.Fatalf("exact select options: %+v", got)
	}
	live.Fields[0].Options = []string{"Remote", "Hybrid", "On site"}
	assert("changed options", answer, public, live, true, "Question changed")
	live = baseLive
	live.ApplicationURL = "https://job-boards.greenhouse.io/another/jobs/12345"
	assert("wrong observed posting", baseAnswer, basePublicInspection, live, true, "Manual only")
	ashbyJob := Job{URL: testAshbyPosting}
	previewField := schema.Field{ID: "preview:story", Label: "Describe a time you improved a system", Kind: "textarea"}
	previewAnswer := earlyApplicationAnswer{Destination: ashbyJob.URL, DiscoverySource: "preview", SourceQuestionID: previewField.ID, QuestionLabel: previewField.Label, Field: previewField, DiscoveryFingerprint: "preview-v1", Answer: "Prepared response", Revision: 1}
	previewInspection := Inspection{Source: "preview", Platform: "ashby", ApplicationURL: ashbyJob.URL + "/application", Fingerprint: "preview-v1", Fields: []InspectedField{{Field: previewField}}}
	ashbyLive := Inspection{Source: "browser", Platform: "ashby", ApplicationURL: ashbyJob.URL + "/application", Fingerprint: "live-v1", Fields: []InspectedField{{Field: schema.Field{ID: "browser:story", Label: previewField.Label, Kind: "textarea"}}}}
	if got := reconcileEarlyAnswer(ashbyJob, previewAnswer, &previewInspection, ashbyLive, true); got.Status != "Ready for explicit review" {
		t.Fatalf("same-renderer Ashby preview identity was not qualified: %+v", got)
	}
}

func TestEarlyAcceptanceRejectsStaleDraftAndChangedLiveInspection(t *testing.T) {
	a, _ := testWorkspace(t)
	job, publicResult := earlyGreenhouseFixture(t, a)
	if _, err := a.saveInspection(job.ID, publicResult); err != nil {
		t.Fatal(err)
	}
	q := earlyQuestionByID(t, a, job, "application:story", "")
	if rec := postApp(newMux(a), "/jobs/1/answers/early-save", earlySaveValues(job, q, "0", "Prepared text"), "", ""); rec.Code != 303 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	liveResult := schema.Result{Platform: "greenhouse", Source: "browser", Completeness: "rendered_partial", ApplicationURL: job.URL, Fields: []schema.Field{{ID: "browser:story", Label: q.Field.Label, Kind: "textarea", Required: true}}}
	if _, err := a.saveInspection(job.ID, liveResult); err != nil {
		t.Fatal(err)
	}
	prep, err := a.prepareEarlyQuestions(job, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if err != nil {
		t.Fatal(err)
	}
	q = prep.Questions[0]
	values := url.Values{
		"early_answer_id":             {fmt.Sprint(q.EarlyAnswerID)},
		"early_revision":              {fmt.Sprint(q.Revision)},
		"destination":                 {job.URL},
		"live_inspection_fingerprint": {q.Reconciliation.LiveInspectionFingerprint},
		"live_question_key":           {q.Reconciliation.LiveQuestionKey},
	}
	stale := values.Clone()
	stale.Set("early_revision", "0")
	if rec := postApp(newMux(a), "/jobs/1/answers/early-accept", stale, "", ""); rec.Code != 409 {
		t.Fatalf("stale early answer accepted: %d %s", rec.Code, rec.Body.String())
	}
	changed := liveResult
	changed.Fields = []schema.Field{{ID: "browser:story", Label: "Different question wording", Kind: "textarea", Required: true}}
	if _, err := a.saveInspection(job.ID, changed); err != nil {
		t.Fatal(err)
	}
	if rec := postApp(newMux(a), "/jobs/1/answers/early-accept", values, "", ""); rec.Code != 409 {
		t.Fatalf("changed live inspection accepted: %d %s", rec.Code, rec.Body.String())
	}
	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM application_answers`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rejected acceptance wrote live answer: %d %v", count, err)
	}
}
