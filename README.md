![YourMove — Less repetition. More of what matters.](docs/images/yourmove-hero.png)

YourMove keeps job opportunities, resumes, application preparation, and submission records in one local workspace. It reduces repeated preparation while leaving every review and submission decision with the applicant.

## What it does

- Save opportunities, organize priorities, and retrieve job details from supported posting sources.
- Store PDF and DOCX resumes, choose a resume for each job, and track versions.
- Prepare editable cover letters from job and resume evidence, and prepare employer answers from candidate-approved facts.
- Inspect selected employer forms and fill supported controls only after an explicit action.
- Record an application only after the applicant confirms they submitted it on the employer's site.

YourMove never submits an application for you. Review the employer form and submit it yourself.

## Install and platform status

**YourMove currently runs from source; there is no public installer or downloadable release.** Install Git and Go 1.27.2 (the version declared in `go.mod`), then use a terminal in macOS or Linux, or PowerShell on Windows:

```sh
git clone https://github.com/jenksed/yourmove-go.git
cd yourmove-go
go run .
```

If the server starts successfully, open `http://127.0.0.1:3000` on the same computer. Stop it with Ctrl+C. Chrome or Chromium is required for assisted employer-form operations, but not basic local organization. Running from source requires the Go toolchain; this is **not** an installed desktop application.

| Operating system | Local browser interface (`go run .`) | Native Wails desktop |
| --- | --- | --- |
| **macOS** | **Verified** on Apple Silicon | macOS-only development run; not a qualified public installer |
| **Windows** | **Not tested**; may not build or run correctly | Not implemented |
| **Linux** | **Not tested**; may not build or run correctly | Not implemented |

Only macOS has been exercised. In particular, Chromium discovery and employer-form automation have **not** been qualified on Windows or Linux; a running local HTTP server would not establish that those features work. Posting metadata and employer-form filling have different coverage. Selected Ashby, Greenhouse, Lever and Paylocity posting metadata is supported; controlled form fixtures cover selected Ashby, Greenhouse and Lever flows, not universal ATS compatibility.

The macOS Wails host is an optional development run (`go run -tags desktop .`). A local Apple Silicon/macOS 26 candidate-build recipe exists, but no signed/notarized public build or clean-install acceptance is established. See [detailed setup for macOS, Windows and Linux](docs/TRY_IT.md), [release status](docs/RELEASE_CHECKLIST.md) and the [macOS candidate recipe](release/macos/README.md).

## Data and network activity

Your records and imported resumes are stored on your computer. On macOS, the default data directory is `~/Library/Application Support/YourMove`; `YOURMOVE_DATA_DIR` can select another absolute path. Fetching a posting, using a configured cover-letter provider, and explicitly filling an employer form are separate network actions. A remote AI provider may receive the selected resume and job details when you request a draft.

## Development and feedback

See [Run YourMove locally](docs/TRY_IT.md) for per-platform setup, isolated data directories, troubleshooting, and test commands. The [codebase guide](docs/CODEBASE_GUIDE.md) maps startup, routes, persistence, browser filling, and common changes.

[Report a bug](https://github.com/jenksed/yourmove-go/issues/new?template=bug-report.md) · [Share experience feedback](https://github.com/jenksed/yourmove-go/issues/new?template=experience-feedback.md) · [Feedback and privacy guidance](docs/FEEDBACK.md)

## License

Copyright (C) 2026 Joshua Jenks. YourMove's original source code is licensed under the [GNU General Public License version 3 only](LICENSE) (`GPL-3.0-only`). Redistribution of covered modified versions must comply with that license. Third-party dependencies and assets retain their applicable licenses.
