# macOS public beta candidate

This recipe prepares a local candidate. Public distribution is gated on signing,
notarization, and the manual acceptance record below. It does not publish anything.
The initial matrix is **Apple Silicon, macOS 26 or later**. Only macOS 26 has
been built and locally exercised; older macOS and Intel support are deferred.
The bundle identity is `com.yourmove.go`, short version `0.1.0`, build `1`.
Increment the build in `Info.plist` for every external candidate.

## Build and freeze

On macOS with Go matching `go.mod`, Apple Command Line Tools, Python 3 and the
pinned modules already cached:

```sh
bash release/macos/build.sh /private/tmp/yourmove-beta-candidate-001
```

Output must be new and outside the repository. The build uses `desktop,production`,
embedded assets, arm64 and an explicit macOS 26.0 deployment target. It records
toolchain/module metadata, actual working-tree hashes (including accepted
uncommitted files), Info.plist and executable hashes. Source changes during the
build fail the manifest comparison. The candidate has a local ad-hoc signature;
it is **not the artifact to send testers**. Signed/timestamped artifacts need not
be byte-for-byte reproducible; the source and artifact manifests identify them.

No Go toolchain, repository, developer Chromium cache or Python is required on
the tester's Mac. The app needs a supported installed Chromium browser for browser
operations. Explicitly qualify an installed Google Chrome path on the clean Mac;
do not treat a developer Playwright cache as proof that this dependency is met.
Basic local document management should remain usable without browser or AI setup.

## Signing and notarization

An owner must provision a **Developer ID Application** signing identity and an
existing `notarytool` keychain profile. Do not put passwords, private keys or API
credentials in this repository or shell history. Once signing and upload to
Apple have been authorized, set the identity/profile names and run:

```sh
export YOURMOVE_SIGN_IDENTITY='Developer ID Application: YOUR ORGANIZATION (TEAMID)'
export YOURMOVE_NOTARY_PROFILE='yourmove-notary'
bash release/macos/distribute.sh /private/tmp/yourmove-beta-candidate-001
```

The script checks the frozen local hashes, signs with hardened runtime and a
secure timestamp, verifies the bundle, uploads a ZIP to Apple, requires an
Accepted result, staples/validates the ticket, assesses Gatekeeper, and creates
the final ZIP and release hashes. It requests no debug or relaxed runtime
entitlements. It never publishes the artifact. A signing/notarization failure
must be investigated rather than bypassed with quarantine removal or disabled
Gatekeeper. A failed signing attempt requires a fresh candidate build before
retrying because the frozen local signature hash has changed.

References: Apple's [notarization requirements](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution)
and [Developer ID distribution](https://developer.apple.com/developer-id/).

## Acceptance independent of native automation

Record the candidate ZIP hash, source manifest hash, bundle/build identity,
macOS version, architecture, installed Chrome version and every result. Keep
screenshots and logs with synthetic data. A human can run this entire checklist;
the temporary native automation harness is not an acceptance dependency.

1. On a separate Mac or clean account with no repository/development caches,
   download the **final notarized ZIP** through a browser, extract it and copy
   YourMove.app into Applications. Launch through Finder with normal Gatekeeper
   protections. Confirm correct identity/version and no missing-asset screen.
   This must exercise quarantine from an actual download, not just a local copy.
2. Add a synthetic unsupported opportunity, edit it, import PDF and DOCX, cancel
   import once, explicitly select a resume, quit all YourMove instances and
   relaunch. Confirm records, selected identity and exact document hashes persist.
3. Export each saved PDF/DOCX and a saved-letter PDF. Choose a destination outside
   the application workspace. Compare each file with its HTTP/saved artifact by
   SHA-256. Cancel each dialog once; the destination and stored documents must
   remain unchanged. Repeat replacement of an existing exported copy, and a
   failed destination. The workspace must remain usable after each operation.
4. Configure only the AI provider(s) intended to be advertised. Launch from
   Finder without relying on a terminal's inherited environment. Generate with
   its existing explicit consent, edit, save, reopen and export the saved letter.
   Confirm saved identity/revision and return to exact Prepare scope. An existing
   synthetic draft is sufficient for export-only testing, but does not qualify
   generation. Defer unqualified providers from beta claims.
5. Against an isolated local employer fixture, complete native resume/contact
   filling, a late saved letter and pending answers. Review exact scope before
   each explicit action. Verify local saves cause zero mutations, one resume
   upload total, preserved manual browser corrections, independent outcomes,
   uncertain-transfer suppression and zero automatic employer submissions.
   Record submission in YourMove only after the explicit human confirmation.
   Native window/CDP harness failure leaves this native gate pending; passing
   browser-rendered tests cannot substitute for it.
6. Run the controlled Chromium qualification suites as the complementary
   mutation-authority gate. Retain fixture counters/hashes and failed logs.
   Run Go and desktop-tag suites on the frozen source. Investigate repeatable
   failures; do not weaken question-save or transfer assertions.
7. Test missing Chrome and absent AI setup without auto-starting a replacement
   employer session. Confirm local documents remain accessible. Relaunch again
   and inspect SQLite integrity and saved document hashes. No real applicant or
   live employer should be used for this acceptance run.

Recruit only for the qualified OS/architecture/provider matrix. Supply the data
location, backup procedure, browser prerequisite and feedback route in the beta
onboarding material. The owner must choose that feedback route before posting;
this task sends no invitations and creates no public release.

## Backup and restore

The default macOS workspace is
`~/Library/Application Support/YourMove`. Quit **all** YourMove processes before
copying. Back up the entire directory together, including `jobs.db`, any SQLite
sidecar files and `resumes/`; a database-only copy is not a document backup.
For example, with a fresh destination on a suitable private backup volume:

```sh
app_workspace="$HOME/Library/Application Support/YourMove"
backup_dir="/Volumes/YOUR_BACKUP/YourMove-2026-10-20"
test ! -e "$backup_dir" && ditto "$app_workspace" "$backup_dir"
```

For restore, keep YourMove closed, preserve the current whole directory under a
different name, then copy the whole backup into the original location. Keep
originals until the restored app opens, SQLite integrity succeeds and document
hashes verify. Do not combine a database from one backup with documents from
another. A legacy workspace with document rows is now refused by automatic
database-only migration; restore its complete matching workspace explicitly.
Jobs-only legacy migration remains supported. Resume reimport reports damaged
or missing stored bytes as requiring recovery; it does not silently overwrite
the registry or claim that a damaged document was repaired.

## Current qualification limit

No valid code-signing identity was available during implementation. Signing,
notarization and clean-machine download/install acceptance have not run. The
native UI tool also lost its transport during export inspection; exact-byte
handler tests qualify the seam, not the actual native Save/Cancel experience.
Complete those manual gates before public beta distribution.
