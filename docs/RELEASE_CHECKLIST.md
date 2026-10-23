# YourMove Release Readiness

Status updated: October 23, 2026.

Initial target: YourMove 0.1.0, Apple Silicon, macOS 26+.

See [Release Process](RELEASE_PROCESS.md) and the
[macOS release notes](releases/0.1.0-macos.md).

## Completed local verification

- [x] Go 1.27.2 verified on macOS 26.5.2 / arm64.
- [x] Normal Go test suite passed locally.
- [x] Desktop-tagged Go test suite passed locally.
- [x] Production-tagged macOS application bundle built.
- [x] Local ad-hoc code-signature verification passed.
- [x] Branded application icon generated and bundled.
- [x] Copied job-hunt workspace passed SQLite integrity checks.
- [x] Copied workspace preserved opportunity and resume counts.
- [x] Maintainer reported passing native workflow and persistence
      testing against a copied workspace.
- [x] Release preparation tool completed successfully, including
      ZIP extraction, signature verification, and checksums.

These results establish a locally tested development candidate.
They do not establish public installation qualification.

## Packaging PR requirements

- [ ] Confirm the branch contains only intended release changes.
- [ ] Confirm release tooling and documentation are committed.
- [ ] Confirm CI builds and verifies the complete application bundle.
- [ ] Confirm Linux Go CI and macOS desktop CI are green.
- [ ] Review the final PR diff and merge into main.

## Distribution requirements

- [ ] Build the final candidate from the merged release commit.
- [ ] Complete the candidate-specific manual acceptance record.
- [ ] Verify actual downloaded installation behavior.
- [ ] Choose controlled unnotarized alpha or standard distribution.
- [ ] For standard distribution: Developer ID sign and notarize.
- [ ] Verify the final signed/stapled artifact where applicable.
- [ ] Publish binary ZIP, corresponding source, and checksums.
- [ ] Publish accurate install instructions and release notes.
- [ ] Verify the public download and feedback links.

## Product boundaries

- No automatic employer application submission.
- No claim of universal ATS compatibility.
- No claim of Windows/Linux or Intel Mac qualification.
- No claim that local ad-hoc signing equals Apple notarization.
- No personal applicant data in public release evidence.

The clean-machine and downloaded-install gates remain open.
