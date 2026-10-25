//go:build qualification

package chromium

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser"
)

// TestContactResolverOwnershipMatchesPostingPolicy checks the policy used at
// the DOM mutation point against the Go observation/session ownership rules.
// It uses only an isolated local page and synthetic posting identities.
func TestContactResolverOwnershipMatchesPostingPolicy(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in controlled Chromium qualification")
	}
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<!doctype html><title>Local ownership fixture</title><form class="ashby-application-form"><label>Email<input name="email" type="email"></label></form>`)
	}))
	defer fixture.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	session, err := LaunchFixture(ctx, fixture.URL+"/fixture")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for {
		var loaded struct {
			Result struct {
				Value bool `json:"value"`
			} `json:"result"`
		}
		if err := session.FixtureCall(ctx, fixture.URL+"/fixture/application", "Runtime.evaluate", map[string]any{
			"expression": fmt.Sprintf("location.origin === %q", fixture.URL), "returnByValue": true,
		}, &loaded); err == nil && loaded.Result.Value {
			break
		}
		if ctx.Err() != nil {
			t.Fatalf("fixture page did not load: %v", ctx.Err())
		}
		time.Sleep(25 * time.Millisecond)
	}

	tests := []struct {
		name, saved, candidate string
		want                   bool
	}{
		{"greenhouse exact posting", "https://boards.greenhouse.io/acme/jobs/123", "/acme/jobs/123", true},
		{"greenhouse rejects descendant", "https://boards.greenhouse.io/acme/jobs/123", "/acme/jobs/123/review", false},
		{"lever apply route", "https://jobs.lever.co/acme/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "/acme/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/apply", true},
		{"lever rejects other descendant", "https://jobs.lever.co/acme/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "/acme/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/other", false},
		{"ashby application descendant", "https://jobs.ashbyhq.com/acme/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "/acme/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/application", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			saved, err := url.Parse(tc.saved)
			if err != nil {
				t.Fatal(err)
			}
			origin, err := url.Parse(fixture.URL)
			if err != nil {
				t.Fatal(err)
			}
			boundary, err := json.Marshal([]string{origin.Scheme + "://" + origin.Host, saved.Path, browser.PostingPlatform(tc.saved)})
			if err != nil {
				t.Fatal(err)
			}
			candidate, err := url.Parse(tc.saved)
			if err != nil {
				t.Fatal(err)
			}
			candidate.Path = tc.candidate
			goOwns := browser.OwnsPostingURL(tc.saved, candidate.String())
			if goOwns != tc.want {
				t.Fatalf("Go ownership = %t, want %t (%s)", goOwns, tc.want, candidate)
			}

			var got struct {
				Result struct {
					Value bool `json:"value"`
				} `json:"result"`
			}
			expression := contactResolver + "(..." + string(boundary) + ").owns(" + fmt.Sprintf("%q", tc.candidate) + ")"
			if err := session.FixtureCall(ctx, fixture.URL+"/fixture/application", "Runtime.evaluate", map[string]any{
				"expression": expression, "returnByValue": true, "silent": true,
			}, &got); err != nil {
				t.Fatal(err)
			}
			if got.Result.Value != tc.want {
				t.Fatalf("contact resolver ownership = %t, want %t (%s); boundary=%s", got.Result.Value, tc.want, candidate, boundary)
			}
		})
	}

	// LaunchFixture opens /application below the saved /fixture path. This was
	// accepted by the former broad prefix check, although strict ownership
	// rejects the route. The form must stay untouched on that descendant route.
	saved := fixture.URL + "/fixture"
	observed := fixture.URL + "/fixture/application"
	if browser.OwnsPostingURL(saved, observed) {
		t.Fatal("fixture setup must model a route rejected by strict posting ownership")
	}
	if _, err := session.FillContacts(ctx, browser.ContactValues{Email: "synthetic@example.test"}); err == nil {
		t.Fatal("contact transfer accepted a descendant outside the saved posting")
	}
	var untouched struct {
		Result struct {
			Value bool `json:"value"`
		} `json:"result"`
	}
	if err := session.FixtureCall(ctx, observed, "Runtime.evaluate", map[string]any{
		"expression": `document.querySelector('[name=email]').value === ''`, "returnByValue": true,
	}, &untouched); err != nil || !untouched.Result.Value {
		t.Fatalf("out-of-scope contact field was changed: %+v %v", untouched, err)
	}
}
