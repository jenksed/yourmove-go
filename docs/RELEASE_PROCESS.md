# YourMove release process

The operational runbook for preparing, accepting, and distributing YourMove. The [release policy](RELEASE_POLICY.md) determines whether a candidate may ship; the [roadmap](ROADMAP.md) sets proposed review dates. macOS-specific details are in [release/macos/README.md](../release/macos/README.md).

## 1. Identify and freeze the candidate

Choose the intended commit, version, build number, platform, supported employer workflows, and distribution audience. Start from a clean checkout. Before an external candidate, increment `CFBundleVersion` in `release/macos/Info.plist` (the initial public version is `0.1.0`). Review untracked source, assets, credentials, resumes, and local databases before committing; none belongs in the distributed source unless it is an intentional sanitized fixture.

The initial build matrix is **Apple Silicon (arm64), macOS 26+**. Intel Macs, older macOS, Windows, and Linux desktop builds are not qualified. A supported installed Chromium browser is required for employer-form operations, but not local organization. AI connection is optional.

## 2. Prepare the local candidate

On a supported Mac with the pinned Go toolchain and Xcode Command Line Tools:

```sh
./release/macos/doctor.sh
./release/macos/prepare.sh
```

`doctor.sh` checks system/compiler requirements, icon, plist, and package scripts. `prepare.sh` requires a **clean Git working tree**, runs both Go test suites with isolated data, builds the Wails `desktop,production` executable through `build.sh`, signs the complete bundle ad hoc, checks metadata/hash integrity, packages the application/source ZIPs, then extracts and verifies the ZIP. It does not access the user's normal applicant workspace or publish anything.

Default output is a newly created directory under `~/Projects/yourmove-builds/`. An absolute unused output directory can be supplied as the one argument to `prepare.sh`.

The output includes:

- `candidate/YourMove.app`, `candidate/source-manifest.json`, toolchain and build details.
- `YourMove-<version>-macOS-arm64-unnotarized.zip`.
- `YourMove-<version>-source.zip`.
- `source-commit.txt` and `SHA256SUMS`.

Verify archived contents from the output directory:

```sh
shasum -a 256 -c SHA256SUMS
```

**These are local, ad-hoc-signed artifacts, not public notarized installers.** Source and binary should be associated by Git commit and manifest, not by assuming timestamped signed binaries are bit-for-bit reproducible.

## 3. Record candidate acceptance

Copy [the macOS acceptance template](../release/macos/ACCEPTANCE.md) into a candidate-specific, sanitized record. Include exact source commit, app version/build, ZIP SHA-256, OS/architecture, installed Chrome version, tester/date, observed results, limitations, and failed paths. Never commit private resumes, employer values, cookies, access tokens, or unredacted screenshots.

Required scope includes local native launch, persistence after restart, correct resume/document identity and export bytes, saved letters, a controlled browser handoff that preserves manual values, and no automatic employer submission. Verify failed/absent AI or browser dependencies without losing local work. Existing native tests against a copied workspace are useful **local evidence**, not a substitute for synthetic acceptance of the frozen candidate.

For v0.1.0, use at least two distinct application-preparation journeys, with exact provider/control scope and manual fallbacks recorded. This sample is not proof of universal ATS support.

## 4. Evaluate release gates

Use [the release policy](RELEASE_POLICY.md): frozen candidate, data integrity, applicant authority/privacy, core workflow, distribution, and recoverability. Record PASS, RESTRICTED, or BLOCKED, with evidence and exact exclusions.

Update [`release/decisions/v0.1.0.md`](../release/decisions/v0.1.0.md) for the initial beta; use a new version-specific decision file afterward. A proposed date or green CI result never grants release authorization on its own.

## 5. Choose distribution deliberately

### Controlled technical alpha — ad-hoc-signed ZIP

May be offered to a small explicitly informed technical audience **only after acceptance of the downloaded ZIP**, accurate warning/installation guidance, and an owner decision. Gatekeeper may block first launch; do not tell users to disable Gatekeeper globally or strip quarantine attributes. Do not describe this as a normal, frictionless Mac installer.

### Normal public macOS download — Developer ID/notarized ZIP

Provision a valid Developer ID Application identity and a `notarytool` keychain profile. Build a fresh frozen candidate, explicitly authorize signing and upload to Apple, and run:

```sh
export YOURMOVE_SIGN_IDENTITY='Developer ID Application: YOUR NAME (TEAMID)'
export YOURMOVE_NOTARY_PROFILE='yourmove-notary'
bash release/macos/distribute.sh /absolute/path/to/candidate
```

`distribute.sh` signs with hardened runtime and timestamp, submits to Apple's notary service, requires accepted notarization, staples/validates, runs Gatekeeper assessment, and packages the final ZIP. **It does not publish.** Test a *browser-downloaded* final ZIP on a clean account/Mac with normal security controls; copying a local build does not exercise downloaded quarantine behavior.

If signing or notarization fails, investigate the reason, correct it, and create a fresh candidate. Do not work around failed security checks.

## 6. Approve, tag, and publish

After the applicable gates pass and a human owner records SHIP or SHIP RESTRICTED:

- Create a version tag pointing at the exact accepted source commit.
- Publish the accepted binary, corresponding source, and checksums through GitHub Releases or the chosen download channel.
- Include [version-specific release notes](releases/0.1.0-macos.md), supported hardware/OS, browser prerequisite, installation guidance, and known exclusions.
- Meet GPL-3.0-only corresponding-source and notice obligations; third-party dependency licenses remain applicable.
- Verify the published URLs and the **downloaded** artifact checksum, extraction, and actual launch.

No prepare/distribute script or CI job publishes on its own.

## 7. Post-release / next version

Collect privacy-safe, voluntary tester feedback. Prioritize data integrity and blocked application paths over new feature breadth. Before any upgrade release, test retention of the existing workspace, resume associations, saved answers, letters, and completed application history. Record the release decision and evidence separately for each distributed artifact.

**Failure policy:** A source mismatch, failed test, invalid signature, corrupted archive, data-loss risk, unauthorized employer action, or unqualified advertised install blocks the affected candidate. Fix, rebuild, and requalify; never mark a failed gate passed by deleting evidence.
