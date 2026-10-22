# Checkpoint 2: Unified Answer Assistance and Visual Refinement

Date: 2026-10-22

Starting commit: `70b990def2ca0fc38af423340d7df386d79e9e90`

## A. Functional before and after

`prepareEarlyQuestions()` built AI source choices only for its server-selected question. The live workbench also eagerly built AI choices for every candidate, while client-side Next/Previous and direct selection only changed visible panels. A narrative question selected later could therefore have no usable AI editor.

AI controls, Answer Library suggestions, and Career Evidence suggestions now load through `POST /jobs/{id}/answers/help` after the applicant opens **Help me answer** for a selected narrative question. The handler rechecks the active job, destination, exact key and answer revision, narrative eligibility, and (for early questions) discovery source, fingerprint, and source question ID. It returns one question-specific assistance fragment. Review, preview recovery, generation tickets, consent, and provider selection continue through the existing AI handlers.

Previous writing, reviewed experience, and AI now share one Help area, with only one capability expanded by default. Previous-answer insertion works for early and live questions. Both flows check the current question and library revision, ask before replacing nonempty or edited text, insert only into the active editor, and leave saving to **Save answer locally**. Empty-library and no-match states are distinct and link to the full library. Career Evidence previews show action, context, and recorded/unknown/not-recorded outcomes; full provenance remains in a disclosure.

Saved answers remain per-question and revision checked. Switching questions does not submit or clear editor text; AI proposals remain read-only and separate until explicit adoption, then remain unsaved until local save. No early answer becomes live without the existing explicit acceptance action.

## B. Visual before and after

### Representative captures

The controlled baseline and after captures are in [`checkpoint2-visuals`](checkpoint2-visuals/). They use isolated synthetic fixtures; they are not captures of the user’s real job record.

| State | Before | After |
|---|---|---|
| Initial editor | [1280px light](checkpoint2-visuals/before-ai-preview-1280.png) | [1280px light](checkpoint2-visuals/after-initial-1280-light.png), [1440px dark](checkpoint2-visuals/after-initial-1440-dark.png), [390px light](checkpoint2-visuals/after-initial-390-light.png), [390px dark](checkpoint2-visuals/after-initial-390-dark.png) |
| Long question | — | [1280px light](checkpoint2-visuals/after-long-question-1280-light.png) |
| Previous writing | — | [1280px light](checkpoint2-visuals/after-previous-writing-1280-light.png) |
| Career Evidence | — | [1280px dark](checkpoint2-visuals/after-career-evidence-1280-dark.png) |
| AI source selection and review | — | [selection, 1280px light](checkpoint2-visuals/after-ai-source-selection-1280-light.png), [selection, 390px dark](checkpoint2-visuals/after-ai-source-selection-390-dark.png), [review, 1280px light](checkpoint2-visuals/after-ai-source-review-1280-light.png), [review, 390px dark](checkpoint2-visuals/after-ai-source-review-390-dark.png) |
| Generated proposal | [1280px light](checkpoint2-visuals/before-ai-preview-1280.png) | [1440px dark](checkpoint2-visuals/after-generated-proposal-1440-dark.png), [390px light](checkpoint2-visuals/after-generated-proposal-390-light.png), [390px dark](checkpoint2-visuals/after-generated-proposal-390-dark.png) |
| Saved answer | [1280px](checkpoint2-visuals/before-saved-answer-1280.png) | [1280px light](checkpoint2-visuals/after-saved-answer-1280-light.png) |
| No relevant suggestions | — | [no writing match, 390px dark](checkpoint2-visuals/after-no-matching-writing-390-dark.png), [no experience match, 390px light](checkpoint2-visuals/after-no-matching-experience-390-light.png) |
| Early Greenhouse preparation | — | [initial, 1280px light](checkpoint2-visuals/after-early-initial-1280-light.png), [AI review, 1280px light](checkpoint2-visuals/after-early-ai-review-1280-light.png), [proposal, 390px dark](checkpoint2-visuals/after-early-generated-390-dark.png) |

The baseline capture showed a compact single-question editor with AI directly under it. In the updated captures, the question and answer field now anchor the page, the rail truncates long labels to three lines with the complete label available in the editor and title, and the editor has more width at desktop sizes. At 390px the rail becomes one accessible selector, and the question remains fully legible. The assistance tools use one shared hierarchy, with previous writing and experience collapsed until requested and AI source settings progressively disclosed. The saved-state screenshot keeps “saved locally” distinct from employer transfer.

Question counts and workflow status appear once per workbench. Early discovery provenance is behind **Discovery details**. Existing safety copy remains where it affects decisions, including the separation of early and live answers and the fact that local saving does not fill the employer form. Light and dark captures use the established YourMove palette and green action token. Controlled screenshot focus outlines are visible because the fixture records keyboard focus around the active surface.

