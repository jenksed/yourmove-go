package main

// HandoffPresentation is a single user-facing interpretation of the browser
// and resume state. Inspection-history status must not override attachment
// outcomes or browser recovery instructions.
type HandoffPresentation struct {
	Title    string `json:"handoff_title"`
	Detail   string `json:"handoff_detail"`
	Recovery bool   `json:"handoff_recovery"`
}

func summarizeHandoff(s browserRunState) HandoffPresentation {
	if !s.Active {
		if s.Phase == "interrupted" {
			return HandoffPresentation{"Application browser closed", "Reopen the application when ready. Browser-only answers or uploads may be lost; YourMove will not resend a prior attachment automatically.", true}
		}
		return HandoffPresentation{"Ready to prepare", "Review the exact assigned resume, then authorize opening the employer application.", false}
	}
	if s.ResumeRetryable {
		return HandoffPresentation{"Resume check needs a retry", "Nothing was selected or uploaded during this failed preflight. Wait for the form to settle, then retry the check.", true}
	}
	switch s.ResumeStatus {
	case "selected":
		return HandoffPresentation{"Resume selected — verify in Ashby", "YourMove selected the approved file. Confirm the employer displays or accepts the attachment; finish and submit the form yourself.", false}
	case "attention":
		return HandoffPresentation{"Resume needs your attention", "Check the existing attachment in Chromium. If its outcome is unknown, no automatic reupload will occur.", true}
	case "checking":
		return HandoffPresentation{"Checking resume control", "The browser is open. YourMove is checking the approved file destination; do not close or refresh it.", false}
	case "waiting":
		return HandoffPresentation{"Waiting for resume field", "The application is open. If Ashby has additional steps, advance them yourself; YourMove keeps watching.", false}
	}
	if s.Phase == "attention" {
		return HandoffPresentation{"Employer page needs attention", "Return to the current application's form or reopen it deliberately. YourMove cannot submit it.", true}
	}
	return HandoffPresentation{"Browser open — manual preparation", "Complete the employer's application in Chromium. YourMove is observing only; no resume upload is authorized in this session.", false}
}
