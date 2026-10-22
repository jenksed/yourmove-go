# Application Workbench v1

> **Historical design proposal — authored 2026-10-20.** This document preserves the proposal and its review status from that date; it is not current implementation guidance. The current Prepare flow uses one scoped continuation form and submit action for eligible resume/contact, saved-letter, and saved-answer material. Included transfers remain independently guarded and reported.

**Status:** proposed design, ready for engineering review. Source review: 2026-10-20, including the existing uncommitted continuity work. This document is the only product artifact created in this session. No redesign is implemented or visually qualified by this review.

## 1. Executive design assessment

Build the next-action composition of Prepare first. Keep Overview, Prepare, and History. Prepare becomes the Application Workbench: the place to see what needs attention, review exactly what YourMove can send, edit local materials, and understand what the browser operation observed. It is a workspace with optional tasks, not a mandatory wizard.

The essential continuation capability already exists. `summarizePreparation` chooses **Fill application**, **Attach cover letter and continue**, **Continue application**, or **Fill saved answers and continue**. The combined POST delegates to independently guarded resume/contact, letter, and answer commands. The redesign should expose this capability more clearly rather than rebuild its orchestration. [S2][S3]

The largest composition problems are the permanently expanded resume picker, repeated job and material information, a full letter preview followed by its editor, and many simultaneous question forms below the writer. Resume selection also returns to Overview. Saving a question returns to Prepare without preserving the question being edited. These are verified navigation/markup facts; the resulting cognitive load is a design assessment, not a measured usability result. [S1][S4][S6][S7]

The workbench has three areas, in document order:

1. **Next action and exact scope.** Current blocker or safe action, destination, included materials and values, and one primary transfer button when eligible.
2. **Application materials.** Compact resume/contacts, cover letter, and questions summaries; expand or focus the object the applicant wants to edit. Unresolved questions precede optional writing when they need attention.
3. **Observed results and supporting detail.** Keep uncertainty visible beside the next action. Place successful detail, diagnostics, job brief, and submission recording in named secondary areas. History remains the durable record of the applicant's submission confirmation.

On a wide window, observed results can occupy a compact right column beside the action scope. On narrower windows, important results and warnings move immediately after the action heading, before the scope and button. Use one DOM copy of each value; do not create separate desktop/mobile forms.

### Qualification boundary

The latest native Wails qualification is **BLOCKED** by the isolated Chromium debugging connection failure and unresolved native window targeting. Storage isolation passed for that attempted launch; Gate A as a whole did not pass. Normal-data incident recovery remains unresolved. Earlier passing Go, desktop, race, and controlled Chromium continuity results are historical evidence. The latest attempt did not observe fresh operation counters; unavailable counters are not zero counters. [Q1]

This review does not launch the app, repair qualification, inspect private incident artifacts, change candidate/application records, revisit browser architecture, or expand employer support. Wireframes below are proposed layouts, not captured UI. Native visual and end-to-end acceptance remains unqualified until the existing blocker is resolved in a separate assignment.

## 2. Current workflow and verified sources

Source links identify exact reviewed spans. Ranges describe the current checkout and may move during implementation. Graft supplied discovery, signatures, and relationships; source spans were opened where more detail was needed. HTTP registration was followed directly because Graft's caller edges did not include these handler references.

