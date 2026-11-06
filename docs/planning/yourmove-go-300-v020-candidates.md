# YourMove Go — 300 candidate changes for v0.2.0 and later

**Research snapshot:** November 5, 2026. **Reviewed:** `jenksed/yourmove-go` on `main` (GitHub tree SHA `fd835f3258cfb7ca928f27cd5ae16058678219a7`) and the published `v0.1.0` release record. **Status:** Product/engineering ideation, not verified defect findings or an approved backlog.

## Key recommendation

The repository roadmap defines v0.2.0 as making the second and later applications materially easier. Keep that outcome. Select a *small vertical slice* from the candidates below; do not attempt the entire list. Keep v0.1.1 for defects evidenced by testers, and keep larger expansion work in later milestones. The classification “v0.2.0 candidate” means **worth evaluating**, not scheduled.

**Current v0.1.0 foundations:** React/TypeScript/Vite in Wails 3; Opportunities; résumé versioning/routing; application workspace; question preparation and saved answers; optional cover-letter AI; Career Evidence; applicant-authorized employer continuation; controlled Chromium and Firefox fixtures; manual submission and local history. Career Discovery/Studio remain parked; live ATS qualification, notarization, Safari/WebKit and public auto-updates remain outside 0.1.0 claims.

**Evidence limits:** This review inspected repository tree, source and docs over the connected GitHub API, recent PR descriptions and public release metadata. No local clone, source compilation, native GUI run, real employer submission or exhaustive code-path audit was performed. Candidate language is intentional: presence in this inventory does not imply the current implementation is defective or missing. The published `v0.1.0` prerelease has a newer installer-r2 DMG asset than the release log's original DMG name, so its exact packaging history should be reconciled when updating release docs; do not assume the binary changed.

## Proposed decision gates

1. Does it shorten a real second application, reduce corrections, or unblock continuation?
2. Is the existing UI/service enough to support it, or is a new system required?
3. Does it preserve separate applicant review, transfer consent and manual employer submission?
4. What one real native user journey would demonstrate the improvement?
5. Is there observed failure evidence, or is this a strategic hypothesis?

## Early v0.2.0 conversation starters

- **YM-081, YM-085, YM-086, YM-091:** remove repeat-work overhead and clarify readiness in the workspace.
- **YM-101, YM-102, YM-103, YM-104:** turn existing exact-match answer reuse into useful, reviewable suggestions.
- **YM-061, YM-063, YM-068:** improve résumé discovery and recommendations without silently selecting files.
- **YM-141, YM-142, YM-144, YM-150:** make employer transfer plans/results clearer and handle uncertainty conservatively.
- **YM-121, YM-122:** use prior cover letters as reviewed starting points.
- **YM-261, YM-266:** whole-workspace backup and actual upgrade preservation.

## Candidate inventory

Legend: **0.2 candidate** = plausibly supports the v0.2 repeat-application outcome; **Later** = future/research option. Candidates are not requirements.
### 1. First-run experience and navigation (10 v0.2 candidates / 10 later)

*Product aim:* Reduce the distance from installation to the first useful application.

*Repository basis:* `frontend/src/shared/AppNavigation.tsx; docs/TRY_IT.md; docs/releases/0.1.0-macos.md`

- **YM-001** [0.2] Offer a dismissible first-job setup checklist without forcing account-style onboarding.
- **YM-002** [0.2] Make the first-run path explicitly separate job organization from optional browser assistance.
- **YM-003** [0.2] Provide a one-screen explanation of what YourMove will never submit automatically.
- **YM-004** [0.2] Show which documents and profile details are ready before a first employer handoff.
- **YM-005** [0.2] Give each empty workspace one obvious next action rather than a tour of every feature.
- **YM-006** [0.2] Explain the difference between saving an opportunity and opening an employer application.
- **YM-007** [0.2] Surface a compact, persistent next-action link for the current unfinished application.
- **YM-008** [0.2] Let users resume their most recent unfinished workspace from the home view.
- **YM-009** [0.2] Add a safe sample opportunity that cannot accidentally reach a live employer.
- **YM-010** [0.2] Explain optional AI setup only at the first moment a user requests AI assistance.
- **YM-011** [Later] Add contextual first-use help that disappears after the relevant action succeeds.
- **YM-012** [Later] Introduce an optional accessibility and text-density preference on first launch.
- **YM-013** [Later] Support importing existing job-tracking CSV records during first-run setup.
- **YM-014** [Later] Add a glossary for unfamiliar labels such as inspection, continuation and evidence.
- **YM-015** [Later] Provide a short interactive walkthrough using synthetic applicant data.
- **YM-016** [Later] Offer a compact guided path for users who only want job tracking, not form filling.
- **YM-017** [Later] Provide a keyboard-navigation cheat sheet accessible from every screen.
- **YM-018** [Later] Let users choose whether the app reopens at Opportunities or the last workspace.
- **YM-019** [Later] Add a searchable, offline help center built around concrete applicant tasks.
- **YM-020** [Later] Offer a simple guided route to a complete second application for returning users.

