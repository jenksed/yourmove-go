package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The Opportunities list must show a truthful source indicator per posting:
// the Go-owned ATS platform for recognized providers (including Recruitee),
// a neutral marker for unknown URLs, employer identity kept separate, and no
// implication of capture/Fill support. React renders the API fields verbatim;
// the Go-rendered fallback uses the same Go-owned values.
func TestOpportunitiesListExposesSourceProvider(t *testing.T) {
	a, _ := testWorkspace(t)
	for _, job := range []Job{
		{URL: "https://jobs.lever.co/example/0e4b8b1e-1234-4a5b-8c9d-0e1f2a3b4c5d", Host: "jobs.lever.co", Title: "Lever role", Company: "Lever Co"},
		{URL: "https://example.recruitee.com/o/platform-engineer", Host: "example.recruitee.com", Title: "Recruitee role", Company: "Example Robotics"},
		{URL: "https://example.com/role", Host: "example.com", Title: "Unknown role"},
	} {
		if err := a.saveJob(job); err != nil {
			t.Fatal(err)
		}
	}
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/api/opportunities", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"provider":"lever"`, `"providerLabel":"Lever"`, `"provider":"recruitee"`, `"providerLabel":"Recruitee"`, `"provider":""`, `"providerLabel":""`} {
		if !strings.Contains(body, want) {
			t.Fatalf("API missing %s: %s", want, body)
		}
	}
	// The Go-rendered fallback carries the same Go-owned indicators.
	page := httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest(http.MethodGet, "http://localhost/jobs", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("jobs status=%d", page.Code)
	}
	rendered := page.Body.String()
	for _, want := range []string{"Lever", "Recruitee", "source-badge", "source-unknown"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("Go list missing %q", want)
		}
	}
	if strings.Contains(rendered, "automatic Fill") {
		t.Fatal("indicator must not imply Fill support")
	}
}
