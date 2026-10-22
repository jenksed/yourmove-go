package app

import (
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Use the same validated saved bytes as HTTP downloads. Only the in-process
// desktop entry point can open a native dialog; browser headers cannot grant it.
func (a *App) exportDesktopDocument(w http.ResponseWriter, r *http.Request, name string, data []byte, back string) bool {
	if r.Method != http.MethodGet || !isDesktopRequest(r) || a.chooseExportPath == nil {
		return false
	}
	if r.URL.Query().Get("job_id") != "" {
		// downloadResume has already validated this job and its selected document.
		jobID, _ := strconv.ParseInt(r.URL.Query().Get("job_id"), 10, 64)
		back = "/jobs/" + strconv.FormatInt(jobID, 10) + "#prepare"
	}
	path, err := a.chooseExportPath(name)
	if err == nil && path != "" {
		err = a.writeExportFile(path, data)
	}
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "private, no-store")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, `<!doctype html><html lang="en"><meta charset="utf-8"><title>Copy not saved</title><p>The copy could not be saved. Your saved document is still available in YourMove.</p><p><a href="%s">Return to your document</a></p></html>`, html.EscapeString(back))
		return true
	}
	// Includes cancellation: no write and return to the existing workspace.
	http.Redirect(&desktopRedirectWriter{ResponseWriter: w, requestPath: r.URL.Path}, r, back, http.StatusSeeOther)
	return true
}

func (a *App) writeExportFile(path string, data []byte) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("export destination must be absolute")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return err
	}
	root, err := filepath.EvalSymlinks(filepath.Dir(a.resumeDir))
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, parent)
	if err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return fmt.Errorf("export destination must be outside the application workspace")
	}
	// Stage beside the destination so cancellation/failure cannot truncate it.
	file, err := os.CreateTemp(parent, ".yourmove-export-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(data)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), filepath.Join(parent, filepath.Base(path)))
}
