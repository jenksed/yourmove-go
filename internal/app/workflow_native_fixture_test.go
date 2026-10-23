//go:build qualification

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// Opt-in seed for manual/native UI qualification; refuses existing databases
// and non-temporary paths. No normal applicant workspace is opened.
func TestPrepareNativeWorkflowFixture(t *testing.T) {
	root := os.Getenv("YOURMOVE_NATIVE_FIXTURE_DIR")
	if root == "" {
		t.Skip("native synthetic fixture not requested")
	}
	if !strings.HasPrefix(root, "/private/tmp/yourmove-workflow-native-") || filepath.Clean(root) != root {
		t.Fatal("fixture must use a dedicated temporary path")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Synthetic native workflow", Company: "Fixture only"}); err != nil {
		t.Fatal(err)
	}
	fields := []schema.Field{{ID: "browser:name", Label: "Name", Kind: "text"}, {ID: "browser:motivation", Label: "Why this team?", Kind: "textarea"}, {ID: "browser:website", Label: "Website", Kind: "text"}}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: testAshbyPosting + "/application", Fields: fields}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`INSERT INTO cover_letters(job_id,body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at) VALUES(1,'Synthetic saved letter','fixture','none',1,'master',1,'role','job','now')`); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "jobs.db")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("refusing existing fixture database")
	}
	if _, err := a.db.Exec("VACUUM INTO ?", target); err != nil {
		t.Fatal(err)
	}
	t.Log("native synthetic workspace:", root)
}
