# Iteration 3.3A — Native desktop shell, typography, and visual direction

**Review date:** October 22, 2026<br>
**Starting point:** `add7bd9031fc0d5c466968a0d5f47bcee7d66418` (`main`)<br>
**Design branch:** `design/wails-desktop-shell-3-3`<br>
**Status:** source audit and synthetic HTML/CSS prototype prepared; screenshot qualification is incomplete because this session had no connected browser.

## A. Executive assessment

The current shell is functional and branded. Its main limitations are about how much of the window it spends on framing and how quickly it exposes the current work:

1. **The sidebar has the same width at 900px as at 1100px and takes 208px at 900.** It carries eight destinations, an eyebrow, and a two-line local-workspace message. In the 900px window that leaves 620px for all page content. The writing form is then narrower than its existing 780px maximum, and the cover-letter textarea fills the narrower column.
2. **The content frame starts low.** The desktop top bar is 68px, followed by 40px of page top padding. Opportunities and other page actions therefore start at 108px. At 750px, the horizontal navigation row adds another 54px before the 24px page inset; the content starts around 131px.
3. **Shell context repeats.** “Application Workspace / Local,” “Local workspace,” “Your resumes stay on this device,” and the footer’s “Local-first application workspace” repeat product and privacy context. The active page title already tells the user which work area is open. The persistent local note is useful once, but its current long form competes with navigation.
4. **The 760px breakpoint changes the whole shell at once.** The full-height sidebar becomes a second horizontal navigation row. At 750px that gives the page more width, but spends vertical room and can scroll out of view while the user is editing. This makes the minimum 700×550 window especially dependent on page scrolling to revisit navigation.
5. **Jost is not part of the current product typography and has not yet been visually qualified.** The system stack is appropriate for controls and long application copy. Jost may add brand character to headings, but its small-size and dark-theme rendering need to be compared in WebKit before choosing it.

The application structure supports a focused shell review: `templates/partials.html` is the shared frame, `templates/job.html` contains Overview, Prepare, History, and the 3.2 writing workspace, and the other requested destinations have their own server-rendered templates. The shell should make those workflows easier to reach without changing their Go-owned forms or decisions.

![Current 900 by 700 Prepare answer workspace, dark appearance](../2026-10-22-writing-workspace/answers-900x700-dark.png)

This is an existing Iteration 3.2 capture, included as baseline context. It is not a new shell comparison. The other existing Prepare views are [1100×800 dark](../2026-10-22-writing-workspace/answers-1100x800-dark.png), [750×650 dark](../2026-10-22-writing-workspace/answers-750x650-dark.png), and a [saved letter at 900×700 dark](../2026-10-22-writing-workspace/saved-letter-900x700-dark.png).

## B. Desktop-shell alternatives

### Source-derived viewport measurements

The following are CSS arithmetic from the current rules and the prototype rules, in CSS pixels. They assume the WebView content viewport equals the listed width; Wails outer-window dimensions have not been measured against its native content viewport in this session.

| Width × height | Current sidebar | Current page content width | Current header / top inset | Current first page content | Adaptive proposal: sidebar | Adaptive page content width | Adaptive header / top inset | Adaptive first page content |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1100×800 | 235 | 777 | 68 / 40 | 108 | 216 | 825 | 52 / 30 | 82 |
| 900×700 | 208 | 620 | 68 / 40 | 108 | 72 | 779 | 52 / 24 | 76 |
| 750×650 | horizontal nav | 718 | 53 / 24 | ~131 | 72 | 646 | 52 / 20 | 72 |
| 700×550 minimum | horizontal nav | 668 | 53 / 24 | ~131 | 72 | 596 | 52 / 20 | 72 |

