package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Structured answer traces and evidence-backed letters use larger compatible
// output budgets; only the trace path requests a JSON object contract.
func TestStructuredCompatibleRequestProfiles(t *testing.T) {
	for _, tc := range []struct {
		name            string
		call            func(Runner) (string, error)
		maxTokens       float64
		reasoning       string
		wantJSONContact bool
	}{
		{"trace", func(r Runner) (string, error) {
			return r.GenerateNarrativeJSON(context.Background(), "trusted career experience", testInstructions)
		}, 2048, "low", true},
		{"letter", func(r Runner) (string, error) {
			return r.GenerateLetter(context.Background(), "trusted career experience", testInstructions)
		}, 2048, "low", false},
		{"plain", func(r Runner) (string, error) {
			return r.Generate(context.Background(), "trusted career experience", testInstructions)
		}, 900, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				if payload["max_tokens"] != tc.maxTokens {
					t.Errorf("max_tokens = %v, want %v", payload["max_tokens"], tc.maxTokens)
				}
				effort, _ := payload["reasoning_effort"].(string)
				if effort != tc.reasoning {
					t.Errorf("reasoning_effort = %q, want %q", effort, tc.reasoning)
				}
				format, hasFormat := payload["response_format"]
				if tc.wantJSONContact {
					object, ok := format.(map[string]any)
					if !hasFormat || !ok || object["type"] != "json_object" {
						t.Errorf("response_format = %v, want json_object", format)
					}
				} else if hasFormat {
					t.Errorf("response_format = %v, want absent", format)
				}
				fmt.Fprintf(w, `{"choices":[{"finish_reason":"stop","message":{"content":"I investigated the synthetic issue."}}]}`)
			}))
			defer server.Close()
			t.Setenv("YOURMOVE_COMPAT_URL", server.URL+"/v1/chat/completions")
			t.Setenv("YOURMOVE_COMPAT_API_KEY", "")

			runner, err := Configure("compatible", "test-model")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tc.call(runner); err != nil {
				t.Fatal(err)
			}
		})
	}
}
