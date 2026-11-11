package app

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/jenksed/yourmove-go/internal/updates"
)

// UpdateCheckResult is the applicant-visible outcome of an update check.
// It carries only release metadata, never applicant workspace material.
// Alias: internal/updates owns the type; this preserves the desktop host and
// template/JSON API surface.
type UpdateCheckResult = updates.CheckResult

// UpdateController buttons the Wails updater (owned by the desktop host) into
// the Go transport edge. Browser mode leaves all three nil: updates are an
// installed-app capability, and the manual DMG path remains the fallback.
// Alias: internal/updates owns the type; this preserves the desktop host API.
type UpdateController = updates.Controller

// updater returns the owned updates.Manager, lazily provisioning it for App
// values not built through New (production always wires it there explicitly).
func (a *App) updater() *updates.Manager {
	if a.updates == nil {
		a.updates = updates.NewManager(CurrentAppVersion())
	}
	return a.updates
}

// SetUpdateController wires the desktop updater callbacks into the owned
// updates.Manager.
func (a *App) SetUpdateController(c UpdateController) {
	a.updater().SetController(c)
}

// updateSnapshot returns a copy for templates and JSON APIs.
func (a *App) updateSnapshot() (phase string, pending *UpdateCheckResult, errMsg string, checkedAt time.Time, enabled bool) {
	return a.updater().Snapshot()
}

func (a *App) setUpdatePhase(phase string, pending *UpdateCheckResult, errMsg string) {
	a.updater().SetPhase(phase, pending, errMsg)
}

// POST /utilities/updates/check — explicit applicant action only.
// Never runs automatically from page loads; background checks live in
// desktop.go and only record availability.
func (a *App) updateCheckHandler(w http.ResponseWriter, r *http.Request) {
	if !a.updater().HasCheck() {
		http.Error(w, "Automatic updates are not available in this mode; use the manual DMG.", http.StatusNotFound)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := a.updater().RunCheck(ctx); err != nil {
		// Offline or endpoint failures must not block local work.
		// Keep the current app usable; surface a comprehensible message.
		log.Printf("update check failed: %v", err)
	}
	http.Redirect(w, r, "/utilities", http.StatusSeeOther)
}

// POST /utilities/updates/install — explicit approval only.
// Downloads, authenticates, stages via Wails; never auto-runs.
func (a *App) updateInstallHandler(w http.ResponseWriter, r *http.Request) {
	if !a.updater().HasInstall() {
		http.Error(w, "Automatic updates are not available in this mode; use the manual DMG.", http.StatusNotFound)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	if err := a.updater().RunInstall(ctx); err != nil {
		log.Printf("update install failed: %v", err)
	}
	http.Redirect(w, r, "/utilities", http.StatusSeeOther)
}

// POST /utilities/updates/restart — explicit approval only.
func (a *App) updateRestartHandler(w http.ResponseWriter, r *http.Request) {
	if !a.updater().HasRestart() {
		http.Error(w, "Automatic updates are not available in this mode; use the manual DMG.", http.StatusNotFound)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if err := a.updater().RunRestart(ctx); err != nil {
		log.Printf("update restart failed: %v", err)
	}
	http.Redirect(w, r, "/utilities", http.StatusSeeOther)
}

func (a *App) currentPending() *UpdateCheckResult {
	_, pending, _, _, _ := a.updater().Snapshot()
	return pending
}

// GET /api/updates/status — JSON for tests and future React use.
func (a *App) updateStatusAPI(w http.ResponseWriter, r *http.Request) {
	phase, pending, errMsg, checkedAt, enabled := a.updateSnapshot()
	out := map[string]any{
		"enabled":        enabled,
		"phase":          phase,
		"currentVersion": CurrentAppVersion(),
		"build":          CurrentAppBuild(),
	}
	if pending != nil {
		out["available"] = pending.Available
		out["latestVersion"] = pending.LatestVersion
		out["name"] = pending.Name
		out["notes"] = pending.Notes
		out["size"] = pending.Size
	}
	if errMsg != "" {
		out["error"] = errMsg
	}
	if !checkedAt.IsZero() {
		out["checkedAt"] = checkedAt.UTC().Format(time.RFC3339)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// BackgroundUpdateCheck records availability without opening any UI.
// It is the nonblocking automatic path: offline or endpoint errors leave
// the current app usable and do not overwrite an explicit user state.
// A background success only surfaces from idle/up-to-date/available; it
// never overwrites a user-initiated checking/installing/ready/error phase.
func (a *App) BackgroundUpdateCheck(ctx context.Context) {
	a.updater().BackgroundCheck(ctx)
}

// updateUserError keeps failure feedback comprehensible and free of
// workspace paths, keys, or applicant material. Owned by internal/updates.
func updateUserError(err error) string {
	return updates.UserError(err)
}