Page content width is the main-column width after shell width and horizontal page gutters; it is not the textarea width after internal padding. The current figures come from `.app-shell`, `.topbar`, `.page-container`, and their 1050px/760px breakpoints in [static/styles.css](../../../static/styles.css#L67). The proposed figures use a 216px expanded sidebar, a 72px compact rail, a 52px application header, and 30/24/20/16px page gutters. The 750px prototype keeps the compact rail and uses 16px horizontal gutters. At 700px the proposed 596px page column leaves about 560px inside a 36px-padded editor.

The existing answer form is capped at 780px. Its available width is about 777px at 1100, 620px at 900, 718px at 750, and 668px at 700. With the proposed 780px writing cap, the available editor width would be 780px, 779px, 646px, and 596px respectively. At 1100 the editor is already at its cap, so collapsing the sidebar offers little writing-width gain. At 900 it adds about 159px. At 750 and 700 it reduces the editor by about 72px, while moving the start of page content up roughly 59px. The 596px minimum-window editor remains a useful text measure; the cost is a narrower canvas for side-by-side controls.

At 700px, the compact rail’s eight 42px navigation targets plus 4px gaps, wordmark, and vertical padding occupy about 442px of the 550px content height. The rail therefore fits without its own scroll area in this CSS model, while the main document scrolls normally. I would keep the declared 700×550 minimum for review; the native window still needs confirmation that the WebView content viewport and the smallest wrapped controls remain usable.

These are not Chromium measurements. Horizontal overflow, clipped controls, actual editor text width, scroll distance to Fill/Continue, and label wrapping remain unmeasured until the rendered matrix is captured.

### Sidebar approaches

| Direction | Measurement and behavior | Advantages | Disadvantages / regression risk | Complexity |
|---|---|---|---|---|
| **A — Refined fixed sidebar** | 216px on desktop; keeps the current horizontal nav below 760px. Page width is about 820px at 1100, 630px at 900, and 718px at 750. | Eight visible destination labels; predictable and least surprising for beta users; CSS-only shell change. | At 900, only about 10px more page width than today. At 750, navigation still occupies a horizontal row and leaves the content area when scrolling. | Low |
| **B — User-controlled collapsible sidebar** | 216px expanded, 72px compact. At 900 compact gives about 779px content; at 750 compact gives 646px. User toggles at any supported width. | Strong user control; at 900 recovers about 159px for the writing control; implementation can remain local to shell markup/CSS/JS. | First-run state and state across full-page GET navigation need a clear rule. Icon-only destinations are harder to scan until expanded. An expanded sidebar at 700 can leave a narrow editor. | Medium |
| **C — Adaptive compact rail (recommended for prototype review)** | 216px and full labels above 920px; 72px compact rail at 920px and below; a labeled top-bar toggle can override. At 1100 it retains full navigation; at 900 it recovers about 159px; at 750 it uses a vertical rail and preserves page height. | Applies compactness where current content is tightest; avoids switching all the way to a horizontal navigation strip; active page title remains in the page; preserves all eight links. | Compact icons reduce at-a-glance label discovery. The “Expand navigation” control, focusable labels, and focus/hover tooltips need visual and keyboard qualification. At 750 the writing column is 72px narrower than current. | Medium |

All approaches keep eight primary destinations. Expanded mode exposes eight labels. Compact mode keeps eight link targets with explicit accessible names, visible keyboard focus, a focus/hover tooltip, and an active state. The prototype includes a clearly labeled “Expand navigation” control, so discovery does not depend on hover or recognizing an icon. Focus/hover tips are supplemental.

**Recommendation:** compare Direction C first, with A as the low-complexity fallback. C has the largest meaningful gain at the ordinary 900×700 target while keeping full labels in the spacious 1100×800 window. Direction B remains useful if owners prefer direct control over a width-triggered default. The recovered-width estimate is not a usability claim until the same answer and letter content is rendered in the matrix.

### Header and content layout

The prototype reduces the app header from 68px to 52px and the page top inset from 40px to 30/24/20px. It removes “APPLICATION WORKSPACE / LOCAL,” keeps a quiet but labeled Appearance selector at the right, and keeps page-specific titles and actions in their existing page heading. The shell toggle lives at the left of the header; it does not impersonate or replace the macOS title bar. No custom traffic-light controls or draggable regions are proposed.

The persistent local-workspace message is reduced to one short cue in expanded mode and omitted in compact mode. The current sidebar and footer repeat “local” and submission reassurance; neither needs to appear on every screen. Prepare keeps its actual continuation scope and explicit user-confirmed submission language at the decision point.

The prototype contains synthetic Opportunities, Prepare, answer editor, saved letter, Documents, Candidate Profile, AI Connections, Answer Library, Career Evidence, Applications, and Utilities views. It uses the real 3.2 Prepare composition idea (one selected activity, active editor, saved draft, and local save controls), rather than an empty shell placeholder. No private records are used.

### Required screenshot matrix and current evidence

The existing 3.2 screenshots provide historical Prepare baseline context only. The prototype source is reviewable at [prototype/index.html](prototype/index.html), but this session had no connected Browser instance (`agent.browsers.list()` returned an empty list), so it could not produce new viewport captures or layout instrumentation. No Chromium or native Wails measurements are claimed.

| View | 1100×800 | 900×700 | 750×650 | Evidence status |
|---|---|---|---|---|
| Existing baseline | Existing Prepare capture | Existing Prepare capture | Existing Prepare capture | Prior controlled screenshots; not a new shell review |
| Refined fixed sidebar | Not captured | Not captured | Not captured | Prototype ready |
| Collapsible sidebar | Expanded and compact not captured | Expanded and compact not captured | Compact not captured | Prototype ready |
| Adaptive recommendation | Not captured | Not captured | Not captured | Prototype ready |
| System versus Jost | Not captured | Not captured | Optional view not captured | Local specimen and font are ready |
| Recommended light and dark | Not captured | Not captured | Not captured | Prototype ready |

The screenshots are therefore an outstanding deliverable. The design direction should remain provisional until these captures are reviewed, especially the active answer editor at all three targets and the compact 700×550 minimum window.

## C. Typography comparison — Jost

The current control is the system stack from `static/styles.css`: `-apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif`. It is retained for dense fields, employer questions, long descriptions, technical metadata, and instructions.

The prototype implements the three requested configurations against identical synthetic copy and includes 11px, 12px, 13px, and 14px samples, the long Northstar employer question, a saved answer paragraph, the cover letter, and Jost weight samples at 300/400/500/600/700. It includes the requested heavier treatment explicitly: Jost Semibold 600 and Bold 700 appear on large-heading comparisons. Query links and the local viewer instructions are in [prototype/README.md](prototype/README.md).

The local specimen uses the official Google Fonts repository’s `Jost[wght].ttf` with a 100–900 weight axis; its metadata records Jost, Owen Earl, and the Jost Project Authors’ copyright. The repository includes `OFL.txt`; SIL OFL 1.1 permits embedding and redistribution with software when the copyright notice and license accompany each copy, subject to the license’s reserved-name conditions. The prototype contains the font and notice locally and works offline. Sources: [Google Fonts Jost directory and variable font](https://github.com/google/fonts/tree/main/ofl/jost), [Jost metadata](https://github.com/google/fonts/blob/main/ofl/jost/METADATA.pb), and [OFL 1.1 notice](https://github.com/google/fonts/blob/main/ofl/jost/OFL.txt).

The machine has no installed Jost family. I retrieved the official variable font and bundled it only in this isolated prototype, but could not render or inspect it because the browser service was unavailable. I therefore do **not** claim that Jost 300, 400, 500, 600, or 700 is more legible in WebKit, that it wraps better, or that its dark-mode contrast is better. No production font assets were added.

**Provisional typographic candidate:** selective Jost at 600 for major page and section headings, with 700 tested for large headings; keep body, controls, employer question text, and small supporting copy in the system font. Treat 300 as a display-only specimen, not a default for dense product screens. This is a candidate for visual review, not an approved choice. The owner’s suggestion to make Jost bolder is reflected in the 600/700 specimens.

## D. Recommended design

Review the adaptive 72px rail below 920px, 216px expanded sidebar above that point, 52px app header, and 20–30px top/page gutters. Keep the compact rail’s active item clear and put its human-readable current page title in the content heading. The header toggle must say “Expand navigation” when compact and “Collapse navigation” when expanded; eight nav anchors remain in the DOM with stable URLs and accessible names.

At 900×700, this recovers approximately 159px of content width and 32px of top framing relative to the source-derived baseline. At 750×650 it accepts a 72px narrower page column to retain vertical navigation and move the first page content up around 59px. At 1100×800 it keeps all labels visible; the writing editor remains capped at 780px. These are geometric estimates, not demonstrated usability gains.

This direction is the best candidate because it addresses the measured narrow-window constraint without imposing an icon rail on the default spacious window or changing the page-specific product workflows. The key unresolved risk is first-use label discovery; the screenshot and keyboard pass must confirm that the labeled toggle and focus tooltips make compact navigation understandable.

## E. Implementation blueprint for a later 3.3B assignment

No production shell was changed in this design stage. If the owner approves this direction, likely production edits are:

- **`templates/partials.html`** — Add a real button with `aria-controls`, `aria-expanded`, and explicit Expand/Collapse text; retain all eight links; wrap link labels so compact CSS can visually hide them without removing their accessible names; keep `aria-current="page"` consistent for every active destination.
- **`static/styles.css`** — Replace/consolidate the shell rules at lines 67–90 and the overlapping 1050px/760px layout rules at lines 145–148. Set 216px expanded/72px compact widths, 52px app header, adaptive gutters, `min-width:0`, and sidebar vertical overflow. Scope compact tooltips to `:hover` and `:focus-visible`, retain visible focus, honor `prefers-reduced-motion`, and make text zoom/wrapping safe. Bound the saved letter editor to the existing ~780px writing measure; do not constrain unrelated pages globally.
- **`static/ui.js`** — Add a small progressive enhancement for the toggle and current compact state. Keep all links usable with JavaScript disabled, reflect state through `aria-expanded`, and do not alter form or route behavior. For an override across server-rendered GET navigations, `sessionStorage` is the smallest client-owned state because it ends with the app session; use the responsive CSS default when storage is unavailable. Read/write it behind try/catch and confirm the Wails-served origin retains the same session across normal routes. Do not add SQLite settings or a new settings system. The design prototype itself does not persist state.
- **`desktop.go`** — No change is indicated. Keep 1100×800 default and 700×550 minimum for initial qualification. The compact rail leaves source-derived widths of about 596px at the minimum; only raise the minimum if native evidence shows a required control cannot fit.
- **Font packaging, only if separately approved** — Add a local font under `static/` and the full OFL notice/copyright. Do not use a Google Fonts runtime request. Use Jost selectively only if Chromium and native WebKit review support it.
- **No change** — `internal/app/app_http.go`, Go POST handlers, Wails redirect adaptation, consent flows, forms, database, and application-workspace behavior.

The main regressions to guard against are compact controls lacking discoverable names, labels becoming inaccessible when hidden visually, sidebar scrolling away from keyboard focus, long employer names forcing overflow, 700×550 form controls clipping, state flashing or resetting after Wails redirects, and text wrap changes that break the current editor or unsaved-draft review. The active application page title and viewport focus must remain clear when resizing.

## F. Verification strategy for the implementation assignment

### Controlled Chromium qualification

Use the same synthetic record set for all shells and typography configurations. Capture viewport-sized images at 1100×800, 900×700, 750×650, and 700×550. Include Opportunities with a long company/position and filters; Prepare with selected resume, readiness, Fill/Continue scope, active answer editor, long question, Help me answer and previous/next; saved cover-letter editing and version access; Documents with long PDF/DOCX names and management disclosure; plus light/dark recommended-shell views.

Measure each capture: CSS viewport and shell widths, page/editor width, header height, first meaningful action y-position, visible destinations, horizontal overflow, clipped controls, long-label wrap, document-name wrap, and scroll position needed to reach the primary interaction. Run keyboard-only focus through the compact rail, toggle, links, editor and controls. Check visible focus, accessible names, active destination, text zoom, no-JavaScript fallback, resize from expanded to compact and back, and reduced motion. Verify that state survives a normal server-rendered GET in the same tab, then resets when the app session ends.

### Native Wails acceptance

Chromium is a controlled layout qualification surface, not native acceptance. Build and launch the actual Wails desktop with isolated synthetic storage on macOS. At 1100×800, 900×700, 750×650, and 700×550, manually resize the real window; inspect actual content viewport dimensions, macOS title-bar relationship, sidebar state, system appearance, native select/input baselines, focus transfer, scroll restoration, and `sessionStorage` across existing GET/POST redirect paths. Confirm offline startup with the local font if chosen, and inspect light/dark WebKit rendering. Keep the native title bar. Do not infer native success from Chromium screenshots.

The current desktop host is Wails v3 `v3.0.0-beta.32` in `go.mod`; `desktop.go` sets width/height/minima and serves the shared app handler. Wails v3 documents logical-pixel window sizing, minimum sizes, and a native WebKit WebView on macOS; CSS should own this layout and respond to the WebView viewport, without a window-resize API. Confirm content viewport and storage behavior in the actual build. References: [Wails window basics](https://v3.wails.io/features/windows/basics/), [window options](https://v3.wails.io/features/windows/options/), and [platform dependencies](https://v3.wails.io/quick-start/installation/).

## G. Open decisions

1. **Sidebar direction:** Direction C is the recommended candidate; compare its 900px gain and 750px width tradeoff with fixed A and user-controlled B in the missing screenshot matrix before approval.
2. **Typography:** Keep the system stack as the control. Decide whether selective Jost belongs on headings, and whether those headings should use Semibold 600 or Bold 700, only after reviewing the matching specimens in Chromium and native WebKit.

No production implementation, merge, or Iteration 3.4 work is part of this branch. The design direction is not approved by this report.
