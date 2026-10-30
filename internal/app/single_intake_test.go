package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// Both transports must use the same intake rules while retaining their own
// response contract. All records live in testWorkspace's temporary SQLite DB.
func TestSingleIntakeTransportsPreserveManualWork(t *testing.T) {
	for _, api := range []bool{false, true} {
		t.Run(fmt.Sprintf("json=%t", api), func(t *testing.T) {
			a, _ := testWorkspace(t)
			handler := newMux(a)
			post := func(rawURL, title string) *httptest.ResponseRecorder {
				path, contentType := "/jobs", "application/x-www-form-urlencoded"
				body := url.Values{"job_url": {rawURL}, "job_title": {title}}.Encode()
				if api {
					path, contentType = "/api/opportunities", "application/json"
					data, err := json.Marshal(map[string]string{"url": rawURL, "title": title})
					if err != nil {
						t.Fatal(err)
					}
					body = string(data)
				}
				req := httptest.NewRequest(http.MethodPost, "http://localhost"+path, strings.NewReader(body))
				req.Header.Set("Content-Type", contentType)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				return rec
			}
			assertResponse := func(rec *httptest.ResponseRecorder, duplicate bool) {
				t.Helper()
				want := http.StatusSeeOther
				if api {
					want = http.StatusCreated
					if duplicate {
						want = http.StatusOK
					}
					var wire map[string]any
					if err := json.Unmarshal(rec.Body.Bytes(), &wire); err != nil {
						t.Fatal(err)
					}
					if len(wire) != 3 || wire["id"] != float64(1) || wire["added"] != !duplicate || wire["duplicate"] != duplicate {
						t.Fatalf("unexpected save contract: %v", wire)
					}
				} else if rec.Header().Get("Location") != "/jobs" {
					t.Fatalf("redirect: %v", rec.Header())
				}
				if rec.Code != want {
					t.Fatalf("status=%d want=%d body=%s", rec.Code, want, rec.Body.String())
				}
			}
			assertResponse(post(" https://EXAMPLE.com/new/#fragment ", " Manual title "), false)
			job, err := a.jobByID(1)
			if err != nil {
				t.Fatal(err)
			}
			if job.URL != "https://example.com/new" || job.Title != "Manual title" || !job.TitleManual || job.Stage != StageReview || job.Priority != PriorityNormal {
				t.Fatalf("unexpected saved job: %+v", job)
			}
			job.Title, job.Stage, job.Priority = "Protected edit", StageReady, PriorityHigh
			if err := a.updateJob(job); err != nil {
				t.Fatal(err)
			}
			assertResponse(post("https://example.com/new/#other", "Replacement"), true)
			stored, err := a.jobByID(1)
			if err != nil {
				t.Fatal(err)
			}
			if stored != job {
				t.Fatalf("duplicate changed saved work:\nbefore=%+v\nafter=%+v", job, stored)
			}
			for _, input := range []struct{ url, title string }{
				{"invalid", ""}, {"https://example.com/another", strings.Repeat("x", 201)},
			} {
				rec := post(input.url, input.title)
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("invalid input: %d %s", rec.Code, rec.Body.String())
				}
				if api && rec.Body.String() != "{\"error\":\"Invalid job URL or title\"}\n" {
					t.Fatalf("error contract: %s", rec.Body.String())
				}
			}
			jobs, err := a.allJobs()
			if err != nil || len(jobs) != 1 {
				t.Fatalf("invalid intake changed records: %v %v", jobs, err)
			}
		})
	}
}
