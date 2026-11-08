YourMove - Windows 11 x64

Install for your current Windows account. Setup does not request administrator
rights. Close YourMove before an upgrade; run the newer installer over the
existing installation. Downgrades are refused. Launch from the Start menu.

WebView2: setup uses the installed Microsoft Evergreen Runtime, or runs the
included Microsoft bootstrapper to download it. Internet is required if missing.
Employer-browser Fill: install Google Chrome stable separately. YourMove discovers
its standard machine or per-user installation. No developer Playwright cache,
Node.js, Go, or checkout is required for the Chromium path. Firefox requires a
separately provisioned Playwright runtime and is not bundled in this installer.

Application files: %LOCALAPPDATA%\Programs\YourMove
Saved work: %APPDATA%\YourMove (database, resumes and associated saved material).
Uninstall via Windows Settings > Apps > Installed apps. Uninstall retains saved
work and preferences; reinstall uses the same workspace. There is no background
Windows updater. Back up the whole data directory with YourMove fully closed.
An explicit YOURMOVE_DATA_DIR environment variable selects a different workspace;
acceptance tests must use an absolute, isolated synthetic workspace.

An installer with '-unsigned' in its filename is for controlled qualification.
Its publisher is not authenticated and Windows may display security warnings.
It has not passed Windows native acceptance or been approved for public release.
