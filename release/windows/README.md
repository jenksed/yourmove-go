# Windows x64 packaging

Build local candidates from a clean, committed checkout. This uses Wails **v3.1.0**'s existing asset, icon, resource, and WebView2-bootstrapper generators and its NSIS helpers. It does not activate Wails' experimental YAML build system. Existing macOS packaging remains independent.

```sh
# Go version from go.mod, Node from frontend/.nvmrc, pnpm 10.14.0,
# Python 3.11+, and NSIS 3.13 on PATH. Private Go module access is required.
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
# macOS build host: brew install makensis
# Windows build host: install NSIS 3.13; add its directory to PATH.
python3 release/windows/prepare.py /absolute/new/candidate-directory
```

On Windows use `python` and a full Windows output path. Set `WAILS3` if the pinned CLI is outside PATH. The output must be outside the checkout and must not exist. Failed output is retained for inspection and is never a releasable candidate. The build owns one ignored, exclusively created root `.syso` temporarily and removes it in `finally`; after an interrupted process, inspect any remaining resource before removing it. Do not run concurrent packaging builds in the same checkout.

`product.json` supplies Windows version, build, product and publisher metadata. It pins the existing `com.yourmove.go` identity. Wails embeds the existing YourMove PNG as a multi-resolution Windows icon. Go linker flags pin `AppVersion`/`AppBuild`, select the GUI subsystem, and embed the production Vite build plus existing templates/static assets/prompts. There are no applicant-storage or database-writer changes.

Outputs include `YourMove-<version>-windows-x64-unsigned.exe`, `YourMove.exe`, a matching source archive, source manifests before/after, tool/build logs, `candidate.json`, `SHA256SUMS`, and unpublished WinGet templates. Hashes describe the **final** bytes. Signed builds omit `-unsigned`. Never replace an already published versioned asset with different bytes. Rebuild and repeat acceptance if the source, version, build, payload, or signature changes.

## Installer contract

| Property | Contract; Windows execution still required |
| --- | --- |
| Supported platform | Windows 11 native x64; x86 and ARM64 are rejected |
| Privilege/scope | Current user; `RequestExecutionLevel user`; no application elevation |
| Files | Fixed `%LOCALAPPDATA%\Programs\YourMove`; `/D` does not change it |
| Registration | HKCU 64-bit uninstall key `com.yourmove.go`, Start menu shortcut, display icon/version/publisher, quoted uninstall and quiet-uninstall commands |
| Install/upgrade | Interactive by default; exact case-sensitive `/S` for silent. Close YourMove first. Overwrite application files in place, same-version repair allowed, newer-to-older version/build refused |
| Persistent work | Existing `%APPDATA%\YourMove` (`internal/storage/paths.go`), or explicit `YOURMOVE_DATA_DIR`; never touched by installer/uninstaller |
| Uninstall | Only named installer-owned files and shortcut removed; nonrecursive directory removal; no workspace, settings, browser cache, WebView2 runtime, or Chrome removal |
| WebView2 | Detect machine/user Evergreen registration; otherwise run Wails' embedded Microsoft bootstrapper `/silent /install`, verify success and registration before writing app files. Missing-runtime install needs internet |
| Employer browser | Production Chromium path discovers a separately installed Google Chrome or Microsoft Edge browser (Program Files/LOCALAPPDATA/PATH), preferring Chrome when both are present; no developer cache, Go or Node required. Firefox/Playwright runtimes are not bundled |
| Return codes | 0 success; 64 OS, 65 architecture (Wails), 66 downgrade, 67 executable locked, 70 WebView2 failure, 71 file/registration failure; cancellation follows NSIS defaults |

This is installer-over-installer maintenance, not a transactional updater or backup system. A running/locked application is refused without killing it or scheduling reboot replacement. A file/registration failure requires rerunning the installer. No Windows background updates are added. Test default-path retention in a disposable Windows account; all application acceptance uses an absolute synthetic `YOURMOVE_DATA_DIR`.

## Production signing

