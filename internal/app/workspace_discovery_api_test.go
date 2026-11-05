package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/discovery"
	"github.com/jenksed/yourmove-go/internal/application/providers/greenhouse"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

type discoveryTransport func(*http.Request) (*http.Response, error)

func (f discoveryTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWorkspacePublicQuestionDiscovery(t *testing.T) {
	a, _ := testWorkspace(t)
	posting := "https://job-boards.greenhouse.io/acme/jobs/1234"
	if err := a.saveJob(Job{URL: posting, Host: "job-boards.greenhouse.io", Title: "Synthetic"}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	registry := discovery.Registry{Greenhouse: greenhouse.New(&http.Client{Transport: discoveryTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != http.MethodGet || r.URL.String() != "https://boards-api.greenhouse.io/v1/boards/acme/jobs/1234?questions=true" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"questions":[{"label":"Why this role?","fields":[{"name":"question_12","type":"textarea"}]}]}`))}, nil
	})})}
	a.inspectApplication = func(ctx context.Context, raw string) (result schema.Result, err error) {
		return registry.Inspect(ctx, raw)
	}
	start := time.Now()
	for i := 0; i < 2; i++ {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/questions/discover", nil)
		newMux(a).ServeHTTP(response, request)
		if response.Code != 200 || !strings.Contains(response.Body.String(), `"available"`) {
			t.Fatalf("discovery: %d %s", response.Code, response.Body.String())
		}
	}
	if calls != 1 {
		t.Fatalf("public requests: %d", calls)
	}
	item, err := a.latestValidInspectionForPosting(1, "public_api", posting)
	if err != nil || len(item.Fields) != 1 || item.Fields[0].Label != "Why this role?" {
		t.Fatalf("inventory: %+v %v", item, err)
	}
	t.Logf("Controlled Greenhouse public discovery completed in %s; no repeated request", time.Since(start))
}

func TestWorkspaceDiscoveryUnsupportedDoesNotBlockContinuation(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/synthetic-job", Host: "example.com", Title: "Synthetic"}); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	newMux(a).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/questions/discover", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"unsupported"`) {
		t.Fatalf("unsupported discovery: %d %s", response.Code, response.Body.String())
	}
	continuation := httptest.NewRecorder()
	newMux(a).ServeHTTP(continuation, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/continuation", nil))
	if continuation.Code != 200 {
		t.Fatalf("continuation blocked: %d %s", continuation.Code, continuation.Body.String())
	}
}
