# Experience strategy and competing directions

## Thesis

YourMove should compound the value of careful preparation. Each accurate fact, reviewed experience, selected document, and useful answer should reduce future repetition while retaining its source and scope. The applicant should spend attention on the employer's actual question, not on reconstructing previous work or interpreting automation state.

**Clarity:** show the current object, its state, and one consequential next action. **Flow:** keep the job context while changing writing activity; preserve queue position and filters. **Trust:** state what is saved, what is proposed, what is authorized, and what was actually observed independently.

### Information architecture

- **Work:** Opportunities; Applications.
- **Your materials:** Documents; Candidate Profile; Answer Library; Career Evidence.
- **Preferences and tools:** AI Connections; Utilities.
- **Inside a job:** Overview; Prepare; Answers; Cover letter; History. Transfer review is a subtask of Prepare, not a new global destination.

Keep the existing routes. Initial implementation can expose existing anchors through progressive enhancement; new route/query variants must be checked against Wails redirect allowlists. No tab should create a duplicate editor. Avoid making a native WebView depend on browser tabs for managing source material: qualify the current `target=_blank` links, then provide deliberate in-app return context if required.

A 184px labeled sidebar is the prototype starting point at content widths of 820px and above. Below that, use a named Navigation control opening the complete labeled destination list. Manual focus mode can hide the sidebar at wider sizes. The breakpoint is a candidate to qualify, not an assertion about the outer native window. Avoid a persistent icon-only rail: it saves space but adds decoding and does little to explain the difference between the two libraries.

One primary document scroll, with ordinary native textareas allowed to scroll internally during editing. Avoid independently scrolling list/editor/help panes. Long Help content follows the editor; returning from source review restores focus to the invoking control. Do not hide a required authorization or blocker in an optional disclosure.

### Behavior priorities

1. Retrieve and adapt existing accurate material before generating more text.
2. Keep manual work productive without AI, discovery, or browser filling.
3. Never make switching an activity a data mutation.
4. Make browser ownership visible. One job's active application must not be silently replaced by another.
5. Preserve explicit save, review, transfer, and recording boundaries. A common visual language does not collapse distinct permissions.
6. Do not add autosave without durable draft and conflict semantics. The prototype demonstrates explicit save only.

## Directions and evaluation

**A — Labeled workbench.** Stable object navigation; compact queue; operational preparation rows; direct writing access; separated transfer review. Best balance of frequent use and first-time orientation. Costs: more job-level destinations and an additional explicit entry to transfer review. These are justified only if return/focus behavior stays coherent.

**B — Editorial workspace.** Jost headings, wider vertical intervals, sequential preparation and larger writing measure/type. Best for reflection and long reading. Loses working rows and controls in compact heights; makes routine operations feel slower. Retain its bounded prose and artifact-first ordering, not its full-screen density.

**C — Task focus.** Global navigation lives in a named menu; job activities become prominent contextual choices. Makes maximum space for one task. Strong optional writing mode, weaker default for people still learning where documents and reusable material live. Avoid turning it into a compulsory linear wizard: applicants often work out of order.

The comparison board includes identical-data Opportunities and switchable Prepare, Answers, Documents, and Letter views. B and C inherit the same underlying capabilities and safety boundaries. Their composition differs in sequential versus parallel sections, always-visible versus menu navigation, writing scale, and the prominence of choosing one current job. These are prototypes, not tested product variants.

### Expert ratings

Scale 1–5: 1 substantial weakness, 3 workable with friction, 5 strong proposed fit. Existing ratings combine source inspection and limited current renders; proposed ratings are design hypotheses supported by prototype renders, not user-test outcomes. No weighted overall score hides the native evidence gap.

