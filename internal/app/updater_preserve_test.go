package app

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestBackgroundCheckPreservesUserPhases(t *testing.T) {
	for _, phase := range []string{"checking", "installing", "ready", "error"} {
		a, _ := testWorkspace(t)
		a.SetUpdateController(UpdateController{
			Check: func(ctx context.Context) (UpdateCheckResult, error) {
				return UpdateCheckResult{Available: true, LatestVersion: "0.2.0"}, nil
			},
		})
		a.setUpdatePhase(phase, &UpdateCheckResult{Available: phase == "ready", LatestVersion: "0.1.0"}, "prior error")
		if phase == "error" {
			a.setUpdatePhase("error", nil, "prior error")
		}
		if phase == "checking" {
			a.setUpdatePhase("checking", nil, "")
		}
		a.BackgroundUpdateCheck(context.Background())
		got, pending, errMsg, _, _ := a.updateSnapshot()
		if got != phase {
			t.Fatalf("phase %q overwritten to %q (pending=%+v err=%q)", phase, got, pending, errMsg)
		}
	}
	// Idle and up-to-date may surface availability.
	for _, phase := range []string{"idle", "up-to-date", "available"} {
		a, _ := testWorkspace(t)
		a.SetUpdateController(UpdateController{
			Check: func(ctx context.Context) (UpdateCheckResult, error) {
				return UpdateCheckResult{Available: true, LatestVersion: "0.2.0"}, nil
			},
		})
		if phase != "idle" {
			a.setUpdatePhase(phase, &UpdateCheckResult{Available: phase == "available", LatestVersion: "0.1.0"}, "")
		}
		a.BackgroundUpdateCheck(context.Background())
		got, pending, _, _, _ := a.updateSnapshot()
		if got != "available" || pending == nil || pending.LatestVersion != "0.2.0" {
			t.Fatalf("phase %q should surface available, got %q %+v", phase, got, pending)
		}
	}
}

func TestUpdateUserErrorRedactsPathsAndSensitiveURLs(t *testing.T) {
	leaky := errors.New("open /Users/alice/Library/Application Support/YourMove/staging/update.dmg: permission denied downloading https://updates.example.test/manifest?token=secret-token-123")
	msg := updateUserError(leaky)
	if strings.Contains(msg, "/Users/alice") || strings.Contains(msg, "secret-token-123") || strings.Contains(msg, "update.dmg:") && strings.Contains(msg, "/") {
		t.Fatalf("leaked path or token: %q", msg)
	}
	if strings.Contains(msg, "jobs.db") {
		t.Fatalf("leaked workspace name: %q", msg)
	}
	if !strings.Contains(msg, "workspace are unchanged") || !strings.Contains(msg, "manual DMG") {
		t.Fatalf("lost recovery guidance: %q", msg)
	}
	// Useful troubleshooting survives.
	sig := updateUserError(errors.New("updater: ed25519 signature did not verify"))
	if !strings.Contains(sig, "signature") {
		t.Fatalf("lost signature context: %q", sig)
	}
	netErr := updateUserError(errors.New("dial tcp 127.0.0.1:9: connect: connection refused"))
	if !strings.Contains(netErr, "connection refused") {
		t.Fatalf("lost network context: %q", netErr)
	}
}
