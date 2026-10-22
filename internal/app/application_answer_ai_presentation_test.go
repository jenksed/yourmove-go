package app

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNarrativeWorkbenchPresentationIsOptionalAndUnselected(t *testing.T) {
	a, _, _ := narrativeFixture(t)
	page := httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	body := page.Body.String()
	for _, s := range []string{"Draft answer with AI", "data-answer-ai-controls hidden", "Inline AI drafting requires JavaScript", "Use as unsaved answer", "data-answer-ai-payload", "name=\"career_evidence\""} {
		if !strings.Contains(body, s) {
			t.Fatal("missing", s)
		}
	}
	if strings.Contains(body, `name="career_evidence" checked`) {
		t.Fatal("auto-selected evidence")
	}
	start, end := strings.Index(body, `class="application-question-form"`), strings.Index(body, `data-answer-ai-controls`)
	if start < 0 || !strings.Contains(body[start:end], "</form>") {
		t.Fatal("drafting nested in answer save form")
	}
	if _, err := a.db.Exec(`UPDATE jobs SET stage=? WHERE id=1`, StageApplied); err != nil {
		t.Fatal(err)
	}
	page = httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if strings.Contains(page.Body.String(), "data-answer-ai-controls") {
		t.Fatal("completed application offers drafting")
	}
}
