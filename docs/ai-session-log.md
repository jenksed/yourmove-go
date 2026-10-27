# AI session log

## 2026-10-27 - Cross-browser Playwright port onto v0.1.0 baseline

- User goal: Port the qualified hackathon cross-browser implementation into the actual November 6 release codebase (`yourmove-go`) as a working, independently testable macOS release candidate, without hackathon-only code.
- Outcome: Delivered integration branch `release/v01-crossbrowser-port` (commits `4b29e68`, `8db784e` on top of `5da7ff2`) plus frozen ad-hoc candidate at `/private/tmp/ym-frozen-002` (source SHA `8db784e`, app ZIP `a474fffe…2237`). Full, desktop-tagged, and cross-engine qualification suites green. Recommendation: SHIP RESTRICTED (controlled technical alpha); HOLD for normal public download (no Developer ID/notarization).
- Tasks attempted:
  1. Preserved experiment worktree in 3 local commits (packaging, harness drift fix, journal).
  2. Created isolated worktree `~/Projects/yourmove-go-xbrowser` from release `main`.
  3. Ported 24 new + 19 verbatim files; hand-merged 8 diverged files, re-extracting neutral page scripts from the release CDP sources (release had simplified `owns()` to origin/base, no platform).
  4. Built, staged (5 pinned revisions incl. headless shell), signed, and verified the integrated `.app` on bundled bytes only.
  5. Fixed ditto round-trip seal break from AppleDouble `._*` sidecars (strip in stager); re-ran `prepare.sh` to a clean frozen candidate.
  6. Verified: bundled 3-engine launch/close, live Ashby (14 controls) + Greenhouse (23 controls) observation identical per engine, native launch + 25 min run + clean quit with correct branding, synthetic upgrade (baseline-seeded job + approved profile intact under port binary), zero YourMove orphans.
- Files inspected: release `Info.plist`, `build.sh`, `prepare.sh`, `doctor.sh`, `desktop.go`, `app_http.go`, chromium `contact/answer/cover_letter/chromium.go`, `posting.go`, `job.html`, `partials.html`, `inspection_handlers.go`, `application/handlers.go` anchors.
- Files changed: port branch only — neutral browser scripts, `playwright/` bridge, session/engine selection, templates (`engine-selector`, `EngineLabel`), `build.sh`/`doctor.sh`/stager/setup scripts, `.gitignore`, cross-browser docs, native workflow staging block, AppleDouble strip.
- Commands run: `git archive`/`diff` port mapping; `go build`, `go test ./...`, desktop-tagged suites, `-tags qualification` engine + workflow suites; `doctor.sh`, `build.sh` (required), `prepare.sh` (frozen 002); bundled-only probes with `env -i`; `qualify-native-workflow.sh --launch`; live observation probes (read-only); synthetic upgrade via HTTP seeding.
- Tests/checks run: all of the above were run; nothing skipped except full Wails-UI click-through (no AX driver in release harness — manual only) and downloaded-install/Gatekeeper checks (need clean Mac + notarization).
- What worked: port compiles; all suites green; frozen candidate zips, extracts, and re-verifies; engines identical across ATS pages; upgrade preserves records; no migrations added.
- What failed or was skipped: first `prepare.sh` run failed ZIP-verify (fixed via sidecar strip); an early probe-SIGKILL orphan scare was a probe artifact, not a product leak (parentage check); one shell upgrade script showed a curl redirect artifact — re-ran deterministically in Python, clean.
- Stack drift risks: none added — no new module deps, no migrations, CDP remains default, platform concept removed from new path to match release model.
- Assumptions or follow-ups: hackathon remote stays the R&D home; release decisions/CHANGELOG untouched (owner signs); Developer ID + notarization + clean-Mac download test still required for public install.
- Recommended next step: Owner reviews frozen candidate `/private/tmp/ym-frozen-002` (SHA256SUMS inside) and records SHIP RESTRICTED vs HOLD in `release/decisions/v0.1.0.md`; then pursue notarization for public path.
