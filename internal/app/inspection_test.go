package app

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func TestProfileApprovalAndExactMappings(t *testing.T) {
	a, _ := testWorkspace(t)
	p := CandidateProfile{FirstName: "Jane", LastName: "Applicant", Email: "jane@example.test", Phone: "555-0100", Approved: true}
	if err := a.saveCandidateProfile(p); err != nil {
		t.Fatal(err)
	}
	saved, err := a.candidateProfile()
	if err != nil || !saved.Approved || saved.Revision != 1 {
		t.Fatalf("profile %+v err %v", saved, err)
	}
	if got := mapProfileField(schema.Field{ID: "application:first_name", Kind: "input_text"}, saved); got != "approved profile" {
		t.Fatal(got)
	}
	if got := mapProfileField(schema.Field{ID: "application:question_92", Label: "First name", Kind: "input_text"}, saved); got != "manual" {
		t.Fatal(got)
	}
	if got := mapProfileField(schema.Field{ID: "application:first_name", Kind: "input_text", Sensitive: true}, saved); got != "manual" {
		t.Fatal(got)
	}
	saved.Approved = false
	if err := a.saveCandidateProfile(saved); err != nil {
		t.Fatal(err)
	}
	later, _ := a.candidateProfile()
	if later.Approved || later.Revision != 2 {
		t.Fatalf("approval not revoked: %+v", later)
	}
}
func TestInspectionPersistenceAndNoTransmission(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://job-boards.greenhouse.io/acme/jobs/123", Title: "Role", Host: "job-boards.greenhouse.io"}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	a.inspectApplication = func(_ context.Context, raw string) (schema.Result, error) {
		calls++
		if raw != "https://job-boards.greenhouse.io/acme/jobs/123" {
			t.Fatal(raw)
		}
		return schema.Result{Platform: "greenhouse", Source: "public_api", Completeness: "public_schema", ApplicationURL: raw, Fields: []schema.Field{{ID: "application:first_name", Label: "First Name", Kind: "input_text", Required: true}, {ID: "application:consent", Label: "Consent", Kind: "input_text", Sensitive: true}}, Steps: []schema.Step{{Key: "initial", Label: "Application", State: "anticipated", Action: "inspect"}}}, nil
	}
	handler := newMux(a)
	rec := postApp(handler, "/jobs/1/inspect", url.Values{}, "", "")
	if rec.Code != 303 || rec.Header().Get("Location") != "/jobs/1/inspection" {
		t.Fatalf("POST %d %s", rec.Code, rec.Body.String())
	}
	rec = postApp(handler, "/jobs/1/inspect", url.Values{}, "", "")
	if rec.Code != 303 || calls != 2 {
		t.Fatalf("repeat inspection %d, calls %d", rec.Code, calls)
	}
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM application_inspections WHERE job_id=1`).Scan(&n)
	if n != 1 {
		t.Fatalf("dedupe failed %d", n)
	}
	item, err := a.latestInspection(1)
	if err != nil || len(item.Fields) != 2 || len(item.Steps) != 1 {
		t.Fatalf("load %+v %v", item, err)
	}
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest("GET", "http://localhost/jobs/1/inspection", nil))
	if get.Code != 200 || !strings.Contains(get.Body.String(), "First Name") || !strings.Contains(get.Body.String(), "Consent") {
		t.Fatalf("render %d %s", get.Code, get.Body.String())
	}
	desktop := postApp(newDesktopMux(a), "/jobs/1/inspect", url.Values{}, "wails://localhost", "1")
	if desktop.Code != 200 || !strings.Contains(desktop.Body.String(), `window.location.replace("/jobs/1/inspection")`) {
		t.Fatalf("desktop %d %s", desktop.Code, desktop.Body.String())
	}
	if _, err := a.deleteJobs([]int64{1}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := a.latestInspection(1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted job retains inspection %v", err)
	}
}
func TestProfileSaveRouteAndCrossOriginGuard(t *testing.T) {
	a, _ := testWorkspace(t)
	f := url.Values{"first_name": {"A"}, "last_name": {"B"}, "email": {"a@example.org"}, "approve": {"yes"}}
	result := postApp(newMux(a), "/candidate", f, "https://evil.test", "")
	if result.Code != 403 {
		t.Fatal(result.Code)
	}
	good := postApp(newDesktopMux(a), "/candidate", f, "wails://localhost", "1")
	if good.Code != http.StatusOK || !strings.Contains(good.Body.String(), `window.location.replace("/candidate")`) {
		t.Fatalf("response %d %s", good.Code, good.Body.String())
	}
}

func TestRequirementsInventoryRendersLatestOnly(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://job-boards.greenhouse.io/acme/jobs/44", Host: "job-boards.greenhouse.io"}); err != nil {
		t.Fatal(err)
	}
	first := schema.Result{Platform: "greenhouse", Source: "public_api", Completeness: "public_schema", Fields: []schema.Field{{ID: "application:question_1", Label: "Why this role?", Kind: "textarea"}}}
	if _, err := a.saveInspection(1, first); err != nil {
		t.Fatal(err)
	}
	first.Fields = []schema.Field{{ID: "application:question_2", Label: "Why this company?", Kind: "textarea"}}
	if _, err := a.saveInspection(1, first); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	newMux(a).ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/requirements", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Why this company?") || strings.Contains(w.Body.String(), "Why this role?") {
		t.Fatalf("requirements %d %s", w.Code, w.Body.String())
	}
}

func TestReinspectionOfPreviouslySeenSchemaBecomesLatest(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://job-boards.greenhouse.io/acme/jobs/99", Host: "job-boards.greenhouse.io"}); err != nil {
		t.Fatal(err)
	}
	first := schema.Result{Platform: "greenhouse", Source: "public_api", Completeness: "public_schema", Fields: []schema.Field{{ID: "a", Label: "One", Kind: "input_text"}}}
	second := schema.Result{Platform: "greenhouse", Source: "public_api", Completeness: "public_schema", Fields: []schema.Field{{ID: "b", Label: "Two", Kind: "input_text"}}}
	if _, err := a.saveInspection(1, first); err != nil {
		t.Fatal(err)
	}
	if _, err := a.saveInspection(1, second); err != nil {
		t.Fatal(err)
	}
	if _, err := a.saveInspection(1, first); err != nil {
		t.Fatal(err)
	}
	latest, err := a.latestInspection(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(latest.Fields) != 1 || latest.Fields[0].Label != "One" {
		t.Fatalf("did not prefer latest reinspection %+v", latest)
	}
}
