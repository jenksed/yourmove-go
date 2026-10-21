# Question workbench qualification — 2026-10-20

**Question workbench: PASS (browser-rendered synthetic qualification).** Native Wails qualification remains **BLOCKED** under the existing continuity handoff. No native app was launched or repaired. No normal candidate storage or live employer application was accessed. No commit was created; pre-existing working-tree changes were preserved.

## Delivered behavior

Prepare now leads with questions before optional writing. JavaScript focuses one server-rendered question at a time, with a grouped index at 1280px and a compact chooser at 1050/800/700/390px. Previous/Next follows observed order, disables at the ends, and retains the original input nodes and unsaved values. Manual/sensitive/unsupported controls remain instructions. Profile facts have their source and a deliberate application-override disclosure. Reuse remains an explicit, unchecked choice in a disclosure.

Local categories derive from existing editability and exact nonempty values, consistent with `ReadyAnswers`. A deliberately saved empty answer stays in Needs local answer. Identical ambiguous definitions share one manual instruction to avoid duplicate DOM IDs; underlying observation and eligibility are unchanged.

Save answer locally still posts one question through the original endpoint. Success redirects to `/jobs/{id}?question={exact-key}#prepare`. The fresh renderer validates membership in the current job/observation; stale, foreign, malformed, and duplicate keys fall back safely. The Wails adapter accepts only the canonical single-key shape from that same job's answer-save route. No other draft/notice/unknown query combinations were added.

Saving restores the same question and keyboard focus after native fragment navigation, with actual saved status. Next remains separate. Review ready material focuses the existing exact continuation scope. The continuation form was not redesigned or copied into editors.

Accepted local POST failures retain escaped submitted text in a separate unsaved recovery page with the original error status, no save/transfer form, and a link to fresh Prepare. The user copies recoverable work before reloading. Rejected or oversized requests that cannot be safely parsed retain the original request rejection; malformed text may be sanitized for display. Recovery never populates preparation values, review hashes, or pending scope.

Dirty guards cover links and other form submissions with explicit stay/discard behavior, plus the ordinary browser beforeunload guard. Same-document question/tab/scope navigation retains inputs. Existing status polling still uses one watch host and offers refresh when edits exist. No autosave, storage persistence, framework, or new poller was added.

## Authority preserved

`questionKey`, `editableQuestion`, `resolveApplicationAnswer`, `prepareQuestions` resolution, revision/destination/observed-option checks, transactional save/reuse checks, `ReadyAnswers`, pending-answer hashing, and independent transfer commands retain their existing authority. The only additions around preparation are display filtering, categories, and return selection. No schema, answer matching, profile approval, CDP transport, browser ownership, document identity/upload, transfer replay protection, consent, or employer submission changes were made. AI remains confined to the existing letter generation path.

## Actual interaction evidence

The Browser plugin reported no connected browser. The repository's existing loopback-only headless Chromium fixture supplied real rendered UI, mouse input, text insertion, keyboard events, and screenshots. Every database/resume directory came from `testWorkspace` temporary storage.

`TestControlledQuestionWorkbench`:

- Typed `https://alex.example.test/work` and a motivation answer using Chromium input events; navigated through wide-index clicks and Previous/Next while retaining the exact input nodes and text.
- Canceled refresh and Documents navigation with dirty work; canceled saving one question while another was dirty, revealing the other unsaved editor.
- Observed a conditional-question update through the real status poller without replacement of dirty input.
- Saved URL and motivation answers and the exact observed `Remote` option individually. Verified SQLite content/revisions, same-question return/focus, index/chooser synchronization, separate Next, and visible common actions without scrolling in the inspected short-question case.
- Activated Review ready material and verified the existing continuation form displayed the saved values. Inspected profile override and manual panels; activated Previous with an Enter key sequence and verified focus followed the selected panel.
- Simulated managed-browser loss while dirty; polling retained the editor and SQLite retained prior local saves.
- Created a revision conflict via the normal save handler, submitted a different edited value through the rendered form, and inspected the recovery textarea. The other window's saved value remained authoritative and no transfer action appeared on recovery.
- Served the same page with enhancement scripts omitted, opened the profile override disclosure, verified all editable forms were reachable, typed and saved through the ordinary POST, and read back the exact local value.
- Observed zero answer transfer calls/armed answer operations during this question-editing journey.

`TestControlledContactJourney` and `TestControlledCombinedContinuation` also passed against their existing real loopback employer fixture. Assertions retained exact displayed scope, saved-answer-only explicit transfer, manual corrections, native choice controls, conditional questions, qualified opt-in reuse, changed-option/ambiguous/sensitive abstention, duplicate suppression, one resume upload per journey, and zero submissions. Their tab targets were updated for exact question-return URLs; transfer/content assertions were retained.

