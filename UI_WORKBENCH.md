# YourMove Workbench — UI iteration

This is a **presentation-only** iteration. The existing routes, Go `html/template` rendering, Wails asset handler, POST/303 compatibility adapter, storage paths, resume picker, submission semantics and document bytes remain authoritative and unchanged. A new read-only `/applications` route lists jobs that have manual submission records.

## Adopted from the TypeScript application

- Stack 01 primary/reverse wordmark SVGs, copied from the provided `src/assets/brand` source, with no font dependency.
- The calm local-first navigation shell, small progress summary, search and stage selection, one obvious next action, progressive disclosure for risky/rare actions, and System/Light/Dark appearance selector.
- Separation of job overview, application preparation and historical submission evidence. Go does **not** inherit TypeScript automation state, fit scores, packet qualification, role recommendations or AI claims.

## UI files and rules

- `templates/partials.html`: shared shell, nav, status/priority labels. All pages use this template.
- `templates/jobs.html`: opportunities, display-only counts, local search/filter/order, collapsed add form.
- `templates/job.html`: overview/prepare/history workspace, existing forms and explicit confirmation.
- `templates/resumes.html`: resume library and current native/browser upload modes.
- `templates/applications.html`: **read-only** manually confirmed submissions.
- `static/styles.css`: responsive design tokens, theme and components.
- `static/ui.js`: progressive enhancements; server forms remain the only mutation channel. Session-only view filters, hash-addressable tabs and appearance choice are not application data.

Routes and `name=` attributes must not be changed casually: the Go handlers, Wails redirect shim and existing tests rely on them. `#prepare` and `#history` deep-link into the job workspace. Without JavaScript, the data and forms remain available and all panels render in sequence. Do not use auto-submit, fetch-based mutations, or client-side stage derivation.

## Verify locally

```sh
cd ~/Projects/yourmove-go
git apply --check "$HOME/Downloads/yourmove-workbench-ui.patch"
git apply "$HOME/Downloads/yourmove-workbench-ui.patch"
go test ./...
go test -tags desktop ./...
go run -tags desktop .
```

Test `/jobs`, `/jobs/{id}`, `/resumes`, `/applications`, job add/edit/refresh, resume assignment/import and explicit submission in both browser and Wails. Exercise search and filters, narrow view (~390px), theme switch, keyboard tabs, a JavaScript-disabled browser, native picker and persistence after restart. The view-state choices do not alter jobs or applications.

## Known limitations

- HTML and CSS are still loaded relative to the repository working directory, so this is **not** a packaged macOS app.
- The Wails `wails://localhost` body/303 adaptation is still required; do not replace it with a native 303 unless separately verified.
- This iteration doesn't include application automation, authenticated sessions or AI-generated documents.
