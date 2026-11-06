package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionMatchesInfoPlist(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "release", "macos", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	plist := string(data)
	if !strings.Contains(plist, "<string>"+AppVersion+"</string>") {
		t.Fatalf("AppVersion %q not found in Info.plist", AppVersion)
	}
	if !strings.Contains(plist, "<string>"+AppBuild+"</string>") {
		t.Fatalf("AppBuild %q not found in Info.plist", AppBuild)
	}
	if AppVersion == "" || AppBuild == "" {
		t.Fatal("version source must not be empty")
	}
}

func TestUpdatesUnavailableWithoutController(t *testing.T) {
	a, _ := testWorkspace(t)
	// No controller set: browser mode / unconfigured desktop.
	req := httptest.NewRequest(http.MethodGet, "/api/updates/status", nil)
	rec := httptest.NewRecorder()
	a.updateStatusAPI(rec, req)
	var out map[string]any
	if err := json.NewDecoder(rec.Result().Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["enabled"] != false {
		t.Fatalf("status enabled = %v, want false", out["enabled"])
	}
	if out["phase"] != "unavailable" {
		t.Fatalf("phase = %v, want unavailable", out["phase"])
	}
	post := httptest.NewRequest(http.MethodPost, "/utilities/updates/check", nil)
	postRec := httptest.NewRecorder()
	a.updateCheckHandler(postRec, post)
	if postRec.Code != http.StatusNotFound {
		t.Fatalf("check without controller = %d, want 404", postRec.Code)
	}
}

func TestUpdateCheckInstallRestartFlow(t *testing.T) {
	a, _ := testWorkspace(t)
	checks := 0
	installs := 0
	restarts := 0
	a.SetUpdateController(UpdateController{
		Check: func(ctx context.Context) (UpdateCheckResult, error) {
			checks++
			return UpdateCheckResult{Available: true, CurrentVersion: AppVersion, LatestVersion: "0.2.0", Name: "Test release", Notes: "Fixes and polish.", Size: 123}, nil
		},
		Install: func(ctx context.Context) error { installs++; return nil },
		Restart: func(ctx context.Context) error { restarts++; return nil },
	})
	// Check.
	req := httptest.NewRequest(http.MethodPost, "/utilities/updates/check", nil)
	rec := httptest.NewRecorder()
	a.updateCheckHandler(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("check status = %d, want 303", rec.Code)
	}
	phase, pending, _, _, enabled := a.updateSnapshot()
	if !enabled || phase != "available" || pending == nil || pending.LatestVersion != "0.2.0" {
		t.Fatalf("after check phase=%q pending=%+v enabled=%v", phase, pending, enabled)
	}
	if checks != 1 {
		t.Fatalf("checks = %d, want 1", checks)
	}
	// Install (explicit approval).
	ireq := httptest.NewRequest(http.MethodPost, "/utilities/updates/install", nil)
	irec := httptest.NewRecorder()
	a.updateInstallHandler(irec, ireq)
	if irec.Code != http.StatusSeeOther {
		t.Fatalf("install status = %d, want 303", irec.Code)
	}
	phase, _, _, _, _ = a.updateSnapshot()
	if phase != "ready" || installs != 1 {
		t.Fatalf("after install phase=%q installs=%d", phase, installs)
	}
	// Restart (explicit approval).
	rreq := httptest.NewRequest(http.MethodPost, "/utilities/updates/restart", nil)
	rrec := httptest.NewRecorder()
	a.updateRestartHandler(rrec, rreq)
	if rrec.Code != http.StatusSeeOther {
		t.Fatalf("restart status = %d, want 303", rrec.Code)
	}
	if restarts != 1 {
		t.Fatalf("restarts = %d, want 1", restarts)
	}
	// Status API reflects ready without applicant data.
	sreq := httptest.NewRequest(http.MethodGet, "/api/updates/status", nil)
	srec := httptest.NewRecorder()
	a.updateStatusAPI(srec, sreq)
	var out map[string]any
	if err := json.NewDecoder(srec.Result().Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["phase"] != "ready" || out["latestVersion"] != "0.2.0" {
		t.Fatalf("status = %v", out)
	}
	if _, ok := out["notes"]; !ok {
		t.Fatal("status should include release notes")
	}
}

func TestUpdateCheckFailureKeepsAppUsable(t *testing.T) {
	a, _ := testWorkspace(t)
	a.SetUpdateController(UpdateController{
		Check: func(ctx context.Context) (UpdateCheckResult, error) {
			return UpdateCheckResult{}, errors.New("dial tcp 127.0.0.1:9: connect: connection refused")
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/utilities/updates/check", nil)
	rec := httptest.NewRecorder()
	a.updateCheckHandler(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("check status = %d, want 303", rec.Code)
	}
	phase, _, errMsg, _, _ := a.updateSnapshot()
	if phase != "error" {
		t.Fatalf("phase = %q, want error", phase)
	}
	if errMsg == "" || !strings.Contains(errMsg, "workspace are unchanged") {
		t.Fatalf("error message missing recovery guidance: %q", errMsg)
	}
	if strings.Contains(errMsg, "jobs.db") || strings.Contains(errMsg, ".sqlite") {
		t.Fatalf("error message leaks workspace paths: %q", errMsg)
	}
	// Workspace remains usable: opportunities page still renders.
	pageReq := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	pageRec := httptest.NewRecorder()
	a.serveOpportunities(pageRec, pageReq)
	if pageRec.Code != http.StatusOK {
		t.Fatalf("workspace page after failed check = %d, want 200", pageRec.Code)
	}
}

func TestUpdateInstallFailureSurfacesRecovery(t *testing.T) {
	a, _ := testWorkspace(t)
	a.SetUpdateController(UpdateController{
		Check: func(ctx context.Context) (UpdateCheckResult, error) {
			return UpdateCheckResult{Available: true, CurrentVersion: AppVersion, LatestVersion: "0.2.0"}, nil
		},
		Install: func(ctx context.Context) error { return errors.New("updater: ed25519 signature did not verify") },
	})
	creq := httptest.NewRequest(http.MethodPost, "/utilities/updates/check", nil)
	crec := httptest.NewRecorder()
	a.updateCheckHandler(crec, creq)
	ireq := httptest.NewRequest(http.MethodPost, "/utilities/updates/install", nil)
	irec := httptest.NewRecorder()
	a.updateInstallHandler(irec, ireq)
	phase, _, errMsg, _, _ := a.updateSnapshot()
	if phase != "error" {
		t.Fatalf("phase = %q, want error", phase)
	}
	if !strings.Contains(errMsg, "signature") && !strings.Contains(errMsg, "Update could not be completed") {
		t.Fatalf("install error missing auth context: %q", errMsg)
	}
}

func TestBackgroundCheckOnlySurfacesAvailable(t *testing.T) {
	a, _ := testWorkspace(t)
	calls := 0
	a.SetUpdateController(UpdateController{
		Check: func(ctx context.Context) (UpdateCheckResult, error) {
			calls++
			return UpdateCheckResult{Available: false, CurrentVersion: AppVersion}, nil
		},
	})
	a.BackgroundUpdateCheck(context.Background())
	if calls != 1 {
		t.Fatalf("background calls = %d, want 1", calls)
	}
	phase, _, _, _, _ := a.updateSnapshot()
	if phase != "idle" {
		t.Fatalf("background no-update changed phase to %q, want idle (no modal)", phase)
	}
}
