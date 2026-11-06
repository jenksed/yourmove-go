#!/usr/bin/env bash
# Package an existing signed YourMove.app as a Wails-updater compatible ZIP.
# This does not sign, notarize, upload, install or publish anything.
#
# Modes (explicit, default safe):
#   --production (default)  Require Developer ID signature, notarization ticket
#                           and Gatekeeper acceptance. Refuse ad-hoc artifacts.
#   --dev                   Accept an ad-hoc-signed test bundle for controlled
#                           development fixtures only. Never for public release.
#                           Output is clearly labeled NONPRODUCTION.
#
# Examples:
#   bash release/macos/package-update.sh /absolute/YourMove.app /absolute/new/output-directory
#   bash release/macos/package-update.sh --production /absolute/YourMove.app /absolute/new/output-directory
#   bash release/macos/package-update.sh --dev /absolute/YourMove.app /absolute/new/output-directory
set -euo pipefail

mode="production"
if [[ "${1:-}" == "--dev" || "${1:-}" == "--production" ]]; then
  case "$1" in
    --dev) mode="dev" ;;
    --production) mode="production" ;;
  esac
  shift
fi
if [[ "$(uname -s)" != "Darwin" || $# -ne 2 || "$1" != /* || "$2" != /* ]]; then
  echo "Usage (macOS): bash release/macos/package-update.sh [--dev|--production] /absolute/YourMove.app /absolute/new/output-directory" >&2
  echo "Default mode is --production (safe). Use --dev only for ad-hoc development fixtures." >&2
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
codesign_detail="$(/usr/bin/codesign -dv --verbose=4 "$app" 2>&1 || true)"
if [[ "$mode" == "production" ]]; then
  printf 'Mode: production (Developer ID + notarization + Gatekeeper required).\n'
  if printf '%s\n' "$codesign_detail" | /usr/bin/grep -q 'Authority=Developer ID Application:'; then
    :
  else
    echo "Developer ID Application signature is required in production mode (use --dev for ad-hoc test fixtures)." >&2
    exit 1
  fi
  if printf '%s\n' "$codesign_detail" | /usr/bin/grep -q 'Signature=adhoc'; then
    echo "Ad-hoc signature refused in production mode." >&2
    exit 1
  fi
  /usr/bin/xcrun stapler validate "$app"
  /usr/sbin/spctl --assess --type execute --verbose=4 "$app"
else
  printf 'Mode: DEVELOPMENT TEST ONLY - ad-hoc fixture, NOT FOR PUBLIC RELEASE.\n'
  if printf '%s\n' "$codesign_detail" | /usr/bin/grep -q 'Authority=Developer ID Application:'; then
    echo "Developer ID-signed bundle given in --dev mode; use --production for release artifacts to keep channels separate." >&2
    exit 1
  fi
  if ! printf '%s\n' "$codesign_detail" | /usr/bin/grep -q 'Signature=adhoc'; then
    echo "Development mode requires an ad-hoc-signed YourMove.app (codesign --sign -)." >&2
    exit 1
  fi
  # No stapler/spctl in dev: ad-hoc bundles do not carry tickets or Gatekeeper acceptance.
fi

mkdir -p "$output"
if [[ "$mode" == "production" ]]; then
  archive="$output/YourMove-$version-darwin-arm64-update.zip"
else
  archive="$output/YourMove-$version-darwin-arm64-update-dev.zip"
fi
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
if [[ "$mode" == "production" ]]; then
  /usr/bin/xcrun stapler validate "$extracted"
  /usr/sbin/spctl --assess --type execute --verbose=4 "$extracted"
else
  extracted_detail="$(/usr/bin/codesign -dv --verbose=4 "$extracted" 2>&1 || true)"
  if ! printf '%s\n' "$extracted_detail" | /usr/bin/grep -q 'Signature=adhoc'; then
    echo "Extracted bundle lost its ad-hoc signature; packaging did not preserve bytes." >&2
    exit 1
  fi
fi
(
  cd "$output"
  /usr/bin/shasum -a 256 "$(basename "$archive")" > SHA256SUMS.update
  /usr/bin/shasum -a 256 -c SHA256SUMS.update
)
if [[ "$mode" == "production" ]]; then
  printf 'Signed/notarized app archive created locally (NOT PUBLISHED).\n'
  printf 'Version: %s (%s); Bundle ID: %s\n' "$version" "$build" "$bundle_id"
  printf 'Archive: %s\nChecksums: %s/SHA256SUMS.update\n' "$archive" "$output"
  printf 'Still required: owner-approved signed update manifest and installed-app acceptance.\n'
else
  cat > "$output/NONPRODUCTION.txt" <<EOF
DEVELOPMENT TEST ARTIFACT - NOT FOR PUBLIC RELEASE.
Ad-hoc-signed Wails updater fixture packaged for controlled A->B testing only.
It does not establish Gatekeeper trust and must never be published.
Version: $version ($build); Bundle ID: $bundle_id
Archive: $(basename "$archive")
EOF
  printf 'DEVELOPMENT ad-hoc app archive created locally (NOT PUBLISHED, NOT FOR RELEASE).\n'
  printf 'Version: %s (%s); Bundle ID: %s\n' "$version" "$build" "$bundle_id"
  printf 'Archive: %s\nChecksums: %s/SHA256SUMS.update\nMarker: %s/NONPRODUCTION.txt\n' "$archive" "$output" "$output"
  printf 'Still required: development signed manifest with matching test key, and installed-app A->B acceptance.\n'
fi
