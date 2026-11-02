#!/bin/bash
# Local qualification only: do not distribute this synthetic fixture build.
set -euo pipefail
[[ "$(uname -s)" == Darwin ]] || { echo 'Requires macOS.' >&2; exit 1; }
[[ $# == 0 ]] || { echo 'Usage: scripts/build-react-native-fixture.sh' >&2; exit 1; }
source_root="$(cd "$(dirname "$0")/.." && pwd)"
root="$(mktemp -d /private/tmp/yourmove-react-native-XXXXXX)"
bundle="$root/YourMoveReactFixture.app"
executable="$bundle/Contents/MacOS/YourMoveReactFixture"
mkdir -p "$bundle/Contents/MacOS" "$bundle/Contents/Resources"
printf 'yourmove-react-fixture-v1\n' > "$root/.yourmove-react-fixture"
cat > "$bundle/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>YourMoveReactFixture</string>
<key>CFBundleIdentifier</key><string>run.yourmove.reactfixture</string>
<key>CFBundleName</key><string>YourMove React Qualification</string>
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
printf 'Run from /private/tmp: YOURMOVE_DATA_DIR=%q %q\n' "$root" "$executable"
printf 'On restart, reuse the same command and workspace; do not rerun this builder.\n'
