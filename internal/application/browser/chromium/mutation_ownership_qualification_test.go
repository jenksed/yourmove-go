//go:build qualification

package chromium

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func TestMutationOwnershipMatchesPostingPolicy(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in controlled Chromium mutation-ownership qualification")
	}

	tests := []struct {
		name, saved string
		mutatePath  string
		wantWrite   bool
	}{
		{
			name:       "Ashby supported descendant",
			saved:      "https://jobs.ashbyhq.com/acme/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
			mutatePath: "/fixture/job/application/step-2",
			wantWrite:  true,
		},
		{
			name:       "Greenhouse exact posting",
			saved:      "https://boards.greenhouse.io/acme/jobs/123",
			mutatePath: "/fixture/job",
			wantWrite:  true,
		},
		{
			name:       "Lever supported apply route",
			saved:      "https://jobs.lever.co/acme/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
			mutatePath: "/fixture/job/apply",
			wantWrite:  true,
		},
		{
			name:       "Greenhouse rejected descendant after navigation",
			saved:      "https://boards.greenhouse.io/acme/jobs/123",
			mutatePath: "/fixture/job/review",
		},
		{
			name:       "Lever rejected descendant after navigation",
			saved:      "https://jobs.lever.co/acme/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
			mutatePath: "/fixture/job/other",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				fmt.Fprint(w, `<!doctype html><form class="ashby-application-form">
<section data-field-entry-id="answer-field"><div class="ashby-application-form-question-title">Why this role?</div><textarea name="answer-field"></textarea></section>
<section data-field-entry-id="cover-field"><div class="ashby-application-form-question-title">Cover Letter</div><textarea name="cover-field"></textarea></section>
<button type="submit">Submit</button></form>
<script>window.submits=0;document.querySelector('form').addEventListener('submit',e=>{e.preventDefault();window.submits++})</script>`)
			}))
			defer fixture.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			base := fixture.URL + "/fixture/job"
			session, err := LaunchFixtureForPosting(ctx, base, tc.saved)
			if err != nil {
				t.Fatalf("launch controlled fixture: %v", err)
			}
			defer session.Close()

			startPath := "/fixture/job"
			if strings.Contains(tc.saved, "jobs.ashbyhq.com/") {
				startPath += "/application"
			} else if strings.Contains(tc.saved, "jobs.lever.co/") {
				startPath += "/apply"
			}
			currentURL := fixture.URL + startPath
			targetURL := fixture.URL + tc.mutatePath
			if targetURL != currentURL {
				if err := session.FixtureCall(ctx, currentURL, "Page.navigate", map[string]any{"url": targetURL}, nil); err != nil {
					t.Fatalf("navigate fixture to mutation route: %v", err)
				}
				waitForFixtureURL(t, ctx, session, targetURL)
			}
			waitForFixtureControls(t, ctx, session, targetURL)

			answer := browser.AnswerValue{
				Field: schema.Field{ID: "browser:answer-field", Label: "Why this role?", Kind: "textarea"},
				Value: "Approved synthetic answer",
			}
			answerOutcome, answerErr := session.FillAnswer(ctx, answer)
			letterOutcome, letterErr := session.FillCoverLetter(ctx, "Approved synthetic cover letter")
			if tc.wantWrite {
				if answerErr != nil || answerOutcome.Status != "filled" {
					t.Fatalf("answer transfer: outcome=%+v err=%v", answerOutcome, answerErr)
				}
				if letterErr != nil || !letterOutcome.Verified {
					t.Fatalf("cover-letter transfer: outcome=%+v err=%v", letterOutcome, letterErr)
				}
				setFixtureValue(t, ctx, session, targetURL, `document.querySelector('[name="answer-field"]').value='Manual answer';document.querySelector('[name="cover-field"]').value='Manual letter';true`)
				_, answerErr = session.FillAnswer(ctx, answer)
				_, letterErr = session.FillCoverLetter(ctx, "Approved synthetic cover letter")
				if answerErr != nil || letterErr != nil {
					t.Fatalf("manual-entry preservation check errored: answer=%v letter=%v", answerErr, letterErr)
				}
				assertFixtureValue(t, ctx, session, targetURL, `document.querySelector('[name="answer-field"]').value==='Manual answer'&&document.querySelector('[name="cover-field"]').value==='Manual letter'`)
			} else {
				if answerErr == nil || letterErr == nil {
					t.Fatalf("rejected destination was accepted: answer outcome=%+v err=%v letter outcome=%+v err=%v", answerOutcome, answerErr, letterOutcome, letterErr)
				}
				assertFixtureValue(t, ctx, session, targetURL, `document.querySelector('[name="answer-field"]').value===''&&document.querySelector('[name="cover-field"]').value===''`)
			}
			assertFixtureValue(t, ctx, session, targetURL, `window.submits===0`)
		})
	}
}

func waitForFixtureURL(t *testing.T, ctx context.Context, session *Session, targetURL string) {
	t.Helper()
	for {
		var result struct {
			Result struct {
				Value string `json:"value"`
			} `json:"result"`
		}
		err := session.FixtureCall(ctx, targetURL, "Runtime.evaluate", map[string]any{"expression": "location.href", "returnByValue": true}, &result)
		if err == nil && result.Result.Value == targetURL {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("fixture did not reach %s: %v (last=%s)", targetURL, ctx.Err(), result.Result.Value)
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func waitForFixtureControls(t *testing.T, ctx context.Context, session *Session, targetURL string) {
	t.Helper()
	for {
		var result struct {
			Result struct {
				Value bool `json:"value"`
			} `json:"result"`
		}
		err := session.FixtureCall(ctx, targetURL, "Runtime.evaluate", map[string]any{
			"expression":    `!!document.querySelector('[name="answer-field"]')&&!!document.querySelector('[name="cover-field"]')&&typeof window.submits==='number'`,
			"returnByValue": true,
		}, &result)
		if err == nil && result.Result.Value {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("fixture controls did not become ready at %s: %v", targetURL, ctx.Err())
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func setFixtureValue(t *testing.T, ctx context.Context, session *Session, targetURL, expression string) {
	t.Helper()
	assertFixtureValue(t, ctx, session, targetURL, expression)
}

func assertFixtureValue(t *testing.T, ctx context.Context, session *Session, targetURL, expression string) {
	t.Helper()
	var result struct {
		Result struct {
			Value bool `json:"value"`
		} `json:"result"`
		Exception json.RawMessage `json:"exceptionDetails"`
	}
	if err := session.FixtureCall(ctx, targetURL, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true}, &result); err != nil {
		t.Fatalf("evaluate fixture state: %v", err)
	}
	if len(result.Exception) != 0 || !result.Result.Value {
		t.Fatalf("fixture assertion failed: %s (result=%+v)", expression, result)
	}
}
