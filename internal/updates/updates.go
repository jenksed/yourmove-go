// Package updates owns the installed-app updater's controller dependencies,
// pending release info, phase/state management and synchronization, and the
// check/install/restart/background-availability operations, including
// updater-specific error sanitization and redaction.
//
// internal/app retains HTTP route registration, request security, form
// validation, request-context timeouts, redirects/JSON status, Utilities
// presentation, and the existing public Go-facing updater API consumed by the
// desktop host (via type aliases and forwarding methods in internal/app).
//
// This package never imports internal/app: the installed version is supplied
// explicitly at construction (callers pass app.CurrentAppVersion()), keeping
// the dependency direction acyclic with exactly one concrete Manager type and
// no generalized interfaces, service containers, repositories, or DI.
package updates

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

// CheckResult is the applicant-visible outcome of an update check.
// It carries only release metadata, never applicant workspace material.
type CheckResult struct {
	Available      bool   `json:"available"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion,omitempty"`
	Name           string `json:"name,omitempty"`
	Notes          string `json:"notes,omitempty"`
	Size           int64  `json:"size,omitempty"`
}

// Controller buttons the Wails updater (owned by the desktop host) into the
// Go transport edge. Browser mode leaves all three nil: updates are an
// installed-app capability, and the manual DMG path remains the fallback.
type Controller struct {
	Check   func(ctx context.Context) (CheckResult, error)
	Install func(ctx context.Context) error
	Restart func(ctx context.Context) error
}

// ErrUnavailable reports that no update controller is configured (browser
// mode or an unconfigured desktop host). Transport maps it to the manual-DMG
// fallback response.
var ErrUnavailable = errors.New("Automatic updates are not available in this mode; use the manual DMG.")

// Manager is the single authority for updater phase and pending release
// state. It mirrors the user-facing phase for the Utilities page; detailed
// Wails lifecycle states remain in Wails event logs.
type Manager struct {
	mu             sync.Mutex
	currentVersion string
	phase          string // idle|checking|available|up-to-date|installing|ready|error|unavailable
	pending        *CheckResult
	err            string
	checkedAt      time.Time
	check          func(ctx context.Context) (CheckResult, error)
	install        func(ctx context.Context) error
	restart        func(ctx context.Context) error
	controllerSet  bool
}

// NewManager returns a Manager that reports currentVersion as the installed
// version whenever a check result omits it. Pass app.CurrentAppVersion()
// explicitly at the call site; this package must not import internal/app.
func NewManager(currentVersion string) *Manager {
	return &Manager{currentVersion: currentVersion, phase: "idle"}
}

// SetController wires the desktop updater callbacks into the manager.
func (m *Manager) SetController(c Controller) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.controllerSet = c.Check != nil
	m.check = c.Check
	m.install = c.Install
	m.restart = c.Restart
	if m.phase == "" {
		m.phase = "idle"
	}
}

// HasCheck reports whether an update check controller is configured.
func (m *Manager) HasCheck() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.check != nil
}

// HasInstall reports whether an update install controller is configured.
func (m *Manager) HasInstall() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.install != nil
}

// HasRestart reports whether an update restart controller is configured.
func (m *Manager) HasRestart() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.restart != nil
}

// Snapshot returns a copy for templates and JSON APIs.
func (m *Manager) Snapshot() (phase string, pending *CheckResult, errMsg string, checkedAt time.Time, enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.check == nil {
		return "unavailable", nil, "", time.Time{}, false
	}
	if m.phase == "" {
		return "idle", nil, "", m.checkedAt, true
	}
	var cp *CheckResult
	if m.pending != nil {
		c := *m.pending
		cp = &c
	}
	return m.phase, cp, m.err, m.checkedAt, true
}

// SetPhase records an explicit phase transition. It is the transport/tests
// seam for seeding or overriding user-facing phase state; the check/install/
// restart/background operations below are the production paths that call it.
func (m *Manager) SetPhase(phase string, pending *CheckResult, errMsg string) {
	m.setPhase(phase, pending, errMsg)
}

func (m *Manager) setPhase(phase string, pending *CheckResult, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.phase = phase
	m.pending = pending
	m.err = errMsg
	if phase != "checking" {
		m.checkedAt = time.Now()
	}
}

func (m *Manager) currentPending() *CheckResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pending == nil {
		return nil
	}
	c := *m.pending
	return &c
}

func (m *Manager) checkFunc() func(ctx context.Context) (CheckResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.check
}

func (m *Manager) installFunc() func(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.install
}

func (m *Manager) restartFunc() func(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.restart
}

func (m *Manager) installedVersion() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentVersion
}

// RunCheck performs one explicit applicant-initiated check. It never runs
// automatically from page loads; background checks live in BackgroundCheck.
// The raw error is returned for transport logging; user-facing state is
// already recorded via SetPhase.
func (m *Manager) RunCheck(ctx context.Context) error {
	check := m.checkFunc()
	if check == nil {
		return ErrUnavailable
	}
	m.setPhase("checking", nil, "")
	res, err := check(ctx)
	if err != nil {
		// Offline or endpoint failures must not block local work.
		// Keep the current app usable; surface a comprehensible message.
		m.setPhase("error", nil, UserError(err))
		return err
	}
	if res.Available {
		c := res
		m.setPhase("available", &c, "")
		return nil
	}
	if res.CurrentVersion == "" {
		res.CurrentVersion = m.installedVersion()
	}
	c := res
	m.setPhase("up-to-date", &c, "")
	return nil
}

// RunInstall performs one explicit approval-gated download/authenticate/stage
// via the desktop updater; it never auto-runs.
func (m *Manager) RunInstall(ctx context.Context) error {
	install := m.installFunc()
	if install == nil {
		return ErrUnavailable
	}
	m.setPhase("installing", m.currentPending(), "")
	if err := install(ctx); err != nil {
		m.setPhase("error", m.currentPending(), UserError(err))
		return err
	}
	m.setPhase("ready", m.currentPending(), "")
	return nil
}

// RunRestart hands off to the desktop updater; the process exits via the
// host after a successful restart. It records no success phase.
func (m *Manager) RunRestart(ctx context.Context) error {
	restart := m.restartFunc()
	if restart == nil {
		return ErrUnavailable
	}
	if err := restart(ctx); err != nil {
		m.setPhase("error", m.currentPending(), UserError(err))
		return err
	}
	return nil
}

// BackgroundCheck records availability without opening any UI.
// It is the nonblocking automatic path: offline or endpoint errors leave
// the current app usable and do not overwrite an explicit user state.
// A background success only surfaces from idle/up-to-date/available; it
// never overwrites a user-initiated checking/installing/ready/error phase.
func (m *Manager) BackgroundCheck(ctx context.Context) {
	check := m.checkFunc()
	if check == nil {
		return
	}
	res, err := check(ctx)
	if err != nil {
		log.Printf("background update check failed (workspace unaffected): %v", err)
		return
	}
	if !res.Available {
		return
	}
	m.mu.Lock()
	phase := m.phase
	m.mu.Unlock()
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
		c.CurrentVersion = m.installedVersion()
	}
	m.setPhase("available", &c, "")
	log.Printf("background update check: version %s available (current %s)", c.LatestVersion, c.CurrentVersion)
}

// UserError keeps failure feedback comprehensible and free of workspace
// paths, keys, or applicant material.
func UserError(err error) string {
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
