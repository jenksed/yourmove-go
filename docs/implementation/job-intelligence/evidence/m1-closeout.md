# M1 Ashby Job Intelligence closeout evidence

**Final result: M1 PASS (see the final maintainer acceptance addendum).** Earlier PARTIAL entries below preserve the exact state and evidence at the time they were written; the final addendum records the maintainer's later acceptance and resolves those statuses for the bounded M1 capability. M2, M3 and M4 were not started at that point.

## Repository and source identity

| Item | Evidence |
| --- | --- |
| Branch / Git HEAD | `feature/ashby-provider-finalization` / `5da7ff214ae1e9ace05d47eed9183d37f7a8c754` |
| M0 shared source snapshot | `/private/tmp/yourmove-go-m1/m0-source.tar.gz`, SHA-256 `af8b78d129791ebeeb7ed083d97b78c6245df6fd6b4e5235541e6d3e9519f8c1` |
| M0 Ashby fixture | SHA-256 `008818ef01d7578e6f5b96a8c2ee0907d24ac63273d256bc0299019ee0842b0c` |
| JI-002 frozen contract | v2 snapshot SHA-256 `679e8315dbd69d618e69936e2d1d47d45ee4b9f52c434442f54cee38cafea5da`; contract manifest SHA-256 `4ae4a3cb2bce04e5ffb86d4dbf9141d0d2a4e819f4e2476724fbbdf9d820bf09` |
| Candidate used for maintainer report | `/private/tmp/yourmove-m1-manual-build/YourMove.app`; executable `/private/tmp/yourmove-m1-manual-build/YourMove.app/Contents/MacOS/YourMove`; executable SHA-256 `8dbac25b28f2db81f74d482ceb0c01a5f0a027668cb7e3c2835985c5eb84b5b2` |
| Candidate source manifest | SHA-256 `3209c103c82c1488cbae643671fcffdf51d839035302c4b2ce6ed6c1a0927549`; its recorded base was the Git HEAD above, with dirty source state. Independent comparison matched the source tree before the subsequent disclosure-group change. |
| Candidate build metadata | Go `go1.27.2`; `desktop,production`; Darwin arm64; CGO enabled; deployment target macOS 26.0; SDK 26.5. Ad-hoc code signature verification passed. |
| Current source state | Dirty, uncommitted tree based on the same HEAD. A current-source manifest can be generated with `python3 release/macos/source_manifest.py`; this report intentionally records the exact accepted candidate separately from the later tree. |

The original dirty `AGENTS.md` modification was preserved and not edited. Existing M0 artifacts and unrelated repository work were retained. No commit, merge, push, release, real applicant data, or employer action was performed.

## Swarm execution and integration

The orchestrator froze JI-002 before starting implementation lanes. Agents A, B and C worked concurrently in isolated directory copies synchronized from the validated M0 snapshot; Agent D reviewed independently and read-only. The implementation patches were integrated centrally rather than merged blindly.

| Lane | Ownership | Handoff / integration result |
| --- | --- | --- |
| Orchestrator | JI-002, JI-008–011, JI-014a, integration and closeout | Froze the versioned source contract; connected provider, parser and storage to transactions, reading view, template and lifecycle. |
| Agent A | JI-005, JI-006, JI-012 | Qualified Ashby identity, retained both source alternatives, mapped essential direct facts; provider and identity tests passed. |
| Agent B | JI-003 | Delivered the pure bounded document parser; unit tests and vet passed; bounded fuzzing completed 84,780 executions. |
| Agent C | JI-007 | Added latest-success source persistence and validation; migration/reopen, corruption and size tests plus vet passed. |
| Agent D | Independent adversarial review | Reported facts-only source loss, source BLOB read-before-bound and parser output-before-bound. Each was corrected with focused regression coverage; no material identity, canonical, salary-precedence, applicant-data, unsafe-markup or browser-authority finding remained. |

The final implementation is grouped in `internal/platforms/` (Ashby contracts/mapping), `internal/jobdocument/` (pure parser), `internal/storage/sqlite.go` and `internal/app/job_posting_sources.go` (persistence), `internal/app/` intake/refresh/deletion and reading-model files, and the existing `templates/job.html`, `static/styles.css` and `static/ui.js` Overview path. Qualification and regression tests remain beside their owning packages. No extra provider framework or new dependency was introduced.

## Frozen source contract and implementation

