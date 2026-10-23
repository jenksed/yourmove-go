# Application workflow stabilization — Phase 2 handoff

Base: latest `origin/main`, `dfb2539c961e6ad0ad8665fbc822f8144b8981eb` (fetched October 23, 2026). Worktree: `/private/tmp/yourmove-workflow-stabilization`; branch: `fix/application-workflow-stabilization`. Product changes: `76233a6`. Final development harness: `222956c`. No release candidate, applicant workspace, production packaging, or design research was changed. Direction A research was read from the original checkout because it is absent from this main commit.

## 1. Findings and remaining uncertainty

- **Navigation mechanism confirmed in source:** document capture cancelled navigation through `window.confirm()`. Cancellation dispatched `question:reveal`, opened ancestor disclosures, and focused the first dirty editor. Repeated sidebar attempts could repeat this cycle. The replacement works in real Wails/WebKit. The original user's exact WebView confirmation failure cannot be reconstructed from source alone.
- **Early/live distinction confirmed:** early discoveries save to `early_application_answers`; they are not live browser answers. Reconciliation and transfer are separate explicit decisions. Their status was buried inside a writing activity, away from Fill. The repaired transfer area shows counts, reasons, and the reconciliation path.
- **No blanket profile gate found for saved answers:** two safely bound live saved answers transfer without an approved profile in the controlled employer fixture. Contacts are excluded independently. The original two answers' exact source, identity and browser outcomes remain unknown; no real applicant database was opened. It would be inaccurate to claim a backend profile dependency caused those missing answers.
- **Strict identity matters:** live question keys hash the employer destination, provider and complete field definition. Changed wording, options or other definition metadata can preserve a saved answer as an earlier unmatched answer. Do not weaken this to label matching. The new summary explains these excluded saved records before Fill.
- **Save-return recovery:** letter save/transferring results now explicitly focus and scroll to `application-action`, with a scroll margin to avoid fractional top-edge clipping. Question save recovery and same-job unsaved-text carry remain intact.
- **Native keyboard issue observed:** WebKit could skip the two dialog buttons under macOS keyboard preferences. The dialog now explicitly moves focus on every Tab/Shift-Tab. A real Wails screenshot showed focus on Leave, and Return reached the sidebar destination.

## 2. Application/frontend contracts

All writes remain Go POSTs. No schema migration, profile inference, autosave, employer submission or transfer-policy relaxation was added. `earlyQuestionPreparation.AwaitingReconciliation()` is a read-only summary of unaccepted saved current/previous early drafts. Current eligible answers and exact hashes continue to come from `summarizePreparation`, `pendingAnswers` and `ReviewHash`.

The transfer area now independently exposes profile exclusion, early reconciliation, changed-definition saved answers, exact eligible saved values, and dirty editor labels. The dirty labels are presentation only: JavaScript never adds their text to transfer scope. Resume selection remains a separate observed result from answers, contacts and letters.

## 3. Dialog/navigation lifecycle

`static/ui.js` creates one native `<dialog>` with the requested title/body, `aria-labelledby` and `aria-describedby`. It uses existing design tokens and buttons. `allowLeaving(ownForm, run)` captures one original action, invoking focus and the dirty forms being acknowledged. Modal inertness prevents competing pointer actions; Tab/Shift-Tab alternate the two controls, including WebKit with full keyboard access disabled.

- Stay and Escape close and clear the pending action, return invoking focus, and preserve every editor. They do not reveal another question or retry navigation.
- Leave resets only the captured dirty forms to their server-rendered defaults (also preventing back/forward cache from resurrecting discarded values), clears only this pathname's carried edits/AI preview recovery, closes, and performs the captured action. Saved SQLite records remain untouched. Other workspaces' recovery is retained.
- Capture interception stops cancelled link events before question/disclosure callbacks. The exact original URL includes query and fragment. Acknowledged departure sets `leaving`, suppressing a redundant `beforeunload`. `pageshow` resets the departure/loading flag.
- Same-page fragments, tabs and question switching keep the same DOM values. New-question refresh and independent material actions use the existing revision-bound save/return carry. External links/downloads and native Open employer operations keep their independent behavior. Other guarded POSTs resume once through `requestSubmit` with the original submitter and a one-use bypass; failed validation restores unload protection.
- Recording completion retains its required explicit submission-confirmation checkbox and separate unsaved-edit decision. It never submits the employer form. Reload/window-close protection remains `beforeunload`, outside the in-app dialog contract.

Keep `data-question-panel`, workbench identity, form action suffixes, recovery markers, `data-stay`, `data-leave`, and the single authoritative editor per question stable, or update the tests together.

## 4. Early/live eligibility and authorization

Inspection → provider-specific identity reconciliation → local save → current preparation hash → explicit Fill/Continue → per-answer current check → Chromium guarded write → independent observation remains authoritative.

An early answer is **saved locally**, not authorized or eligible. In Prepare, the applicant sees the summary beside Fill, opens **Review earlier discovered questions**, finds that question's **Live form review**, and chooses **Use prepared answer for this question** only when an exact supported relationship is available. That operation saves a live answer; a separate displayed transfer scope must then be authorized. Ambiguous, changed, sensitive, unsupported, foreign-destination and stale-revision cases preserve text and explain manual/review steps.

Eligible answers are attempted individually. Existing manual entries, active user fields and files are preserved. Unsaved text never enters server transfer requests. Successfully attempted or uncertain answers are not silently replayed; current saved changes require new authorization. Every write still checks domain ownership and the live field definition.

