package app

// HandoffPresentation is a single user-facing interpretation of the browser
// and prepared material. Inspection-history status must not override attachment
// outcomes or browser recovery instructions.
type HandoffPresentation struct {
	Title                                        string             `json:"handoff_title"`
	Detail                                       string             `json:"handoff_detail"`
	Recovery                                     bool               `json:"handoff_recovery"`
	Action                                       string             `json:"-"`
	OpenExternally                               bool               `json:"-"`
	IncludeResume, IncludeLetter, IncludeAnswers bool               `json:"-"`
	PendingAnswers                               []preparedQuestion `json:"-"`
	AnswerSummary                                string             `json:"-"`
}

func summarizeHandoff(s browserRunState) HandoffPresentation {
	if !s.Active {
		if s.Phase == "interrupted" {
			return HandoffPresentation{Title: "Application browser closed", Detail: "Reopen the application when ready. Browser-only answers or uploads may be lost; YourMove will not resend a prior attachment automatically.", Recovery: true}
		}
		return HandoffPresentation{Title: "Ready to prepare", Detail: "Review the exact assigned resume, then authorize opening the employer application.", Recovery: false}
	}
	if s.ResumeRetryable {
		return HandoffPresentation{Title: "Resume check needs a retry", Detail: "Nothing was selected or uploaded during this failed preflight. Wait for the form to settle, then retry the check.", Recovery: true}
	}
	switch s.ResumeStatus {
	case "selected":
		return HandoffPresentation{Title: "Resume selected — verify with the employer", Detail: "YourMove selected the approved file. Confirm the employer displays or accepts the attachment; finish and submit the form yourself.", Recovery: false}
	case "attention":
		return HandoffPresentation{Title: "Resume needs your attention", Detail: "Check the existing attachment in " + engineDisplay(s.Engine) + ". If its outcome is unknown, no automatic reupload will occur.", Recovery: true}
	case "checking":
		return HandoffPresentation{Title: "Checking resume control", Detail: "The browser is open. YourMove is checking the approved file destination; do not close or refresh it.", Recovery: false}
	case "waiting":
		return HandoffPresentation{Title: "Waiting for resume field", Detail: "The application is open. If the employer has additional steps, advance them yourself; YourMove keeps watching.", Recovery: false}
	}
	if s.Phase == "attention" {
		return HandoffPresentation{Title: "Employer page needs attention", Detail: "Return to the current application's form or reopen it deliberately. YourMove cannot submit it.", Recovery: true}
	}
	return HandoffPresentation{Title: "Browser open — manual preparation", Detail: "Complete the employer's application in " + engineDisplay(s.Engine) + ". YourMove is observing only; no resume upload is authorized in this session.", Recovery: false}
}
