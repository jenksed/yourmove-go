# Editing the cover-letter instructions

**Edit only [`cover_letter.md`](cover_letter.md)** to tune writing style, relevance, length, and structure. This one source is embedded into the Go desktop application and delivered to every configured provider. Provider request fields and model budgets remain in Go because they are transport behavior, not writing instructions.

## A human-only workflow

1. Open `prompts/cover_letter.md`, copy its entire text, and ask ChatGPT to revise it based on an anonymized bad/good letter comparison. Ask for the **complete replacement file**, not a code change.
2. Replace the file contents in your editor. No agent, migrations, credentials, or model configuration are needed.
3. Review `git diff -- prompts/cover_letter.md` and run `go test ./...` and `go test -tags desktop ./...`.
4. Rebuild/restart YourMove to use the updated embedded file. Simply editing the file does **not** update an already-running app or previously built `.app`.
5. Compare one generated letter with a previous result, using the same job, resumes, provider and model before committing: `git add prompts/cover_letter.md && git commit -m "Tune cover-letter instructions"`.

Do **not** paste private resume text, job applications, credentials, or API responses containing personal data into public issues. A writing instruction cannot guarantee accurate output; always review the generated letter. Malformed responses are rejected by separate Go validation, which should remain strict and shared across providers.

Only consider a provider-specific prompt file after a repeatable provider-specific failure is demonstrated. Otherwise, diverging rules create inconsistent letters and more files to maintain.