The question fixture's navigation-confirm choice was controlled through `window.confirm`; native Wails dialogs and OS window-close behavior are not qualified by that test. The production beforeunload listener is present, but native dialog rendering is not claimed.

## Before/after comparison

The captured pre-change template, styles, scripts, and old save redirect were replayed through a temporary Go build overlay, without restoring or replacing working-tree files. Two answers were typed and saved through the rendered old UI. After both saves, the saved editor was outside the viewport because the page returned to the start of Prepare. Screenshots recorded those returns.

| Common path | Captured old UI | Focused workbench |
| --- | --- | --- |
| Enter and save one answer | Focus editor, type, Save answer | Focus editor, type, Save answer locally |
| Location after save | Prepare; saved editor offscreen in both observed saves | Same exact question, saved status and focus |
| Move to next question | Locate/scroll to its expanded form | One separate Next activation |
| Unsaved question navigation | Many expanded forms; no focused navigation | Index/Previous/Next retains original inputs |
| Review transferable values | Scroll back to continuation | Review ready material link |

No time improvement, total scroll-event count, or employer performance improvement was measured. In the short-question after case, the editor, save, and Next controls fit together; long answers, expanded reuse details, or large exact transfer scopes can still require page scrolling.

## Visual evidence

Captured and inspected light and dark appearance at **1280, 1050, 800, 700, and 390 pixels**, height 900. No horizontal page overflow was detected at those widths. Question labels wrap; form controls and focus outlines remain accessible; there is no question-workspace nested scroller. The application shell's existing narrow navigation strip is unchanged. No animations were added; existing reduced-motion styles remain applicable.

Evidence root: `/private/tmp/yourmove-question-workbench/`.

- `baseline/questions-{1280,1050,800,700,390}-{light,dark}.png`
- `baseline/baseline-save-{1,2}-return.png`
- `after/questions-{1280,1050,800,700,390}-{light,dark}.png`
- `after/dirty-navigation.png`
- `after/saved-return.png`
- `after/ready-scope.png`
- `after/conflict-recovery.png`
- `after/no-javascript.png`
- `after/long-question-390-dark.png` (long unbroken prompt; no overflow)

The after save-return, scope, recovery, no-JS, and long-question screenshots were also inspected. These are browser screenshots, not native Wails screenshots.

## Regression results

Passed:

- Focused baseline question/answer/reuse/UI/continuation/redirect tests before implementation.
- Focused question status/identity/save-return/error/empty-answer/no-JS markup and Wails redirect regressions.
- `go test ./...`
- `go test -tags desktop ./...`
- `YOURMOVE_QUALIFY_QUESTIONS=1 go test -tags qualification . -run TestControlledQuestionWorkbench -count=1 -v`
- Baseline UI replay using the temporary overlay and `YOURMOVE_QUESTION_BASELINE=1`.
- `YOURMOVE_QUALIFY_BROWSER=1 go test -tags qualification . -run 'TestControlled(ContactJourney|CombinedContinuation)' -count=1 -v`
- `git diff --check`
- `graft build`

Go cache: `/private/tmp/yourmove-go-build`. Local fixture listeners and isolated Chromium execution ran with sandbox escalation. The initial default-cache/sandbox listener failures were environmental; reruns using the temporary cache and permitted loopback listeners passed.

Logs: `baseline-journey.log`, `ui-journey.log`, `continuation-journeys.log`, `focused-tests.log`, `go-test.log`, `go-test-desktop.log` under the evidence root. Intermediate controlled runs caught a real native-fragment focus ordering issue, which was fixed and verified; other harness corrections tracked the revealed dirty editor, conditional question count, and full Enter key sequence. No failing assertion was removed to obtain acceptance.

## Exact files changed by this slice

- `application_answers.go`
- `handlers.go`
- `desktop_redirect.go`
- `templates/job.html`
- `static/ui.js`
- `static/inspection.js`
- `static/styles.css`
- `question_workbench_test.go` (new)
- `question_workbench_qualification_test.go` (new)
- `contact_qualification_test.go` (existing qualification tab targets)
- `internal/application/browser/chromium/qualification.go` (test-build screenshot selector/appearance support)
- `docs/implementation/2026-10-20-question-workbench.md` (this handoff)

## Limits and stopping point

Native Wails qualification remains **BLOCKED** by the previously documented isolated environment/targeting problem; it was not attempted. Wails POST-to-GET behavior is verified by handler tests, not a native launch. Screen-reader announcements and native platform dialog appearance were not manually qualified. Synthetic observations do not prove real employer acceptance or universal ATS compatibility.

The question slice is independently useful and complete. No cover-letter save navigation, resume-picker compaction, manual letter creation, general Prepare refactor, browser automation migration, or ATS expansion was started.

**Next approved slice: Cover-letter save returning directly to scope-first Prepare.** Saving must remain local and transfer must remain explicit.
