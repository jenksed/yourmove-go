package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNarrativeHTTPProfiles(t *testing.T) {
	for _, tc := range []struct{ provider, response, incomplete string }{
		{"ollama", `{"done":true,"response":"I investigated it."}`, `{"done":false,"response":"I investigated it."}`},
		{"openai", `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"I investigated it."}]}]}`, `{"status":"incomplete"}`},
		{"anthropic", `{"stop_reason":"end_turn","content":[{"type":"text","text":"I investigated it."}]}`, `{"stop_reason":"max_tokens","content":[{"type":"text","text":"I investigated it."}]}`},
		{"openrouter", `{"choices":[{"finish_reason":"stop","message":{"content":"I investigated it."}}]}`, `{"choices":[{"finish_reason":"length","message":{"content":"I investigated it."}}]}`},
		{"compatible", `{"choices":[{"message":{"content":"I investigated it."}}]}`, `{"choices":[{"finish_reason":"tool_calls","message":{"content":"I investigated it."}}]}`},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			response := tc.response
			narrative := true
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req map[string]any
				if json.NewDecoder(r.Body).Decode(&req) != nil {
					t.Error("request")
				}
				if narrative {
					switch tc.provider {
					case "ollama":
						if req["options"].(map[string]any)["num_predict"] != float64(1024) {
							t.Error("narrative inherited letter budget")
						}
					case "openai":
						if req["max_output_tokens"] != float64(1600) {
							t.Error("budget")
						}
					default:
						if req["max_tokens"] != float64(1024) {
							t.Error("budget")
						}
					}
				}
				fmt.Fprint(w, response)
			}))
			defer server.Close()
			runner := Runner{provider: tc.provider, backend: coverBackend{Provider: tc.provider, Model: "minimax-m3.1-flash:cloud", URL: server.URL, APIKey: "synthetic-key"}}
			answer, err := runner.GenerateNarrative(context.Background(), "synthetic evidence", "narrative policy")
			if err != nil || answer != "I investigated it." {
				t.Fatal(answer, err)
			}
			response = tc.incomplete
			if _, err := runner.GenerateNarrative(context.Background(), "synthetic evidence", "policy"); err == nil {
				t.Fatal("accepted incomplete")
			}
			response = tc.response
			narrative = false
			if _, err := runner.Generate(context.Background(), "synthetic evidence", "policy"); err == nil {
				t.Fatal("changed letter minimum")
			}
		})
	}
}
func TestNarrativeSubscriptionProfiles(t *testing.T) {
	dir := t.TempDir()
	scripts := map[string]string{
		"claude": `#!/bin/sh
if [ "$1" = auth ]; then printf '{"authMethod":"claude.ai"}'; exit; fi
for a in "$@"; do case "$a" in *cover\ letter*|*synthetic\ evidence*) exit 1;; esac; done
cat >/dev/null
printf 'I investigated it.'
`,
		"codex": `#!/bin/sh
if [ "$1" = login ]; then echo 'Logged in using ChatGPT'; exit; fi
out=''; while [ "$#" -gt 0 ]; do if [ "$1" = --output-last-message ]; then shift; out="$1"; fi; shift; done
cat >/dev/null
printf 'I investigated it.' > "$out"
`}
	for name, script := range scripts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, provider := range []string{"codex", "claude-cli"} {
		runner, err := Configure(provider, "default")
		if err != nil {
			t.Fatal(err)
		}
		answer, err := runner.GenerateNarrative(context.Background(), "synthetic evidence", "policy")
		if err != nil || !strings.HasPrefix(answer, "I investigated") {
			t.Fatal(provider, answer, err)
		}
	}
}
