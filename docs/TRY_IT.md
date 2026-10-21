# Run YourMove locally

This is the authoritative source-run quickstart for contributors and people evaluating YourMove. It does not describe an installer or a public desktop download.

[Back to README](../README.md) · [Codebase guide](CODEBASE_GUIDE.md) · [Give feedback](FEEDBACK.md)

## Requirements

- Go 1.27.2, as declared in `go.mod`, and Git.
- For the Wails desktop development run: macOS and Xcode Command Line Tools. Wails v3 is a beta dependency.
- For employer-form browser assistance: a supported local Chromium-compatible browser. Other parts of the app remain available without it.

## Run the local browser interface

From a checkout:

```sh
git clone https://github.com/jenksed/yourmove-go.git
cd yourmove-go
go run .
```

Open `http://127.0.0.1:3000` in a browser on the same computer. The server listens on loopback; it is not a hosted service. Stop it with Ctrl+C. If port 3000 is already in use, check the terminal output instead of stopping another YourMove process.

## Use an isolated data directory

By default, macOS stores data in `~/Library/Application Support/YourMove`; other systems use the OS user-config directory under `YourMove`. Set `YOURMOVE_DATA_DIR` to a new absolute path to keep a development run separate from your normal workspace:

```sh
export YOURMOVE_DATA_DIR="$(mktemp -d /tmp/yourmove-go-dev.XXXXXX)"
go run .
```

The new directory contains `jobs.db` and a `resumes/` folder. YourMove creates it with owner-only permissions. The same exported `YOURMOVE_DATA_DIR` is used by the Wails command below if you run it in this terminal. If the selected directory is empty and a legacy `jobs.db` exists in the repository's current working directory, startup may import that jobs-only database. Use a clean checkout or make sure no legacy database is present when you need a completely empty test workspace. Do not point test runs at your normal data directory.

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

The first command tests browser mode and shared application logic. The second compiles and tests the macOS desktop build path. Both commands passed on Go 1.27.2, Darwin/arm64, at baseline commit `2345d3157eb61e8ed7633a7ad8d8d5c318b50227`. Opt-in browser qualification tests use local fixtures and extra environment guards; inspect the specific test before enabling one.

For a focused test while iterating:

```sh
go test ./... -run TestName -count=1
```

## Follow a change and debug it

Start with the source map in the [codebase guide](CODEBASE_GUIDE.md). It shows how a template control maps to an HTTP route, handler, persistence operation, and browser command.

For browser mode, keep the server terminal open to read startup and error logs, and use the browser's developer tools to inspect the local page. Reproduce data-related issues with a separate `YOURMOVE_DATA_DIR`; reproduce logic issues with a focused Go test. A managed employer browser session is in memory and does not return after the YourMove process exits.

## Try the workflow and report friction

Save an opportunity, review its imported details, add a test resume if needed, and read the Prepare workspace. If you choose to open an employer form, review it yourself. Do not submit an application as part of a test. Only record a completed application in YourMove after you have actually submitted it yourself.

[Report a bug](https://github.com/jenksed/yourmove-go/issues/new?template=bug-report.md) or [share experience feedback](https://github.com/jenksed/yourmove-go/issues/new?template=experience-feedback.md). Remove personal and employer application data from logs or screenshots before posting.
