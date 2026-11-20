//go:build qualification

package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// Opt-in seed for manual/native UI qualification; refuses existing databases
// and non-temporary paths. No normal applicant workspace is opened.
// The seeded job is visibly synthetic ("Synthetic native workflow" /
// "Fixture only" / "Synthetic saved letter") so fixture state is never
// mistaken for applicant data.
func TestPrepareNativeWorkflowFixture(t *testing.T) {
	root := os.Getenv("YOURMOVE_NATIVE_FIXTURE_DIR")
	if root == "" {
		t.Skip("native synthetic fixture not requested")
	}
	if err := prepareNativeWorkflowFixture(t, root); err != nil {
		t.Fatal(err)
	}
	t.Log("native synthetic workspace:", root)
}

func prepareNativeWorkflowFixture(t *testing.T, root string) error {
	t.Helper()
	if !strings.HasPrefix(root, "/private/tmp/yourmove-workflow-native-") || filepath.Clean(root) != root {
		return errors.New("fixture must use a dedicated temporary path")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Synthetic native workflow", Company: "Fixture only"}); err != nil {
		return err
	}
	if err := a.saveCandidateProfile(CandidateProfile{
		FirstName: "Alex", LastName: "Fixture", Email: "alex@example.test",
		LinkedIn: "https://www.linkedin.com/in/native-fixture", GitHub: "https://github.com/native-fixture",
		Portfolio: "https://portfolio.example.test/native-fixture", Approved: true,
	}); err != nil {
		return err
	}
	fields := []schema.Field{
		{ID: "browser:name", Label: "Name", Kind: "text"},
		{ID: "browser:linkedin", Label: "LinkedIn URL", Kind: "url"},
		{ID: "browser:website", Label: "Website", Kind: "url", Required: true},
		{ID: "browser:github_url", Label: "GitHub profile URL", Kind: "url"},
		{ID: "browser:motivation", Label: "[Synthetic fixture] Why this team?", Kind: "textarea"},
	}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: testAshbyPosting + "/application", Fields: fields}); err != nil {
		return err
	}
	if _, err := a.db.Exec(`INSERT INTO cover_letters(job_id,body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at) VALUES(1,'Synthetic saved letter','fixture','none',1,'master',1,'role','job','now')`); err != nil {
		return err
	}
	target := filepath.Join(root, "jobs.db")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		return errors.New("refusing existing fixture database")
	}
	if _, err := a.db.Exec("VACUUM INTO ?", target); err != nil {
		return err
	}
	return nil
}
