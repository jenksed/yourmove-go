//go:build qualification

package playwright

// Loopback-only harness, absent from production builds. It exercises the
// Playwright session against controlled local pages, never a live employer
// site or a mocked browser interface.
import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

func urlParse(raw string) (*url.URL, error) { return url.Parse(raw) }

// LaunchFixture opens a loopback fixture page in a headless managed browser
// of the requested engine. It changes only the browser-side route-policy
// inputs; production launch has no override. Set YOURMOVE_QUAL_HEADED=1 to
// exercise the same flow in a visible window for manual verification.
func LaunchFixture(ctx context.Context, engine, fixturePosting string) (*Session, error) {
	return launchFixturePage(ctx, engine, fixturePosting, fixturePosting+"/application", os.Getenv("YOURMOVE_QUAL_HEADED") == "1")
}

// LaunchFixtureHeaded is the visible variant for manual engine verification.
func LaunchFixtureHeaded(ctx context.Context, engine, fixturePosting string) (*Session, error) {
	return launchFixturePage(ctx, engine, fixturePosting, fixturePosting+"/application", true)
}

func launchFixturePage(ctx context.Context, engine, fixturePosting, page string, headed bool) (*Session, error) {
	if !ValidEngine(engine) {
		return nil, fmt.Errorf("unsupported browser engine: %s", engine)
	}
	if !fixtureURL(fixturePosting) {
		return nil, fmt.Errorf("qualification accepts loopback fixtures only")
	}
	if !fixtureURL(page) || (page != fixturePosting && !strings.HasPrefix(page, fixturePosting+"/")) {
		return nil, fmt.Errorf("qualification page must stay under its loopback fixture")
	}
	s, err := launchRaw(ctx, engine, page, headed, "", fixturePosting)
	if err != nil {
		return nil, err
	}
	s.postingURL = fixturePosting
	s.fixtureOwner = func(candidate string) bool {
		return candidate == fixturePosting || strings.HasPrefix(candidate, fixturePosting+"/")
	}
	parsed := parseFixture(fixturePosting)
	s.fixtureContactOrigin = parsed
	s.fixtureContactBase = fixtureBase(fixturePosting)
	return s, nil
}

// FixtureServer is a controlled employer-application fixture plus a
// submission tripwire: any POST or GET to /submitted proves automation
// attempted a final employer submission, which must never happen.
type FixtureServer struct {
	Server    *httptest.Server
	submitted atomic.Int64
}