JI-002 keeps three identities separate: validated employer posting identity, deterministic digest of selected source content/facts, and exact canonical-description digest used by existing writing and matching workflows. The versioned Ashby source contains only provider evidence: validated posting identity, selected HTML and plain alternatives, direct location/workplace/employment facts, and the existing salary summary. Typed availability distinguishes rich source, canonical-only success, absent optional data, failed retrieval, and retained unconfirmed values. It carries no applicant data or authorization. Input, serialization, and parser work are bounded; unknown or corrupt source falls back to legacy job data.

The Ashby mapper selects one source-qualified board/posting record, rejects contradictory IDs/URLs, wrong-board associations and ambiguous matches, and retains the qualified `jobUrl` fallback when the JSON posting ID is absent. Existing plain-first canonical description selection and salary output remain unchanged. Direct location, workplace and employment fields are conservative; unknown provider values are not inferred. Application URLs remain informational and do not expand browser authority.

`internal/jobdocument` is a pure bounded parser using the existing HTML dependency. It emits owned structured data for headings, paragraphs, nested lists, emphasis and safe display-only links; malformed or unsafe content does not become executable markup or trigger remote fetches. One latest-success source observation is stored additively beside each job, with latest attempt kept separate. Intake, refresh and deletion couple source writes to existing transactions. Manual description and salary corrections remain authoritative; failed retrieval preserves earlier success, while a successful canonical-only response supersedes old rich availability honestly.

The app builds one reading view from the validated source and existing `Job`. Templates render only application-owned structures, retain full canonical text and its copy path, and show primary location, workplace arrangement, employment type, salary and provenance when available. No requirement sidecar, classifier, other ATS, new browser stack, or generic metadata framework was added.

Provider, parser, SQLite, lifecycle, reading-model, salary and canonical tests cover identity mismatch, optional-ID fallback, malformed/oversized input, digest stability/change, rollback, restart, corrupt/future source, manual precedence, canonical-only success and source freshness. The M0 canonical description SHA-256 remains `35c9e859b907f7e9043ba282564fe4d83b8da62c53e6c75b9eb06ab5735458c6`; salary remains `€120K - €150K`, origin `employer`.

The early M1 demo uses the same seeded canonical projection for the 1,848-byte stored description and clipboard output (`strings.TrimSpace`, 1,830 characters). The focused M0 Overview regression independently verifies its 1,310-byte clipboard content. HTML-only changes do not replace canonical evidence. No changed answer key, cover-letter evidence hash, resume matching input, saved applicant answer, or applicant artifact was observed in focused regressions.

## Controlled-browser qualification

The actual production intake, persistence, handlers, templates and scripts were exercised with a synthetic Ashby posting and controlled transport. No employer page was opened, no form was mutated, and transfer/submission counters stayed at zero.

**Focused M0/M1 command — PASS, exit 0:**

```sh
YOURMOVE_QUALIFY_BROWSER=1 \
YOURMOVE_CHROMIUM="/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary" \
YOURMOVE_QUALIFY_M0_OVERVIEW=1 \
YOURMOVE_M0_EVIDENCE_DIR=/private/tmp/yourmove-m1/final-m0-regression \
YOURMOVE_QUALIFY_M1_DEMO=1 \
YOURMOVE_M1_EVIDENCE_DIR="$PWD/docs/implementation/job-intelligence/evidence/m1-demo" \
GOCACHE=/private/tmp/yourmove-m1/go-cache \
go test -tags=qualification -count=1 \
  -run '^(TestJobOverviewM0ResponsiveCapture|TestJobIntelligenceM1ConnectedOverviewDemo)$' \
  ./internal/app -v
```

Controlled browser: Google Chrome Canary `158.0.8122.0`.

The M1 test passed production-path intake and Overview rendering; seven headings, eight paragraphs and three lists; safe same-host relative link and rejection of a foreign link; location `Remote - European Union`, remote, full-time and employer salary; latest-success label; exact canonical copy selection; edit then refresh with applicant text preserved and updated source retained for comparison; and no-JavaScript, keyboard disclosure and selectable full-text fallback. It captured 1280×900 and 390×900 in light and dark appearances and asserted no horizontal overflow. Raster captures are retained in the local review archive and excluded from the source PR.

Screenshot SHA-256 values (light/dark at each width):

