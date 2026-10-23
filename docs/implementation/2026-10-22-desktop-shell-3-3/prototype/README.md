# Desktop shell prototype

This isolated page reuses the production `static/styles.css`, YourMove wordmark, navigation icons, and theme tokens. Its job, profile, answer, letter, and document content is synthetic. Prototype controls are visual only; they do not call YourMove routes or save data.

## Open it locally

From the repository root, run:

```sh
python3 -m http.server 4173
```

Open `http://127.0.0.1:4173/docs/implementation/2026-10-22-desktop-shell-3-3/prototype/index.html` and resize the browser to the viewport you want to inspect. The page uses no network font request. Jost is bundled here only with its OFL notice; it is not referenced by the production app.

## Shell comparison

Add one of these query strings to `index.html`:

| State | Query string |
|---|---|
| Current shell, Opportunities | `?shell=baseline&view=jobs&theme=light` |
| Refined fixed sidebar | `?shell=refined&view=jobs&theme=light` |
| Collapsible, expanded | `?shell=collapsible&mode=expanded&view=prepare&focus=answer&theme=dark` |
| Collapsible, compact | `?shell=collapsible&mode=compact&view=prepare&focus=answer&theme=dark` |
| Adaptive recommendation | `?shell=adaptive&view=prepare&focus=answer&theme=light` |
| Adaptive compact, saved letter | `?shell=adaptive&mode=compact&view=letter&focus=letter&theme=dark` |
| Adaptive dark Documents | `?shell=adaptive&view=documents&theme=dark` |

The adaptive direction selects a compact rail at widths up to 920 CSS pixels. The top-bar toggle can override that choice for the current page. The prototype does not persist this choice.

## Typography comparison

Use the same type specimen and copy for each configuration:

| Configuration | Query string |
|---|---|
| System control | `?shell=adaptive&view=type&type=system&theme=light` |
| Selective Jost, Semibold | `?shell=adaptive&view=type&type=selective&weight=600&theme=light` |
| Selective Jost, Bold | `?shell=adaptive&view=type&type=selective&weight=700&theme=dark` |
| Expanded Jost, Semibold | `?shell=adaptive&view=type&type=expanded&weight=600&theme=light` |
| Expanded Jost, Bold | `?shell=adaptive&view=type&type=expanded&weight=700&theme=dark` |

The specimen includes 11, 12, 13, and 14 px samples, a long employer question, an answer, a paragraph of working prose, and Jost weights 300/400/500/600/700. Use `view=prepare` and `view=letter` to inspect those same Jost modes against realistic work surfaces.

## Evidence status

The Browser service was unavailable in the design session, so no new screenshots or viewport instrumentation were produced. The page is ready for a visual pass when a browser is available. Existing 3.2 Prepare screenshots are linked from the design review as historical baseline context; they do not represent the shell alternatives.
