//go:build qualification

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/storage"
)

const twoJobAshbyPosting = "https://jobs.ashbyhq.com/render/aaaaaaaa-1111-4111-8111-eeeeeeeeeeee"

// TestNativeFixtureGenericQuestionTrace follows the opt-in native fixture's
// generic question ("[Synthetic fixture] Why this team?") from the seed through Go persistence,
// job association, and the Questions projection. The seed bypasses browser
// observation (saveInspection with Source "browser"), so this test documents
// that provenance explicitly instead of treating the seeded row as captured
// evidence. It also pins the visible synthetic markers and the opt-in
// isolation: dedicated temporary path, refusal of an existing database, and
// no access to a normal applicant workspace.
func TestNativeFixtureGenericQuestionTrace(t *testing.T) {
	root, err := os.MkdirTemp("/private/tmp", "yourmove-workflow-native-*")
	if err != nil {
		t.Skipf("fixture temp dir unavailable: %v", err)
	}
	defer os.RemoveAll(root)
	if err := prepareNativeWorkflowFixture(t, root); err != nil {
		t.Fatal(err)
	}
	db, err := storage.OpenDB(filepath.Join(root, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newTestApp(t, db, t.TempDir())

	var title, company string
	if err := a.db.QueryRow(`SELECT title,company FROM jobs WHERE id=1`).Scan(&title, &company); err != nil {
		t.Fatal(err)
	}
	if title != "Synthetic native workflow" || company != "Fixture only" {
		t.Fatalf("fixture job markers changed: title=%q company=%q", title, company)
	}
	var letter string
	if err := a.db.QueryRow(`SELECT body FROM cover_letters WHERE job_id=1`).Scan(&letter); err != nil {
		t.Fatal(err)
	}
	if letter != "Synthetic saved letter" {
		t.Fatalf("fixture letter marker changed: %q", letter)
	}
	var fieldID, label, kind string
	if err := a.db.QueryRow(`SELECT field_id,label,kind FROM application_fields WHERE inspection_id IN (SELECT id FROM application_inspections WHERE job_id=1) AND label='[Synthetic fixture] Why this team?'`).Scan(&fieldID, &label, &kind); err != nil {
		t.Fatalf("seeded generic question missing from persistence: %v", err)
	}
	if fieldID != "browser:motivation" || kind != "textarea" {
		t.Fatalf("seeded generic question identity changed: id=%q kind=%q", fieldID, kind)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := a.candidateProfile()
	if err != nil {
		t.Fatal(err)
	}
	prep, err := a.prepareQuestions(job, profile)
	if err != nil {
		t.Fatal(err)
	}
	var found *preparedQuestion
	for i, q := range prep.Questions {
		if q.Field.Label == "[Synthetic fixture] Why this team?" {
			found = &prep.Questions[i]
		}
	}
	if found == nil {
		t.Fatal("seeded generic question missing from the Questions projection")
	}
	if !found.Editable || found.JobID != 1 || found.Destination != testAshbyPosting || !questionReturnKey.MatchString(found.Key) {
		t.Fatalf("seeded question projection not job-associated: %+v", found)
	}
	t.Logf("fixture seed -> persistence (job 1, %s) -> Questions projection (editable, key %.8s...): visible synthetic markers intact", fieldID, found.Key)

	if err := prepareNativeWorkflowFixture(t, root); err == nil || !strings.Contains(err.Error(), "refusing existing fixture database") {
		t.Fatalf("fixture seed must refuse an existing database, got: %v", err)
	}
	if err := prepareNativeWorkflowFixture(t, filepath.Join(os.TempDir(), "not-the-fixture")); err == nil {
		t.Fatal("fixture seed must refuse a non-temporary path")
	}
}

func twoJobContinuationScope(t *testing.T, a *App, jobID int64) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/"+strconv.FormatInt(jobID, 10)+"/continuation", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("continuation job %d status=%d body=%s", jobID, rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func twoJobReviewHash(t *testing.T, scope map[string]any) string {
	t.Helper()
	prep, ok := scope["preparation"].(map[string]any)
	if !ok {
		t.Fatalf("scope missing preparation: %v", scope)
	}
	hash, _ := prep["answerReviewHash"].(string)
	if hash == "" {
		t.Fatalf("scope missing answerReviewHash: %v", prep)
	}
	return hash
}

// TestControlledTwoJobGenericQuestionIsolation exercises the real Go-controlled
// browser path — capture, review, explicit authorized Fill, DOM readback — on
// two deterministic controlled forms with distinctive different questions.
// It verifies per-job transfer mapping, that the unsupported control stays
// manual, that nothing submits, and that job 1's saved answer never enters
// job 2's Fill scope.
func TestControlledTwoJobGenericQuestionIsolation(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in controlled Chromium qualification")
	}
	serve := func(questionLabel, control string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><form class="ashby-application-form">
<label>First name<input name="first_name"></label>
<label>%s<textarea name="%s"></textarea></label>
<label>Custom control<input name="custom" role="combobox"></label>
<button type="submit">Submit application</button></form><script>window.submits=0;document.querySelector('form').onsubmit=e=>{e.preventDefault();submits++}</script>`, questionLabel, control)
		}))
	}
	fixture1 := serve("Why this team?", "motivation")
	defer fixture1.Close()
	fixture2 := serve("Describe a production incident you resolved", "incident")
	defer fixture2.Close()
	base1 := fixture1.URL + "/fixture/job"
	base2 := fixture2.URL + "/fixture/job"

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	chrome1, err := chromium.LaunchFixture(ctx, base1)
	if err != nil {
		t.Fatal(err)
	}
	defer chrome1.Close()
	chrome2, err := chromium.LaunchFixture(ctx, base2)
	if err != nil {
		t.Fatal(err)
	}
	defer chrome2.Close()

	a, _ := testWorkspace(t)
	requireQualificationStorage(t, a)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Synthetic two-job one"}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveJob(Job{URL: twoJobAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Synthetic two-job two"}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveCandidateProfile(CandidateProfile{FirstName: "Alex", LastName: "Fixture", Email: "alex@example.test"}); err != nil {
		t.Fatal(err)
	}
	profile, err := a.candidateProfile()
	if err != nil {
		t.Fatal(err)
	}
	a.browsers.factory = func(_ context.Context, posting string) (readOnlySession, error) {
		switch posting {
		case testAshbyPosting:
			return &contactFixtureSession{Session: chrome1, fixture: base1, saved: testAshbyPosting}, nil
		case twoJobAshbyPosting:
			return &contactFixtureSession{Session: chrome2, fixture: base2, saved: twoJobAshbyPosting}, nil
		default:
			return nil, fmt.Errorf("fixture posting not authorized")
		}
	}
	a.browsers.pollEvery = 50 * time.Millisecond
	defer a.browsers.closeAll()
	startAndCapture := func(id int64) {
		t.Helper()
		if rec := postApp(newMux(a), "/jobs/"+strconv.FormatInt(id, 10)+"/browser/start", mapValues(), "", ""); rec.Code != 303 {
			t.Fatalf("browser start job %d: %d %s", id, rec.Code, rec.Body.String())
		}
		for deadline := time.Now().Add(10 * time.Second); a.browsers.state(id).LatestID == 0; {
			if time.Now().After(deadline) {
				t.Fatalf("capture timeout job %d", id)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	stopBrowser := func(id int64) {
		t.Helper()
		if rec := postApp(newMux(a), "/jobs/"+strconv.FormatInt(id, 10)+"/browser/stop", mapValues(), "", ""); rec.Code != 303 && rec.Code != 200 {
			t.Fatalf("browser stop job %d: %d %s", id, rec.Code, rec.Body.String())
		}
	}

	// One managed browser at a time: job 1 completes capture through DOM
	// readback before job 2 starts.
	startAndCapture(1)

	job1, _ := a.jobByID(1)
	prep1, err := a.prepareQuestions(job1, profile)
	if err != nil {
		t.Fatal(err)
	}
	var key1, unsupportedKey string
	for _, q := range prep1.Questions {
		switch q.Field.Label {
		case "Why this team?":
			if !q.Editable || q.Key == "" {
				t.Fatalf("job 1 generic question not Fill-eligible: %+v", q)
			}
			key1 = q.Key
		case "Describe a production incident you resolved":
			t.Fatal("job 2 question leaked into job 1 projection")
		case "Custom control":
			if q.Editable {
				t.Fatalf("unsupported control Fill-eligible on job 1: %+v", q)
			}
			unsupportedKey = q.Key
		}
	}
	if key1 == "" {
		t.Fatalf("job 1 generic question missing: %+v", prep1.Questions)
	}
	if unsupportedKey == "" {
		t.Fatal("unsupported control missing from job 1 projection")
	}
	// Failure case: the unsupported control cannot be saved as a local answer.
	rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers", `{"question_key":"`+unsupportedKey+`","revision":0,"destination":"`+testAshbyPosting+`","answer":"manual only"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("unsupported save must be refused, got %d %s", rec.Code, rec.Body.String())
	}

	const answer1 = "I enjoy solving customer problems with careful teams"
	const answer2 = "I diagnosed the outage isolated the service and restored it"
	rec = postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers", `{"question_key":"`+key1+`","revision":0,"destination":"`+testAshbyPosting+`","answer":"`+answer1+`"}`)
	if rec.Code != 200 {
		t.Fatalf("save job 1 answer: %d %s", rec.Code, rec.Body.String())
	}
	revision := strconv.FormatInt(profile.Revision, 10)
	continueJob := func(id int64, destination string) {
		t.Helper()
		hash := twoJobReviewHash(t, twoJobContinuationScope(t, a, id))
		input, _ := json.Marshal(continueInput{Destination: destination, IncludeAnswers: true, ReviewHash: hash, ProfileRevision: revision})
		if rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/"+strconv.FormatInt(id, 10)+"/continue", string(input)); rec.Code != 200 {
			t.Fatalf("authorized continue job %d: %d %s", id, rec.Code, rec.Body.String())
		}
	}
	readBack := func(session *chromium.Session, base, expr string, id int64) {
		t.Helper()
		var result struct {
			Result struct {
				Value bool `json:"value"`
			} `json:"result"`
		}
		for deadline := time.Now().Add(15 * time.Second); ; {
			_ = session.FixtureCall(ctx, base+"/application", "Runtime.evaluate", map[string]any{"expression": expr, "returnByValue": true}, &result)
			if result.Result.Value {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("authorized Fill mismatch job %d", id)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	continueJob(1, testAshbyPosting)
	readBack(chrome1, base1, `document.querySelector('[name=motivation]').value==='`+answer1+`'&&document.querySelector('[name=first_name]').value===''&&document.querySelector('[name=custom]').value===''&&submits===0`, 1)
	stopBrowser(1)

	startAndCapture(2)
	job2, _ := a.jobByID(2)
	prep2, err := a.prepareQuestions(job2, profile)
	if err != nil {
		t.Fatal(err)
	}
	var key2 string
	for _, q := range prep2.Questions {
		switch q.Field.Label {
		case "Describe a production incident you resolved":
			if !q.Editable || q.Key == "" {
				t.Fatalf("job 2 generic question not Fill-eligible: %+v", q)
			}
			key2 = q.Key
		case "Why this team?":
			t.Fatal("job 1 question leaked into job 2 projection")
		case "Custom control":
			if q.Editable {
				t.Fatalf("unsupported control Fill-eligible on job 2: %+v", q)
			}
		}
	}
	if key2 == "" {
		t.Fatalf("job 2 generic question missing: %+v", prep2.Questions)
	}
	if key1 == key2 {
		t.Fatal("question keys not destination-bound across jobs")
	}
	// Cross-job isolation: job 1's saved answer never enters job 2's scope.
	scope2 := twoJobContinuationScope(t, a, 2)
	pending, ok := scope2["pendingAnswers"].([]any)
	if !ok {
		t.Fatalf("job 2 Fill scope missing pendingAnswers array: %#v", scope2["pendingAnswers"])
	}
	if len(pending) != 0 {
		t.Fatalf("job 1 answer leaked into job 2 Fill scope: %v", pending)
	}
	rec = postWritingJSON(t, a, http.MethodPost, "/api/jobs/2/answers", `{"question_key":"`+key2+`","revision":0,"destination":"`+twoJobAshbyPosting+`","answer":"`+answer2+`"}`)
	if rec.Code != 200 {
		t.Fatalf("save job 2 answer: %d %s", rec.Code, rec.Body.String())
	}
	continueJob(2, twoJobAshbyPosting)
	readBack(chrome2, base2, `document.querySelector('[name=incident]').value==='`+answer2+`'&&document.querySelector('[name=first_name]').value===''&&document.querySelector('[name=custom]').value===''&&submits===0`, 2)
	stopBrowser(2)

	var jobAssociation int64
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM application_answers WHERE job_id=1 AND destination=? AND answer=?`, testAshbyPosting, answer1).Scan(&jobAssociation); err != nil || jobAssociation != 1 {
		t.Fatalf("job 1 answer provenance: count=%d err=%v", jobAssociation, err)
	}
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM application_answers WHERE job_id=2 AND destination=? AND answer=?`, twoJobAshbyPosting, answer2).Scan(&jobAssociation); err != nil || jobAssociation != 1 {
		t.Fatalf("job 2 answer provenance: count=%d err=%v", jobAssociation, err)
	}
	var submissions int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM submissions`).Scan(&submissions); err != nil || submissions != 0 {
		t.Fatalf("employer submission boundary violated: count=%d err=%v", submissions, err)
	}
	t.Log("two-job capture -> review -> explicit authorized Fill -> DOM readback: distinctive questions mapped per job, unsupported control untouched, profile controls preserved, zero submissions, no cross-job reuse")
}
