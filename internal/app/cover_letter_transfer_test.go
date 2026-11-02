package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http/httptest"

	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/ledongthuc/pdf"
)

func TestCoverLetterPDFDeterministicAndReadable(t *testing.T) {
	body := "Dear Hiring Team,\n\nI’d bring experience with Linux (and SQL) to your team.\n" + strings.Repeat("A longer paragraph with verified experience. ", 150) + "\n\nSincerely,\nZoë Example"
	data, err := coverLetterPDF(body)
	if err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv("YOURMOVE_EVIDENCE_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "sample-letter.pdf"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	again, err := coverLetterPDF(body)
	if err != nil || !bytes.Equal(data, again) {
		t.Fatal("export is not deterministic")
	}
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if reader.NumPage() < 2 {
		t.Fatal("long letter was not paginated")
	}
	text, err := reader.GetPlainText()
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	got.ReadFrom(text)
	for _, part := range []string{"Dear Hiring Team", "Linux (and SQL)", "Zoë Example"} {
		if !strings.Contains(got.String(), part) {
			t.Fatalf("missing exported text %q", part)
		}
	}
	for _, invalid := range []string{"", "Hello\x00there", "Hello 世界"} {
		if _, err := coverLetterPDF(invalid); err == nil {
			t.Fatal("unsupported export was silently changed")
		}
	}
}

type fakeLetterBrowser struct {
	*fakeBrowser
	letterMu sync.Mutex
	calls    int
	gotBody  string
	gotFile  []byte
	gotName  string
}

func (f *fakeLetterBrowser) AttachCoverLetter(_ context.Context, path, digest string) (chromium.AttachmentResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return chromium.AttachmentResult{}, err
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != digest {
		return chromium.AttachmentResult{}, fmt.Errorf("digest mismatch")
	}
	f.letterMu.Lock()
	defer f.letterMu.Unlock()
	f.calls++
	f.gotFile = data
	f.gotName = filepath.Base(path)
	return chromium.AttachmentResult{Selected: true, Verified: true, Detail: "File selected; employer acceptance unverified"}, nil
}
func (f *fakeLetterBrowser) FillCoverLetter(_ context.Context, body string) (chromium.AttachmentResult, error) {
	f.letterMu.Lock()
	defer f.letterMu.Unlock()
	f.calls++
	f.gotBody = body
	return chromium.AttachmentResult{Selected: true, Verified: true, Detail: "Text observed"}, nil
}
func TestCoverLetterTransferPinsRevisionAndContinuesSession(t *testing.T) {
	for _, mode := range []string{"file", "text"} {
		t.Run(mode, func(t *testing.T) {
			a, _ := testWorkspace(t)
			if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
				t.Fatal(err)
			}
			body := "Dear Hiring Team, I bring verified support experience to this role."
			_, err := a.db.Exec(`INSERT INTO cover_letters(job_id,body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at) VALUES(1,?,'ollama','test',1,'master',1,'role','job','now')`, body)
			if err != nil {
				t.Fatal(err)
			}
			draft, err := a.latestCoverLetter(1)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := a.db.Exec(`INSERT INTO cover_letters(job_id,body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at) VALUES(1,?,'ollama','test',1,'master',1,'role','job','later')`, body+" This newer draft must not be transferred."); err != nil {
				t.Fatal(err)
			}
			f := &fakeLetterBrowser{fakeBrowser: &fakeBrowser{url: testAshbyPosting + "/application"}}
			a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return f, nil }
			// Slow watcher lets this test drive deterministic dispatch and stale checks.
			a.browsers.pollEvery = time.Hour
			if err := a.browsers.start(context.Background(), 1, testAshbyPosting, ""); err != nil {
				t.Fatal(err)
			}
			a.browsers.cancels = map[int64]context.CancelFunc{1: func() {}}
			a.browsers.handoffs = map[int64]*resumeHandoff{1: {status: "selected", name: "existing.pdf"}}
			values := url.Values{"draft_id": {fmt.Sprint(draft.ID)}, "body_hash": {draft.ContentHash()}, "destination": {testAshbyPosting}, "confirm": {"yes"}, "mode": {mode}}
			mux := newMux(a)
			bad := values.Encode()
			values.Set("body_hash", "stale")
			if rec := postApp(mux, "/jobs/1/cover-letter/transfer", values, "", ""); rec.Code != 409 {
				t.Fatalf("stale approval: %d", rec.Code)
			}
			values, _ = url.ParseQuery(bad)
			rec := postApp(newDesktopMux(a), "/jobs/1/cover-letter/transfer", values, "wails://localhost", "")
			if rec.Code != 200 || !strings.Contains(rec.Body.String(), `window.location.replace("/jobs/1?cover=transferring\u0026draft=1#prepare")`) {
				t.Fatalf("desktop transfer: %d %s", rec.Code, rec.Body.String())
			}
			a.browsers.tryCoverLetter(context.Background(), 1)
			a.browsers.tryCoverLetter(context.Background(), 1)
			if rec := postApp(mux, "/jobs/1/cover-letter/transfer", values, "", ""); rec.Code != 409 {
				t.Fatalf("duplicate transfer allowed: %d", rec.Code)
			}
			f.letterMu.Lock()
			if mode == "file" && f.gotName != "Cover_Letter.pdf" {
				t.Fatalf("unexpected employer filename: %q", f.gotName)
			}
			if f.calls != 1 || (mode == "text" && f.gotBody != body) || (mode == "file" && !bytes.HasPrefix(f.gotFile, []byte("%PDF-"))) {
				t.Fatalf("wrong transfer count/body: %d", f.calls)
			}
			f.letterMu.Unlock()
			state := a.browsers.state(1)
			if state.LetterStatus != "complete" || state.ResumeStatus != "selected" || !state.Active {
				t.Fatalf("continuation damaged: %+v", state)
			}
			page := httptest.NewRecorder()
			mux.ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1?cover=transferring&draft=1", nil))
			if strings.Contains(page.Body.String(), "This newer draft must not be transferred.") {
				t.Fatal("redirect selected a different letter than the approved draft")
			}
			if !strings.Contains(page.Body.String(), state.LetterMessage) {
				t.Fatal("transfer result missing from Prepare")
			}
			status := httptest.NewRecorder()
			mux.ServeHTTP(status, httptest.NewRequest("GET", "http://localhost/jobs/1/browser-status", nil))
			if strings.Contains(status.Body.String(), body) {
				t.Fatal("letter content leaked into status")
			}
			dir := a.browsers.letters[1].dir
			a.browsers.close(1)
			if dir != "" {
				if _, err := os.Stat(dir); !os.IsNotExist(err) {
					t.Fatal("prepared file was not removed")
				}
			}
		})
	}
}

