package app

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/browser/playwright"
)

// settingApplicationBrowser is the settings-table key for the operator's
// chosen employer-application browser. Only engines the production session
// factory can launch are accepted; WebKit stays out until the factory and
// its qualification support it.
const settingApplicationBrowser = "application_browser"

// supportedApplicationBrowsers are the engines the Utilities selector offers.
func supportedApplicationBrowsers() []string { return []string{"chromium", "firefox"} }

func validApplicationBrowser(name string) bool {
	for _, supported := range supportedApplicationBrowsers() {
		if name == supported {
			return true
		}
	}
	return false
}

// applicationBrowserSetting returns the saved preference, or "" when the
// operator never chose one. Unknown stored values are preserved in the
// database; resolution falls back to chromium.
func (a *App) applicationBrowserSetting() (string, error) {
	var value string
	err := a.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, settingApplicationBrowser).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

func (a *App) saveApplicationBrowserSetting(engine string) error {
	if !validApplicationBrowser(engine) {
		return errors.New("unsupported application browser; choose chromium or firefox")
	}
	_, err := a.db.Exec(`INSERT INTO settings(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, settingApplicationBrowser, engine)
	return err
}

// defaultApplicationBrowserFromEnv preserves the historical environment-only
// behavior for sessionHost users without a wired preference.
func defaultApplicationBrowserFromEnv() (string, error) {
	return os.Getenv("YOURMOVE_APPLICATION_BROWSER"), nil
}

// resolveApplicationBrowser is the session factory's engine source. An
// explicit environment override wins (operator tooling and tests); otherwise
// the saved preference applies; unset or unknown values mean chromium.
func (a *App) resolveApplicationBrowser() (string, error) {
	if chosen := os.Getenv("YOURMOVE_APPLICATION_BROWSER"); chosen != "" {
		return chosen, nil
	}
	saved, err := a.applicationBrowserSetting()
	if err != nil {
		return "", err
	}
	if validApplicationBrowser(saved) {
		return saved, nil
	}
	return "chromium", nil
}

type utilitiesPageData struct {
	Desktop        bool
	BrowserCurrent string
	BrowserEngines []playwright.EngineInfo
	Notice         string
	AppVersion     string
	AppBuild       string
	UpdateEnabled  bool
	UpdatePhase    string
	UpdatePending  *UpdateCheckResult
	UpdateError    string
}

func (a *App) renderUtilities(w http.ResponseWriter, notice string) {
	current, err := a.resolveApplicationBrowser()
	if err != nil {
		log.Printf("utilities browser preference: %v", err)
		http.Error(w, "Cannot load browser preference", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	engines := playwright.AvailableEngines()
	supported := make([]playwright.EngineInfo, 0, len(engines))
	for _, name := range supportedApplicationBrowsers() {
		for _, info := range engines {
			if info.Name != name {
				continue
			}
			// Firefox launches from the Playwright cache, which
			// AvailableEngines already checks. Chromium launches the
			// installed system browser instead, so availability must
			// follow the production resolution path.
			if name == "chromium" {
				if _, err := chromium.ResolveExecutable(); err != nil {
					info.Available = false
					info.Reason = err.Error()
				} else {
					info.Available = true
					info.Reason = ""
				}
			}
			supported = append(supported, info)
		}
	}
	data := utilitiesPageData{BrowserCurrent: current, BrowserEngines: supported, Notice: notice, Desktop: a.openPosting != nil,
		AppVersion: CurrentAppVersion(), AppBuild: CurrentAppBuild()}
	phase, pending, updateErr, _, enabled := a.updateSnapshot()
	data.UpdateEnabled = enabled
	data.UpdatePhase = phase
	data.UpdatePending = pending
	data.UpdateError = updateErr
	if err := a.resources.utilitiesTemplate.Execute(w, data); err != nil {
		log.Printf("render utilities: %v", err)
	}
}

func (a *App) utilitiesPage(w http.ResponseWriter, r *http.Request) { a.renderUtilities(w, "") }

func (a *App) saveApplicationBrowser(w http.ResponseWriter, r *http.Request) {
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	engine := r.PostForm.Get("browser")
	if err := a.saveApplicationBrowserSetting(engine); err != nil {
		a.renderUtilities(w, "That browser cannot be selected: "+err.Error())
		return
	}
	http.Redirect(w, r, "/utilities", http.StatusSeeOther)
}

// openReleaseNotes uses a fixed destination with the desktop system browser.
func (a *App) openReleaseNotes(w http.ResponseWriter, r *http.Request) {
	if a.openPosting == nil {
		http.NotFound(w, r)
		return
	}
	if err := a.openPosting("https://www.yourmove.run/docs/releases/"); err != nil {
		http.Error(w, "Could not open release notes in your browser", http.StatusBadGateway)
		return
	}
	http.Redirect(w, r, "/utilities", http.StatusSeeOther)
}
