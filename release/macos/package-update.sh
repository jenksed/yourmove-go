#!/usr/bin/env bash
# Package an existing Developer ID-signed, notarized YourMove.app for Wails updater.
# This does not sign, notarize, upload, install or publish anything.
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" || $# -ne 2 || "$1" != /* || "$2" != /* ]]; then
  echo "Usage (macOS): bash release/macos/package-update.sh /absolute/YourMove.app /absolute/new/output-directory" >&2
  exit 2
fi
app="$1"
output="$2"
if [[ ! -d "$app" || "$(basename "$app")" != "YourMove.app" || -e "$output" ]]; then
  echo "Expected an existing YourMove.app and a nonexisting output directory." >&2
  exit 2
fi
case "$output/" in
  "$app/"*) echo "Output must not be inside the signed application." >&2; exit 2 ;;
esac
plist="$app/Contents/Info.plist"
executable="$app/Contents/MacOS/YourMove"
test -f "$plist" && test -s "$executable"
/usr/bin/plutil -lint "$plist"
bundle_id="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$plist")"
version="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$plist")"
build="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' "$plist")"
[[ "$bundle_id" == "com.yourmove.go" ]] || { echo "Unexpected bundle ID: $bundle_id" >&2; exit 1; }
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ && "$build" =~ ^[0-9]+$ ]] || {
  echo "Invalid app version/build: $version ($build)" >&2; exit 1;
}
/usr/bin/file "$executable" | /usr/bin/grep -q "arm64"
/usr/bin/codesign --verify --deep --strict --verbose=2 "$app"
/usr/bin/codesign -dv --verbose=4 "$app" 2>&1 | /usr/bin/grep -q 'Authority=Developer ID Application:' || {
  echo "Developer ID Application signature is required." >&2
  exit 1
}
/usr/bin/xcrun stapler validate "$app"
/usr/sbin/spctl --assess --type execute --verbose=4 "$app"

mkdir -p "$output"
archive="$output/YourMove-$version-darwin-arm64-update.zip"
/usr/bin/ditto -c -k --keepParent "$app" "$archive"

unpacked="$(mktemp -d /tmp/yourmove-updater-archive-XXXXXX)"
trap 'rm -rf "$unpacked"' EXIT
/usr/bin/ditto -x -k "$archive" "$unpacked"
entries="$(/usr/bin/find "$unpacked" -mindepth 1 -maxdepth 1 -print)"
[[ "$entries" == "$unpacked/YourMove.app" ]] || {
  echo "Update ZIP must contain exactly one top-level YourMove.app." >&2; exit 1;
}
extracted="$unpacked/YourMove.app"
[[ "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$extracted/Contents/Info.plist")" == "$bundle_id" ]]
[[ "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$extracted/Contents/Info.plist")" == "$version" ]]
/usr/bin/codesign --verify --deep --strict --verbose=2 "$extracted"
/usr/bin/xcrun stapler validate "$extracted"
/usr/sbin/spctl --assess --type execute --verbose=4 "$extracted"
(
  cd "$output"
  /usr/bin/shasum -a 256 "$(basename "$archive")" > SHA256SUMS.update
  /usr/bin/shasum -a 256 -c SHA256SUMS.update
)
printf 'Signed/notarized app archive created locally (NOT PUBLISHED).\n'
printf 'Version: %s (%s); Bundle ID: %s\n' "$version" "$build" "$bundle_id"
printf 'Archive: %s\nChecksums: %s/SHA256SUMS.update\n' "$archive" "$output"
printf 'Still required: owner-approved signed update manifest and installed-app acceptance.\n'
