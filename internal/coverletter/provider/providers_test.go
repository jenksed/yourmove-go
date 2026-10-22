package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testInstructions = "trusted writing policy"

func TestCoverProviderHTTPContracts(t *testing.T) {
	const letter = "  I have worked on production troubleshooting, incident recovery and Linux systems for several years.  "
	examples := []struct {
		provider string
		response string
	}{
		{"ollama", `{"response":"` + letter + `"}`},
		{"openai", `{"status":"completed","output":[{"type":"reasoning"},{"type":"message","content":[{"type":"output_text","text":"` + letter + `"}]}]}`},
		{"anthropic", `{"content":[{"type":"text","text":"` + letter + `"}]}`},
		{"openrouter", `{"choices":[{"finish_reason":"stop","message":{"content":"` + letter + `"}}]}`},
		{"compatible", `{"choices":[{"finish_reason":"stop","message":{"content":"` + letter + `"}}]}`},
	}
	for _, example := range examples {
		t.Run(example.provider, func(t *testing.T) {
			const key = "private-unit-test-key"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method %s", r.Method)
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Error("missing content type")
				}
				if example.provider == "anthropic" {
					if r.Header.Get("x-api-key") != key || r.Header.Get("anthropic-version") != "2023-06-01" {
						t.Error("Anthropic header contract")
					}
				} else if example.provider != "ollama" && r.Header.Get("Authorization") != "Bearer "+key {
					t.Error("missing bearer credential")
				}
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if !bytes.Contains(data, []byte("trusted career experience")) {
					t.Error("prompt omitted")
				}
				if !bytes.Contains(data, []byte("sample-model")) {
					t.Error("model omitted")
				}
				var payload map[string]any
				if err := json.Unmarshal(data, &payload); err != nil {
					t.Fatal(err)
				}
				if example.provider == "ollama" {
					if think, exists := payload["think"]; !exists || think != false || payload["options"].(map[string]any)["num_predict"] != float64(850) {
						t.Error("Ollama must disable thinking at the top level within the existing budget")
					}
				} else if _, exists := payload["think"]; exists {
					t.Error("Ollama thinking control leaked to another provider")
				}
				if example.provider == "openai" && !bytes.Contains(data, []byte(`"store":false`)) {
					t.Error("OpenAI request must disable storage")
				}
				// The genre/format contract is consistent across API providers.
				switch example.provider {
				case "openai":
					if payload["instructions"] != testInstructions {
						t.Error("OpenAI format instructions missing")
					}
				case "anthropic":
					if payload["system"] != testInstructions {
						t.Error("Anthropic format instructions missing")
					}
				case "openrouter", "compatible":
					messages, ok := payload["messages"].([]any)
					if !ok || len(messages) != 2 || messages[0].(map[string]any)["role"] != "system" || messages[0].(map[string]any)["content"] != testInstructions {
						t.Error("chat provider format instructions missing")
					}
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, example.response)
			}))
			defer server.Close()
			got, err := runCoverProvider(context.Background(), coverBackend{Provider: example.provider, Model: "sample-model", URL: server.URL, APIKey: key}, "trusted career experience", testInstructions, nil)
			if err != nil || got != letter {
				t.Fatalf("got %q error %v", got, err)
			}
		})
	}
}

func TestCoverProviderFailureCategories(t *testing.T) {
	tests := []struct {
		status int
		body   string
		want   string
	}{
		{401, `{"error":{"message":"not authorized"}}`, "auth"},
		{403, `{}`, "auth"},
		{402, `{}`, "credits"},
		{429, `{}`, "rate"},
		{404, `{}`, "model-missing"},
		{504, `{}`, "timeout"},
		{503, `{}`, "provider-unavailable"},
		{200, `{"choices":[]}`, "output"},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprint(tc.status)+tc.want, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			_, err := runCoverProvider(context.Background(), coverBackend{Provider: "openrouter", Model: "test", URL: server.URL, APIKey: "secret"}, "prompt", testInstructions, nil)
			if got := coverErrorNotice(err); got != tc.want {
				t.Fatalf("got %q wanted %q", got, tc.want)
			}
		})
	}
}

