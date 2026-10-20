package ashby

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeTransport func(*http.Request) (*http.Response, error)

func (f fakeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestAshbyDoesNotInventApplicationQuestions(t *testing.T) {
	const id = "e1f2d6a2-b9e2-4f16-b863-3ca228c83e75"
	client := &http.Client{Transport: fakeTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" || r.URL.Host != "api.ashbyhq.com" || r.URL.Path != "/posting-api/job-board/render" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"jobs":[{"id":"` + id + `","jobUrl":"https://jobs.ashbyhq.com/render/` + id + `","applyUrl":"https://jobs.ashbyhq.com/render/` + id + `/application"}]}`))}, nil
	})}
	result, err := New(client).Inspect(context.Background(), "https://jobs.ashbyhq.com/render/"+id)
	if err != nil {
		t.Fatal(err)
	}
	if result.Completeness != "partial" || len(result.Fields) != 0 || len(result.Steps) != 3 || result.Steps[1].State != "anticipated" {
		t.Fatalf("misleading result: %+v", result)
	}
}
func TestRejectsForgedAshbyURLs(t *testing.T) {
	for _, s := range []string{"https://bad.test/render/e1f2d6a2-b9e2-4f16-b863-3ca228c83e75", "https://jobs.ashbyhq.com/render/notuuid", "http://jobs.ashbyhq.com/render/e1f2d6a2-b9e2-4f16-b863-3ca228c83e75"} {
		if _, _, err := posting(s); err == nil {
			t.Errorf("accepted %s", s)
		}
	}
}

func TestApplicationDestinationUsesExactPublicApplyURL(t *testing.T) {
	const id = "e1f2d6a2-b9e2-4f16-b863-3ca228c83e75"
	postingURL := "https://jobs.ashbyhq.com/render/" + id + "?utm_source=trello&retain=yes"
	client := &http.Client{Transport: fakeTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.ashbyhq.com" {
			t.Fatalf("unexpected host: %s", r.URL.Host)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"jobs":[{"id":"` + id + `","applyUrl":"https://jobs.ashbyhq.com/render/` + id + `/application?existing=1"}]}`))}, nil
	})}
	got, err := New(client).ApplicationDestination(context.Background(), postingURL)
	if err != nil || got != "https://jobs.ashbyhq.com/render/"+id+"/application?existing=1&utm_source=trello" {
		t.Fatalf("destination %q: %v", got, err)
	}
}

func TestApplicationDestinationSafeSamePostingFallbacks(t *testing.T) {
	const id = "e1f2d6a2-b9e2-4f16-b863-3ca228c83e75"
	postingURL := "https://jobs.ashbyhq.com/render/" + id + "?utm_source=trello"
	for _, tc := range []struct {
		name     string
		response string
		status   int
	}{
		{"api offline", "", 503},
		{"missing apply URL", `{"jobs":[{"id":"` + id + `"}]}`, 200},
		{"other employer", `{"jobs":[{"id":"` + id + `","applyUrl":"https://jobs.ashbyhq.com/other/` + id + `/application"}]}`, 200},
		{"wrong posting", `{"jobs":[{"id":"` + id + `","applyUrl":"https://jobs.ashbyhq.com/render/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/application"}]}`, 200},
		{"external destination", `{"jobs":[{"id":"` + id + `","applyUrl":"https://evil.example/receive"}]}`, 200},
		{"submit endpoint", `{"jobs":[{"id":"` + id + `","applyUrl":"https://jobs.ashbyhq.com/render/` + id + `/submit"}]}`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: fakeTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.response))}, nil
			})}
			got, err := New(client).ApplicationDestination(context.Background(), postingURL)
			want := "https://jobs.ashbyhq.com/render/" + id + "/application?utm_source=trello"
			if err != nil || got != want {
				t.Fatalf("destination %q, want %q: %v", got, want, err)
			}
		})
	}
	if _, err := New(nil).ApplicationDestination(context.Background(), "https://evil.example/jobs/id"); err == nil {
		t.Fatal("untrusted posting accepted")
	}
}
