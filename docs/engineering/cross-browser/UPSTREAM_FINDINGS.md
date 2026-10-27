# Upstream findings — contribution shortlist

Ranked by usefulness × confidence. No issues opened or PRs submitted; each
entry needs owner review first. Verify every claim against the pinned
upstream source before proposing changes.

## U-001 — `PLAYWRIGHT_BROWSERS_PATH` single-directory semantics (docs)

- Upstream: Playwright (Node) browser management, `playwright-core@1.58.2`.
- Problem: it is easy to assume `PLAYWRIGHT_BROWSERS_PATH` accepts a
  search list (like `PATH`). A joined macOS+XDG value produced
  `Executable doesn't exist at <a>:<b>` — the whole string is one path.
- Evidence: bridge launch failure log, fixed by selecting the first
  existing root (`internal/application/browser/playwright/rpc.go:
  bridgeEnv`).
- Proposal: one sentence in the Playwright browsers-path docs clarifying
  single-directory semantics (plus operator override passthrough pattern).
- Confidence: high. Next step: check current docs wording, then propose.

## U-002 — `page.evaluate(string)` function-vs-expression handling (docs/investigation)

- Upstream: Playwright `page.evaluate` with a string argument.
- Observation: the string `()=>1+2` evaluated to `undefined` rather than
  `3`; call-shaped strings evaluated normally. Our bridge sidesteps the
  question entirely by composing invoked-call expressions from function
  sources, so no product behavior depends on the answer.
- Proposal: none yet. Needs a minimal repro against current Playwright
  plus a docs/source read to say what the contract actually is.
- Confidence: low (single probe, mechanism not established). Do not file
  as a defect.

## Not pursued

- `crypto.subtle` availability, `setInputFiles` behavior, file-input
  identity across rerenders: all worked as documented; no gap found.
- `playwright-community/playwright-go` lag: a maintenance judgment, not
  an actionable upstream contribution.
