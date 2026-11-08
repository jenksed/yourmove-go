# YourMove v0.2.0 Windows packaging readiness

**Disposition: Windows CI-qualified packaging candidate (13 PASS, 0 FAIL, 3 NOT RUN on a hosted Windows Server 2025 x64 runner); installation, process launch, persistence, uninstall retention and reinstall recovery are demonstrated. Wails GUI interaction, managed-browser Fill, A/B upgrade, production signing and OWNER-ACCEPTED release status remain pending.** Nothing was pushed except the `v0.2/feat-windows-packaging` branch, and nothing was published, submitted, released, or changed on the live website. Other worktrees were inspected read-only. No application/storage code, data paths, database schema, updater or applicant workspace was changed.

## A. Existing capabilities and audit

Started on the assigned clean worktree `/Users/jenksed/Projects/yourmove-go-windows-packaging`, branch `v0.2/feat-windows-packaging`, HEAD `e75f6bf389a28aff2d59e57b014399734d3690a6`, exactly the `v0.2/develop` baseline. No pre-existing dirty/untracked work. Host: macOS 26.5.2, Darwin arm64. Coverage follows the supplied assignment; no separate checklist attachment was present in the checkout.

“Verified” below names its evidence boundary; source/build evidence does not establish native Windows behavior.

| Requirement | Existing mechanism / baseline classification | Result of this assignment |
| --- | --- | --- |
| Windows app and production resources | **Implemented and verified** by source/build: Wails v3.1.0, `desktop_windows.go` → `desktop_shared.go`; `assets.go` embeds frontend/templates/static/prompts; Windows compile CI existed | Reused. Actual x64 GUI-subsystem app built; all three generated Vite files found byte-for-byte in the executable |
| Windows branding/version/icon | App name YourMove existed; Windows PE resources/installer branding **Missing** | **Implemented and verified** by PE inspection: YourMove product/Joshua Jenks publisher, 0.2.0 / 0.2.0.2, icon/version/manifest resources, asInvoker. Actual desktop appearance **unverified** |
| Installer, path, Start menu, Installed Apps | Project implementation **Missing**; pinned Wails already supplied usable NSIS templates/helpers | NSIS build/extraction **verified**; per-user fixed path, stable HKCU key and shortcut **implemented, unverified** at runtime |
| WebView2 provisioning | Wails runtime/generator existed; project installer dependency handling **Missing** | Embedded Microsoft bootstrapper, registration detection and refusal on failed setup **implemented, unverified** on Windows. Offline missing-runtime installation **Missing** |
| Production employer browser | Chrome/Chromium discovery and CDP execution **implemented, unverified** on Windows; Playwright caches are only fallback, not distribution | Reused standard Chrome paths and documented separately installed Chrome prerequisite. No developer Node/cache assumption. Bundled Firefox/Playwright **Deferred** |
| Per-user saved work | **Implemented and verified** by source/tests: `internal/storage/paths.go` uses Windows `os.UserConfigDir()/YourMove`; existing SQLite migrations/writers | Unchanged `%APPDATA%\YourMove`, separate from installation. Native storage behavior **unverified** |
| Upgrade and retention | Database migration/storage contracts existed; Windows installer-over-installer/uninstall retention **Missing** | Stable install path/key, close-app guard, downgrade guard, explicit-owned-file deletion and data retention **implemented, unverified** on Windows |
| Signing | Wails supports Authenticode/timestamps; project Windows configuration **Missing**. Trusted credential and verified publisher identity **Blocked externally** | App, embedded-uninstaller and final-installer signing/verification hooks **implemented, unverified**; candidate explicitly unsigned |
| Version/hash/provenance | AppVersion/AppBuild injection, source-manifest/checksum/source-archive conventions existed in macOS packaging: **Implemented and verified** by source | Reused those conventions. Final hashes, clean VCS revision, source manifests, tool versions and matching source archive **verified** |
| Release/download pipeline | macOS local build/distribute scripts and CI existed; Windows release assets/website mapping **Missing** | Existing Windows CI extended to package/retain private artifacts. Native CI execution **unverified**; publishing/live download **Blocked externally** pending acceptance, owner approval and actual asset URL |
| WinGet | **Missing** | Three split templates, actual installer traits and candidate SHA prepared. Official JSON schema checks pass except deliberately unresolved public URL. Publication **Blocked externally** |
| Store EXE | **Missing** | Evaluated; **Blocked externally** on account/identity/publication/acceptance, with concrete **Missing** offline-runtime and all-PE signing work described below |
| Store MSIX | Wails tooling already supports it; repository configuration **Missing** | **Deferred**; no second installer or identity created |
| Automatic Windows updates / ARM64 / Ubuntu installer / other package managers | Not needed for this release | **Deferred**; no Ubuntu installer advertised |

