package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func postDesktopUnsaved(t *testing.T, a *App, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/desktop/unsaved", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	return rec
}

func TestDesktopUnsavedFlagRoundTrip(t *testing.T) {
	a, _ := testWorkspace(t)
	if a.HasDesktopUnsaved() {
		t.Fatal("fresh workspace reports unsaved work")
	}
	if rec := postDesktopUnsaved(t, a, `{"dirty":true}`); rec.Code != http.StatusOK {
		t.Fatalf("report dirty: %d %s", rec.Code, rec.Body.String())
	}
	if !a.HasDesktopUnsaved() {
		t.Fatal("dirty report did not set the flag")
	}
	if rec := postDesktopUnsaved(t, a, `{"dirty":false}`); rec.Code != http.StatusOK {
		t.Fatalf("report clean: %d %s", rec.Code, rec.Body.String())
	}
	if a.HasDesktopUnsaved() {
		t.Fatal("clean report did not clear the flag")
	}
}

func TestDesktopUnsavedRejectsBadInput(t *testing.T) {
	a, _ := testWorkspace(t)
	if rec := postDesktopUnsaved(t, a, `{"dirty":`); rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed JSON: %d %s", rec.Code, rec.Body.String())
	}
	req := httptest.NewRequest(http.MethodGet, "http://localhost/api/desktop/unsaved", nil)
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	// Like every other POST-only API route, a GET falls through to the
	// home redirect; it must not touch the flag.
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("GET: %d %s", rec.Code, rec.Body.String())
	}
	if a.HasDesktopUnsaved() {
		t.Fatal("rejected input changed the flag")
	}
}
