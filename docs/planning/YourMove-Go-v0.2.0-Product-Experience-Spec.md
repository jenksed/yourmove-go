# YourMove Go v0.2.0 — Product Experience Specification

**Status:** Draft for product review; not an approved release commitment  
**Date:** November 5, 2026  
**Audience:** Product, design, engineering, release owner  
**Primary basis:** YourMove Go 300-candidate inventory (November 5, 2026), proposed v0.2.0 scope, repository architecture reference (October 21, 2026), and the documented v0.1.0 technical-alpha baseline.

> **Working release thesis:** YourMove should become easier to use with every application, and more people should be able to run it on their own computers without surrendering control of their applications or data.

## 1. Purpose and product promise

v0.2.0 should deliver two outcomes that users can directly experience:

1. **The second application is materially easier.** A returning applicant can reuse previously approved materials, understand what still needs work, and reach the employer's reviewable application with less repetition and fewer unnecessary decisions.
2. **YourMove runs natively on more computers.** Extend beyond the existing Apple Silicon macOS technical alpha toward Windows 11 x64 and Ubuntu 24.04 x64, with clear distinctions between a build that exists, a working preview, and a fully qualified release for that platform.

These are co-equal product directions, but **qualification is platform-specific**. The release may include a qualified subset of platforms if another target is not ready; it must not advertise unqualified support. December 4, 2026 remains a *readiness checkpoint* from the existing roadmap, not a promised ship date.

The target user is someone applying to multiple jobs over time, often encountering repeated employer questions, choosing between résumé versions, and returning to incomplete applications. The product should reduce this work without hiding what it intends to do.

## 2. What the existing product already establishes

The v0.1.0 technical-alpha baseline includes opportunities, résumé versioning and routing, an application workspace, saved answers, optional cover-letter assistance, Career Evidence, an applicant-authorized employer handoff, manual submission, and local application history. Its accepted audience is Apple Silicon macOS; browser-assistance qualification is bounded to controlled fixtures and known workflows.

**v0.2.0 is an extension, not a rewrite.** Preserve the existing React/Go/Wails desktop experience, the local-first data model, and the separate employer browser. The source inventory is a set of proposals, not evidence that every listed improvement is missing or defective.

## 3. Experience principles

1. **Previously approved work is reusable, never silently repurposed.** Exact-match reuse and suggested adaptations are visibly different. A suggestion is not an instruction to fill an employer field.
2. **The applicant remains the decision-maker.** YourMove may prepare or, after explicit authorization, transfer information into an employer form. It never submits the application.
3. **Tell the truth about state.** Locally prepared, offered for transfer, observed as filled, uncertain, and employer-submitted are distinct conditions. An uncertain browser result cannot be called success.
4. **Local work survives interruption.** A closed browser, failed AI call, canceled dialog, application restart, or supported upgrade must not destroy approved materials or silently change their associations.
5. **Optional capabilities remain optional.** Job organization and manual preparation must work without AI credentials or a managed browser. Unsupported employer controls retain a comprehensible manual path.
6. **One product, platform-honest delivery.** The core user workflow should feel consistent across qualified operating systems, while native install and platform dependency details can differ.

## 4. Primary user journeys

### Journey A — Install and return to a usable workspace

**User intent:** Download YourMove, open it, and begin organizing or continuing applications without an account or mandatory AI setup.

**Proposed experience:**

1. The user chooses an explicitly labeled platform download with architecture and support status.
2. Installation instructions explain prerequisites, signing/trust limitations where relevant, and whether assisted employer-form work requires a separate browser.
3. On first launch, the application clearly offers **add an opportunity** or **resume existing work**. If there is no prior workspace, the empty state provides one next action rather than a broad feature tour.
4. The workspace remains local and available after quitting and reopening the app.
5. Missing browser or AI prerequisites affect only the corresponding optional behavior, not job management and manual preparation.

**Expected result:** A job seeker can reach a functional local workspace through the distributed native application. A source-run web interface or passing packaging job alone does not meet this journey.

