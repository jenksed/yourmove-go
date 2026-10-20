// Package flow classifies navigation before a browser controller takes action.
// It intentionally does not know how to click an employer button.
package flow

import "strings"

type Transition string

const (
	NewStep        Transition = "step"
	Conditional    Transition = "conditional"
	Authentication Transition = "authentication"
	Submission     Transition = "submission"
	External       Transition = "external"
	Unknown        Transition = "unknown"
)

type Decision struct {
	Kind                  Transition
	ContinueAutomatically bool
	Reason                string
}

// Classify is conservative: labels alone cannot authorize clicking any control.
// observedMethod/destination must be established by actual read-only inspection.
func Classify(label, method, destination string, changesVisibleFields bool) Decision {
	label = strings.ToLower(strings.TrimSpace(label))
	method = strings.ToUpper(strings.TrimSpace(method))
	if method != "" && method != "GET" {
		return Decision{Submission, false, "This action may transmit application data; user review required."}
	}
	if strings.Contains(label, "submit") || strings.Contains(label, "send application") || strings.Contains(label, "apply now") {
		return Decision{Submission, false, "Possible final application submission."}
	}
	if strings.Contains(label, "sign in") || strings.Contains(label, "log in") || strings.Contains(label, "create account") {
		return Decision{Authentication, false, "Authentication is handled by the applicant."}
	}
	if changesVisibleFields {
		return Decision{Conditional, false, "Dependent questions must be re-inspected after the user changes a value."}
	}
	if destination != "" {
		return Decision{External, false, "Navigation target requires ownership and same-job verification."}
	}
	if strings.Contains(label, "next") || strings.Contains(label, "continue") {
		return Decision{NewStep, false, "A Next/Continue label alone is not evidence of a safe step."}
	}
	return Decision{Unknown, false, "Unclassified action: preserve the current form and ask the applicant."}
}
