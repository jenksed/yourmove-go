package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestOpportunitiesAPIListAndSingleAdd(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/role/1", Host: "example.com", Title: "Existing"}); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/api/opportunities", nil))
	if rec.Code != 200 {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}
	var list struct {
		Jobs      []opportunityJobJSON `json:"jobs"`
		Total     int                  `json:"total"`
		Remaining int                  `json:"remaining"`
		Desktop   bool                 `json:"desktop"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 || len(list.Jobs) != 1 || list.Jobs[0].DisplayTitle != "Existing" {
		t.Fatalf("unexpected list: %+v", list)
	}
	// The React dashboard reads next.jobId/next.title/... in lowercase.
	var listWire map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &listWire); err != nil {
		t.Fatal(err)
	}
	if next, _ := listWire["next"].(map[string]any); next != nil {
		for _, key := range []string{"jobId", "title", "company", "label", "reason", "href"} {
			if _, ok := next[key]; !ok {
				t.Fatalf("next action missing lowercase key %q: %v", key, next)
			}
		}
	} else {
		t.Fatalf("expected a next action for one review-stage job: %s", rec.Body.String())
	}

	// Duplicate single add preserves existing work and reports duplicate.
	dupBody := strings.NewReader(`{"url":"https://example.com/role/1#frag","title":"Would overwrite"}`)
	dupReq := httptest.NewRequest(http.MethodPost, "http://localhost/api/opportunities", dupBody)
	dupReq.Header.Set("Content-Type", "application/json")
	dupRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(dupRec, dupReq)
	if dupRec.Code != 200 {
		t.Fatalf("duplicate status=%d body=%s", dupRec.Code, dupRec.Body.String())
	}
	var dup map[string]any
	if err := json.Unmarshal(dupRec.Body.Bytes(), &dup); err != nil {
		t.Fatal(err)
	}
	if dup["duplicate"] != true {
		t.Fatalf("expected duplicate flag: %v", dup)
	}
	stored, err := a.jobByID(1)
	if err != nil || stored.Title != "Existing" {
		t.Fatalf("duplicate overwrote job: %+v err=%v", stored, err)
	}

	// Invalid URL is rejected without a write.
	badBody := strings.NewReader(`{"url":"not-a-url"}`)
	badReq := httptest.NewRequest(http.MethodPost, "http://localhost/api/opportunities", badBody)
	badReq.Header.Set("Content-Type", "application/json")
	badRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid accepted: %d %s", badRec.Code, badRec.Body.String())
	}
	jobs, err := a.allJobs()
	if err != nil || len(jobs) != 1 {
		t.Fatalf("invalid write changed jobs: %+v err=%v", jobs, err)
	}
}

func TestOpportunitiesAPIBatchPreservesExisting(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://jobs.example.org/role/1/", Host: "jobs.example.org", Title: "Manual"}); err != nil {
		t.Fatal(err)
	}
	body := strings.NewReader(`{"urls":"https://jobs.example.org/role/1#x\nhttps://jobs.example.org/new?posting=1\nbad-url"}`)
	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/opportunities/import", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("import status=%d body=%s", rec.Code, rec.Body.String())
	}
	var result BatchResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Added != 1 || result.Duplicates != 1 || result.Invalid != 1 {
		t.Fatalf("bad summary: %+v", result)
	}
	// The React client reads lowercase keys; capitalized Go field names
	// previously rendered as undefined and blanked the Wails webview.
	var wire map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &wire); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"items", "added", "duplicates", "invalid"} {
		if _, ok := wire[key]; !ok {
			t.Fatalf("batch response missing lowercase key %q: %s", key, rec.Body.String())
		}
	}
	first, _ := wire["items"].([]any)[0].(map[string]any)
	for _, key := range []string{"line", "url", "status", "detail", "jobId"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("batch item missing lowercase key %q: %s", key, rec.Body.String())
		}
	}
	jobs, err := a.allJobs()
	if err != nil || len(jobs) != 2 || jobs[0].Title != "Manual" {
		t.Fatalf("batch altered existing: %+v err=%v", jobs, err)
	}
}

func TestOpportunitiesAPIDeleteRefusesHistory(t *testing.T) {
	a, _ := testWorkspace(t)
	if _, _, err := a.storeJobIfNew(Job{URL: "https://example.com/deletable", Host: "example.com"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.storeJobIfNew(Job{URL: "https://example.com/keep", Host: "example.com"}); err != nil {
		t.Fatal(err)
	}
	// Give job 2 a submission so bulk delete must refuse.
	if _, err := a.db.Exec(`INSERT INTO submissions (job_id, submitted_at, token) VALUES (2, '2026-01-01T00:00:00Z', 'test-token-1')`); err != nil {
		t.Fatal(err)
	}

	mixed := strings.NewReader(`{"ids":[1,2],"confirm":true}`)
	mixedReq := httptest.NewRequest(http.MethodPost, "http://localhost/api/opportunities/delete", mixed)
	mixedReq.Header.Set("Content-Type", "application/json")
	mixedRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(mixedRec, mixedReq)
	if mixedRec.Code != http.StatusConflict {
		t.Fatalf("history mixed delete accepted: %d %s", mixedRec.Code, mixedRec.Body.String())
	}
	if jobs, _ := a.allJobs(); len(jobs) != 2 {
		t.Fatalf("refused delete removed jobs: %+v", jobs)
	}

	single := strings.NewReader(`{"ids":[1],"confirm":true}`)
	singleReq := httptest.NewRequest(http.MethodPost, "http://localhost/api/opportunities/delete", single)
	singleReq.Header.Set("Content-Type", "application/json")
	singleRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(singleRec, singleReq)
	if singleRec.Code != 200 {
		t.Fatalf("delete status=%d body=%s", singleRec.Code, singleRec.Body.String())
	}
	if jobs, _ := a.allJobs(); len(jobs) != 1 {
		t.Fatalf("delete did not remove job: %+v", jobs)
	}

	// Missing confirmation is rejected.
	unconfirmed := strings.NewReader(`{"ids":[2],"confirm":false}`)
	unReq := httptest.NewRequest(http.MethodPost, "http://localhost/api/opportunities/delete", unconfirmed)
	unReq.Header.Set("Content-Type", "application/json")
	unRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(unRec, unReq)
	if unRec.Code != http.StatusBadRequest {
		t.Fatalf("unconfirmed delete accepted: %d", unRec.Code)
	}
}

func TestOpportunitiesServesReactWhenEmbedded(t *testing.T) {
	a, _ := testWorkspace(t)
	config := testConfig(t, a.db, a.resumeDir)
	config.FrontendDist = fstest.MapFS{
		"frontend/dist/index.html":    &fstest.MapFile{Data: []byte(`<!doctype html><div id="root">React</div>`)},
		"frontend/dist/assets/app.js": &fstest.MapFile{Data: []byte(`js`)},
	}
	b, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Close)

	rec := httptest.NewRecorder()
	newMux(b).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/jobs", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `<div id="root">`) {
		t.Fatalf("react hosting missing: %d %s", rec.Code, rec.Body.String())
	}

	assetRec := httptest.NewRecorder()
	newMux(b).ServeHTTP(assetRec, httptest.NewRequest(http.MethodGet, "http://localhost/app/assets/app.js", nil))
	if assetRec.Code != 200 {
		t.Fatalf("asset hosting missing: %d", assetRec.Code)
	}

	// Without an embedded bundle the legacy Go page remains.
	legacyRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(legacyRec, httptest.NewRequest(http.MethodGet, "http://localhost/jobs", nil))
	if legacyRec.Code != 200 || !strings.Contains(legacyRec.Body.String(), "Opportunities") {
		t.Fatalf("legacy fallback missing: %d", legacyRec.Code)
	}
}

func TestOpportunityOpenAPIRequiresDesktop(t *testing.T) {
	a, _ := testWorkspace(t)
	if _, _, err := a.storeJobIfNew(Job{URL: "https://example.com/role/9", Host: "example.com"}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "http://localhost/api/opportunities/1/open", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("browser-mode open should 404: %d", rec.Code)
	}
}
