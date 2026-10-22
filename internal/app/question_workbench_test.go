package app

import (
	"fmt"
	"html"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func TestQuestionWorkbenchLocalStatusAndReturnContext(t *testing.T) {
	editable := preparedQuestion{Key: strings.Repeat("a", 64), Editable: true, Source: "Saved for this application", Revision: 1}
	ready := preparedQuestion{Key: strings.Repeat("b", 64), Editable: true, Value: "false", Revision: 2}
	profile := preparedQuestion{Key: strings.Repeat("d", 64), Editable: true, Value: "casey@example.test", Source: "Approved candidate information"}
	manual := preparedQuestion{Key: strings.Repeat("c", 64), Value: "ignored", Field: schema.Field{Sensitive: true}}
	if editable.LocalStatus() != "Needs your answer" || ready.LocalStatus() != "Answer saved locally. It has not been filled on the employer website." ||
		profile.LocalStatus() != "Approved candidate information; transfer requires your authorization" || manual.LocalStatus() != "Requires manual completion in Chromium" {
		t.Fatal("categories misrepresent empty or manual values")
	}
	ambiguous := questionPreparation{Questions: []preparedQuestion{manual, manual}}
	if len(ambiguous.WorkbenchQuestions()) != 1 || ambiguous.WorkbenchQuestions()[0].Editable {
		t.Fatal("ambiguous controls duplicated DOM identity or gained an editor")
	}

	for _, suffix := range []string{"", "?question=" + ready.Key, "?question=" + strings.Repeat("f", 64), "?question=bad", "?question=" + ready.Key + "&question=" + ready.Key, "?question=%zz"} {
		p := questionPreparation{Questions: []preparedQuestion{editable, ready, manual}}
		r := httptest.NewRequest("GET", "http://localhost/jobs/1"+suffix, nil)
		p.selectQuestion(r)
		want := editable.Key
		if suffix == "?question="+ready.Key {
			want = ready.Key
		}
		if p.SelectedKey != want {
			t.Fatalf("%s selected %q", suffix, p.SelectedKey)
		}
	}
}

func TestQuestionSaveReturnAndRecoverableFailures(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
		t.Fatal(err)
	}
	fields := []schema.Field{{ID: "browser:narrative", Label: "Why this team?", Kind: "textarea"}, {ID: "browser:pattern", Label: "Working pattern", Kind: "select", Options: []string{"Remote", "Office"}}, {ID: "browser:legal", Label: "Work authorization", Kind: "text", Sensitive: true}}
	observation := schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: testAshbyPosting + "/application", Fields: fields}
	if _, err := a.saveInspection(1, observation); err != nil {
		t.Fatal(err)
	}
	job, _ := a.jobByID(1)
	p, _ := a.prepareQuestions(job, CandidateProfile{})
	key := p.Questions[0].Key
	values := url.Values{"question_key": {key}, "revision": {"0"}, "destination": {job.URL}, "answer": {"Saved deliberately"}}
	mux := newMux(a)
	rec := postApp(mux, "/jobs/1/answers/save", values, "", "")
	target := "/jobs/1?question=" + key + "#prepare"
	if rec.Code != 303 || rec.Header().Get("Location") != target {
		t.Fatal(rec.Code, rec.Header(), rec.Body.String())
	}
	p, _ = a.prepareQuestions(job, CandidateProfile{})
	if p.Questions[0].Value != "Saved deliberately" || p.Questions[1].Revision != 0 {
		t.Fatal("save affected the wrong question")
	}
	hash := p.ReviewHash()
	// Rejected work is rendered escaped, without any transfer or save authority.
	for _, tc := range []struct {
		name, key, revision, value string
		status                     int
	}{
		{"revision conflict", key, "0", `Recover <script>alert('no')</script>`, 409},
		{"stale identity", strings.Repeat("f", 64), "0", "Keep this unmatched work", 409},
		{"unobserved option", p.Questions[1].Key, "0", "Elsewhere", 422},
		{"manual field", p.Questions[2].Key, "0", "Keep manual work", 409},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values.Set("question_key", tc.key)
			values.Set("revision", tc.revision)
			values.Set("answer", tc.value)
			r := postApp(mux, "/jobs/1/answers/save", values, "", "")
			if r.Code != tc.status || !strings.Contains(r.Body.String(), html.EscapeString(tc.value)) || !strings.Contains(r.Body.String(), "Answer not saved") {
				t.Fatal(r.Code, r.Body.String())
			}
			if strings.Contains(r.Body.String(), `action="`) || strings.Contains(r.Body.String(), `name="review_hash"`) {
				t.Fatal("recovery conferred save or transfer authority")
			}
			current, _ := a.prepareQuestions(job, CandidateProfile{})
			if current.ReviewHash() != hash {
				t.Fatal("unsaved work entered transfer scope")
			}
		})
	}
	// Current observed options are identity, not a mutable client convenience.
	observation.Fields[1].Options = []string{"Hybrid"}
	if _, err := a.saveInspection(1, observation); err != nil {
		t.Fatal(err)
	}
	values.Set("question_key", p.Questions[1].Key)
	values.Set("revision", "0")
	values.Set("answer", "Remote")
	if r := postApp(mux, "/jobs/1/answers/save", values, "", ""); r.Code != 409 || !strings.Contains(r.Body.String(), "observed question changed") {
		t.Fatal(r.Code, r.Body.String())
	}
	// Clearing remains an explicit local save and does not claim a nonempty answer.
	values.Set("question_key", key)
	values.Set("revision", "1")
	values.Set("answer", "")
	if r := postApp(mux, "/jobs/1/answers/save", values, "", ""); r.Code != 303 {
		t.Fatal(r.Code, r.Body.String())
	}
	current, _ := a.prepareQuestions(job, CandidateProfile{})
	if len(current.ReadyAnswers()) != 0 || current.Questions[0].Revision != 2 {
		t.Fatal("empty answer counted as ready")
	}
	// GET/no-JS exposes every authoritative editor with exact options and identity.
	page := httptest.NewRecorder()
	mux.ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1?question="+key, nil))
	for _, text := range []string{`name="question_key" value="` + key + `"`, `<option value="Hybrid"`, "Needs your answer", "Complete on employer site", "Save answer locally"} {
		if !strings.Contains(page.Body.String(), text) {
			t.Fatalf("missing %s", text)
		}
	}
	if strings.Contains(page.Body.String(), `data-question-panel hidden`) {
		t.Fatal("server hid no-JS editors")
	}
	// The Wails adapter retains POST-to-GET semantics for this exact save only.
	values.Set("revision", "2")
	values.Set("answer", "Desktop local value")
	desktop := postApp(newDesktopMux(a), "/jobs/1/answers/save", values, "wails://localhost", "1")
	if desktop.Code != 200 || !strings.Contains(desktop.Body.String(), fmt.Sprintf(`window.location.replace(%q)`, target)) {
		t.Fatal(desktop.Code, desktop.Body.String())
	}
}

