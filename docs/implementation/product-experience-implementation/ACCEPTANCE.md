# Phase 2 acceptance

**Overall: NOT VERIFIED for full native acceptance.** Production changes cover the eight increments; automated and controlled-browser results below do not establish full Wails acceptance or public-release readiness. The PR remains a draft pending native review.

Statuses: **PASS** exercised successfully; **FAIL** exercised and incorrect; **BLOCKED** cannot execute because of an identified constraint; **NOT VERIFIED** not actually exercised.

## Automated gates

Commands and required environment flags are in [HANDOFF.md](HANDOFF.md#verification). Evidence summaries are in `evidence/verification.txt`.

| Gate | Status | Evidence |
|---|---|---|
| Baseline ordinary and desktop-tagged suites | PASS | Clean `6709840` before production changes; sandbox cache/loopback restrictions resolved through temporary cache and authorized execution. |
| Final `go test -count=1 ./...` | PASS | All packages pass. |
| Final `go test -count=1 -tags desktop ./...` | PASS | All packages pass. |
| Both JS syntax checks, shell syntax, diff whitespace | PASS | `node --check static/ui.js`, `node --check static/inspection.js`, `bash -n scripts/qualify-native-workflow.sh`, `git diff --check`. |
| Unapproved profile journey | PASS | `TestControlledUnapprovedProfileJourney`; eligible saved answers and resume work without profile approval; contacts remain independent. |
| Early answers remain excluded until explicit reconciliation/authorization | PASS | `TestEarlySavedAnswersStayExcludedUntilReconciledAndAuthorized`. |
| Early workbench | PASS | `TestControlledEarlyPreparationWorkbench`, required opt-in flag enabled. |
| Question workbench / dirty recovery / no-JS | PASS | `TestControlledQuestionWorkbench`; full assertions retained, updated activity entry and no-script composition. |
| Contacts / combined continuation | PASS | `TestControlledContactJourney`, `TestControlledCombinedContinuation`; one resume upload, independent verified outcomes, manual values preserved, zero employer submissions. |
| Product-wide production page interaction | PASS | `TestControlledProductExperience`; actual Go pages and scripts, synthetic SQLite/PDF/writing/experience, both themes, direct activities, compact navigation, Escape/focus, Stay/Leave and exact destination. |
| Preparation entries / Back / no-JS disclosures | PASS | `TestControlledPrepareEntryPoints`; dirty answer survives writing changes and history traversal; no-JS forms remain usable. |
| Completion and transactional recovery | PASS | `TestControlledSubmissionRecordingWorkflow`; keyboard, multiple dirty editors, no-JS, idempotency, missing-resume and injected transaction-failure recovery. Desktop redirect compatibility exercised in Chromium, not native Wails. |
| Application brief privacy / clipboard | PASS | `TestControlledApplicationBriefClipboardAndWorkspace`; exact server export, clipboard denial/selectable fallback and privacy boundaries. Evidence now uses a temporary directory rather than overwriting historical captures. |
| Chromium guarded-control trials | PASS | Documented `TestControlled|Test.*Trial` package suite; one upload, exact control writes/readback and uncertain/detached control protection. |
| Full HTTP runtime restart | NOT VERIFIED | `TestControlledRuntimeRestart` skipped because normal port 3000 was occupied; existing process deliberately left untouched. A skipped test is not a pass. |
| Native-only UI and employer interaction | NOT VERIFIED | Separate native matrix below. |

## Screen and workflow checks

All PASS entries here refer to Go or controlled Chromium, **not native WebKit**. [Open comparable evidence](evidence/index.html). Before/after captures use the same synthetic fixture on baseline `6709840` and the implementation; dimensions are CSS viewport sizes.

| Surface / journey | Status | Specific evidence |
|---|---|---|
| Opportunities | PASS | 900×700: first real row moves from approximately y=1055 to y=551; full title wraps; saved filters, recommendation arithmetic and POST behavior retain existing coverage. |
| Labeled shell | PASS | Eight named destinations; Submissions replaces Applications in the menu/page, route retained. Compact menu tested at 700×550. |
| Job Overview | PASS | Saved posting, source, metadata and edit forms preserved; screenshot in both revisions. |
| Prepare | PASS | Explicit selected resume, profile approval/exclusion, early/live distinctions and browser state; no application-wide completion percentage. |
| Answers | PASS | One authoritative save form per question; complete prompt, selector, local state, Previous/Next and contextual assistance retained. Dirty same-node switching and save-return covered. |
| Cover letter | PASS | Saved artifact precedes generator, explicit local save/PDF and versions retained; native export dialog itself unverified. |
| Exact transfer review | PASS | Existing Go inclusion flags/hashes/revisions remain; unsafe, unsaved, stale and unreconciled answers excluded. Existing independent outcomes are shown. |
| Disconnected browser / crowded recovery state | PASS | One visible no-fill recovery action; prior session details collapsed; empty results and duplicate letter/readiness/contact blocks omitted. Specific regression test covers no selected resume. |
| Documents and role families | PASS | Unicode name/version/size/object-first controls; suggestion semantics, immutable document associations and revisions retain Go coverage. Native dialogs unverified. |
| Answer Library | PASS | Original question and saved writing lead; independent snapshot and revision behavior retained; insertion remains unsaved. |
| Career Evidence | PASS | Situation/actions/outcome/review/source retained; unknown outcome stays unknown; optional context disclosed. |
| Candidate Profile | PASS | Applicant approval language, fresh approval at save, optional pronouns, no mandatory workflow. |
| AI Connections | PASS | Local vs external/custom grouping; configuration caveat; optional setup and manual-writing path; no provider call on view. |
| Submissions / History | PASS | Actual record date, local-confirmation language, pinned resume, notes, explicit confirmation token and Next behavior retained. |
| Employer inspection | PASS | Browser details terminology, existing start/stop/recheck/restart and handoff forms retained. |
| Posting checks / observed questions / utilities / imports | PASS | Shared shell and both-theme captures; direct Posting checks link; future-feature copy removed from operational inventory. |
| Empty first-run workspace | PASS | Real empty server state captured; save/import destinations available without mandatory profile/AI setup. |
| Errors / preserved editor text | PASS | Existing conflict/unsaved recovery pages and copyable text retained; notices remain visible above activity panels. |
| Browser layout sizes | PASS | Production captures at 1100×800, 900×700, 750×650 and 700×550; measurements record inner dimensions and document width. |
| Palette contrast subset | PASS | 14 body/muted/surface/selected/control/focus/button combinations in `evidence/contrast.json`. This does not cover every semantic/hover background. |
| Reduced motion / enlarged system text / 200% zoom | NOT VERIFIED | Reduced-motion CSS remains; no claim of a complete motion or zoom exercise. |

## Description and salary follow-up

PASS: both full ordinary and desktop Go suites, spacing-policy cases, provider fallback/structured precedence, optional JSON-LD salary, benefit-budget exclusions, source provenance, original-description preservation and manual-salary protection. Existing screenshots/native evidence above predate this follow-up; no new native interaction acceptance is implied.

## Native gates

The installed Computer Use skill requires `node_repl`; that tool is absent from the available catalog. An existing signed harness can be built/launched from the terminal, but actual native interaction cannot be driven or observed through the required facility here.

| Gate | Status | Actual evidence / limitation |
|---|---|---|
| Fresh Wails desktop build | PASS | Existing repaired harness, isolated fixture. |
| Explicit executable and bundle ad-hoc signing | PASS | Strict verification before launch; no signed bundle contents changed. |
| Startup | PASS | Wails startup at `f6cc186`. The final production difference at `f050e16` is the compact focus-control correction, built/signed separately. This establishes process startup only; final-source UI interaction is unverified. |
| Rendered native window / visual appearance | NOT VERIFIED | No current native screenshot or accessibility tree captured. |
| Four outer dimensions and corresponding WebView inner dimensions | NOT VERIFIED | Browser viewport measurements do not substitute for native window bounds. |
| Native menu / direct activities / Back / keyboard / dialog focus | NOT VERIFIED | Covered in controlled Chromium only. |
| Multiple dirty editors in native host | NOT VERIFIED | Phase 1 Go/Chromium contracts retained; native exercise outstanding. |
| Native import/save, cancellation, Unicode and exact bytes | NOT VERIFIED | Existing backend handling retained; actual dialogs not operated. |
| Native resume/contact/answer/late-letter transfer and manual-entry protection | NOT VERIFIED | Controlled employer transfer passes separately; no native-host transfer acceptance established. |
| Native recording and restart recovery | NOT VERIFIED | Local persistence and nonrestored browser semantics tested outside native host. |
| VoiceOver and 200% zoom | NOT VERIFIED | No VoiceOver or native zoom evidence available. |
| Native interaction automation facility | BLOCKED | Required `node_repl`/Computer Use facility unavailable. |
| Production signing / notarization / release readiness | NOT VERIFIED | Development ad-hoc qualification does not establish these. Release tooling unchanged. |

## Reproducible native acceptance

1. Run `GOCACHE=/private/tmp/yourmove-phase2-go-cache bash scripts/qualify-native-workflow.sh --launch`. Use only its unique synthetic bundle/workspace. Stop if strict signing fails. The fixture employer URL is a synthetic identity, **not a safe transfer target**; do not click employer actions against that URL.
2. Confirm the actual window rendered; record its source commit, outer bounds and WebView `innerWidth`/`innerHeight` separately at 1100×800, 900×700, 750×650 and 700×550. Record light/dark screenshots with scroll origin.
3. On the synthetic job, use Answers and Cover letter directly. Type into two answers and the letter. Switch activities and return. Confirm every value remains and nothing is saved/transferred.
4. Open Navigation → Candidate profile. Exercise Stay, Escape, Tab/Shift-Tab and Leave. Confirm exact destination once, no dialog loop, focus restoration and unchanged saved SQLite values. Repeat with Documents.
5. Import synthetic PDF/DOCX through the native picker; test Cancel and a Unicode name. Assign a family and resume to a job, import a revision, and confirm old job/submission associations stay pinned. Export the selected original and compare its bytes; separately save/export an edited synthetic letter and cancel the Save dialog.
6. Inspect the simplified disconnected transfer section with no selected resume. The no-fill action must be visible, previous-session details optional, profile exclusion concise, and empty results absent. Complete profile independently and check that approval still does not authorize filling.
7. Full transfer requires a dedicated native loopback employer fixture/factory bound to the synthetic destination (not provided by the navigation-only native fixture). Reuse the controlled browser fixtures in `workflow_stabilization_qualification_test.go` and `contact_qualification_test.go`; qualify this native fixture wiring before treating any native fill as accepted. Never use a real employer URL/account. Verify one upload, two saved answers, one unsaved answer excluded, approved contacts only with explicit scope, manual values preserved, late letter, independent outcomes and zero submit requests.
8. Simulate applicant-confirmed completion locally, verify one idempotent record with the correct resume, inspect History and move to another opportunity. Verify browser ownership and cleanup.
9. Quit/relaunch the synthetic workspace. Confirm durable materials remain and no browser-only work is claimed as recovered. Reinspect before transfer.
10. Repeat representative navigation/dialog tasks keyboard-only, with VoiceOver, enlarged text, 200% zoom and reduced motion. Capture actual evidence. Change each NOT VERIFIED status only after observing it.

No unresolved automated product defect is intentionally accepted here. Full native acceptance remains an explicit outstanding gate; the review artifacts do not imply it has passed.