// A partial answer should not become a saved draft even when its text
// otherwise looks plausible. Do not retry automatically or trim to fit.
func TestCoverProviderRejectsIncompleteChatResponse(t *testing.T) {
	for _, provider := range []string{"openrouter", "compatible"} {
		for _, reason := range []string{"length", "content_filter", "tool_calls"} {
			t.Run(provider+"/"+reason, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprintf(w, `{"choices":[{"finish_reason":%q,"message":{"content":"Dear Hiring Team,\n\nI have documented production failures and helped customers.\n\nSincerely,\nAlex"}}]}`, reason)
				}))
				defer server.Close()
				_, err := runCoverProvider(context.Background(), coverBackend{Provider: provider, Model: "test-model", URL: server.URL, APIKey: "secret"}, "evidence", testInstructions, nil)
				if !errors.Is(err, errCoverOutput) {
					t.Fatalf("accepted incomplete response: %v", err)
				}
			})
		}
	}
}

func TestCoverProviderDoesNotFollowRedirect(t *testing.T) {
	forwarded := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded++ }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	_, err := runCoverProvider(context.Background(), coverBackend{Provider: "openai", Model: "test", URL: server.URL, APIKey: "secret"}, "prompt", testInstructions, nil)
	if coverErrorNotice(err) != "provider-unavailable" || forwarded != 0 {
		t.Fatalf("redirect followed: err=%v requests=%d", err, forwarded)
	}
}

func TestCoverProviderConfiguration(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	if _, err := configuredCoverBackend("openai", "gpt-4.1-mini"); coverErrorNotice(err) != "key" {
		t.Fatalf("missing key %v", err)
	}
	t.Setenv("OPENAI_API_KEY", "test-123")
	cfg, err := configuredCoverBackend("openai", "gpt-4.1-mini")
	if err != nil || cfg.APIKey != "test-123" || cfg.URL != "https://api.openai.com/v1/responses" {
		t.Fatalf("openai config %+v %v", cfg, err)
	}
	t.Setenv("YOURMOVE_COMPAT_URL", "http://example.com/v1/chat/completions")
	if _, err := configuredCoverBackend("compatible", "test"); !errors.Is(err, errCoverEndpoint) {
		t.Fatalf("unsafe endpoint %v", err)
	}
	t.Setenv("YOURMOVE_COMPAT_URL", "http://127.0.0.1:9001/v1/chat/completions")
	if _, err := configuredCoverBackend("compatible", "test"); err != nil {
		t.Fatalf("loopback endpoint %v", err)
	}
	if validCompatibleEndpoint("https://example.com/v1/chat/completions?key=secret") {
		t.Fatal("query credentials accepted")
	}
}

func TestConfiguredRunnerUsesCompatibleEndpointAndInstructions(t *testing.T) {
	letter := "I have supported production services and documented reliable troubleshooting steps for engineering teams."
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if len(payload.Messages) != 2 || payload.Messages[0].Content != testInstructions || payload.Messages[1].Content != "trusted career experience" {
			t.Errorf("unexpected configured request: %+v", payload.Messages)
		}
		fmt.Fprintf(w, `{"choices":[{"finish_reason":"stop","message":{"content":%q}}]}`, letter)
	}))
	defer server.Close()
	t.Setenv("YOURMOVE_COMPAT_URL", server.URL+"/v1/chat/completions")
	t.Setenv("YOURMOVE_COMPAT_API_KEY", "")

	runner, err := Configure("compatible", "test-model")
	if err != nil {
		t.Fatal(err)
	}
	got, err := runner.Generate(context.Background(), "trusted career experience", testInstructions)
	if err != nil || got != letter {
		t.Fatalf("got %q, error %v", got, err)
	}
}

func TestMiniMaxCoverLetterReasoningBudget(t *testing.T) {
	for _, model := range []string{"minimax-m3.1-flash:cloud", "minimax-m3.1-flash"} {
		t.Run(model, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload struct {
					Think   *bool `json:"think"`
					Options struct {
						NumPredict int `json:"num_predict"`
					} `json:"options"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				if payload.Think == nil || *payload.Think || payload.Options.NumPredict != 16384 {
					t.Errorf("reasoning control/budget: %+v", payload)
				}
				fmt.Fprint(w, `{"response":"I investigated application failures and provided reproducible findings to engineering."}`)
			}))
			defer server.Close()
			if _, err := runCoverProvider(context.Background(), coverBackend{Provider: "ollama", Model: model, URL: server.URL}, "Synthetic evidence", testInstructions, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}
