---
name: native-qualification
description: Establish what actually works in the macOS Wails 3 window, including native interactions and same-executable/workspace restart when claimed. Do not use for documentation-only or Go-only work.
---

# Native qualification — YourMove Go

A test or browser preview is not a native GUI observation. Qualify **only the user behavior claimed by the current assignment**.

## Preflight

- Record branch, HEAD, dirty changes, build source, executable path/hash and host. Inspect already running apps/ports/workspace ownership; do not stop another process or start a second writer.
- Use an expressly isolated synthetic `YOURMOVE_DATA_DIR`, never a real applicant workspace. Read the relevant setup section of `docs/TRY_IT.md` only when needed; note legacy database import risk when launching from a repository directory.
- Available methods include `go run -tags desktop .` on supported macOS or `scripts/qualify-native-workflow.sh --launch`. **Inspect the script and its fixture before use**: it creates a new qualification workspace and may not exercise the changed feature. A new run is not a same-workspace restart.

## Observe the actual application

1. Perform the assigned action through mouse/keyboard/native GUI in Wails, not an independent HTTP client. Confirm the displayed result reflects Go state.
2. Exercise one relevant refusal, cancel, or unsaved-state recovery if it matters to the change. Inspect native file dialogs, focus, or layout only if affected.
3. For durability claims, record saved identities/values, fully quit and confirm exit, then reopen the **same executable/hash** with the **same absolute synthetic workspace** without reseeding; inspect those values/associations in the native UI. Unsaved drafts and employer sessions are not expected to survive.

Report exactly what was **agent-observed**, **owner-reported**, or **NOT VERIFIED**; include revision, binary identity, workspace, interactions, and results. If the native window cannot be observed, provide a small owner walkthrough instead of claiming a pass. Development/ad-hoc signing does not qualify distribution.
