# Application field classification and capture/Fill architecture

**Authoritative reference as of `2baa899`** (includes `6f55a3d`, release branch
`v0.2/release-0.2.0`). Field-role decisions, capture/Fill boundaries, and the
Questions workflow below reflect the current implementation. Verify against source
before changing behavior; dated reports under `docs/implementation/` are historical.

Related authoritative references: [ATS platform capabilities](../implementation/ats-platform-integration-notes.md)
(by-operation support matrix), [codebase guide](../CODEBASE_GUIDE.md) (source map),
[release policy](../RELEASE_POLICY.md) (ship/hold gates).

## Authoritative entry point

All field-role decisions go through one Go classifier:

- `internal/app/application_field_classification.go: classifyApplicationField`

Supporting identity predicates live beside it in
`internal/app/application_answer_policy.go`
(`stableQuestionIdentity`, `editableQuestion`, `editableQuestionForPlatform`,
`employerOnlyQuestion`) and projection in `writingCategory` (same file as the
classifier). Do not duplicate these rules in React or another Go subsystem.

## The five field roles

Classification depends **only on the observed control** — its identity, label,
kind, options, required/sensitive flags, and whether its identity is unique in the
observation. It never depends on values, saved answers, answer provenance, profile
approval, or prior Fill history.

| Role | Meaning | Examples | Fill implication |
| --- | --- | --- | --- |
| Candidate profile (`fieldCandidateProfile`) | A recognized applicant fact handled through Candidate Profile / contact Fill | First/last name, email, phone (`profileContactKey`); city, region, country, location, LinkedIn, GitHub, portfolio, pronouns (`additionalProfileAttribute`) | Never a writing task or answer-Fill target, even when the value is missing or unapproved. Profile controls are excluded from Questions (`unifyQuestionInventories`), from `ActionableQuestions`, and from `ReadyAnswers` / `pendingAnswers`. |
| Application question (`fieldApplicationQuestion`) | An employer question the applicant may answer in writing and, if eligible, Fill | Narrative text/textarea/select/radio answers with stable identities | Eligible for local save and, when saved and explicitly authorized, for answer Fill. |
| Document (`fieldDocument`) | A file control owned by the Documents workflow | Resume upload; anything labeled cover letter | Never an applicant writing task; excluded from the Questions projection (`displayDocumentControl`). |
| Employer-only / sensitive (`fieldEmployerOnly`) | A control the applicant must complete manually on the employer site | Work authorization, visa/sponsorship, demographics, salary expectations, attestations/consents, credentials, anything flagged sensitive | Presented as “Complete on the employer site”. Never saved as a local answer, never a Fill target. |
| Unsupported / ambiguous (`fieldUnsupported`) | A control that cannot be safely operated | Unstable, positional (`question-1`), or `unidentified-*` identities; option-less comboboxes; `unsupported_ambiguous` merged controls | Presented as “Needs manual completion”. Never saved as a supported local answer, never a Fill target. |

`writingCategory` is a transport/presentation projection of role: editability may
restrict an employer question, but it can never change a profile field's role.
Recognized candidate-profile controls stay profile data when values are missing,
unapproved, or historically saved as application answers — including controls whose
employer identity is an opaque per-form UUID (Ashby namespaces rotate; the stable
suffix is what matters).

## The contract

1. **Role is intrinsic to the control.** Value presence, saved-profile approval,
   historical answers on profile-kind controls, provenance strings, and Fill
   history do not change classification (`TestProfileRoleStableAcrossValuesApprovalAndHistory`).
2. **Historical answers are preserved, never promoted.** Answers previously saved
   against a profile-kind control remain visible under Previous writing; they gain
   no transfer authority and do not reclassify the field.
3. **Profile Fill and answer Fill are separate operations** with separate
   authorization, scope, and revision checks. Profile/contact Fill never authorizes
   answer transfer and vice versa.
4. **Observation grants no authority.** Read-only capture, preview, and discovery
   never mint transfer authorization.
5. **Explicit authorization of the displayed, revision-validated profile snapshot is
   sufficient** on the React continuation path; saved profile approval is not an
   additional prerequisite there. Fill never silently marks a stored profile
   approved (`TestControlledFieldClassificationFill` asserts approval unchanged).
6. **Every transfer revalidates identity, revision, destination, ownership, and the
   live DOM target**, and preserves existing or actively edited employer values.
   A changed field fails locally; other fields and documents are left alone.
7. **Final employer submission is always manual.** `POST /jobs/{id}/submit` records
   the applicant's local declaration only; no Fill path submits.