### 2. Opportunity intake and job intelligence (10 v0.2 candidates / 10 later)

*Product aim:* Make job intake accurate, quick and informative without fabricating posting data.

*Repository basis:* `internal/app/single_intake.go; internal/app/intake.go; internal/platforms/; frontend/src/features/opportunities/`

- **YM-021** [0.2] Show a preview of extracted title, employer and location before committing an imported URL.
- **YM-022** [0.2] Group bulk-import successes, duplicates and recoverable failures in one useful result view.
- **YM-023** [0.2] Show the original posting URL next to the resolved canonical URL when they differ.
- **YM-024** [0.2] Let users correct mistaken employer identity without losing the original source evidence.
- **YM-025** [0.2] Detect already-saved postings by supported posting identity, not just URL spelling.
- **YM-026** [0.2] Mark which imported fields came from employer evidence versus user edits.
- **YM-027** [0.2] Improve manual recovery for sites that refuse automated posting retrieval.
- **YM-028** [0.2] Keep the user's edited job title when a later refresh obtains new employer metadata.
- **YM-029** [0.2] Offer quick manual location, remote-status and compensation corrections after intake.
- **YM-030** [0.2] Distinguish confirmed closed postings from temporarily unreachable pages.
- **YM-031** [Later] Add optional drag-and-drop import of a browser-saved job posting file.
- **YM-032** [Later] Extract a bounded list of explicit requirements from a posting into a reviewable view.
- **YM-033** [Later] Detect important posting changes between two explicit user-requested fetches.
- **YM-034** [Later] Surface job-posting language and work-authorization caveats without inferring eligibility.
- **YM-035** [Later] Add configurable saved search queries with deliberate user-triggered refresh.
- **YM-036** [Later] Support importing public job-feed results as candidates rather than auto-saving them.
- **YM-037** [Later] Expand posting metadata coverage only for providers with reproducible fixtures.
- **YM-038** [Later] Offer side-by-side original posting text and normalized summary for corrections.
- **YM-039** [Later] Allow users to save a role with no URL, including referral-only opportunities.
- **YM-040** [Later] Preserve a human-readable snapshot of the posting as originally reviewed.

### 3. Pipeline, prioritization and follow-up (9 v0.2 candidates / 11 later)

*Product aim:* Help applicants decide what to do next, not just accumulate saved links.

*Repository basis:* `internal/app/jobs.go; internal/app/triage.go; frontend/src/features/opportunities/OpportunitiesPage.tsx`

- **YM-041** [0.2] Add a focused Today queue combining unfinished preparation and user-set follow-ups.
- **YM-042** [0.2] Let users set a next-action date without turning the app into a calendar system.
- **YM-043** [0.2] Show which saved opportunities are blocked on a missing resume or unanswered question.
- **YM-044** [0.2] Make stage transitions explicit and reversible with a concise change history.
- **YM-045** [0.2] Pin a small number of high-priority opportunities at the top of the list.
- **YM-046** [0.2] Offer saved filters for actionable, awaiting reply, rejected and archived opportunities.
- **YM-047** [0.2] Distinguish applied status from merely reaching the employer submission screen.
- **YM-048** [0.2] Include a private, short note attached to each opportunity.
- **YM-049** [0.2] Allow bulk changes to priority or archive status with an exact confirmation preview.
- **YM-050** [Later] Add user-entered employer contact and recruiter notes without automatic outreach.
- **YM-051** [Later] Track interview stages and interview dates in the same opportunity timeline.
- **YM-052** [Later] Track referral introductions and application-source channels separately.
- **YM-053** [Later] Provide a simple follow-up checklist with user-controlled due dates.
- **YM-054** [Later] Show weekly application activity without suggesting activity equals effectiveness.
- **YM-055** [Later] Let users compare two saved opportunities against their own stated priorities.
- **YM-056** [Later] Support user-defined tags for role type, industry and urgency.
- **YM-057** [Later] Add a lightweight decision log for declining or pausing an opportunity.
- **YM-058** [Later] Generate a private weekly summary from locally recorded progress only.
- **YM-059** [Later] Provide opt-in local reminders for manually chosen next-action dates.
- **YM-060** [Later] Offer a compact archived-opportunities view with deliberate restore behavior.