| Dimension | Existing | A | B | C | Basis |
|---|---:|---:|---:|---:|---|
| Visual identity | 3 | 4 | 4 | 4 | Existing wordmark/green already distinctive; shared typography and quieter framing improve coherence. |
| Task clarity | 3 | 5 | 4 | 4 | Direct writing and explicit transfer review reduce competing decisions. |
| Navigation | 3 | 4 | 4 | 3 | A/B keep labeled destinations; C requires opening a menu. |
| Writing comfort | 3 | 4 | 5 | 5 | B/C favor writing space; A remains bounded and offers focus. |
| Information density | 2 | 5 | 3 | 4 | Rendered A queue shows two full rows where baseline shows none at 900×700. |
| Workflow speed | 3 | 4 | 3 | 4 | Reasoned reduction in navigation/scrolling; not timed with users. |
| Trust and safety | 4 | 4 | 4 | 4 | Backend strengths retained; clearer labels help, but new transitions require regression checks. |
| Consistency | 3 | 5 | 4 | 4 | Common object/action hierarchy across eleven principal surfaces. |
| Accessibility | 3 | 4 | 4 | 3 | Explicit labels and focus; C adds modal navigation obligations. Full audit pending. |
| Native fitness | 3 | 4 | 3 | 4 | Compact rendered layout supports hypothesis; all native ratings provisional. |
| Feasibility | 4 | 4 | 4 | 3 | A/B mostly composition; C requires broader navigation/return behavior. |

A is the recommended foundation. Use C only as an explicit focus option and compact-window navigation behavior. B's typographic restraint and bounded reading remain useful without adopting its large vertical rhythm. Beta readiness depends on a native acceptance pass, not these scores.

## Critique of Luna

Retain: source-derived shell diagnosis; explicit viewport caveats; local licensed Jost; synthetic content; no production changes; compact header; protection of handlers and consent.

Challenge: a 72px adaptive icon rail at 900 was selected primarily for recovered width. Width gain does not establish findability or task speed. It also narrows the 750px content relative to the existing horizontal navigation. All eight destinations were treated equally rather than grouped by applicant purpose. The prototype's combined “Fill application / Continue” action did not demonstrate exact scope. A green “Ready to continue” beside unanswered questions risks a blanket interpretation. The whole application requires more than shell substitution.

Extend: keep labels where they fit, expose writing as a job task, separate current objects from their generators, show state-specific recovery, reduce redundant queue framing. Reused Luna's synthetic naming conventions and licensed font assets; new composition and behavior are independent. Luna's historical screenshots are not counted as this package's native evidence.

## External pattern research

These are observations from public product documentation, not hands-on competitor usability research. Accessed October 22, 2026. Transfer choices below are design inferences.

| Reference | Documented pattern | Transfer to YourMove / reject |
|---|---|---|
| [Things keyboard guide](https://culturedcode.com/things/support/articles/2785159/) | Explicit open, save/close, selection, sidebar, and navigation shortcuts | Expose discoverable shortcuts beside visible commands. Reject immediate task-completion semantics for employer submission. |
| [Linear Peek](https://linear.app/docs/peek) | Search result documentation describes transient preview and adjacent-item navigation; direct fetch returned 404 in this session | Consider quick read-only job preview later. Do not make hidden Space behavior the only entrance. Lower-confidence reference; not required for recommendation. |
| [1Password Quick Access](https://support.1password.com/quick-access/) | Contextual suggestions plus explicit available actions | Put relevant reusable material beside the active question. Reject implied universal form control or autonomous filling. |
| [Raycast Action Panel](https://manual.raycast.com/action-panel) | Context-specific secondary actions grouped around an item | Document maintenance belongs to the document. Keep mandatory save/authorization visibly outside overflow menus. |
| [Teal Job Tracker](https://www.tealhq.com/tools/job-tracker) | Vendor presents centralized job tracking and related preparation tools | Centralizing context is useful. Do not import match scores, a SaaS dashboard, or marketing performance claims as evidence. |
| [Wails window options](https://v3.wails.io/features/windows/options/) | Window sizing, native options, and key bindings are available | Keep native chrome and validate pinned Wails APIs before adding shortcuts or window persistence. Current app only establishes sizing and native file callbacks. |

### Research still needed

Use synthetic tasks with 3–5 first-time and 3–5 frequent applicants: find a missing answer, distinguish saved from transferred, select the correct resume revision, recover a stopped browser, record completion, and locate reusable writing. Observe misinterpretations, backtracking, focus loss, and scroll distance. Compare A versus C at 900×700 before adopting default focus behavior. This is a proposed study, not completed research.
