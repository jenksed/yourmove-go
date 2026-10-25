![YourMove — Less repetition. More of what matters.](docs/images/yourmove-hero.png)

YourMove keeps job opportunities, resumes, application preparation, and submission records in one local workspace. It reduces repeated preparation while leaving every review and submission decision with the applicant.

## What it does

- Save opportunities, organize priorities, and retrieve job details from supported posting sources.
- Store PDF and DOCX resumes, choose a resume for each job, and track versions.
- Prepare and edit cover letters and supported application answers using approved candidate information and optional AI assistance.
- Inspect selected employer forms and fill supported controls only after an explicit applicant action.
- Record an application only after the applicant confirms they submitted it on the employer's site.

**YourMove never submits an employer application for you.** Review and submit on the employer website yourself.

## Downloads and macOS status

**A branded macOS application can now be built and run locally, but there is no published or notarized public download yet.** See [GitHub Releases](https://github.com/jenksed/yourmove-go/releases) for actual published artifacts; a successful CI build is not a release.

The first desktop target is **Apple Silicon (arm64), macOS 26 or later**. The local `YourMove.app` candidate has been built, ad-hoc signed, and exercised on macOS 26.5.2 using an isolated copy of an existing workspace. Both Go test suites and the complete bundle packaging verification passed; [the merged packaging CI run](https://github.com/jenksed/yourmove-go/actions/runs/39908694684) also passed. These results do not qualify an ordinary browser-downloaded installation, every employer site, or an advertised AI provider.

| Platform | Local browser interface (`go run .`) | Native desktop release |
| --- | --- | --- |
| macOS Apple Silicon, macOS 26+ | Tested | Local packaged candidate verified; public distribution pending |
| Older macOS or Intel Mac | Not qualified | Not qualified |
| Windows | Not tested; may fail | Not implemented |
| Linux | Not tested; may fail | Not implemented |

For an ordinary low-friction public Mac download, Developer ID signing, Apple notarization, and downloaded-install acceptance are still required. An ad-hoc-signed development ZIP is **not** equivalent to a notarized installer and may trigger Gatekeeper restrictions.

### Build the macOS candidate from source

On a supported Mac with Git, Go 1.27.2, Python 3, and Xcode Command Line Tools installed:

```sh
git clone https://github.com/jenksed/yourmove-go.git
cd yourmove-go
./release/macos/doctor.sh
./release/macos/prepare.sh
```

The preparation command tests, builds, verifies, and packages an ad-hoc-signed `.app`, an application ZIP, matching source ZIP, and checksums under `~/Projects/yourmove-builds`. **It does not install, notarize, upload, or publish anything.** The Go toolchain is needed to build from source, not to run the compiled `.app`.

For the local browser interface instead, run `go run .` and visit `http://127.0.0.1:3000`. See [source-run instructions](docs/TRY_IT.md). A compatible installed Chrome/Chromium is required for assisted employer-form operations, not for basic organization.

## Release plan

YourMove's initial release program targets an invited macOS beta for **v0.1.0**, a stabilization release **v0.1.1**, and a repeat-application improvement release **v0.2.0**. Proposed dates are planning checkpoints, not availability promises. A release may ship earlier when its evidence gates pass; failures require a hold or narrower qualified scope.

Read the [roadmap](docs/ROADMAP.md), [release policy](docs/RELEASE_POLICY.md), [current readiness](docs/RELEASE_CHECKLIST.md), [release process](docs/RELEASE_PROCESS.md), and [macOS v0.1.0 notes](docs/releases/0.1.0-macos.md).

## Data and network activity

Your records and imported resumes are stored on your computer. On macOS, the default data directory is `~/Library/Application Support/YourMove`; `YOURMOVE_DATA_DIR` can select another **absolute** directory for isolated testing. Back up the complete workspace, including the database and `resumes/`, while YourMove is closed.

Fetching a posting, calling a configured writing provider, and explicitly filling an employer form are separate network actions. When using an external AI provider, relevant selected applicant/job information may be transmitted after explicit authorization. Active employer-browser sessions do not persist across application restarts.

## Development and feedback

See [Run YourMove locally](docs/TRY_IT.md) for per-platform setup, isolated data directories, troubleshooting, and test commands. The [codebase guide](docs/CODEBASE_GUIDE.md) maps startup, routes, persistence, browser filling, and common changes.

[Report a bug](https://github.com/jenksed/yourmove-go/issues/new?template=bug-report.md) · [Share experience feedback](https://github.com/jenksed/yourmove-go/issues/new?template=experience-feedback.md) · [Feedback and privacy guidance](docs/FEEDBACK.md)

Do not post resumes, application answers, credentials, cookies, or unredacted employer forms in public issues.

## Support independent development

YourMove is independently developed and open source. If the project is useful to you or you want to support the work, you can [contribute through Ko-fi](https://ko-fi.com/yourmove). Contributions are optional and do not affect access to the software or influence its development decisions.

## License

Copyright (C) 2026 Joshua Jenks. YourMove's original source code is licensed under the [GNU General Public License version 3 only](LICENSE) (`GPL-3.0-only`). Redistribution of covered modified versions must comply with that license. Third-party dependencies and assets retain their applicable licenses.
