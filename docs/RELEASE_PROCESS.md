# YourMove Release Process

This is the operational runbook for preparing and publishing YourMove
releases. The macOS-specific packaging details live in
[release/macos/README.md](../release/macos/README.md).

## Release principles

- Every release corresponds to an identifiable Git commit.
- Release candidates are built from clean, committed source.
- Tests and bundle verification run before distribution.
- Source archives and checksums accompany binary archives.
- Manual native acceptance supplements automated testing.
- Signing and notarization are separate from local candidate creation.
- No release tool silently publishes artifacts or accesses applicant data.
- User submission authority is never changed by packaging.

## Supported initial platform

YourMove 0.1.0 targets Apple Silicon (arm64), macOS 26 or later.

Intel Macs, older macOS versions, Windows, and Linux desktop releases
are outside the initial qualified release scope.

Chrome or another supported Chromium installation is required for
assisted employer-form operations. Basic local organization does not
require Chromium or an AI provider.

## 1. Prepare the source

Start from a clean checkout of the intended source commit.

Verify:

- No unintended or uncommitted source changes.
- The release version/build in release/macos/Info.plist is correct.
- The application icon and assets are present.
- The intended commit includes all release fixes and documentation.
- No applicant records, credentials, or private documents are tracked.

Use a fresh build number for each externally distributed candidate.

## 2. Run release preparation

From the repository root:

    ./release/macos/doctor.sh
    ./release/macos/prepare.sh

doctor.sh validates local prerequisites and packaging configuration.

prepare.sh:

1. Requires a clean Git working tree.
2. Runs normal and desktop-tagged Go tests with isolated test data.
3. Builds the production-tagged native macOS application.
4. Ad-hoc signs and verifies the application bundle.
5. Verifies bundle ID, icon, architecture, and source hashes.
6. Produces an application ZIP and matching source ZIP.
7. Extracts the ZIP and validates the preserved code signature.
8. Produces SHA-256 checksums and a source-commit record.

The output is written under ~/Projects/yourmove-builds unless an
explicit absolute output directory is supplied.

These operations do not publish, notarize, or upload anything.

## 3. Review artifacts

Each prepared candidate includes:

- candidate/YourMove.app
- YourMove-<version>-macOS-arm64-unnotarized.zip
- YourMove-<version>-source.zip
- SHA256SUMS
- source-commit.txt
- candidate/source-manifest.json
- Candidate build and toolchain metadata

From the candidate's output directory:

    shasum -a 256 -c SHA256SUMS

Preserve the release manifest and source commit alongside the
artifacts. Never describe an ad-hoc signature as Developer ID signing.

## 4. Manual acceptance

Use [release/macos/ACCEPTANCE.md](../release/macos/ACCEPTANCE.md)
as the checklist for the exact candidate being evaluated.

Verify at minimum:

- Extracted application bundle and Finder launch.
- Brand icon, bundle identity, and navigation.
- Startup without the repository or Go toolchain.
- Isolated SQLite workspace and persistence across restart.
- Resume PDF/DOCX import, selection, export, and cancellation.
- Saved cover-letter editing and PDF export.
- Optional AI behavior and explicit provider consent, if advertised.
- Supported Chromium handoff on a controlled employer fixture.
- No automatic employer application submission.
- Missing Chrome and unconfigured AI behavior.

Record the candidate SHA-256, commit, operating system, tester,
date, failures, and qualification limits. Use synthetic test data.

A passing Go test suite does not establish native UI acceptance.

## 5. Choose the distribution channel

### Local or controlled technical alpha

An ad-hoc-signed ZIP may be used for controlled technical testing,
provided recipients are explicitly told it is not notarized and
macOS may block the initial launch.

Do not instruct users to disable Gatekeeper globally or remove
download quarantine attributes.

### Normal public macOS distribution

For a normal downloaded-app installation experience:

1. Obtain a Developer ID Application signing identity.
2. Configure an authorized notarytool keychain profile.
3. Prepare a fresh candidate from the intended release commit.
4. Explicitly authorize and run release/macos/distribute.sh
   against that candidate's candidate/ directory.
5. Confirm Apple notarization was accepted.
6. Verify the stapled application and Gatekeeper assessment.
7. Test the final browser-downloaded ZIP on a clean Mac/account.
8. Publish only the accepted final ZIP and matching source.

Do not upload the unnotarized ZIP in place of the final signed ZIP.

The distribution script does not publish a GitHub Release.

## 6. Publish

Only after the applicable distribution and acceptance gates pass:

- Confirm the release version and source commit.
- Create the release tag on the intended commit.
- Publish the matching, qualified application ZIP.
- Publish the corresponding source archive.
- Include checksums and user-facing release notes.
- Document supported operating systems and dependencies.
- Provide a route for reporting issues.

YourMove's original source is licensed under GPL-3.0-only.
Binary distribution must include access to corresponding source
and the applicable license notices.

## 7. Post-release verification

Verify:

- Published download URL.
- Downloaded artifact checksum.
- Successful extraction and permitted launch path.
- App identity and version.
- Release notes and source download accessibility.
- Privacy-safe feedback channel.

Keep release evidence free of applicant information, credentials,
cookies, and unredacted employer application screenshots.

## Release failure policy

A failed test, corrupted archive, unexpected source change,
unverified signature, or rejected notarization blocks that
candidate's release.

Correct the cause, commit the fix, and prepare a new candidate.
Do not bypass security controls to turn a failure into a pass.
