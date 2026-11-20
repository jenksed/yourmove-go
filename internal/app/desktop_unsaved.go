package app

import (
	"encoding/json"
	"net/http"
)

// Desktop unsaved-work flag. React editors report dirty answer, letter,
// job-detail, and evidence drafts through POST /api/desktop/unsaved; the
// Wails host consults HasDesktopUnsaved in its window-close and quit
// guards so ordinary application closure cannot silently discard them.
// Browser tabs keep their beforeunload listeners. Explicit saves and
// revision checks are unchanged; this flag never writes applicant data.
func (a *App) SetDesktopUnsaved(dirty bool) {
	a.desktopUnsaved.Store(dirty)
}

func (a *App) HasDesktopUnsaved() bool {
	return a.desktopUnsaved.Load()
}

func (a *App) desktopUnsavedAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
	var input struct {
		Dirty bool `json:"dirty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid unsaved state", http.StatusBadRequest)
		return
	}
	a.SetDesktopUnsaved(input.Dirty)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"dirty": input.Dirty})
}
