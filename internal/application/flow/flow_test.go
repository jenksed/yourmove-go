package flow

import "testing"

func TestNavigationNeverAutoAdvances(t *testing.T) {
	tests := []struct {
		label, method, dest string
		dependent           bool
		kind                Transition
	}{
		{"Continue", "POST", "", false, Submission}, {"Submit Application", "", "", false, Submission}, {"Next", "GET", "", false, NewStep}, {"Continue", "", "", true, Conditional}, {"Sign In", "GET", "", false, Authentication}, {"Apply", "GET", "https://example.com/apply", false, External},
	}
	for _, tc := range tests {
		r := Classify(tc.label, tc.method, tc.dest, tc.dependent)
		if r.ContinueAutomatically || r.Kind != tc.kind {
			t.Errorf("%+v: %+v", tc, r)
		}
	}
}
