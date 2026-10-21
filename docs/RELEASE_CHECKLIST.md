# Before announcing YourMove v0.1 broadly

This is a short maintainer checklist, not a commitment that the work below is already finished. The documentation PR was drafted against public `main`, which may lag active local development. **Reconcile every claim with the actual v0.1 release commit.**

## Minimum release gate

- [ ] Pick the exact release commit and decide whether this is a **source preview** or an **installable desktop preview**. Do not imply an installer exists when it does not.
- [ ] Walk through the real job seeker path from a clean, isolated data directory: add job → inspect/prepare → choose resume → open employer form → return without losing work → record only a genuinely submitted application.
- [ ] Verify both entry points against that same commit: `go run .` and macOS `go run -tags desktop .`, if native mode is advertised.
- [ ] Confirm the Wails native build can actually open, save, restart, and retain data; describe any packaging/signing limitations explicitly.
- [ ] Confirm experimental browser automation works on supported employer forms without accidental submission or unwanted repeated document transfer. Do not generalize platform support beyond observed evidence.
- [ ] Verify test/qualification harnesses cannot write synthetic profiles or resume selections into normal user storage; test in a separate `YOURMOVE_DATA_DIR`.
- [ ] If newer local features (cover letters, question preparation, expanded autofill/continuation) land in the release, update the README with **verified** behavior and limits. If not, do not advertise them as shipped.
- [ ] Capture redacted/synthetic screenshots from the release build and replace the README's three visible image placeholders using `docs/VISUALS.md`.
- [ ] Provide appropriate project licensing information before describing the repository as “open source.” As of this documentation draft, the public root had no LICENSE file.
- [ ] Verify that public GitHub Issues and both feedback templates work. Consider a lower-friction feedback form later if non-technical testers struggle with GitHub.
- [ ] Create the v0.1 GitHub release/tag and attach tested downloads **only** when such artifacts exist. If distributing source only, say so in launch copy.

## What not to solve in this PR

No Go code, Go dependencies, Wails configuration, automation behavior, data migration, packaging, tests, or release binary changes. This branch should be safe to merge or cherry-pick alongside ongoing implementation.

### Bring into the working branch

The PR is based on `main` and introduces only new documentation and issue-template paths. Merge the PR normally if the working branch can consume `main` safely. Otherwise cherry-pick its single documentation commit:

```sh
git fetch origin docs/v0.1-public-launch
git cherry-pick origin/docs/v0.1-public-launch
```

Check the resulting diff and update product claims as needed against the **actual** release branch before publishing.