**Sources:** YM-002, YM-005, YM-008, YM-010, YM-242, YM-266; XP-01, XP-04, XP-05, XP-07–XP-09, XP-14.

### Journey B — Complete a first application without hidden automation

**User intent:** Save a posting, choose the correct résumé, prepare relevant answers and a letter where useful, and personally submit to the employer.

**Baseline behavior to preserve:** Opportunity → prepare/review materials → inspect employer application → explicitly authorize any supported transfer → review on employer site → submit personally → record completion locally.

**Proposed refinement:** Each workspace clearly indicates the current safe next action and any unresolved preparation. If the employer has no narrative questions, the user is not forced through a meaningless question step. Before transfer, the selected résumé and other intended materials are clear.

**Expected result:** No ambiguity between “I prepared this,” “YourMove filled a field,” and “I submitted the application.”

**Sources:** YM-082, YM-085–YM-087, YM-091, YM-141–YM-142.

### Journey C — Make the second application easier (primary differentiation)

**Starting condition:** The user has an existing opportunity with an approved résumé choice, one or more saved answers, and optionally a saved cover letter. They now save a different job posting.

**Target experience:**

1. **Start:** The new opportunity opens with a focused view of the materials potentially useful again, rather than presenting every preparation step as brand-new work.
2. **Résumé:** If prior manual choices or role-family associations offer a plausible résumé, show it as a *suggestion* with its document identity. Do not silently assign or transfer it. The user can search/select another version.
3. **Questions:** For each newly observed question, distinguish:
   - **Exact reuse eligible:** Existing answer meets the product's exact-match rules; show which saved answer is being used.
   - **Suggested prior answer:** Related wording or topic; user must inspect and intentionally copy/adapt/save it.
   - **No reusable answer:** Provide a straightforward editor; never fabricate an answer.
4. **Letter:** If a saved prior letter is useful, offer **Use prior letter as starting draft**, visibly editable, without treating the previous employer-specific claims as approved for the new role.
5. **Prepare:** Show the unresolved work and a primary action appropriate to the actual state. Preserve drafts if the user navigates away or the employer question list changes.
6. **Continue:** The user reviews the proposed transfer and explicitly authorizes it. Afterward, show what the browser observed, what was skipped, and what remains uncertain or manual.
7. **Complete:** The applicant submits on the employer site and records completion separately in YourMove.

**Expected result:** The second application requires fewer repeated entries, selections, and corrective actions than the same journey in v0.1.0, **without more automatic employer actions**.

**Primary sources:** YM-061, YM-068, YM-081, YM-085, YM-091, YM-101, YM-103–YM-104, YM-121, YM-142, YM-287. The exact presentation and suggestion logic above are proposed design decisions, not verified current implementation.

### Journey D — Employer form differs, fails, or becomes uncertain

**User intent:** Continue safely even if a form changes or includes a control YourMove cannot reliably handle.

**Proposed experience:**

1. The user knows which observed fields YourMove intends to fill.
2. A material change in employer field identity requires fresh review rather than assuming an older authorization applies.
3. Transfer feedback distinguishes **filled**, **unchanged**, **skipped**, **uncertain**, and **needs manual action** where the observation supports those states.
4. An unsupported field receives field-specific manual guidance where available. An unknown outcome is never automatically retried as a write.
5. A lost browser session leaves saved local drafts intact. The app explains what is recoverable locally and what must be checked on the employer website.

**Expected result:** Users can continue manually without confusing a partial transfer with a completed application.

**Sources:** YM-141–YM-144, YM-150, YM-164, YM-168. These items are candidates; the full per-field preview may be reduced if the smallest usable slice proves sufficient.

### Journey E — Upgrade without losing accumulated work

**User intent:** Install a later build and find the same opportunities, résumé bytes, approved answers, letters, evidence associations, and completion history.

**Proposed experience:** The upgrade preserves the existing workspace. Where a reference cannot be resolved, YourMove reports the problem and offers repair; it does not silently replace the file or claim the job is ready. Backups and restore should encompass the SQLite database *and* referenced documents.

