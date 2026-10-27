# Architecture decisions — cross-browser automation

## ADR-001: Official Playwright via a Go-managed Node.js bridge

**Date:** 2026-10-27 · **Commit:** pending (experiment/browser-playwright)

**Decision:** Drive Chromium, Firefox, and WebKit through the official
Playwright runtime (Node.js `playwright-core`), controlled by Go over a
narrowly scoped stdio JSON-RPC bridge. Go remains the authority: it owns
session lifecycle, authorization, posting-ownership checks, and outcome
reporting. Node executes only authorized page operations and returns observed
results.

**Options considered:**

1. `playwright-community/playwright-go` — Go bindings. Rejected: community
   binding lags the official runtime, pins its own driver version, and adds a
   second browser-version matrix to maintain. The DOM semantics we need are
   JavaScript anyway; a Go binding buys no engine coverage.
2. Official Playwright through a small Go-managed Node bridge. Chosen.
3. WebDriver BiDi — Rejected for now: per-engine driver processes, separate
   session semantics, and weaker auto-waiting/file-upload ergonomics for
   visible-window form filling. Revisit if Playwright distribution proves
   untenable.
4. Custom CDP (status quo) — Retained as the Chromium fallback until the new
   path demonstrates parity. It cannot reach Firefox or WebKit.

**What the CDP implementation did well:** single-binary distribution, exact
target/ownership selection, purpose-qualified file selection with
preflight/ambiguous separation, and read-only observation that never captures
values. The new path keeps all of these: the same `inspect.js`,
contact/answer/cover-letter/file-survey scripts are evaluated verbatim
through Playwright, so DOM semantics are single-sourced in
`internal/application/browser` (`transfer.go`, `document.go`,
`contact_script.go`, `answer_script.go`, `cover_letter_script.go`).

**What changes:** transport only. CDP websocket calls become bridge RPCs
(`evaluate`, `setFiles`, `close`); Chromium-only error/result types moved to
the `browser` package with `chromium` aliases; `sessionHost` selects an engine
per session instead of assuming Chromium.

**Cost introduced:** Node.js runtime + `playwright-core` npm package must ship
with the desktop app, and browser binaries must be resolved at install time
(see distribution notes). Mitigation: reuse the existing Playwright browser
cache layout; no surprise downloads (bridge resolves, never installs).

## ADR-002: One DOM-logic source, two transports

All employer-page JavaScript lives in `internal/application/browser` and is
byte-identical to the proven CDP scripts (verified by extraction test during
the move). `chromium` and `playwright` differ only in how they ship text to
the page and files to inputs. Shared cross-engine qualification asserts the
same outcomes against both transports.

## ADR-003: Parity decision — CDP stays the release default (M5, 2026-10-27)

**Decision:** the Playwright path is the proven cross-browser capability;
the custom CDP path remains the default engine and the November 6 release
path. No engine is removed.

**Evidence for the new path:** 18/18 shared driver assertions across
Chromium/Firefox/WebKit (observe, contacts, resume, answers incl. manual
preservation and rerender, cover text/file incl. digest, multi-step,
no-submit tripwire, crash cleanup with zero orphans); 3/3 app-level HTTP
workflow runs (open → handoff → observe → save → transfer → restart →
stop, incl. the no-silent-switch 409); headed Firefox end-to-end;
CDP regression suites still green after the shared-script extraction.

**What keeps CDP default:** real-ATS qualification exists only for CDP;
packaging (Node runtime + bridge + browser binaries) is resolved in code
but not shipped in an installer; native Wails interaction is blocked on
pre-existing harness drift; Safari ≠ WebKit; Windows/Linux unverified.

**Retire CDP when:** Playwright-Chromium passes real-provider
qualification, the installer ships the bridge + browsers, and native
desktop acceptance covers the selector path. Until then the fallback
costs one maintained transport, not two products.

## ADR-004: Distribution shape for the bridge

The bridge never downloads: `PLAYWRIGHT_BROWSERS_PATH` resolves installed
browsers, `bridgeEnv` neuters the download host, and `AvailableEngines`
gates the UI on what can actually run. The installer must ship (1) the
bridge directory (driver + `npm ci` output) beside the executable or in
`Contents/Resources/playwright-bridge`, (2) a Node 20+ runtime on PATH,
`YOURMOVE_NODE`, or the bundle-relative slots, and (3) Playwright browsers
for the advertised engines. `scripts/setup-playwright-bridge.sh` is the
reproducible dev-side install.