| Capture | SHA-256 |
| --- | --- |
| 1280×900 light | `25754931f122a95dc3c75080aa405c43f7e9fb3266a1f53e9d0024da51dfe132` |
| 1280×900 dark | `055d479a464b67d67d62f381d1a1f5b43ab2d17871c34b00b1572e5e7985f115` |
| 390×900 light | `f1fc7e70ee9f13ada474c6d6e0ae6f49b329e5383d84453e2cbceedc6389ecf1` |
| 390×900 dark | `64f24a3a52c75f56bd886e8d966c5e78fc6ed8885c8fa9aac392f1b7856ffb45` |

The M0 test passed 1280×900 and 390×900 captures and clicked the actual Overview Copy control, confirming all 1,310 expected canonical bytes and its success status.

**Broader opt-in app qualification — FAIL, exit 1:**

```sh
YOURMOVE_QUALIFY_BROWSER=1 \
YOURMOVE_QUALIFY_M0_OVERVIEW=1 \
YOURMOVE_M0_EVIDENCE_DIR=/private/tmp/yourmove-m1/final-m0-regression \
YOURMOVE_QUALIFY_M1_DEMO=1 \
YOURMOVE_M1_EVIDENCE_DIR="$PWD/docs/implementation/job-intelligence/evidence/m1-demo" \
GOCACHE=/private/tmp/yourmove-m1/go-cache \
go test -tags=qualification -count=1 -v ./internal/app
```

That broad run did not explicitly set `YOURMOVE_CHROMIUM`; the focused M0/M1 pass was rerun with the Chrome Canary path and version recorded above.

The M1 app-path intake and refresh checks used the fixed synthetic Ashby transport. No fresh live Ashby API request was made during this qualification; provider conclusions are limited to the retained M0 observations and contract fixtures.

Two broader writing-workbench tests remain unresolved:

- `TestControlledNarrativeAnswerWorkbench`, `internal/app/application_answer_ai_qualification_test.go:394`: after the test switches between the answer and letter activities and attempts keyboard adoption, the answer editor does not reach the expected `EXPLAIN` text. The earlier mutual-exclusion activity assertions now pass.
- `TestControlledScopeFirstWorkbench`, `internal/app/cover_letter_workbench_qualification_test.go:165`: after saving a locally edited letter, its combined assertion for `#prepare`, a closed letter editor and the exact saved transfer-scope text fails. The test does not currently identify which conjunct failed.

These tests are outside the focused Ashby Overview gate, but they exercise the shared job writing surface and remain compatibility risks. Other qualification cases controlled by dedicated opt-in flags were skipped; a skip is not a PASS. The focused M1/M0 tests above passed within the same broad run.

## Native Wails acceptance

**Maintainer-reported result: PASS.** The maintainer reported manually testing the candidate app bundle listed above with isolated workspace `/private/tmp/yourmove-m1-manual`, exercising Job Intelligence and persistence across restart. This status is recorded as reported; the agent did not perform native interaction. The bundle signature, executable hash, build metadata and source-manifest identity were independently checked. No workspace database was inspected or changed.

The report did not individually enumerate these JI-024a checks, so this evidence does not claim them separately: full process exit and reuse of the exact executable/workspace without reseeding; exact clipboard payload; safe external-link behavior and focus return; Wails navigation/redirects; 700×550 and normal window sizes; light/dark, keyboard and focus details; VoiceOver, effective zoom/text scaling or native overflow; persistence of each named item (source, manual description, salary state, saved answers, resume assignment, cover letters and history); or absence of restored browser-transfer authority.

After the maintainer-tested candidate was built, the current source added `name="job-writing-activity"` to the answer and cover-letter `<details>` elements and updated the corresponding markup assertion. This bounded shared-template behavior change makes those two activities mutually exclusive and allows their switch assertions to pass, but it means the current tree is not the exact manually tested source. JI-024a for the current source is therefore **NOT VERIFIED** pending a rebuild and maintainer recheck. The earlier maintainer-reported PASS remains attached to the exact candidate and paths above.

## Final verification ledger