## The seven stages and their Go owners

| # | Stage | Owner | Decision |
| --- | --- | --- | --- |
| 1 | Browser observation | `internal/application/browser` (`observation.go: AsResult`, `inspect.js`); `chromium.Preview` for the Ashby read-only preview; `chromium.LaunchFixture` / `PreviewFixture` for tests | Read-only structural snapshot of the live form (identities, labels, kinds, options). No values stored. `OwnsPostingURL` rejects off-posting observations. |
| 2 | Field classification | `application_field_classification.go: classifyApplicationField`; `application_answer_policy.go` identity/eligibility predicates | Assigns one of the five roles from the observed control alone. |
| 3 | Profile / answer value resolution | `application_answer_policy.go: resolveApplicationAnswer`, `resolveAdditionalProfileFact`; `reusable_answers.go` (exact employer-scoped memory); Answer Library auto-preparation (review-only) | Resolves what value to show. Pure and display-only: resolution never confers save or transfer authority. Auto-prepared library text keeps revision 0 until explicitly saved. |
| 4 | Question presentation and preparation | `application_answers.go: prepareQuestions` (live), `early_application_answers.go: prepareEarlyQuestions` (public API / preview discovery), `workspace_writing_api.go: buildWritingQuestions`, `unifyQuestionInventories` | Builds the read-only inventory: live + early + unified projection, Previous writing, profile/employer/review counts. Never mints authorization or writes state. |
| 5 | Transfer eligibility | `application_answer_transfer.go: ReadyAnswers`, `application_continuation.go: pendingAnswers`, `summarizePreparation`; `workspace_continuation_api.go: buildContinuation` | Computes the exact authorized scope: editable application questions with saved values (or explicitly resolved early answers), minus profile controls, minus already-attempted or pending authorizations. Display-only until authorized. |
| 6 | Explicit Fill authorization | `answerTransferCommand` / `answerTransferForAction` (legacy form), `reactAnswerTransfer` + `continuationContinueAPI` (React), `resumeHandoffCommand` / `resumeHandoffForAction` + `coverLetterCommand` (documents) | Binds one immutable scope: destination equality, `review_hash` / scope-hash match, per-item revision and identity checks, profile-revision binding for contact/resume material. A failed item does not roll back or block other eligible items. |
| 7 | Browser-side validation and transfer | `browser_session.go` (`watch`, `armAnswers`, `armExistingContacts`, `tryAnswers`, `tryContacts`, …); `chromium/answer.go: FillAnswer`, `chromium/contact.go: FillContacts`, cover-letter and attachment operations | Re-resolves each control in the current DOM, preserves existing/active values, and reports per-field outcomes (`filled` / `correct` / `manual` / `uncertain`). Uncertain transfers are never replayed. Browser close/reopen discards transfer authority and staged files. |

## Questions workflow (Job → Documents → Questions → Apply → Submission history)

1. **Job.** Save the posting (intake). Supported sources fill in metadata; anything
   else stays manual from the start.
2. **Documents.** Choose the résumé for this job; prepare the cover letter. File
   controls never appear as writing tasks.
3. **Questions.** Two discovery contexts, one coherent list:
   - *Public/early discovery where supported*: Greenhouse public API definitions;
     Ashby read-only rendered preview (`previewEarlyQuestions`). Early drafts save
     locally with discovery identity + revision and are reconciled against the live
     form before any transfer. Lever and other providers have no early discovery.
   - *Managed capture from Questions*: the applicant opens the managed browser and
     navigates the employer form; the read-only observer records control
     definitions (not values). Capture status and failures are reported in the UI;
     capture never authorizes transfer.
   - *Automatic Answer Library preparation*: an exact equivalent library entry may
     pre-fill an eligible narrative editor for review. It is saved only by explicit
     applicant action and never enters Fill scope unsaved.
   - *Applicant edits and saves*: saves are revision-checked against the exact
     observed question and destination. Non-editable questions (profile,
     employer-only, unsupported) cannot be saved as local answers.
4. **Apply.** The continuation surface shows the exact authorized scope (resume,
   contacts snapshot, letter, saved answers with `review_hash`). Each included item
   is independently guarded; Fill executes only explicitly authorized material in
   the owned session. Browser status and interruptions are reported; anything
   unsupported stays manual. **A captured question is not assumed to be
   automatically fillable**: identifiable employer questions still require manual
   completion when their controls cannot be safely operated.
5. **Submission history.** After submitting personally on the employer site, the
   applicant records completion explicitly. Recording never submits, and transfer
   success never marks a job Applied.

