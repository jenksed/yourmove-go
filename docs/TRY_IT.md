# Try YourMove locally

This is the **source-run preview** guide. It is not an installer guide, and it does not claim that a packaged v0.1 build exists.

[Back to README](../README.md) · [Give feedback](FEEDBACK.md)

## Choose your mode

Both modes run the same Go application and read the same user data. Use one at a time for an uncomplicated first trial.

### Local browser (the easiest development path)

1. Install Git and the Go version required by `go.mod` (currently Go 1.27.2).
2. Run the following from a terminal:

   ```sh
   git clone https://github.com/jenksed/yourmove-go.git
   cd yourmove-go
   go run .
   ```

3. Visit `http://127.0.0.1:3000` in a browser on the **same computer**.
4. Stop YourMove with Ctrl+C when done.

This address is a loopback server on your computer, not a publicly accessible web app. If port 3000 is already in use, this preview may fail to start; check the terminal rather than assuming the app has launched.

### Wails native window (macOS development mode)

1. Install the Go version required by `go.mod` and the Xcode Command Line Tools.
2. From the repository root, run:

   ```sh
   go run -tags desktop .
   ```

3. The Wails v3 WebView should present the same YourMove workspace.

Wails v3 is still a beta dependency in this repository. This mode has been developed for macOS and needs release-build qualification; there is no signed, notarized, or packaged app offered here. Other operating systems should not be assumed supported.

## Protect your normal data when experimenting

By default, YourMove stores data under:

- **macOS:** `~/Library/Application Support/YourMove`
- **Other systems:** the OS user config directory, in a `YourMove` folder.

You can explicitly choose a *new, absolute* directory for a throwaway trial. For example on macOS:

```sh
mkdir -p /tmp/yourmove-preview
YOURMOVE_DATA_DIR=/tmp/yourmove-preview go run .
```

Do not point a test run at an existing YourMove data directory. A first run may import an old `jobs.db` from the repository working directory into a new data directory when one exists. Use a clean checkout if you want a completely fresh start. Do not share the local database or uploaded document directory when filing bugs.

## A useful 10-minute test

1. **Save one opportunity.** Use a real job-description link you are considering, or a public job link for a non-submission trial.
2. **Open its workspace.** Check the title, company, description, and source-status note. If something is wrong, edit it and save.
3. **Add a resume.** Go to Documents, import a PDF or DOCX, and select it for the job. For privacy, a test resume is fine.
4. **Review Prepare.** Identify the next action and what the app is offering to do. If inspection or browser handoff is available, review the stated scope before using it.
5. **Visit the employer form if you want.** Review everything in the real employer UI. Do **not** click Submit as part of a test. Only record a completed application in YourMove after you have actually submitted one yourself.

You can provide useful feedback after any step. Unsupported posting sources and incomplete form inspection are expected limitations, not proof that you did anything incorrectly.

## What would be most valuable to report?

Was the next action obvious? Did the right resume remain selected? Did opening the employer application preserve your work? Did a source or form behave differently from what YourMove suggested? What took longer than it should?

[Send experience feedback](https://github.com/jenksed/yourmove-go/issues/new?template=experience-feedback.md) or [report a bug](https://github.com/jenksed/yourmove-go/issues/new?template=bug-report.md). Remove all personal or employer application data from logs and screenshots first.