func TestQuestionDesktopRedirectAllowlist(t *testing.T) {
	key := strings.Repeat("a", 64)
	valid := "/jobs/1?question=" + key + "#prepare"
	if !safeDesktopRedirectTarget(valid) {
		t.Fatal("canonical question return rejected")
	}
	for _, bad := range []string{
		"https://foreign.test" + valid, "//foreign.test" + valid,
		"/jobs/01?question=" + key + "#prepare", "/jobs/1?question=bad#prepare",
		"/jobs/1?question=" + strings.ToUpper(key) + "#prepare",
		"/jobs/1?question=" + key + "&question=" + key + "#prepare",
		"/jobs/1?question=" + key + "&unknown=1#prepare",
		"/jobs/1?draft=1&question=" + key + "#prepare",
		"/jobs/1?question=%61" + key[1:] + "#prepare",
	} {
		if safeDesktopRedirectTarget(bad) {
			t.Fatal("unsafe question return", bad)
		}
	}
	// Even a canonical target cannot be used by a different job or operation.
	for _, source := range []string{"/jobs/2/answers/save", "/jobs/1/continue"} {
		rec := httptest.NewRecorder()
		writer := &desktopRedirectWriter{ResponseWriter: rec, requestPath: source}
		writer.Header().Set("Location", valid)
		writer.WriteHeader(303)
		if rec.Code != 303 {
			t.Fatalf("foreign source %s adapted", source)
		}
	}
}
