# v0.1 critical path implementation

This follows the [release assessment](2026-10-20-v01-release-readiness.md) and
the owner's authorization to implement its three engineering tasks. The audience
is publicly recruited beta testers. **Public release remains BLOCKED.** Local-data
fixes pass, native export is implemented with passing handler tests but awaits
actual Save/Cancel qualification, and the local production packaging path passes.
No signing identity is available; notarization and clean-machine acceptance have
not run. No commit or publication was made.

## Implemented behavior

Automatic legacy migration inspects its consistent SQLite snapshot before
installing it. A workspace with resume records is explicitly refused with
whole-workspace recovery instructions; the source database and documents stay
intact, and the owned temporary snapshot is removed. Jobs-only migration still
passes. Existing destination databases and pre-existing partial migrations retain
their previous protection.

Reimport's deduplication paths now verify stored size/hash/bytes before reporting
success. A missing or damaged stored document produces a recovery error without
creating a second registry entry or overwriting the original artifact. This is
explicit refusal, not automatic document repair.

The letter-save request bound now accommodates URL encoding of the entire
12,000-byte accepted body plus bounded form overhead. A maximum-length multibyte
letter saves to the same draft and canonical Prepare URL. A slightly oversized
body within the request bound remains recoverable in the existing error editor;
the saved revision remains unchanged. Arbitrarily oversized HTTP requests remain
bounded. Generation and transfer authority are unchanged.

Native document downloads now use Wails' Save dialog after the existing handler
has validated the selected saved document. Resume PDF/DOCX and saved-letter PDF
use exactly the same bytes and suggested filenames as browser HTTP downloads.
The native callback is available only through the trusted in-process desktop
entry point. Browser headers cannot enable it, and HEAD requests do not open a
dialog. Cancellation writes nothing. Successful export stages a private file in
the chosen destination directory and atomically renames it. Export into the
active workspace, including through a parent symlink, is refused. Save/cancel
returns through the existing allowlisted Wails navigation response; failure
provides a link back to the saved document. No transfer form, browser transport,
employer navigation or submission behavior changed.

The [macOS recipe](../../release/macos/README.md) builds an arm64 production
bundle with embedded assets, stable identity `com.yourmove.go`, version `0.1.0`,
build `1`, explicit macOS 26.0 minimum, and actual working-tree source hashes.
The initial OS/architecture scope matches the environment tested; compatibility
with older macOS and Intel remains deferred. The distribution script checks the
frozen hashes and prepares Developer ID signing, hardened runtime, notarization,
stapling, Gatekeeper assessment and final ZIP hashes. That script passed shell
syntax checks; its signing/upload stages have not executed.

## Verification and evidence

Evidence root: `/private/tmp/yourmove-v01-engineering-20261020/evidence`.

| Check | Actual result |
| --- | --- |
| Focused new migration, reimport, encoded save and export tests | PASS |
| `go test ./... -count=1` | PASS |
| `go test -tags desktop ./... -count=1` | PASS |
| `go test -race ./... -count=1` | PASS |
| Controlled contact journey | PASS: exact resume/contact values, manual corrections preserved |
| Controlled combined continuation | PASS: exact letter/answers, no contact replay or resume reupload |
| Controlled Scope-First Workbench | PASS: saved draft identity, canonical return, zero extra actions to scope, keyboard/no-JS paths |
| Controlled Question Workbench | PASS in the combined run, including the formerly timed-out no-JS revision assertion |
| `git diff --check` and shell syntax checks | PASS |
| Production local package | PASS after correcting a metadata step to support Command Line Tools without full Xcode |
| Native Save/Cancel UI | BLOCKED by Computer Use transport failure |
| Developer ID / notarization / clean-machine install | BLOCKED: zero valid code-signing identities available; external acceptance not run |

The controlled command used `YOURMOVE_QUALIFY_BROWSER=1`,
`YOURMOVE_QUALIFY_QUESTIONS=1` and isolated evidence directories, then:

```sh
go test -tags qualification . \
  -run '^TestControlled(ContactJourney|CombinedContinuation|ScopeFirstWorkbench|QuestionWorkbench)$' \
  -count=1 -v
```

It passed all four tests in 43.135 seconds. Fixture evidence showed zero
save-triggered employer mutations, one exact resume upload per journey, zero
duplicate uploads, preserved manual corrections and zero employer submissions.
Generation consent and human submission remain separate. No assertions were
weakened, and no focused-question template, style or navigation code changed.

The suites regenerated light/dark browser screenshots at 1280, 1050, 800, 700 and
390px under `scope/` and `questions/`, with continuation state images under
`states/`. The 1280px light and 390px dark saved-scope images were additionally
opened and visually inspected. This is browser evidence, not native acceptance.

The production candidate built successfully under
`/private/tmp/yourmove-v01-engineering-candidate-20261020-r2/YourMove.app`.
Its native binary was copied into a separate qualification bundle with a
storage-only launcher forcing synthetic data and an empty CWD on every launch.
The startup log was recorded, but the first UI inspection lost its tool transport;
both retry and reset returned `Transport closed`. No Save/Cancel interaction or
native screenshot was obtained. The native process was no longer present on a
subsequent exact-PID check; the cause was not established. Do not infer either a
native export pass or a demonstrated application crash from this observation.
The older Chromium/window-targeting qualification blocker is also unchanged.

The final source snapshot, including these release instructions, is packaged at
`/private/tmp/yourmove-v01-engineering-candidate-20261020-final/`.
`SHA256SUMS.local`, `source-manifest.json`, `build-info.txt` and
`deployment-target.txt` identify that local artifact. It is ad-hoc signed and
must not be distributed as the public beta.

`verification.json` and `normal-storage-verification.json` record this run's
results. All six normal-storage files were byte-for-byte unchanged. A comparison
against the accepted source baseline found no missing existing files and changes
only in the seven existing production files listed below. All other accepted
uncommitted work remains intact. No normal applicant database was opened through
the application, and no live employer or external AI provider was contacted.

## Files changed in this task

Existing production files: `storage.go`, `resumes.go`, `cover_letters.go`,
`app_http.go`, `desktop.go`, `application_handlers.go`, `attachment_downloads.go`.

New implementation/tests: `desktop_export.go`, `desktop_export_test.go`,
`release_data_test.go`.

New release artifacts: `release/macos/Info.plist`, `release/macos/build.sh`,
`release/macos/distribute.sh`, `release/macos/source_manifest.py`,
`release/macos/README.md`, and this report. The original assessment is preserved
as historical evidence.

## Smallest remaining release actions

1. A human or restored native UI connection must exercise Save/Cancel for all
   three document formats and compare exported hashes. Use the isolated data;
   do not repair the old Chromium harness as a prerequisite for this check.
2. Provision a Developer ID Application identity and an existing notarytool
   profile, then authorize signing/upload of a frozen candidate. `notarytool`
   and `stapler` are installed; the missing signing identity is concrete.
3. Run the documented manual full native journey and downloaded notarized
   candidate acceptance on a clean supported Mac/account with installed Chrome.
   Keep actual native employer acceptance pending until this succeeds.

Remaining Application Workbench polish and manual-letter authorship review are
deferred. They are not required to resolve these distribution and qualification
gates, and no cosmetic Workbench changes were started.