### 4. Résumé library, versions and routing (10 v0.2 candidates / 10 later)

*Product aim:* Make selecting the correct exact document nearly effortless.

*Repository basis:* `frontend/src/features/resumes/ResumesPage.tsx; internal/app/resumes.go; internal/app/resume_choices.go`

- **YM-061** [0.2] Replace numeric job-ID assignment in Documents with a searchable opportunity picker.
- **YM-062** [0.2] Preview a résumé's actual PDF/DOCX content before assigning it to an opportunity.
- **YM-063** [0.2] Explain why a suggested résumé matches a role family, including no-match cases.
- **YM-064** [0.2] Display the selected résumé version and hash clearly at application handoff.
- **YM-065** [0.2] Let users replace an archived or missing document reference from the job workspace.
- **YM-066** [0.2] Offer a one-click route from an opportunity to its selected document in the library.
- **YM-067** [0.2] Show which active opportunities still use an older résumé revision.
- **YM-068** [0.2] Remember the last manually chosen résumé for a role family as a nonbinding suggestion.
- **YM-069** [0.2] Warn when a selected document changed on disk before transfer.
- **YM-070** [0.2] Make the distinction between master résumé and employer-upload résumé more visible.
- **YM-071** [Later] Offer side-by-side content differences between two résumé revisions.
- **YM-072** [Later] Show an editable display name distinct from the original filename.
- **YM-073** [Later] Add an applicant-reviewed experience/skills inventory linked to résumé source sections.
- **YM-074** [Later] Offer optional role-specific résumé variant organization with no automatic rewriting.
- **YM-075** [Later] Support user-controlled export of an existing résumé's exact file bytes.
- **YM-076** [Later] Expose file metadata such as upload date, format and last assigned job.
- **YM-077** [Later] Provide a path to repair an unavailable externally referenced document.
- **YM-078** [Later] Make duplicate-document detection understandable when filenames differ.
- **YM-079** [Later] Support a native quick-look action for compatible résumé formats.
- **YM-080** [Later] Offer bulk archiving only for unused documents after dependency review.

### 5. Application workspace and repeat-use flow (14 v0.2 candidates / 6 later)

*Product aim:* Shorten the second application while retaining deliberate applicant decisions.

*Repository basis:* `frontend/src/features/application-workspace/WorkspacePage.tsx; internal/app/workspace_api.go`

- **YM-081** [0.2] Offer a next-application starting point that reuses only explicitly approved local materials.
- **YM-082** [0.2] Display a readiness summary for documents, questions and handoff before opening a browser.
- **YM-083** [0.2] Carry the user back to the last active workspace activity after navigating away.
- **YM-084** [0.2] Make the most recently chosen letter and résumé visible together in one overview.
- **YM-085** [0.2] Highlight only the unresolved work rather than repeating already completed instructions.
- **YM-086** [0.2] Explain exactly which steps may be skipped on a job with no application questions.
- **YM-087** [0.2] Show a clear difference between locally prepared, transferred and employer-submitted.
- **YM-088** [0.2] Add a nonblocking warning if the user switches jobs while an unsaved editor is open.
- **YM-089** [0.2] Offer a compact summary of changes made since the last employer inspection.
- **YM-090** [0.2] Provide an easy way to return from manual employer work to the correct saved job.
- **YM-091** [0.2] Make the primary button depend on the actual next safe action, not a generic Continue.
- **YM-092** [0.2] Display the last successful save time for each applicant-editable material.
- **YM-093** [0.2] Keep application context readable on narrow desktop windows without horizontal scrolling.
- **YM-094** [0.2] Add a final applicant-controlled review screen for selected material before transfer.
- **YM-095** [Later] Offer a reusable per-application checklist template chosen by the user.
- **YM-096** [Later] Provide a side-by-side preparation-versus-employer status view during handoff.
- **YM-097** [Later] Track time spent in broad workflow phases locally and only with opt-in.
- **YM-098** [Later] Allow users to copy a prior job's private preparation note into a new job.
- **YM-099** [Later] Support multiple manual application attempts for the same posting without confusing history.
- **YM-100** [Later] Provide a compact keyboard-driven application preparation mode for repeat users.

### 6. Question workbench and Answer Library (12 v0.2 candidates / 8 later)

*Product aim:* Reuse good answers without misapplying them to different employer questions.

