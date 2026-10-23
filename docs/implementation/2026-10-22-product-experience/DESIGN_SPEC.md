# Recommended visual and interaction system

## Typography

Keep the existing wordmark. Use the macOS system stack for navigation, controls, questions, metadata, and editable prose. Prototype body is 14px/1.5, metadata 12px/1.5, labels 13px/600, page title 28px/650, job title 25px/650, section title 19px, question 20px/1.45, textarea 15px/1.65. Avoid all-caps explanatory paragraphs. A small eyebrow may identify employer or source, but must not repeat the heading.

Jost is bundled locally with its complete OFL notice. The specimen compares 12/14/16px system and Jost text, real question wrapping, and 300/400/500/600/700 weights. Direction B uses Jost 500 for headings. Recommendation: Jost 300 only for large brand/display uses; 400 has no demonstrated dense-control advantage; 500/600 can be evaluated for large headings; 700 is too assertive as the default alongside the heavy existing wordmark. This is visual judgment, not measured legibility. Native WebKit rendering remains outstanding. No runtime Google Fonts request.

## Color and surfaces

| Token | Light | Dark | Use |
|---|---|---|---|
| Background | `#f7f8f5` | `#171d1b` | Calm work surface |
| Surface | `#ffffff` | `#202824` | Inputs and independent review groups |
| Subtle | `#eef1ec` | `#29322d` | Navigation/secondary grouping |
| Text | `#202b27` | `#edf2eb` | Primary content |
| Muted text | `#59675f` | `#afbbb1` | Supporting text, never disabled essential information |
| Primary/selection text | `#245f4c` | `#acd6b6` | Deliberate action and active destination |
| Selected background | `#e5eee7` | `#2e4436` | Current item |
| Control border | `#89998d` | `#7b8f80` | Visible input boundary |
| Attention | `#81521b` | `#e9c18a` | Actionable uncertainty/missing material |
| Danger | `#a23731` | `#f4aaa3` | Destructive operation |
| Focus | `#2468ae` | `#91bcf0` | 3px outline, 3px offset |

Text labels carry state; color is supplemental. A hollow dot means a neutral state, not unfinished work in every context. Green selection is not evidence of employer acceptance. Avoid rainbow stage pills. Separate surface dividers may be quieter than interactive control borders. Verify contrast across every actual background during implementation; do not infer accessibility from token names.

## Shell and composition

- 184px labeled sidebar; grouped Work, Your materials, Preferences/tools. Default 48px web toolbar below native chrome; no fake traffic lights or draggable header.
- 28px content gutters at spacious sizes, 22px intermediate, 20px compact. Spacing rhythm 4/8/12/16/24/32; borders delimit actual objects or decisions.
- One page heading; job identity persists across activities. Five text destinations fit ordinary desktop widths and wrap rather than horizontally scroll.
- Below 820px **content viewport**, replace sidebar with named Navigation menu; keep all labels. Manual focus mode may hide sidebar at wider widths. Persist layout preference separately from work state, tolerate unavailable storage, restore focus on close. Prototype preference is intentionally ephemeral.
- Writing max 780px; normal prose roughly 60–80 characters per line. At 900px the labeled shell still permits a useful editor. Below 820px reclaim sidebar width instead of compressing to an icon rail.
- No giant sticky job header. The page scrolls naturally; save follows the editor. At 700×550 a long question requires scrolling to save; do not shrink text to force everything above the fold. Consider a compact save affordance only after native overlap/focus evidence.

## Controls

Production baseline is 42px form controls; prototype explores 40px inputs and 36px secondary buttons. Adopt the denser sizes only after native pointer/focus acceptance. Adjacent controls must share baselines; textarea uses native selection, spellcheck, and vertical resize. Primary action per decision, not per card. A save button and transfer button must never look like interchangeable outcomes.

Use actual anchors for destinations, buttons for commands, labeled native selects for question choice, native details for secondary settings. Implement keyboard tab semantics only if the element is truly a tab widget. Hash navigation must reveal target sections without JS; do not hide all content behind a client router. Prototype routing is illustrative.

Questions show complete employer wording; shortened selector labels are navigation aids only. Save state sits beside save: “Unsaved changes,” “Saved locally,” “Not saved.” Never use a transient toast as the only failure evidence. Move focus to an error summary or preserved editor; keep recoverable text copyable.

## Content and state

| Replace / avoid | Prefer |
|---|---|
| Verified information | Reviewed by you / Approved by you |
| Ready (global) | Resume selected; 2 answers saved locally; browser not connected |
| AI answer complete | Generated proposal · not saved |
| Applied / success without context | Submission recorded locally |
| Resume uploaded successfully | File selected in employer browser; inspect acceptance |
| Continue after restart | Open browser without filling; recheck current form |
| Save revision as leading copy | Save profile; explain approval next to checkbox |
| Provider ready | Configured for this session; generation checks access |

Preserve exact transmission scope and permission copy where consequential. The transfer preview must use server-authoritative saved material, including revision/hash bindings. Do not synthesize eligibility in JavaScript. Each partial result retains its own meaning.

## Keyboard, accessibility, motion

Visible focus, skip link, labels, live status, sufficient targets, no hover-only actions. Menu/dialog Escape closes and restores invoking focus; cancellation never saves, submits, or silently discards writing. Test VoiceOver reading order, 200% zoom, enlarged system text, keyboard-only flow, and both themes. Command-S may invoke the active explicit save only after native shortcut testing; never intercept system Quit/Close or normal text-editing keys. Next/Previous remain visible controls. Shortcuts are an enhancement, not a navigation prerequisite.

Only short background-color transitions are used; reduced-motion disables them. No animated progress estimate for AI or fake percentage. Show named generation state and cancellation only when supported. Native file picker and export Save/Cancel stay authoritative.

## Prototype scope

The eleven required principal surfaces are rendered, plus transfer, utilities, state studies, and typography. Navigation, filters, question choice, dirty state, local simulated save, source review, separate AI proposal, explicit record confirmation, disclosure, theme, and focus can be explored. Maintenance operations are boundary explanations. The prototype does not establish real persistence, no-JS fallback, Wails redirect behavior, provider access, native file dialogs, or employer transfer. Those are roadmap acceptance work.