**Expected result:** An actual v0.1.0-to-v0.2.0 upgrade on macOS, plus candidate-to-candidate upgrades on new platforms, preserve synthetic representative user work.

**Sources:** YM-261–YM-262, YM-266–YM-268; XP-05, XP-11.

## 5. Screens and user-visible changes

| Surface | Proposed user-visible change | Boundary |
| --- | --- | --- |
| Opportunities/home | Clear resume-current-work affordance; next safe action | No mandatory onboarding or automated job applications |
| Documents | Searchable opportunity association and reviewable résumé suggestion | File identity and applicant choice remain explicit |
| Application workspace | Unresolved-work emphasis, state-specific primary action, selected-materials overview | Existing approved work is not silently changed |
| Questions / Answer Library | Exact-match reuse visibly separate from manually reviewed suggestions | No semantic auto-fill of changed employer questions |
| Cover letter | Start a new editable draft from a prior saved letter | Prior employer-specific claims are not assumed applicable |
| Apply / employer continuation | Planned/observed transfer summary and clear manual fallback | No employer submission, no unsafe replay |
| Settings / availability | Platform/browser prerequisites and limitations in plain language | Do not advertise a browser engine merely because code exists |
| Download / release notes | OS, architecture, installer type, signature/trust status, qualified capabilities | Build availability is not support qualification |

The table identifies *experience responsibilities*, not a mandate to redesign each page or add a new route. Prefer changing an existing surface if it can support the outcome.

## 6. Platform experience and distribution promise

**Proposed targets (not yet qualified):**

| Platform | Intended v0.2.0 experience | Current release status at drafting |
| --- | --- | --- |
| Apple Silicon macOS 26+ | Preserve existing native journey; qualify the repeat-application improvements and upgrade | v0.1.0 technical-alpha baseline exists |
| Windows 11 x64 | Install native app, use local workspace, prepare and authorize supported employer transfers | Native target to implement and qualify |
| Ubuntu 24.04 x64 | Same core workflow through an Ubuntu-native installation | Native target to implement and qualify |
| Windows/Linux ARM64; Intel/older macOS | No default v0.2.0 support commitment | Later investigation only |

A platform's journey must be checked in stages: **source compiles → artifact builds → clean install launches → local workflow works → browser-assisted journey qualifies → platform is eligible to be advertised as supported**. Qualification at one stage must not be presented as evidence for a later one.

**Proposed release policy:** Ship the platforms that pass the relevant evidence gates; mark other targets *preview*, *not yet supported*, or omit their installers. A successful macOS/Windows release need not be blocked indefinitely by Ubuntu if Ubuntu is still unqualified, but that choice requires a recorded release-owner decision.

## 7. How to judge whether v0.2.0 is better

Use the same representative synthetic two-opportunity workspace for the v0.1.0 baseline and v0.2.0 acceptance. Where useful, invite consenting testers to repeat the journey and report friction without sharing private job-seeker data.

**Product comparison signals:**

- Manual text entry that repeats already approved information.
- Number of explicit material selections and corrections.
- Time from opening a second opportunity to reaching a reviewable employer form.
- Number of unresolved or confusing states encountered.
- Whether the user can correctly explain what was prepared, transferred, and personally submitted.

**Non-negotiable invariants:** No silent data loss or misassociation; no unapproved field mutation or employer submission; no false transfer-success claim; no unexpected external sharing of applicant material.

Do not set arbitrary percentage-improvement claims before measuring the baseline. Passing tests demonstrates behavior on those paths, not universal ATS compatibility.

## 8. Proposed release scope

### Release-defining experience (proposed commitment)

- A complete second-application journey based on *reviewed* résumé and answer reuse, unresolved-work guidance, and truthful transfer results. Choose the smallest UI/service slice that demonstrates improvement.
- Native platform enablement and qualification efforts for Windows 11 x64 and Ubuntu 24.04 x64, while preserving supported macOS behavior. Only qualified platforms become release claims.
- Preservation of real workspace contents across restart and supported upgrades; exact artifact identity and platform-specific installation instructions.
- Clear manual completion path when employer filling is unavailable or unsupported.

