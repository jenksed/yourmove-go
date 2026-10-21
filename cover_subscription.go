package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	errCoverCLI       = errors.New("subscription command is not installed")
	errCoverLogin     = errors.New("subscription command needs account sign-in")
	errCoverCLIFailed = errors.New("subscription CLI invocation failed")
)

// Subscription routes launch the vendor's unmodified CLI, authenticated by the
// account owner. Never import CLI OAuth tokens or make direct API calls with them.
// These are personal desktop integrations; redistribution needs a terms review.
func coverSubscription(provider string) bool {
	return provider == "codex" || provider == "claude-cli"
}

func coverCLIName(provider string) string {
	if provider == "codex" {
		return "codex"
	}
	if provider == "claude-cli" {
		return "claude"
	}
	return ""
}

// Remove provider API credentials from subscription-mode processes. This avoids
// accidental API billing (particularly Claude Code's API-key precedence).
func subscriptionEnvironment() []string {
	blocked := map[string]bool{
		"OPENAI_API_KEY": true, "ANTHROPIC_API_KEY": true,
		"CLAUDE_CODE_API_KEY": true, "CODEX_API_KEY": true,
		"CLAUDE_CODE_USE_BEDROCK": true, "CLAUDE_CODE_USE_VERTEX": true,
		"CLAUDE_CODE_USE_FOUNDRY": true,
	}
	env := make([]string, 0, len(os.Environ()))
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if !blocked[key] {
			env = append(env, value)
		}
	}
	return env
}

// Used for page rendering without spawning a potentially slow login probe.
func coverCLIInstalled(provider string) bool {
	name := coverCLIName(provider)
	if name == "" {
		return false
	}
	_, err := exec.LookPath(name)
	return err == nil
}

// A readiness check is local and never performs a model request.
// Results can change if the operator signs out while the app is running.
func coverCLIReadiness(provider string) (string, string) {
	name := coverCLIName(provider)
	if name == "" {
		return "unavailable", "Unknown provider"
	}
	binary, err := exec.LookPath(name)
	if err != nil {
		return "unavailable", "Install " + name + " and sign in"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	args := []string{"login", "status"}
	if provider == "claude-cli" {
		args = []string{"auth", "status"}
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = subscriptionEnvironment()
	var out limitedOutput
	cmd.Stdout, cmd.Stderr = &out, &out
	if cmd.Run() != nil {
		return "signin", "Run " + name + " to sign in"
	}
	result := strings.ToLower(out.String())
	if provider == "codex" {
		if strings.Contains(result, "chatgpt") {
			return "ready", "ChatGPT sign-in detected"
		}
		return "signin", "Use codex login with ChatGPT (not API key)"
	}
	var auth struct {
		AuthMethod string `json:"authMethod"`
	}
	if json.Unmarshal([]byte(out.String()), &auth) == nil && auth.AuthMethod == "claude.ai" {
		return "ready", "Claude subscription sign-in detected"
	}
	return "signin", "Sign in with your Claude subscription (not Console API)"
}

type limitedOutput struct{ b []byte }

func (b *limitedOutput) Write(p []byte) (int, error) {
	if len(b.b) < 32<<10 {
		limit := 32<<10 - len(b.b)
		if len(p) < limit {
			limit = len(p)
		}
		b.b = append(b.b, p[:limit]...)
	}
	return len(p), nil
}
func (b *limitedOutput) String() string { return string(b.b) }

func runCoverSubscription(ctx context.Context, provider, model, prompt string) (string, error) {
	if !coverSubscription(provider) || !validCoverModel(model) {
		return "", errCoverModel
	}
	state, _ := coverCLIReadiness(provider)
	if state == "unavailable" {
		return "", errCoverCLI
	}
	if state != "ready" {
		return "", errCoverLogin
	}
	path, err := exec.LookPath(coverCLIName(provider))
	if err != nil {
		return "", errCoverCLI
	}
	dir, err := os.MkdirTemp("", "yourmove-letter-cli-")
	if err != nil {
		return "", errCoverUnavailable
	}
	defer os.RemoveAll(dir)
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	var args []string
	outputFile := filepath.Join(dir, "letter.txt")
	if provider == "codex" {
		args = []string{"exec", "--ephemeral", "--sandbox", "read-only", "--skip-git-repo-check", "--output-last-message", outputFile}
		if model != "default" {
			args = append(args, "--model", model)
		}
		args = append(args, "-")
	} else {
		args = []string{"--safe-mode", "-p", "--no-session-persistence", "--no-chrome", "--output-format", "text", "--tools", "", "--disallowedTools", "mcp__*"}
		if model != "default" {
			args = append(args, "--model", model)
		}
		args = append(args, "Write the cover letter using only the supplied text on stdin. Return only the letter body. Do not use tools.")
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = dir
	cmd.Env = subscriptionEnvironment()
	cmd.Stdin = strings.NewReader(prompt)
	var out limitedOutput
	cmd.Stdout = &out
	cmd.Stderr = io.Discard // Never display or persist prompt excerpts from CLI diagnostic output.
	if cmd.Run() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", errCoverTimeout
		}
		return "", errCoverCLIFailed
	}
	text := out.String()
	if provider == "codex" {
		body, err := os.ReadFile(outputFile)
		if err != nil || len(body) > maxLetterBytes {
			return "", errCoverOutput
		}
		text = string(body)
	}
	text = strings.TrimSpace(text)
	if len(text) < 30 || len(text) > maxLetterBytes || strings.ContainsRune(text, 0) {
		return "", errCoverOutput
	}
	return text, nil
}