| Check | Result | Notes |
| --- | --- | --- |
| `GOCACHE=/private/tmp/yourmove-m1/go-cache go test -count=1 ./...` | PASS, exit 0 | All packages, including app, Ashby providers, parser and storage. |
| `GOCACHE=/private/tmp/yourmove-m1/go-cache go test -tags=desktop -count=1 ./...` | PASS, exit 0 | Desktop-tagged app and all packages. |
| `GOCACHE=/private/tmp/yourmove-m1/go-cache go vet ./...` | PASS, exit 0 | Uses isolated Go cache. |
| `node --check static/ui.js` | PASS, exit 0 | No JavaScript syntax issue. |
| `git diff --check` | PASS, exit 0 | No whitespace errors. |
| Focused M0/M1 controlled Chromium command above | PASS, exit 0 | Production path; synthetic data; four M1 captures. |
| Full opt-in app controlled Chromium command above | FAIL, exit 1 | Two failures named above; M1/M0 focused tests pass in the same run. |
| Current-source actual Wails build and interaction | NOT VERIFIED | Maintainer's report applies to the prior exact candidate; current source changed afterward. |

No new M2/M3/M4 task was started. The capability is useful and demonstrable through the controlled app, but this closeout remains **M1 PARTIAL** until current-source native reacceptance and explicit disposition of the two broader controlled failures.

## Final regression resolution — 2026-10-24

This dated addendum updates the earlier failure disposition while preserving the recorded failures and prior maintainer-reported native result above.

### Starting source and integration

| Item | Value |
| --- | --- |
| Branch / Git HEAD | `feature/ashby-provider-finalization` / `5da7ff214ae1e9ace05d47eed9183d37f7a8c754` |
| Initial source snapshot | `/private/tmp/yourmove-m1-source-snapshot-20261024/source.tar`; SHA-256 of the complete tracked + nonignored untracked path/content manifest: `d2089ad8fc939f6d8ad15d997dccbfa84ef93bd07ab2d9ce748160e2f7c9c491` |
| Initial preserved worktree | Existing modified `AGENTS.md`, M0/M1 implementation and docs, fixtures and qualification tests were included. `AGENTS.md` was not edited. No reset, clean, stash, commit, merge, push, or release occurred. |
| Ownership | Agent A diagnosed the narrative test; Agent B diagnosed scope-first; Agent C prepared native acceptance; Agent D reviewed read-only. I integrated both test-only corrections centrally. Agents worked from isolated copies/read-only review; no competing source edits were integrated. |

### Failure classification and correction

**`TestControlledNarrativeAnswerWorkbench` — test fixture/navigation defect.** The test selected the visible Prepare tab but then operated on controls inside the hidden `#answers` activity. In three individual reproductions the enabled adoption button had a zero-sized rectangle, focus remained on `BODY`, Space generated no click, and the editor retained its manually entered value while the preview held the expected `EXPLAIN` text. The corrected test follows the visible `[data-prepare-question-entry]` link and waits for the Answers activity to render before the keyboard interaction. Space then adopts the exact generated sentence; the answer remains unsaved at revision 0 until the explicit Save action. The final database assertion now checks the exact adopted sentence plus the user's edit.

**`TestControlledScopeFirstWorkbench` — test fixture/navigation defect.** The test found the cover-letter textarea in the DOM while its `#cover-letter` activity was hidden. Calling `focus()` did not move focus from `#prepare`, and Chrome ignored `Input.insertText`; the unchanged 185-word fixture text was saved. The corrected test selects the visible Cover letter tab, waits for the textarea to have visible bounds, verifies focus and exact inserted text, then checks Prepare navigation, closed disclosure, exact saved transfer scope, exact SQLite draft body, and zero employer-browser actions separately.

Neither investigation demonstrated a product keyboard, persistence, transfer, or authorization defect. No production files changed for these failures. The `name="job-writing-activity"` native disclosure group remains; it supplies the mutual-exclusion behavior separately covered by the test. The previous candidate manifest (`/private/tmp/yourmove-m1-manual-build/source-manifest.json`, SHA-256 `3209c103c82c1488cbae643671fcffdf51d839035302c4b2ce6ed6c1a0927549`) has hashes for files but not prior template contents. The unchanged test cannot reach the reported assertion with the old template because it fails earlier at the exclusive-disclosure expectation. Therefore, historical reproduction against that candidate is **not equivalent**, and no introduced/pre-existing product claim is made.

The independent review found no reason to change production behavior. It recommended exact narrative adoption/persistence assertions, separate scope-first outcomes, and a pinned browser. All were addressed without relaxing acceptance conditions.

### Final automated qualification