// ControllerForm serves a representative employer-application fixture:
// text inputs and textarea, selects, checkboxes, radio groups, file uploads,
// a dynamically rerendered control, multi-step navigation, a same-origin
// iframe case, and an already populated manual value.
func ControlledFormServer() *FixtureServer {
	fix := &FixtureServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/fixture/app/application", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, controlledFormHTML("/fixture/app/frame", "/fixture/app/application/step2"))
	})
	mux.HandleFunc("/fixture/app/application/step2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, stepTwoHTML())
	})
	mux.HandleFunc("/fixture/app/frame", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><body><form><label>Frame notes<input type="text" name="frame_notes"></label></form></body></html>`)
	})
	mux.HandleFunc("/fixture/app/submitted", func(w http.ResponseWriter, r *http.Request) {
		fix.submitted.Add(1)
		w.WriteHeader(http.StatusTeapot)
		fmt.Fprint(w, "must never submit")
	})
	fix.Server = httptest.NewServer(mux)
	return fix
}

// Close shuts the fixture down and reports whether any submission request arrived.
func (f *FixtureServer) Close() int64 {
	f.Server.Close()
	return f.submitted.Load()
}

func controlledFormHTML(framePath, stepTwoPath string) string {
	return `<!DOCTYPE html><html><head><title>Fixture application</title></head><body>
<div class="ashby-application-form-container">
<form method="post" action="/fixture/app/submitted">
<div data-field-entry-id="systemfield_firstname"><div class="ashby-application-form-question-title">First Name</div><input type="text" name="systemfield_firstname" autocomplete="given-name"></div>
<div data-field-entry-id="systemfield_lastname"><div class="ashby-application-form-question-title">Last Name</div><input type="text" name="systemfield_lastname" autocomplete="family-name"></div>
<div data-field-entry-id="systemfield_email"><div class="ashby-application-form-question-title">Email</div><input type="email" name="systemfield_email" autocomplete="email"></div>
<div data-field-entry-id="systemfield_phone"><div class="ashby-application-form-question-title">Phone</div><input type="tel" name="systemfield_phone" autocomplete="tel"></div>
<div data-field-entry-id="custom_essay"><div class="ashby-application-form-question-title">Why are you a good fit?</div><textarea name="custom_essay"></textarea></div>
<div data-field-entry-id="custom_location"><div class="ashby-application-form-question-title">Location</div><select name="custom_location"><option value="">Select…</option><option>Remote</option><option>Onsite</option></select></div>
<div data-field-entry-id="custom_remote"><div class="ashby-application-form-question-title">Open to remote?</div><label><input type="checkbox" name="custom_remote" value="true"> Yes</label></div>
<div data-field-entry-id="custom_level"><div class="ashby-application-form-question-title">Level</div><label><input type="radio" name="custom_level" value="Junior"> Junior</label><label><input type="radio" name="custom_level" value="Senior"> Senior</label></div>
<div data-field-entry-id="custom_resume"><div class="ashby-application-form-question-title">Resume</div><input type="file" name="resume" accept=".pdf"></div>
<div data-field-entry-id="custom_cover"><div class="ashby-application-form-question-title">Cover Letter</div><input type="file" name="cover_letter" accept=".pdf"></div>
<div data-field-entry-id="custom_cover_text"><div class="ashby-application-form-question-title">Cover Letter</div><textarea name="cover_letter_text"></textarea></div>
<div data-field-entry-id="custom_manual"><div class="ashby-application-form-question-title">Manual note</div><input type="text" name="manual_note" value="Keep me"></div>
<div id="dynamic-slot"><div data-field-entry-id="custom_dynamic"><div class="ashby-application-form-question-title">Dynamic field</div><input type="text" name="custom_dynamic"></div></div>
</form>
<a href="` + stepTwoPath + `">Continue to step 2</a>
<iframe src="` + framePath + `" title="Fixture frame"></iframe>
</div>
<script>
setTimeout(() => {
  const slot = document.getElementById('dynamic-slot');
  if (slot) slot.innerHTML = '<div data-field-entry-id="custom_dynamic"><div class="ashby-application-form-question-title">Dynamic field</div><input type="text" name="custom_dynamic"></div>';
}, 400);
</script>
</body></html>`
}

func stepTwoHTML() string {
	return `<!DOCTYPE html><html><head><title>Fixture step two</title></head><body>
<div class="ashby-application-form-container">
<form method="post" action="/fixture/app/submitted">
<div data-field-entry-id="custom_step2"><div class="ashby-application-form-question-title">Step two question</div><textarea name="custom_step2"></textarea></div>
<div data-field-entry-id="custom_step2_resume"><div class="ashby-application-form-question-title">Resume</div><input type="file" name="resume" accept=".pdf"></div>
</form>
</div>
</body></html>`
}

func fixtureURL(raw string) bool {
	for _, prefix := range []string{"http://127.0.0.1", "http://localhost"} {
		if strings.HasPrefix(raw, prefix) {
			return true
		}
	}
	return false
}

func parseFixture(raw string) string {
	// Origin of the loopback fixture server, e.g. http://127.0.0.1:port.
	u, err := urlParse(raw)
	if err != nil {
		return raw
	}
	return u.Scheme + "://" + u.Host
}

func fixtureBase(raw string) string {
	u, err := urlParse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(u.EscapedPath(), "/")
}

// waitForCondition polls a predicate until it holds or the context ends.
func waitForCondition(ctx context.Context, what string, check func() bool) error {
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		if check() {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s: %w", what, ctx.Err())
		case <-tick.C:
		}
	}
}