The owner-selected publisher is **Joshua Jenks**, used for Windows metadata and rendered WinGet publisher fields and intended for the future Store listing. The product name remains **YourMove**. Production signing still requires a matching verified identity and trusted credential. Keep Windows metadata, WinGet publisher and Partner Center identity consistent. The uninstall key and install path do not change with publisher spelling.

The pinned Wails CLI supports `wails3 tool sign -input ... -thumbprint ... -timestamp ...` on Windows and portable PE signing with an exported PFX. No project Windows signing configuration existed at the baseline. This package uses the standard NSIS `!uninstfinalize` integration plus Windows SDK SignTool so all three owner-produced PEs can be verified under Windows trust policy:

```powershell
# Use an already enrolled certificate and available private key. No secrets in source.
$env:YOURMOVE_WINDOWS_CERT_THUMBPRINT = '<existing certificate thumbprint>'
$env:YOURMOVE_WINDOWS_TIMESTAMP_URL = '<approved RFC 3161 service URL>'
$env:YOURMOVE_WINDOWS_SIGNER_SUBJECT = '<exact approved certificate subject>'
python release/windows/prepare.py C:\Builds\YourMove-new-signed-candidate --sign
```

`sign.ps1` signs/timestamps the app first, the generated uninstaller before embedding, and the final installer. It requires successful `signtool verify /pa /all /v`, a valid Authenticode result, a timestamp and the expected certificate subject. Logs stay in the candidate. No credential was supplied or signature verified in the unsigned build. The signed branch and extracted-uninstaller verification must be exercised on Windows before production use.

