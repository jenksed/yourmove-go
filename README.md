![YourMove — Less repetition. More of what matters.](docs/images/yourmove-hero.png)

YourMove keeps job opportunities, resumes, application preparation, and submission records in one local workspace. It reduces repeated preparation while leaving every review and submission decision with the applicant.

## What it does

- Save opportunities, organize priorities, and retrieve job details from supported posting sources.
- Store PDF and DOCX resumes, choose a resume for each job, and track versions.
- Prepare editable cover letters from job and resume evidence, and prepare employer answers from candidate-approved facts.
- Inspect selected employer forms and fill supported controls only after an explicit action.
- Record an application only after the applicant confirms they submitted it on the employer's site.

YourMove never submits an application for you. Review the employer form and submit it yourself.

## Support and release status

The source-run browser interface has been verified on macOS at `http://127.0.0.1:3000`; other operating systems are not currently qualified. The optional Wails desktop host is macOS-only and currently a development run. The repository includes a local Apple Silicon/macOS 26 candidate-build recipe; it does not provide a public installer. No downloadable release is published yet, and signed, notarized, clean-install acceptance is still outstanding. See the [release status and checklist](docs/RELEASE_CHECKLIST.md).

Posting metadata support covers selected Ashby, Greenhouse, Lever, and Paylocity pages. Browser form inspection and filling are experimental and depend on the exact page and controls. Controlled fixtures cover selected Ashby, Greenhouse, and Lever flows; they do not establish universal ATS compatibility.

## Data and network activity

Your records and imported resumes are stored on your computer. On macOS, the default data directory is `~/Library/Application Support/YourMove`; `YOURMOVE_DATA_DIR` can select another absolute path. Fetching a posting, using a configured cover-letter provider, and explicitly filling an employer form are separate network actions. A remote AI provider may receive the selected resume and job details when you request a draft.

## Run and contribute

From the repository root, run `go run .` and open `http://127.0.0.1:3000` on the same computer. The full source-run and macOS Wails instructions, isolated data setup, and Go test commands are in [Try YourMove locally](docs/TRY_IT.md). The [codebase guide](docs/CODEBASE_GUIDE.md) maps startup, routes, persistence, browser filling, and common changes.

[Report a bug](https://github.com/jenksed/yourmove-go/issues/new?template=bug-report.md) · [Share experience feedback](https://github.com/jenksed/yourmove-go/issues/new?template=experience-feedback.md) · [Feedback and privacy guidance](docs/FEEDBACK.md)
