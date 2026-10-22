package app

import (
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestResumeSelectorsUseSameIdentityAndLabel(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveFamily(RoleFamily{Name: "Technical Support Engineering", TitleTerms: "support engineer"}); err != nil {
		t.Fatal(err)
	}
	resume, err := a.importResume("Technical Support Resume.pdf", strings.NewReader("%PDF-1.4\nfixture"))
	if err != nil {
		t.Fatal(err)
	}
	if err = a.assignResumeFamily(resume.ID, 1, true); err != nil {
		t.Fatal(err)
	}
	if err = a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	render := func() string {
		t.Helper()
		rec := httptest.NewRecorder()
		newMux(a).ServeHTTP(rec, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
		if rec.Code != 200 {
			t.Fatal(rec.Code)
		}
		return rec.Body.String()
	}
	selected := func(body, id string) string {
		t.Helper()
		sel := regexp.MustCompile(`(?s)<select id="` + id + `".*?</select>`).FindString(body)
		option := regexp.MustCompile(`<option value="1" selected>(.*?)</option>`).FindStringSubmatch(sel)
		if len(option) != 2 {
			t.Fatalf("saved selection missing: %s", sel)
		}
		return option[1]
	}
	page := render()
	application, letter := selected(page, "resume_id"), selected(page, "letter_resume_id")
	if application != letter || !strings.Contains(letter, "Technical Support Engineering · Technical Support Resume.pdf (.pdf) · Primary") {
		t.Fatalf("inconsistent labels: application=%q letter=%q", application, letter)
	}
	// An unavailable writing source must not silently select another document.
	if _, err = a.db.Exec("UPDATE resumes SET archived_at='fixture' WHERE id=?", resume.ID); err != nil {
		t.Fatal(err)
	}
	page = render()
	selector := regexp.MustCompile(`(?s)<select id="letter_resume_id".*?</select>`).FindString(page)
	if !strings.Contains(selector, `<option value="">Choose a resume</option>`) || strings.Contains(selector, `value="1"`) {
		t.Fatalf("unavailable resume fell back silently: %s", selector)
	}
}
