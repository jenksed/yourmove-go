package app

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func addCareerEvidenceForSearch(t *testing.T, a *App, content careerEvidenceContent, reviewed bool) CareerEvidence {
	t.Helper()
	id, err := a.createCareerEvidenceDraft()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.updateCareerEvidence(id, 1, content, false); err != nil {
		t.Fatal(err)
	}
	evidence, err := careerEvidenceByID(a.db, id)
	if err != nil {
		t.Fatal(err)
	}
	if reviewed {
		if err := a.updateCareerEvidence(id, evidence.Revision, content, true); err != nil {
			t.Fatal(err)
		}
		evidence, err = careerEvidenceByID(a.db, id)
		if err != nil {
			t.Fatal(err)
		}
	}
	return evidence
}

func TestCareerEvidenceRankingIsRelevantDeterministicAndBounded(t *testing.T) {
	query := "Describe a production incident you investigated for a customer"
	docs := []careerEvidenceSearchDocument{
		newCareerEvidenceSearchDocument(CareerEvidence{ID: 8, Title: "Customer newsletter launch", Actions: "Coordinated marketing and communications."}),
		newCareerEvidenceSearchDocument(CareerEvidence{ID: 5, Title: "Production incident investigation", Situation: "A customer-facing application slowed during production.", Actions: "Reviewed Linux and MySQL logs and compared EXPLAIN plans."}),
		newCareerEvidenceSearchDocument(CareerEvidence{ID: 3, Title: "Production incident investigation", Situation: "A customer-facing application slowed during production.", Actions: "Reviewed Linux and MySQL logs and compared EXPLAIN plans."}),
	}
	ranked := rankCareerEvidenceDocuments(query, docs, 20)
	if len(ranked) != 2 || ranked[0].document.evidenceID != 3 || ranked[1].document.evidenceID != 5 {
		t.Fatalf("relevance ordering or result cap changed: %+v", ranked)
	}
	if unrelated := rankCareerEvidenceDocuments("Describe a team mentorship program with marketing partners", docs, careerEvidenceRelatedLimit); len(unrelated) != 0 {
		t.Fatalf("unrelated evidence surfaced: %+v", unrelated)
	}
}

