package app

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	browser "github.com/jenksed/yourmove-go/internal/application/browser"
)

func writeTransferResult(w http.ResponseWriter, r *http.Request, status int, detail string) {
	if status == http.StatusSeeOther {
		http.Redirect(w, r, detail, status)
		return
	}
	http.Error(w, detail, status)
}

// continueApplication composes independently guarded commands. The posted flags
// are the displayed scope, never an instruction to discover and send more data.
// A failed item does not roll back or prevent another eligible item.
func (a *App) continueApplication(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 8<<10) {
		return
	}
	job, err := a.jobByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if r.PostForm.Get("destination") != job.URL {
		http.Error(w, "The employer destination changed. Review Prepare again.", 409)
		return
	}
	var notices []string
	included := false
	for _, item := range []struct {
		flag, label string
		run         func(*http.Request, int64) (int, string)
	}{
		{"include_resume", "Resume and contacts", a.resumeHandoffCommand},
		{"include_letter", "Cover letter", a.coverLetterCommand},
		{"include_answers", "Saved answers", a.answerTransferCommand},
	} {
		if r.PostForm.Get(item.flag) != "yes" {
			continue
		}
		included = true
		status, detail := item.run(r, id)
		if status != http.StatusSeeOther {
			notices = append(notices, item.label+": "+detail)
		}
	}
	if !included {
		http.Error(w, "No transfer material was selected. Review Prepare again.", 422)
		return
	}
	target := fmt.Sprintf("/jobs/%d", id)
	query := url.Values{}
	if draft := r.PostForm.Get("draft_id"); r.PostForm.Get("include_letter") == "yes" && draft != "" {
		query.Set("draft", draft)
	}
	if len(notices) > 0 {
		query.Set("transfer_notice", strings.Join(notices, " "))
	}
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	http.Redirect(w, r, target+"#prepare", http.StatusSeeOther)
}

func pendingAnswers(ready, attempted []preparedQuestion, contactsHandled bool) []preparedQuestion {
	var pending []preparedQuestion
	for _, q := range ready {
		if contactsHandled && q.profileContact() {
			continue
		}
		seen := false
		for _, old := range attempted {
			if old.Key == q.Key && old.Value == q.Value {
				seen = true
				break
			}
		}
		if !seen {
			pending = append(pending, q)
		}
	}
	return pending
}

// summarizePreparation is read-only. Only command handlers confer authority.
func summarizePreparation(w jobWorkspace) HandoffPresentation {
	s := w.BrowserState
	out := summarizeHandoff(s)
	if w.Stage == StageClosed || w.Stage == StageApplied || w.SubmissionCount > 0 {
		return out
	}
	if w.URL != "" && !browser.OwnsPostingURL(w.URL, w.URL) {
		out.Title = "Apply on the employer website"
		out.Detail = "Automatic filling is not available for this job site yet. Download your selected resume above and any saved letter below, then open the application to attach them and complete the form."
		out.OpenExternally = true
		return out
	}
	if w.OtherBrowser != nil {
		out.Title = "Another application browser is open"
		out.Detail = "Return to that application to preserve its browser work. If it is complete, record completion and use Next job; otherwise close its Chromium window deliberately before opening another job."
		out.Recovery = false
		return out
	}
	if s.Active && s.Destination != w.URL {
		out.Title = "Application destination changed"
		out.Detail = "Inspect the current browser before deliberately reopening the new destination. Browser-only work may be lost."
		out.Recovery = true
		return out
	}
	if s.Phase == "opening" {
		out.Title = "Opening application"
		out.Detail = "Wait for the managed browser to open."
		return out
	}
	// Inspection warnings do not revoke independently guarded operations. Each
	// transfer still checks the live destination and control before writing.
	if !s.Active && s.Phase == "interrupted" {
		return out
	}
	out.IncludeResume = w.Handoff.Allowed && s.ResumeStatus == ""
	out.IncludeLetter = w.Letter.ID > 0 && (s.LetterStatus == "" || s.LetterStatus == "review")
	out.PendingAnswers = pendingAnswers(w.Questions.ReadyAnswers(), s.AnswerAttempted, s.ContactStatus != "" || (out.IncludeResume && w.Profile.Approved))
	out.IncludeAnswers = len(out.PendingAnswers) > 0 && s.AnswerStatus != "waiting" && s.AnswerStatus != "checking"
	if !s.Active && !out.IncludeResume {
		out.IncludeLetter = false
		out.IncludeAnswers = false
	}
	switch {
	case out.IncludeResume:
		out.Action = "Fill application"
		out.Title = "Ready to fill application"
		out.Detail = "Review the material below, then fill the supported controls in the employer application."
	case out.IncludeLetter || out.IncludeAnswers:
		out.Action = "Continue application"
	}
	if out.Action != "" && !out.IncludeResume {
		out.Title = "New material ready"
		out.Detail = "Continue in the same employer browser with the material below. Completed contacts and documents stay in place."
	}
	if out.Action == "" {
		switch {
		case s.LetterStatus == "attention" || s.LetterStatus == "review":
			out.Title = "Cover letter needs inspection"
			out.Detail = "Review the letter result and existing employer attachment in Chromium. An uncertain upload will not be retried."
		case s.AnswerStatus == "complete" || s.LetterStatus == "complete":
			out.Title = "Supported transfers completed"
			out.Detail = "Review each result, complete any manual fields, and submit personally in the same employer browser."
		}
	}
	if out.Action == "" && (s.LetterStatus == "checking" || s.LetterStatus == "waiting" || s.AnswerStatus == "waiting" || s.AnswerStatus == "checking") {
		out.Title = "Transferring application material"
		out.Detail = "The current browser remains open. Each item is checked independently; review its result below."
	}
	return out
}

// The rendered hash covers exactly the pending answers shown beside the action.
func (p HandoffPresentation) AnswerReviewHash() string {
	return (questionPreparation{Questions: p.PendingAnswers}).ReviewHash()
}

// Moving on is explicit: recording completion alone keeps the employer browser
// open for review. Only this action retires the completed job's own session.
func (a *App) nextApplication(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 2<<10) {
		return
	}
	job, err := a.jobByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if job.Stage != StageApplied || job.SubmissionCount == 0 {
		http.Error(w, "Record the completed application before moving on. The current employer browser remains open.", 409)
		return
	}
	jobs, err := a.allJobs()
	if err != nil {
		http.Error(w, "Unable to load the next opportunity. Return to Opportunities.", 500)
		return
	}
	if err = a.browsers.close(id); err != nil {
		http.Error(w, "Could not close the completed application browser. Close it in Chromium, then return to Opportunities.", 500)
		return
	}
	target := "/jobs"
	if next := recommendNext(jobs); next != nil {
		target = fmt.Sprintf("/jobs/%d#prepare", next.JobID)
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
