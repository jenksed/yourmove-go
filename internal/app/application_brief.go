package app

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jenksed/yourmove-go/internal/application/browser"
)

const (
	briefFormatApplication = "brief"
	briefFormatQuestions   = "questions"
	briefFormatJobDetails  = "details"
	briefScopeUnanswered   = "unanswered"
	briefScopeAll          = "all"
)

type briefQuestion struct {
	Label          string
	Required       bool
	Options        []string
	HasLocalAnswer bool
}

type applicationQuestionSnapshot struct {
	Available         bool
	Source            string
	Completeness      string
	Partial           bool
	Questions         []briefQuestion
	NeedsAnswerCount  int
	PreparedCount     int
	ProfileCount      int
	EmployerOnlyCount int
	ManualReviewCount int
	Anchor            string
}

type applicationReadiness struct {
	ResumeReady        bool
	ResumeUnavailable  bool
	ProfileApproved    bool
	QuestionsAvailable bool
	NeedsAnswerCount   int
	PreparedCount      int
	ProfileCount       int
	EmployerOnlyCount  int
	ManualReviewCount  int
	ManualWorkCount    int
	QuestionSource     string
	QuestionPartial    bool
	QuestionAnchor     string
	CoverLetterSaved   bool
	BrowserStatus      string
	BrowserDetail      string
}

func chooseApplicationQuestionSnapshot(job Job, state browserRunState, live questionPreparation, early earlyQuestionPreparation) applicationQuestionSnapshot {
	// A stored browser inspection is current only while the managed session is
	// active and points at that exact saved observation. After a restart or
	// interruption, use one early discovery inventory instead of merging it with
	// an old live form.
	if state.Active && state.Destination == job.URL && state.LatestID > 0 &&
		live.InspectionID == state.LatestID && live.InspectionSource == "browser" &&
		browser.OwnsPostingURL(job.URL, live.InspectionURL) {
		out := applicationQuestionSnapshot{
			Available: true, Source: questionSourceLabel(live.InspectionSource),
			Completeness: live.InspectionCompleteness, Anchor: "#application-questions",
		}
		questions := live.ActionableQuestions()
		for _, q := range questions {
			out.Questions = append(out.Questions, briefQuestionFromLive(q))
		}
		out.ProfileCount = len(live.ProfileQuestions())
		out.EmployerOnlyCount = len(live.EmployerSiteQuestions())
		out.ManualReviewCount = len(live.ManualReviewQuestions())
		finishQuestionSnapshot(&out)
		return out
	}

	if item := early.Item; item != nil && browser.OwnsPostingURL(job.URL, item.ApplicationURL) {
		out := applicationQuestionSnapshot{
			Available: true, Source: questionSourceLabel(item.Source),
			Completeness: item.Completeness, Anchor: "#early-application-questions",
		}
		for _, q := range early.ActionableQuestions() {
			out.Questions = append(out.Questions, briefQuestionFromEarly(q))
		}
		out.ProfileCount = len(early.ProfileQuestions())
		out.EmployerOnlyCount = len(early.EmployerSiteQuestions())
		out.ManualReviewCount = len(early.ManualReviewQuestions())
		finishQuestionSnapshot(&out)
		return out
	}
	return applicationQuestionSnapshot{}
}

// writingAnswerContext keeps the answer activity on the context the user just
// saved or selected. Otherwise it follows the same current-live-versus-early
// snapshot choice used by Prepare readiness. This only chooses presentation;
// question identity and save/transfer checks remain server-owned.
func writingAnswerContext(r *http.Request, snapshot applicationQuestionSnapshot) string {
	if r != nil {
		query := r.URL.Query()
		if query.Has("early_question") {
			return "early"
		}
		if query.Has("question") {
			return "live"
		}
	}
	if snapshot.Anchor == "#application-questions" {
		return "live"
	}
	return "early"
}

func briefQuestionFromLive(q preparedQuestion) briefQuestion {
	return briefQuestion{
		Label: q.Field.Label, Required: q.Field.Required,
		Options:        safePublishedOptions(q.Field.Kind, q.Field.Options),
		HasLocalAnswer: q.Revision > 0 || strings.TrimSpace(q.Value) != "",
	}
}

