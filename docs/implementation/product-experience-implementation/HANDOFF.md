# Product experience implementation handoff

October 23, 2026. Branch: `feat/phase2-product-experience`. Target: `main`.

Production implementation spans all eight increments. **Native workflow acceptance remains incomplete; this is not release acceptance.** Review the separate [acceptance matrix](ACCEPTANCE.md) before merging. No employer application was submitted, no provider generation was authorized during qualification, and no applicant workspace was used.

## Baseline and working boundaries

Started clean at `6709840d70e84e1df0d66425479c1bf3ac17574a`, the PR #6 merge. History already included `74b7b05`, `4619551`, and `8d71fde`; none was reapplied. Used the existing dedicated worktree. The original checkout's `AGENTS.md` and `opencode.json` were not changed, stashed, restored, or copied. Git operations necessarily update shared linked-worktree metadata.

Read the approved design package and Phase 1 handoff. The supplied mandate supersedes the historical prototype's increment-by-increment approval policy. `graft map` could not run (`graft` is not installed and no graph directory is checked in); direct repository inspection was used. Historical research and stabilization documents are unchanged.

## Implementation by increment

| Increment | Production behavior | Qualification / limits |
|---|---|---|
| 1. Shell and Opportunities | 184px grouped labeled navigation; Work, Your materials, Preferences/tools; named compact Navigation below 820 CSS px; Escape and focus return; short-window sidebar scrolling; compact counters and optional progress details; shorter suggested opportunity. | Same-fixture baseline/after at 900×700; real row appears in the first viewport. Existing filtering, sorting, recommendation and deletion semantics retained. Native dimensions remain unmeasured. |
| 2. Job and writing | Overview, Prepare, Answers, Cover letter, History; existing editors moved into direct activities; bounded writing; optional Focus writing / Show navigation; activity history and deep links; saved disconnected-browser writing leads when no early inventory exists. | Same DOM editor survives activity switches. Existing save/revision/conflict and application-owned departure dialog remain. Query-bound redirects and direct fragments are covered separately. |
| 3. Preparation / review | Resume selection precedes exact transfer review. Independent profile exclusion, saved values, review hashes and existing consent forms remain. Outcomes stay per material/field. Disconnected recovery leads with one no-fill action; prior session metadata is collapsed; empty results and duplicate letter/contact copy are omitted. | Required controlled transfer journeys cover one resume upload, saved-answer eligibility, manual-value preservation and zero employer submissions. Native employer transfer remains NOT VERIFIED. |
| 4. Documents / families | Filename, version, readable size and download lead; existing Manage document disclosure retained; Unicode filename evidence; family copy explains suggestions and explicit job selection. | Existing byte/revision/archival/association tests retained. Native picker/save/cancel and exact exported bytes remain NOT VERIFIED. |
| 5. Reusable materials | Library entries lead with the actual question and saved answer; source context stays secondary. Career Evidence keeps situation/actions/outcome prominent, with optional contextual fields disclosed. | Snapshot, revision, unknown-outcome and unsaved insertion behavior retained. Existing contextual Help supplies previous writing, reviewed evidence and separate AI proposals. |
| 6. Profile / AI | Applicant approval replaces verification claims; approval is explicitly fresh at each save; pronouns labeled optional. Optional AI is grouped by local vs external/custom access; setup is secondary; configuration is not guaranteed access. | No credential storage, discovery, fallback, autosave or mandatory onboarding. Existing generation review/authorization/adoption/save boundaries retained. |
| 7. Completion / recovery | Menu and page renamed **Submissions** at the user's request; `/applications` retained. Recorded dates come from SQLite. History says “Submission recorded locally” and retains employer-receipt caveat and Next consequences. Refresh/recovery notices remain visible across activities. | Existing tokens, idempotency, pinned resume history and browser ownership retained. Restart persistence is covered by existing storage tests; full native restart journey remains NOT VERIFIED. |
| 8. Consistency / accessibility | Shared palette and focus tokens; wrapping activities; compact named navigation; reduced-motion support retained; bounded prose; utility/inspection/import wording and direct Posting checks link; short-window navigation and breakpoint focus correction. | Browser layout captures, keyboard/menu/dialog interactions, no-JS question saving and token contrast checks. VoiceOver, native zoom/text resizing, native keyboard and full native visual acceptance remain NOT VERIFIED. |

## Navigation and authority contracts

- Server-rendered Go forms and existing POST routes remain authoritative. No SPA, client router, new dependency, migration, autosave or remote persistence was introduced.
- Job activities progressively enhance normal anchors. Without JavaScript all panels remain in the document and disclosures/forms remain operable. Activity changes preserve editor nodes and text; Back reveals the addressed activity.
- Existing return URLs remain accepted by the Wails redirect layer. `?question=...#prepare` returns to Answers; `?cover=saved#prepare` returns to transfer review; `?draft=...#prepare` remains Prepare. An explicit activity fragment outranks old source queries.
- A disconnected browser's saved answers may lead the writing view, but do not become current readiness. Strict employer/provider/field definition/revision policy and transmission checks are unchanged.
- Resume, approved contacts, letter and saved answers retain their independent inclusion flags and hashes. Unsaved editor text never enters scope. Reconciliation remains a local operation requiring separate transfer authorization.
- Global notices appear above the activities rather than inside a hidden panel. Polling offers refresh instead of replacing dirty editors.
- Transfer results are displayed only when there are actual results. Missing browser state is not presented as restored. Session URL/timestamp/control counts are secondary details; the no-fill recovery action remains visible without JavaScript.