*Repository basis:* `frontend/src/features/application-workspace/WritingSection.tsx; internal/app/answer_library.go; internal/app/reusable_answers.go`

- **YM-101** [0.2] Offer reviewed Answer Library suggestions next to a newly observed narrative question.
- **YM-102** [0.2] Show exactly why an answer is eligible or ineligible for automatic exact-match reuse.
- **YM-103** [0.2] Let users manually browse prior answers when the employer question wording differs.
- **YM-104** [0.2] Separate exact-match fill from a suggested adaptation requiring applicant review.
- **YM-105** [0.2] Offer a one-click route to save a finished narrative answer into the Answer Library.
- **YM-106** [0.2] Display the original source question and employer beside reusable answer text.
- **YM-107** [0.2] Show whether a suggested answer has been modified since it was saved.
- **YM-108** [0.2] Preserve the active draft when an employer question inventory refreshes.
- **YM-109** [0.2] Make unanswered, answered and manually required question counts easy to understand.
- **YM-110** [0.2] Support safe manual copying of an approved answer without asserting successful transfer.
- **YM-111** [0.2] Clearly identify sensitive questions that must stay applicant-controlled.
- **YM-112** [0.2] Show the expected field type and limits before users invest in a long answer.
- **YM-113** [Later] Detect possible near-duplicate questions for manual review without auto-applying answers.
- **YM-114** [Later] Offer a locally searchable history of how a particular answer evolved.
- **YM-115** [Later] Provide answer variants for different length limits with explicit selection.
- **YM-116** [Later] Group related question patterns such as motivation, experience and availability.
- **YM-117** [Later] Surface contradictions between two saved reusable answers for user correction.
- **YM-118** [Later] Support optional user-defined answer categories and favorites.
- **YM-119** [Later] Help trim a long answer to an employer's observed character limit while preserving meaning.
- **YM-120** [Later] Offer an exportable personal answer collection in a simple portable format.

### 7. Cover letters and reusable writing (10 v0.2 candidates / 10 later)

*Product aim:* Make accurate, personal writing faster while preserving human authorship.

*Repository basis:* `frontend/src/features/application-workspace/CoverLetterSection.tsx; internal/app/cover_letters.go; internal/coverletter/`

- **YM-121** [0.2] Provide an explicit Use prior letter as starting draft action for the next application.
- **YM-122** [0.2] Show the role and résumé version behind an existing letter before reuse.
- **YM-123** [0.2] Make stale-evidence warnings specific to the changed input, not just a generic badge.
- **YM-124** [0.2] Provide a side-by-side view of the current draft and its previous revision.
- **YM-125** [0.2] Create short and full-length letter variants from the same applicant-approved material.
- **YM-126** [0.2] Offer an applicant-controlled facts checklist before any generated letter is saved.
- **YM-127** [0.2] Show exactly which Career Evidence items a generated letter relied on.
- **YM-128** [0.2] Let users change the selected transfer format without regenerating the letter.
- **YM-129** [0.2] Distinguish a saved letter from a letter actually transferred to an employer form.
- **YM-130** [0.2] Improve copy/paste of plain-text letters when the employer page rejects attachments.
- **YM-131** [Later] Provide reusable opening and closing paragraphs editable as normal local writing.
- **YM-132** [Later] Offer an optional narrative consistency review against the selected résumé.
- **YM-133** [Later] Let users maintain multiple tone presets without implicitly changing personal claims.
- **YM-134** [Later] Add a simple print/export preview for supported letter formats.
- **YM-135** [Later] Show a plain language explanation of provider failures and preserve the draft unchanged.
- **YM-136** [Later] Offer a direct comparison of an AI suggestion with the user's original text.
- **YM-137** [Later] Store user-approved employer-specific phrasing patterns separately from facts.
- **YM-138** [Later] Support a user-maintained preferred length for future letter drafts.
- **YM-139** [Later] Offer a private collection of polished writing excerpts usable across applications.
- **YM-140** [Later] Provide a manual no-AI letter starter that asks only for employer, role and motivation.

### 8. Employer form observation, filling and ATS coverage (10 v0.2 candidates / 10 later)

*Product aim:* Increase successful authorized transfers and reduce silent mismatches.

*Repository basis:* `internal/application/browser/; internal/application/providers/; internal/platforms/`

