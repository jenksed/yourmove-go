# YourMove UI foundations

**Status:** working design language, not a component-library project. Existing `static/styles.css` tokens and server-rendered Go templates remain the implementation. Revise this document when an actual design decision changes.

## Product character

YourMove is a focused local-first desktop workbench for applying to jobs. The interface should feel considered, capable, and calm. It is not a dashboard of everything the system knows. It should make the user's next decision obvious, show the evidence behind relevant recommendations, and leave submission authority with the user.

Aesthetic direction: **editorial utility with precise operational feedback**. Preserve the current YourMove wordmark and typography baseline. Fewer decorative cards, fewer repeated titles and instructions, more intentional negative space. Avoid generic gradient heroes and oversized banners inside task workflows.

## Foundations already implemented

- **Typography:** system sans-serif in `styles.css`; use size, weight, and alignment for hierarchy before introducing labels, pills, or decorative frames. One page heading; section headings identify tasks, not marketing promises.
- **Color:** existing light/dark tokens (`--color-bg`, `--color-surface`, `--color-text`, `--color-muted`, `--color-border`, `--color-primary`, semantic attention/info/danger variants). Green is an action accent, not a background treatment for every section.
- **Spacing:** prefer a stable rhythm of 8 / 12 / 16 / 24 px. Dense form controls are fine; large vertical gaps must separate *decisions* rather than decorate individual inputs.
- **Surfaces:** start with page background + separators. Add a panel only when it encloses a meaningful independent workspace or evidence group. No default panel-within-panel stacks.
- **Status:** expose selected, missing, ready, active, interrupted, and recorded only when they change the user's action or interpretation. Do not use badge chips for ordinary metadata.
- **Controls:** use one visually dominant action per decision context. Other destinations and optional activities should be links or secondary controls. Never demote consent, irreversible actions, or recovery just to save space.
- **Disclosure:** native `<details>` is suitable for optional activities (cover-letter generation, diagnostics, recorded-submission form). Essential blockers and the immediate next action stay outside disclosure.

## Screen compositions

| Surface | Primary user decision | Secondary information |
| --- | --- | --- |
| Opportunities | Which opportunity should I work on? | Sorting, filters, bulk operations |
| Job overview | Is this opportunity worth pursuing / do details need correction? | Source provenance, deletion |
| Prepare | Which resume will I use, then how do I continue? | Optional cover-letter studio, job brief, submission recording |
| Employer browser | What do I need to do to complete this application? | Field observations, public requirements, troubleshooting |
| Applications | Which recorded submission should I review? | Evidence caveat and history count |
| History | What submission did I record? | Details and notes |

These are distinct compositions, not variations of the same dashboard card stack.

## Behavior is not styling

The Go handlers remain authoritative. Keep server-backed form POSTs, explicit resume selection saving, per-action permission checks, source verification, draft versioning, and human submission confirmation. A WebView decoration must not imply the employer accepted an attachment or submission. Local and remote generation consent must remain explicit. Browser and Wails redirect compatibility are separate concerns.

The design must degrade honestly: without JavaScript, the page, forms, and sections remain usable; when JavaScript is present, hash-addressable tabs and progressively enhanced controls must still work. Honor keyboard navigation, focus indication, accessible labels, responsive layout, and reduced-motion preferences.

## Change discipline

Start from a real screenshot at ~1280 px and a narrow window (~390 px). Record (1) the current decision, (2) competing visual elements, (3) what was removed/deprioritized, (4) what user's ability must remain, and (5) a real-path check. Prefer adjusting one screen and its CSS selectors to inventing reusable abstractions. Reuse existing markup/tokens. Only promote a pattern to a shared component after observing another genuine consumer.

## R2: object-first layout and measurable consistency

The screenshot of an existing cover-letter draft revealed a missing rule: **a produced artifact must be easier to access than the tool that produced it**. When a letter exists, the editor opens first; model selection and regeneration live in a named secondary disclosure. When the letter is absent, the generation form is the main task. This lifecycle ordering is a composition decision, not a new model or storage state.

**Type and prose:** Operational headings name the current artifact or action; avoid a second marketing introduction inside a subtask. Keep body/textarea reading width roughly within 65–90 characters per line; for the writing desk, use a ~780px maximum width rather than stretching to the whole content region. Concise help text appears adjacent to its relevant field, not as a page-length preamble.

**Controls:** Standard labeled text inputs and selects use a 42px target height with consistent border, radius, and vertical alignment. Avoid native macOS dropdown size differences where they disrupt visual equality. The appearance selector, checkboxes, and file controls retain their appropriate variants. Verify the desktop WebView rendering as the final visual source of truth.

**Document lifecycle:** Rows show the object, current status and frequent action. Only then show lower-frequency maintenance operations through "Manage document." Native `<details>` provides mouse, keyboard, and no-JavaScript access without introducing client state or changing server forms.

**State-specific disclosures:** A saved letter has an editable artifact even if generation sources are no longer present; never conceal editing behind fresh generation readiness. Conversely, a missing job description or master resume blocks *generation*, not the ability to read or edit an existing saved draft.

**Cross-surface consistency:** Prepare, Documents and AI Connections share the same typography and action hierarchy, not identical card structures. The shell, color tokens, and other application screens remain stable; another screen only adopts a specific primitive when the same problem is observed there.

See `RULES.md` R6–R10 for specific failure modes and verification criteria.

## Prepare: discovered questions and exact reuse

Missing application questions lead in one bounded editor area; ready answers and approved candidate facts use a secondary disclosure. Each question has one authoritative local editor with an optional, narrowly scoped reuse choice. Saving is local; field/document transfer retains explicit consent. Live status polling must offer refresh instead of discarding unsaved edits. Resume/contact, saved-letter and answer approvals currently remain distinct actions within Prepare; consolidation is still an experience gap. Native qualification of the new save/transfer paths remains release work.

Ready, editable answers from the approved candidate profile appear in a compact “From your profile” disclosure below employer questions. They do not contribute to question navigation or counts. Missing, unapproved, manual, and application-specific answers remain in the workbench. The disclosure retains values and per-application override forms; presentation does not change transfer scope or approval. Checked in isolated Chromium at desktop and 390px widths, including keyboard navigation and no-JavaScript saving; native Wails visual verification remains separate.

## Opportunities: progress drives the dashboard

A prominent progress panel pairs current work with the suggested next opportunity. It shows opportunities left to work on, distinct jobs with applications recorded today (local calendar day), and overall jobs with recorded applications. Its progress bar measures recorded jobs against active plus applied opportunities; closed jobs are excluded. Recording an application increases the recorded counters and reduces remaining work on the next list load.

The suggested job and its existing action sit alongside progress on desktop and below it on narrow windows. Clicking the remaining metric returns the list to active opportunities and clears search. The saved lifetime total stays secondary list context. “Marked ready” remains a manual planning label, not an automated readiness check. All saved rows remain available through All stages and without JavaScript.