### Control-level limitation: custom dropdowns and structured controls

Only uniquely identified native controls are filled (`input`, `textarea`, native
`select`, single radio groups, checkboxes with exact semantics, plus the dedicated
Ashby location-autocomplete path). Custom dropdowns (`role="combobox"`,
`custom_combobox`, `role="listbox"`), option-less selects, ambiguous multi-match
identities, and controls inside cross-origin frames or social widgets remain manual.

Observation is read-only and provider-neutral: accessible labels are resolved
through `aria-labelledby`, wrapping labels, and semantic group containers, and
choices already rendered in the visible DOM are preserved as read-only evidence
for custom structured controls — nothing is opened or clicked to discover
choices, and choices are collected onto a Fill/save candidate's record only
when they cannot flap across transient popup states. A recognizable custom
control without a stable control identity is shown under its real question
label with a relationship-scoped display identity; that identity is never a
transfer identity and the question stays manual-only with no Fill authority.
Controls with neither a reliable label nor an associated control keep the
unidentified-control notice.

Known investigation example: the live Render posting's “How did you hear
about Render?” question arrives as an option-less Ashby combobox without a
stable control identity. It is represented in the Questions projection under
its real label with manual-completion guidance and never enters automated-Fill
scope (`workspace_writing_api_test.go`, `workspace_continuation_api_test.go`,
`react_questions_unified_test.go`, `structured_question_capture_test.go`,
`structured_question_capture_qualification_test.go`).

### Unsupported postings (`2baa899`)

The Questions API exposes `ownsPosting` (`browser.OwnsPostingURL(job.URL, job.URL)`);
unsupported postings disable managed capture up front (browser routes return `422`
with explicit guidance) and the React Questions area renders dimmed/non-interactive
while preserved writing stays legible. Existing Fill restrictions are unchanged.

Known product gap: the intended experience keeps useful manual question preparation
available without managed capture, but the current unsupported-posting implementation
may lack a usable manually created manual-question workflow — early saves require an
existing discovered question identity and revision
(`earlyQuestionForGeneration`), and live preparation returns no inventory without
ownership. Dimming the Questions interface is not a complete manual-preparation
fallback. Verify this gap in source before designing around it.

## Qualification status

- `6f55a3d` native acceptance: **PARTIAL**. Established: native Questions behavior,
  explicit Fill, contact transfer, restart persistence, and preservation of manual
  submission authority. **NOT VERIFIED**: live captured-question-to-answer Fill, and
  the cause of one unexplained browser launch that was not reproduced. No later
  evidence on this branch establishes a stronger result for this commit.
- Fill and capture logic are additionally covered by Go unit tests and controlled
  Chromium fixtures (never live employer submissions). Controlled fixtures do not
  prove compatibility with every hosted form, and passing tests do not constitute
  native Wails acceptance.

## Tests that enforce this contract

- `internal/app/application_field_classification_test.go` — role stability across
  values, approval states, and historical answers; profile controls never gain
  answer authority.
- `internal/app/field_classification_qualification_test.go` (`-tags qualification`,
  `YOURMOVE_QUALIFY_BROWSER=1`) — controlled end-to-end: managed capture → Questions
  projection → save → explicit Apply Fill; unapproved snapshot fills, existing
  values preserved, misleading/unsupported controls untouched, saved approval
  unchanged, zero submissions.
- `internal/app/workspace_continuation_api_test.go`, `react_questions_unified_test.go`,
  `workspace_writing_api_test.go` — Render namespace rotation and option-less
  combobox handling; `ownsPosting` exposure for supported vs unsupported postings.
- `internal/app/application_answer_transfer_test.go`,
  `question_workbench_test.go`, `candidate_profile_autofill_test.go`,
  `application_answer_reconciliation_test.go`, `early_application_answers*_test.go` —
  scope hashing, revision/identity checks, reconciliation without bypassing
  transfer authorization.
- Platform and observation suites: `internal/platforms/*_test.go`,
  `internal/application/browser/*_test.go`, `chromium/platform_forms_test.go`,
  `chromium/trial_run_test.go` — posting identity, observation limits, and
  controlled transfer behavior.

## Extending capture or Fill

1. Trace the real observed control through normalization, classification, API
   projection, and transfer (see the seven stages above).
2. Reuse the existing classifier and ownership/authorization mechanisms; add no
   parallel rules.
3. Prefer provider-neutral semantic control handling when the failure is not
   provider-specific.
4. Verify with controlled browser fixtures; update the
   [platform capability reference](../implementation/ats-platform-integration-notes.md)
   when support changes.
