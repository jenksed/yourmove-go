package app

import (
	"bytes"
	"context"
	"html"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// Real local generation/save paths, with synthetic documents and a deterministic
// generator. No normal storage, external provider, or employer is accessed.
func scopeFirstWorkspace(t *testing.T) (*App, *continuationBrowser) {
	t.Helper()
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Company: "Synthetic Northstar", Title: "Support Engineer", Description: "Resolve production incidents"}); err != nil {
		t.Fatal(err)
	}
	master, err := a.importResume("master.docx", bytes.NewReader(sampleDOCXForPreview(t, "Verified production support experience")))
	if err != nil {
		t.Fatal(err)
	}
	role, err := a.importResume("selected-support-resume.docx", bytes.NewReader(sampleDOCXForPreview(t, "Technical troubleshooting experience")))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.setMasterResume(master.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, role.ID); err != nil {
		t.Fatal(err)
	}
	a.coverLetterGenerate = func(context.Context, string) (string, error) {
		return "Dear Hiring Team, my verified technical support experience is relevant to this role.", nil
	}
	for range 2 {
		if r := postApp(newMux(a), "/jobs/1/cover-letter/generate", url.Values{"confirm_local": {"yes"}}, "", ""); r.Code != 303 {
			t.Fatal(r.Code, r.Body.String())
		}
	}
	f := &continuationBrowser{fakeLetterBrowser: &fakeLetterBrowser{fakeBrowser: &fakeBrowser{url: testAshbyPosting + "/application"}}}
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return f, nil }
	t.Cleanup(a.browsers.closeAll)
	return a, f
}

