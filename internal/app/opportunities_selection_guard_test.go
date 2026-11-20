package app

import (
	"context"
	"errors"
	"testing"
)

// Focused regression for independent review:
// (1) bulk attention moves are atomic and refuse terminal selections;
// (2) closed History rows without submissions stay deletable, submitted rows do not.
func TestSelectionGuardTerminalMoveRefusesBatch(t *testing.T) {
	a, _ := testWorkspace(t)
	movable, _, err := a.savePostingURLWithFocus(context.Background(), "https://example.com/guard-movable", "", true)
	if err != nil {
		t.Fatal(err)
	}
	terminal, _, err := a.savePostingURLWithFocus(context.Background(), "https://example.com/guard-terminal", "", true)
	if err != nil {
		t.Fatal(err)
	}
	closing, err := a.jobByID(terminal)
	if err != nil {
		t.Fatal(err)
	}
	closing.Stage = StageClosed
	if err := a.updateJob(closing); err != nil {
		t.Fatal(err)
	}
	if _, err := a.moveJobsAttention([]int64{movable, terminal}, "saved", "2026-11-18T00:00:00Z"); !errors.Is(err, ErrAttentionTerminal) {
		t.Fatalf("mixed batch should refuse as terminal, got %v", err)
	}
	kept, err := a.jobByID(movable)
	if err != nil {
		t.Fatal(err)
	}
	if kept.Attention() != "focus" {
		t.Fatalf("atomic move must leave eligible job untouched, got %q", kept.Attention())
	}
}

func TestSelectionGuardClosedWithoutSubmissionsDeletable(t *testing.T) {
	a, _ := testWorkspace(t)
	closedID, _, err := a.savePostingURLWithFocus(context.Background(), "https://example.com/guard-closed-deletable", "", true)
	if err != nil {
		t.Fatal(err)
	}
	closing, err := a.jobByID(closedID)
	if err != nil {
		t.Fatal(err)
	}
	closing.Stage = StageClosed
	if err := a.updateJob(closing); err != nil {
		t.Fatal(err)
	}
	if _, err := a.deleteJobs([]int64{closedID}, false); err != nil {
		t.Fatalf("closed job without submissions should stay bulk-deletable, got %v", err)
	}
	submittedID, _, err := a.savePostingURLWithFocus(context.Background(), "https://example.com/guard-submitted", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`INSERT INTO submissions(job_id, submitted_at, resume_name, resume_sha256, token, notes) VALUES(?, 'now', 'r', 's', 'guard-token-1', 'x')`, submittedID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.deleteJobs([]int64{submittedID}, false); !errors.Is(err, ErrJobHasHistory) {
		t.Fatalf("submitted job should refuse bulk deletion, got %v", err)
	}
}