- **YM-141** [0.2] Show per-field planned actions before a Fill operation when the form is inspectable.
- **YM-142** [0.2] Report filled, unchanged, skipped and uncertain fields in a human-readable transfer summary.
- **YM-143** [0.2] Provide clearer in-place retry for a failed employer question inspection.
- **YM-144** [0.2] Make manual fallback instructions specific to the observed unsupported control.
- **YM-145** [0.2] Prevent repeated uploads when the same exact approved document already appears attached.
- **YM-146** [0.2] Give the applicant a clear stop control before any new authorized transfer begins.
- **YM-147** [0.2] Record source and observed destination field labels for debugging without secrets.
- **YM-148** [0.2] Distinguish an employer form validation warning from a confirmed transfer failure.
- **YM-149** [0.2] Improve handling of multi-step forms while respecting changes in observed scope.
- **YM-150** [0.2] Require a new review when an employer page changes material field identity.
- **YM-151** [Later] Expand live qualification for Ashby forms with a published supported-behavior matrix.
- **YM-152** [Later] Expand live qualification for Greenhouse forms with a published supported-behavior matrix.
- **YM-153** [Later] Expand live qualification for Lever forms with a published supported-behavior matrix.
- **YM-154** [Later] Prototype additional ATS fill adapters only after meaningful user demand and qualification.
- **YM-155** [Later] Add user-reviewed rules for optional fields that should always remain unfilled.
- **YM-156** [Later] Handle custom dropdowns only when their selected value can be observed reliably.
- **YM-157** [Later] Improve file-attachment state detection on sites that retain opaque upload widgets.
- **YM-158** [Later] Provide a safe answer-transfer preview for checkbox and multi-select questions.
- **YM-159** [Later] Offer a manual field mapping correction that remains limited to the active form.
- **YM-160** [Later] Record a privacy-safe compatibility outcome when a supported employer form changes.

### 9. Browser engines, continuity and recovery (10 v0.2 candidates / 10 later)

*Product aim:* Make a single employer session reliable without fabricating recovery or consent.

*Repository basis:* `internal/app/browser_session.go; internal/app/application_browser.go; internal/application/browser/playwright/`

- **YM-161** [0.2] Explain at session start which browser engine is actually running and why.
- **YM-162** [0.2] Show missing Firefox runtime prerequisites with a precise local setup path.
- **YM-163** [0.2] Offer a safe retry when a browser closes before any transfer occurred.
- **YM-164** [0.2] Clearly label which employer-side edits cannot be recovered after a crash.
- **YM-165** [0.2] Keep one discoverable route back to the active managed browser window.
- **YM-166** [0.2] Prevent an engine change from silently replacing the current employer session.
- **YM-167** [0.2] Show whether a recovered page requires manual login, navigation or inspection again.
- **YM-168** [0.2] Preserve local drafts when browser recovery has to start from scratch.
- **YM-169** [0.2] Add a transparent compatibility warning when a platform requires an unsupported browser.
- **YM-170** [0.2] Offer a session diagnostics summary that omits cookies, tokens and applicant content.
- **YM-171** [Later] Evaluate bundling a supported Firefox runtime instead of relying on separate setup.
- **YM-172** [Later] Qualify WebKit through the complete production dispatch path before offering a selector.
- **YM-173** [Later] Add browser process cleanup checks when users quit the desktop app during a session.
- **YM-174** [Later] Distinguish network timeout, navigation redirect and form structure change in recovery copy.
- **YM-175** [Later] Provide a visible manual-navigation mode when automated deep links are blocked.
- **YM-176** [Later] Use controlled retries only for read-only inspection, never uncertain employer writes.
- **YM-177** [Later] Add a supportable path for session startup on machines with multiple Chrome installs.
- **YM-178** [Later] Test browser handoff across sleep/wake and network reconnect scenarios.
- **YM-179** [Later] Add a compact per-session sequence of locally observed actions for support.
- **YM-180** [Later] Make repeated application continuation efficient without preserving third-party credentials.

### 10. Candidate profile, career evidence and personal context (10 v0.2 candidates / 10 later)

*Product aim:* Keep reusable applicant facts accurate, inspectable and intentionally shared.

*Repository basis:* `internal/app/candidate_profile.go; internal/app/career_evidence.go; templates/candidate.html`