func TestCareerEvidenceSuggestionsRequireReviewAndStaySeparateFromPreviousWriting(t *testing.T) {
	a, _, job, field, key := newAnswerLibraryFixture(t)
	originalAnswer := "Synthetic previous writing about production incident investigation."
	saveLibrarySourceAnswer(t, a, job, key, originalAnswer, 0)
	if promoted := promoteLibrarySource(t, a, job.ID, key, 1); promoted.Code != http.StatusSeeOther {
		t.Fatalf("promote previous writing: %d %s", promoted.Code, promoted.Body.String())
	}
	approved := addCareerEvidenceForSearch(t, a, careerEvidenceContent{
		Title: "Production incident investigation", Situation: "A customer-facing MySQL service slowed during production.",
		Actions:     "Reviewed query logs and EXPLAIN plans. Recommended an index change.",
		RoleContext: "Support engineer", EmployerContext: "Synthetic Systems", Technologies: "Linux, MySQL, SSH",
		OutcomeState: "unknown",
	}, true)
	draft := addCareerEvidenceForSearch(t, a, careerEvidenceContent{
		Title: "Production incident response draft", Situation: "Production incident investigation for a customer.",
		Actions: "Investigated the incident with MySQL and Linux.", OutcomeState: "not_recorded",
	}, false)
	docs, err := a.careerEvidenceSearchDocuments()
	if err != nil || len(docs) != 1 || docs[0].evidenceID != approved.ID || docs[0].evidenceID == draft.ID {
		t.Fatalf("draft evidence entered automatic retrieval: docs=%+v err=%v", docs, err)
	}
	question := preparedQuestion{Key: key, Field: field, Editable: true}
	suggestions, err := a.careerEvidenceSuggestionsForQuestions([]preparedQuestion{question})
	if err != nil || len(suggestions[key]) != 1 || suggestions[key][0].Evidence.ID != approved.ID ||
		suggestions[key][0].Evidence.OutcomeState != "unknown" {
		t.Fatalf("reviewed relevant experience missing or outcome changed: %+v %v", suggestions, err)
	}
	unrelated := preparedQuestion{Key: strings.Repeat("c", 64), Editable: true, Field: schema.Field{
		ID: "browser:team-launch", Label: "Describe a team mentorship program with marketing partners", Kind: "textarea",
	}}
	if noMatch, err := a.careerEvidenceSuggestionsForQuestions([]preparedQuestion{unrelated}); err != nil || len(noMatch) != 0 {
		t.Fatalf("unrelated experience surfaced: %+v %v", noMatch, err)
	}

	request := httptest.NewRequest(http.MethodGet, "http://localhost/jobs/1", nil)
	request.Host = "localhost"
	response := httptest.NewRecorder()
	newMux(a).ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, field.Label) ||
		!strings.Contains(body, "Related previous writing") || !strings.Contains(body, "Relevant career experiences") ||
		!strings.Contains(body, approved.Title) || !strings.Contains(body, "Reviewed query logs and EXPLAIN plans") ||
		!strings.Contains(body, "Outcome</strong> · Unknown") || !strings.Contains(body, "Inspect complete experience") ||
		!strings.Contains(body, "/career-evidence/"+strconv.FormatInt(approved.ID, 10)) || strings.Contains(body, draft.Title) {
		t.Fatalf("separate, read-only suggestions did not render with the exact question: %d %s", response.Code, body)
	}
	if strings.Contains(body, ">"+approved.Title+"</textarea>") || strings.Contains(body, ">"+draft.Title+"</textarea>") {
		t.Fatal("Career Evidence was copied into the application answer editor")
	}
	var stored string
	var revision int64
	if err := a.db.QueryRow(`SELECT answer,revision FROM application_answers WHERE job_id=? AND question_key=?`, job.ID, key).Scan(&stored, &revision); err != nil || stored != originalAnswer || revision != 1 {
		t.Fatalf("rendering writing suggestions changed the saved employer answer: %q revision=%d err=%v", stored, revision, err)
	}
	var submissions int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE job_id=?`, job.ID).Scan(&submissions); err != nil || submissions != 0 {
		t.Fatalf("suggestions recorded an application submission: %d %v", submissions, err)
	}
	if a.browsers.state(job.ID).Active {
		t.Fatal("rendering evidence armed an employer browser transfer")
	}
}

func BenchmarkCareerEvidenceSearchThousandDocuments(b *testing.B) {
	records := make([]CareerEvidence, 1000)
	for i := range records {
		if i%25 == 0 {
			records[i] = CareerEvidence{
				ID: int64(i + 1), Title: "Production MySQL incident investigation",
				Situation:    "A customer-facing application slowed during production.",
				Actions:      "Reviewed query logs and EXPLAIN plans, then recommended indexing changes.",
				Technologies: "Linux, MySQL, SSH",
			}
		} else {
			records[i] = CareerEvidence{
				ID: int64(i + 1), Title: "Internal project coordination",
				Situation:    "A small planning group organized a staff event.",
				Actions:      "Scheduled a meeting and shared the agenda.",
				Technologies: "Calendar, email",
			}
		}
	}
	queries := []string{
		"Describe a production incident you investigated for a customer",
		"Explain your MySQL performance investigation and query plan review",
		"Tell us about the actions you took during a Linux service outage",
		"Describe your experience recommending database indexing changes",
		"Share a customer-facing application incident response example",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		docs := make([]careerEvidenceSearchDocument, len(records))
		for j, record := range records {
			docs[j] = newCareerEvidenceSearchDocument(record)
		}
		frequency := careerEvidenceDocumentFrequency(docs)
		for _, query := range queries {
			rankCareerEvidenceDocumentsWithFrequency(query, docs, careerEvidenceRelatedLimit, frequency)
		}
	}
}
