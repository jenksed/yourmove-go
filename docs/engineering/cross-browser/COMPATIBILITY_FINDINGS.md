# Compatibility findings — Chromium, Firefox, WebKit

Status vocabulary: **observed** (witnessed in an execution), **verified**
(reproducible test evidence), **inferred** (reasonable interpretation),
**unverified** (plausible, not exercised).

## CF-001 — No per-engine DOM workarounds needed for the supported form scope

- Engines: Chromium 141 (chromium-1248), Firefox Nightly (firefox-1509),
  WebKit (webkit-2248), all via `playwright-core@1.58.2`, macOS arm64.
- Expected: Firefox and WebKit would need quirk handling for at least file
  inputs, `setInputFiles`, or rerendered controls.
- Observed: the byte-identical shared scripts (`inspect.js`, contact
  resolver, answer/cover-letter/file-survey operations) behaved identically
  on all three engines across text inputs, textarea, select, checkbox, radio,
  file uploads, a 400ms rerender, multi-step navigation, and a same-origin
  iframe (recorded, not entered — same as CDP).
- Evidence: `TestCrossEngineObserveControlledForm`,
  `TestCrossEngineContactTransfer`, `TestCrossEngineResumeSelection`,
  `TestCrossEngineAnswerTransfer`, `TestCrossEngineCoverLetter`,
  `TestCrossEngineMultiStepNavigation` — 18/18 PASS.
- Status: verified (fixture scope only). Not a claim about arbitrary ATS
  pages, Safari, or authenticated flows.

## CF-002 — `crypto.subtle` is available on loopback in all three engines

- Expected: cover-letter digest verification might need a TLS fixture on
  some engine.
- Observed: `crypto.subtle` exists and `isSecureContext` is true on
  `http://127.0.0.1` in Chromium, Firefox, and WebKit alike. Cover-file
  digest verification passes everywhere with no TLS setup.
- Status: verified by direct probe. An early `verified=false` on
  Firefox/WebKit was our own composition bug (see journal 2026-10-27), not
  an engine gap.

## CF-003 — Launch latency differs structurally, per-operation cost does not

- Measured (single run, controlled fixture, headless): launch to settled
  form — CDP Chromium 0.3–0.6s, bridge Chromium 2.0s, Firefox 2.4s,
  WebKit 1.4s. Observe — CDP 3ms, bridge 4–15ms. Contacts (4 fields with
  verify settles) — 1.51s CDP vs 1.53s all bridge engines.
- Root cause (inferred): CDP returns at DevTools-port exposure, before the
  page loads; the bridge `launch` waits for `domcontentloaded`. Session
  startup cost only; steady-state operations are transport-indifferent.
- Status: observed. One run; treat as order-of-magnitude, not a benchmark.

## CF-004 — WebKit session RSS is highest of the three

- Measured (single run, `ps` group sum incl. helpers): Chromium 90MB,
  Firefox 85MB, WebKit 120MB, each including one ~50MB Node bridge process.
- Status: observed, single run. No optimization attempted; theoretical
  overhead until a real-workflow profile says otherwise.

## CF-005 — First WebKit launch paid install validation

- One `TestCrossEngineObserveControlledForm/webkit` run took 17s against a
  ~10s norm; subsequent runs matched the norm. Plausibly one-time
  `DEPENDENCIES_VALIDATED` work; not pursued.
- Status: unverified one-off, recorded so a repeat is recognized.
