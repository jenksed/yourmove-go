# YourMove macOS packaging and distribution

**Status (October 23, 2026):** The branded Apple Silicon `.app` packaging pipeline is merged into `main` ([PR #4](https://github.com/jenksed/yourmove-go/pull/4)); its [merge CI checks](https://github.com/jenksed/yourmove-go/actions/runs/39908694684) passed. The maintainer reports successful local native testing and one-command release preparation on macOS 26.5.2/arm64. **There is no publicly qualified download or notarized installer yet.** See the [release checklist](../../docs/RELEASE_CHECKLIST.md) and [artifact decision](../decisions/v0.1.0.md).

The initial release target is Apple Silicon **arm64**, **macOS 26 or later**. Intel and older macOS require separate qualification. The bundle identifier is `com.yourmove.go`; initial version is `0.1.0` (inspect `Info.plist` for the current build number). Increment `CFBundleVersion` for each externally offered candidate.

## Normal local preparation — recommended

From a clean checkout on a supported Mac with the Go version in `go.mod`, Python 3, Xcode Command Line Tools, and the pinned Go dependencies available:

```sh
./release/macos/doctor.sh
./release/macos/prepare.sh
```

`doctor.sh` performs a read-only prerequisite check and reports whether an optional Developer ID signing certificate is available. Lack of Developer ID does **not** prevent an ad-hoc-signed local build.

`prepare.sh` refuses uncommitted source; runs `go test -count=1 ./...` and `go test -count=1 -tags desktop ./...` in an isolated test workspace; calls `build.sh`; verifies the arm64 bundle, icon, plist, signature, manifest, and checksums; archives the app and corresponding Git source; extracts and re-verifies the ZIP. It writes a new output directory under `~/Projects/yourmove-builds` by default, or accepts an unused absolute directory as its sole argument.

Its outputs include `candidate/YourMove.app`, `candidate/source-manifest.json`, build/toolchain evidence, `YourMove-<version>-macOS-arm64-unnotarized.zip`, `YourMove-<version>-source.zip`, `source-commit.txt`, and `SHA256SUMS`. It **does not notarize, upload, install, or publish** anything.

For a lower-level build only:

```sh
bash release/macos/build.sh /private/tmp/yourmove-local-candidate-001
```

The output path must be new, absolute, and outside the checkout. `build.sh` embeds templates/static assets, copies `YourMove.icns` into `Contents/Resources`, uses `desktop,production`, sets macOS 26.0 minimum deployment target, captures source/toolchain metadata, and ad-hoc signs/verifies the assembled bundle. Source-manifest comparison detects source changes during the build. Signing/timestamps mean separately signed artifacts need not be byte-identical; preserve source and binary hashes for identification.

## Running and acceptance

The compiled `.app` does not require Go, a checkout, Python, or Xcode tools on the destination Mac. It needs a compatible **installed** Chrome/Chromium executable for employer-form assistance. Do not rely on a developer's Playwright cache as proof of a clean user's browser dependency. Basic document and job management work without Chrome or AI setup.

Use [ACCEPTANCE.md](ACCEPTANCE.md) as a template for each *exact candidate*, including commit, ZIP SHA-256, Mac architecture/OS, native file import/export, local persistence, saved letter, optional AI consent/failure, controlled browser handoff, no automatic employer submission, and recovery from missing external dependencies. Perform shareable tests with synthetic data. A successful local run against a copied personal workspace is useful evidence but not publishable applicant-data test material.

A local ZIP extraction and signature verification are **not equivalent** to downloading it through a browser and launching via Finder with normal quarantine/Gatekeeper checks. Test the exact distribution artifact that will be advertised.

## Signing and notarization for ordinary public installation

A normal low-friction macOS download needs a **Developer ID Application** signing identity and a configured `notarytool` keychain profile. Do not commit credentials or write private keys/passwords into logs, scripts, or shell history. After the owner explicitly authorizes Apple signing and notarization of a fresh frozen candidate:

```sh
export YOURMOVE_SIGN_IDENTITY='Developer ID Application: YOUR NAME (TEAMID)'
export YOURMOVE_NOTARY_PROFILE='yourmove-notary'
bash release/macos/distribute.sh /absolute/path/to/prepared/output/candidate
```

`distribute.sh` checks the frozen candidate hashes, signs with hardened runtime/timestamp, uploads a ZIP to Apple's notary service, requires an Accepted result, staples and validates the ticket, assesses Gatekeeper, and produces the signed final distribution ZIP and release hashes. It does **not** publish a release. A signing/notarization failure requires investigation and a fresh candidate before retrying; do not bypass Gatekeeper to call it a pass.

Apple references: [Notarization](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution) and [Developer ID](https://developer.apple.com/developer-id/).

### Controlled unnotarized technical alpha

An ad-hoc-signed ZIP can be considered for a narrowly informed technical audience if the real downloaded-install experience is documented and accepted. First launch may be blocked by Gatekeeper. Do not describe this artifact as a notarized installer; do not instruct recipients to disable Gatekeeper globally or remove quarantine attributes. The owner must approve the audience and known limitations in a version-specific [decision record](../decisions/v0.1.0.md).

## Backup and restore

Normal macOS storage is `~/Library/Application Support/YourMove`. Quit all YourMove processes before backing up; copy the **complete directory**, including `jobs.db`, SQLite sidecars, and `resumes/`. A database-only copy is not a complete resume backup. For an isolated manual run set an absolute `YOURMOVE_DATA_DIR` prior to launching the app executable. A Finder launch does not inherit shell environment variables automatically; use a separate test account when evaluating default-path installation behavior.

Never merge a `jobs.db` backup with a different `resumes/` version. Preserve the old workspace until a restore opens and integrity/document hashes pass. Test upgrade and migration behavior before distributing 0.1.1 or later.

## Release management references

- [Release policy and gate definitions](../../docs/RELEASE_POLICY.md)
- [First three release targets](../../docs/ROADMAP.md)
- [Release process](../../docs/RELEASE_PROCESS.md)
- [Current execution checklist](../../docs/RELEASE_CHECKLIST.md)
- [v0.1.0 release notes](../../docs/releases/0.1.0-macos.md)
- [v0.1.0 decision record](../decisions/v0.1.0.md)

**Local packaging is qualified for its tested environment; public installation and the final frozen candidate still require explicit acceptance.**