## 5. Candidate profile readiness

Missing/unapproved means **contact filling disabled**, with **Complete candidate profile** visible before Fill and subject to the same repaired navigation guard. Job organization, resume selection and eligible saved answers remain available. Approval qualifies only reviewed facts; it does not authorize transmission. Existing profile revisions and transfer authority are unchanged. Later approved facts can enter a separate authorized continuation without uploading the resume again.

## 6. Result meanings

| State | Meaning |
| --- | --- |
| Resume selected in browser | Browser file control selected the resume; employer acceptance is not established. |
| Contacts excluded | Profile approval is missing; no contact authority was granted. |
| Awaiting reconciliation | Early text is preserved and excluded until explicit live acceptance. |
| Earlier saved answer excluded | Current field definition no longer matches its saved key; review preserved text and current field. |
| Eligible for transfer | Current supported saved value is in the displayed exact scope; a click still must authorize it. |
| Unsaved edits not transferred | Editor text is outside scope; an older saved value, if displayed, can still be authorized. |
| Filled and verified | The individual browser write was independently read back. |
| Already correct | Existing browser value matched and was left unchanged. |
| Manual/unsupported | No forced fill; complete or review that specific field manually. |
| Uncertain | Verification failed/interrupted; inspect the employer control; no automatic replay. |
| Processing finished | Attempts ended; inspect each outcome, not a blanket form success. |

## 7. Regression gates

Passed against synthetic temporary SQLite/documents and loopback employer pages:

- `go test -count=1 ./...`
- `go test -count=1 -tags desktop ./...` (one concurrent run hit the existing fake subscription CLI readiness timeout; a subsequent complete standalone run passed).
- `git diff --check`; `node --check static/ui.js`; `bash -n scripts/qualify-native-workflow.sh`.
- Targeted `go test -race ./internal/app -run 'Test(EarlySavedAnswersStayExcludedUntilReconciledAndAuthorized|AnswerTransferApprovalAndContinuation|CombinedApplicationScopeAndIndependentFailures)'`.
- Scenario A/D/C: `YOURMOVE_QUALIFY_WORKFLOW=1 go test -count=1 -tags qualification ./internal/app -run TestControlledUnapprovedProfileJourney -v`: role family/resume assignment, three fields, two saved values, one unsaved name, one resume selection, two observed fills, no profile contacts/submission; subsequent reviewed profile fact, preserved employer manual value, no resume replay; exact destination/query/fragment, Stay, Escape, Tab, cover-letter plus multiple dirty forms, persisted records unchanged.
- Scenario B: `TestEarlySavedAnswersStayExcludedUntilReconciledAndAuthorized`: two early answers stay excluded and visible; each acceptance saves locally; separate authorization transfers both. `YOURMOVE_QUALIFY_EARLY=1 ... -run TestControlledEarlyPreparationWorkbench` also passed with a live local Greenhouse fixture.
- Scenario C/recovery: `YOURMOVE_QUALIFY_QUESTIONS=1 ... -run TestControlledQuestionWorkbench` passed mouse question/save flows, cancelled sidebar/save navigation, dirty polling, conflict recovery and no-JavaScript POST. CDP pointer press/release now shares a connection.
- D/E: `YOURMOVE_QUALIFY_BROWSER=1 ... -run 'TestControlled(ContactJourney|CombinedContinuation)$'` passed actual contacts, independent answer/letter continuation, changed identities, sensitive/ambiguous/manual controls, observed outcomes, one resume and zero employer submissions. Legacy qualification now follows lazy **Help me answer**, awaits the persisted revision, and waits for save-return layout; the early test follows its exact query redirect.
- Browser package: `YOURMOVE_QUALIFY_BROWSER=1 go test -count=1 -tags qualification ./internal/application/browser/chromium -run 'TestControlled|Test.*Trial' -v` passed real handoff, normalized controls, uncertain location suggestions, detached/changed controls and clipboard trial.

Use an isolated `YOURMOVE_DATA_DIR`; these tests additionally create their own `testWorkspace` storage. Do not point qualification at applicant data. Normal Go tests do not execute opt-in Chromium or native gates.

## 8. Native evidence and limits

See [NATIVE_QUALIFICATION.md](NATIVE_QUALIFICATION.md) for signature incident, exact signed source/bundle, commands, logs and separate gate results. The fresh signed Wails window and navigation interactions were exercised through the Computer Use skill, not inferred from Chromium. Native employer filling, simultaneous multiple dirty forms, VoiceOver, 200% zoom and older macOS versions are **not verified** in this assignment. Controlled browser regressions cover transfers and multiple forms; they do not replace these native checks. No conclusion about production v0.1.0 signing/notarization follows from the development-harness incident.

## 9. Redesign-sensitive files

`static/ui.js` owns dirty baselines, carry/recovery, modal departure, question selection and save focus. `static/inspection.js` must offer refresh instead of reloading dirty editors. `templates/job.html` owns authoritative transfer flags/hash/revision fields and explicit submission recording. `application_continuation.go`, `application_answer_transfer.go`, `application_answers.go`, `application_answer_reconciliation.go`, `early_application_answers.go`, `application_answer_policy.go`, browser handlers/session and Chromium `answer.go` retain the safety boundaries. Phase 2 may recompose their presentation, but must preserve these contracts and outcomes. The harness script is development infrastructure, not production release tooling.
