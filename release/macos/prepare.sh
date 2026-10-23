#!/usr/bin/env bash
# Produce an ad-hoc-signed, local release candidate; never publish or notarize.
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo"

if (( $# > 1 )); then
  echo 'Usage: release/macos/prepare.sh [absolute/new/output-directory]' >&2
  exit 2
fi

output="${1:-$HOME/Projects/yourmove-builds/mac-candidate-$(date +%Y%m%d-%H%M%S)-$$}"
case "$output" in
  /*) ;;
  *) echo 'Output directory must be absolute.' >&2; exit 2 ;;
esac
case "$output/" in
  "$repo/"*) echo 'Output must be outside the source repository.' >&2; exit 2 ;;
esac
if [[ -e "$output" ]]; then
  echo "Output already exists; choose a new directory: $output" >&2
  exit 2
fi

bash release/macos/doctor.sh

if [[ -n "$(git status --porcelain)" ]]; then
  echo 'Refusing a release candidate from an uncommitted worktree.' >&2
  git status --short >&2
  exit 1
fi

test_data="$(mktemp -d "${TMPDIR:-/tmp/}yourmove-release-tests.XXXXXX")"
extract_dir=''
cleanup() {
  rm -rf "$test_data"
  if [[ -n "$extract_dir" ]]; then rm -rf "$extract_dir"; fi
}
trap cleanup EXIT

printf '\nRunning tests against an isolated workspace...\n'
YOURMOVE_DATA_DIR="$test_data" go test -count=1 ./...
YOURMOVE_DATA_DIR="$test_data" go test -count=1 -tags desktop ./...

mkdir -p "$(dirname "$output")" "$output"
candidate="$output/candidate"
printf '\nBuilding app bundle...\n'
bash release/macos/build.sh "$candidate"

app="$candidate/YourMove.app"
test -s "$app/Contents/MacOS/YourMove"
test -s "$app/Contents/Resources/YourMove.icns"
/usr/bin/plutil -lint "$app/Contents/Info.plist"

test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$app/Contents/Info.plist")" = 'com.yourmove.go'
test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIconFile' "$app/Contents/Info.plist")" = 'YourMove.icns'
/usr/bin/file "$app/Contents/MacOS/YourMove" | grep -q 'arm64'
/usr/bin/codesign --verify --strict "$app"
(
  cd "$candidate"
  cmp source-manifest.json source-manifest-after.json
  shasum -a 256 -c SHA256SUMS.local
)

version="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$app/Contents/Info.plist")"
binary_zip="$output/YourMove-${version}-macOS-arm64-unnotarized.zip"
source_zip="$output/YourMove-${version}-source.zip"

printf '\nPackaging app and matching source...\n'
/usr/bin/ditto -c -k --sequesterRsrc --keepParent "$app" "$binary_zip"
git archive --format=zip -o "$source_zip" HEAD

printf '\nTesting ZIP extraction and preserved code signature...\n'
extract_dir="$(mktemp -d "${TMPDIR:-/tmp/}yourmove-extracted.XXXXXX")"
/usr/bin/ditto -x -k "$binary_zip" "$extract_dir"
extracted_app="$extract_dir/YourMove.app"
test -s "$extracted_app/Contents/MacOS/YourMove"
test -s "$extracted_app/Contents/Resources/YourMove.icns"
/usr/bin/codesign --verify --strict "$extracted_app"
cmp "$app/Contents/Resources/YourMove.icns" "$extracted_app/Contents/Resources/YourMove.icns"

git rev-parse HEAD > "$output/source-commit.txt"
(
  cd "$output"
  shasum -a 256 \
    "$(basename "$binary_zip")" \
    "$(basename "$source_zip")" \
    candidate/source-manifest.json \
    > SHA256SUMS
)

printf '\nRELEASE CANDIDATE PREPARED\n'
printf 'Commit: %s\n' "$(git rev-parse --short HEAD)"
printf 'Binary ZIP: %s\n' "$binary_zip"
printf 'Source ZIP: %s\n' "$source_zip"
printf 'Checksums: %s\n' "$output/SHA256SUMS"
printf 'Candidate bundle: %s\n' "$app"
printf '\nAd-hoc signed, NOT notarized or publicly qualified. No files uploaded.\n'