Browser: `/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary`, version `158.0.8122.0`. Controlled browser runs used synthetic local fixtures. The test-only Chromium actions did not open an employer form or submit an application.

| Qualification | Result |
| --- | --- |
| `TestControlledNarrativeAnswerWorkbench`, individual `-count=1 -v`, 3 runs | PASS 3/3; exact Space adoption, dirty/revision state, save and exact persisted value asserted. |
| `TestControlledScopeFirstWorkbench`, individual `-count=1 -v`, 3 runs | PASS 3/3; visible-editor input, exact persisted transfer text, Prepare return, closed editor, and zero employer actions asserted. |
| Focused `TestJobOverviewM0ResponsiveCapture` + `TestJobIntelligenceM1ConnectedOverviewDemo` | PASS; M0 copied exactly 1,310 canonical bytes; M1 retained 1,848 canonical bytes and copied 1,830 trimmed characters, rendered 7 headings/8 paragraphs/3 lists and the direct facts, preserved the manual edit across refresh, and recorded zero submissions. Captures were written to isolated `/private/tmp/yourmove-m1-final-*` directories. |
| Original broader opt-in `./internal/app` qualification, pinned Chrome | PASS, Go output `ok` in 70.747s; 10 separately gated tests skipped and remain NOT VERIFIED; no failures. Skipped cases: Ashby preview read-only/cancellation, application brief clipboard, early preparation, opportunity queue, Prepare entry points, product experience, question workbench, native fixture seeding, and unapproved-profile journey. |
| `GOCACHE=/private/tmp/yourmove-m1-go-cache go test -count=1 ./...` | PASS, exit 0. |
| `GOCACHE=/private/tmp/yourmove-m1-go-cache go test -tags=desktop -count=1 ./...` | PASS, exit 0. |
| `GOCACHE=/private/tmp/yourmove-m1-go-cache go vet ./...` | PASS, exit 0. |
| `node --check static/ui.js` | PASS, exit 0. |
| `git diff --check` | PASS, exit 0. |

The broad test itself completed with `PASS` and `ok`; the shell wrapper then reported zsh's reserved read-only variable `status` while formatting its summary. The test log was checked directly: 10 skips, no `FAIL` lines. This wrapper issue did not change the Go test result.

### Current-source native candidate and handoff

The final candidate is built from this closeout state at `/private/tmp/yourmove-m1-final-candidate/YourMove.app` with:

```sh
GOCACHE=/private/tmp/yourmove-m1-go-cache \
  bash release/macos/build.sh /private/tmp/yourmove-m1-final-candidate
```

The reproducible current-source build produced executable SHA-256 `3d8518bb8926c0dd28c9c5d0dd89b6f317d7234d130d45e9b83f5184cbf58717`. The final build is required to match this identity. Host: macOS 26.5.2 (25F84), arm64. Toolchain: Go 1.27.2, Apple clang 21.0.0, SDK 26.5, deployment target macOS 26.0. `codesign --verify --strict` passed with an ad-hoc signature; the app is not notarized or distributable. The build output retains `source-manifest.json`, `source-manifest-after.json`, `SHA256SUMS.local`, `toolchain.txt`, `build-info.txt` and `deployment-target.txt`. The source manifest includes this evidence and records the exact dirty source state at build time.

**Native interaction: NOT VERIFIED.** This session has no local Computer Use `node_repl` interface, so no Wails window interaction was performed. Build, signature validation, process startup and database-path verification do not substitute for JI-024a. The earlier native acceptance remains attached only to `/private/tmp/yourmove-m1-manual-build/YourMove.app` and `/private/tmp/yourmove-m1-manual`.

The isolated workspace `/private/tmp/yourmove-m1-final-workspace` already exists from the process/data-root check and contains an initialized empty database. Reuse it as-is for maintainer acceptance; do not recreate or reseed it. Launch and same-workspace restart command:

```sh
cd /private/tmp/yourmove-m1-final-workspace
YOURMOVE_DATA_DIR=/private/tmp/yourmove-m1-final-workspace \
  /private/tmp/yourmove-m1-final-candidate/YourMove.app/Contents/MacOS/YourMove
```

Use a public Ashby posting only for reading its Overview; do not open its application form. Use synthetic manual description edits and synthetic applicant text. Do not use production YourMove data or real applicant materials. Before relaunch, quit the app fully and confirm its process exited; then repeat the same executable command from the same workspace without reseeding. Confirm `lsof -p <pid>` lists `jobs.db` only under this workspace.