- **YM-181** [0.2] Make profile approval and incomplete-draft saving visibly distinct actions.
- **YM-182** [0.2] Render recoverable field-level errors without replacing the profile editor.
- **YM-183** [0.2] Show the approval status and revision used in each contact-data transfer.
- **YM-184** [0.2] Allow per-application overrides of contact facts without silently editing the master profile.
- **YM-185** [0.2] Make pronoun and preferred-name mappings clear and individually controlled.
- **YM-186** [0.2] Provide a quick review of employment dates before using Career Evidence in writing.
- **YM-187** [0.2] Show source, edit date and approval state on saved Career Evidence items.
- **YM-188** [0.2] Make it easy to correct one evidence claim without modifying unrelated experiences.
- **YM-189** [0.2] Explain when profile facts are intentionally withheld from an employer form.
- **YM-190** [0.2] Offer direct links from a generated claim back to its editable evidence record.
- **YM-191** [Later] Support a user-maintained skills inventory connected to actual evidence examples.
- **YM-192** [Later] Group evidence by experience, responsibility, result and transferable skill.
- **YM-193** [Later] Offer an optional evidence freshness reminder driven by explicit user dates.
- **YM-194** [Later] Keep a local history of materially changed experience claims.
- **YM-195** [Later] Support evidence attachments whose provenance remains local and reviewable.
- **YM-196** [Later] Provide user-controlled redaction of personal details from exported evidence.
- **YM-197** [Later] Allow short accomplishment notes to be captured directly after an application.
- **YM-198** [Later] Offer a guided evidence entry for people with nontraditional work histories.
- **YM-199** [Later] Display potential overlap or duplication between evidence items for manual reconciliation.
- **YM-200** [Later] Support reuse of approved examples across answer writing and cover letters.

### 11. Career Discovery, Studio and Realize adjacency (3 v0.2 candidates / 17 later)

*Product aim:* Explore discovery as a separate opt-in value path rather than treating hidden code as shipped.

*Repository basis:* `internal/app/career_discovery.go; frontend/src/features/career/; docs/releases/0.1.0-log.md`

- **YM-201** [0.2] Run an explicit acceptance review of parked Discovery before deciding whether to surface it.
- **YM-202** [0.2] Clarify how discovered strengths become applicant-approved Career Evidence, if enabled.
- **YM-203** [0.2] Make any promotion from discovery to saved evidence a deliberate, reversible action.
- **YM-204** [Later] Offer optional discovery prompts for applicants who struggle to describe past work.
- **YM-205** [Later] Create a concise 'what I actually did' interview based on user-selected experience.
- **YM-206** [Later] Show exact supporting statements for a suggested transferable skill.
- **YM-207** [Later] Allow users to reject a suggested strength without losing their interview notes.
- **YM-208** [Later] Provide a natural stopping point when sufficient discovery evidence exists.
- **YM-209** [Later] Offer a review screen distinguishing observation, interpretation and unresolved questions.
- **YM-210** [Later] Support export of approved discovery takeaways without exporting raw interview history.
- **YM-211** [Later] Prototype importing a Realize takeaway JSON only through a reviewed local mapping.
- **YM-212** [Later] Allow a user to connect an approved Realize insight to a role-family résumé strategy.
- **YM-213** [Later] Keep the Realize demo/service independent of core desktop job application availability.
- **YM-214** [Later] Measure whether discovery materially improves application writing rather than engagement alone.
- **YM-215** [Later] Add a narrative-to-evidence conversion flow that preserves the original source quote.
- **YM-216** [Later] Offer a user-owned correction workflow for claims inferred from interview answers.
- **YM-217** [Later] Provide role-specific examples of how one evidence item may be described without invention.
- **YM-218** [Later] Support a confidence-building reflection after an application that avoids performance scoring.
- **YM-219** [Later] Make all external inference controls explicit and optional for discovery sessions.
- **YM-220** [Later] Evaluate Studio as a separate user outcome before exposing its existing navigation.

### 12. AI assistance, model choice and provenance (10 v0.2 candidates / 10 later)

*Product aim:* Improve usefulness of optional AI without letting it invent applicant authority.

*Repository basis:* `internal/app/application_answer_ai.go; internal/coverletter/provider/; prompts/`