| Visual change | Original issue | Change | Easier now | Preserved |
|---|---|---|---|---|
| Question navigation | Long labels formed an uneven, hard-to-compare rail. | Three-line desktop clamp, full-question title, mobile native selector, tighter rail. | Scan status and move among questions without losing the original wording. | Question ordering, stable identity, keyboard links, and full active question. |
| Editor | Question, metadata, and controls had similar weight. | Stronger question typography, readable textarea dimensions, one green local-save action. | The employer prompt and answer area read as the main task. | Native text selection, keyboard editing, normal POST save, and dirty-answer protection. |
| Assistance | Three source tools appeared as separate capabilities or were hard to discover. | One question-specific **Help me answer** area with three progressive disclosures. | Find examples, experience, and AI from the current question. | Manual source choice, source review, consent, and separate AI preview/adoption/save states. |
| Status/provenance | Counts and inspection explanations repeated near the editor. | One concise count line; detailed discovery data in a disclosure. | Keep attention on the answer while retaining provenance on demand. | Inspection warnings, employer-only/manual categories, and transfer meaning. |
| Narrow window | A side rail consumed writing width. | One full-width native question selector at narrow sizes; wrapped secondary actions. | Read and edit without whole-page horizontal scrolling. | Normal forms, labels, visible keyboard focus, and browser-native controls. |

## C. Measurable results

All measurements below are synthetic test fixtures, not user data or real-world latency claims.

- Eligible narrative questions with usable AI assistance: initial selection only in the prior flow; all eligible questions in the new per-question flow. Controlled Chromium used three narrative questions, including a long prompt. Next, Previous, direct selection, reload, and preview recovery all activated the matching question’s AI controls.
- Starting points: live and early controlled Chromium adoption and local save passed. The early flow also checks/cancels replacement of typed text, validates discovery/source identity, revision, eligibility, destination, and library revision, and verifies that insertion alone does not persist. The source library entry remains unchanged.
- Fifty-question fixture: initial Prepare response was **154,752 HTML bytes in 56.1 ms**, with **50** assistance entry points and **0** rendered AI forms. Opening one question returned **9,384 bytes in 1.6 ms** with one AI form. Retrieval of library and Career Evidence recommendations now happens only for the opened question.
- Three-question controlled Chromium fixture: initial GET was **31,147 bytes in 6.6 ms**; there were **0** assistance requests before opening Help me answer, **8** requests during the full navigation/recovery journey, and question activation averaged **17.5 ms** (maximum **53.5 ms**). These are local fixture results, not production timing.
- Initial page data is bounded by the rendered actionable editors and question rail. It no longer carries all candidate AI forms or ranks recommendation lists for unopened questions. Before Help opens, the code does not call either suggestion-ranking path or load AI source choices. Opening a question runs one bounded Answer Library retrieval, one Career Evidence retrieval, then existing bounded AI source-choice reads for only that question. This query behavior is verified from the code path; a database statement trace counter was not added. There is no new external request until the user explicitly reviews and authorizes generation.
- Controlled screenshots at 1280px, 1440px, and 390px were reviewed in both themes. Chromium found no horizontal overflow at 390px. No native Wails WebView session was available, so native interaction remains unqualified here.

## D. Safety verification

- Controlled flows used isolated temporary SQLite/resume storage and a deterministic fake AI provider. Source review made **zero** provider calls; one explicit generation made exactly one call even under repeated activation.
- No employer form was filled or submitted in the controlled flows. Read-only preview behavior and the normal local POST save were exercised; no submission record or transfer was armed by assistance.
- Previous Answer Library text is a style/reference starting point, not factual verification. The original library record remains unchanged. Adoption targets only the active editor, requires confirmation when replacing text, and does not save.
- Early suggestions and AI requests are bound to the current discovery fingerprint and question identity. No early answer is promoted into live application answers without explicit acceptance.
- Reviewed Career Evidence remains the only Career Evidence AI source; resume source selection remains available without Career Evidence. Evidence and resume choices are not auto-selected, and stale source/question revisions are rejected.
- Checkpoint 1’s sensitive, demographic, legal, ambiguous, employer-only, and unsupported field categories are unchanged. Transfer, resume, contact, cover-letter, and submission behavior were outside this patch; existing repository suites passed.

## E. Changes and next checkpoint

Main changes are in `internal/app/application_answer_assistance.go`, `internal/app/application_answers.go`, `internal/app/early_application_answers.go`, `internal/app/answer_library_handlers.go`, `internal/app/answer_library_search.go`, `templates/job.html`, `static/ui.js`, and `static/styles.css`. Focused unit, race, and controlled Chromium qualifications are in the corresponding `internal/app/*_test.go` files. Visual evidence is stored beside this report in `checkpoint2-visuals/`.

The implementation uses a small server-rendered POST fragment for one exact question, rather than embedding a large set of AI forms or introducing a client-side application state layer. The endpoint independently validates question identity and eligibility. Recommendation ranking, source lists, and AI provider controls are all loaded only when that question’s Help area opens.

The broader Prepare composition still has duplicated resume/contact information and the expanded transfer scope identified in the original request. Those remain Checkpoint 3 work. No Checkpoint 3 redesign was started.

Native Wails was not available for this qualification. Manual check when running the desktop app: open a job with multiple narrative questions, open Help on a later question, load an example and cancel/confirm replacement, navigate with unsaved text, review AI sources and consent, generate once, adopt and save, then confirm the employer browser remains unchanged until the separate transfer action is authorized. Repeat at a narrow window and in both themes.

## F. Commit and CI

Verification completed for this checkpoint:

- `go test -count=1 ./...`
- `go test -count=1 -tags desktop ./...`
- Focused `go test -race` for question assistance and starting-point routes
- Controlled Chromium qualification for narrative AI, Answer Library, Career Evidence, and early Greenhouse preparation using synthetic fixtures and fake generation
- `go build ./...`
- `go build -tags desktop,production -o /private/tmp/yourmove-go-answer-assistance .`
- `node --check static/ui.js`
- `git diff --check`

Commit and push status will be recorded after branch and upstream verification.
