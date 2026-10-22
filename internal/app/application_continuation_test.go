package app

import (
	"bytes"
	"context"
	"errors"
	"html"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// One test browser implements the existing optional capabilities, with no new owner.
type continuationBrowser struct {
	*fakeLetterBrowser
	resumes, contacts, answers int
	uncertain                  bool
}

func (f *continuationBrowser) AttachResume(context.Context, string) (chromium.AttachmentResult, error) {
	f.resumes++
	return chromium.AttachmentResult{Selected: true, Verified: true}, nil
}
func (f *continuationBrowser) FillContacts(context.Context, browser.ContactValues) (browser.ContactResult, error) {
	f.contacts++
	return browser.ContactResult{Ready: true}, nil
}
func (f *continuationBrowser) FillAnswer(_ context.Context, v browser.AnswerValue) (browser.AnswerOutcome, error) {
	f.answers++
	return browser.AnswerOutcome{Label: v.Field.Label, Status: "filled"}, nil
}

func (f *continuationBrowser) AttachCoverLetter(ctx context.Context, path, digest string) (chromium.AttachmentResult, error) {
	if f.uncertain {
		f.calls++
		return chromium.AttachmentResult{}, errors.New("unknown upload outcome")
	}
	return f.fakeLetterBrowser.AttachCoverLetter(ctx, path, digest)
}

func renderedContinuation(t *testing.T, a *App) (url.Values, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	body := rec.Body.String()
	start := strings.Index(body, `action="/jobs/1/continue"`)
	if rec.Code != 200 || start < 0 {
		t.Fatalf("no continuation: %d %s", rec.Code, body)
	}
	form := body[start:]
	form = form[:strings.Index(form, "</form>")]
	if strings.Contains(form, `name="confirm"`) {
		t.Fatal("generic approval checkbox returned")
	}
	values := url.Values{}
	for _, m := range regexp.MustCompile(`name="([^"]+)" value="([^"]*)"`).FindAllStringSubmatch(form, -1) {
		values.Set(m[1], html.UnescapeString(m[2]))
	}
	values.Set("mode", "file")
	return values, form
}

func TestAnswerTransferSummaryDoesNotImplyEveryFieldWasFilled(t *testing.T) {
	outcomes := []browser.AnswerOutcome{
		{Label: "Narrative", Status: "filled"},
		{Label: "Work pattern", Status: "correct"},
		{Label: "Unsupported", Status: "manual"},
		{Label: "Changed field", Status: "uncertain"},
	}
	want := "1 filled and verified; 1 already correct; 1 needs manual completion; 1 uncertain and needs inspection"
	if got := summarizeAnswerOutcomes(outcomes); got != want {
		t.Fatalf("summary=%q, want %q", got, want)
	}
	presentation := summarizePreparation(jobWorkspace{Job: Job{URL: testAshbyPosting}, BrowserState: browserRunState{Active: true, Destination: testAshbyPosting, AnswerStatus: "complete", AnswerOutcomes: outcomes}})
	if presentation.Title != "Answer transfer processing finished" || !strings.Contains(presentation.Detail, want) || !strings.Contains(presentation.Detail, "submit personally") {
		t.Fatalf("answer processing status is inaccurate or incomplete: %+v", presentation)
	}
}

func TestCombinedApplicationScopeAndIndependentFailures(t *testing.T) {
	for _, tc := range []struct{ posting, platform, suffix string }{
		{testAshbyPosting, "ashby", "/application"},
		{"https://job-boards.greenhouse.io/cordance/jobs/5423762008", "greenhouse", "#application"},
		{"https://jobs.lever.co/acme/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "lever", "/apply"},
	} {
		t.Run(tc.platform, func(t *testing.T) {
			posting, platform := tc.posting, tc.platform
			application := posting + tc.suffix
			for _, stale := range []string{"none", "letter", "answers", "destination", "resume only", "uncertain"} {
				t.Run(stale, func(t *testing.T) {
					a, _ := testWorkspace(t)
					if err := a.saveJob(Job{URL: posting, Host: "jobs.ashbyhq.com", Title: "Controlled role", Company: "Fixture", Description: "Resolve production incidents"}); err != nil {
						t.Fatal(err)
					}
					master, err := a.importResume("master.docx", bytes.NewReader(sampleDOCXForPreview(t, "Production incident experience")))
					if err != nil {
						t.Fatal(err)
					}
					role, err := a.importResume("role.docx", bytes.NewReader(sampleDOCXForPreview(t, "Technical support experience")))
					if err != nil {
						t.Fatal(err)
					}
					if err := a.setMasterResume(master.ID, true); err != nil {
						t.Fatal(err)
					}
					if err := a.assignResume(1, role.ID); err != nil {
						t.Fatal(err)
					}
					if err := a.saveCandidateProfile(CandidateProfile{FirstName: "Casey", LastName: "Fixture", Email: "casey@example.test", Approved: true}); err != nil {
						t.Fatal(err)
					}
					field := schema.Field{ID: "browser:why", Label: "Why this team?", Kind: "textarea"}
					if _, err := a.saveInspection(1, schema.Result{Platform: platform, Source: "browser", ApplicationURL: application, Fields: []schema.Field{field}}); err != nil {
						t.Fatal(err)
					}
					mux := newMux(a)
					saveAnswer := func(revision, value string) {
						t.Helper()
						rec := postApp(mux, "/jobs/1/answers/save", url.Values{"destination": {posting}, "question_key": {questionKey(posting, platform, field)}, "revision": {revision}, "answer": {value}}, "", "")
						if rec.Code != 303 {
							t.Fatal(rec.Code, rec.Body.String())
						}
					}
					saveAnswer("0", "I enjoy solving incidents with this team.")
					a.coverLetterGenerate = func(context.Context, string) (string, error) {
						return "Dear Hiring Team, I bring technical support experience to this role.", nil
					}
					rec := postApp(mux, "/jobs/1/cover-letter/generate", url.Values{"confirm_local": {"yes"}}, "", "")
					if rec.Code != 303 {
						t.Fatal(rec.Code)
					}
					f := &continuationBrowser{fakeLetterBrowser: &fakeLetterBrowser{fakeBrowser: &fakeBrowser{url: application}}}
					a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return f, nil }
					// Deliberately hold dispatch so tests control interleavings, not timing.
					a.browsers.cancels = map[int64]context.CancelFunc{1: func() {}}
					defer a.browsers.closeAll()
					values, form := renderedContinuation(t, a)
					for _, phrase := range []string{"Fill application", "role.docx", "casey@example.test", "Dear Hiring Team", "Why this team?", "I enjoy solving"} {
						if !strings.Contains(form, phrase) {
							t.Fatalf("scope omitted %q", phrase)
						}
					}
					for _, flag := range []string{"include_resume", "include_letter", "include_answers"} {
						if values.Get(flag) != "yes" {
							t.Fatal("missing scope", flag)
						}
					}
					if f.resumes+f.contacts+f.answers+f.calls != 0 {
						t.Fatal("render/save transmitted data")
					}
					switch stale {
					case "letter":
						rec = postApp(mux, "/jobs/1/cover-letter/save", url.Values{"draft_id": {"1"}, "body": {"A newly edited letter which must be reviewed before transmission."}}, "", "")
						if rec.Code != 303 {
							t.Fatal(rec.Code)
						}
					case "answers":
						saveAnswer("1", "New answer requiring a fresh review.")
					case "resume only":
						values.Del("include_letter")
						values.Del("include_answers")
					case "uncertain":
						f.uncertain = true
					case "destination":
						values.Set("destination", "https://example.com/changed")
					}
					if bad := postApp(mux, "/jobs/1/continue", values, "https://evil.example", ""); bad.Code != 403 {
						t.Fatal("foreign request authorized")
					}
					rec = postApp(newDesktopMux(a), "/jobs/1/continue", values, "wails://localhost", "")
					if stale == "destination" {
						if rec.Code != 409 || a.browsers.has(1) {
							t.Fatal("wrong destination opened browser")
						}
						return
					}
					if rec.Code != 200 || !strings.Contains(rec.Body.String(), "window.location.replace") {
						t.Fatalf("native PRG: %d %s", rec.Code, rec.Body.String())
					}
					if (stale == "letter" || stale == "answers") && !strings.Contains(rec.Body.String(), "transfer_notice") {
						t.Fatal("partial rejection not reported")
					}
					a.browsers.tryResume(context.Background(), 1)
					a.browsers.tryContacts(context.Background(), 1)
					a.browsers.tryCoverLetter(context.Background(), 1)
					a.browsers.tryAnswers(context.Background(), 1)
					wantLetters, wantAnswers := 1, 1
					if stale == "letter" {
						wantLetters = 0
					}
					if stale == "answers" {
						wantAnswers = 0
					}
					if stale == "resume only" {
						wantLetters = 0
						wantAnswers = 0
					}
					if stale == "uncertain" && a.browsers.state(1).LetterStatus != "attention" {
						t.Fatal("uncertain upload was not flagged for inspection")
					}
					if f.resumes != 1 || f.contacts != 1 || f.calls != wantLetters || f.answers != wantAnswers {
						t.Fatalf("operations were not independent: %+v letters=%d", f, f.calls)
					}
					// Ordinary reload and repeated command never replay completed operations.
					page := httptest.NewRecorder()
					mux.ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
					a.browsers.tryResume(context.Background(), 1)
					a.browsers.tryContacts(context.Background(), 1)
					a.browsers.tryCoverLetter(context.Background(), 1)
					a.browsers.tryAnswers(context.Background(), 1)
					if f.resumes != 1 || f.contacts != 1 || f.calls != wantLetters || f.answers != wantAnswers {
						t.Fatal("completed operations replayed")
					}
				})
			}
		})
	}
}

func TestPreparationActionStates(t *testing.T) {
	q := preparedQuestion{Key: "q", Value: "answer", Editable: true}
	for _, tc := range []struct {
		name   string
		state  browserRunState
		letter bool
		action string
	}{
		{"initial", browserRunState{}, true, "Fill application"},
		{"late letter", browserRunState{Active: true, ResumeStatus: "selected"}, true, "Continue application"},
		{"running", browserRunState{Active: true, ResumeStatus: "selected", LetterStatus: "checking"}, true, ""},
		{"uncertain", browserRunState{Active: true, ResumeStatus: "selected", LetterStatus: "attention"}, true, ""},
		{"lost", browserRunState{Phase: "interrupted"}, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := jobWorkspace{Handoff: HandoffEligibility{Allowed: true}, BrowserState: tc.state}
			if tc.letter {
				w.Letter.ID = 1
			}
			if got := summarizePreparation(w); got.Action != tc.action {
				t.Fatalf("got %+v", got)
			}
		})
	}
	w := jobWorkspace{BrowserState: browserRunState{Active: true, ResumeStatus: "selected"}, Questions: questionPreparation{Questions: []preparedQuestion{q}}}
	w.Letter.ID = 1
	if got := summarizePreparation(w); got.Action != "Continue application" {
		t.Fatal(got)
	}
	w.BrowserState.AnswerAttempted = []preparedQuestion{q}
	w.BrowserState.LetterStatus = "complete"
	if got := summarizePreparation(w); got.Action != "" {
		t.Fatal("completed work offered", got)
	}
}

func TestUnsupportedPreparationOffersExternalApplication(t *testing.T) {
	a, _ := testWorkspace(t)
	posting := "https://example.wd5.myworkdayjobs.com/Careers/job/Support_R123"
	if err := a.saveJob(Job{URL: posting, Host: "example.wd5.myworkdayjobs.com", Title: "Technical Support Specialist"}); err != nil {
		t.Fatal(err)
	}
	for _, desktop := range []bool{false, true} {
		if desktop {
			a.openPosting = func(string) error { return nil }
		}
		rec := httptest.NewRecorder()
		newMux(a).ServeHTTP(rec, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
		body := rec.Body.String()
		if rec.Code != 200 || !strings.Contains(body, "Open application ↗") || !strings.Contains(body, "Automatic filling is not available") || strings.Contains(body, "Ready to fill application") {
			t.Fatalf("desktop=%v: missing manual next action: %d %s", desktop, rec.Code, body)
		}
		if desktop && !strings.Contains(body, `action="/jobs/1/open"`) {
			t.Fatal("desktop application action must use the existing native opener")
		}
	}
}
