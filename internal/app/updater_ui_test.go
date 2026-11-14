package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The Software Updates screen must describe the real updater behavior:
// automatic availability checks when enabled, manual re-check, explicit
// approval for install and restart, and an honest unavailable state.
func renderUtilities(t *testing.T, a *App) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/utilities", nil)
	rec := httptest.NewRecorder()
	a.utilitiesPage(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("utilities status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

func TestUtilitiesDescribesRealUpdateBehavior(t *testing.T) {
	a, _ := testWorkspace(t)
	a.SetUpdateController(UpdateController{
		Check: func(ctx context.Context) (UpdateCheckResult, error) {
			return UpdateCheckResult{Available: true, CurrentVersion: AppVersion, LatestVersion: "0.2.2", Name: "YourMove 0.2.2", Notes: "Authenticated fixture notes.", Size: 42}, nil
		},
		Install: func(ctx context.Context) error { return nil },
		Restart: func(ctx context.Context) error { return nil },
	})

	idle := renderUtilities(t, a)
	for _, want := range []string{
		"Installed version " + AppVersion + " (build " + AppBuild + ")",
		"checks for new versions automatically",
		"explicit approval",
		"Check for updates",
		"No update check has run yet",
	} {
		if !strings.Contains(idle, want) {
			t.Errorf("idle utilities page missing %q", want)
		}
	}
	if strings.Contains(idle, "only after you ask") {
		t.Error("idle utilities page still claims checks happen only on demand")
	}

	check := httptest.NewRequest(http.MethodPost, "/utilities/updates/check", nil)
	checkRec := httptest.NewRecorder()
	a.updateCheckHandler(checkRec, check)
	if checkRec.Code != http.StatusSeeOther {
		t.Fatalf("check status = %d, want 303", checkRec.Code)
	}
	available := renderUtilities(t, a)
	for _, want := range []string{
		"Version 0.2.2 is available",
		"YourMove 0.2.2",
		"Authenticated fixture notes.",
		"Install version 0.2.2",
		"Release notes",
	} {
		if !strings.Contains(available, want) {
			t.Errorf("available utilities page missing %q", want)
		}
	}

	install := httptest.NewRequest(http.MethodPost, "/utilities/updates/install", nil)
	installRec := httptest.NewRecorder()
	a.updateInstallHandler(installRec, install)
	if installRec.Code != http.StatusSeeOther {
		t.Fatalf("install status = %d, want 303", installRec.Code)
	}
	ready := renderUtilities(t, a)
	for _, want := range []string{"Update staged.", "Restart now"} {
		if !strings.Contains(ready, want) {
			t.Errorf("ready utilities page missing %q", want)
		}
	}
}

func TestUtilitiesUnavailableStateIsHonest(t *testing.T) {
	a, _ := testWorkspace(t)
	page := renderUtilities(t, a)
	if !strings.Contains(page, "Automatic updates are not available") {
		t.Error("unavailable utilities page must say automatic updates are not available")
	}
	if strings.Contains(page, "Check for updates") {
		t.Error("unavailable utilities page must not offer an update check")
	}
	if strings.Contains(page, "checks for new versions automatically") {
		t.Error("unavailable utilities page must not describe automatic checks as active")
	}
}

func TestUtilitiesSurfacesInstallFailureWithoutBreaking(t *testing.T) {
	a, _ := testWorkspace(t)
	a.SetUpdateController(UpdateController{
		Check: func(ctx context.Context) (UpdateCheckResult, error) {
			return UpdateCheckResult{Available: true, CurrentVersion: AppVersion, LatestVersion: "0.2.2"}, nil
		},
		Install: func(ctx context.Context) error { return errors.New("updater: ed25519 signature did not verify") },
		Restart: func(ctx context.Context) error { return nil },
	})
	check := httptest.NewRequest(http.MethodPost, "/utilities/updates/check", nil)
	a.updateCheckHandler(httptest.NewRecorder(), check)
	install := httptest.NewRequest(http.MethodPost, "/utilities/updates/install", nil)
	installRec := httptest.NewRecorder()
	a.updateInstallHandler(installRec, install)
	if installRec.Code != http.StatusSeeOther {
		t.Fatalf("install status = %d, want 303", installRec.Code)
	}
	page := renderUtilities(t, a)
	if !strings.Contains(page, "Update could not be completed") {
		t.Error("failed install must surface a comprehensible error on the utilities page")
	}
	if !strings.Contains(page, "manual DMG remains available") {
		t.Error("failed install must point at the manual fallback")
	}
	if strings.Contains(page, "[path]") && strings.Contains(page, "/tmp") {
		t.Error("failure text must not leak filesystem paths")
	}
	phase, _, _, _, enabled := a.updateSnapshot()
	if !enabled || phase != "error" {
		t.Fatalf("after failed install enabled=%v phase=%q, want enabled phase=error", enabled, phase)
	}
}