Record PASS/FAIL/NOT VERIFIED against the executable SHA-256 for: HTML structure and the four facts; actual canonical clipboard contents; manual edit/save/refresh and freshness/source state; safe link behavior and focus return; Prepare/workbench navigation and keyboard/disclosure state; 700×550 and normal sizes; both appearances; accessible text/keyboard focus/zoom where practical; same-executable/same-workspace restart; retained source/manual/applicant artifacts; and no restored browser-transfer authority. Do not open an employer form, transfer materials, or submit anything. The maintainer must provide the observed results to close JI-024a.

### Acceptance dispositions

- **JI-023a: PASS** for the current controlled production app paths and M1 scope; see the focused and broad results above. The 10 separately gated cases remain NOT VERIFIED and do not expand this disposition.
- **JI-024a: PARTIAL / NOT VERIFIED.** A fresh current-source Wails candidate is built and identified, but native interaction and restart acceptance require maintainer execution against the exact executable/workspace.
- **JI-025a: PARTIAL.** The two reported browser failures are resolved as test setup defects and corrected assertions pass repeatedly. The capability remains partial solely because JI-024a native acceptance is outstanding.

**Final status: M1 PARTIAL — working capability with the specific current-candidate native acceptance gap above.** M2, M3 and M4 remain unstarted.

<a id="m1-voiceover"></a>

## Final VoiceOver qualification — 2026-10-24

This addendum supersedes the candidate identity and current-source focus disposition above. Earlier build and maintainer reports remain recorded as historical evidence.

### Exact current-source candidate and fixture

| Item | Evidence |
| --- | --- |
| Branch / Git HEAD | `feature/ashby-provider-finalization` / `5da7ff214ae1e9ace05d47eed9183d37f7a8c754` |
| Candidate app | `/private/tmp/yourmove-m1-voiceover-candidate-kQ4jkz/build/YourMove.app` |
| Executable SHA-256 | `3a45473bad2dd6afbdacf004ca219f59a7fe623b63e12ac945e34ee790e28255` |
| Build-time source-manifest SHA-256 | `e16d3020b08288980c08a75ecc5c955e8025942c33c3f8c4112450d634fdf9b6` |
| Build-time focus source hashes | `static/ui.js`: `1929636bfc0c1b5cade2399ea8f2b84a0e2edbf6856837a25821655c95879886`; `templates/job.html`: `043da2191a65002bb5ca96e07d019afd46179bbd63c37c49e99290c3ceee343c` |
| Preserved `AGENTS.md` hash | `2fefb7c2cafc0c78632278a5d2a493472c5660bce0d4ed2b7c2132c41cf161a2` |
| Host / build | macOS 26.5.2 (25F84), arm64; Go 1.27.2, Apple clang 21.0.0, SDK 26.5, deployment target macOS 26.0 |
| Signature | Ad-hoc signature; `codesign --verify --strict` PASS. Not notarized or distributable. |
| Synthetic fixture workspace | `/private/tmp/yourmove-workflow-native-gb1eMJ` |

The earlier candidate's manifest did not match the current focus source. A fresh local candidate was built with `release/macos/build.sh`; its before/after source manifests matched. `TestPrepareNativeWorkflowFixture` seeded the new workspace once and passed. It contains a synthetic Ashby job, a synthetic inspected question and a synthetic saved cover letter. The Wails candidate was not launched in this session; no application save was made in this workspace, no real workspace was opened, and no employer form, transfer or submission action occurred.

The source manifest records the exact dirty worktree at build time. The closeout, implementation-plan and backlog status edits in this addendum were made afterward and are documentation-only; no executable source or bundled asset changed after the build.

### Automated focus and keyboard results

The current-source controlled Chromium run below passed both focused workflows using Google Chrome Canary 158.0.8122.0:

```sh
YOURMOVE_QUALIFY_BROWSER=1 \
YOURMOVE_CHROMIUM="/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary" \
GOCACHE=/private/tmp/yourmove-m1-voiceover-go-cache \
go test -tags=qualification -count=1 \
  -run '^(TestControlledNarrativeAnswerWorkbench|TestControlledScopeFirstWorkbench)$' \
  ./internal/app -v
```

