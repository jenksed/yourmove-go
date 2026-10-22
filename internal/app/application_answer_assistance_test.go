package app

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func TestQuestionAssistanceDefersAIForFiftyQuestions(t *testing.T) {
	a, _, job, first, firstKey := newAnswerLibraryFixture(t)
	fields := []schema.Field{first}
	for i := 2; i <= 50; i++ {
		fields = append(fields, schema.Field{ID: "browser:story-" + strconv.Itoa(i), Label: "Describe a customer technical incident and how you resolved it", Kind: "textarea"})
	}
	if _, err := a.saveInspection(job.ID, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: job.URL + "/application", Fields: fields}); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	page := httptestGet(t, newMux(a), "/jobs/1")
	renderTime := time.Since(started)
	if page.Code != http.StatusOK {
		t.Fatalf("render 50 discovered questions: %d", page.Code)
	}
	body := page.Body.String()
	if strings.Contains(body, `data-answer-ai `) || strings.Contains(body, `data-answer-ai-controls`) || strings.Contains(body, "Choose factual evidence") {
		t.Fatal("initial Prepare rendered per-question AI source forms")
	}
	if strings.Count(body, `data-question-help `) != 50 {
		t.Fatalf("bounded help controls missing: got %d of 50", strings.Count(body, `data-question-help `))
	}
	t.Logf("50-question synthetic initial render: %d HTML bytes in %s; AI forms=0; assistance entry points=50", len(body), renderTime)
	profile, err := a.candidateProfile()
	if err != nil {
		t.Fatal(err)
	}
	q, err := a.prepareQuestions(job, profile)
	if err != nil || len(q.Questions) != 50 {
		t.Fatalf("prepare 50 questions: count=%d err=%v", len(q.Questions), err)
	}
	started = time.Now()
	values := url.Values{"question_key": {firstKey}, "revision": {"0"}, "destination": {job.URL}}
	response := postApp(newMux(a), "/jobs/1/answers/help", values, "", "")
	activationTime := time.Since(started)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Draft with AI") || strings.Count(response.Body.String(), `data-answer-ai data-job=`) != 1 {
		t.Fatalf("single-question assistance response: status=%d AI forms=%d", response.Code, strings.Count(response.Body.String(), `data-answer-ai data-job=`))
	}
	t.Logf("synthetic assistance activation: one request in %s; response=%d bytes; AI forms=1", activationTime, response.Body.Len())
}
