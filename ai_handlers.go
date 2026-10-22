package main

import (
	"html/template"
	"net/http"

	coverprovider "example.com/yourmove-lite/internal/coverletter/provider"
)

type aiConnection struct {
	ID, Name, Access, State, Detail, Setup, Model string
}

type aiConnectionsPage struct{ Connections []aiConnection }

var aiConnectionsTemplate = template.Must(template.ParseFS(templateFiles, "templates/ai.html", "templates/partials.html"))

// The connections dashboard describes effective runtime access; it does not
// render API keys, persist credentials, contact a model, or start sign-in.
func (a *App) aiConnectionsPage(w http.ResponseWriter, r *http.Request) {
	sources := []aiConnection{
		{ID: "codex", Name: "ChatGPT via Codex", Access: "Subscription CLI", Setup: "Install Codex, then run codex login and choose ChatGPT.", Model: "default"},
		{ID: "claude-cli", Name: "Claude via Claude Code", Access: "Subscription CLI", Setup: "Install Claude Code, run claude, and sign in with Claude.ai (not Console).", Model: "sonnet"},
		{ID: "ollama", Name: "Ollama", Access: "On-device", Setup: "Start Ollama, run ollama list, then choose an installed model.", Model: coverprovider.DefaultModel("ollama")},
		{ID: "openai", Name: "OpenAI API", Access: "Usage billed separately", Setup: "Set OPENAI_API_KEY in your shell environment and restart YourMove.", Model: coverprovider.DefaultModel("openai")},
		{ID: "anthropic", Name: "Anthropic API", Access: "Usage billed separately", Setup: "Set ANTHROPIC_API_KEY in your shell environment and restart YourMove.", Model: coverprovider.DefaultModel("anthropic")},
		{ID: "openrouter", Name: "OpenRouter", Access: "Usage billed separately", Setup: "Set OPENROUTER_API_KEY and restart YourMove.", Model: coverprovider.DefaultModel("openrouter")},
		{ID: "compatible", Name: "OpenAI-compatible", Access: "Custom endpoint", Setup: "Set YOURMOVE_COMPAT_URL to the full /chat/completions URL and YOURMOVE_COMPAT_API_KEY when required.", Model: coverprovider.DefaultModel("compatible")},
	}
	for i := range sources {
		s := &sources[i]
		switch s.ID {
		case "codex", "claude-cli":
			s.State, s.Detail = coverprovider.SubscriptionReadiness(s.ID)
		case "ollama":
			s.State, s.Detail = "setup", "Local service; readiness checked at generation"
		default:
			if coverprovider.Configured(s.ID) {
				s.State, s.Detail = "ready", "Configured for this session"
			} else {
				s.State, s.Detail = "setup", "Configuration needed"
			}
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	if err := aiConnectionsTemplate.Execute(w, aiConnectionsPage{Connections: sources}); err != nil {
		_ = err // Avoid rendering secrets from unexpected template errors.
	}
}