Exit 0. `TestControlledScopeFirstWorkbench` focused the visible `Saved locally` target after cover-letter save (viewport position 758.4–774.4 of 818 px). `TestControlledNarrativeAnswerWorkbench` focused the selected question's `Answer saved locally. It has not been filled on the employer website.` target (791.9–806.9 of 818 px). The tests also exercised keyboard save/navigation, the letter and resume disclosures, narrow light/dark layouts and the long keyboard-scrollable scope. These are controlled Chromium results; they do not establish VoiceOver speech or native keyboard behavior.

The template targets remain `role="status"` elements with a child `tabindex="-1"` target: the cover-letter target reads `Saved locally`; the answer targets use the selected question's `LocalStatus`. `static/ui.js` focuses these targets on save return and scrolls the selected status into view. The maintainer reports PASS for the latest native Wails focus behavior, as supplied for this task; no executable hash accompanied that report, so it is not independently tied to the candidate above.

### VoiceOver capability and speech result

VoiceOver 10 is installed at `/System/Library/CoreServices/VoiceOver.app`. Its `Contents/Resources/VoiceOver.sdef` dictionary is present and declares a `last phrase` object with readable `content`, plus an `announcement history` outputable. The `sdef` command could not run because this host has Command Line Tools active without full Xcode; the dictionary was inspected directly.

Actual speech capture was unavailable in this session. JXA reported `VoiceOver.running=false` and `AXIsProcessTrusted=false`; the System Events Accessibility query failed with OS error `-10827`; launching the VoiceOver bundle failed with `kLSNoExecutableErr` (`-10827`); and reading `last phrase.content` returned a missing-parameter error while VoiceOver was stopped. The TCC database was not readable by this process. No VoiceOver output or scripted speech command was used as a substitute for an application announcement.

| Workflow / observation | Result |
| --- | --- |
| Cover-letter status focus | PASS in current-source controlled Chromium; maintainer-reported PASS in native Wails. |
| Cover-letter VoiceOver announcement after save | **NOT VERIFIED** — no VoiceOver runtime or speech output was available. |
| Selected answer status focus | PASS in current-source controlled Chromium; maintainer-reported PASS in native Wails. |
| Answer-save VoiceOver announcement | **NOT VERIFIED** — no VoiceOver runtime or speech output was available. |
| Duplicate, missing or misleading VoiceOver announcement | **NOT VERIFIED**. |
| Keyboard navigation | PASS in the focused controlled Chromium workflows; current-candidate native keyboard navigation was not observed here. |
| Focus returning to oversized content scope | PASS in the focused controlled Chromium workflows; maintainer-reported native focus PASS has no executable identity in the supplied report. |
| Save behavior / employer actions | No product behavior changed. Only the synthetic fixture was seeded; no employer form, transfer or submission action occurred. |

### Minimal maintainer VoiceOver check

Use the exact candidate and already-seeded synthetic workspace above, with VoiceOver active:

```sh
YOURMOVE_DATA_DIR=/private/tmp/yourmove-workflow-native-gb1eMJ \
  /private/tmp/yourmove-m1-voiceover-candidate-kQ4jkz/build/YourMove.app/Contents/MacOS/YourMove
```

1. Open the synthetic job **Synthetic native workflow**. In Cover letter, edit the existing synthetic letter and choose its local Save action. Record the exact VoiceOver utterance and whether it occurs once; the visible target should read **Saved locally**.
2. In Answers, edit the synthetic **Why this team?** answer and choose its local Save action. Record the exact utterance and whether it occurs once; confirm it describes a locally saved answer and says nothing was filled on the employer website.
3. Use Tab and Shift+Tab after each save. Confirm focus stays at the relevant status and keyboard navigation continues through the workspace instead of returning to the oversized Prepare container.
4. Stop after local saves. Do not open the employer form or choose any transfer or submit action.

The maintainer-observed speech result has not been supplied. Record the exact phrase and duplicate count separately for each workflow before changing the NOT VERIFIED entries above.

### M1 acceptance disposition

- **JI-023a: PASS.** Existing focused production app-path qualification remains accepted.
- **JI-024a: PARTIAL / NOT VERIFIED.** The exact current-source Wails candidate is built and identified, but its native workflows and restart were not exercised here. The supplied native focus PASS has no executable identity; both VoiceOver save announcements and duplicate-announcement behavior remain NOT VERIFIED.
- **JI-025a: PARTIAL.** Current-source controlled focus workflows pass. The remaining JI-024a native acceptance evidence prevents closeout.

