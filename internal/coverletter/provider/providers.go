package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const maxLetterBytes = 12000

// Provider endpoints are fixed by the application. The optional compatible
// endpoint is operator-configured at startup, never supplied by an HTTP form.
type coverBackend struct {
	Provider string
	Model    string
	URL      string
	APIKey   string
}

var (
	errCoverAPIKey       = errors.New("provider API key is not configured")
	errCoverModel        = errors.New("local Ollama model not configured")
	errCoverModelMissing = errors.New("Ollama model not installed")
	errCoverOffline      = errors.New("local Ollama service not reachable")
	errCoverAuth         = errors.New("provider rejected API credentials")
	errCoverRate         = errors.New("provider rate limit reached")
	errCoverCredits      = errors.New("provider account has no available credits")
	errCoverTimeout      = errors.New("provider timed out")
	errCoverUnavailable  = errors.New("provider is unavailable")
	errCoverOutput       = errors.New("provider returned no usable text")
	errCoverEndpoint     = errors.New("compatible provider endpoint is missing or invalid")
)

// Runner contains validated provider configuration without exposing endpoint
// or credential details to application callers.
type Runner struct {
	provider string
	backend  coverBackend
}

// Configure validates a provider/model pair and captures its startup
// configuration. Subscription CLI availability and sign-in are checked when
// Generate runs, as before.
func Configure(provider, model string) (Runner, error) {
	if !validCoverProvider(provider) {
		return Runner{}, errCoverEndpoint
	}
	if coverSubscription(provider) {
		if !validCoverModel(model) {
			return Runner{}, errCoverModel
		}
		return Runner{provider: provider, backend: coverBackend{Provider: provider, Model: model}}, nil
	}
	backend, err := configuredCoverBackend(provider, model)
	if err != nil {
		return Runner{}, err
	}
	return Runner{provider: provider, backend: backend}, nil
}

// Generate sends the supplied evidence prompt to the configured provider.
// Instructions stay application-owned and are passed only for HTTP providers
// that carry them in a separate system/instructions field.
func (r Runner) Generate(ctx context.Context, prompt, instructions string) (string, error) {
	if coverSubscription(r.provider) {
		return runCoverSubscription(ctx, r.provider, r.backend.Model, prompt)
	}
	return runCoverProvider(ctx, r.backend, prompt, instructions, nil)
}

// GenerateNarrative performs one bounded narrative request, independent of letter formatting.
func (r Runner) GenerateNarrative(ctx context.Context, prompt, instructions string) (string, error) {
	if coverSubscription(r.provider) {
		return runSubscriptionProfile(ctx, r.provider, r.backend.Model, prompt+"\n\n"+instructions, true)
	}
	if r.provider == "ollama" {
		prompt += "\n\n" + instructions
	}
	return runProviderProfile(ctx, r.backend, prompt, instructions, nil, true)
}

func ValidProvider(provider string) bool { return validCoverProvider(provider) }

func ValidModel(model string) bool { return validCoverModel(model) }

func DefaultModel(provider string) string { return coverDefaultModel(provider) }

func Configured(provider string) bool { return coverProviderConfigured(provider) }

func SubscriptionReadiness(provider string) (string, string) { return coverCLIReadiness(provider) }

// ErrorNotice returns the stable application notice code for provider errors.
func ErrorNotice(err error) string { return coverErrorNotice(err) }

func validCoverProvider(provider string) bool {
	switch provider {
	case "ollama", "openai", "anthropic", "openrouter", "compatible", "codex", "claude-cli":
		return true
	}
	return false
}

func validCoverModel(model string) bool {
	return model != "" && len(model) <= 120 && !strings.ContainsAny(model, " \t\r\n\\\x00")
}

// Never derive a network destination or credential from the user-facing form.
func configuredCoverBackend(provider, model string) (coverBackend, error) {
	if !validCoverProvider(provider) {
		return coverBackend{}, errCoverEndpoint
	}
	if !validCoverModel(model) {
		return coverBackend{}, errCoverModel
	}
	config := coverBackend{Provider: provider, Model: model}
	switch provider {
	case "ollama":
		config.URL = "http://127.0.0.1:11434/api/generate"
	case "openai":
		config.URL, config.APIKey = "https://api.openai.com/v1/responses", os.Getenv("OPENAI_API_KEY")
	case "anthropic":
		config.URL, config.APIKey = "https://api.anthropic.com/v1/messages", os.Getenv("ANTHROPIC_API_KEY")
	case "openrouter":
		config.URL, config.APIKey = "https://openrouter.ai/api/v1/chat/completions", os.Getenv("OPENROUTER_API_KEY")
	case "compatible":
		config.URL, config.APIKey = os.Getenv("YOURMOVE_COMPAT_URL"), os.Getenv("YOURMOVE_COMPAT_API_KEY")
		if !validCompatibleEndpoint(config.URL) {
			return coverBackend{}, errCoverEndpoint
		}
	}
	if provider != "ollama" && (provider != "compatible" || strings.HasPrefix(config.URL, "https://")) && strings.TrimSpace(config.APIKey) == "" {
		return coverBackend{}, errCoverAPIKey
	}
	if strings.ContainsAny(config.APIKey, "\r\n") {
		return coverBackend{}, errCoverAPIKey
	}
	return config, nil
}

func validCompatibleEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		!strings.HasSuffix(u.Path, "/chat/completions") {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	if u.Scheme != "http" {
		return false
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func coverProviderConfigured(provider string) bool {
	if coverSubscription(provider) {
		return coverCLIInstalled(provider)
	}
	switch provider {
	case "ollama":
		return true // Connectivity/model availability is checked at generation.
	case "openai":
		return strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != ""
	case "anthropic":
		return strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")) != ""
	case "openrouter":
		return strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")) != ""
	case "compatible":
		return validCompatibleEndpoint(os.Getenv("YOURMOVE_COMPAT_URL"))
	}
	return false
}

// One bounded request, no redirects, no retries (and therefore no double charge).
// Provider response and request bodies are never logged, including on failures.
func runCoverProvider(ctx context.Context, backend coverBackend, prompt, instructions string, client *http.Client) (string, error) {
	return runProviderProfile(ctx, backend, prompt, instructions, client, false)
}

func runProviderProfile(ctx context.Context, backend coverBackend, prompt, instructions string, client *http.Client, narrative bool) (string, error) {
	if !validCoverProvider(backend.Provider) || !validCoverModel(backend.Model) || backend.URL == "" {
		return "", errCoverModel
	}
	if backend.Provider != "ollama" && backend.Provider != "compatible" && backend.APIKey == "" {
		return "", errCoverAPIKey
	}
	var request any
	switch backend.Provider {
	case "ollama":
		// Disable optional reasoning before budgeting. MiniMax M3.1 Flash on
		// Ollama Cloud still reasons with think:false (verified on both APIs);
		// its reasoning and answer share num_predict. Keep other models bounded
		// as before, and allow this model the budget its measured calls need.
		budget := 850
		if backend.Model == "minimax-m3.1-flash:cloud" || backend.Model == "minimax-m3.1-flash" {
			budget = 16384
		}
		if narrative {
			budget = 1024
		}
		request = map[string]any{"model": backend.Model, "prompt": prompt, "stream": false, "think": false,
			"options": map[string]any{"temperature": 0.35, "num_predict": budget}}
	case "openai":
		// The API's output_text is an SDK convenience; parse output[] in raw HTTP.
		request = map[string]any{"model": backend.Model, "input": prompt, "instructions": instructions, "store": false, "max_output_tokens": 1400}
	case "anthropic":
		request = map[string]any{"model": backend.Model, "max_tokens": 900, "system": instructions,
			"messages": []map[string]string{{"role": "user", "content": prompt}}}
	default:
		request = map[string]any{"model": backend.Model, "stream": false, "max_tokens": 900,
			"messages": []map[string]string{{"role": "system", "content": instructions}, {"role": "user", "content": prompt}}}
	}
	if narrative {
		fields := request.(map[string]any)
		switch backend.Provider {
		case "openai":
			fields["max_output_tokens"] = 1600
		case "ollama":
		default:
			fields["max_tokens"] = 1024
		}
	}
	data, err := json.Marshal(request)
	if err != nil {
		return "", errCoverOutput
	}
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, backend.URL, bytes.NewReader(data))
	if err != nil {
		return "", errCoverEndpoint
	}
	req.Header.Set("Content-Type", "application/json")
	switch backend.Provider {
	case "anthropic":
		req.Header.Set("x-api-key", backend.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	case "openai", "openrouter":
		req.Header.Set("Authorization", "Bearer "+backend.APIKey)
	case "compatible":
		if backend.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+backend.APIKey)
		}
	}
	if client == nil {
		client = &http.Client{Timeout: 120 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	// Redirects must never forward credential headers or sensitive source text.
	// Copying the transport lets tests supply a local recorder without an external call.
	guarded := *client
	guarded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := guarded.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", errCoverTimeout
		}
		if backend.Provider == "ollama" {
			return "", errCoverOffline
		}
		return "", errCoverUnavailable
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return "", errCoverAuth
	case http.StatusPaymentRequired:
		return "", errCoverCredits
	case http.StatusTooManyRequests:
		return "", errCoverRate
	case http.StatusNotFound:
		return "", errCoverModelMissing
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return "", errCoverTimeout
	default:
		return "", errCoverUnavailable
	}
	limited, err := io.ReadAll(io.LimitReader(resp.Body, 128<<10+1))
	if err != nil || len(limited) > 128<<10 {
		return "", errCoverOutput
	}
	var text string
	switch backend.Provider {
	case "ollama":
		var result struct {
			Done       *bool  `json:"done"`
			DoneReason string `json:"done_reason"`
			Response   string `json:"response"`
			Error      string `json:"error"`
		}
		if json.Unmarshal(limited, &result) != nil || result.Error != "" {
			return "", errCoverOutput
		}
		if narrative && ((result.Done != nil && !*result.Done) || result.DoneReason == "length") {
			return "", errCoverOutput
		}
		text = result.Response
	case "openai":
		var result struct {
			Status string `json:"status"`
			Output []struct {
				Type    string `json:"type"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"output"`
		}
		if json.Unmarshal(limited, &result) != nil || (result.Status != "" && result.Status != "completed") {
			return "", errCoverOutput
		}
		var parts []string
		for _, item := range result.Output {
			if item.Type != "message" {
				continue
			}
			for _, part := range item.Content {
				if part.Type == "output_text" {
					parts = append(parts, part.Text)
				}
			}
		}
		text = strings.Join(parts, "\n")
	case "anthropic":
		var result struct {
			StopReason string `json:"stop_reason"`
			Content    []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if json.Unmarshal(limited, &result) != nil {
			return "", errCoverOutput
		}
		if narrative && result.StopReason != "" && result.StopReason != "end_turn" && result.StopReason != "stop_sequence" {
			return "", errCoverOutput
		}
		var parts []string
		for _, part := range result.Content {
			if part.Type == "text" {
				parts = append(parts, part.Text)
			}
		}
		text = strings.Join(parts, "\n")
	default:
		var result struct {
			Choices []struct {
				FinishReason string `json:"finish_reason"`
				Message      struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if json.Unmarshal(limited, &result) != nil || len(result.Choices) == 0 {
			return "", errCoverOutput
		}
		// A completion stopped by the output cap is not a complete letter.
		// Compatible endpoints may omit finish_reason; reject known failures,
		// and require a successful stop when OpenRouter supplies a reason.
		reason := strings.ToLower(strings.TrimSpace(result.Choices[0].FinishReason))
		if reason == "length" || reason == "max_tokens" || reason == "content_filter" || reason == "tool_calls" ||
			(backend.Provider == "openrouter" && reason != "" && reason != "stop") {
			return "", errCoverOutput
		}
		text = result.Choices[0].Message.Content
	}
	min, max := 30, maxLetterBytes
	if narrative {
		min, max = 1, 6000
	}
	if len(text) < min || (narrative && strings.TrimSpace(text) == "") || len(text) > max || strings.ContainsRune(text, 0) {
		return "", errCoverOutput
	}
	return text, nil
}

func coverErrorNotice(err error) string {
	switch {
	case errors.Is(err, errCoverModel), errors.Is(err, errCoverEndpoint):
		return "model"
	case errors.Is(err, errCoverCLI):
		return "cli"
	case errors.Is(err, errCoverLogin):
		return "login"
	case errors.Is(err, errCoverCLIFailed):
		return "cli-failed"
	case errors.Is(err, errCoverAPIKey):
		return "key"
	case errors.Is(err, errCoverAuth):
		return "auth"
	case errors.Is(err, errCoverRate):
		return "rate"
	case errors.Is(err, errCoverCredits):
		return "credits"
	case errors.Is(err, errCoverTimeout):
		return "timeout"
	case errors.Is(err, errCoverOffline):
		return "offline"
	case errors.Is(err, errCoverModelMissing):
		return "model-missing"
	case errors.Is(err, errCoverOutput):
		return "output"
	case errors.Is(err, errCoverUnavailable):
		return "provider-unavailable"
	default:
		return "failed"
	}
}

func coverDefaultModel(provider string) string {
	switch provider {
	case "codex":
		return "default"
	case "claude-cli":
		return "sonnet"
	case "openai":
		return "gpt-4.1-mini"
	case "anthropic":
		return "claude-sonnet-4-5"
	case "openrouter":
		return "openai/gpt-4o-mini"
	case "compatible":
		return strings.TrimSpace(os.Getenv("YOURMOVE_COMPAT_MODEL"))
	default:
		if s := strings.TrimSpace(os.Getenv("YOURMOVE_OLLAMA_MODEL")); s != "" {
			return s
		}
		return "qwen3.5:4b"
	}
}