### Conditional additions (only if the journey needs them)

- Expanded answer browsing, role-family rationale, prior letter metadata, detailed per-field Fill preview, extra accessibility/window refinements, and local backup UI.
- Additional browser-engine prerequisite help when tests identify a common point of failure.
- Limited platform-specific adjustments to menus, dialogs, and appearance.

### Deliberate exclusions

- Automatic employer submission or unattended application processing.
- Universal ATS compatibility or unqualified live-employer support claims.
- Career Discovery/Studio/Realize integration as a release dependency.
- Cloud accounts, cloud sync, mobile apps, an automatic updater, or a new generalized platform/ATS plugin architecture.
- ARM64 Windows/Linux and Intel/older macOS as launch requirements.
- A repository-wide architectural or frontend rewrite.

## 9. Decisions for product review

| ID | Decision needed | Suggested default | Status |
| --- | --- | --- | --- |
| D-01 | Must Windows and Ubuntu *both* ship in v0.2.0? | Target both; release each only after qualification | Proposed |
| D-02 | What makes the second application measurably easier? | Less repeated entry/correction and clearer next steps, shown on one two-job journey | Proposed; baseline needed |
| D-03 | Should a similar previously saved answer ever be auto-applied? | **No.** Similarity permits suggestions only; applicant reviews and chooses | Proposed |
| D-04 | May a suggested résumé be silently assigned to a new job? | **No.** Show the suggestion and require explicit selection | Proposed |
| D-05 | Does normal signed/notarized distribution block an invited alpha? | No; label alpha trust limitations accurately. Decide public distribution separately | Proposed |
| D-06 | How much of the per-field transfer preview must v0.2 implement? | Prioritize a truthful result summary; add detailed preview if evidence shows it prevents meaningful error | Open |
| D-07 | Which installed-platform prerequisites will YourMove bundle versus ask users to install? | Decide from native feasibility results and distribution burden | Open |
| D-08 | What is the v0.2.0 supported release audience? | Explicit per-platform beta audience after real install/upgrade acceptance | Open |

**Approval should settle D-01 through D-04 and the release audience at a useful level.** D-06 and D-07 can be decided from the first implementation/qualification spikes without delaying unrelated product work.

## 10. Handoff to engineering and design

From this document, the next artifacts are:

1. One annotated storyboard or low-fidelity walkthrough for **Journey C**, using existing screens where possible.
2. One baseline run of the two-application v0.1.0 synthetic fixture, recording repeated steps and corrections.
3. One per-platform native acceptance specification for **Journeys A, C, D, and E**.
4. A short dependency-aware implementation plan that starts with one real native platform slice and one real repeat-use product slice, rather than a horizontal abstraction project.

**Stop planning and start implementing when:** the release outcomes and user-authority decisions are approved, one representative journey is unambiguous, actual Windows/Ubuntu test access is identified, and the smallest implementation slices have observable acceptance. Platform unknowns should be resolved by executable spikes rather than more prose.

## Source basis and qualification notes

- `yourmove-go-300-v020-candidates.md` and `.csv`: original 300 improvement ideas, with **148 classified as potential v0.2.0 candidates and 152 as later/research**. These classifications do not constitute implementation commitments or verified defects.
- `YourMove-Go-v0.2.0-proposed-scope.md`: two proposed release outcomes; XP-01–XP-14 cross-platform proposals; re-ranked candidate inventory and conditional release approach.
- `yourmove-go-repository-architecture-reference.md`: keep current application boundaries and make only evidence-motivated refactors.
- Public repository and recorded v0.1.0 release: https://github.com/jenksed/yourmove-go and https://github.com/jenksed/yourmove-go/releases/tag/v0.1.0 .

*This is a product specification draft, not verification that the proposed features exist, that any untested platform works, or that the release owner has approved the scope.*