- **YM-221** [0.2] Provide a pre-request summary of exactly which resume, job and evidence inputs will be sent.
- **YM-222** [0.2] Keep provider selection and external-data consent legible next to the generation action.
- **YM-223** [0.2] Show a review-before-save comparison between original writing and an AI suggestion.
- **YM-224** [0.2] Explain why an AI response is rejected or held for correction without discarding user text.
- **YM-225** [0.2] Show supported claim sources beside an AI-generated answer or letter.
- **YM-226** [0.2] Give users an explicit way to remove an evidence item from a pending generation request.
- **YM-227** [0.2] Make AI-unavailable mode fully usable for the application-writing journey.
- **YM-228** [0.2] Record which model/provider produced a saved suggestion without storing credentials.
- **YM-229** [0.2] Separate language polishing from generating new factual claims in the interface.
- **YM-230** [0.2] Flag assertions unsupported by selected applicant evidence for manual review.
- **YM-231** [Later] Evaluate local model inference for selected low-stakes rewriting tasks.
- **YM-232** [Later] Support multiple provider profiles with clear credential and data-boundary differences.
- **YM-233** [Later] Offer a manually initiated quality comparison between two provider drafts.
- **YM-234** [Later] Show approximate request size and expected charges where provider data allows.
- **YM-235** [Later] Allow users to choose a preferred model per writing task without hidden switching.
- **YM-236** [Later] Support explicit retry after provider failure without duplicating a saved draft.
- **YM-237** [Later] Add a low-cost, text-only generation path for weak or offline hardware where feasible.
- **YM-238** [Later] Maintain privacy-safe provider reliability metrics in a local diagnostic view.
- **YM-239** [Later] Offer an applicant-controlled policy for never sending selected evidence categories externally.
- **YM-240** [Later] Test an evidence-backed answer adaptation workflow against real correction effort.

### 13. Native macOS feel and accessibility (10 v0.2 candidates / 10 later)

*Product aim:* Make YourMove feel like a trustworthy Mac application, not a wrapped web page.

*Repository basis:* `desktop.go; release/macos/; frontend/src/shared/quiet-ink.css`

- **YM-241** [0.2] Pursue Developer ID signing and notarization for normal macOS distribution.
- **YM-242** [0.2] Exercise first launch from the actual downloaded DMG on a clean test user account.
- **YM-243** [0.2] Check native file-picker cancellation with unsaved workspace state intact.
- **YM-244** [0.2] Improve focus return after native dialogs and application modal confirmations.
- **YM-245** [0.2] Verify keyboard-only navigation for every primary workspace activity.
- **YM-246** [0.2] Provide meaningful visible focus styles in both Quiet Ink themes.
- **YM-247** [0.2] Ensure all saved-state messages are announced appropriately to screen readers.
- **YM-248** [0.2] Make compact and narrow window layouts consistently usable.
- **YM-249** [0.2] Reduce hard navigation reloads between React and unmigrated template pages.
- **YM-250** [0.2] Use platform-appropriate names and icons for file selection, export and open actions.
- **YM-251** [Later] Add standard Mac menu items for common actions such as Open, Settings and Quit.
- **YM-252** [Later] Persist last useful window size and position with safe off-screen recovery.
- **YM-253** [Later] Add a native 'Open recent opportunity' menu driven by local data.
- **YM-254** [Later] Support drag-and-drop of supported documents into the correct application area.
- **YM-255** [Later] Expose light, dark and system appearance choices through an understandable setting.
- **YM-256** [Later] Honor reduced-motion preferences across transitions and progress states.
- **YM-257** [Later] Implement accessible keyboard shortcuts for search, navigation and saving drafts.
- **YM-258** [Later] Integrate macOS Quick Look where it adds value to document review.
- **YM-259** [Later] Evaluate a signed automatic-updater only after the distribution chain is trustworthy.
- **YM-260** [Later] Qualify Intel and older macOS only if real demand justifies separate support cost.

### 14. Local data, privacy, backup and portability (10 v0.2 candidates / 10 later)

*Product aim:* Protect the actual user workspace, including files outside the SQLite database.

*Repository basis:* `internal/storage/; internal/app/desktop_export.go; docs/releases/0.1.0-macos.md`

- **YM-261** [0.2] Add an explicit local backup command covering SQLite plus referenced document files.
- **YM-262** [0.2] Verify a backup can restore a complete synthetic application history into a clean workspace.
- **YM-263** [0.2] Warn before installing a new version when the user's workspace has no recoverable backup.
- **YM-264** [0.2] Provide a straightforward export of opportunity records with stable IDs and associations.
- **YM-265** [0.2] Offer a human-readable inventory of where the application stores its local information.
- **YM-266** [0.2] Test upgrade from v0.1.0 against the same real workspace shape before shipping v0.2.0.
- **YM-267** [0.2] Make missing-document recovery preserve original references until explicitly repaired.
- **YM-268** [0.2] Prevent partial archive/restore outcomes when document copying fails.
- **YM-269** [0.2] Explain what deleting a job removes and what historical evidence is preserved.
- **YM-270** [0.2] Ensure diagnostics can be shared without accidentally including private applicant text.
- **YM-271** [Later] Add a dry-run restore preview listing objects that would be replaced.
- **YM-272** [Later] Support portable encrypted backup as an opt-in advanced option.
- **YM-273** [Later] Provide granular export choices for jobs, answers, letters and Career Evidence.
- **YM-274** [Later] Offer an audit of orphaned document files before any optional cleanup.
- **YM-275** [Later] Provide user-controlled retention settings for obsolete browser inspection results.
- **YM-276** [Later] Create a documented migration path from source-run workspaces to installed-app workspaces.
- **YM-277** [Later] Add a full local-data deletion workflow with explicit effect preview.
- **YM-278** [Later] Support an opt-in privacy-safe error report that the user can inspect first.
- **YM-279** [Later] Provide an integrity-check command that reports issues without silently repairing them.
- **YM-280** [Later] Evaluate portable workspace movement between Mac devices without adding a cloud account.

