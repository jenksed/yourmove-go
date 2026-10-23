package app

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/platforms"
)

func TestDescriptionSpacingPreservesStoredSourceAndSalaryManualEdits(t *testing.T) {
	a, _ := testWorkspace(t)
	raw := "About the role" + strings.Repeat("\n", 5) + "Salary: $90k–$120k per year"
	if err := a.saveJob(Job{URL: "https://example.test/role", Host: "example.test", Title: "Engineer", Description: raw}); err != nil {
		t.Fatal(err)
	}
	meta := platforms.Metadata{Description: raw, SalaryText: platforms.SalaryFromDescription(raw), SalaryOrigin: "description"}
	if err := a.applyPostingRefresh(1, meta, "retrieved", "fixture", "now"); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if job.Description != raw || job.DisplayDescription() != "About the role\n\n\nSalary: $90k–$120k per year" {
		t.Fatalf("source/spacing changed: %+v", job)
	}
	if job.SalaryText != meta.SalaryText || job.SalaryOrigin != "description" || job.SalaryOriginLabel() != "Employer description" {
		t.Fatalf("salary provenance lost: %+v", job)
	}
	w := httptest.NewRecorder()
	newMux(a).ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if !strings.Contains(w.Body.String(), `id="job-description">`+job.DisplayDescription()+`</div>`) {
		t.Fatal("description rendering ignored spacing policy")
	}
	job.SalaryText = "$130,000, confirmed manually"
	if err = a.updateJob(job); err != nil {
		t.Fatal(err)
	}
	if err = a.applyPostingRefresh(1, meta, "retrieved", "fixture", "later"); err != nil {
		t.Fatal(err)
	}
	job, err = a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if job.SalaryText != "$130,000, confirmed manually" || job.SalaryOrigin != "manual" || !job.SalaryManual {
		t.Fatalf("refresh overwrote manual pay: %+v", job)
	}
	if job.Description != raw {
		t.Fatal("presentation normalization rewrote source text")
	}
}
