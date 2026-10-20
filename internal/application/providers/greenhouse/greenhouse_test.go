package greenhouse

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestGreenhousePublicFields(t *testing.T) {
	called := 0
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		called++
		if r.Method != "GET" || r.URL.Host != "boards-api.greenhouse.io" || r.URL.Path != "/v1/boards/acme/jobs/1234" || r.URL.Query().Get("questions") != "true" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"questions":[{"label":"First Name","required":true,"fields":[{"name":"first_name","type":"input_text"}]},{"label":"Resume","required":true,"fields":[{"name":"resume","type":"input_file"},{"name":"resume_text","type":"textarea"}]},{"label":"Why us?","fields":[{"name":"question_12","type":"textarea"}]},{"label":"What city?","fields":[{"name":"city","type":"multi_value_single_select","values":[{"label":"Detroit"},{"label":"Lansing"}]}]}],"compliance":[{"label":"Voluntary self-identification","fields":[{"name":"race","type":"multi_value_single_select"}]}]}`)), Header: make(http.Header)}, nil
	})}
	out, err := New(client).Inspect(context.Background(), "https://job-boards.greenhouse.io/acme/jobs/1234")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Fields) != 6 || out.Fields[0].ID != "application:first_name" || out.Fields[0].Label != "First Name" || !out.Fields[0].Required || out.Fields[4].Options[1] != "Lansing" || !out.Fields[5].Sensitive {
		t.Fatalf("unexpected fields %#v", out.Fields)
	}
	if out.Completeness != "public_schema" || called != 1 {
		t.Fatalf("result %+v called %d", out, called)
	}
}
func TestRejectsUntrustedURLs(t *testing.T) {
	for _, s := range []string{"https://bad.test/acme/jobs/123", "http://job-boards.greenhouse.io/acme/jobs/123", "https://job-boards.greenhouse.io@evil.test/acme/jobs/123", "https://job-boards.greenhouse.io/acme/jobs/not-id", "https://job-boards.greenhouse.io/acme/jobs/123/extra"} {
		if _, _, err := posting(s); err == nil {
			t.Errorf("accepted %s", s)
		}
	}
}
