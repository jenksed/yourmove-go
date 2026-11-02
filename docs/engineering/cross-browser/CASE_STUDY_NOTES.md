# Case study notes — from custom CDP to cross-browser automation

Source material, not a finished article. Central question: what does it
actually take to evolve a custom Go/CDP browser integration into reliable
cross-browser automation inside a real desktop application?

## The starting point

- YourMove drove one visible Chromium via hand-rolled CDP (`websocket`
  dial, `Runtime.evaluate`, `DOM.setFileInputFiles`). It owned exact target
  selection, posting-ownership checks, purpose-qualified file selection with
  preflight/ambiguous separation, and read-only observation that never
  captures values.
- Complexity owned: DevTools target lifecycle, CDP call correlation,
  per-operation DOM re-resolution, upload ambiguity discipline.
- Complexity avoided: everything non-Chromium, auto-waiting, cross-engine
  quirks, driver distribution (one binary + system Chrome).

## The migration

- Technology decision: official Playwright through a Go-managed Node bridge
  (`playwright-core@1.58.2` pinned; `PLAYWRIGHT_BROWSERS_PATH` reuse; no
  runtime downloads). Rejected: community Go bindings (version lag, no
  engine gain) and WebDriver BiDi (session-per-driver complexity for now).
- Key architectural move: the page JavaScript did NOT change. `inspect.js`
  and the contact/answer/cover-letter/file-survey scripts moved byte-
  identical into the neutral `browser` package; Chromium and Playwright
  differ only in how text reaches the page. Result: zero per-engine DOM
  workarounds for the supported form scope (CF-001).
- Forced seam work: result/error types and ownership sentinels moved from
  `chromium` to `browser`; `sessionHost` gained a per-session engine with a
  Chromium-default fallback; the factory stayed signature-stable via a
  launch closure (34 test call sites untouched except the 9 `start` calls).
- The first working Firefox path broke twice on bridge composition, never
  on Firefox: template-literal escape corruption (`\$` replacement
  semantics) fixed by concatenation; IIFE trailing-`;` fixed by one
  wrapping rule; a function-returning-function serialized to `undefined`
  and surfaced as `verified=false` on exactly two engines.

## The discoveries (counterintuitive first)

1. The DOM is portable; the transport is not. All engine risk sat in the
   200-line Node bridge, none in the 8,000-line-equivalent page semantics.
2. Launch-latency comparisons lie across completion definitions: CDP
   returns at port exposure (possibly blank page); the bridge waits for
   `domcontentloaded`. Steady-state operations were identical to 20ms.
3. `crypto.subtle` just worked on loopback everywhere — the feared TLS
   fixture matrix never materialized.
4. The no-silent-switch rule bit the implementer first: the handoff's
   default-Chromium start tripped on an active Firefox session and wiped
   the state display via the launch-failure path. Enforcement belongs at
   the open endpoint, not inside `start()`.
5. Pending-scope hashing (transferred contacts excluded from answer scope)
   is where app policy meets engine neutrality — the test initially sent
   the unfiltered hash and got a correct 409.

## The outcome (so far)

- Works across engines: open/observe/contacts/answers/cover-text/cover-
  file/resume/dynamic-rerender/multi-step/no-submit/no-replay/cleanup —
  18/18 driver assertions, 3/3 app-workflow runs, crash-recovery with
  process-group cleanup, headed Firefox end-to-end (13s).
- Remains Chromium/CDP: the release path default (unchanged behavior,
  unchanged copy), real-ATS qualification, packaged distribution
  (Node + bridge + browsers), native Wails interaction (blocked on
  pre-existing harness drift), Safari (WebKit is not Safari).
- Judgment so far: replacing CDP is justified for coverage but not yet
  for the release path. Keep the fallback until real-provider
  qualification and installable distribution are proven.

## Candidate article angles (pick ≤3 at the end)

1. *One DOM source, three engines: the boring migration that worked* —
   byte-identical page scripts as a portability strategy.
2. *The hidden cost of browser automation in desktop software is launch,
   not control* — completion-definition traps in benchmarking.
3. *Ownership outlives the driver* — posting-ownership, approval hashes,
   and no-replay rules as the actual product, with CDP/Playwright as
   interchangeable transports.