func workbenchPage(t *testing.T, a *App, path string) string {
	t.Helper()
	w := httptest.NewRecorder()
	// Browsers omit fragments from the HTTP request.
	newMux(a).ServeHTTP(w, httptest.NewRequest("GET", "http://localhost"+strings.Split(path, "#")[0], nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	return w.Body.String()
}

func TestLetterSaveReturnsToExactScopeWithoutBrowserAuthority(t *testing.T) {
	for _, state := range []string{"initial", "late", "combined", "complete", "uncertain", "uncertain with answers", "unavailable", "closed", "submitted", "unsupported"} {
		t.Run(state, func(t *testing.T) {
			a, f := scopeFirstWorkspace(t)
			job, _ := a.jobByID(1)
			active := state != "initial" && state != "unavailable"
			if active {
				a.browsers.sessions = map[int64]readOnlySession{1: f}
				a.browsers.states = map[int64]browserRunState{1: {Destination: job.URL, Phase: "detected"}}
				a.browsers.handoffs = map[int64]*resumeHandoff{1: {status: "selected", name: "earlier-browser-resume.pdf", message: "Exact named file present; employer acceptance unverified", contactStatus: "complete"}}
			}
			if state == "combined" || state == "uncertain with answers" {
				field := schema.Field{ID: "browser:why", Label: "Why this team?", Kind: "textarea"}
				if _, err := a.saveInspection(1, schema.Result{Source: "browser", Platform: "ashby", ApplicationURL: job.URL + "/application", Fields: []schema.Field{field}}); err != nil {
					t.Fatal(err)
				}
				r := postApp(newMux(a), "/jobs/1/answers/save", url.Values{"question_key": {questionKey(job.URL, "ashby", field)}, "revision": {"0"}, "destination": {job.URL}, "answer": {"I enjoy resolving incidents with this team."}}, "", "")
				if r.Code != 303 {
					t.Fatal(r.Code, r.Body.String())
				}
			}
			if state == "complete" || strings.HasPrefix(state, "uncertain") {
				status := "complete"
				if strings.HasPrefix(state, "uncertain") {
					status = "attention"
				}
				a.browsers.letters = map[int64]*coverLetterTransfer{1: {draftID: 1, status: status, body: "Previously attempted body", message: "Original browser result requires review"}}
			}
			if state == "unavailable" {
				a.browsers.states = map[int64]browserRunState{1: {Phase: "interrupted", Message: "Browser unavailable; browser-only work may be lost."}}
			}
			if state == "closed" || state == "submitted" {
				stage := StageClosed
				if state == "submitted" {
					stage = StageApplied
				}
				if _, err := a.db.Exec("UPDATE jobs SET stage=? WHERE id=1", stage); err != nil {
					t.Fatal(err)
				}
			}
			if state == "unsupported" {
				if _, err := a.db.Exec("UPDATE jobs SET url='https://unsupported.example.test/role' WHERE id=1"); err != nil {
					t.Fatal(err)
				}
			}
			before := a.browsers.state(1)
			body := "Edited selected draft one. This is the exact locally saved revision for review."
			r := postApp(newMux(a), "/jobs/1/cover-letter/save", url.Values{"draft_id": {"1"}, "body": {body}}, "", "")
			if r.Code != 303 || r.Header().Get("Location") != "/jobs/1?cover=saved&draft=1#prepare" {
				t.Fatal(r.Code, r.Header(), r.Body.String())
			}
			draft, err := a.coverLetterByID(1, 1)
			if err != nil || draft.Body != body || draft.EditedAt == "" {
				t.Fatal(draft, err)
			}
			latest, _ := a.latestCoverLetter(1)
			if latest.ID != 2 || latest.Body == body {
				t.Fatal("save replaced a different draft")
			}
			if !reflect.DeepEqual(before, a.browsers.state(1)) || f.calls+f.contacts+f.resumes+f.answers != 0 {
				t.Fatal("local save changed browser authority or results")
			}
			if state == "complete" || strings.HasPrefix(state, "uncertain") {
				// Even the newly saved hash cannot confer replacement/replay authority.
				replay := postApp(newMux(a), "/jobs/1/cover-letter/transfer", url.Values{"draft_id": {"1"}, "body_hash": {draft.ContentHash()}, "destination": {job.URL}, "mode": {"file"}}, "", "")
				if replay.Code != 409 || f.calls != 0 || !reflect.DeepEqual(before, a.browsers.state(1)) {
					t.Fatal("edited draft authorized replacement or uncertain replay", replay.Code)
				}
			}
			page := workbenchPage(t, a, r.Header().Get("Location"))
			for _, part := range []string{"Saved locally", "Draft 1", draft.EditedAt, draft.ContentHash()[:12], body, "Edit letter"} {
				if !strings.Contains(page, part) {
					t.Fatalf("saved return omitted %q", part)
				}
			}
			if strings.Contains(page, `id="letter-editor" open`) {
				t.Fatal("save reopened editor")
			}
			start := strings.Index(page, `action="/jobs/1/continue"`)
			want := map[string]string{"initial": "Fill application", "late": "Continue application", "combined": "Continue application", "uncertain with answers": "Continue application"}[state]
			if want == "" {
				if start >= 0 {
					t.Fatal("ineligible transfer offered", state)
				}
			} else {
				if start < 0 {
					t.Fatal("eligible action missing", state)
				}
				form := page[start:]
				form = form[:strings.Index(form, "</form>")]
				button := strings.Index(form, ">"+want+"</button>")
				if button < 0 {
					t.Fatal("wrong server-derived action", form)
				}
				if state != "uncertain with answers" && (!strings.Contains(form, body) || !strings.Contains(form, draft.ContentHash())) {
					t.Fatal("exact saved revision missing beside authorization")
				}
				if state == "initial" && !strings.Contains(form, "selected-support-resume.docx") {
					t.Fatal("first fill omitted selected resume")
				}
				if state != "initial" && strings.Contains(form, `name="include_resume"`) {
					t.Fatal("completed resume re-included")
				}
				if state == "combined" || state == "uncertain with answers" {
					if !strings.Contains(form, "Why this team?") || !strings.Contains(form, "I enjoy resolving incidents with this team.") {
						t.Fatal("pending answer authority omitted")
					}
				}
				if state == "uncertain with answers" && (strings.Contains(form, `name="include_letter"`) || strings.Index(page, "Cover letter needs manual inspection") > start) {
					t.Fatal("uncertainty hidden by unrelated eligible material")
				}
			}
			if active && !strings.Contains(page, "earlier-browser-resume.pdf") {
				t.Fatal("prior browser evidence lost")
			}
			// Deliberate reopening and downloading use this draft, not the latest.
			reopened := workbenchPage(t, a, "/jobs/1?draft=1#prepare")
			if !strings.Contains(reopened, `id="letter-editor" open`) || !strings.Contains(reopened, body) {
				t.Fatal("cannot reopen selected draft")
			}
			download := httptest.NewRecorder()
			newMux(a).ServeHTTP(download, httptest.NewRequest("GET", "http://localhost/jobs/1/cover-letter/1/download", nil))
			pdf, err := coverLetterPDF(body)
			if err != nil || download.Code != 200 || !bytes.Equal(download.Body.Bytes(), pdf) {
				t.Fatal("download differs from actual saved revision", download.Code, err)
			}
		})
	}
}

func TestLetterSaveFailurePreservesTextOutsideTransferScope(t *testing.T) {
	a, _ := scopeFirstWorkspace(t)
	saved, _ := a.coverLetterByID(1, 1)
	for _, value := range []string{"Too short <script>no</script>", strings.Repeat("a", maxLetterBytes+1), "Invalid text with a zero byte\x00 preserved in recovery", "Invalid UTF-8 text with a byte \xff preserved in recovery"} {
		r := postApp(newMux(a), "/jobs/1/cover-letter/save", url.Values{"draft_id": {"1"}, "body": {value}}, "", "")
		if r.Code != 422 || !strings.Contains(r.Body.String(), "Letter not saved") || !strings.Contains(r.Body.String(), html.EscapeString(strings.ReplaceAll(strings.ToValidUTF8(value, "�"), "\x00", "�"))) {
			t.Fatal(r.Code, r.Body.String())
		}
		if r.Header().Get("Location") != "" || strings.Contains(r.Body.String(), `action="`) || strings.Contains(r.Body.String(), `name="body_hash"`) {
			t.Fatal("unsaved recovery gained transfer authority")
		}
		current, _ := a.coverLetterByID(1, 1)
		if current.ContentHash() != saved.ContentHash() {
			t.Fatal("failed save changed saved scope")
		}
	}
	if _, err := a.db.Exec(`CREATE TRIGGER reject_letter_save BEFORE UPDATE ON cover_letters BEGIN SELECT RAISE(ABORT, 'synthetic write failure'); END`); err != nil {
		t.Fatal(err)
	}
	value := "Recover this complete submitted letter after a synthetic storage write failure."
	r := postApp(newMux(a), "/jobs/1/cover-letter/save", url.Values{"draft_id": {"1"}, "body": {value}}, "", "")
	if r.Code != 422 || !strings.Contains(r.Body.String(), value) || !strings.Contains(r.Body.String(), "Letter not saved") {
		t.Fatal(r.Code, r.Body.String())
	}
	if _, err := a.db.Exec("DROP TRIGGER reject_letter_save"); err != nil {
		t.Fatal(err)
	}
	desktop := postApp(newDesktopMux(a), "/jobs/1/cover-letter/save", url.Values{"draft_id": {"1"}, "body": {value}}, "wails://localhost", "1")
	// JSON escapes ampersands in the unchanged canonical Wails HTML redirect.
	if desktop.Code != 200 || !strings.Contains(desktop.Body.String(), `window.location.replace("/jobs/1?cover=saved\u0026draft=1#prepare")`) {
		t.Fatal(desktop.Code, desktop.Body.String())
	}
}
