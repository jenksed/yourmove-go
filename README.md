![YourMove wordmark](static/yourmove-wordmark-primary.svg)

# YourMove
**The job search is enough work already. YourMove helps you keep the rest under control.**

Save opportunities, organize resumes, prepare applications, and keep track of what you actually submitted. YourMove is a local-first job application workspace built for people doing the work of finding their next job.

**Early access · v0.1 release candidate.** This is a work in progress, not a finished job-search service. The current public repository is a source-based preview; packaged downloads have not been released.

[Try it locally](#try-yourmove) · [Tell us what got in your way](docs/FEEDBACK.md) · [Report a bug](https://github.com/jenksed/yourmove-go/issues/new?template=bug-report.md)

<!-- VISUAL SLOT 01: Replace this line with a REAL captured screenshot of the YourMove opportunities screen, not an AI-rendered UI. Suggested path: docs/assets/opportunities.png. See docs/VISUALS.md. -->
> **[Product screenshot to add]** The actual Opportunities workspace, with fictional job entries and no personal information.

## A better place to manage the application grind

You probably have jobs scattered across tabs, notes, spreadsheets, downloads, and your inbox. YourMove puts the preparation and tracking in one workspace so the next action is visible.

- **Keep opportunities together.** Add a posting URL or import a list of URLs, set priorities, and track where you are.
- **Review the job details.** Retrieve available metadata from supported posting sources, edit the result, and keep your own corrections.
- **Choose the resume you mean to send.** Maintain PDF and DOCX resumes locally and select one for each opportunity.
- **Prepare before visiting the employer form.** Inspect available application requirements and use experimental browser-assisted handoff where supported. Coverage varies by employer and build.
- **Record real applications.** Track submissions you have confirmed, instead of treating a button click or a browser action as proof you applied.

YourMove does **not** autonomously submit job applications. You remain responsible for reviewing the employer form and pressing Submit.

## One workspace. Two ways to open it.

The same Go application has a browser interface and an experimental native desktop host built with [Wails](https://wails.io/). This is **not** two different products and **not** a hosted website.

| | Desktop mode | Local browser mode |
| --- | --- | --- |
| Experience | YourMove in a native window | YourMove in your regular browser |
| UI | Wails v3 WebView hosting the Go routes | Go HTTP server at `http://127.0.0.1:3000` |
| Data | Local SQLite database and documents | The same local database and documents |
| Current status | macOS development preview; packaging not yet qualified | Source-run local preview |

You can try browser mode without using the Wails window. The web address is local to the computer running YourMove; it is **not** an online account or cloud dashboard.

<!-- VISUAL SLOT 02: Add a side-by-side composite of two REAL captures of the same YourMove job workspace: Wails window + local browser. Do not generate fictional UI. See docs/VISUALS.md. -->
> **[Desktop + browser comparison to add]** Two real captures that demonstrate the shared interface.

## Try YourMove

For now, this preview is for people comfortable running a project from source. An installer and a simpler download flow are still to come.

**Prerequisites:** Go 1.27.2 (as specified in `go.mod`) and Git. macOS is the primary desktop testing target; other operating systems are not yet qualified for a v0.1 release. Browser-assisted employer interactions may additionally need a locally installed Chromium-compatible browser.

**Option A — open in your web browser**

```sh
git clone https://github.com/jenksed/yourmove-go.git
cd yourmove-go
go run .
```

Open **http://127.0.0.1:3000** on that same computer. Stop the server with Ctrl+C.

**Option B — native desktop window on macOS (Wails)**

You will need Xcode Command Line Tools and a working Go development environment for this Wails v3 beta build. From the repository directory:

```sh
go run -tags desktop .
```

A YourMove window should open. This is a **development run**, not an installed or signed macOS application.

For a safe first run, a complete walkthrough, and local data details, see [Try the preview](docs/TRY_IT.md).

## What's local, and what reaches an employer?

YourMove stores its job records in SQLite and keeps imported resume files on the machine running the application. On macOS, the default app-data directory is `~/Library/Application Support/YourMove`. Browser and desktop modes use the same storage location.

Saving and preparing a job is not the same as submitting to an employer. Retrieving a public posting contacts the posting source. A separate, explicitly authorized resume handoff can interact with the employer's form. Check what the interface says it will transfer before taking that action; application-form data and employer sites are outside YourMove's control.

This preview has limitations: some posting sources are unsupported; application questions can change, appear on later steps, or be inaccessible to inspection; and experimental browser interactions may need manual correction. Always review the actual employer form.

## Help us make v0.1 genuinely useful

We want feedback from people who are **actually applying for jobs**, not just developers reviewing the code.

A useful first test is simple: save a real opportunity, select a resume, review the preparation screen, and tell us where the workflow feels confusing, slow, or unreliable. You do **not** need to submit an application to give useful feedback.

**[Tell us how the experience went](https://github.com/jenksed/yourmove-go/issues/new?template=experience-feedback.md)** · [Report a problem](https://github.com/jenksed/yourmove-go/issues/new?template=bug-report.md) · [See all feedback](https://github.com/jenksed/yourmove-go/issues)

GitHub requires an account to open an issue. Please **do not post resumes, personal contact details, application answers, session cookies, or unredacted employer forms** in public reports. [Feedback guide and test tasks](docs/FEEDBACK.md).

<!-- VISUAL SLOT 03: Add branded feedback invitation graphic with the existing YourMove logo, without invented quotes/testimonials. See docs/VISUALS.md. -->
> **[Feedback graphic to add]** A restrained, branded invitation: “Try the next move. Tell us what gets in the way.”

## Built for the person applying

YourMove is written in Go, uses SQLite for local records, serves its own HTML interface, and uses Wails v3 as an optional desktop host. The experimental employer-form integration uses Chromium/CDP. The goal is a practical assistant for the application process, not an agent that makes employment decisions or submits applications for you.

This is a rapidly changing early build. [Known release gates](docs/RELEASE_CHECKLIST.md) document what must be checked before a broad v0.1 announcement.

**Your search. Your decisions. YourMove.**
