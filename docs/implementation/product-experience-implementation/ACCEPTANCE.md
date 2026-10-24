# Phase 2 acceptance

**Overall: NOT VERIFIED for full native acceptance.** The salary-extraction and dirty-completion fixes are committed and their affected package/Chromium tests pass. A fresh Wails bundle also launched from an isolated workspace, but native interaction could not proceed because macOS LaunchServices returned `-10827` when starting the accessibility automation process. Required native Journeys A–D remain NOT VERIFIED. The PR remains a draft pending native acceptance.

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
| Full HTTP runtime restart | PASS | `TestControlledRuntimeRestart` now reserves a dynamic loopback port, passes it via `YOURMOVE_HTTP_PORT`, starts/stops/restarts the isolated ordinary executable, and verifies local persistence plus changed-resume recovery. Port 3000 and its existing process were untouched. |
| Question workbench final rerun | PASS | An earlier combined run timed out waiting for an optional manual-review disclosure; isolated and final combined reruns passed all assertions. No production defect was confirmed. |
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

PASS: both full ordinary and desktop Go suites, spacing-policy cases, provider fallback/structured precedence, optional JSON-LD salary, benefit-budget exclusions, source provenance, original-description preservation and manual-salary protection. Native UI captures below do not qualify salary capture behavior inside Wails.

## Native gates

The user enabled macOS accessibility access during qualification. System Events then exposed the real Wails accessibility tree and allowed native navigation and picker interaction. A few controls (notably free-form text entry through this automation route) were not reliably exercised, so dirty-editor behavior remains unverified natively. Evidence uses the isolated synthetic workspace only.

| Gate | Status | Actual evidence / limitation |
|---|---|---|
| Fresh Wails desktop build | PASS | `scripts/qualify-native-workflow.sh --launch`; clean final source `f0100bc57adb47f324222d7b1020830bb12d42fe`; isolated synthetic workspace. |
| Explicit executable and bundle ad-hoc signing | PASS | `codesign --verify --strict` passed for executable and complete bundle before launch and again after launch. Ad-hoc development identity only. |
| Startup | PASS | PID `46556`; Wails log reports `Wails=v3.0.0-beta.32`, source revision `f0100bc57adb47f324222d7b1020830bb12d42fe`, and startup at `/jobs`. Process remained alive through capture. |
| Rendered native window / both appearances | PASS | Actual Wails window captures include [Opportunities at 1100×800 dark](evidence/native-opportunities-1100x800-dark.png), [Prepare dark](evidence/native-prepare-1100x800-dark.png), [Documents dark](evidence/native-documents-1100x800-dark.png), [Answers light](evidence/native-answers-1100x800-light.png), [Cover letter light](evidence/native-cover-letter-1100x800-light.png), and [Role families light](evidence/native-role-families-1100x800-light.png). Appearance selector changed from System to Light in-app. |
| Requested outer sizes | PASS | Native outer dimensions set/read through macOS Accessibility: 1100×800, 900×700, 750×650, 700×550. Opportunities captures exist at all four sizes in `evidence/`. |
| Inner WebView size | NOT VERIFIED | Accessibility reported the WebView scroll-area sizes as 1101×769, 901×669, 751×619 and 701×519 respectively. These are AX scroll-area dimensions, not direct JavaScript `innerWidth`/`innerHeight` measurements. |
| Named navigation and direct job activities | PASS | Native sidebar labels were visible; Opportunities opened the synthetic job; Overview, Prepare, Answers, Cover letter and History tabs were exposed; Answers and Cover letter were selected and rendered. A native History action and every Back/return path remain NOT VERIFIED. |
| Native keyboard / dirty dialog / multiple dirty editors / focus return | NOT VERIFIED | Accessibility tree worked, but this session did not establish reliable keyboard text entry and did not observe the application-owned dirty-navigation dialog. Chromium Phase 1 assertions still pass. |
| Native completion confirmation / History with dirty editors | NOT VERIFIED | The completion path now uses the application-owned dialog, verified in controlled Chromium with cancellation preserving exact dirty answer/letter values and affirmative recording producing one local event pinned to the selected resume. Native Wails interaction is not demonstrated. |
| Native import cancellation, Unicode, exact bytes and resume association | PASS | Native picker Cancel returned to the app; `Synthetic résumé 漢字.pdf` was imported. Stored PDF was 142 bytes and SHA-256 matched the source (`554f5147984d75fe7e43c6f15d08e3d52097907a122299c06613e84b3b4c8749`); selected resume ID 1 persisted for the synthetic job. |
| Native role-family example action | PASS | Documents → Role families opened and the built-in eight example families were added to the isolated synthetic DB. |
| Native free-form role-family creation/assignment and document/letter export | NOT VERIFIED | No family was created/assigned and no native export/save round trip was observed. |
| Native recording, History, next opportunity and Wails relaunch recovery | NOT VERIFIED | Runtime persistence/restart passes outside native Wails; this Wails process was not quit and relaunched. |
| Native resume/contact/answer/late-letter transfer and manual-entry protection | NOT VERIFIED | No safe native loopback fixture/factory was completed. The existing navigation fixture points at a nonlocal synthetic Ashby URL and was not opened. Controlled Chromium transfer passes are separate evidence. |
| VoiceOver, 200% zoom, enlarged text and reduced motion | NOT VERIFIED | Accessibility-tree inspection is not VoiceOver acceptance; no actual VoiceOver, zoom, text-size or reduced-motion result was observed. |
| Native accessibility tree via System Events | PASS | After the user's settings update, System Events exposed the app's accessibility tree. This does not establish VoiceOver acceptance; free-form keyboard interaction was not reliable in this session. |
| Current-session System Events attachment | BLOCKED | Reproduced with `osascript`: requesting GUI processes from System Events returns `-10827`. `open` reports `LSOpenStuffCallLocal` / `kLSNoExecutableErr` for System Events, Accessibility Inspector, Finder and Calculator, although their declared executable files exist. YourMove itself was launched directly from its fresh bundle and stayed alive. This isolates the current failure to the host's LaunchServices/accessibility launch path; no app attachment or journey was exercised. |
| Production signing / notarization / release readiness | NOT VERIFIED | Development ad-hoc qualification does not establish these. Release tooling unchanged. |

