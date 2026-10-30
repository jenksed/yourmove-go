---
name: browser-workflow-testing
description: Choose and execute trustworthy synthetic tests for YourMove Next's React browser preview, Go-owned Playwright Chromium and direct CDP operations, or real Wails-to-browser workflows. Use when changing browser sessions, controls, bridge/CDP code, applicant continuation, or claiming browser behavior is verified.
---

# Browser workflow testing — YourMove Next

Test the **YourMove application path**, not an independently automated imitation of it. Select only the level of verification needed for the assigned change. This skill provides procedure, **not** permission to start a YMN task, open real applicant data, contact real employers, or execute consequential effects.

Read the current assignment and relevant sections of `AGENTS.md`, `CONTRIBUTING.md`, `docs/next/RELEASE_BOUNDARY.md`, `docs/next/TASK_INDEX.md`, and `docs/next/CHANGE_CONTROL.md`. For browser work, inspect the current source and `docs/next/YMN-003_EVIDENCE.md`; for employer continuation consult the YMN-012 scope in `TASK_INDEX.md`. Current source and acceptance records outrank stale status prose. Use Graft only as optional navigation, per `AGENTS.md`.

## Choose the right path

| Need to establish | Existing path | What it does **not** establish |
| --- | --- | --- |
| React layout, focus, recoverable drafts, visible errors | Component tests; `./scripts/dev-browser.sh` (synthetic in-memory preview) | Wails bindings, Go decisions, SQLite, employer effects. Native transport is deliberately blocked. |
| Go authorization, operation states, duplicate/uncertain outcomes | Focused Go tests under `internal/browserproof/` and owning feature packages | A real Chromium or native GUI run when using fake sessions. |
| Real Chromium control discovery and backend launch | Opt-in `TestRealSyntheticQuestionObservation`, both Playwright and CDP | File/answer transfer, native Wails integration, employer-site support. This test **observes** controls only. |
| Browser mutation, exact file/field receipt, or applicant-facing continuation | Explicitly authorized synthetic run through real Go-owned browser service; native Wails window for applicant-facing behavior | Release packaging or actual employer support beyond the tested control and backend. |

For a CSS or isolated component change, stop at component/preview checks unless a native claim is being made. For a Go authorization change, run the relevant policy and store tests. For changed selectors, bridge, CDP, uploads or continuation, exercise the real controlled browser path as well. Do not run a full ATS matrix for a narrow change.

## Existing entry points

- Frontend component tests: `npm --prefix frontend test -- BrowserProof.test.tsx` (or the specific affected component file).
- Focused Go tests: `go test -mod=readonly -race -count=1 ./internal/browserproof` (real-browser test skips by default).
- Opt-in Chromium observation: `YMN_REAL_BROWSER=1 go test -mod=readonly -count=1 -run '^TestRealSyntheticQuestionObservation$' -v ./internal/browserproof`. This requires the existing local Node/Playwright/Chromium setup; report unavailable prerequisites rather than claiming a PASS or silently substituting CDP.
- Visual preview: `./scripts/dev-browser.sh`, or set `YMN_BROWSER_PORT` if the default port is occupied. Preview is disposable and cannot make native calls.
- Real Wails dev: `./scripts/dev-native.sh` creates a **new** marked synthetic workspace. Record its exact path. For same-workspace restart follow `docs/next/LIVE_DEVELOPMENT.md`; launching the script again creates a different workspace. Never run concurrent writers on one root.
- Repository regression gate when the changed source warrants it: `./scripts/check.sh`; native build: `./scripts/build.sh`. Binding changes must come from Go generation, not hand-edited wire DTOs.

Look at `internal/browserproof/{service.go,service_test.go,questions_test.go,fixture.go,backend.go,cdp.go}`, `internal/browserproof/bridge/driver.mjs`, `browserproof.go`, and `frontend/src/components/{BrowserProof.tsx,BrowserProof.test.tsx}` before extending existing behavior. `frontend/src/preview/blocked-runtime.ts` explains why preview cannot qualify real effects.

## When an effect is in scope

Use only the owned synthetic fixture and an explicitly marked synthetic workspace. Test through **React → generated Wails binding → thin Go service → Go authorization/operation store → selected Playwright or CDP backend → observation/receipt**, using the shortest real path that supports the claim. Do **not** directly script form filling with an independent Playwright client and call that a YourMove transfer test.

Verify the properties implicated by the change:

1. **Before dispatch:** Go authorizes the exact destination, session, backend, control, saved material/revision and file digest as applicable. Stale or changed scope must refuse before mutation. UI consent alone proves nothing.
2. **Actual result:** Inspect the fixture's observed field values and file bytes/hash, and the Go operation receipt. Separate requested, dispatched, observed, partial, denied and unknown outcomes. Check the synthetic submit counter stays at zero for browser-assistance tests.
3. **No unintended changes:** Preserve prefilled/manual fields; do not transfer unchecked or unsaved material, automate consent/login/CAPTCHA, or claim a filled form was submitted.
4. **Interrupted effects:** A timeout, cancel-after-dispatch, bridge loss or browser crash can leave an **unknown** outcome. Observe/reconcile without replay. Never switch from Playwright to CDP (or vice versa) to retry an uncertain effect. A new session needs explicit reconciliation/acknowledgment and fresh Go authorization.
5. **Lifecycle when relevant:** Check owned-session/profile cleanup and same-workspace restart only if changed or required by the assigned acceptance. Do not infer a clean restart from a fresh fixture root.

For **YMN-012**, qualify only the assigned Ashby/Greenhouse/Lever provider/control subsets. Record Playwright and CDP findings separately; explicitly mark controls requiring manual completion. Source/posting retrieval is not evidence of browser filling. Do not create generalized browser tooling or provider fixtures before the actual slice needs them.

## Report, without inflating evidence

Report: tested source revision; exact command and result; environment (preview, fake-Go, real Go/Chromium, or Wails native); backend and synthetic fixture; observed vs expected field/file/result; relevant operation ID and receipt status; submission count if exercised; and **PASS / FAIL / NOT VERIFIED / NOT APPLICABLE** for each relevant claim. Distinguish owner-reported native observations from independently executed runs. Never label component mocks, Go fakes or browser preview as native acceptance.

No automatic push, merge, release, task acceptance or next-task start. Preserve unrelated worktrees and any running browser/native sessions; identify the owner before stopping a process. If the required real path cannot be exercised, report the specific missing evidence and stop rather than adding a new framework to conceal it.
