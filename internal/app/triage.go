package app

import (
	"sort"
	"strconv"
)

// NextAction is an advisory navigation cue, never an application status or an
// assertion that a resume was uploaded or an employer accepted a submission.
type NextAction struct {
	JobID   int64
	Title   string
	Company string
	Label   string
	Reason  string
	Href    string
}

func (j Job) ActionLabel() string {
	switch {
	case j.SubmissionCount > 0 || j.Stage == StageApplied:
		return "View application history"
	case j.Stage == StageClosed:
		return "Closed"
	case j.SelectedResumeID == 0:
		return "Choose a resume"
	case j.Stage == StageReady:
		return "Open application workspace"
	default:
		return "Review and prepare"
	}
}

func (j Job) ActionHref() string {
	base := "/jobs/" + formatJobID(j.ID)
	switch {
	case j.SubmissionCount > 0 || j.Stage == StageApplied:
		return base + "#history"
	case j.Stage == StageClosed:
		return base + "#overview"
	case j.SelectedResumeID == 0 || j.Stage == StageReady:
		return base + "#prepare"
	default:
		return base + "#overview"
	}
}

func (j Job) ActionReason() string {
	switch {
	case j.SubmissionCount > 0 || j.Stage == StageApplied:
		return "This opportunity is marked applied. Review its recorded history."

	case j.Stage == StageClosed:
		return "This opportunity is closed in YourMove. Change its stage to resume preparation."

	case j.SelectedResumeID == 0:
		return "No resume selected. Choose a document or continue manually."

	case j.Stage == StageReady:
		return "You marked this job ready. Review the employer application before submitting."

	default:
		return "Review the saved posting and decide whether to continue."
	}
}

func formatJobID(id int64) string { return strconv.FormatInt(id, 10) }

// recommendNext uses only verified local decisions. Priority wins; then ready
// jobs, then jobs with a chosen resume, then the newest opportunity.
func recommendNext(jobs []Job) *NextAction {
	var candidates []Job
	for _, job := range jobs {
		if job.Stage == StageClosed || job.Stage == StageApplied || job.SubmissionCount > 0 {
			continue
		}
		candidates = append(candidates, job)
	}
	if len(candidates) == 0 {
		return nil
	}
	scorePriority := func(p string) int {
		if p == PriorityHigh {
			return 0
		}
		if p == PriorityLow {
			return 2
		}
		return 1
	}
	sort.Slice(candidates, func(i, k int) bool {
		a, b := candidates[i], candidates[k]
		if x, y := scorePriority(a.Priority), scorePriority(b.Priority); x != y {
			return x < y
		}
		if a.Stage != b.Stage {
			return a.Stage == StageReady
		}
		if (a.SelectedResumeID > 0) != (b.SelectedResumeID > 0) {
			return a.SelectedResumeID > 0
		}
		return a.ID > b.ID
	})
	best := candidates[0]
	title := best.Title
	if title == "" {
		title = "Untitled opportunity"
	}
	company := best.Company
	if company == "" {
		company = best.Host
	}
	return &NextAction{JobID: best.ID, Title: title, Company: company, Label: best.ActionLabel(), Reason: best.ActionReason(), Href: best.ActionHref()}
}
