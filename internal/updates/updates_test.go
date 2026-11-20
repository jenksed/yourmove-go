package updates

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestManagerUnavailableWithoutController(t *testing.T) {
	m := NewManager("0.2.0")
	phase, pending, _, _, enabled := m.Snapshot()
	if enabled || phase != "unavailable" || pending != nil {
		t.Fatalf("unconfigured snapshot = phase %q pending %+v enabled %v, want unavailable/nil/false", phase, pending, enabled)
	}
	if err := m.RunCheck(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("RunCheck without controller = %v, want ErrUnavailable", err)
	}
	if err := m.RunInstall(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("RunInstall without controller = %v, want ErrUnavailable", err)
	}
	if err := m.RunRestart(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("RunRestart without controller = %v, want ErrUnavailable", err)
	}
}

func TestManagerCheckInstallRestartFlow(t *testing.T) {
	m := NewManager("0.1.9")
	checks, installs, restarts := 0, 0, 0
	m.SetController(Controller{
		Check: func(ctx context.Context) (CheckResult, error) {
			checks++
			return CheckResult{Available: true, CurrentVersion: "0.1.9", LatestVersion: "0.2.0", Name: "Test release", Notes: "Fixes and polish.", Size: 123}, nil
		},
		Install: func(ctx context.Context) error { installs++; return nil },
		Restart: func(ctx context.Context) error { restarts++; return nil },
	})
	if err := m.RunCheck(context.Background()); err != nil {
		t.Fatal(err)
	}
	phase, pending, _, _, enabled := m.Snapshot()
	if !enabled || phase != "available" || pending == nil || pending.LatestVersion != "0.2.0" {
		t.Fatalf("after check phase=%q pending=%+v enabled=%v", phase, pending, enabled)
	}
	if checks != 1 {
		t.Fatalf("checks = %d, want 1", checks)
	}
	if err := m.RunInstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	phase, _, _, _, _ = m.Snapshot()
	if phase != "ready" || installs != 1 {
		t.Fatalf("after install phase=%q installs=%d", phase, installs)
	}
	if err := m.RunRestart(context.Background()); err != nil {
		t.Fatal(err)
	}
	if restarts != 1 {
		t.Fatalf("restarts = %d, want 1", restarts)
	}
}

func TestManagerUpToDateFillsInstalledVersion(t *testing.T) {
	m := NewManager("0.2.0")
	m.SetController(Controller{
		Check: func(ctx context.Context) (CheckResult, error) {
			return CheckResult{Available: false}, nil
		},
	})
	if err := m.RunCheck(context.Background()); err != nil {
		t.Fatal(err)
	}
	phase, pending, _, _, _ := m.Snapshot()
	if phase != "up-to-date" || pending == nil || pending.CurrentVersion != "0.2.0" {
		t.Fatalf("phase=%q pending=%+v, want up-to-date with installed version", phase, pending)
	}
}

func TestManagerCheckFailureKeepsRecoveryGuidance(t *testing.T) {
	m := NewManager("0.2.0")
	m.SetController(Controller{
		Check: func(ctx context.Context) (CheckResult, error) {
			return CheckResult{}, errors.New("dial tcp 127.0.0.1:9: connect: connection refused")
		},
	})
	if err := m.RunCheck(context.Background()); err == nil {
		t.Fatal("expected check error")
	}
	phase, _, errMsg, _, _ := m.Snapshot()
	if phase != "error" {
		t.Fatalf("phase = %q, want error", phase)
	}
	if errMsg == "" || !strings.Contains(errMsg, "workspace are unchanged") {
		t.Fatalf("error message missing recovery guidance: %q", errMsg)
	}
}

func TestManagerBackgroundPreservesUserPhases(t *testing.T) {
	for _, phase := range []string{"checking", "installing", "ready", "error"} {
		m := NewManager("0.2.0")
		m.SetController(Controller{
			Check: func(ctx context.Context) (CheckResult, error) {
				return CheckResult{Available: true, LatestVersion: "0.2.0"}, nil
			},
		})
		m.SetPhase(phase, &CheckResult{Available: phase == "ready", LatestVersion: "0.1.0"}, "prior error")
		if phase == "error" {
			m.SetPhase("error", nil, "prior error")
		}
		if phase == "checking" {
			m.SetPhase("checking", nil, "")
		}
		m.BackgroundCheck(context.Background())
		got, pending, errMsg, _, _ := m.Snapshot()
		if got != phase {
			t.Fatalf("phase %q overwritten to %q (pending=%+v err=%q)", phase, got, pending, errMsg)
		}
	}
	for _, phase := range []string{"idle", "up-to-date", "available"} {
		m := NewManager("0.2.0")
		m.SetController(Controller{
			Check: func(ctx context.Context) (CheckResult, error) {
				return CheckResult{Available: true, LatestVersion: "0.2.0"}, nil
			},
		})
		if phase != "idle" {
			m.SetPhase(phase, &CheckResult{Available: phase == "available", LatestVersion: "0.1.0"}, "")
		}
		m.BackgroundCheck(context.Background())
		got, pending, _, _, _ := m.Snapshot()
		if got != "available" || pending == nil || pending.LatestVersion != "0.2.0" {
			t.Fatalf("phase %q should surface available, got %q %+v", phase, got, pending)
		}
	}
}

func TestUserErrorRedactsPathsAndSensitiveURLs(t *testing.T) {
	leaky := errors.New("open /Users/alice/Library/Application Support/YourMove/staging/update.dmg: permission denied downloading https://updates.example.test/manifest?token=secret-token-123")
	msg := UserError(leaky)
	if strings.Contains(msg, "/Users/alice") || strings.Contains(msg, "secret-token-123") || strings.Contains(msg, "update.dmg:") && strings.Contains(msg, "/") {
		t.Fatalf("leaked path or token: %q", msg)
	}
	if strings.Contains(msg, "jobs.db") {
		t.Fatalf("leaked workspace name: %q", msg)
	}
	if !strings.Contains(msg, "workspace are unchanged") || !strings.Contains(msg, "manual DMG") {
		t.Fatalf("lost recovery guidance: %q", msg)
	}
	sig := UserError(errors.New("updater: ed25519 signature did not verify"))
	if !strings.Contains(sig, "signature") {
		t.Fatalf("lost signature context: %q", sig)
	}
	netErr := UserError(errors.New("dial tcp 127.0.0.1:9: connect: connection refused"))
	if !strings.Contains(netErr, "connection refused") {
		t.Fatalf("lost network context: %q", netErr)
	}
}
