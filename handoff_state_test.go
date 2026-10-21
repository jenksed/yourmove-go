package main

import "testing"

func TestHandoffPresentationPrioritizesResumeAndRecovery(t *testing.T) {
	for _, tc := range []struct {
		name     string
		state    browserRunState
		want     string
		recovery bool
	}{
		{"idle", browserRunState{}, "Ready to prepare", false},
		{"lost tab", browserRunState{Phase: "interrupted"}, "Application browser closed", true},
		{"selected even if inspection failed", browserRunState{Active: true, Phase: "attention", ResumeStatus: "selected"}, "Resume selected — verify with the employer", false},
		{"uncertain upload", browserRunState{Active: true, ResumeStatus: "attention"}, "Resume needs your attention", true},
		{"preflight", browserRunState{Active: true, ResumeStatus: "attention", ResumeRetryable: true}, "Resume check needs a retry", true},
		{"waiting", browserRunState{Active: true, ResumeStatus: "waiting"}, "Waiting for resume field", false},
		{"read only", browserRunState{Active: true, Phase: "watching"}, "Browser open — manual preparation", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := summarizeHandoff(tc.state)
			if got.Title != tc.want || got.Recovery != tc.recovery {
				t.Fatalf("got %+v, want %q recovery=%t", got, tc.want, tc.recovery)
			}
		})
	}
}
