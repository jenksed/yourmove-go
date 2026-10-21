package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRoleFamilyExampleRoutingAndPersistence(t *testing.T) {
	a, dir := testWorkspace(t)
	count, err := a.installExampleFamilies()
	if err != nil || count != 8 {
		t.Fatalf("install examples: count=%d err=%v", count, err)
	}
	again, err := a.installExampleFamilies()
	if err != nil || again != 0 {
		t.Fatalf("examples not idempotent: count=%d err=%v", again, err)
	}
	families, err := a.allFamilies()
	if err != nil {
		t.Fatal(err)
	}
	var techID, linuxID int64
	for _, f := range families {
		switch f.Name {
		case "Technical Support Engineering":
			techID = f.ID
		case "Linux & Hosting Support":
			linuxID = f.ID
		}
	}
	if techID == 0 || linuxID == 0 {
		t.Fatal("missing example families")
	}
	original, err := a.importResume("Support_Editorial.pdf", bytes.NewReader([]byte("%PDF-1.4\nSupport editorial")))
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.importResume("Support_Modern.pdf", bytes.NewReader([]byte("%PDF-1.4\nSupport modern")))
	if err != nil {
		t.Fatal(err)
	}
	linux, err := a.importResume("Linux.pdf", bytes.NewReader([]byte("%PDF-1.4\nLinux")))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResumeFamily(original.ID, techID, false); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResumeFamily(second.ID, techID, true); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResumeFamily(linux.ID, linuxID, true); err != nil {
		t.Fatal(err)
	}
	resumes, err := a.allResumes()
	if err != nil {
		t.Fatal(err)
	}
	support := matchResume(Job{Title: "Senior Technical Support Engineer", Description: "Debugging REST APIs, Python and application logs"}, families, resumes)
	if support.Status != "matched" || support.ResumeID != second.ID || support.FamilyName != "Technical Support Engineering" || support.TitleEvidence != "technical support engineer" {
		t.Fatalf("support match: %+v", support)
	}
	linuxMatch := matchResume(Job{Title: "Linux Systems Administrator", Description: "Linux NGINX DNS and MySQL"}, families, resumes)
	if linuxMatch.Status != "matched" || linuxMatch.ResumeID != linux.ID {
		t.Fatalf("linux match: %+v", linuxMatch)
	}
	// Linux-flavored support must outrank the generic support-engineer title phrase.
	overlap := matchResume(Job{Title: "Senior Linux Technical Support Engineer", Description: "Linux SSH and DNS"}, families, resumes)
	if overlap.Status != "matched" || overlap.ResumeID != linux.ID {
		t.Fatalf("specific linux title should win: %+v", overlap)
	}
	noTitle := matchResume(Job{Title: "", Description: "Support engineer, logs, APIs"}, families, resumes)
	if noTitle.Status != "no-title" {
		t.Fatalf("missing title: %+v", noTitle)
	}
	if err := a.assignResumeFamily(second.ID, techID, false); err != nil {
		t.Fatal(err)
	}
	resumes, _ = a.allResumes()
	noPrimary := matchResume(Job{Title: "Technical Support Engineer"}, families, resumes)
	if noPrimary.Status != "choose-primary" {
		t.Fatalf("multiple resumes not resolved safely: %+v", noPrimary)
	}
	if err := a.assignResumeFamily(original.ID, techID, true); err != nil {
		t.Fatal(err)
	}
	resumes, _ = a.allResumes()
	revised := matchResume(Job{Title: "Technical Support Engineer"}, families, resumes)
	if revised.ResumeID != original.ID {
		t.Fatalf("preferred selection not used: %+v", revised)
	}
	// A recommended document never overwrites a job's actual manual selection.
	if err := a.saveJob(Job{URL: "https://example.com/one", Host: "example.com", Title: "Technical Support Engineer"}); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, linux.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.recordSubmission(1, "confirmed", reviewedSubmissionToken(t, a, 1)); err != nil {
		t.Fatal(err)
	}
	if err := a.deleteFamily(techID); err != nil {
		t.Fatal(err)
	}
	job, _ := a.jobByID(1)
	record, _ := a.jobSubmissions(1)
	if job.SelectedResumeID != linux.ID || len(record) != 1 || record[0].ResumeID != linux.ID {
		t.Fatalf("family cleanup touched application records: %+v %+v", job, record)
	}
	reread, err := openDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reread.Close()
	others, err := (&App{db: reread}).allResumes()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range others {
		if r.ID == original.ID && r.RoleFamilyID != 0 {
			t.Fatalf("deleted family retained: %+v", r)
		}
	}
}

