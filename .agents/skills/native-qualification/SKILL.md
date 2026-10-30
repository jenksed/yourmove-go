---
name: native-qualification
description: Verify YourMove Go behavior in the actual macOS Wails 3 window, especially UI-to-Go writes, native controls, and persistence across full quit/relaunch using the same synthetic workspace. Use when claiming native GUI acceptance, not for a Go-only or documentation change.
---

# Native qualification — YourMove Go

Do not infer native behavior from Go tests, Chromium fixtures, a browser screenshot or a compiled desktop binary. This skill is a verification method, not authorization to access an applicant workspace, merge, or distribute.

## Preflight

Identify the source commit and dirty changes (`git rev-parse HEAD`, `git status --short`), executable/build provenance, and the owner of any running Wails/browser/Go process. Verify the test workspace is synthetic. Never reuse the normal applicant workspace or stop another process just to obtain a clean port/lock.

Existing entry points on supported macOS:

- `go test ./...` and `go test -tags desktop ./...` — source-level evidence only.
- `YOURMOVE_DATA_DIR="$(mktemp -d)" go run -tags desktop .` — Wails development run, if isolated workspace checks permit; note the legacy-import caveat in `docs/TRY_IT.md`.
- `bash scripts/qualify-native-workflow.sh --launch` — existing freshly built, ad-hoc-signed *development qualification* bundle. Inspect the script and `docs/implementation/application-workflow-stabilization/NATIVE_QUALIFICATION.md` first: the script seeds a specific fixture; it may not cover a newly migrated React feature. It is not a release artifact or universal acceptance fixture.

If the existing fixture cannot exercise the new behavior, use an explicitly isolated invented-data workspace with a suitable non-destructive setup. Do not repurpose a historical proof as a new PASS. The application uses `YOURMOVE_DATA_DIR` for workspace selection, **not** YourMove Next's `--workspace` convention.

## Exercise the actual window

1. Using native mouse/keyboard/accessibility interaction, perform the scoped applicant action; capture observed UI state and the Go-saved result (optional read-only SQLite corroboration). For a frontend migration, verify it is the new React screen under Wails, not a standalone Vite browser tab.
2. Exercise one relevant error, cancel, or unsaved-edit recovery scenario. Preserve manual edits and correctly represent what saved versus what merely appeared on screen.
3. For durable-state claims, record the exact bundle/executable path, hash and `YOURMOVE_DATA_DIR`. Fully quit the app and verify the process is gone. **Relaunch that same executable with that same directory, without rerunning the fixture-creating script**, and inspect the saved values, IDs and associations in the native UI. An in-memory employer session is not expected to survive restart.
4. Check affected native dialogs, Wails redirects, focus, and practical narrow-window layout; do not impose a global screen matrix on a small slice.

Report source SHA/dirty state, command and result, executable identity, synthetic workspace, exact native actions, what persisted, any relevant refusal, and **PASS / FAIL / NOT VERIFIED** per claimed behavior. If you cannot observe the macOS GUI, request a bounded owner-operated walkthrough and label observations accordingly. Development signing is not Developer ID notarization or distribution qualification.
