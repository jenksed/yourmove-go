#!/bin/bash
# KAN-16 deterministic closure-qualification fixture: local only, do not distribute.
# Builds the explicitly opt-in qualification binary and mints a FRESH synthetic
# workspace. No real applicant data is read; no employer is contacted (the
# qualification enrichment seam answers from a fixed map); nothing submits.
set -euo pipefail
[[ "$(uname -s)" == Darwin ]] || { echo 'Requires macOS.' >&2; exit 1; }
[[ $# == 0 ]] || { echo 'Usage: scripts/build-kan16-closure-fixture.sh' >&2; exit 1; }
source_root="$(cd "$(dirname "$0")/.." && pwd)"
root="$(mktemp -d /private/tmp/yourmove-react-native-kan16-XXXXXX)"
bundle="$root/YourMoveKAN16Closure.app"
executable="$bundle/Contents/MacOS/YourMoveKAN16Closure"
mkdir -p "$bundle/Contents/MacOS" "$bundle/Contents/Resources"
printf 'yourmove-react-fixture-v1\n' > "$root/.yourmove-react-fixture"
cat > "$bundle/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>YourMoveKAN16Closure</string>
<key>CFBundleIdentifier</key><string>run.yourmove.kan16closure</string>
<key>CFBundleName</key><string>YourMove KAN-16 Closure Qualification</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleVersion</key><string>1</string>
<key>NSHighResolutionCapable</key><true/>
</dict></plist>
PLIST
cd "$source_root"
go build -tags desktop,qualification -o "$executable" .
codesign --force --sign - --timestamp=none "$executable"
codesign --force --sign - --timestamp=none "$bundle"
codesign --verify --strict "$bundle"
shasum -a 256 "$executable"
printf 'Source commit: %s\n' "$(git rev-parse HEAD)"
printf 'Synthetic workspace: %s\nExecutable: %s\n' "$root" "$executable"
cat <<EOF
First launch (seeds the four fixtures; run from /private/tmp):
  cd /private/tmp && YOURMOVE_DATA_DIR=${root} YOURMOVE_QUALIFY_KAN16=1 ${executable}
Restart (same executable, same workspace, no reseeding):
  cd /private/tmp && YOURMOVE_DATA_DIR=${root} YOURMOVE_QUALIFY_KAN16=1 ${executable}
Fixture mapping (all initially active; open the workspace job to qualify):
  1. Synthetic Open Role (KAN-16)     https://job-boards.greenhouse.io/kan16qual-open/jobs/11111111      -> open    (View: managed browser via loopback fixture; resume+answer+letter in scope)
  2. Synthetic Closable Role (KAN-16) https://job-boards.greenhouse.io/kan16qual-closed/jobs/22222222    -> closed  (View: persistent closure notice, moved to History, no launch; resume preserved)
  3. Synthetic Unknown Role (KAN-16)  https://job-boards.greenhouse.io/kan16qual-unknown/jobs/33333333  -> unknown (View: warning + manual fallback proceeds)
  4. Synthetic Applied Role (KAN-16)  https://job-boards.greenhouse.io/kan16qual-applied/jobs/44444444   -> Applied immune (recorded submission preserved; availability reports complete)
Availability evidence (after each single click; same workspace):
  sqlite3 ${root}/jobs.db "SELECT id,title,stage,source_status FROM jobs;" 2>/dev/null || python3 -c "import sqlite3;print([(r[0],r[1],r[2],r[3]) for r in sqlite3.connect('${root}/jobs.db').execute('SELECT id,title,stage,source_status FROM jobs')])"
  Expected: fixture 2 stage=closed after its check; fixture 4 stays applied with its submission row; fixtures 1 and 3 stay active.
Notes: external posting open stays disabled in qualification (truthful error, no real call).
  HOLD: native GUI paths are UNVERIFIED until observed; passing fixtures do not prove native acceptance. No submit.
EOF