func briefQuestionFromEarly(q earlyApplicationQuestion) briefQuestion {
	return briefQuestion{
		Label: q.Field.Label, Required: q.Field.Required,
		Options:        safePublishedOptions(q.Field.Kind, q.Field.Options),
		HasLocalAnswer: q.Revision > 0 || strings.TrimSpace(q.Value) != "",
	}
}

func safePublishedOptions(kind string, options []string) []string {
	switch kind {
	case "select", "radio", "multi_value_single_select":
		return append([]string(nil), options...)
	default:
		return nil
	}
}

func finishQuestionSnapshot(snapshot *applicationQuestionSnapshot) {
	snapshot.Partial = snapshot.Completeness != "rendered_complete"
	for _, question := range snapshot.Questions {
		if question.HasLocalAnswer {
			snapshot.PreparedCount++
		} else {
			snapshot.NeedsAnswerCount++
		}
	}
}

func questionSourceLabel(source string) string {
	switch source {
	case "browser":
		return "Current browser-observed form"
	case "preview":
		return "Read-only application preview"
	default:
		return "Published question definitions"
	}
}

func prepareReadiness(resumeValid bool, profile CandidateProfile, state browserRunState, questions applicationQuestionSnapshot, letter CoverLetter) applicationReadiness {
	out := applicationReadiness{
		ResumeReady:        resumeValid,
		ResumeUnavailable:  !resumeValid,
		ProfileApproved:    profile.Approved,
		QuestionsAvailable: questions.Available,
		NeedsAnswerCount:   questions.NeedsAnswerCount,
		PreparedCount:      questions.PreparedCount,
		ProfileCount:       questions.ProfileCount,
		EmployerOnlyCount:  questions.EmployerOnlyCount,
		ManualReviewCount:  questions.ManualReviewCount,
		ManualWorkCount:    questions.EmployerOnlyCount + questions.ManualReviewCount,
		QuestionSource:     questions.Source,
		QuestionPartial:    questions.Partial,
		QuestionAnchor:     questions.Anchor,
		CoverLetterSaved:   letter.ID > 0,
	}
	switch {
	case state.Active:
		out.BrowserStatus = "Open in this session"
	case state.Phase == "interrupted":
		out.BrowserStatus = "Interrupted"
		out.BrowserDetail = state.Message
	case state.Phase == "opening":
		out.BrowserStatus = "Opening"
		out.BrowserDetail = state.Message
	case state.Phase == "attention":
		out.BrowserStatus = "Needs attention"
		out.BrowserDetail = state.Message
	default:
		out.BrowserStatus = "Not open"
	}
	return out
}

func normalizeBriefScope(scope string) (string, error) {
	if scope == "" || scope == briefScopeUnanswered {
		return briefScopeUnanswered, nil
	}
	if scope == briefScopeAll {
		return briefScopeAll, nil
	}
	return "", fmt.Errorf("unknown question scope")
}

func normalizeBriefFormat(format string) (string, error) {
	if format == "" || format == briefFormatApplication {
		return briefFormatApplication, nil
	}
	switch format {
	case briefFormatQuestions, briefFormatJobDetails:
		return format, nil
	default:
		return "", fmt.Errorf("unknown brief format")
	}
}

// formatApplicationBrief is deterministic and read-only. The snapshot carries
// question wording and metadata only; saved answer contents never enter this
// formatter.
func formatApplicationBrief(job Job, questions applicationQuestionSnapshot, format, scope string) string {
	var out strings.Builder
	switch format {
	case briefFormatJobDetails:
		out.WriteString("JOB DETAILS\n\n")
		writeJobDetails(&out, job)
		out.WriteString("\n\nEND OF JOB DETAILS\n")
	case briefFormatQuestions:
		writeQuestionSection(&out, questions, scope)
		out.WriteString("\nEND OF QUESTIONS\n")
	default:
		out.WriteString("JOB APPLICATION BRIEF\n\n")
		writeJobDetails(&out, job)
		out.WriteString("\n\n")
		writeQuestionSection(&out, questions, scope)
		out.WriteString("\nEND OF BRIEF\n")
	}
	return out.String()
}

