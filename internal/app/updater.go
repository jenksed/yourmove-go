package app

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// UpdateCheckResult is the applicant-visible outcome of an update check.
// It carries only release metadata, never applicant workspace material.
type UpdateCheckResult struct {
	Available      bool   `json:"available"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion,omitempty"`
	Name           string `json:"name,omitempty"`
	Notes          string `json:"notes,omitempty"`
	Size           int64  `json:"size,omitempty"`
}

// UpdateController buttons the Wails updater (owned by desktop.go) into the
// Go transport edge. Browser mode leaves all three nil: updates are an
// installed-app capability, and the manual DMG path remains the fallback.
type UpdateController struct {
	Check   func(ctx context.Context) (UpdateCheckResult, error)
	Install func(ctx context.Context) error
	Restart func(ctx context.Context) error
}

// updateState mirrors the user-facing phase for the Utilities page.
// Detailed Wails lifecycle states remain in Wails event logs.
type updateState struct {
	mu            sync.Mutex
	phase         string // idle|checking|available|up-to-date|installing|ready|error|unavailable
	pending       *UpdateCheckResult
	err           string
	checkedAt     time.Time
	controllerSet bool
}

func (a *App) SetUpdateController(c UpdateController) {
	a.updates.mu.Lock()
	defer a.updates.mu.Unlock()
	a.updates.controllerSet = c.Check != nil
	a.updateCheck = c.Check
	a.updateInstall = c.Install
	a.updateRestart = c.Restart
	if a.updates.phase == "" {
		a.updates.phase = "idle"
	}
}

// updateSnapshot returns a copy for templates and JSON APIs.
func (a *App) updateSnapshot() (phase string, pending *UpdateCheckResult, errMsg string, checkedAt time.Time, enabled bool) {
	a.updates.mu.Lock()
	defer a.updates.mu.Unlock()
	if a.updateCheck == nil {
		return "unavailable", nil, "", time.Time{}, false
	}
	if a.updates.phase == "" {
		return "idle", nil, "", a.updates.checkedAt, true
	}
	var cp *UpdateCheckResult
	if a.updates.pending != nil {
		c := *a.updates.pending
		cp = &c
	}
	return a.updates.phase, cp, a.updates.err, a.updates.checkedAt, true
}

func (a *App) setUpdatePhase(phase string, pending *UpdateCheckResult, errMsg string) {
	a.updates.mu.Lock()
	defer a.updates.mu.Unlock()
	a.updates.phase = phase
	a.updates.pending = pending
	a.updates.err = errMsg
	if phase != "checking" {
		a.updates.checkedAt = time.Now()
	}
}

// POST /utilities/updates/check — explicit applicant action only.
// Never runs automatically from page loads; background checks live in
// desktop.go and only record availability.
func (a *App) updateCheckHandler(w http.ResponseWriter, r *http.Request) {
	if a.updateCheck == nil {
		http.Error(w, "Automatic updates are not available in this mode; use the manual DMG.", http.StatusNotFound)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	a.setUpdatePhase("checking", nil, "")
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	res, err := a.updateCheck(ctx)
	if err != nil {
		// Offline or endpoint failures must not block local work.
		// Keep the current app usable; surface a comprehensible message.
		log.Printf("update check failed: %v", err)
		a.setUpdatePhase("error", nil, updateUserError(err))
		http.Redirect(w, r, "/utilities", http.StatusSeeOther)
		return
	}
	if res.Available {
		c := res
		a.setUpdatePhase("available", &c, "")
	} else {
		if res.CurrentVersion == "" {
			res.CurrentVersion = CurrentAppVersion()
		}
		c := res
		a.setUpdatePhase("up-to-date", &c, "")
	}
	http.Redirect(w, r, "/utilities", http.StatusSeeOther)
}

// POST /utilities/updates/install — explicit approval only.
// Downloads, authenticates, stages via Wails; never auto-runs.
func (a *App) updateInstallHandler(w http.ResponseWriter, r *http.Request) {
	if a.updateInstall == nil {
		http.Error(w, "Automatic updates are not available in this mode; use the manual DMG.", http.StatusNotFound)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	a.setUpdatePhase("installing", a.currentPending(), "")
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	if err := a.updateInstall(ctx); err != nil {
		log.Printf("update install failed: %v", err)
		a.setUpdatePhase("error", a.currentPending(), updateUserError(err))
		http.Redirect(w, r, "/utilities", http.StatusSeeOther)
		return
	}
	a.setUpdatePhase("ready", a.currentPending(), "")
	http.Redirect(w, r, "/utilities", http.StatusSeeOther)
}

// POST /utilities/updates/restart — explicit approval only.
func (a *App) updateRestartHandler(w http.ResponseWriter, r *http.Request) {
	if a.updateRestart == nil {
		http.Error(w, "Automatic updates are not available in this mode; use the manual DMG.", http.StatusNotFound)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if err := a.updateRestart(ctx); err != nil {
		log.Printf("update restart failed: %v", err)
		a.setUpdatePhase("error", a.currentPending(), updateUserError(err))
		http.Redirect(w, r, "/utilities", http.StatusSeeOther)
		return
	}
	// Restart hands off to the Wails helper; the process exits via Host.Quit.
	http.Redirect(w, r, "/utilities", http.StatusSeeOther)
}

func (a *App) currentPending() *UpdateCheckResult {
	a.updates.mu.Lock()
	defer a.updates.mu.Unlock()
	if a.updates.pending == nil {
		return nil
	}
	c := *a.updates.pending
	return &c
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
func (a *App) BackgroundUpdateCheck(ctx context.Context) {
	if a.updateCheck == nil {
		return
	}
	res, err := a.updateCheck(ctx)
	if err != nil {
		log.Printf("background update check failed (workspace unaffected): %v", err)
		return
	}
	if !res.Available {
		return
	}
	c := res
	if c.CurrentVersion == "" {
		c.CurrentVersion = CurrentAppVersion()
	}
	a.setUpdatePhase("available", &c, "")
	log.Printf("background update check: version %s available (current %s)", c.LatestVersion, c.CurrentVersion)
}

// updateUserError keeps failure feedback comprehensible and free of
// workspace paths, keys, or applicant material.
func updateUserError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	// Truncate to avoid leaking staging paths or endpoint internals.
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	return "Update could not be completed: " + msg + ". Your current app and workspace are unchanged; the manual DMG remains available."
}