func TestLiteralBoundariesExclusionsAndAmbiguousAbstention(t *testing.T) {
	f1 := RoleFamily{ID: 1, Name: "Go developer", TitleTerms: "go", ContextTerms: "backend"}
	f2 := RoleFamily{ID: 2, Name: "Backend developer", TitleTerms: "go", ContextTerms: "backend"}
	resumes := []Resume{{ID: 1, Name: "Go.pdf", RoleFamilyID: 1}}
	if result := matchResume(Job{Title: "Django developer", Description: "Go language"}, []RoleFamily{f1}, resumes); result.Status != "no-family" {
		t.Fatalf("substring match: %+v", result)
	}
	if result := matchResume(Job{Title: "Developer", Description: "Go backend"}, []RoleFamily{f1}, resumes); result.Status != "no-family" {
		t.Fatalf("context-only match: %+v", result)
	}
	if result := matchResume(Job{Title: "Go Developer", Description: "backend"}, []RoleFamily{f1, f2}, resumes); result.Status != "ambiguous" {
		t.Fatalf("ties must abstain: %+v", result)
	}
	f1.ExcludeTerms = "senior"
	if result := matchResume(Job{Title: "Senior Go Developer"}, []RoleFamily{f1}, resumes); result.Status != "no-family" {
		t.Fatalf("exclusion ignored: %+v", result)
	}
	if result := matchResume(Job{Title: "Go Developer"}, []RoleFamily{f1}, resumes); result.Status != "matched" {
		t.Fatalf("valid eligible rule: %+v", result)
	}
}

func TestFamilyValidationRejectsBadRules(t *testing.T) {
	for _, f := range []RoleFamily{
		{Name: "x", TitleTerms: "engineer"},
		{Name: "Example", TitleTerms: ""},
		{Name: "Example", TitleTerms: "a"},
		{Name: "Example\nOther", TitleTerms: "engineer"},
		{Name: "Example", TitleTerms: func() string {
			var terms []string
			for i := 0; i < 31; i++ {
				terms = append(terms, fmt.Sprintf("engineer %d", i))
			}
			return strings.Join(terms, "\n")
		}()},
	} {
		if _, err := validateFamily(f); err == nil {
			t.Fatalf("accepted invalid family %+v", f)
		}
	}
}

