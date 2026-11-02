# Engineering journal — cross-browser automation

## 2026-10-27 — Bridge composition contract (playwright driver)

- Engine/version/OS: Chromium 141 (chromium-1248), Firefox Nightly (firefox-1509), WebKit (webkit-2248) via `playwright-core@1.58.2`, macOS arm64, Node v26.3.0.
- Attempted: reuse the proven CDP page scripts verbatim through `page.evaluate` by wrapping caller-supplied sources as `(SRC)(...args)`.
- What happened: three distinct composition failures, all ours, none upstream:
  1. `PLAYWRIGHT_BROWSERS_PATH` is a **single directory**, not a list. Joining the macOS and XDG cache paths produced a bogus combined path and `Executable doesn't exist`. Fix: pick the first existing root; operator override passes through untouched.
  2. Wrapping an IIFE that ends with `;` in extra parens (`(()=>(...);)()`) is a **parse error** (`missing ) in parenthetical`). Fix: `asFunction` trims the trailing semicolon and wraps as `()=>{return (IIFE)}`.
  3. `String.replace` with a `\$` replacement doubles the backslash (`'$1'` regex anchors became literal backslashes), and template-literal composition re-parses escapes. Fix: compose the call by plain concatenation; no escaping needed.
  4. `coverVerify` returned `()=>(async… )()` — a function-returning-function that serializes to `undefined` instead of the verification object (observed as `verified=false` on Firefox/WebKit while Chromium CDP passed, since CDP evaluates the raw IIFE). Fix: pass the raw async IIFE and wrap exactly once.
- Verification: `TestCrossEngineCoverLetter` green on all three engines after the fix.
- Remaining uncertainty: none on this contract; rule documented in `ARCHITECTURE_DECISIONS.md` (single DOM source, transport-only difference).

## 2026-10-27 — `crypto.subtle` on loopback is universal (verified)

- Probed directly: `crypto.subtle` exists and `isSecureContext` is true on `http://127.0.0.1` in all three Playwright engines. Cover-letter digest verification therefore works everywhere without a TLS fixture.
- Status: observed + verified.

## 2026-10-27 — Driver performance vs CDP baseline (measured)

- Method: controlled loopback fixture, headless, macOS arm64. Launch timed to
  first settled form (10+ observed controls); contacts = 4 approved fields
  with the established 120ms + 250ms verify settles. One run each, same
  machine, warm browser cache.
- Results:
  - Launch: CDP Chromium 0.3–0.6s; bridge Chromium 2.0s, Firefox 2.4s,
    WebKit 1.4s. The gap is structural: CDP returns at DevTools-port
    exposure (the page may still be blank; the watcher settles later),
    while the bridge `launch` waits for `domcontentloaded`. One-time per
    session; no per-operation impact.
  - Observe: CDP 3ms; bridge 4–15ms per engine (one extra JSON-RPC hop).
    Negligible against the 2.2s watcher cadence.
  - Contacts: 1.51s (CDP) vs 1.53s (all three bridge engines). Identical:
    dominated by the intentional verify settles, not the transport.
  - Session group RSS (bridge + browser + helpers): Chromium 90MB,
    Firefox 85MB, WebKit 120MB. CDP browser RSS not measured in this run;
    the bridge adds one Node process (~50MB of the totals).
- Status: observed, single-run. No material per-operation regression; launch
  latency is the only structural cost and is session-amortized.

## 2026-10-27 — No-silent-switch enforcement belongs at the open endpoint

- Attempted: `sessionHost.start` returned an error when the requested engine
  differed from the active session's engine.
- What happened: the resume handoff (which carries no engine choice and
  starts with the Chromium default) tripped the guard on an active Firefox
  session, and the generic launch-failure path replaced the live session's
  state display with an interrupted notice. The session itself was fine;
  the error handling lied about it.
- Fix: `start` keeps an active session unconditionally; the open endpoint
  (`browser/start`) rejects an explicitly different engine with a clean
  409 before launching; transfer commands without an engine choice continue
  on the active session. Verified by the app-workflow test's 409 case.
- Status: verified.

## 2026-10-27 — Wails smoke: launch PASS, identity gate blocked on branding drift

- Built, ad-hoc-signed, and launched the desktop app with the bridge code
  included: process started, served `/jobs`, wrote its identity record,
  templates (incl. the engine selector) parsed at startup. No crash.
- Blockers (both pre-existing, out of scope): `native-candidate.py`
  requires a `/private/tmp/yourmove-native-` workspace while the seed test
  requires `/private/tmp/yourmove-workflow-native-`; and the harness
  expects window title `YourMove · …` while this branch titles
  `YourMove Hackathon Edition · …`. Native interaction remains
  NOT VERIFIED, same as before this initiative.
- Status: observed. Smoke proves startup + template validity, not desktop
  acceptance.

## 2026-10-27 — Distribution: headless-shell revision must ship in the bundle

- First `stage-playwright.sh` bundle staged 4 browser dirs (chromium, firefox,
  webkit, ffmpeg). Headless bridge launch failed on bundled bytes only:
  playwright-core resolves `chromium_headless_shell-1208` for headless
  sessions, which was absent from the bundle (dev cache masked this).
- Fix: stage `chromium_headless_shell-1208` too (5 dirs in the manifest);
  `setup-playwright-bundle.sh` installs it. Headless launch+close then
  passed on all three engines with `env -i` (no dev node/cache/PATH).
- Adjacent lesson: immediate post-launch observation races client hydration
  (0 controls on live Ashby/Greenhouse); a bounded read-only poll for DOM
  readiness precedes the shared `inspect.js` projection. No page interaction
  was used on live sites; mutations stay on the controlled fixture.
- Orphan audit: every surviving Playwright browser process parented to the
  gstack QA daemon (by design); zero YourMove orphans after graceful close
  (close op + stdio half-close + natural driver exit, the `Session.Close`
  sequence). An early SIGKILL-the-driver probe artifact was misread as a
  product leak once; process-parentage check (`PPID` chain) is the screen.
- Status: verified on `/private/tmp/ym-vertical-slice-001` (1.2G,
  core 1.58.2, node v22.23.4).
