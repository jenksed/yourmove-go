package app

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testPDFBytes() []byte {
	return []byte("%PDF-1.4\nresumes-api-fixture")
}

func multipartResume(t *testing.T, field, filename string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}

func decodeAPIError(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		return strings.TrimSpace(rec.Body.String())
	}
	return payload["error"]
}

func TestResumesAPIImportListSelectPersisted(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/role/api", Host: "example.com", Title: "API Role"}); err != nil {
		t.Fatal(err)
	}

	// Import through the React transport.
	body, contentType := multipartResume(t, "resume", "api.pdf", testPDFBytes())
	importReq := httptest.NewRequest(http.MethodPost, "http://localhost/api/resumes/import", body)
	importReq.Header.Set("Content-Type", contentType)
	importRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(importRec, importReq)
	if importRec.Code != http.StatusCreated {
		t.Fatalf("import status=%d error=%s", importRec.Code, decodeAPIError(t, importRec))
	}
	var imported struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		Duplicate bool   `json:"duplicate"`
	}
	if err := json.Unmarshal(importRec.Body.Bytes(), &imported); err != nil || imported.ID == 0 {
		t.Fatalf("bad import response: %s err=%v", importRec.Body.String(), err)
	}
	if imported.Duplicate {
		t.Fatalf("first import reported duplicate: %+v", imported)
	}

	// Duplicate bytes through the same transport return the same record.
	dupBody, dupType := multipartResume(t, "resume", "renamed.pdf", testPDFBytes())
	dupReq := httptest.NewRequest(http.MethodPost, "http://localhost/api/resumes/import", dupBody)
	dupReq.Header.Set("Content-Type", dupType)
	dupRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(dupRec, dupReq)
	if dupRec.Code != http.StatusOK {
		t.Fatalf("duplicate status=%d error=%s", dupRec.Code, decodeAPIError(t, dupRec))
	}
	var dup struct {
		ID        int64 `json:"id"`
		Duplicate bool  `json:"duplicate"`
	}
	if err := json.Unmarshal(dupRec.Body.Bytes(), &dup); err != nil {
		t.Fatal(err)
	}
	if !dup.Duplicate || dup.ID != imported.ID {
		t.Fatalf("duplicate not detected: %+v want id=%d", dup, imported.ID)
	}

	// List through the React transport exposes the stored document.
	listRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "http://localhost/api/resumes", nil))
	if listRec.Code != 200 {
		t.Fatalf("list status=%d error=%s", listRec.Code, decodeAPIError(t, listRec))
	}
	var list struct {
		Resumes []resumeJSON      `json:"resumes"`
		Total   int               `json:"total"`
		Desktop bool              `json:"desktop"`
		Families []resumeFamilyJSON `json:"families"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 || len(list.Resumes) != 1 || list.Resumes[0].ID != imported.ID {
		t.Fatalf("unexpected list: %s", listRec.Body.String())
	}
	if ct := listRec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("wrong content type: %q", ct)
	}
	if listRec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("missing no-store: %v", listRec.Header())
	}
	// Lowercase wire keys: the React client reads them verbatim.
	var wire map[string]any
	if err := json.Unmarshal(listRec.Body.Bytes(), &wire); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"resumes", "families", "desktop", "total"} {
		if _, ok := wire[key]; !ok {
			t.Fatalf("list missing lowercase key %q: %s", key, listRec.Body.String())
		}
	}

	// Resume options mirror the Prepare activity: one selectable document.
	optionsRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(optionsRec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/resume-options", nil))
	if optionsRec.Code != 200 {
		t.Fatalf("options status=%d error=%s", optionsRec.Code, decodeAPIError(t, optionsRec))
	}
	var options struct {
		Options []resumeOptionJSON `json:"options"`
		Selected int64             `json:"selectedResumeId"`
		Job      map[string]any    `json:"job"`
	}
	if err := json.Unmarshal(optionsRec.Body.Bytes(), &options); err != nil {
		t.Fatal(err)
	}
	if len(options.Options) != 1 || options.Options[0].ID != imported.ID {
		t.Fatalf("unexpected options: %s", optionsRec.Body.String())
	}

	// Select through the React transport and confirm persistence.
	assignReq := httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/resume", strings.NewReader(`{"resume_id":1}`))
	assignReq.Header.Set("Content-Type", "application/json")
	assignRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(assignRec, assignReq)
	if assignRec.Code != 200 {
		t.Fatalf("assign status=%d error=%s", assignRec.Code, decodeAPIError(t, assignRec))
	}
	job, err := a.jobByID(1)
	if err != nil || job.SelectedResumeID != imported.ID {
		t.Fatalf("assignment not persisted: %+v err=%v", job, err)
	}

	// Fabricated resume IDs are refused without changing the selection.
	badAssign := httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/resume", strings.NewReader(`{"resume_id":9999}`))
	badAssign.Header.Set("Content-Type", "application/json")
	badRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(badRec, badAssign)
	if badRec.Code != http.StatusNotFound {
		t.Fatalf("fabricated assign accepted: %d %s", badRec.Code, badRec.Body.String())
	}
	if job, _ := a.jobByID(1); job.SelectedResumeID != imported.ID {
		t.Fatalf("refused assign changed selection: %+v", job)
	}
}

func TestResumesAPIMasterArchiveDeleteRefusals(t *testing.T) {
	a, _ := testWorkspace(t)
	body, contentType := multipartResume(t, "resume", "refusal.pdf", testPDFBytes())
	importReq := httptest.NewRequest(http.MethodPost, "http://localhost/api/resumes/import", body)
	importReq.Header.Set("Content-Type", contentType)
	importRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(importRec, importReq)
	if importRec.Code != http.StatusCreated {
		t.Fatalf("import status=%d %s", importRec.Code, importRec.Body.String())
	}
	var imported struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(importRec.Body.Bytes(), &imported); err != nil {
		t.Fatal(err)
	}
	postJSON := func(url, payload string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		newMux(a).ServeHTTP(rec, req)
		return rec
	}

	// Referenced documents are refused: archive instead.
	if err := a.saveJob(Job{URL: "https://example.com/role/ref", Host: "example.com", Title: "Ref"}); err != nil {
		t.Fatal(err)
	}
	assignRec := postJSON("http://localhost/api/jobs/1/resume", `{"resume_id":1}`)
	if assignRec.Code != 200 {
		t.Fatalf("assign status=%d %s", assignRec.Code, assignRec.Body.String())
	}
	usedDelete := postJSON("http://localhost/api/resumes/1/delete", `{"confirm":true,"confirmPhrase":"DELETE"}`)
	if usedDelete.Code != http.StatusConflict {
		t.Fatalf("in-use delete allowed: %d %s", usedDelete.Code, usedDelete.Body.String())
	}

	// Clear the assignment, then designate master through the API. A master
	// resume is career evidence, never an automatic employer attachment.
	clearRec := postJSON("http://localhost/api/jobs/1/resume", `{"resume_id":0}`)
	if clearRec.Code != 200 {
		t.Fatalf("clear status=%d %s", clearRec.Code, clearRec.Body.String())
	}
	masterRec := postJSON("http://localhost/api/resumes/1/master", `{"master":true}`)
	if masterRec.Code != 200 {
		t.Fatalf("master status=%d %s", masterRec.Code, masterRec.Body.String())
	}

	// Master documents cannot be deleted.
	deleteRec := postJSON("http://localhost/api/resumes/1/delete", `{"confirm":true,"confirmPhrase":"DELETE"}`)
	if deleteRec.Code != http.StatusConflict {
		t.Fatalf("master delete allowed: %d %s", deleteRec.Code, deleteRec.Body.String())
	}

	// Unconfirmed deletion is rejected without a write.
	unconfirmed := postJSON("http://localhost/api/resumes/1/delete", `{"confirm":false,"confirmPhrase":"DELETE"}`)
	if unconfirmed.Code != http.StatusBadRequest {
		t.Fatalf("unconfirmed delete accepted: %d", unconfirmed.Code)
	}

	// Remove the master designation and re-select so the archive test below
	// preserves a live job association.
	unmasterRec := postJSON("http://localhost/api/resumes/1/master", `{"master":false}`)
	if unmasterRec.Code != 200 {
		t.Fatalf("unmaster status=%d %s", unmasterRec.Code, unmasterRec.Body.String())
	}
	reassignRec := postJSON("http://localhost/api/jobs/1/resume", `{"resume_id":1}`)
	if reassignRec.Code != 200 {
		t.Fatalf("reassign status=%d %s", reassignRec.Code, reassignRec.Body.String())
	}

	// Archiving preserves the job association and submission evidence path.
	archiveRec := postJSON("http://localhost/api/resumes/1/archive", `{"action":"archive","confirm":true}`)
	if archiveRec.Code != 200 {
		t.Fatalf("archive status=%d %s", archiveRec.Code, archiveRec.Body.String())
	}
	if job, _ := a.jobByID(1); job.SelectedResumeID != imported.ID {
		t.Fatalf("archive changed job assignment: %+v", job)
	}
	if resume, _ := a.resumeByID(1); resume.ArchivedAt == "" {
		t.Fatalf("archive did not persist: %+v", resume)
	}
}

func TestResumesAPIReviseKeepsHistoryPinned(t *testing.T) {
	a, _ := testWorkspace(t)
	first, firstType := multipartResume(t, "resume", "v1.pdf", []byte("%PDF-1.4\nfirst version bytes"))
	firstReq := httptest.NewRequest(http.MethodPost, "http://localhost/api/resumes/import", first)
	firstReq.Header.Set("Content-Type", firstType)
	firstRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("import status=%d %s", firstRec.Code, firstRec.Body.String())
	}
	var imported struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(firstRec.Body.Bytes(), &imported); err != nil {
		t.Fatal(err)
	}
	if err := a.saveJob(Job{URL: "https://example.com/role/rev", Host: "example.com", Title: "Rev"}); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, imported.ID); err != nil {
		t.Fatal(err)
	}

	// Identical bytes are refused as a revision.
	sameBody, sameType := multipartResume(t, "resume", "same.pdf", []byte("%PDF-1.4\nfirst version bytes"))
	sameReq := httptest.NewRequest(http.MethodPost, "http://localhost/api/resumes/1/revise", sameBody)
	sameReq.Header.Set("Content-Type", sameType)
	sameRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(sameRec, sameReq)
	if sameRec.Code != http.StatusConflict {
		t.Fatalf("duplicate revision allowed: %d %s", sameRec.Code, sameRec.Body.String())
	}

	// New bytes publish a new version; the job stays pinned to the old document.
	nextBody, nextType := multipartResume(t, "resume", "v2.pdf", []byte("%PDF-1.4\nsecond version bytes!!"))
	nextReq := httptest.NewRequest(http.MethodPost, "http://localhost/api/resumes/1/revise", nextBody)
	nextReq.Header.Set("Content-Type", nextType)
	nextRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(nextRec, nextReq)
	if nextRec.Code != http.StatusCreated {
		t.Fatalf("revise status=%d %s", nextRec.Code, nextRec.Body.String())
	}
	var revised struct {
		ID      int64 `json:"id"`
		Version int   `json:"version"`
	}
	if err := json.Unmarshal(nextRec.Body.Bytes(), &revised); err != nil || revised.ID == imported.ID {
		t.Fatalf("bad revision response: %s err=%v", nextRec.Body.String(), err)
	}
	if job, _ := a.jobByID(1); job.SelectedResumeID != imported.ID {
		t.Fatalf("revision reassigned job: %+v", job)
	}
	if old, _ := a.resumeByID(imported.ID); old.ArchivedAt == "" {
		t.Fatalf("previous version not archived: %+v", old)
	}
	if fresh, _ := a.resumeByID(revised.ID); string(mustReadResume(t, a, fresh)) == "" {
		t.Fatalf("revised bytes missing")
	}
}

func mustReadResume(t *testing.T, a *App, resume Resume) []byte {
	t.Helper()
	data, err := a.readResumeBytes(resume)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestResumesAPIFamilyRoutingAndOptionsFiltering(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveFamily(RoleFamily{Name: "Support", TitleTerms: "support engineer"}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveJob(Job{URL: "https://example.com/role/fam", Host: "example.com", Title: "Support Engineer"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"fam-a.pdf", "fam-b.pdf"} {
		body, contentType := multipartResume(t, "resume", name, []byte("%PDF-1.4\n"+name+" bytes"))
		req := httptest.NewRequest(http.MethodPost, "http://localhost/api/resumes/import", body)
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()
		newMux(a).ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("import %s status=%d %s", name, rec.Code, rec.Body.String())
		}
	}
	postJSON := func(url, payload string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		newMux(a).ServeHTTP(rec, req)
		return rec
	}

	// Unknown families are refused.
	if rec := postJSON("http://localhost/api/resumes/1/family", `{"family_id":999,"primary":false}`); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown family accepted: %d %s", rec.Code, rec.Body.String())
	}
	// Route resume 1 as the family primary through the API.
	if rec := postJSON("http://localhost/api/resumes/1/family", `{"family_id":1,"primary":true}`); rec.Code != 200 {
		t.Fatalf("family status=%d %s", rec.Code, rec.Body.String())
	}
	if resume, _ := a.resumeByID(1); resume.RoleFamilyID != 1 || !resume.IsPrimary {
		t.Fatalf("routing not persisted: %+v", resume)
	}

	// Masters never appear as selectable employer attachments.
	if rec := postJSON("http://localhost/api/resumes/2/master", `{"master":true}`); rec.Code != 200 {
		t.Fatalf("master status=%d %s", rec.Code, rec.Body.String())
	}
	optionsRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(optionsRec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/resume-options", nil))
	var options struct {
		Options []resumeOptionJSON `json:"options"`
	}
	if err := json.Unmarshal(optionsRec.Body.Bytes(), &options); err != nil {
		t.Fatal(err)
	}
	if len(options.Options) != 1 || options.Options[0].ID != 1 {
		t.Fatalf("master leaked into options: %s", optionsRec.Body.String())
	}

	// Archived versions appear only when already selected for that job.
	if rec := postJSON("http://localhost/api/resumes/1/archive", `{"action":"archive","confirm":true}`); rec.Code != 200 {
		t.Fatalf("archive status=%d %s", rec.Code, rec.Body.String())
	}
	emptyRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(emptyRec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/resume-options", nil))
	var empty struct {
		Options []resumeOptionJSON `json:"options"`
	}
	if err := json.Unmarshal(emptyRec.Body.Bytes(), &empty); err != nil {
		t.Fatal(err)
	}
	if len(empty.Options) != 0 {
		t.Fatalf("archived listed without selection: %s", emptyRec.Body.String())
	}
}

func TestResumesServesReactWhenEmbedded(t *testing.T) {
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
	newMux(b).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/resumes", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `<div id="root">`) {
		t.Fatalf("react hosting missing: %d %s", rec.Code, rec.Body.String())
	}

	// Without an embedded bundle the legacy Go document library remains.
	legacyRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(legacyRec, httptest.NewRequest(http.MethodGet, "http://localhost/resumes", nil))
	if legacyRec.Code != 200 || !strings.Contains(legacyRec.Body.String(), "Documents") {
		t.Fatalf("legacy fallback missing: %d %s", legacyRec.Code, legacyRec.Body.String())
	}
}

func TestResumesAPINativeRequiresDesktop(t *testing.T) {
	a, _ := testWorkspace(t)
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "http://localhost/api/resumes/import-native", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("browser-mode native import should 404: %d %s", rec.Code, rec.Body.String())
	}
}
