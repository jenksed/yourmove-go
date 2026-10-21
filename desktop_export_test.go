package main

import (
	"bytes"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDesktopExportsExactSavedDocumentsAndReturns(t *testing.T) {
	a, f := scopeFirstWorkspace(t)
	pdf, err := a.importResume("extra.pdf", strings.NewReader("%PDF-1.4\nsynthetic saved PDF"))
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"/resumes/2/download?job_id=1", "/resumes/" + strconv.FormatInt(pdf.ID, 10) + "/download", "/jobs/1/cover-letter/1/download"} {
		t.Run(route, func(t *testing.T) {
			browser := httptest.NewRecorder()
			newMux(a).ServeHTTP(browser, httptest.NewRequest("GET", "http://localhost"+route, nil))
			if browser.Code != 200 {
				t.Fatal(browser.Code, browser.Body.String())
			}
			destination := filepath.Join(t.TempDir(), "saved-copy")
			calls := 0
			a.chooseExportPath = func(name string) (string, error) {
				calls++
				if !strings.Contains(browser.Header().Get("Content-Disposition"), name) {
					t.Fatalf("suggested filename differs: %s", name)
				}
				return destination, nil
			}
			response := httptest.NewRecorder()
			newDesktopMux(a).ServeHTTP(response, httptest.NewRequest("GET", "http://localhost"+route, nil))
			if calls != 1 || response.Code != 200 || !strings.Contains(response.Body.String(), "window.location.replace") {
				t.Fatalf("native return: %d %s", response.Code, response.Body.String())
			}
			got, err := os.ReadFile(destination)
			if err != nil || !bytes.Equal(got, browser.Body.Bytes()) {
				t.Fatalf("export differs from saved HTTP artifact: %v", err)
			}
			info, _ := os.Stat(destination)
			if info.Mode().Perm() != 0600 {
				t.Fatalf("export permissions %v", info.Mode())
			}
		})
	}
	if f.calls != 0 {
		t.Fatal("export transferred to employer")
	}
}

func TestDesktopExportCancellationAndTrustBoundary(t *testing.T) {
	a, _ := scopeFirstWorkspace(t)
	calls := 0
	a.chooseExportPath = func(string) (string, error) { calls++; return "", nil }
	for _, route := range []string{"/resumes/2/download", "/jobs/1/cover-letter/1/download"} {
		for _, method := range []string{"GET", "HEAD"} {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(method, "http://localhost"+route, nil)
			r.Header.Set("X-Wails-Window-Id", "1")
			newMux(a).ServeHTTP(w, r)
			if w.Code != 200 || calls != 0 {
				t.Fatal("browser gained native capability")
			}
		}
		w := httptest.NewRecorder()
		newDesktopMux(a).ServeHTTP(w, httptest.NewRequest("GET", "http://localhost"+route, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), "window.location.replace") {
			t.Fatal("cancel did not return")
		}
		calls = 0
	}
	// Integrity errors and cross-origin requests cannot even open a dialog.
	r, _ := a.resumeByID(2)
	if err := os.Remove(filepath.Join(a.resumeDir, r.StoredName)); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	newDesktopMux(a).ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/resumes/2/download", nil))
	if w.Code != 404 || calls != 0 {
		t.Fatal("missing document reached Save dialog")
	}
	w = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://localhost/jobs/1/cover-letter/1/download", nil)
	req.Header.Set("Origin", "https://attacker.example")
	newDesktopMux(a).ServeHTTP(w, req)
	if w.Code != 403 || calls != 0 {
		t.Fatal("cross-origin native export")
	}
}

func TestExportCannotOverwriteWorkspaceAndFailurePreservesDestination(t *testing.T) {
	a, _ := testWorkspace(t)
	path := filepath.Join(filepath.Dir(a.resumeDir), "jobs.db")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.writeExportFile(path, []byte("wrong")); err == nil {
		t.Fatal("workspace overwrite allowed")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("workspace changed")
	}
	outside := t.TempDir()
	link := filepath.Join(outside, "alias")
	if err := os.Symlink(filepath.Dir(a.resumeDir), link); err != nil {
		t.Fatal(err)
	}
	if err := a.writeExportFile(filepath.Join(link, "jobs.db"), []byte("wrong")); err == nil {
		t.Fatal("workspace alias overwrite allowed")
	}
	if err := a.writeExportFile(outside, []byte("wrong")); err == nil {
		t.Fatal("directory replaced")
	}
	entries, _ := os.ReadDir(filepath.Dir(outside))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".yourmove-export-") {
			t.Fatal("failed export leaked staging file")
		}
	}
}
