//go:build qualification

package app

import (
	"github.com/jenksed/yourmove-go/internal/application/schema"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareLibraryNativeFixture(t *testing.T) {
	root := os.Getenv("YOURMOVE_NATIVE_FIXTURE_DIR")
	if root == "" {
		t.Skip("opt-in synthetic fixture")
	}
	if !strings.HasPrefix(root, "/private/tmp/yourmove-library-native-") || filepath.Clean(root) != root {
		t.Fatal("synthetic temporary path required")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	a, _ := testWorkspace(t)
	for i, label := range []string{"Tell us about a time you handled a difficult customer.", "Describe a time you resolved a customer complaint."} {
		url := testAshbyPosting
		if i == 1 {
			url = strings.TrimSuffix(url, "5") + "6"
		}
		if err := a.saveJob(Job{URL: url, Host: "jobs.ashbyhq.com", Title: []string{"Opportunity A", "Opportunity B"}[i], Company: "Synthetic Library Acceptance"}); err != nil {
			t.Fatal(err)
		}
		_, err := a.saveInspection(int64(i+1), schema.Result{Platform: "ashby", Source: "preview", Completeness: "public_schema", ApplicationURL: url, Fields: []schema.Field{{ID: "application:story", Label: label, Kind: "textarea", Required: true}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(root, "jobs.db")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("refusing existing database")
	}
	if _, err := a.db.Exec("VACUUM INTO ?", target); err != nil {
		t.Fatal(err)
	}
}