Storage separation protects the existing opportunities, résumés/document identities and associations, Candidate Profile, Role Families, saved answers/Answer Library, letters, application history and settings together. This is an installer design/source conclusion, not a Windows preservation acceptance result.

## B. Changes and verification

Local implementation commits:

- `2b51c21179b275548783f0598dcde754f08aafe0` — Wails/NSIS packaging, identity/resources, signing hooks, provenance, WinGet templates and existing CI extension.
- `eb48b3a2fd2c7733d36cb45c1cbf6bd5f71c3cbd` — temporary-resource cleanup, compact welcome text, documented WinGet refusal codes/browser dependency.
- `dd01eeb3e90088b45c4ad18e280bec27b9a79638` — retain every file referenced by candidate checksums in CI artifacts.
- `4085099dd9636504133d4b8dcf4fba53e4a5b90d` — apply the owner-selected **Joshua Jenks** publisher and increment the qualification build to 2.

Modified paths: `.github/workflows/ci.yml`, `.gitignore`, and `release/windows/` (`prepare.py`, `product.json`, `project.nsi`, `sign.ps1`, `qualify-ci.ps1`, installed README, runbook and three WinGet templates). This report is documentation committed after the artifact source revision; implementation is unchanged from that revision.

Observed checks:

- `WAILS3=/tmp/yourmove-packaging-tools/wails3 python3 release/windows/prepare.py /Users/jenksed/Projects/yourmove-builds/windows-0.2.0-x64-build2-joshua-jenks` — **PASS** from clean commit `4085099`; frozen pnpm install, production frontend build, Wails resource generation, production Windows cross-build, NSIS compilation, source archive/manifests/checksums. Tools: Go 1.27.3, Wails CLI 3.1.0, Node 26.3.0, pnpm 10.14.0, NSIS 3.13.
- `go test -count=1 ./...` and `go test -count=1 -tags desktop ./...`, each with its own new absolute synthetic `YOURMOVE_DATA_DIR` — **PASS**, all 14 internal packages. Executed at `2b51c21`; subsequent changes only touched packaging/CI/documentation.
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet -tags desktop .` — **PASS** at the final implementation revision.
- Final `7zz x` extraction and byte comparison — **PASS**; installer contains the exact built application, uninstall executable, license/README, Microsoft bootstrapper and NSIS plugins.
- `shasum -a 256 -c SHA256SUMS` — **PASS**, all six entries. Before/after source manifests identical; executable build info records the full source revision and `vcs.modified=false`.
- Independent PE inspection with pefile 2024.8.26 — **PASS** for app x64 machine, GUI subsystem, version/publisher, icon/manifest and asInvoker resources on app/installer/uninstaller; all three owner-produced PEs lack signature tables as expected. NSIS installer/uninstaller are the normal x86 Unicode stubs carrying an x64-only payload, not x86 application builds. Microsoft's bootstrapper has a signature table; Windows trust validation **NOT VERIFIED**.
- All three built Vite output files found unchanged in the final app — **PASS**, including JS/CSS.
- Workflow/template YAML parse and official Microsoft v1.10.0 JSON schema checks on final rendered templates — **PASS** for version/default locale; installer has exactly one expected failure: unresolved `InstallerUrl`. No fabricated URL was used, even for validation. WinGet CLI installation/detection/upgrade **NOT VERIFIED**.
- Refusal checks: relative output, pre-existing output and signing on an unsupported host — **PASS**, no candidate overwritten. Forced compiler failure using an intentionally invalid `GOFLAGS` — **PASS**, failure surfaced and temporary root `.syso` removed; subsequent clean final build succeeded.

Evidence beside the final installer: `build.log`, `build-info.txt`, `source-manifest*.json`, `candidate.json`, `pe-inspection.json`, `publisher-inspection.json`, `frontend-inspection.json`, `inspection-summary.json`, and `SHA256SUMS`. Extra inspection files are local evidence, not additions to the installer. Earlier local candidates, including build 1 with the YourMove publisher label, are superseded by the single build 2 handoff candidate below. Publisher fields were independently checked on the current app, installer, extracted uninstaller and rendered WinGet locale/Installed Apps entries; all use Joshua Jenks. The proposed package identifier, stable uninstall key and install/data paths remain unchanged.

**NOT VERIFIED before CI:** actual Windows installation or launch, Start menu/Installed Apps presentation, `/S` execution, WebView2 installation/failure recovery, Chrome discovery/Fill, dialogs/import/export/cancel, restart persistence, locked-app/downgrade refusal at runtime, upgrade/uninstall/reinstall data retention, production signing/timestamps/trust, remote CI execution, or any public download. No native or owner acceptance is claimed.

## B2. Windows CI qualification (run 42446133465)

GitHub Actions run [`42446133465`](https://github.com/jenksed/yourmove-go/actions/runs/42446133465), source `533efebe8a3aad42f7a8581bb66f92d861d968ca`, runner `Microsoft Windows Server 2025 Datacenter` on AMD64, standard hosted allocation. The Windows job built the installer natively with `release/windows/prepare.py`, then `release/windows/qualify-ci.ps1` exercised the installed artifact. The retained `YourMove-windows-x64-qualification-533efebe8a3aad42f7a8581bb66f92d861d968ca` artifact holds `evidence.json` plus installer-launch, server-cycle and upload logs. Result: **13 PASS, 0 FAIL, 3 NOT RUN**.

| Check | Result | Evidence |
| --- | --- | --- |
| artifact-sha256 | PASS | `3d787215d3a46d3dc6625b64a0370c54133c29ed3071329c262b39a4a959ba11` matches `candidate.json` |
| payload-arch-x64 | PASS | Installed `YourMove.exe` is `IMAGE_FILE_MACHINE_AMD64` (NSIS stub is x86 by design) |
| webview2-runtime | PASS | Evergreen registration present, `pv=156.0.4314.58`; installer detection path used, no bootstrap download needed |
| install-silent | PASS | `installer /S` exit 0, AMD64 payload, file version `0.2.0.2` |
| install-registration | PASS | HKCU `com.yourmove.go` key with `DisplayName=YourMove`, `DisplayVersion=0.2.0`, correct `InstallLocation`, `/S` quiet string |
| install-shortcut | PASS | Per-user Start Menu `YourMove.lnk` |
| install-per-user | PASS | HKCU key, `LOCALAPPDATA` path, `RequestExecutionLevel=user`; runner was elevated so standard-user non-elevation was **not** verified |
| installed-launch | PASS | Installed process alive after 25s (pid 4136); window visibility **not** verified on headless Server |
| backend-startup | PASS | Same-source server `GET /jobs` 200 on loopback with synthetic `YOURMOVE_DATA_DIR` |
| backend-save | PASS | Candidate profile, role family and resume bytes saved over HTTP (`POST /candidate`, `/families`, `/resumes/import` → 303) |
| backend-restart-persistence | PASS | Profile, role family and resume listing readable after server restart; `jobs.db` sha `68274b28ccdf4fab923d75673900f1fc986682a58851666c2bd5d5aa7978984b`; downloaded resume bytes identical |
| uninstall-retention | PASS | Program files and HKCU registration removed; synthetic workspace retained byte-identical |
| reinstall-recovery | PASS | Retained synthetic records reopen after reinstall |
| upgrade-preservation | NOT RUN | No second versioned installer exists; same-binary reinstall is not an upgrade test |
| browser-fill | NOT RUN | Managed Chromium Fill needs an interactive session; no automatic submission was attempted |
| gui-acceptance | NOT RUN | Native window presentation needs Windows 11 interactive acceptance |

CI defects found and fixed on this branch (packaging/CI only, no application changes): `GOPATH/bin` was missing from the Windows job PATH so `prepare.py` could not spawn `wails3`; the hosted runner exposes `pnpm` as an extensionless shim that `CreateProcess` cannot execute, so `prepare.py` resolves it (and runs it through node when needed); `Invoke-WebRequest` mishandles the application's 303 POST responses on the runner, so the save-path checks POST with `curl.exe`; resume listing is asserted through `/api/resumes` JSON plus an exact downloaded-bytes comparison because the embedded React shell on `/resumes` carries no filenames.

Overall run conclusion was **failure** for reasons outside this assignment: the `Go source verification` job fails deterministically in `TestLetterSelectionPersistsAndStaysWithinJob` (`workspace_writing_api_test.go:763`, stale selection accepted with 503 `Chromium did not expose a debugging port`). That test exercises managed-Chromium launch on the Ubuntu runner, is untouched by any commit here, and is already fixed upstream by `ce015d0` (`fix(continuation): reject stale letter selection without launching Chromium`) on `v0.2/develop`. The application change was deliberately not duplicated. The `macOS desktop verification` job passed.

## C. Installer candidates

Local handoff candidate (pre-CI, owner transfer to UTM if still wanted):

| Field | Exact value |
| --- | --- |
| Filename | `YourMove-0.2.0-windows-x64-unsigned.exe` |
| Location | `/Users/jenksed/Projects/yourmove-builds/windows-0.2.0-x64-build2-joshua-jenks/YourMove-0.2.0-windows-x64-unsigned.exe` |
| Version / build | `0.2.0` / `2`; Windows file version `0.2.0.2` |
| Platform | Windows 11 native x64; AMD64 application payload |
| Size | 10,132,011 bytes |
| SHA-256 | `def20521848682240e6a7416755754cd5d036027145f571649bd2b69b33597df` |
| Source commit | `4085099dd9636504133d4b8dcf4fba53e4a5b90d`, clean at build |
| Publisher label | Joshua Jenks — owner-selected; production signing identity still needs verification |
| Signature / acceptance | Explicitly unsigned; Windows native acceptance pending |

The same directory contains `YourMove-0.2.0-source.zip`, `YourMove.exe`, checksums and provenance. Transfer those with the installer to the owner's Windows 11 x64 UTM environment and check hashes after transfer. Signing or rebuilding changes artifact identity and requires new checksums/acceptance evidence.

CI-built qualification candidate (current evidence basis; do not substitute the local hash above):

| Field | Exact value |
| --- | --- |
| Filename | `YourMove-0.2.0-windows-x64-unsigned.exe` |
| Location | CI artifact `YourMove-windows-x64-unsigned-533efebe8a3aad42f7a8581bb66f92d861d968ca` on run `42446133465` (14-day retention, private); provenance, source archive and checksums in the same artifact |
| Version / build | `0.2.0` / `2`; Windows file version `0.2.0.2` |
| Platform | Windows Server 2025 AMD64 runner build; AMD64 application payload verified installed |
| SHA-256 | `3d787215d3a46d3dc6625b64a0370c54133c29ed3071329c262b39a4a959ba11` |
| Source commit | `533efebe8a3aad42f7a8581bb66f92d861d968ca`, clean checkout at build |
| Publisher label | Joshua Jenks — owner-selected; production signing identity still needs verification |
| Signature / acceptance | Explicitly unsigned; Windows CI lifecycle qualified (B2), Windows 11 owner GUI acceptance pending |

## D. Distribution matrix

| Channel | Status | Precise remaining prerequisite |
| --- | --- | --- |
| GitHub Releases | **Prepared but blocked** | Verify Joshua Jenks signing identity, sign/verify final candidate, complete Windows acceptance, authorize publication, then publish immutable versioned installer/source/checksums. No release was created |
| yourmove.run | **Prepared but blocked** | Actual approved GitHub asset URL and website-owner integration/publication. Website source is absent here; live download was not verified |
| WinGet | **Prepared but blocked** | Approve proposed `YourMove.YourMove` identifier; publisher is Joshua Jenks; real immutable HTTPS URL and final signed SHA; native `winget validate`, install/detect/upgrade/uninstall tests; separately authorized community submission |
| Microsoft Store EXE | **Prepared but blocked** | Offline WebView2 installer packaging, trusted signatures for every required PE including extracted NSIS DLLs, Windows offline/silent/lifecycle acceptance, immutable URL, approved Partner Center account/publisher/listing |
| Microsoft Store MSIX | **Deferred** | Existing Wails support is real, but Store identity, package configuration, Windows execution/storage/coexistence acceptance and account prerequisites are unresolved. No MSIX produced |

Recommend **NSIS EXE as the default future Store path**: it keeps the current installer/data identity and upgrade model. [Microsoft's requirements](https://learn.microsoft.com/en-us/windows/apps/publish/publish-your-app/msi/app-package-requirements) make the bootstrapper/offline gap and unsigned `nsDialogs.dll`/`System.dll` concrete Store blockers. The current signing hooks cover the app, uninstaller and installer; they do not yet cover those toolchain DLLs. MSIX offers Store signing/updates but adds package identity, WindowsApps lifecycle and [filesystem/uninstall behavior](https://learn.microsoft.com/en-us/windows/msix/desktop/desktop-to-uwp-behind-the-scenes) to qualify, including existing data retention and external Chrome/CDP execution. The [runbook](README.md) records both paths and exact first-WinGet publication actions.

## E. Remaining release blockers

- **CI/verification state:** The Windows and macOS jobs pass on run `42446133465`. The overall run is red only because the `Go source verification` job fails in `TestLetterSelectionPersistsAndStaysWithinJob` (managed-Chromium launch on the Ubuntu runner); the demonstrated upstream fix is `ce015d0` on `v0.2/develop` and was intentionally not duplicated here. A rebase past that commit is an owner scheduling decision, not packaging work.
- **Code/package gaps:** Store-only offline WebView2 provisioning and signing of required NSIS plugin PEs remain missing. The existing acceptance runner has no installed-artifact input: its source fixture passes cannot substitute for native installer acceptance. Missing-WebView2 bootstrap behavior was not qualified (the CI runner already had the Evergreen runtime), and standard-user non-elevation was not qualified (the runner was elevated).
- **Native acceptance:** Owner-run Windows 11 x64 install/first launch with visible GUI, WebView2 behavior without a preinstalled runtime, browser Fill, native files/dialogs, candidate A→B upgrade, and record/byte/association preservation. Use only synthetic workspaces/accounts and the same installed executable/workspace for restart claims. CI proves the installed process starts and the backend persists; it does not prove the Wails window renders or is usable.
- **Signing/publisher:** No production Windows credential was supplied. The owner selected Joshua Jenks. Need matching verified identity and trusted certificate/private-key access plus RFC 3161 timestamp service, then exercise the signing hooks and verify extracted payloads under Windows trust policy. [Azure Artifact Signing](https://learn.microsoft.com/en-us/azure/artifact-signing/quickstart) is an option after owner-approved subscription/account, identity validation, certificate profile and signer access; its adapter is not implemented. No enrollment or account changes performed. The CI candidate is explicitly unsigned.
- **Publisher accounts:** Store Partner Center identity/account, reserved product identity and approved listing assets remain owner prerequisites; account availability was not assumed or changed.
- **Publication:** Owner SHIP decision, GitHub Release/tag/asset upload and immutable actual URL, website download wiring, then WinGet validation/submission and later Store submission. None are authorized by successful packaging alone. No release, tag, manifest submission, website change, or Store publication was made.

## F. Smallest next action

Owner release decisions now gate everything downstream of the qualified unsigned candidate: verify the Joshua Jenks signing identity and provide the production credential (or authorize Azure Artifact Signing enrollment), perform Windows 11 interactive GUI acceptance including a missing-WebView2 machine, and authorize the SHIP decision that creates the immutable public release asset WinGet and the website depend on. No further packaging work should precede those decisions; a second versioned installer is only needed when the A→B upgrade check is scheduled.

Reuse `scripts/qualify-acceptance` from `v0.2/feat-acceptance-runner` (`ce015d02b25757da5e84f70d5875c89cefdac50c`) for its existing controlled-browser/source coverage. It currently accepts an evidence-directory argument only and expressly excludes native installer/upgrade behavior; no replacement acceptance harness was created here.