| Ref | Verified source and responsibility |
| --- | --- |
| S1 | [templates/job.html:4–85](../../templates/job.html#L4-L85): job hero, tabs, resume picker, handoff status, exact scoped continuation form and links. |
| S2 | [application_continuation.go:23–96](../../internal/app/application_continuation.go#L23-L96): independent included commands, partial-failure notices, Prepare redirect and pending key/value filtering. |
| S3 | [application_continuation.go:99–172](../../internal/app/application_continuation.go#L99-L172), [handoff_state.go:6–40](../../internal/app/handoff_state.go#L6-L40): action/title/detail, inclusion flags, pending-answer hash and recovery presentation. |
| S4 | [templates/job.html:87–168](../../templates/job.html#L87-L168), [237–250](../../templates/job.html#L237-L250): writing disclosure, saved editor, generator, questions, reuse and submission confirmation. |
| S5 | [handlers.go:125–204](../../internal/app/handlers.go#L125-L204), [205–276](../../internal/app/handlers.go#L205-L276): resume filtering, submission review token, selected/latest draft, notices, profile, questions and presentation assembly. |
| S6 | [application_handlers.go:54–91](../../internal/app/application_handlers.go#L54-L91), [500–565](../../internal/app/application_handlers.go#L500-L565): handoff eligibility, explicit resume save and redirect, confirmed submission recording. |
| S7 | [application_answers.go:19–172](../../internal/app/application_answers.go#L19-L172), [174–281](../../internal/app/application_answers.go#L174-L281): identity, editability, answer source precedence, current browser inspection and optimistic save/reuse validation. |
| S8 | [application_answer_transfer.go:15–32](../../internal/app/application_answer_transfer.go#L15-L32), [59–132](../../internal/app/application_answer_transfer.go#L59-L132): ready answers, review hash, current-value guards and attempted-answer suppression. |
| S9 | [cover_letters.go:28–97](../../internal/app/cover_letters.go#L28-L97), [182–317](../../internal/app/cover_letters.go#L182-L317): latest/specific draft, edit-in-place save, generation consent/insertion and redirects. |
| S10 | [cover_letter_transfer.go:46–192](../../internal/app/cover_letter_transfer.go#L46-L192): exact body/destination/mode, same-browser arming, duplicate prevention and result distinctions. |
| S11 | [browser_session.go:44–126](../../internal/app/browser_session.go#L44-L126), [249–291](../../internal/app/browser_session.go#L249-L291), [344–369](../../internal/app/browser_session.go#L344-L369), [372–509](../../internal/app/browser_session.go#L372-L509), [510–535](../../internal/app/browser_session.go#L510-L535): session results, selection uncertainty, interrupted state, stable observations, independent operations and cleanup. |
| S12 | [templates/inspection.html:5–66](../../templates/inspection.html#L5-L66), [inspection_handlers.go:61–125](../../internal/app/inspection_handlers.go#L61-L125), [browser_handlers.go:33–135](../../internal/app/browser_handlers.go#L33-L135): inspection presentation, public/browser snapshots, recheck, start, restart consent, stop and redirects. |
| S13 | [static/ui.js:90–148](../../static/ui.js#L90-L148), [151–217](../../static/ui.js#L151-L217): local hash tabs, submit-button guard, provider preferences, generation progress and word count. |
| S14 | [static/inspection.js:1–64](../../static/inspection.js#L1-L64): first watch host, polling, reload, dirty-editor detection and refresh offer. |
| S15 | [templates/partials.html:1–31](../../templates/partials.html#L1-L31), [static/styles.css:385–400](../../static/styles.css#L385-L400), [490–535](../../static/styles.css#L490-L535), [537–609](../../static/styles.css#L537-L609), [625–637](../../static/styles.css#L625-L637), [674–687](../../static/styles.css#L674-L687): shell, facts/outcomes, bounded prose, Prepare composition and responsive rules. |
| S16 | [internal/application/browser/observation.go:62–105](../../internal/application/browser/observation.go#L62-L105), [internal/application/browser/chromium/answer.go:18–120](../../internal/application/browser/chromium/answer.go#L18-L120): definitions-only observations and guarded answer outcomes, including manual values and uncertain readback. |
| S17 | [browser_handlers.go:153–246](../../internal/app/browser_handlers.go#L153-L246), [desktop_redirect.go:63–92](../../internal/app/desktop_redirect.go#L63-L92), [103–174](../../internal/app/desktop_redirect.go#L103-L174), [app_http.go:48–81](../../internal/app/app_http.go#L48-L81): command guards, Wails POST-to-GET adapter, redirect allowlist and actual routes. |
| S18 | [application_handlers.go:141–156](../../internal/app/application_handlers.go#L141-L156), [cover_letter_document.go:15–95](../../internal/app/cover_letter_document.go#L15-L95): generation-source readiness and existing PDF limitations. |
| S19 | [templates/job.html:173–233](../../templates/job.html#L173-L233): job brief, competing review link and submission-only History. [desktop.go:43–49](../../desktop.go#L43-L49): native window size/minimum. |
| S20 | [database.go:211–237](../../database.go#L211-L237), [attachment_downloads.go:10–50](../../internal/app/attachment_downloads.go#L10-L50), [resumes.go:485–492](../../internal/app/resumes.go#L485-L492): mandatory draft resume references, body-only export and referenced-document deletion protection. |
| Q1 | [2026-10-20 application-continuity handoff](../implementation/2026-10-20-application-continuity.md): latest BLOCKED qualification supersedes earlier historical evidence. |

[S1]: ../../templates/job.html#L4-L85
[S2]: ../../internal/app/application_continuation.go#L23-L96
[S3]: ../../internal/app/application_continuation.go#L99-L172
[S4]: ../../templates/job.html#L87-L250
[S5]: ../../internal/app/handlers.go#L125-L276
[S6]: ../../internal/app/application_handlers.go#L500-L565
[S7]: ../../internal/app/application_answers.go#L19-L281
[S8]: ../../internal/app/application_answer_transfer.go#L15-L132
[S9]: ../../internal/app/cover_letters.go#L28-L317
[S10]: ../../internal/app/cover_letter_transfer.go#L46-L192
[S11]: ../../internal/app/browser_session.go#L44-L535
[S12]: ../../templates/inspection.html#L5-L66
[S13]: ../../static/ui.js#L90-L217
[S14]: ../../static/inspection.js#L1-L64
[S15]: ../../static/styles.css#L490-L687
[S16]: ../../internal/application/browser/observation.go#L62-L105
[S17]: ../../internal/app/desktop_redirect.go#L63-L174
[S18]: ../../internal/app/application_handlers.go#L141-L156
[S19]: ../../templates/job.html#L173-L233
[S20]: ../../database.go#L211-L237
[Q1]: ../implementation/2026-10-20-application-continuity.md

### What the screens do today

**Overview:** job description and edit/tracking form, role facts, posting provenance/refresh, and deletion. The hero has Open employer site and Prepare application, regardless of the active tab. The three sections are hash tabs with JavaScript; all sections remain readable without it. [S1][S13]

**Prepare:** resume picker/save and optional family suggestion; action/status block; cover-letter disclosure; current observed questions; submission-record disclosure; job brief. It is effectively one column, capped at 920px, despite a grid wrapper. The writer and questions are bounded around 780px. [S1][S4][S15]

**Cover letter:** a saved draft leads inside the writing disclosure; generation is under Write another version. Without a draft, the generator leads, with provider/model/resume selection and separate consent. There is no empty manual-letter form. Generation inserts a new draft; Save my edits updates the selected draft in place. Earlier versions are separate generation records, not a history of every edit. `?draft=` chooses the working draft for that page; there is no durable “active draft” selection. [S4][S5][S9]

**Questions:** all unresolved editable forms appear simultaneously; noneditable non-file fields get a Chromium instruction. Forms with any nonempty `Source` go into Ready answers and approved candidate details, even a deliberately saved empty answer. Each has its own Save answer POST and optional exact-employer reuse consent. There is no focused question navigation or save-success notice. [S4][S7]

**Inspection:** a separate full page titled Employer application, led by Resume handoff. It has the resume-only handoff route, live browser/resume/contact results, recheck/close/reopen, observations, earlier snapshots and optional public requirements. Its introductory promise mentions contacts, but the displayed handoff form posts resume identity without `transfer_contacts`; actual contacts originate in the scoped Prepare action. Do not copy that introductory promise as authority. Public requirements are not the question editor's data source. [S12][S17]

**History:** confirmed submission records, timestamp, resume name and notes. It is not an audit log of browser transfer events. Do not populate a proposed transfer timeline from submission records. [S19]

### Data and action ownership

- `jobDetail` assembles `jobWorkspace` from saved job/resume/profile/letter/answer data and `a.browsers.state(id)`, then calls `summarizePreparation`. Preserve these owners. [S5]
- Resume eligibility checks job closure/submission, selected file availability/archive and supported destination. Profile approval is required to include contacts, **not** to allow a resume-only first fill. Case A must not introduce a new all-or-nothing candidate-profile gate. [S1][S6][S17]
- The transfer form posts only its displayed flags and identities: destination; resume ID/hash and optional approved profile revision; letter ID/body hash/mode; exact pending-answer review hash. The employer can receive included materials immediately. [S1][S2]
- Answer precedence is explicit application answer, qualified approved profile fact, then exact reusable employer answer. Existing deliberate application overrides remain valid. This proposal adds no AI matching, inferred answers, or alternate precedence. [S7]
- `ReadyAnswers()` uses editable plus nonempty value, then sorts by key. `pendingAnswers` removes already attempted identical key/value pairs and contacts already owned by the contact operation. Reuse “ready” only for actual eligible scope; a source label alone is insufficient. [S2][S8]
- Browser transfer statuses and attempted values belong to the current in-memory session. SQLite holds local preparation and inspection definitions. Closing clears session operations; interruption preserves only the explicitly copied resume/snapshot context. Do not promise durable transfer receipts, letter results after interruption, or a complete outcome history after relaunch. [S11]
- A letter in `review` may be eligible again after a nonselection/review result. `attention` means an uncertain attempt and blocks automatic duplicate transfer. A completed letter also blocks another automatic letter operation in that session, even if the local draft is subsequently edited or a different draft is opened. Saving is not permission to replace the employer attachment. [S3][S10]
- Definitions-only browser snapshots contain no employer values. Unanswered locally means no local prepared answer, not an empty employer control. Transfer outcomes can report “Existing user answer preserved”; their labels/details do not provide the value itself. Answer outcomes have labels, not stable question keys: keep them in an outcome list; do not join them to editors by fuzzy or duplicate label. [S7][S16]
- Answer operation `complete` means its batch finished, including manual or uncertain per-field outcomes; it does not mean every answer succeeded. [application_answer_transfer.go:133–170](../../internal/app/application_answer_transfer.go#L133-L170)

### Navigation and refresh boundaries

| Interaction | Current behavior | Design treatment |
| --- | --- | --- |
| Overview / Prepare / History | Hash change/local visibility with JS; ordinary document anchors otherwise. | Preserve; no new top-level tab. |
| Save resume / use suggestion | POST, then full GET `/jobs/{id}`; JS defaults to Overview. | Small separately identified return-location fix; preserve explicit save. |
| Generate letter | POST, waits for generation, full GET with `cover=generated#prepare`; latest draft shown and writer open. | Preserve server operation and consent; focus result after navigation. |
| Save letter | POST, full GET `?cover=saved&draft=…#prepare`; writer open. | Preserve selected draft and saved/unsaved distinction. |
| Earlier draft | Full GET `?draft=…#prepare`. | Keep explicit, guard dirty work before leaving. |
| Save answer | POST, full GET `#prepare`; same question focus not retained. | Add bounded return-question context in the focused-editor slice. |
| Fill / continuation | POST `/continue`, full GET `#prepare`, optional draft/per-item notices. | Preserve exact scope and independent results. |
| Inspection / profile / documents / AI connections | Full page navigation; employer window is separate. | Make diagnostics optional; do not require visiting them for routine local editing. |
| Recheck / start / restart / stop | Browser operation POST, full GET Inspection. | Reuse intact; recovery can remain there in v1. |
| Browser polling | Every 2.5s while visible; action/result changes can reload; dirty answer/letter input offers refresh instead. First watch host wins. | Preserve; one intentional watch host in the redesigned Prepare. |
| Record completed application | Explicit confirmation/token POST, full GET job page. | Preserve; user submission is separate from this local record. |

Wails replaces allowed 303s with a safe HTML redirect using GET (script plus meta/link fallback). Any new return query requires a narrow allowlist addition and native redirect tests; do not replace this with an assumed browser-only fetch flow. [S13][S14][S17]

## 3. User friction and design response

| Verified friction | Consequence / proposed response |
| --- | --- |
| Resume selection UI remains expanded after selection and during active filling. | Repeated setup dominates continuation. Show filename + Download + Change resume disclosure when selected; expand for missing/invalid selection. Show separately what was selected in the employer browser if the assignment later changes. |
| Hero Prepare button, Prepare heading, resume-selected badge, repeated company/title/URL and job-brief review CTA compete with the actual action. | Keep one job identity and tabs. Make the hero's Prepare link secondary; remove its redundant primary treatment on Prepare. Keep exact destination near transfer authority. Drop the job-brief Review application button when the action area already provides that route. |
| Letter body is a transfer preview above a second full editable body. | Retain exact saved text in the transfer scope, bounded and scrollable. Open the editor only on request or a save/generation result; label it unsaved when changed. Do not present two default-expanded reading surfaces. |
| Questions follow the optional writer and render many save forms. | Lead with question attention summary when present; use one focused editor with a question index. Keep optional writing accessible from its material row. |
| `.Source` groups saved empty answers as ready. | Derive local missing-answer counts from editability and actual value, consistent with `ReadyAnswers`; no lifecycle enum. |
| Save resume returns to Overview; answer save loses question context. | Preserve Prepare return and stable question focus through small validated navigation changes. Neither operation sends material. |
| Answer outcome header says “Transferring…” for every non-complete status. | Display actual status and raw per-item details, including attention/manual/uncertain; never infer ongoing work from a nonempty status. |
| Technical resume handoff page duplicates Prepare but lacks the letter/answer composition. | Make Prepare the routine workbench; retain Inspection for browser controls and evidence. Add a clear Return to Prepare link and less resume-centric diagnostics wording. Preserve the distinct resume-only form scope. |
| Dirty-editor protection covers poll reloads, not every full navigation or another form's POST. | Add a small common dirty-form guard for edits before leaving/submitting a different form. Do not silently save, transfer, or persist draft text in web storage. |

These are source-based hypotheses to validate with real paths and future screenshots, not claims of observed completion-time improvements.

## 4. State-by-state interaction matrix

A–I are design review cases, not new application lifecycle values. They overlap: an uncertain letter can coexist with successful answers and further eligible material. Use existing presentation flags for the transfer action; place each unresolved result next to that action. Do not make the aggregate look atomic.

“Review in Chromium” below is the applicant's manual window-switch task. There is no inspected endpoint to focus an existing employer window. Render an instruction, not a button secretly opening a second application or restarting the browser.

| Case and existing truth | Primary action | Supporting actions | Always visible | Disclosure / errors and uncertainty |
| --- | --- | --- | --- | --- |
| **A. New application** — `SelectedResume`, `Handoff.Allowed/Reason`, profile approval, stage; S1/S6/S17. | No resume: **Save resume**, or **Use suggested resume** when one confident suggestion is offered. Profile incomplete alone: **Fill application** remains eligible for resume-only scope. | Manage documents; approve candidate information; open employer site manually. If no resumes exist, Open documents becomes the setup action. | Missing prerequisite; selected destination; explicit save boundary. Unapproved contacts say “Contacts not included.” | Optional letter/settings and job brief. Invalid/archived/missing document reason stays expanded. Never imply that missing profile prevents resume-only filling. |
| **B. Ready for first fill** — `.Preparation.IncludeResume` and other flags; approved profile may be included; S3. | **Fill application**. | Change/download resume; edit approved facts in profile; start letter; inspect only if needed. | Exact destination, resume, approved contact values, and any included letter/pending answers; immediate-receipt and personal-submission notice. | Technical evidence. Stale identity/revision is rejected by existing command; show affected-item notice, not a generic all-failed result. |
| **C. Initial fill completed** — active matching session, resume/contact statuses, no letter; S3/S11. | Review the selected attachment and remaining form in Chromium. If unanswered local questions exist, **Review questions** is the primary local control. | Start cover letter; recheck through browser details; download saved material. | Resume result and employer-acceptance caveat; contact exceptions; whether observer is waiting/checking; local questions summary. | Successful field detail and diagnostics. “File selected” never becomes “Employer accepted.” Waiting/checking is ongoing, not completed. |
| **D. Late letter saved** — active session, `.Letter.ID`, `.IncludeLetter`, no other pending included material; S3/S10. | **Attach cover letter and continue** using the existing mode selector. For text mode, retain the action but explicitly show “Fill saved letter text” in scope. | Edit draft; download saved PDF; earlier versions; browser details. | Draft ID, exact saved body, PDF/text mode, matching destination and unchanged previous result context. | Sources/generation settings. Local save notice says saved locally; post-operation message says selected/observed/uncertain as actually reported. |
| **E. Multiple pending materials** — `IncludeLetter`, `IncludeAnswers`, exact `PendingAnswers`; S2/S3/S8. | **Continue application**. | Revise letter/answers; inspect employer form. | Exact letter/version/mode and every pending question/value; independent outcome rows; previously transferred material excluded from this scope. | Previous successful details. One rejected item does not roll back another; show per-item notices and outcomes. |
| **F. Questions need attention** — latest browser `InspectionID`, `Questions`, `Previous`, ready/pending subset and raw outcomes; S7/S8/S16. | **Review questions**, then **Save answer locally** in the focused task. If scope is already eligible, the existing continuation remains available through **Review ready material**. | Previous/Next; question index; candidate-profile link; manual completion in Chromium. | Local missing/ready distinction, exact observed question/options, required marker, source and unsaved status; “Saving does not fill the employer form.” | Ready candidate facts, exact-employer reuse consent, earlier unmatched answers and technical identity. Unsupported/sensitive controls stay manual. Existing manual values are reported only from actual outcomes; never pre-inferred. |
| **G. Uncertain document transfer** — resume selected with unverifiable message or attention; letter `attention`; S10/S11. | Inspect the existing employer attachment in Chromium. Primary local control: **Browser details and troubleshooting**. | Download local document; recheck observationally; continue unrelated eligible material if presentation allows it and scope is explicit. | Which document, what was attempted/observed, acceptance unknown, no automatic resend. Other independent successes remain visible. | Raw diagnostics. Do not show Retry upload. Existing **Retry resume check (no file sent yet)** is available only when `ResumeRetryable` proves preflight nonselection. Letter `review` is not synonymous with uncertain selection. |
| **H. Browser unavailable / unobservable** — interrupted inactive session, active attention, or changed destination; S3/S11/S12. | **Review browser recovery** links to existing Inspection controls. In an inactive interrupted browser, existing **Open application (no automatic upload)** is available there; active restart retains explicit loss confirmation. | Review/download locally saved work; return to correct application manually when still open; deliberate reopen. | Browser-only work may be lost or unavailable; local saves remain; last observed context is historical; no automatic document replay. | Snapshot history. Poll network failure alone says live status unavailable, not “browser closed.” No re-open-on-page-load, and no synthesized letter/contact/answer receipt after interruption. |
| **I. Human review** — no currently eligible transfer action; inspect independent results, manual/uncertain items and browser availability; S3/S11/S19. | Review, complete manual fields, advance steps and submit personally **in Chromium**. There is no YourMove submit-to-employer control. | Recheck; revise local materials; **Record completed application** after actual submission. | Remaining manual/uncertain items, selection/acceptance distinction, incomplete observation caveat and personal submission authority. | Successful detail, diagnostics and recording form until intentionally opened. No new “ready to submit” certification or requirement to dismiss manual items in SQLite. |

Closed/applied/already-recorded jobs are an overriding boundary, not case I. Show the existing history/closed reason and no transfer button. Unsupported destinations keep the existing Open application/manual-download route. Opening, waiting and checking suppress repeated authorizations according to current guards; do not fabricate a cancel or retry-all operation. [S3][S6]

## 5. Wireframes and layout contract

All names, filenames, draft IDs, question text and counts below are illustrative synthetic content. Counts describe observed/local items only. Brackets indicate controls; `▸` is disclosure; text saying “Switch to Chromium” is an instruction. Border lines explain grouping, not a request to draw nested cards around every region.

### 5.1 Initial Prepare — no resume selected

```text
┌───────────────────┬──────────────────────────────────────────────────────────┐
│ YOURMOVE          │ Senior Go Engineer · Northstar                          │
│ Opportunities     │ Overview    Prepare ━━━    History     Open employer ↗  │
│ Documents         ├──────────────────────────────────────────────────────────┤
│ Candidate profile │ Choose a resume                                         │
│ Applications      │ Save your choice before filling the employer form.      │
│ AI connections    │                                                          │
│ Utilities         │ Resume  [Choose a document                         ▾]   │
│                   │ [Save resume]       Manage documents                    │
│                   │ Suggested: backend.pdf · Backend engineering             │
│                   │ Title phrase: “Go Engineer”   [Use suggested resume]     │
│                   │ (When offered, this is the primary setup action.)       │
│                   │                                                          │
│                   │ Contacts not included · Candidate information unapproved│
│                   │ Approve candidate information →                         │
│                   │ Resume-only filling is available after selection.       │
│                   │ Destination: jobs.ashbyhq.com/northstar/…                 │
│                   ├──────────────────────────────────────────────────────────┤
│                   │ Cover letter   Not started · Optional     Start letter →│
│                   │ Questions      Not observed yet                          │
│                   │ ▸ Job brief             ▸ Browser details                │
└───────────────────┴──────────────────────────────────────────────────────────┘

┌─────────────────────────────────────┐
│ YOURMOVE                 Appearance │
│ Opportunities  Documents  …         │
│ Senior Go Engineer                  │
│ Northstar                           │
│ Overview  Prepare ━━━  History      │
├─────────────────────────────────────┤
│ Choose a resume                     │
│ [Choose a document              ▾] │
│ [          Save resume           ] │
│ Manage documents                    │
│ Contacts not included               │
│ Approve candidate information →     │
│ Resume-only fill remains available. │
│ Destination: jobs.ashbyhq.com/…      │
├─────────────────────────────────────┤
│ Cover letter · Optional   Start →   │
│ Questions · Not observed yet        │
│ ▸ Job brief    Browser details →    │
└─────────────────────────────────────┘
```

When a suggestion exists, avoid two equally prominent setup buttons: Use suggested resume leads, Save resume is secondary for a manually chosen alternative. Once saved and eligible, replace the setup heading with the existing ready presentation and exact first-fill scope, then **Fill application**. Contacts remain optional as described in case A.

### 5.2 Application in progress — active browser, no letter

```text
┌───────────────────┬──────────────────────────────────────────────────────────┐
│ YOURMOVE          │ Senior Go Engineer · Northstar                          │
│ Opportunities     │ Overview    Prepare ━━━    History                      │
├───────────────────┼─────────────────────────────────┬────────────────────────┤
│                   │ Review the employer form        │ Observed in Chromium   │
│                   │ Switch to the existing Chromium │ Resume: file selected  │
│                   │ window to inspect attachments   │ backend.pdf            │
│                   │ and complete manual questions.  │ Acceptance: check with │
│                   │                                 │ the employer           │
│                   │ Application questions           │ Contacts: 3 fields     │
│                   │ 2 need a local answer           │ filled/observed        │
│                   │ [Review questions]              │ ▸ Successful details   │
│                   │                                 │ Browser details →      │
├───────────────────┼─────────────────────────────────┴────────────────────────┤
│                   │ Resume       backend.pdf       Download   ▸ Change      │
│                   │ Cover letter Not started       Start letter →           │
│                   │ Questions    2 local answers missing; 1 manual control  │
│                   │ ▸ Job brief                 ▸ Record completed application│
└───────────────────┴──────────────────────────────────────────────────────────┘

┌─────────────────────────────────────┐
│ Senior Go Engineer · Northstar      │
│ Overview  Prepare ━━━  History      │
├─────────────────────────────────────┤
│ Review the employer form            │
│ Resume: backend.pdf selected        │
│ Check employer attachment acceptance│
│ Contacts: 3 fields filled/observed   │
│ Switch to existing Chromium window. │
│                                     │
│ 2 questions need a local answer     │
│ [       Review questions         ] │
│ Cover letter · Start letter →       │
│ Resume · Download   ▸ Change        │
│ Browser details →                   │
│ ▸ Record completed application      │
└─────────────────────────────────────┘
```

If waiting for a later employer field, the existing waiting message replaces the review heading; the applicant advances employer steps manually. Do not report all work complete merely because the resume was selected.

### 5.3 Late cover letter — saved locally, continuation ready

```text
┌───────────────────┬──────────────────────────────────────────────────────────┐
│ YOURMOVE          │ Senior Go Engineer · Northstar                          │
│ Opportunities     │ Overview    Prepare ━━━    History                      │
├───────────────────┼─────────────────────────────────┬────────────────────────┤
│                   │ New material ready              │ Earlier browser results│
│                   │ Same employer application       │ Resume: file selected  │
│                   │ jobs.ashbyhq.com/northstar/…      │ Contacts: observed     │
│                   │                                 │ These are outside this │
│                   │ Saved cover letter · Draft 12   │ continuation's scope.  │
│                   │ ┌─────────────────────────────┐ │                        │
│                   │ │ Dear Northstar team,        │ │ Browser details →      │
│                   │ │ …exact saved letter body… ↕ │ │                        │
│                   │ └─────────────────────────────┘ │                        │
│                   │ Employer cover-letter control   │                        │
│                   │ [Attach saved letter as PDF  ▾] │                        │
│                   │ Employer may receive it now.    │                        │
│                   │ You review and submit personally│                        │
│                   │ [Attach cover letter and continue]                       │
├───────────────────┼─────────────────────────────────┴────────────────────────┤
│                   │ Cover letter · Draft 12 saved locally    Edit letter →  │
│                   │ Download saved PDF     ▸ Earlier versions               │
│                   │ Questions · No new prepared answers to fill             │
└───────────────────┴──────────────────────────────────────────────────────────┘

┌─────────────────────────────────────┐
│ Overview  Prepare ━━━  History      │
│ New material ready                  │
│ Existing employer session           │
│ jobs.ashbyhq.com/northstar/…         │
│ Earlier: resume selected, contacts  │
│ observed · check employer acceptance│
│ Saved cover letter · Draft 12       │
│ ┌─────────────────────────────────┐ │
│ │ Dear Northstar team,           ↕ │ │
│ │ …exact saved text…              │ │
│ └─────────────────────────────────┘ │
│ [Attach saved letter as PDF     ▾] │
│ Employer may receive it immediately│
│ You review and submit personally.   │
│ [ Attach cover letter and continue ]│
│ Edit letter →    Download saved PDF │
│ ▸ Earlier versions                 │
└─────────────────────────────────────┘
```

The bounded preview contains the full text with keyboard scrolling; no ellipsis replaces real scope. If supported answers are also pending, insert every exact question/value before the button and use **Continue application**. Do not repeat earlier contact values as if they will be sent again.

### 5.4 Focused application-question editor

```text
┌───────────────────┬──────────────────────────────────────────────────────────┐
│ YOURMOVE          │ Overview    Prepare ━━━    History                      │
├───────────────────┼──────────────────────────────────────────────────────────┤
│                   │ Application questions                  Back to Prepare ↑│
│                   │ Observed employer form · Later steps may add questions. │
│                   │ 2 need local answers · 1 saved · 1 manual               │
│                   ├───────────────────────┬──────────────────────────────────┤
│                   │ Needs local answer    │ Question 1 of 3 editable questions│
│                   │ › Portfolio URL       │ Portfolio URL (required)         │
│                   │   Why this role?      │ [https://alex.example           ]│
│                   │                       │ Unsaved changes                  │
│                   │ Saved locally         │ Saving does not fill the employer│
│                   │   Availability        │ form.                            │
│                   │                       │ [Save answer locally]            │
│                   │ Manual in Chromium    │ ▸ Reuse for this exact question  │
│                   │   Work authorization  │   at this employer               │
│                   │ ▸ Candidate facts     │ [Previous: disabled]  [Next →]  │
│                   ├───────────────────────┴──────────────────────────────────┤
│                   │ Ready material: 1 pending answer · Review ready material ↑│
│                   │ Unsaved text is excluded from continuation.             │
│                   │ ▸ Transfer outcomes   ▸ Earlier unmatched saved answers │
└───────────────────┴──────────────────────────────────────────────────────────┘

┌─────────────────────────────────────┐
│ Overview  Prepare ━━━  History      │
│ Application questions               │
│ Back to Prepare ↑                   │
│ 2 need local answers · 1 saved       │
│ [Question 1 of 3: Portfolio URL ▾]  │
│ Portfolio URL (required)             │
│ [https://alex.example             ]│
│ Unsaved changes                     │
│ Save locally; fill separately.       │
│ [       Save answer locally       ]│
│ [Previous: disabled]      [Next →] │
│ ▸ Reuse at this employer            │
│ Manual: Work authorization          │
│ Complete this in Chromium.          │
│ Review ready material ↑             │
│ ▸ Approved candidate facts          │
│ ▸ Transfer outcomes                 │
└─────────────────────────────────────┘
```

The index and narrow chooser refer to the same ordered forms; previous/next traverses the application-answer queue in observed order, not hash-sort transfer order. Approved profile facts sit in their separate disclosure by default; their explicit override editor remains accessible. Manual items remain visible in the index/summary but are not fake answer forms. Counts exclude document controls and label approved facts separately. Without JavaScript, all original forms and manual instructions remain usable in document order.

### 5.5 Uncertain attachment / browser loss

```text
┌───────────────────┬──────────────────────────────────────────────────────────┐
│ YOURMOVE          │ Overview    Prepare ━━━    History                      │
├───────────────────┼──────────────────────────────────────────────────────────┤
│                   │ Cover-letter transfer could not be verified             │
│                   │ Draft 12 was attempted. Inspect the existing employer   │
│                   │ attachment in Chromium before changing anything.        │
│                   │ YourMove will not resend this uncertain attachment.     │
│                   │ [Browser details and troubleshooting →]                 │
│                   │ Download saved letter PDF                               │
│                   │                                                          │
│                   │ Independent result: saved answers filled and observed.  │
│                   │ Resume: earlier file selection; acceptance unconfirmed. │
│                   │ ▸ Technical observation                                 │
├───────────────────┼──────────────────────────────────────────────────────────┤
│                   │ If browser becomes unavailable, replace the above with: │
│                   │ Application browser closed                              │
│                   │ Browser-only answers or uploads may have been lost.     │
│                   │ Local resume, letter and saved answers remain available.│
│                   │ [Review browser recovery →]                             │
│                   │ No automatic resend. Last snapshot is historical.       │
└───────────────────┴──────────────────────────────────────────────────────────┘

┌─────────────────────────────────────┐
│ Overview  Prepare ━━━  History      │
│ Application browser closed          │
│ Browser-only work may be lost.      │
│ Your saved materials remain local.  │
│ [    Review browser recovery →    ]│
│ No attachment will resend itself.   │
│ Resume: last observed file selection│
│ ▸ Last captured form                │
│ Download resume / saved letter      │
└─────────────────────────────────────┘
```

These are alternative views, not stacked production warnings. Recovery stays on the existing Inspection page for v1: inactive open is read-only; active restart requires the existing confirmation of possible loss. `ResumeRetryable` exposes the specifically safe preflight check, never a generic attachment retry. If only status polling fails, keep the current page and say live status is unavailable; do not substitute the browser-closed view.

### 5.6 Final human review

```text
┌───────────────────┬──────────────────────────────────────────────────────────┐
│ YOURMOVE          │ Overview    Prepare ━━━    History                      │
├───────────────────┼──────────────────────────────────────────────────────────┤
│                   │ Review and finish in Chromium                           │
│                   │ Supported work has results below. Review the employer's │
│                   │ form, complete manual questions, and submit personally. │
│                   │                                                          │
│                   │ Resume       File selected · verify employer acceptance │
│                   │ Contacts     Filled/observed · review exceptions         │
│                   │ Cover letter Draft 12 · text filled and observed         │
│                   │ Answers      Filled / already matches / manual outcomes │
│                   │ Manual       Work authorization · complete in Chromium  │
│                   │ Later steps and embedded controls may remain unknown.   │
│                   │ Browser details →               Edit local materials →  │
│                   ├──────────────────────────────────────────────────────────┤
│                   │ ▾ Record completed application                          │
│                   │ This records your confirmation, not employer receipt.   │
│                   │ Notes [Confirmation number or follow-up               ] │
│                   │ □ I confirm I submitted on the employer website.        │
│                   │ [Record completed application]                          │
└───────────────────┴──────────────────────────────────────────────────────────┘

┌─────────────────────────────────────┐
│ Overview  Prepare ━━━  History      │
│ Review and finish in Chromium       │
│ Review attachments and manual fields│
│ Submit personally in that window.   │
│ Resume · selected, check acceptance │
│ Cover letter · text observed        │
│ Answers · review per-field outcomes │
│ Manual · Work authorization         │
│ Later steps may remain unknown.     │
│ Browser details →                   │
│ ▸ Record completed application      │
└─────────────────────────────────────┘
```

The desktop recording disclosure is shown open to illustrate its controls, not because the app can detect submission. It starts closed. Opening it makes Record completed application the local decision's primary; it never becomes an employer-submit button. No “all required fields complete” badge.

### Responsive and interaction details

- Reuse the current shell and theme tokens. Start with the actual 1280px browser viewport and the native configured window; native minimum is 700×550. A 390px browser layout is an additional responsive target, not a claim that the native window can shrink to 390px. [S15][S19]
- Aim for a 240–280px result/index column only when the remaining main column can support readable text (roughly a 1040px viewport with the current sidebar). Collapse based on available space; validate at 1050, 800, 700 and 390px instead of forcing a desktop split into the Wails minimum.
- Keep prose and editors within approximately 65–90 characters per line and 780px maximum. Maintain 42px form-control height, visible labels/focus, 8/12/16/24px spacing, existing light/dark variants and reduced motion.
- Use normal page flow. No fixed bottom transfer bar detached from scope, no nested scrolling for the whole workspace. A long exact letter preview can have its own clearly labeled, keyboard-focusable bounded viewport; text wraps, filenames/URLs wrap anywhere. Pending answer values are all available before the transfer action; long batches may require scrolling. Do not invent a shortened scope to force a button above the fold.
- In ordinary compact scopes, target the next decision in the first viewport after the tabs. On narrow screens, warnings precede action; material setup controls appear only when needed. The focused question task has local Save as its visual primary; Review ready material navigates to the existing scoped action, rather than duplicating the transfer form.
- Use links/buttons with clear action verbs. Keep existing transfer labels to preserve recognizable scope. Rename **Save answer** to **Save answer locally** to clarify the boundary; retain **Save my edits** with an adjacent saved/unsaved indicator. “Observed results” describes session evidence more honestly than “Completed application.”

## 6. Focused application-question editing proposal

### Selection and ordering

Enhance the existing server-rendered forms; retain one authoritative form per question and unchanged hidden identity/revision fields. The client reveals one editable form at a time and an index. No framework, matching service, batch save, or AI answer generator.

Default to the first editable question with an empty local value, including explicitly saved empty values. If none, show the first editable application answer; if only approved candidate facts/manual controls remain, show their summary instead of an empty editor. The default Previous/Next queue excludes profile-derived facts in the separate disclosure, while a deliberate application override remains accessible. Exclude file/cover-letter controls from the answer count because those have separate document handling. Required/optional markers come from observations, not assumptions about employer completion. Preserve observed order within that queue and observation; saving an answer does not change its Previous/Next neighbors even if its local category changes. Previous/Next never wraps and disables at each end. A filter/index jump does not silently redefine next order.

Local categories are calculated for display from existing fields: editable without value, nonempty prepared value with its actual source, approved candidate fact, and manual control. They are not lifecycle enums or persisted review flags. Do not subtract local saved counts from an employer-wide “remaining” total. The currently observed form is partial.

### Saving and revising

Previous/Next and selecting a question switch local DOM panels without submitting. Keep edited inputs in their existing DOM forms so navigating between questions does not lose text. Show Unsaved changes on affected index items. **Save answer locally** submits only that question through the existing endpoint. No automatic transfer follows a save, and no unsaved DOM value enters the ready scope.

For predictable post-save focus, add narrowly validated presentation context: an optional `question=<exact current key>` on the canonical job GET, with the POST returning to that key after a successful save. The renderer accepts only a key present in its freshly prepared question list; otherwise it falls back to the first locally unanswered editable question and explains that the observed question changed. Keep `#prepare` for the existing workspace tabs. A hidden return key can be posted, but it confers no answer authority. The desktop redirect validator must admit only a canonical single key of the actual hash shape and permitted existing draft/notice combinations, with wrong job/key, foreign origin, duplicates and unknown parameters tested. This is small backend/navigation work, not new storage. Until implemented, the first composition slice retains the current save redirect.

After save, remain on the same question with a confirmed Saved locally message; Next is separate and predictable. Reopening a saved question uses its current saved revision and permits editing. Preserve exact observed select/radio labels, the explicit unanswered option and checkbox true/false semantics. Saving blank clears the application answer value; do not claim a local answer deletion API or clear the employer field.

Existing approved facts remain authoritative facts from Candidate profile. Show their source and a link to change that profile. If the applicant deliberately chooses **Edit answer for this application**, expose the existing per-application editor and explain that saving creates an application-specific override without modifying approved profile facts. Preserve existing explicit-override precedence; do not silently rewrite candidate data or offer profile editing as question matching.

Move the long reuse checkbox into a named disclosure, keeping its unchecked default, exact-employer/exact-question explanation, hidden reusable revision and explicit opt-in. No general memory toggle. A saved application edit leaves the reusable original unchanged unless opted in again.

### Manual values and current observations

Unsupported, unstable, duplicate-ID, sensitive and cover-letter controls retain existing abstention. Manual entries are completed in Chromium. A saved local answer does not certify that the employer field is eligible or blank. During transfer, preserve the existing independent live-identity, option, current-value, active-control and readback checks. [S7][S16]

Show answer outcomes as their own labeled list. `filled` and `correct` mean observed entry/already matching; manual/unsupported/uncertain details remain visible. The list cannot reliably decorate a particular question editor by label alone. A key-linked per-question outcome badge would require an actual data-contract requirement and is deferred; no such backend extension is needed for this design.

When polling finds a new observation, offer **Refresh discovered questions**. Dirty work never triggers automatic refresh or silent rematching. Let the applicant save current work first. On refresh, rebuild from the current inspection: changed identity/options become different questions; previously saved unmatched work stays in the existing Earlier saved answers needing review disclosure and is excluded from transfer. Saving an old question can legitimately return 409 if it has disappeared; do not force it into the new form. Later conditional questions become available only after the applicant advances the employer form and a stable observation is saved. Public API requirements never enter this editing list. [S7][S11][S14]

### Error and dirty-work handling

The initial layout change keeps current server errors. The focused-editor slice should render save conflicts/validation failures in an editing response with the submitted text still available, an explicit Not saved message, and a way to reload current data. This requires a small handler rendering change; it must not turn submitted text into the saved preparation/transfer source. On an error response, show the editor plus error without an actionable transfer form until a fresh canonical Prepare GET.

Enhance dirty handling for both editors and unsaved resume choice. For in-app navigation or another form's POST, offer Save first / Stay / Discard; Save first returns focus to the dirty editor for explicit saving, Stay cancels navigation, and Discard proceeds deliberately. Browser/window exit uses the platform's ordinary before-unload leave/stay protection; do not promise custom native prompt buttons. Do not auto-submit another dirty form. Within question navigation, retain dirty fields and allow switching. Polling currently marks any editor input dirty and does not reset until navigation; retain that conservative behavior until successful save navigation. The one watch host must retain all four operation statuses and observation ID. Do not clone it per question.

## 7. Cover-letter editing proposal

### Existing draft

Open Edit letter from the material row as a focused section within Prepare, with **Back to Prepare** returning to the next-action area rather than leaving the job. Keep the saved artifact first, a 780px writing measure, word count, and **Save my edits**. Show **Draft N · Saved locally [edited/created timestamp]** from existing fields; after typing show **Unsaved changes**. A timestamp is save evidence, not remote receipt. No autosave claim or edit-version history claim.

Keep Download **saved** letter PDF explicit. While dirty, warn that download and continuation use saved text; the applicant saves or deliberately discards before moving to the transfer context. Place AI connections, provider/model and Write another version after the editor in disclosure. The selected provider and nonlocal data-sharing consequence must still be visible at the Generate button when generation is open. Consent stays explicit and separate from local save and transfer.

After a normal save redirect, show the saved notice, draft ID and an immediate **Review ready material** link to the next-action block. That block retains the full exact saved body and PDF/text mode before the eligible continuation button. There is no implicit transfer on return. After transfer, show the existing `LetterMessage` and independently reported outcomes; selection or text readback never becomes employer acceptance.

If a letter is already complete or uncertain in this session, editing remains available locally but the workbench must not promise automatic replacement. Show that the existing attachment/entry requires manual review; a new local save does not reset duplicate protection. When `review` describes a proven nonselection or stale draft, retain that specific explanation and let existing eligibility determine any offered authorization. [S10]

### Starting a letter

The empty workspace should offer two choices: **Write manually** and **Generate my letter**. Until manual creation is implemented, only the actual Generate path can be offered; “generate or edit” must not imply a supported manual start.

Manual creation is the one clear new local capability justified by this design. It needs a narrow local create POST in `cover_letters.go` and registration in `app_http.go`; reuse the existing table, body validation, download and transfer paths. Check active job eligibility and local request protections. Never invoke generation or require provider credentials/consent. An unsaved blank editor exists only in the current document; the first valid Save creates a normal draft and returns via the existing saved-draft Prepare URL.

**Verified schema constraint:** `cover_letters` requires non-null master and role resume foreign keys plus non-null source-hash strings. The current readers scan those IDs directly into `int64`. A source-independent manual draft cannot simply insert missing IDs or nullable values. Zero/dummy IDs or fabricated source hashes are not acceptable. [S9][S20]

The smallest v1 implementation is therefore **manual authorship with real application context**: require an active master resume and selected active role resume; pin their actual IDs/file hashes and the current job-description hash when creating the record. No source text extraction, nonempty description, model or provider connection is required for manual writing. Use `provider=manual`, `model=''`, and render the provenance as “Written manually · application context recorded,” not AI generation evidence. This is provenance metadata, not a new generation provider option; hide empty model separators in draft rows. Show the document prerequisites before opening a new manual editor; they constrain saving a new record, not editing an existing one. Download and transfer already operate on saved body rather than provider identity. Keep existing referenced-resume deletion protection. All later edits remain edit-in-place. Test these consumers and the schema's actual foreign-key enforcement without changing constraints. [S10][S20]

This is a deliberate compromise imposed by the current record model. If product review requires saving a manual letter without those resume references, separate schema/reader work is necessary and outside the visual v1 scope. Do not smuggle that migration into the layout slice. Prefer that explicit review over making users generate a disposable AI letter to unlock editing.

Generation remains the current handler and new-draft behavior. Show source readiness and a concise provider choice near the consent/action; disclose model and evidence detail as needed. Missing readable source text blocks generation, not editing an existing draft; manual creation has only the record-context prerequisites above. If a different writing resume is chosen, retain the existing explanation that it does not change the resume sent to the employer. No new provider defaults, fallback, credentials UI, provider expansion or library changes. [S4][S9][S18]

For save validation failures, retain submitted letter text in an error editor response with Not saved and no actionable transfer scope, as for question errors. Do not put unsaved body into `workspace.Letter` and accidentally recompute a transfer hash for text that is not in SQLite. Keep existing PDF character/format errors specific; supported text mode or manual attachment remain alternatives, never silent text substitution. [S10][S18]

## 8. Existing primitives and components to reuse

| Existing primitive | Workbench use / boundary |
| --- | --- |
| `jobWorkspace`, `HandoffPresentation`, `summarizePreparation`, `summarizeHandoff` | Continue deriving eligible transfer labels/flags and recovery text. No workbench lifecycle type. Add derived counts/helpers only if template logic becomes unclear; no new service/interface. |
| `browserRunState`, existing operation messages/outcomes | Show actual session evidence. Do not infer acceptance, durable receipts or aggregate success. |
| `questionPreparation`, `ReadyAnswers`, `pendingAnswers`, review hashes | Local question source, exact continuation subset and stale-scope protection. UI order need not be transfer hash order. |
| `application-question` template and existing per-question form | Enhance focus/index; keep native options, identity/revisions and exact-employer consent. |
| `contact-facts`, `contact-outcomes` | Reuse exact approved values and exceptions; put optional successful detail behind disclosure when useful. Do not change outcome semantics. |
| Writer form, `LetterNotice`, selected draft URLs, source disclosures | Artifact-first editor, explicit save, selected draft preservation, secondary regeneration. |
| Existing continue POST and material commands | Preserve independent execution and individual outcomes. No combined atomic transaction or retry-all. |
| Hash tabs, native `<details>`, buttons/fields, theme and spacing tokens | Progressive enhancement, shared shell, bounded prose and scoped layout changes. No component-library project. |
| `inspection.js` polling and dirty guard | One watch host; refresh offers for changed observations; server-authored action scope after navigation. Extend conservatively. |
| Inspection browser forms and Wails redirect adapter | Keep recovery authority and GET navigation. Reuse routes rather than adding browser focus/recovery behavior. |
| Submission confirmation token/form and History | Keep final applicant confirmation and durable submission records separate from transfer feedback. |

The existing [design foundations](DESIGN_SYSTEM.md) and [R1/R6–R10](RULES.md) govern composition. Their final foundations paragraph still says resume/contact, letter and answer approvals are separate; current S2/S3 supersede that historical description. Updating that sentence can accompany implementation, but does not justify another orchestration change.

## 9. Smallest practical implementation slices

Ordered by user value and technical risk. Each slice ships behind the existing routes and can be reviewed independently. File lists are impact estimates, not authorization to implement during this review.

| Priority | Concrete slice and independent value | Likely files | Risk / backend work |
| --- | --- | --- | --- |
| **1 — build first** | **Make the existing continuation action and observed result easy to find.** Keep missing/invalid resume setup expanded; otherwise show selected filename/Download and Change resume disclosure. Lead with current action/status, preserve exact scoped form and warnings, keep saved writer collapsed unless intentionally opened, and demote duplicate hero/job-brief primary links. Add an obvious local link from saved-letter notice to the existing action block. | `templates/job.html`, scoped rules in `static/styles.css`; relevant expectations in `ui_pages_test.go` and `application_continuation_test.go`. | **Low. No handler, transport, database or new JS required.** Independent of focused questions/manual letters. Keep all hidden fields, modes, consent, watch attributes and form actions intact. Do not silently move setup into disclosure when it is a blocker. |
| **2** | **Focused questions with honest local status.** Surface missing local values before optional writing; enhance existing forms with index, Previous/Next and clear local-save wording; preserve focus after save and display save failures without losing text. | `templates/job.html`, `static/styles.css`, `static/ui.js`, `static/inspection.js`; small changes in `application_answers.go`, `handlers.go`, `desktop_redirect.go`; `application_answers_test.go`, `desktop_redirect_test.go`, `ui_pages_test.go`. | **Medium.** Optional validated question return context and error rendering only; no answer policy changes. Build dirty protection before hiding multiple forms. Keep no-JS forms available. |
| **3** | **Improve letter authorship.** Emphasize saved/unsaved status, return to scope and local save error recovery; subordinate generation settings. Add manual start with the verified resume-context prerequisites, subject to the review decision below. | `templates/job.html`, `static/styles.css`, `static/ui.js`, `static/inspection.js`, `cover_letters.go`, `app_http.go`, possibly `handlers.go`; `cover_letters_test.go`, existing transfer/desktop tests. | **Medium.** New local create operation justified by missing manual-start capability; existing table only, real resume references and manual provenance. Source-independent creation needs separate schema review. No transfer replacement feature. Reuse current redirect URL shapes. |
| **4** | **Polish navigation and evidence.** Return resume save directly to Prepare; align Inspection wording/back link with the workbench; verify compact outcomes and responsive layout across cases. | `application_handlers.go`, `templates/inspection.html`, `templates/job.html`, `static/styles.css`; `application_test.go`, `desktop_redirect_test.go`, `ui_pages_test.go`; small presentation changes in `handoff_state.go`/`application_continuation.go` only if raw wording remains contradictory. | **Low–medium.** Resume return is a validated existing URL shape. Browser recovery forms/behavior unchanged. Presentation helpers may improve wording but cannot broaden eligibility. |

First-slice completion means a user returning with a late saved letter sees its existing continuation context without first traversing a resume-selection form. It does not depend on AJAX saves, navigation-query additions, manual letter support, a new two-column component or native blocker repair. Long exact scope can still require scrolling; evaluate meaningful decision hierarchy rather than an unconditional “every button above the fold” target.

Backend work is limited to the explicitly named items above: question return context/save-error rendering, manual draft creation/save-error rendering, and resume save return location. Counts, disclosures, layout, local question navigation and saved/unsaved indicators need no new records or interfaces. Programmatic employer-window focus, durable transfer history, key-linked answer outcome badges, autosave and attachment replacement are deferred because they would introduce requirements beyond this redesign.

## 10. Acceptance requirements and regression checks

### Product and interaction acceptance

- Cases A–I render from existing data and flags, including overlaps, unsupported destination, closed/applied job, empty question list, saved blank answer, and long letter/answer scope. No new lifecycle enums or persisted UI completion flags.
- First fill can remain resume-only without approved contacts. If contacts are included, display their exact approved values and revision-backed authority. Changing the dropdown alone never changes saved selection or transfer authority.
- One visually primary transfer action per current decision; exact included scope stays adjacent and reviewable before sending. The first slice retains the same form action/fields and four existing transfer labels. Setup/editor saves are distinct local actions.
- Prior selected file/browser result stays distinguishable from a newly assigned local resume. Saved letter text/version stays distinguishable from an already attempted attachment. An edited completed/uncertain letter does not acquire a replacement button.
- Every independent result remains available. A partial failure must not display an all-failed or all-complete banner. Use actual manual/unsupported/uncertain details; never show ongoing transfer just because a status is nonempty.
- Focused question Next/Previous is stable and nonwrapping; index selection and keyboard focus agree; dirty text survives intra-page question navigation. Save confirms only local persistence and returns to the same valid key. Required means observed requirement, not employer validity.
- Approved profile and reusable answers retain existing precedence and opt-in semantics. Sensitive/unsupported/ambiguous controls remain manual. No AI question answering, matching or submission.
- Changed/conditional observations offer refresh; no dirty content is discarded by polling. Previous identities remain accessible but never transferred as current questions. Browser values are never invented or captured by the redesign.
- Manual letter start, when its slice ships, requires no generator/provider or readable-text readiness; it clearly states the existing schema's master/role resume context requirement. Save/generated/edit states expose the actual saved artifact; new generation creates a separate draft, edits update it in place. No external generation before existing explicit consent.
- Poll errors do not claim browser loss. Interrupted browser warnings, deliberate reopening and explicit active-restart consent stay intact. Recovery does not implicitly replay documents.
- Final review states remaining uncertainty and unknown later steps; submission remains personal in Chromium. Recording requires existing confirmation and review token, and makes no employer-receipt claim.

### Visual and accessibility acceptance

- Inspect real rendered screens in light and dark appearance at 1280px and about 390px; check native configured size and 700×550 minimum when safe native qualification becomes available. Keep those evidence claims separate. No fabricated screenshots or pass claims while native targeting is blocked.
- No horizontal page overflow for long URLs, attachment filenames, question labels, option text or unbroken content. Buttons wrap at narrow width; source/scope text remains accessible. Use semantic headings, labels, keyboard-operable disclosure and visible focus; errors use alerts, routine results polite status, not a constantly reread whole page.
- Without JavaScript, all templates/forms, disclosures and current server POST paths work. No duplicate IDs, nested forms or hidden required controls trapping submission. Question focus enhancement must not hide unsaved inputs by deleting/recreating them.
- Preserve tab keyboard behavior and `#prepare` recognition. Local subtask links use existing tab handling plus focus/scroll enhancement rather than a new unrecognized fragment that switches the app back to Overview.

### Engineering verification, when implementation is authorized

For slice 1, use existing render/handler tests to verify eligible and interrupted states, exact scope/hidden identities, independent notices and no duplicate primary actions. Adjust assertions only for intended presentation differences; do not add tests that merely restate CSS. Review screenshots of missing resume, eligible first fill, active application, late letter, combined material and uncertainty. The first slice should not need new handler tests for changed behavior because no command behavior changes.

For later slices, add meaningful checks for valid/stale return-question keys, no-JS save, preserved validation/conflict text without transfer authority, manual draft provenance and transfer compatibility, and resume-save return. Test native redirect allowlisting with foreign URLs, duplicate parameters, wrong shapes, unexpected query keys and canonical accepted paths.

Relevant existing suites include `TestPreparationActionStates`, `TestCombinedApplicationScopeAndIndependentFailures`, `TestUnsupportedPreparationOffersExternalApplication`, `TestApplicationAnswerPersistenceAndQuestionIdentity`, `TestAnswerResolutionUsesQualifiedFactsAndExplicitOverrides`, and `TestUnstableSensitiveAndAmbiguousQuestionsStayManual`. Retain cover-letter transfer, browser recovery, desktop redirect and submission-review coverage. Use `go test ./...` and `go test -tags desktop ./...` for the eventual changes; run focused race/controlled Chromium qualification only when changes or unresolved concerns warrant it and isolated execution is authorized.

Regression evidence must continue to establish: exactly displayed material only; hashes/revisions/destination revalidated; independent commands on partial failure; no stale operation modifying a replacement session; one resume transfer; unchanged successful or uncertain answers not replayed; preserved employer manual corrections/attachments; conditional questions tied to the current observation; zero automatic employer submissions. Local generation/save must cause no employer mutation. Historical controlled journeys are useful regression scenarios, not a fresh qualification pass. [Q1]

No tests, native launches, fixture transfers or provider requests are performed by this design-only assignment. No graph refresh is needed because implementation code is unchanged.

### Remaining review decisions

1. **Manual draft prerequisites:** accept the source-grounded v1 compromise (`manual` provenance with real master/role resume context), or commission separate schema/reader design for source-independent manual drafts. This spec recommends the former within the current no-migration boundary; existing-letter editing improvements do not depend on this decision.
2. **Focused-editor return URL:** review the narrow `question` query allowlist alongside existing draft/notice combinations. Recommended default is same-question-after-save, separate Next, and current-question fallback on changed observation; no autosave or client-side answer persistence.
3. **Column threshold:** confirm the proposed split against actual rendered content and native minimum once qualification is available. A single well-composed column is acceptable; the action and evidence hierarchy is the requirement.

Build slice 1 first: it improves the already-working late-letter and combined continuation journeys immediately, with the smallest technical surface and no change to application authority.
