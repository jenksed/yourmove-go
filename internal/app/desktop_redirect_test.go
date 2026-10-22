package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/storage"
)

func TestDesktopPOSTRedirectsToGetNavigation(t *testing.T) {
	db, err := storage.OpenDB(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	app := newTestApp(t, db, "")
	handler := newDesktopMux(app)
	post := func(path string, data url.Values) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "http://localhost"+path, strings.NewReader(data.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", "wails://localhost")
		r.Header.Set("X-Wails-Window-Id", "1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}

	w := post("/jobs", url.Values{"job_url": {"https://example.com/wails-test"}, "job_title": {"Test job"}})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `window.location.replace("/jobs")`) || w.Header().Get("Location") != "" {
		t.Fatalf("add-job navigation response: status=%d location=%q body=%s", w.Code, w.Header().Get("Location"), w.Body.String())
	}
	jobs, err := app.allJobs()
	if err != nil || len(jobs) != 1 || jobs[0].Title != "Test job" {
		t.Fatalf("add-job persistence: jobs=%+v err=%v", jobs, err)
	}

	w = post("/jobs/1/edit", url.Values{
		"title": {"Updated title"}, "company": {"Example & Company"},
		"stage": {StageReady}, "priority": {PriorityHigh},
	})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `window.location.replace("/jobs/1")`) {
		t.Fatalf("edit navigation response: status=%d body=%s", w.Code, w.Body.String())
	}
	job, err := app.jobByID(1)
	if err != nil || job.Title != "Updated title" || job.Company != "Example & Company" {
		t.Fatalf("edit persistence: job=%+v err=%v", job, err)
	}

	w = post("/jobs/1/refresh", url.Values{})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `window.location.replace("/jobs/1")`) {
		t.Fatalf("refresh navigation response: status=%d body=%s", w.Code, w.Body.String())
	}

	w = post("/jobs/1/resume", url.Values{"resume_id": {"0"}})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `window.location.replace("/jobs/1#prepare")`) {
		t.Fatalf("resume recovery should return to Prepare: status=%d body=%s", w.Code, w.Body.String())
	}

	// The browser route continues to issue a real 303 for PRG.
	r := httptest.NewRequest(http.MethodPost, "http://localhost/jobs/1/refresh", nil)
	w = httptest.NewRecorder()
	newMux(app).ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/jobs/1" {
		t.Fatalf("browser redirect changed: status=%d location=%s", w.Code, w.Header().Get("Location"))
	}
	r = httptest.NewRequest(http.MethodPost, "http://localhost/jobs/1/resume", strings.NewReader(url.Values{"resume_id": {"0"}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	newMux(app).ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/jobs/1#prepare" {
		t.Fatalf("browser resume recovery redirect changed: status=%d location=%s", w.Code, w.Header().Get("Location"))
	}
}

func TestDesktopRedirectShimLeavesOtherResponsesUntouched(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		status       int
	}{
		{"external", "https://untrusted.test", http.StatusSeeOther},
		{"protocol relative", "//untrusted.test", http.StatusSeeOther},
		{"non numeric job", "/jobs/bad", http.StatusSeeOther},
		{"query", "/jobs/1?next=bad", http.StatusSeeOther},
		{"valid job", "/jobs/1", http.StatusOK},
		{"prepare", "/jobs/1#prepare", http.StatusOK},
		{"history", "/jobs/1#history", http.StatusOK},
		{"history noncanonical id", "/jobs/01#history", http.StatusSeeOther},
		{"history zero id", "/jobs/0#history", http.StatusSeeOther},
		{"history query", "/jobs/1?next=bad#history", http.StatusSeeOther},
		{"history external", "https://untrusted.test/jobs/1#history", http.StatusSeeOther},
		{"history extra fragment", "/jobs/1#prepare#history", http.StatusSeeOther},
		{"prepare noncanonical id", "/jobs/01#prepare", http.StatusSeeOther},
		{"prepare query", "/jobs/1?next=bad#prepare", http.StatusSeeOther},
		{"inspection prepare", "/jobs/1/inspection#prepare", http.StatusSeeOther},
		{"inspection", "/jobs/1/inspection", http.StatusOK},
		{"inspection external suffix", "/jobs/1/inspection/more", http.StatusSeeOther},
		{"inspection query", "/jobs/1/inspection?redirect=x", http.StatusSeeOther},
		{"resume library", "/resumes", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := withDesktopPostRedirects(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, tc.target, http.StatusSeeOther)
			}))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "http://localhost/jobs", nil))
			if w.Code != tc.status {
				t.Fatalf("status=%d want=%d", w.Code, tc.status)
			}
		})
	}
}

func TestContinuationRedirectTargetsStayLocalAndCanonical(t *testing.T) {
	for _, target := range []string{"/jobs/1?draft=2#prepare", "/jobs/1?transfer_notice=Cover+letter%3A+changed#prepare", "/jobs/1?draft=2&transfer_notice=Review+the+letter#prepare"} {
		if !safeDesktopRedirectTarget(target) {
			t.Fatal("rejected continuation", target)
		}
	}
	for _, target := range []string{"//evil.test/jobs/1?draft=2#prepare", "https://evil.test/jobs/1?draft=2#prepare", "/jobs/01?draft=2#prepare", "/jobs/1?draft=0#prepare", "/jobs/1?draft=2&draft=3#prepare", "/jobs/1?transfer_notice=one&transfer_notice=two#prepare", "/jobs/1?next=https%3A%2F%2Fevil.test#prepare", "/jobs/1?draft=2#other"} {
		if safeDesktopRedirectTarget(target) {
			t.Fatal("accepted unsafe target", target)
		}
	}
}

// Exercise the actual completion handler, not just an artificial redirect:
// persistence succeeds once and desktop navigation returns to rendered history.
func TestDesktopRecordCompletionReturnsToHistory(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/completed", Title: "Completion regression"}); err != nil {
		t.Fatal(err)
	}
	token := reviewedSubmissionToken(t, a, 1)
	form := url.Values{"confirm": {"yes"}, "submission_token": {token}, "notes": {"Completed in employer browser"}}
	for _, desktop := range []bool{true, false} {
		var handler http.Handler = newMux(a)
		if desktop {
			handler = newDesktopMux(a)
		}
		response := postApp(handler, "/jobs/1/submit", form, "http://localhost", "")
		if desktop {
			if response.Code != http.StatusOK || response.Header().Get("Location") != "" || !strings.Contains(response.Body.String(), `window.location.replace("/jobs/1#history")`) || !strings.Contains(response.Body.String(), `href="/jobs/1#history"`) {
				t.Fatalf("desktop completion cannot navigate: %d %s", response.Code, response.Body.String())
			}
		} else if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/jobs/1#history" {
			t.Fatal("browser PRG changed", response.Code, response.Header())
		}
	}
	history, err := a.jobSubmissions(1)
	if err != nil || len(history) != 1 {
		t.Fatalf("completion retry duplicated or lost record: %+v %v", history, err)
	}
	page := httptest.NewRecorder()
	newDesktopMux(a).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Application recorded as Applied.") || !strings.Contains(page.Body.String(), "Completed in employer browser") {
		t.Fatalf("history destination unavailable: %d %s", page.Code, page.Body.String())
	}
}