## Reproducible native acceptance

1. Run `GOCACHE=/private/tmp/yourmove-phase2-go-cache bash scripts/qualify-native-workflow.sh --launch`. Use only its unique synthetic bundle/workspace. Stop if strict signing fails. The navigation fixture employer URL is a synthetic identity, **not a safe transfer target**; do not click employer actions against that URL.
2. Confirm the actual window rendered; record its source commit, outer bounds and WebView `innerWidth`/`innerHeight` separately at 1100×800, 900×700, 750×650 and 700×550. Record light/dark screenshots with scroll origin.
3. On the synthetic job, use Answers and Cover letter directly. Type into two answers and the letter. Switch activities and return. Confirm every value remains and nothing is saved/transferred.
4. Open Navigation → Candidate profile. Exercise Stay, Escape, Tab/Shift-Tab and Leave. Confirm exact destination once, no dialog loop, focus restoration and unchanged saved SQLite values. Repeat with Documents.
5. Import synthetic PDF/DOCX through the native picker; test Cancel and a Unicode name. Assign a family and resume to a job, import a revision, and confirm old job/submission associations stay pinned. Export the selected original and compare its bytes; separately save/export an edited synthetic letter and cancel the Save dialog.
6. Inspect the simplified disconnected transfer section with no selected resume. The no-fill action must be visible, previous-session details optional, profile exclusion concise, and empty results absent. Complete profile independently and check that approval still does not authorize filling.
7. Full transfer requires a dedicated native loopback employer fixture/factory bound to the synthetic destination (not provided by the navigation-only native fixture). Reuse the controlled browser fixtures in `workflow_stabilization_qualification_test.go` and `contact_qualification_test.go`; qualify this native fixture wiring before treating any native fill as accepted. Never use a real employer URL/account. Verify one upload, two saved answers, one unsaved answer excluded, approved contacts only with explicit scope, manual values preserved, late letter, independent outcomes and zero submit requests.
8. Simulate applicant-confirmed completion locally, verify one idempotent record with the correct resume, inspect History and move to another opportunity. Verify browser ownership and cleanup.
9. Quit/relaunch the synthetic workspace. Confirm durable materials remain and no browser-only work is claimed as recovered. Reinspect before transfer.
10. For keyboard acceptance, use only Tab, Shift+Tab, Return/Space and Escape. Open Answers, edit a synthetic answer and letter, navigate to Candidate profile, and record Stay, Escape, Leave, exact destination, restored focus and preserved/suppressed values. Repeat with two dirty editors.
11. For VoiceOver, enable it in System Settings → Accessibility → VoiceOver. Start at Opportunities, navigate every named sidebar destination and job activity using VoiceOver navigation, then read the answer editor, save status, transfer authorization scope, picker buttons and dirty-navigation dialog. Confirm reading order follows visual order, each control has a meaningful name/role/state, and dialog focus returns to the initiating control. Disable VoiceOver after the run.
12. For zoom/text resizing, use macOS Accessibility Zoom to reach 200% effective magnification and increase text size using available system text-size settings. Repeat Opportunities, Answers, Cover letter, Prepare and transfer review. Check control labels, focus, primary actions, scroll reachability and no horizontal clipping; record actual zoom/text setting and dimensions. If the current WebView does not expose a supported text-size setting, record that limitation and use the app's supported zoom route rather than claiming enlarged-text acceptance.
13. Enable System Settings → Accessibility → Display → Reduce motion. Repeat navigation, disclosures and save/dirty-state feedback; verify no required state or focus transition depends on animation. Restore the original macOS settings after recording results.
14. For native transfer, add a qualification-only Wails launch/factory that seeds an isolated workspace whose exact approved employer destination is a controlled `127.0.0.1` fixture. Assert the managed Chromium owner, scheme/host/port/path and fixture request log before allowing transfer controls. Exercise one resume, approved contacts, two eligible saved answers, one unsaved answer exclusion, a manually entered browser value, and a late letter continuation; assert per-field observations, no duplicate upload, and zero POSTs to any submission endpoint. Never use the current remote navigation fixture. Capture fixture request logs and Wails-window evidence with synthetic content only.
15. Capture screenshots/accessibility evidence and run the above controls in both themes at each gate where layout matters. Change a NOT VERIFIED status only after observing that exact behavior.

No unresolved automated product defect is intentionally accepted here. The fixed-port restart-test gap is resolved and passes. Native signing, launch, visual rendering, four outer sizes, both themes, direct Answers/Cover letter views, native picker cancellation, Unicode import, and local resume association have prior observed evidence from commit `f0100bc57adb47f324222d7b1020830bb12d42fe`; these do not cover the current closeout diff. A fresh closeout bundle from commit `02726ab` plus the salary fix launched from a new synthetic workspace and passed strict ad-hoc signature verification. Native attachment did not occur: LaunchServices failed to start System Events and several unrelated GUI apps with `-10827`. Full native Journeys A–D, dirty-editor/dialog interaction, completion/History, Wails restart, export dialog, loopback-owned transfer fixture, and VoiceOver/zoom/reduced-motion gates remain NOT VERIFIED. PR #7 must remain draft.
