package app

import (
	"fmt"

	coverprovider "github.com/jenksed/yourmove-go/internal/coverletter/provider"
)

type answerAIChoice struct{ Identity, Label string }
type answerAIChoices struct {
	Evidence, Writing               []answerAIChoice
	ApplicationResume, MasterResume *answerAIChoice
}

type answerAIProviderOption struct {
	ID, Label, Model, Help string
}

func applicationAnswerAIProviders(configured map[string]bool) []answerAIProviderOption {
	var options []answerAIProviderOption
	for _, id := range []string{"ollama", "openai", "anthropic", "openrouter", "compatible", "codex", "claude-cli"} {
		option := answerAIProviderOption{ID: id, Model: coverprovider.DefaultModel(id)}
		switch id {
		case "ollama":
			option.Label = "Ollama (local or cloud inference)"
			option.Help = "Ollama is checked when you generate. Keep the service running and choose a model shown by ollama list. Some models use cloud inference; see AI connections."
		case "openai":
			option.Label = "OpenAI API (remote)"
			if configured[id] {
				option.Help = "The API key is available in this launch environment. Remote use may be billed separately."
			} else {
				option.Help = "Set OPENAI_API_KEY in the environment used to launch YourMove, then restart it. API use may be billed separately."
			}
		case "anthropic":
			option.Label = "Anthropic API (remote)"
			if configured[id] {
				option.Help = "The API key is available in this launch environment. Remote use may be billed separately."
			} else {
				option.Help = "Set ANTHROPIC_API_KEY in the environment used to launch YourMove, then restart it. API use may be billed separately."
			}
		case "openrouter":
			option.Label = "OpenRouter (remote)"
			if configured[id] {
				option.Help = "The API key is available in this launch environment. Remote use may be billed separately."
			} else {
				option.Help = "Set OPENROUTER_API_KEY in the environment used to launch YourMove, then restart it. API use may be billed separately."
			}
		case "compatible":
			option.Label = "OpenAI-compatible endpoint"
			if configured[id] && option.Model != "" {
				option.Help = "The configured endpoint and model are available. It may forward selected text to a remote service."
			} else if configured[id] {
				option.Help = "Set YOURMOVE_COMPAT_MODEL for the configured endpoint, then restart YourMove. The endpoint may forward selected text remotely."
			} else {
				option.Help = "Set YOURMOVE_COMPAT_URL to a full /chat/completions endpoint and YOURMOVE_COMPAT_MODEL, then restart YourMove. The service may forward text remotely."
			}
		case "codex":
			option.Label = "ChatGPT via Codex CLI (vendor service)"
			option.Model = "default"
			if configured[id] {
				option.Help = "Codex is installed; sign-in is checked when you generate. Use codex login with ChatGPT, not an API key."
			} else {
				option.Help = "Install Codex and sign in with codex login using ChatGPT. See AI connections for setup."
			}
		case "claude-cli":
			option.Label = "Claude via Claude Code CLI (vendor service)"
			option.Model = "sonnet"
			if configured[id] {
				option.Help = "Claude Code is installed; sign-in is checked when you generate. Sign in with Claude.ai, not Console."
			} else {
				option.Help = "Install Claude Code and sign in with Claude.ai. See AI connections for setup."
			}
		}
		options = append(options, option)
	}
	return options
}

// Bound additional local retrieval; suggestions are listed first and never
// selected automatically. All authoritative reads happen again at review.
func (a *App) answerAIChoices(job Job, q preparedQuestion) *answerAIChoices {
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 || !narrativeAnswerEligible(q) {
		return nil
	}
	out := &answerAIChoices{}
	evidenceIDs := []int64{}
	libraryIDs := []int64{}
	for _, s := range q.CareerEvidenceSuggestions {
		evidenceIDs = append(evidenceIDs, s.Evidence.ID)
	}
	for _, s := range q.LibrarySuggestions {
		libraryIDs = append(libraryIDs, s.EntryID)
	}
	for _, query := range []struct {
		sql    string
		target *[]int64
	}{
		{`SELECT id FROM career_evidence WHERE status='reviewed' ORDER BY updated_at DESC,id DESC LIMIT 30`, &evidenceIDs},
		{`SELECT id FROM answer_library ORDER BY updated_at DESC,id DESC LIMIT 30`, &libraryIDs},
	} {
		rows, err := a.db.Query(query.sql)
		if err != nil {
			continue
		}
		for rows.Next() {
			var id int64
			if rows.Scan(&id) == nil {
				*query.target = append(*query.target, id)
			}
		}
		rows.Close()
	}
	seen := map[int64]bool{}
	for _, id := range evidenceIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		e, err := careerEvidenceByID(a.db, id)
		if err != nil || e.Status != "reviewed" {
			continue
		}
		out.Evidence = append(out.Evidence, answerAIChoice{fmt.Sprintf("%d:%d:%s", e.ID, e.Revision, e.CreatedAt), fmt.Sprintf("%s · revision %d", e.Title, e.Revision)})
	}
	seen = map[int64]bool{}
	for _, id := range libraryIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		e, err := a.answerLibraryBySourceID(a.db, id)
		if err != nil {
			continue
		}
		out.Writing = append(out.Writing, answerAIChoice{fmt.Sprintf("%d:%d:%s", e.ID, e.Revision, e.CreatedAt), fmt.Sprintf("%s · %s · %s · revision %d", e.OriginalQuestion, e.SourceEmployer, e.SourceJobTitle, e.Revision)})
	}
	choice := func(r Resume) *answerAIChoice {
		return &answerAIChoice{fmt.Sprintf("%d:%d:%s:%s", r.ID, r.Version, r.SHA256, r.ImportedAt), fmt.Sprintf("%s · version %d", r.Name, r.Version)}
	}
	if r, err := a.resumeByID(job.SelectedResumeID); err == nil && r.ArchivedAt == "" {
		out.ApplicationResume = choice(r)
	}
	if r, err := a.masterResume(); err == nil && r.ArchivedAt == "" {
		out.MasterResume = choice(r)
	}
	return out
}