## Production files

- Shell and navigation: `templates/partials.html`, `templates/jobs.html`, `static/ui.js`, `static/styles.css`.
- Job preparation/writing/results/history: `templates/job.html`, `static/inspection.js`, `internal/app/handlers.go` (presentation context only).
- Materials/settings: `templates/resumes.html`, `families.html`, `answer_library.html`, `career_evidence.html`, `candidate.html`, `ai.html`.
- Completion: `templates/applications.html`, `internal/app/jobs.go` (read-only latest-submission timestamp), `internal/app/resumes.go` (display size only).
- Secondary screens: `templates/inspection.html`, `requirements.html`, `import_results.html`.
- Regression additions: `product_experience_test.go`, `product_experience_qualification_test.go`; existing assertions updated for direct activities and accurate local-record labels. No safety assertions were deleted.

## Design decisions and deviations

Transfer review is a distinct section of Prepare, not another modal/wizard or route. This keeps authoritative consent and outcomes together and preserves normal POST return behavior. Recovery copy follows the current decision: disconnected sessions lead with reconnecting without sending material, rather than an empty transfer dashboard. The user's crowded-section feedback is incorporated into this composition.

System typography and the existing wordmark remain; no external fonts are downloaded. Compact navigation is nonmodal: it does not trap focus and all eight destinations remain named. Optional writing focus hides only shell framing and has an explicit Show navigation control. The two writing disclosures no longer share a native exclusive-name group; JavaScript activity selection reveals the corresponding artifact, and no-JS users may open both.

The proposed 820px threshold is measured in browser CSS pixels. It is **not** claimed to be qualified against native outer-window sizes. Review [the screenshot gallery](evidence/index.html) at original image size before judging text density.

## Verification

Use a temporary Go cache where sandboxing prevents the default cache. Full test commands require permission for synthetic loopback HTTP listeners; the first sandboxed attempt failed to bind IPv6 loopback. Both baseline suites passed when run with that access.

```sh
GOCACHE=/private/tmp/yourmove-phase2-go-cache go test -count=1 ./...
GOCACHE=/private/tmp/yourmove-phase2-go-cache go test -count=1 -tags desktop ./...
node --check static/ui.js
node --check static/inspection.js
bash -n scripts/qualify-native-workflow.sh
git diff --check
```

Required opt-in suite (all named tests must run, not skip):

```sh
GOCACHE=/private/tmp/yourmove-phase2-go-cache \
YOURMOVE_QUALIFY_WORKFLOW=1 YOURMOVE_QUALIFY_EARLY=1 \
YOURMOVE_QUALIFY_QUESTIONS=1 YOURMOVE_QUALIFY_BROWSER=1 \
YOURMOVE_QUALIFY_PRODUCT=1 \
YOURMOVE_PRODUCT_EVIDENCE=/private/tmp/yourmove-phase2-product-evidence \
go test -count=1 -tags qualification ./internal/app \
-run 'TestControlled(ProductExperience|UnapprovedProfileJourney|EarlyPreparationWorkbench|QuestionWorkbench|ContactJourney|CombinedContinuation)$|TestEarlySavedAnswersStayExcludedUntilReconciledAndAuthorized' -v
```

The additional activity-entry/history/no-JS test runs with `YOURMOVE_QUALIFY_PREPARE=1`, `-tags qualification`, and `-run '^TestControlledPrepareEntryPoints$'`.

Additional completion, brief/clipboard and Chromium trials passed. The full HTTP runtime restart test skipped because port 3000 was occupied; it remains NOT VERIFIED and the existing process was left untouched. Final outcomes and native evidence are recorded in [ACCEPTANCE.md](ACCEPTANCE.md). Diagnostic failures discovered during implementation were corrected: hidden refresh notices, query-bound return destinations, breakpoint focus, and asynchronous no-script disclosure assertions. A passed process startup or signature check is never counted as a native workflow pass.

## Evidence and limits

The browser fixture uses temporary SQLite, a synthetic PDF, a Unicode filename, saved narrative writing, reviewed experience with an unknown outcome, a saved letter, and local submission confirmation. Both revisions use the same test fixture. Baseline source was exported to a temporary directory with `git archive 6709840`; neither checkout was replaced or reset.

At a 900×700 CSS viewport and scroll origin zero, the first real opportunity begins at approximately **1055px before / 551px after**. These are renderer measurements from one synthetic fixture, not native measurements or user-study speed claims. Measurements and before/after PNGs are retained under `evidence/`.

`evidence/contrast.json` checks body, muted, surface, selected-link, control-border, focus and primary-button combinations in both themes. This is not a full automated or manual accessibility audit.

## Native qualification and outstanding gates

The existing `scripts/qualify-native-workflow.sh` safely assembles fresh bundles and signs/verifies the executable and bundle before launching. It was used without changing release signing/notarization or modifying a signed bundle.

The available tool catalog has no `node_repl`, which the installed Computer Use skill requires for Mac interaction. Therefore native screenshots, outer/inner sizing, menu/dialog keyboard behavior, picker/export cancellation, Unicode file round trips, VoiceOver, 200% zoom, native employer filling and native restart acceptance could not be exercised here. See the explicit procedures in ACCEPTANCE.md. Do not infer these passes from controlled Chromium.

## Delivery

Implementation commits and PR metadata are recorded in [DELIVERY.md](DELIVERY.md). Changes are submitted for review, not merged. Native acceptance remains a merge/release review gate; production release readiness is not claimed.
