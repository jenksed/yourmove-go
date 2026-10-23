# Native qualification and development-harness signature recovery

October 23, 2026; macOS 26.5.2, Apple Silicon; Wails v3.0.0-beta.32, Go 1.27.2.

## Infrastructure incident

The original helper was generated interactively through terminal commands, not by an existing repository script. Its assembly was `go build -tags desktop -o /private/tmp/yourmove-workflow-native .`, creation of `/private/tmp/YourMoveWorkflow.app/Contents/MacOS`, copying the executable into that directory, and writing a minimal `Info.plist`. There was no explicit final bundle signing/strict verification step. A subsequent rebuild copied over a previously executed executable. This also allowed stale executable/signature state to be reused.

The crash report `/Users/jenksed/Library/Logs/DiagnosticReports/YourMoveWorkflow-2026-10-23-091118.ips` confirms:

- Capture: `2026-10-23 09:11:17.7192 -0400`.
- Process: `/private/tmp/YourMoveWorkflow.app/Contents/MacOS/YourMoveWorkflow`, parent `codex`.
- Bundle: `run.yourmove.workflowqualification`.
- `EXC_CRASH`, `SIGKILL (Code Signature Invalid)`.
- Termination: `CODESIGNING`, code `1`, `Taskgated Invalid Signature`.

The user additionally inspected the original linker-generated signature and found identifier `a.out`. The original overwritten executable was not retained, so that identifier was not independently rechecked against the original bytes. The omitted final bundle signing is confirmed from the actual assembly commands. The crash record alone cannot distinguish every contribution of identity, sealing and cached executable state. This was **test infrastructure failure**: native tests for that failed launch were **BLOCKED**, not passed. It establishes no production YourMove.app defect.

## Repaired repeatable harness

[qualify-native-workflow.sh](../../../scripts/qualify-native-workflow.sh) creates a new `mktemp` output root every run. It assembles `Contents/MacOS`, `Contents/Resources`, `Info.plist` and a freshly built executable before signing. All templates/static resources are embedded at build time. This bundle contains one executable and no nested frameworks, plug-ins or helper executables. It signs the executable first, then the complete enclosing bundle. Future nested executables must be individually signed inside out before the enclosing sign; recursive signing is not used.

It sets `--identifier run.yourmove.workflowqualification` explicitly and signs ad hoc for local development. `set -euo pipefail` makes any build/signature/strict-verification failure abort before launch. No executable, plist or resource is changed after signing; storage and logs live outside the bundle. It never reads or changes the release candidate, production release packaging or the normal applicant data directory.

Run from the worktree:

```sh
GOCACHE=/private/tmp/yourmove-workflow-gocache \
YOURMOVE_DATA_DIR=/private/tmp/yourmove-workflow-data \
bash scripts/qualify-native-workflow.sh --launch
```

## Final signed rerun evidence

- Source: `222956c56dea8869e95934496aa765b3934e230a` (product change `76233a6`; tracked source diff empty; only documentation/evidence untracked during build).
- Bundle: `/private/tmp/yourmove-workflow-native-FVYI5x/YourMoveWorkflow.app`.
- Data: `/private/tmp/yourmove-workflow-native-FVYI5x/workspace`.
- Log root: `/private/tmp/yourmove-workflow-native-FVYI5x/evidence`.
- Launch: `2026-10-23T13:18:45Z` / 09:18:45 EDT, PID `39077`.

Commands, in order:

```sh
codesign --force --sign - --timestamp=none \
  --identifier run.yourmove.workflowqualification "$app/Contents/MacOS/YourMoveWorkflow"
codesign --force --sign - --timestamp=none \
  --identifier run.yourmove.workflowqualification "$app"
codesign --verify --strict --verbose=4 "$app/Contents/MacOS/YourMoveWorkflow"
codesign --verify --strict --verbose=4 "$app"
codesign --display --verbose=4 "$app"
```

Both verification outputs reported `valid on disk` and `satisfies its Designated Requirement`. Display reported `Identifier=run.yourmove.workflowqualification`, `Signature=adhoc`, `TeamIdentifier=not set`. The executable SHA-256 after signing was `69725219f80793ac8b14c1990235d7a430cc4101f40ce8a6ebdf61f16522e054`. This is a development signature, not public-distribution notarization.

| Gate | Actual result |
| --- | --- |
| Explicit ad-hoc signing and strict verification | **PASS**, executable and complete bundle, before launch and again after interaction. |
| Wails launch | **PASS**, process survived startup and actual window rendered Opportunities at `wails://localhost/jobs`. |
| Dirty question → Candidate profile | **PASS**, real native interaction: dialog appeared; Stay preserved `Synthetic final signed name`; Escape closed and preserved it; Tab/Shift-Tab stayed among actions; Return activated Leave; `/candidate` rendered without a second dialog/refocus loop. |
| Dirty cover letter → Documents | **PASS**, real native mouse interaction: Stay preserved unsaved letter; retry + Leave rendered `/resumes` without a second dialog. |
| Persistence | **PASS**, read-only check of this synthetic workspace: original letter unchanged, zero live answers saved and zero submissions recorded by these navigation checks. |
| Full native employer transfer / simultaneous multiple dirty forms | **NOT VERIFIED**; covered in controlled Chromium separately, not claimed as native acceptance. |

The first accessibility snapshot immediately after Return was taken before navigation settled; the following snapshot confirmed `/candidate`. That transient automation assertion was not a signature or Wails crash.

Evidence: [harness/signature log](evidence/native-harness.log), [Wails startup log](evidence/native-launch.log), [post-interaction signature checks](evidence/native-post-interaction-signature.log), [dialog](evidence/native-dialog.png), [keyboard focus](evidence/native-keyboard.png), [Stay and preserved name](evidence/native-stay.txt), [Candidate destination](evidence/native-destination.txt), [Documents destination](evidence/native-letter-destination.txt). UI evidence was captured from the native app through the [Computer Use skill](/Users/jenksed/.codex/plugins/cache/openai-bundled/computer-use/1.0.1001501/skills/computer-use/SKILL.md), not from standalone Chromium.

## Repeatable manual acceptance

1. Run the harness above. Stop if signing/verification fails; classify launch-dependent tests **BLOCKED**. A printed signature PASS or live PID alone does not establish native interaction.
2. Open its unique bundle path only. Confirm **Synthetic native workflow** in Opportunities. Do not use an existing YourMove window. Its synthetic inspection is seeded for navigation testing; do not click employer opening, preview or filling controls, which are not bound to the loopback transfer fixture in this native harness.
3. Choose the synthetic opportunity's resume/preparation link. Under Application answers, expand **Previously observed browser questions**. Type a synthetic name without saving.
4. Click Candidate profile. Confirm exact title/body/actions. Stay must preserve the name; Escape must do the same. Repeat, cycle Tab and Shift-Tab, then activate Leave with Return. Confirm `/candidate`, no second confirmation and no question focus loop.
5. Return through Opportunities. Open the saved cover letter, type synthetic text without saving, click Documents. Test Stay and Leave. Confirm `/resumes` and unchanged saved letter on return.
6. For the remaining native gate, repeat with two dirty question editors and a dirty letter together; also qualify native file picker cancellation, VoiceOver and 200% zoom. These checks remain **NOT VERIFIED** here. Run employer filling only with the controlled loopback fixture and isolated Chromium factory, never an actual applicant or employer application.

Production release signing/notarization is unchanged. The corrected helper is not a replacement for acceptance of the separately packaged release candidate.