func TestPendingCoverLetterChangeRevokesTransfer(t *testing.T) {
	for _, change := range []string{"body", "destination", "stage"} {
		t.Run(change, func(t *testing.T) {
			a, _ := testWorkspace(t)
			if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
				t.Fatal(err)
			}
			_, err := a.db.Exec(`INSERT INTO cover_letters(job_id,body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at) VALUES(1,'This is the exact saved draft that was reviewed.','ollama','test',1,'master',1,'role','job','now')`)
			if err != nil {
				t.Fatal(err)
			}
			d, _ := a.latestCoverLetter(1)
			f := &fakeLetterBrowser{fakeBrowser: &fakeBrowser{url: testAshbyPosting + "/application"}}
			a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return f, nil }
			if err := a.browsers.start(context.Background(), 1, testAshbyPosting, ""); err != nil {
				t.Fatal(err)
			}
			defer a.browsers.closeAll()
			a.browsers.cancels = map[int64]context.CancelFunc{1: func() {}}
			values := url.Values{"draft_id": {fmt.Sprint(d.ID)}, "body_hash": {d.ContentHash()}, "destination": {testAshbyPosting}, "confirm": {"yes"}, "mode": {"text"}}
			if rec := postApp(newMux(a), "/jobs/1/cover-letter/transfer", values, "", ""); rec.Code != 303 {
				t.Fatal(rec.Code, rec.Body.String())
			}
			switch change {
			case "body":
				err = a.saveCoverLetterBody(1, d.ID, "This changed draft needs a new review before transfer.")
			case "destination":
				_, err = a.db.Exec("UPDATE jobs SET url='https://jobs.ashbyhq.com/other/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa' WHERE id=1")
			case "stage":
				_, err = a.db.Exec("UPDATE jobs SET stage='closed' WHERE id=1")
			}
			if err != nil {
				t.Fatal(err)
			}
			a.browsers.tryCoverLetter(context.Background(), 1)
			if f.calls != 0 || a.browsers.state(1).LetterStatus != "review" {
				t.Fatal("changed approval reached browser")
			}
		})
	}
}

func TestCoverLetterDestinationCanChangeOnlyBeforeSelection(t *testing.T) {
	h := &sessionHost{}
	h.factory = func(context.Context, string) (readOnlySession, error) {
		return &fakeBrowser{url: testAshbyPosting}, nil
	}
	if err := h.start(context.Background(), 1, testAshbyPosting, ""); err != nil {
		t.Fatal(err)
	}
	defer h.closeAll()
	h.cancels = map[int64]context.CancelFunc{1: func() {}}
	oldDir := t.TempDir()
	old := &coverLetterTransfer{dir: oldDir, destination: testAshbyPosting, mode: "file", status: "waiting"}
	if err := h.armCoverLetter(1, old); err != nil {
		t.Fatal(err)
	}
	next := &coverLetterTransfer{destination: testAshbyPosting, mode: "text", status: "waiting"}
	if err := h.armCoverLetter(1, next); err != nil {
		t.Fatal("unwritten letter could not change destination", err)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatal("old staging retained")
	}
	for _, status := range []string{"checking", "attention", "complete"} {
		next.status = status
		if err := h.armCoverLetter(1, &coverLetterTransfer{destination: testAshbyPosting, status: "waiting"}); err == nil {
			t.Fatal("in-flight, uncertain, or successful selection was replayable")
		}
	}
}
