package app

import (
	"context"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/platforms"
)

func TestJobOverviewM0Characterization(t *testing.T) {
	a, _ := testWorkspace(t)
	description := strings.Join([]string{
		"About Northstar Analytics",
		"Northstar builds data tools for distributed public-interest teams. This synthetic posting describes a long role so the current Overview can be characterized without candidate information.",
		"",
		"About the role",
		"You will help a small platform group improve reliability, documentation, and the day-to-day work of product engineers. The team values clear writing and careful handoffs.",
		"",
		"What you'll do",
		"• Design dependable service boundaries and explain tradeoffs.",
		"• Improve operational tooling and developer workflows.",
		"  • Add useful metrics and test failure recovery.",
		"  • Keep runbooks current and easy to follow.",
		"• Work with product and support on issues that cross team boundaries.",
		"",
		"What we're looking for",
		"• Experience maintaining production services.",
		"• Ability to investigate unfamiliar systems and communicate clearly.",
		"• Interest in mentoring colleagues and improving team practices.",
		"",
		"How we work",
		"The role works with a remote-first team across several time zones. Written plans and recorded design decisions help colleagues contribute asynchronously.",
		"",
		"Additional information",
		"Read the synthetic team guide at https://guide.example.test/engineering before interviews. The hiring team will discuss accommodations and interview timing with candidates.",
		"",
		"End of synthetic posting.",
	}, "\n")
	postingURL := "https://jobs.ashbyhq.com/northstar/11111111-1111-4111-8111-111111111111"
	id, added, err := a.storeJobIfNew(Job{
		URL:             postingURL,
		Host:            "jobs.ashbyhq.com",
		Title:           "Senior Platform Engineer",
		Company:         "Northstar Analytics",
		Description:     description,
		SalaryText:      "$145,000–$185,000 USD per year",
		SalaryOrigin:    "employer",
		Stage:           StageReview,
		Priority:        PriorityNormal,
		SourceStatus:    "retrieved",
		SourceNote:      "Synthetic source fixture; application availability is not verified.",
		SourceCheckedAt: "2026-10-24T12:00:00Z",
	})
	if err != nil || !added {
		t.Fatalf("store synthetic Ashby job: id=%d added=%v err=%v", id, added, err)
	}
	job, err := a.jobByID(id)
	if err != nil {
		t.Fatalf("load inserted job %d: %v", id, err)
	}
	mux := newMux(a)
	path := "/jobs/" + strconv.FormatInt(id, 10)
	viewRequest := httptest.NewRequest(http.MethodGet, "http://localhost"+path, nil)
	viewRequest.RemoteAddr = "127.0.0.1:12345"
	view := httptest.NewRecorder()
	mux.ServeHTTP(view, viewRequest)
	if view.Code != http.StatusOK {
		t.Fatalf("GET Overview status=%d job=%+v body=%s", view.Code, job, view.Body.String())
	}
	for _, want := range []string{
		`id="job-description">` + html.EscapeString(job.DisplayDescription()) + `</div>`,
		`id="edit-details"`,
		`action="/jobs/` + strconv.FormatInt(id, 10) + `/edit"`,
		`name="description"`,
		`action="/jobs/` + strconv.FormatInt(id, 10) + `/refresh"`,
		`data-copy-description hidden`,
		"$145,000–$185,000 USD per year",
		"End of synthetic posting.",
	} {
		if !strings.Contains(view.Body.String(), want) {
			t.Errorf("Overview missing %q", want)
		}
	}

	manualDescription := "Applicant's synthetic correction.\n\nKeep this paragraph during refresh."
	manualSalary := "$155,000–$190,000 USD per year (reviewed manually)"
	form := url.Values{
		"title":       {job.Title},
		"company":     {job.Company},
		"description": {manualDescription},
		"salary_text": {manualSalary},
		"stage":       {StageReview},
		"priority":    {PriorityNormal},
	}
	editRequest := httptest.NewRequest(http.MethodPost, "http://localhost"+path+"/edit", strings.NewReader(form.Encode()))
	editRequest.RemoteAddr = "127.0.0.1:12345"
	editRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	edit := httptest.NewRecorder()
	mux.ServeHTTP(edit, editRequest)
	if edit.Code != http.StatusSeeOther {
		t.Fatalf("POST edit status=%d body=%s", edit.Code, edit.Body.String())
	}

	var refreshCalls int
	a.enrichmentFetch = func(_ context.Context, rawURL string) (platforms.Metadata, string, string) {
		refreshCalls++
		if rawURL != postingURL {
			t.Fatalf("refresh URL=%q, want saved posting URL", rawURL)
		}
		return platforms.Metadata{
			Title:       "Senior Platform Engineer (updated source)",
			Company:     "Northstar Analytics",
			Description: "Synthetic refreshed employer description.",
			SalaryText:  "$160,000–$200,000 USD per year",
		}, "retrieved", "Synthetic source check completed."
	}
	refreshRequest := httptest.NewRequest(http.MethodPost, "http://localhost"+path+"/refresh", nil)
	refreshRequest.RemoteAddr = "127.0.0.1:12345"
	refresh := httptest.NewRecorder()
	mux.ServeHTTP(refresh, refreshRequest)
	if refresh.Code != http.StatusSeeOther || refreshCalls != 1 {
		t.Fatalf("POST refresh status=%d calls=%d body=%s", refresh.Code, refreshCalls, refresh.Body.String())
	}
	job, err = a.jobByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if job.Description != manualDescription || !job.DescriptionManual {
		t.Fatalf("refresh changed manual description: %+v", job)
	}
	if job.SalaryText != manualSalary || job.SalaryOrigin != "manual" || !job.SalaryManual {
		t.Fatalf("refresh changed manual salary: %+v", job)
	}
	if job.Title != "Senior Platform Engineer (updated source)" || job.SourceStatus != "retrieved" {
		t.Fatalf("refresh did not update nonmanual source fields: %+v", job)
	}
}
