# Run YourMove locally

This is the authoritative source-run quickstart for contributors and people evaluating YourMove. **Only macOS has been tested. Windows and Linux commands below are unverified and may fail.** There is no public installer, downloadable release, or qualified Windows/Linux desktop build.

[Back to README](../README.md) · [Codebase guide](CODEBASE_GUIDE.md) · [Give feedback](FEEDBACK.md)

## Requirements and supported platforms

- Git and **Go 1.27.2**, matching `go.mod`. These commands compile and run from source; they do not install a packaged application.
- A browser to visit the local interface at `http://127.0.0.1:3000`.
- To inspect or fill employer forms: a locally installed Chrome/Chromium executable. Basic opportunity, resume and draft management does not require it. The Chromium discovery path is not qualified on Windows or Linux.
- For the optional Wails desktop development host: **macOS only** with Xcode Command Line Tools. Wails v3 is a beta dependency. No Windows/Linux native entry point is included.

| Platform | Source-run local web interface | Native Wails host | Installable release |
| --- | --- | --- | --- |
| macOS (Apple Silicon) | Tested | Development only, not fully accepted for public distribution | None published |
| Windows | **Not tested** | Not implemented | None |
| Linux | **Not tested** | Not implemented | None |

Passing a build or opening the local UI does not qualify browser-based employer automation or native file dialogs.

## Run the local browser interface

### macOS (tested)

In Terminal, using an installed Go toolchain and Git:

```sh
git clone https://github.com/jenksed/yourmove-go.git
cd yourmove-go
go run .
```

### Windows (not tested)

In PowerShell, with Git and Go installed and available on `PATH`:

```powershell
git clone https://github.com/jenksed/yourmove-go.git
cd yourmove-go
go run .
```

This is an **unverified source-run attempt**, not a Windows installer. In particular, the current Chromium locator has Unix/macOS-oriented executable checks and no qualified Windows browser-discovery path. Employer-form assistance may not work even if the HTTP interface starts.

### Linux (not tested)

In a terminal, with Git and Go installed and available on `PATH`:

```sh
git clone https://github.com/jenksed/yourmove-go.git
cd yourmove-go
go run .
```

This is an **unverified source-run attempt**, not a Linux installer. A Chrome/Chromium binary on `PATH` may be discoverable, but employer-form assistance has not been qualified.

### After starting

If startup succeeds, open `http://127.0.0.1:3000` on the **same computer**. The server listens only on loopback, not on a public interface. Stop it with Ctrl+C. If port 3000 is occupied, read the terminal error instead of stopping another YourMove process. The app does not automatically submit applications.

## Use an isolated data directory

YourMove stores `jobs.db` and imported document files under its workspace. Default locations are chosen by `internal/storage/paths.go`:

| Platform | Default workspace |
| --- | --- |
| macOS | `~/Library/Application Support/YourMove` |
| Windows (untested) | Under the user's roaming configuration directory, normally `%APPDATA%\YourMove` |
| Linux (untested) | `$XDG_CONFIG_HOME/YourMove` when set, otherwise `~/.config/YourMove` |

Set `YOURMOVE_DATA_DIR` to an **absolute** path before launch to keep test data separate from a normal workspace.

macOS or Linux (sh-compatible shell):

```sh
export YOURMOVE_DATA_DIR="$(mktemp -d)"
go run .
```

Windows PowerShell (untested):

```powershell
$env:YOURMOVE_DATA_DIR = Join-Path $env:LOCALAPPDATA 'YourMove-Source-Test'
go run .
```

The Windows example retains test data across restarts; the Unix example uses a fresh temporary directory. These examples assume you are already in the repository root. Both browser mode and macOS Wails mode use the selected path. Quit the app before backing up its workspace, and copy the **whole workspace**, including the database and `resumes/` directory. An open employer browser session will **not** survive restart.

**Legacy-workspace caution:** If the selected directory is empty and a `jobs.db` exists in the current working directory, startup may migrate that old jobs-only database. Use a clean checkout without an existing repository-root database for a completely empty evaluation; never point destructive tests at your normal workspace.

## Run the macOS Wails development host

From the repository root on macOS:

```sh
go run -tags desktop .
```

This opens the same Go application and routes in a Wails window. The `desktop` entry point is built only for Darwin. This command is a development run; it does not create a signed or notarized app. The local candidate recipe and its current limits are in [release/macos/README.md](../release/macos/README.md).

## Run the tests

```sh
go test ./...
go test -tags desktop ./...
```

The first command tests browser mode and shared application logic. The second compiles and tests the **macOS-only** desktop build path; do not run it on Windows or Linux. Both commands passed on Go 1.27.2, Darwin/arm64, at baseline commit `2345d3157eb61e8ed7633a7ad8d8d5c318b50227`. Opt-in browser qualification tests use local fixtures and extra environment guards; inspect the specific test before enabling one.

For a focused test while iterating:

```sh
go test ./... -run TestName -count=1
```

## Troubleshooting and platform limitations

- **`go` or `git` not found:** Install the required tool, restart the terminal and check `go version` / `git --version`.
- **Port 3000 already in use:** Stop the instance you started or check what else is listening before relaunching. The port is currently fixed.
- **Chrome/Chromium not found:** Employer-form operations need a local Chromium-compatible executable. You may set `YOURMOVE_CHROMIUM` to an absolute executable path; macOS is the only platform where this path has been exercised. On Windows, the current executable permission check may reject a valid `.exe`, and setting the override is not a verified fix.
- **An employer form doesn't fill:** Only selected controls/platforms are supported. Do not treat a successful posting fetch or UI startup as proof of live form support.
- **Windows/Linux issues:** These platforms have not been built or run as part of the qualification evidence. Report the OS/architecture, `go version`, exact command and sanitized error output. Do not include personal resumes or employer form data.

For the macOS candidate package and its signing/installation limitations, see [release/macos/README.md](../release/macos/README.md) and the [release checklist](RELEASE_CHECKLIST.md).

## Follow a change and debug it

Start with the source map in the [codebase guide](CODEBASE_GUIDE.md). It shows how a template control maps to an HTTP route, handler, persistence operation, and browser command.

For browser mode, keep the server terminal open to read startup and error logs, and use the browser's developer tools to inspect the local page. Reproduce data-related issues with a separate `YOURMOVE_DATA_DIR`; reproduce logic issues with a focused Go test. A managed employer browser session is in memory and does not return after the YourMove process exits.

## Try the workflow and report friction

Save an opportunity, review its imported details, add a test resume if needed, and read the Prepare workspace. If you choose to open an employer form, review it yourself. Do not submit an application as part of a test. Only record a completed application in YourMove after you have actually submitted it yourself.

[Report a bug](https://github.com/jenksed/yourmove-go/issues/new?template=bug-report.md) or [share experience feedback](https://github.com/jenksed/yourmove-go/issues/new?template=experience-feedback.md). Remove personal and employer application data from logs or screenshots before posting.
