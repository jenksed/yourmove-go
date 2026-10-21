# Release status and checklist

**Status checked:** October 21, 2026, against baseline commit `2345d3157eb61e8ed7633a7ad8d8d5c318b50227`. This page describes the evidence available now; dated records in `docs/implementation/` remain historical.

## What is available

| Area | Current evidence |
| --- | --- |
| Run from source | The local browser interface runs with `go run .` on `127.0.0.1:3000`. Go 1.27.2 is required. |
| Local desktop development | The Wails host uses `go run -tags desktop .` on macOS. The desktop-tagged Go test suite passed on Darwin/arm64; this is not a clean-machine native acceptance run. |
| Packaged development candidate | `release/macos/build.sh` describes a local Apple Silicon/macOS 26 candidate with an ad-hoc signature. Its output is not a public download. |
| Public downloads | No GitHub release is published as of this check. See [GitHub Releases](https://github.com/jenksed/yourmove-go/releases) for current availability. |
| Signing and notarization | No signed, notarized public build or clean-machine download/install acceptance is established. The distribution script requires an owner-provisioned Developer ID identity and notary profile. |
| Employer-site behavior | Current browser evidence is from controlled local fixtures for selected Ashby, Greenhouse, and Lever flows. Real Ashby qualification remains unresolved; the dated Lever/Spreedly notes record markup inspection, not a successful live upload. Fixture results do not prove compatibility with a live employer account or every site. No universal ATS support is claimed. |

`go test ./...` and `go test -tags desktop ./...` passed on Go 1.27.2, Darwin/arm64, at the status commit above. These tests do not replace manual Wails acceptance, notarization, clean-machine installation, or employer-specific qualification.

## Before publishing a release

- [ ] Select and freeze the exact source commit and target platform/architecture.
- [ ] Run both Go test commands from the frozen checkout and record the results.
- [ ] Use a new isolated `YOURMOVE_DATA_DIR` for a manual local-browser walkthrough; confirm stored jobs and resumes survive restart.
- [ ] On macOS, manually qualify the Wails window, native file dialogs, persistence, and restart behavior on the stated OS/architecture.
- [ ] Run controlled browser fixtures and any intended employer-site checks separately. Describe only the exact sites, pages, controls, and actions that were verified. Never treat a successful local fixture as universal compatibility.
- [ ] Build the local candidate outside the repository and retain its source and artifact hashes.
- [ ] Before distribution, sign and notarize with owner-provisioned credentials, then verify the downloaded artifact on a clean Mac with normal Gatekeeper protections.
- [ ] Publish a GitHub release only after the artifact passes the applicable checks. Until then, describe YourMove as source-run and do not imply that a packaged download is available.
- [ ] Keep user feedback links and the warning not to post resumes, application answers, credentials, or unredacted employer forms.

The app does not submit employer applications. Release qualification must preserve the applicant's review and explicit submission decision.