[Azure Artifact Signing](https://learn.microsoft.com/en-us/azure/artifact-signing/quickstart) is a viable managed alternative. It requires an Azure subscription/account, successful eligible individual/organization public-trust identity validation, certificate profile, and Certificate Profile Signer permissions. Its [SignTool integration](https://learn.microsoft.com/en-us/azure/artifact-signing/how-to-signing-integrations) uses the Artifact Signing client/Dlib and account/profile metadata in place of the certificate-store selection in `sign.ps1`; retain timestamping and verification at all three signing points. This adapter is not implemented or verified. No enrollment, paid resource or account change is authorized by this procedure.

## Channels and publication handoff

The existing workflow is `.github/workflows/ci.yml`. Its Windows job now builds/inspects and retains an **unsigned CI artifact**. It does not publish a release. Repository release scripts previously covered macOS only; no Windows publishing automation or website source/configuration was found here.

After signing and exact-artifact Windows acceptance, the owner can approve publication of `YourMove-<version>-windows-x64.exe`, its source archive, `candidate.json`, and `SHA256SUMS` to the version's GitHub Release. Preserve the actual source commit and module access/source obligations of the existing release process. Use the release's actual immutable asset URL for yourmove.run's Windows download. Confirm HTTP download, filename, final SHA-256 and downloaded-install behavior before claiming either channel works. No release, upload, live website edit or push is performed by `prepare.py`.

WinGet uses this same NSIS installer. The [Microsoft manifest specification](https://learn.microsoft.com/en-us/windows/package-manager/package/manifest) and split templates in `winget/` describe `nullsoft`, x64, user scope, `/S`, install-in-place upgrades and the real Installed Apps key. `YourMove.YourMove` is a **proposed**, owner-reviewable identifier. The candidate renders its actual hash locally but leaves `@IMMUTABLE_INSTALLER_URL@` unresolved; unsigned-candidate hashes must not be copied to a subsequently signed release.

First WinGet publication actions, after Windows release approval:

1. Confirm the proposed package identifier, verify the Joshua Jenks signing identity, and freeze the signed, accepted version. Publish the final installer at its actual immutable HTTPS release asset URL.
2. Start from that candidate's three `winget-unpublished/*.yaml.in` files. Rename to `.yaml`; substitute the real URL and confirm final SHA-256 from an independent download. Confirm publisher, version and ARP matching on Windows.
3. On Windows run `winget validate --manifest <directory>`; enable local manifests if needed and test `winget install --manifest <directory> --scope user`, silent installation, detection, upgrade from candidate A to B, and `winget uninstall --id YourMove.YourMove`. Use only disposable synthetic accounts/workspaces. Revalidate with the community repository's current schema/tools.
4. With separate owner authorization, submit the three files to `microsoft/winget-pkgs` under `manifests/y/YourMove/YourMove/<version>/`. Do not use wingetcreate's submission option during local preparation.

## Store recommendation

Default to the **existing NSIS EXE path**, once eligible. [Microsoft's EXE requirements](https://learn.microsoft.com/en-us/windows/apps/publish/publish-your-app/msi/app-package-requirements) require a silent standalone/offline installer, immutable versioned HTTPS URL and trusted signing of the binary and its PE files. Current `/S`, identity and lifecycle are suitable foundations, but this candidate is unsigned and its WebView2 bootstrapper downloads missing bits. Inspection also found no signature tables on NSIS' extracted `nsDialogs.dll` and `System.dll`; the three owner-produced-EXE signing hooks alone do not satisfy the Store's all-PE requirement. Before Store EXE submission, bundle Microsoft's offline x64 Evergreen standalone installer in this same packaging identity, prepare and verify trusted signatures for the required NSIS plugins as well as the executable payloads, and prove offline clean installation and Store lifecycle on Windows. Chrome or Edge remains a separately disclosed optional feature prerequisite; review Store dependency policy during certification. An approved Partner Center publisher/account, metadata, privacy/support/listing assets and submission remain external owner tasks.

MSIX is **deferred**, rather than claimed unsupported. Wails v3.1.0 has `wails3 tool msix` (legacy route requires Windows SDK MakeAppx) and an experimental portable Wake implementation. Neither is configured for this repository. Store MSIX offers Microsoft signing/hosting/updates, but would add package identity and WindowsApps lifecycle. Microsoft documents [filesystem/registry virtualization and uninstall differences](https://learn.microsoft.com/en-us/windows/msix/desktop/desktop-to-uwp-behind-the-scenes) and [desktop packaging constraints](https://learn.microsoft.com/en-us/windows/msix/desktop/desktop-to-uwp-prepare). A later experiment must qualify `%APPDATA%\YourMove` access/retention and any migration, NSIS coexistence/dual launch/single SQLite writer, full-trust Chrome/CDP process launch and temporary profiles, WebView2 prerequisites, native import/export and Store identity/capabilities. No MSIX, second installation identity, storage migration or updater change is created here.

## Existing acceptance runner

Read-only audit: branch `v0.2/feat-acceptance-runner`, `ce015d02b25757da5e84f70d5875c89cefdac50c`, `scripts/qualify-acceptance`. It accepts only an evidence-directory argument, builds/runs source tests and controlled production-browser fixtures, and expressly excludes native installer/upgrade/artifact behavior. It cannot currently consume an installer. It is not on this packaging baseline; do not copy/reimplement it or claim its results cover this candidate.

Hand the owner the installer plus `candidate.json`, `SHA256SUMS`, matching source and this contract for Windows 11 x64 UTM. Verify the checksum after transfer. In a disposable account, run interactive and `/S` setup; inspect Start menu/Installed Apps/icon/publisher/default paths; check WebView2 present and missing/offline failure; test missing browser, then installed Chrome or Edge, using the existing controlled fixtures. Use an absolute synthetic `YOURMOVE_DATA_DIR` before launching the installed executable. Save opportunities, distinct résumé bytes/IDs/associations, profile, role families, answers/Answer Library, letters, history and preferences; quit/reopen the same executable/workspace without reseeding. Exercise native import/export/cancel, a locked-app refusal, A-to-B upgrade then same-workspace comparison, downgrade refusal, uninstall/reinstall retention. For the default data path use only the disposable account and synthetic records. Record exact installer/app hashes, commits, OS, browser version and evidence through the existing acceptance process. No new harness is supplied.

Native Windows behavior, signatures, CI execution and public download links remain **NOT VERIFIED** until those actual paths run. Packaging implementation is not owner release acceptance. Ubuntu installers, Windows ARM64, additional package managers and automatic Windows updates are deferred.