**Final status: M1 PARTIAL — Ashby Job Intelligence is not yet accepted.** Exact current-candidate native interaction and maintainer-observed VoiceOver save announcements are outstanding. M2, M3 and M4 remain unstarted.

## Maintainer follow-up — VoiceOver spike deferred

The maintainer reports opening VoiceOver Utility, but VoiceOver was not working when using YourMove. No exact save action, spoken phrase, duplicate count, or VoiceOver focus observation was supplied. Treat this as a maintainer-reported unresolved result; it does not establish either a pass or a YourMove defect.

Further VoiceOver qualification is tabled for the deferred accessibility spike **YM-017** in `docs/BACKLOG.md`. JI-024a and JI-025a remain PARTIAL, and M1 remains **PARTIAL**. No additional app interaction or implementation was performed after this report.

## Maintainer acceptance — M1 closeout — 2026-10-24

This is the final M1 disposition and supersedes the preceding PARTIAL status statements. The maintainer explicitly accepts the M1 Ashby Job Intelligence scope. The acceptance covers the working intake-to-Overview capability, source/fact reading, applicant-state preservation, same-workspace persistence across restart, and the refined native focus behavior that returns focus to the saved-status element instead of outlining the whole application-action container.

### Native evidence and identity

| Accepted behavior | Evidence and identity | Limit |
| --- | --- | --- |
| Native Wails Job Intelligence interaction and persistence across restart | Maintainer-reported PASS for `/private/tmp/yourmove-m1-manual-build/YourMove.app` with isolated workspace `/private/tmp/yourmove-m1-manual`; executable SHA-256 `8dbac25b28f2db81f74d482ceb0c01a5f0a027668cb7e3c2835985c5eb84b5b2`; source-manifest SHA-256 `3209c103c82c1488cbae643671fcffdf51d839035302c4b2ce6ed6c1a0927549`. | The report establishes the accepted core interaction and restart. It did not enumerate every native detail in the extended qualification checklist. The candidate predates the later mutually exclusive disclosure markup and refined focus source. |
| Refined saved-status focus in native Wails | Maintainer-reported PASS for the latest focus behavior: focus returns to the saved-status element rather than the application-action container. | The report supplied no executable or source-manifest identity, so it is not attributed to the earlier executable above or to the locally built `3a45473b…` candidate. Current-source controlled Chromium focus/keyboard checks are separately recorded above. |

The native acceptance corresponds to the capability being closed: an applicant can read and preserve the job, perform local edits, and retain the accepted state across restart, with the refined focus behavior reported as working in Wails. This is maintainer-reported native acceptance, not an agent-performed native run. Historical evidence and the earlier executable identity remain intact.

### Results and boundaries

- **JI-023a: PASS.** Focused M0/M1 production-path qualification and broader controlled Chromium qualification passed; ten separately gated workflows remain NOT VERIFIED and are outside this disposition.
- **JI-024a: PASS for the accepted M1 native reading/persistence scope.** The maintainer reports native interaction and same-workspace restart PASS for the exact identified candidate above, and separately reports the refined native focus behavior PASS. No Chromium or Go result is substituted for native evidence.
- **JI-025a: PASS.** The M1 scope, existing automated evidence, native maintainer acceptance, and remaining qualifications are now dispositioned.
- **VoiceOver announcements, duplicate-announcement behavior, and comprehensive accessibility audit: NOT VERIFIED.** VoiceOver investigation remains deferred as **YM-017** and does not block M1.
- Individually unenumerated native checks—including exact clipboard payload, the complete viewport/theme matrix, effective zoom/text scaling, every persisted artifact, and restored browser-transfer authority—remain **NOT VERIFIED**. Their status is not promoted by the capability-level maintainer acceptance.
- The broader historical qualification failures were resolved as test navigation/setup defects with exact assertions retained; the corrected tests passed three runs each and the pinned broader controlled suite passed. Historical failed-run records remain unchanged.
- This acceptance does not establish release readiness or qualify M2, M3, M4, Paylocity, or browser transfer/submission behavior.

**Final status: M1 PASS.** VoiceOver remains deferred under YM-017. M2 focused reading, M3 provider portability, and the independent JI-019a label correction retain their separate acceptance gates.