### 15. Reliability, performance, releases and engineering ownership (10 v0.2 candidates / 10 later)

*Product aim:* Keep improvements deliverable and releases evidence-backed without building a framework.

*Repository basis:* `docs/architecture/FEATURE_OWNERSHIP.md; .github/workflows/ci.yml; release/macos/; internal/app/`

- **YM-281** [0.2] Fix stale public quickstart statements that contradict the published v0.1.0 release.
- **YM-282** [0.2] Record real first-run, second-application and transfer-recovery timings on synthetic journeys.
- **YM-283** [0.2] Build a small native v0.1.0-to-v0.2.0 upgrade acceptance fixture.
- **YM-284** [0.2] Keep release candidates tied to exact source, app bundle and checksums.
- **YM-285** [0.2] Improve the existing CI action that warns about its Node 20 runtime deprecation.
- **YM-286** [0.2] Add a concise changelog of user-observable behavior and meaningful limitations.
- **YM-287** [0.2] Define one end-to-end second-application acceptance path with the real native host.
- **YM-288** [0.2] Test malformed or absent employer questions without crashing any workspace view.
- **YM-289** [0.2] Measure application workspace interaction cost before choosing performance refactors.
- **YM-290** [0.2] Preserve a source/acceptance distinction when documenting browser support.
- **YM-291** [Later] Profile the slowest Go/SQLite operations under a realistically populated local workspace.
- **YM-292** [Later] Reduce unnecessary application-question polling when the employer browser is idle.
- **YM-293** [Later] Extract a Go feature package only where observed coupling warrants the move.
- **YM-294** [Later] Consolidate duplicated React/legacy UI rules as each actual route is migrated.
- **YM-295** [Later] Add a short dependency map for session ownership, database writes and document effects.
- **YM-296** [Later] Expand controlled employer-fixture coverage based on actual field failure patterns.
- **YM-297** [Later] Exercise sleep/wake, force-quit and app relaunch in a targeted native reliability suite.
- **YM-298** [Later] Pin and verify packaging inputs for reproducible Mac release candidates.
- **YM-299** [Later] Create a privacy-safe tester feedback triage loop tied to specific user journeys.
- **YM-300** [Later] Evaluate macOS crash diagnostics that are locally inspectable and opt-in to share.
## How to turn 300 options into a real release

**Stage A — evidence gathering:** observe 3–5 successive synthetic or consenting real-user application journeys, recording repeated data entry, misfills, interruptions and corrections. Avoid sending or collecting real personal data in public reports.

**Stage B — choose a single measurable v0.2 outcome:** for example, the applicant can start a second application, inspect suggested prior writing and selected résumé, explicitly approve only correct material, and complete employer handoff with fewer manual steps than v0.1.0.

**Stage C — commit small slices:** prioritize the first complete vertical path (UI → Go authority → persistent data → observed browser outcome) and qualify the actual packaged Mac workflow. Defer generalized plugin frameworks, major transport changes and platform ports.

**Release split:** v0.1.1 should be limited to observed failures and upgrade safety; v0.2.0 should deliver repeat-use improvements; v0.3+ can absorb larger discovery, ATS, portability and platform experiments after evidence.

## Source anchors

- [Repository](https://github.com/jenksed/yourmove-go)
- [v0.1.0 published technical-alpha release](https://github.com/jenksed/yourmove-go/releases/tag/v0.1.0)
- [v0.1.0 release log](https://github.com/jenksed/yourmove-go/blob/main/docs/releases/0.1.0-log.md)
- [Roadmap](https://github.com/jenksed/yourmove-go/blob/main/docs/ROADMAP.md)
- [Feature ownership](https://github.com/jenksed/yourmove-go/blob/main/docs/architecture/FEATURE_OWNERSHIP.md)
- [Existing backlog](https://github.com/jenksed/yourmove-go/blob/main/docs/BACKLOG.md) (older snapshot; reconcile before promoting candidates to issues)
