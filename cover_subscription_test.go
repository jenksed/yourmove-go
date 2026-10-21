package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubscriptionEnvironmentScrubsAPIKeys(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "never-in-claude-cli")
	t.Setenv("OPENAI_API_KEY", "never-in-codex-cli")
	t.Setenv("CODEX_API_KEY", "never-in-codex-cli")
	all := strings.Join(subscriptionEnvironment(), "\n")
	if strings.Contains(all, "never-in-") {
		t.Fatal("subscription subprocess received API credentials")
	}
}

func TestCoverSubscriptionRunsVendorCLIWithNoTools(t *testing.T) {
	dir := t.TempDir()
	for name, script := range map[string]string{
		"claude": `#!/bin/sh
if [ "$1" = auth ]; then printf '{"authMethod":"claude.ai"}'; exit 0; fi
if [ -n "$ANTHROPIC_API_KEY" ]; then exit 1; fi
found=0; for a in "$@"; do if [ "$a" = --tools ]; then found=1; fi; done
if [ "$found" -ne 1 ]; then exit 1; fi
cat >/dev/null
printf 'I have supported production incidents and customer-facing Linux systems. This is relevant to the role.'
`,
		"codex": `#!/bin/sh
if [ "$1" = login ]; then echo 'Logged in using ChatGPT'; exit 0; fi
if [ -n "$OPENAI_API_KEY" ]; then exit 1; fi
found=0; for a in "$@"; do if [ "$a" = --ephemeral ]; then found=1; fi; done
if [ "$found" -ne 1 ]; then exit 1; fi
out=''; while [ "$#" -gt 0 ]; do
  if [ "$1" = --output-last-message ]; then shift; out="$1"; fi
  shift
done
cat >/dev/null
printf 'I have supported production incidents and customer-facing Linux systems. This is relevant to the role.' > "$out"
`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ANTHROPIC_API_KEY", "bad-key")
	t.Setenv("OPENAI_API_KEY", "bad-key")
	for _, provider := range []string{"codex", "claude-cli"} {
		t.Run(provider, func(t *testing.T) {
			if state, detail := coverCLIReadiness(provider); state != "ready" {
				t.Fatalf("%s: %s", state, detail)
			}
			result, err := runCoverSubscription(context.Background(), provider, "default", "private evidence")
			if err != nil || !strings.Contains(result, "production incidents") {
				t.Fatalf("%q: %v", result, err)
			}
		})
	}
}
