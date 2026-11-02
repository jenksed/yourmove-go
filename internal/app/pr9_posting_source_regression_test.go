package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/jenksed/yourmove-go/internal/platforms"
	"github.com/jenksed/yourmove-go/internal/storage"
)

// A newly integrated supported provider must flow through the real single-URL
// intake: normalized opportunity persisted, sealed posting-source evidence
// attached to that job, visible through the React transport, and intact after
// a database restart. The provider response is synthetic but sealed and
// validated by the real platforms pipeline; no network is used.
func TestSingleIntakePersistsPostingSourceEvidence(t *testing.T) {
	a, dir := testWorkspace(t)
	const savedURL = "https://jobs.smartrecruiters.com/SampleCo/744000123456789"
	const description = "Build reliable applicant-controlled systems."
	sealed, err := platforms.SealPostingSource(platforms.PostingSource{
		Identity:         platforms.PostingIdentity{Provider: "smartrecruiters", Board: "SampleCo", PostingID: "744000123456789"},
		DescriptionHTML:  "<p>Build reliable applicant-controlled systems.</p>",
		DescriptionPlain: description,
		Location:         "Berlin, Germany",
	}, description)
	if err != nil {
		t.Fatal(err)
	}
	a.enrichmentFetch = func(context.Context, string) (platforms.Metadata, string, string) {
		return platforms.Metadata{
			Title: "Platform Engineer", Company: "SampleCo", Description: description,
			Source: platforms.AvailablePostingSource(sealed),
		}, "retrieved", "Posting details returned by a supported source; application availability not verified."
	}

	id, duplicate, err := a.savePostingURL(context.Background(), savedURL, "")
	if err != nil || duplicate || id <= 0 {
		t.Fatalf("intake id=%d duplicate=%v err=%v", id, duplicate, err)
	}
	job, err := a.jobByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if job.URL != savedURL || job.Host != "jobs.smartrecruiters.com" || job.Title != "Platform Engineer" ||
		job.Company != "SampleCo" || job.Description != description || job.SourceStatus != "retrieved" {
		t.Fatalf("unexpected saved opportunity: %+v", job)
	}
	state, found, err := loadJobPostingSource(a.db, job)
	if err != nil || !found || state.Invalid || state.Source == nil {
		t.Fatalf("posting source missing: %+v found=%v err=%v", state, found, err)
	}
	if state.Availability != platforms.PostingSourceAvailable || !state.DescriptionConfirmed {
		t.Fatalf("unexpected source state: %+v", state)
	}
	if err := state.Source.Identity.ValidateURL(savedURL); err != nil {
		t.Fatalf("source identity detached from saved posting: %v", err)
	}

	// A duplicate intake must report the existing job without replacing
	// applicant-owned records or evidence.
	dupID, duplicate, err := a.savePostingURL(context.Background(), savedURL+"#form", "Replacement")
	if err != nil || !duplicate || dupID != id {
		t.Fatalf("duplicate id=%d duplicate=%v err=%v", dupID, duplicate, err)
	}
	kept, err := a.jobByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if kept.Title != "Platform Engineer" || kept.SourceStatus != "retrieved" {
		t.Fatalf("duplicate replaced saved work: %+v", kept)
	}

	recorder := httptest.NewRecorder()
	a.opportunitiesAPI(recorder, httptest.NewRequest(http.MethodGet, "/api/opportunities", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("opportunities API status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Jobs []opportunityJobJSON `json:"jobs"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Jobs) != 1 || payload.Jobs[0].ID != id || payload.Jobs[0].Title != "Platform Engineer" ||
		payload.Jobs[0].SourceStatus != "retrieved" {
		t.Fatalf("unexpected React payload: %+v", payload.Jobs)
	}

	// Restart: the opportunity and its evidence must survive a reopen of the
	// same workspace database.
	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var title, status string
	if err := reopened.QueryRow(`SELECT title, source_status FROM jobs WHERE id=?`, id).Scan(&title, &status); err != nil {
		t.Fatal(err)
	}
	if title != "Platform Engineer" || status != "retrieved" {
		t.Fatalf("opportunity lost across restart: %q %q", title, status)
	}
	restarted, found, err := loadJobPostingSource(reopened, Job{ID: id, URL: savedURL, Description: description})
	if err != nil || !found || restarted.Invalid || restarted.Source == nil {
		t.Fatalf("posting source lost across restart: %+v found=%v err=%v", restarted, found, err)
	}
	if err := restarted.Source.Identity.ValidateURL(savedURL); err != nil {
		t.Fatalf("restarted source identity invalid: %v", err)
	}
	if restarted.Source.Location != "Berlin, Germany" {
		t.Fatalf("restarted source facts changed: %+v", restarted.Source)
	}
}