func TestRoleFamilyHTTPAndWailsRouting(t *testing.T) {
	a, _ := testWorkspace(t)
	ordinary := newMux(a)
	desktop := newDesktopMux(a)
	for _, target := range []string{"/utilities", "/families", "/resumes"} {
		w := httptest.NewRecorder()
		ordinary.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://localhost"+target, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s: %d %s", target, w.Code, w.Body.String())
		}
	}
	create := postApp(ordinary, "/families", url.Values{"name": {"Technical Support Engineering"}, "title_terms": {"technical support engineer"}, "context_terms": {"api\nlogs"}}, "", "")
	if create.Code != 303 || create.Header().Get("Location") != "/families" {
		t.Fatalf("browser create: %d %s", create.Code, create.Body.String())
	}
	edit := postApp(desktop, "/families/1/edit", url.Values{"name": {"Technical Support Engineering"}, "title_terms": {"technical support engineer\nsupport engineer"}, "context_terms": {"api\nlogs"}}, "wails://localhost", "1")
	if edit.Code != 200 || !strings.Contains(edit.Body.String(), `window.location.replace("/families")`) {
		t.Fatalf("desktop edit: %d %s", edit.Code, edit.Body.String())
	}
	if _, err := a.importResume("cv.pdf", bytes.NewReader([]byte("%PDF-1.4\nCV"))); err != nil {
		t.Fatal(err)
	}
	assign := postApp(desktop, "/resumes/1/family", url.Values{"family_id": {"1"}, "primary": {"yes"}}, "wails://localhost", "1")
	if assign.Code != 200 || !strings.Contains(assign.Body.String(), `window.location.replace("/resumes")`) {
		t.Fatalf("desktop routing: %d %s", assign.Code, assign.Body.String())
	}
	resumes, err := a.allResumes()
	if err != nil || len(resumes) != 1 || !resumes[0].IsPrimary {
		t.Fatalf("no primary persisted: %+v %v", resumes, err)
	}
	forged := postApp(ordinary, "/families", url.Values{"name": {"Untrusted"}, "title_terms": {"anything"}}, "https://untrusted.test", "")
	if forged.Code != 403 {
		t.Fatalf("cross origin family mutation: %d", forged.Code)
	}
	invalid := postApp(ordinary, "/resumes/1/family", url.Values{"family_id": {"9999"}}, "", "")
	if invalid.Code != 404 {
		t.Fatalf("unknown family accepted: %d", invalid.Code)
	}
	preview := postApp(ordinary, "/families/test", url.Values{"title": {"Technical Support Engineer"}, "description": {"debug API logs"}}, "", "")
	if preview.Code != 200 || !strings.Contains(preview.Body.String(), "cv.pdf") {
		t.Fatalf("matching preview failed: %d %s", preview.Code, preview.Body.String())
	}
	a.saveJob(Job{URL: "https://example.com/eng", Host: "example.com", Title: "Technical Support Engineer"})
	w := httptest.NewRecorder()
	ordinary.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Use suggested resume") {
		t.Fatalf("job suggestion: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	ordinary.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/jobs", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Suggested: cv.pdf") {
		t.Fatalf("list suggestion: %d %s", w.Code, w.Body.String())
	}
}

func TestFamilyMigrationFromExistingDocumentSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE resumes (id INTEGER PRIMARY KEY,name TEXT NOT NULL,stored_name TEXT NOT NULL UNIQUE,sha256 TEXT NOT NULL UNIQUE,size INTEGER NOT NULL,kind TEXT NOT NULL,imported_at TEXT NOT NULL);
        INSERT INTO resumes VALUES (1,'Existing.pdf','one.pdf','hash',123,'.pdf','2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	migrated, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := (&App{db: migrated}).resumeByID(1)
	if err != nil || r.Name != "Existing.pdf" || r.RoleFamilyID != 0 {
		t.Fatalf("legacy resume lost: %+v %v", r, err)
	}
	migrated.Close()
	reopened, err := openDB(path)
	if err != nil {
		t.Fatalf("migration not idempotent: %v", err)
	}
	reopened.Close()
}

func TestExampleRulesRouteTechnicalLinuxAndCustomerSuccess(t *testing.T) {
	families := append([]RoleFamily(nil), exampleFamilies...)
	var support, linux, success int64
	for i := range families {
		families[i].ID = int64(i + 1)
		switch families[i].Name {
		case "Technical Support Engineering":
			support = families[i].ID
		case "Linux & Hosting Support":
			linux = families[i].ID
		case "Customer Success":
			success = families[i].ID
		}
	}
	documents := []Resume{{ID: 7, Name: "Support.pdf", RoleFamilyID: support}, {ID: 8, Name: "Linux.pdf", RoleFamilyID: linux}, {ID: 9, Name: "Success.pdf", RoleFamilyID: success}}
	cases := []struct {
		title, description string
		want               int64
	}{
		{"Senior Technical Support Engineer", "Investigate APIs, logs and Python errors", 7},
		{"Senior Linux Technical Support Engineer", "Linux hosting with SSH and DNS", 8},
		{"Linux Systems Administrator", "Nginx and MySQL", 8},
		{"Customer Success Manager", "Customer onboarding and renewals", 9},
	}
	for _, tc := range cases {
		result := matchResume(Job{Title: tc.title, Description: tc.description}, families, documents)
		if result.Status != "matched" || result.ResumeID != tc.want {
			t.Errorf("%q matched %+v; want resume ID %d", tc.title, result, tc.want)
		}
	}
}
