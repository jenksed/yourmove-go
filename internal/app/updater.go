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
// A background success only surfaces from idle/up-to-date/available; it
// never overwrites a user-initiated checking/installing/ready/error phase.
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
	a.updates.mu.Lock()
	phase := a.updates.phase
	a.updates.mu.Unlock()
	switch phase {
	case "", "idle", "up-to-date", "available":
		// Safe to surface availability.
	default:
		// Preserve user-initiated checking/installing/ready/error states.
		log.Printf("background update check: version %s available but preserving user phase %q", res.LatestVersion, phase)
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
	// Keep only the first line: multi-line staging logs never reach the UI.
	if idx := indexNewline(msg); idx >= 0 {
		msg = msg[:idx]
	}
	msg = redactUpdateDetail(msg)
	// Truncate to avoid leaking staging paths or endpoint internals.
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	msg = redactUpdateDetail(msg)
	if msg == "" {
		msg = "an update error occurred"
	}
	return "Update could not be completed: " + msg + ". Your current app and workspace are unchanged; the manual DMG remains available."
}

func indexNewline(s string) int {
	for i, r := range s {
		if r == '\n' || r == '\r' {
			return i
		}
	}
	return -1
}

// redactUpdateDetail removes absolute filesystem paths and sensitive URL
// material while preserving short actionable phrases such as signature or
// network failures.
func redactUpdateDetail(s string) string {
	// Redact absolute POSIX paths.
	for {
		start := indexAbsPath(s)
		if start < 0 {
			break
		}
		end := start
		for end < len(s) && s[end] != ' ' && s[end] != '\t' && s[end] != '"' && s[end] != '\'' && s[end] != ')' && s[end] != ';' {
			end++
		}
		s = s[:start] + "[path]" + s[end:]
	}
	// Redact Windows absolute paths (e.g., C:\...).
	for {
		idx := indexWindowsPath(s)
		if idx < 0 {
			break
		}
		end := idx
		for end < len(s) && s[end] != ' ' && s[end] != '\t' && s[end] != '"' && s[end] != '\'' && s[end] != ')' && s[end] != ';' {
			end++
		}
		s = s[:idx] + "[path]" + s[end:]
	}
	// Redact URLs carrying query strings or credentials; bare hosts stay.
	for {
		idx := indexSensitiveURL(s)
		if idx < 0 {
			break
		}
		end := idx
		for end < len(s) && s[end] != ' ' && s[end] != '\t' && s[end] != '"' && s[end] != '\'' && s[end] != ')' && s[end] != ';' {
			end++
		}
		s = s[:idx] + "[url]" + s[end:]
	}
	return s
}

func indexAbsPath(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' && i+1 < len(s) && isPathChar(s[i+1]) {
			return i
		}
	}
	return -1
}

func isPathChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-' || c == '~'
}

func indexWindowsPath(s string) int {
	for i := 0; i+2 < len(s); i++ {
		c := s[i]
		if ((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) && s[i+1] == ':' && (s[i+2] == '\\' || s[i+2] == '/') {
			return i
		}
	}
	return -1
}

func indexSensitiveURL(s string) int {
	for i := 0; i+7 < len(s); i++ {
		var prefix string
		if len(s)-i >= 8 && (s[i:i+8] == "https://" || s[i:i+7] == "http://") {
			if s[i:i+8] == "https://" {
				prefix = "https://"
			} else {
				prefix = "http://"
			}
			rest := s[i+len(prefix):]
			// Only redact when the URL carries credentials, a query, or a token-like path.
			hasSensitive := false
			for j := 0; j < len(rest) && rest[j] != ' ' && rest[j] != '\t' && rest[j] != '"' && rest[j] != '\'' && rest[j] != ')' && rest[j] != ';'; j++ {
				if rest[j] == '?' || rest[j] == '@' || rest[j] == '#' {
					hasSensitive = true
					break
				}
			}
			if hasSensitive {
				return i
			}
		}
		_ = prefix
	}
	return -1
}
