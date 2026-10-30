---
name: frontend-migration
description: Migrate a specific YourMove Go-rendered route or interaction to React, strict TypeScript, and Vite inside Wails 3. Use for screen-by-screen frontend replacement; not for unrelated Go changes.
---

# Frontend migration — YourMove Go

Migrate **one working screen or workflow**, not the application all at once. `AGENTS.md` owns architecture and authority rules; this skill covers migration-specific mechanics.

## Inspect only the affected view

- Find the existing `templates/` page, relevant `static/ui.js` behavior, route in `internal/app/app_http.go`, handler and actual persisted/effect path. Read the relevant interaction/composition rules in `docs/design/RULES.md` and `DESIGN_SYSTEM.md`; do not build a new design system.
- Inventory important states and interactions *on that screen*: loading, empty, saved, dirty, failure/retry, navigation, native dialog or consent, as applicable. Preserve useful existing behavior rather than cloning every DOM detail.
- Check the protected HTTP handler before choosing transport. Reuse it when suitable; introduce a narrow JSON contract or Wails binding only if actual React behavior requires it. Don't assume the Next repository's generated binding architecture exists here.

## Build a true Wails slice

- Put feature UI/API logic in `frontend/src/features/<workflow>/`. Use strict TypeScript and one Vite app entry. Extract `shared/` only after a second real consumer.
- Read authoritative values from Go and send decisions back to Go. Keep React state unsaved/disposable; don't duplicate stage/consent/validation policy or persist applicant records in browser storage.
- Embed built frontend output into the **existing** Wails/Go executable and account for its asset handler, origin protection and navigation behavior. Retain legacy route handling and the unaffected local-browser mode. Production must not rely on Vite's dev server.
- Preserve exact records, native posting/file-dialog behavior, error recovery, focus, and practical narrow-window usability where the migrated view uses them.

## Accept the smallest complete route

Demonstrate the migrated screen in a **real Wails window**: a representative Go-backed action, truthful failure/recovery, correct navigation to one adjacent legacy page, and the affected browser-hosted path. For a saved result, confirm it after a full same-executable/same-synthetic-workspace restart. Use `native-qualification` for the observation procedure and `feature-slice` when the change spans the application workflow. Use `durable-state-integrity` only if the underlying writer, associations, schema, or file lifecycle changes.

Stop at the assigned route; do not begin the next screen or a generalized component library.