func writeJobDetails(out *strings.Builder, job Job) {
	company := strings.TrimSpace(job.Company)
	if company == "" {
		company = "Not specified"
	}
	title := strings.TrimSpace(job.Title)
	if title == "" {
		title = "Not specified"
	}
	compensation := strings.TrimSpace(job.SalaryText)
	if compensation == "" {
		compensation = "Not specified"
	}
	posting := strings.TrimSpace(job.URL)
	if posting == "" {
		posting = "Not specified"
	}
	description := job.Description
	if strings.TrimSpace(description) == "" {
		description = "Not available in the saved job record."
	}
	fmt.Fprintf(out, "Company: %s\nPosition: %s\nCompensation: %s\nCompensation source: %s\nPosting: %s\n\nJOB DESCRIPTION\n\n%s",
		company, title, compensation, job.SalaryOriginLabel(), posting, description)
}

func writeQuestionSection(out *strings.Builder, snapshot applicationQuestionSnapshot, scope string) {
	if scope == briefScopeAll {
		out.WriteString("ACTIONABLE APPLICATION QUESTIONS (WORDING ONLY)\n\n")
	} else {
		out.WriteString("QUESTIONS NEEDING MY INPUT\n\n")
	}
	if !snapshot.Available {
		out.WriteString("Application questions have not been discovered yet.\n")
		return
	}
	fmt.Fprintf(out, "Source: %s.\n", snapshot.Source)
	if snapshot.Partial {
		out.WriteString("Inspection is partial; later or conditional questions may not be represented.\n")
	}
	if scope == briefScopeAll {
		out.WriteString("Question wording only; saved answers are not included.\n")
	}
	var selected []briefQuestion
	for _, question := range snapshot.Questions {
		if scope == briefScopeAll || !question.HasLocalAnswer {
			selected = append(selected, question)
		}
	}
	if len(selected) == 0 {
		if scope == briefScopeUnanswered && snapshot.PreparedCount > 0 {
			out.WriteString("No unanswered actionable questions are available in this inspection.\n")
		} else {
			out.WriteString("No supported questions requiring applicant input were identified in this inspection.\n")
		}
		return
	}
	if scope == briefScopeUnanswered {
		out.WriteByte('\n')
	} else {
		out.WriteByte('\n')
	}
	for index, question := range selected {
		fmt.Fprintf(out, "%d. %s\n   %s\n", index+1, question.Label, requiredLabel(question.Required))
		if len(question.Options) > 0 {
			out.WriteString("   Published options:\n")
			for _, option := range question.Options {
				fmt.Fprintf(out, "   - %s\n", option)
			}
		}
		if index+1 < len(selected) {
			out.WriteByte('\n')
		}
	}
}

func requiredLabel(required bool) string {
	if required {
		return "Required"
	}
	return "Optional"
}

func (a *App) applicationBriefHandler(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	job, err := a.jobByID(id)
	if errorsIsNoRows(err) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Unable to load saved job details", http.StatusInternalServerError)
		return
	}
	format, err := normalizeBriefFormat(r.URL.Query().Get("format"))
	if err != nil {
		http.Error(w, "Invalid brief format", http.StatusBadRequest)
		return
	}
	scope, err := normalizeBriefScope(r.URL.Query().Get("scope"))
	if err != nil {
		http.Error(w, "Invalid question scope", http.StatusBadRequest)
		return
	}
	profile, err := a.candidateProfile()
	if err != nil {
		http.Error(w, "Unable to load application preparation", http.StatusInternalServerError)
		return
	}
	live, err := a.prepareQuestions(job, profile)
	if err != nil {
		http.Error(w, "Unable to load inspected application questions", http.StatusInternalServerError)
		return
	}
	early, err := a.prepareEarlyQuestions(job, nil)
	if err != nil {
		http.Error(w, "Unable to load discovered application questions", http.StatusInternalServerError)
		return
	}
	questions := chooseApplicationQuestionSnapshot(job, a.browsers.state(id), live, early)
	brief := formatApplicationBrief(job, questions, format, scope)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "inline; filename=job-application-brief.txt")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write([]byte(brief))
}

func errorsIsNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }
