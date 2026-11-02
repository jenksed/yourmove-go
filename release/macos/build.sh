#!/bin/bash
# Local candidate only. No signing, network upload, installation, or launch.
set -euo pipefail
test "$(uname -s)" = Darwin || { echo 'Build on macOS.' >&2; exit 1; }
test "$#" = 1 && [[ "$1" = /* ]] || { echo 'Usage: bash release/macos/build.sh /absolute/new/output-directory' >&2; exit 1; }
release_output="$1"
test ! -e "$release_output" || { echo 'Output must not already exist.' >&2; exit 1; }
release_repo="$(cd "$(dirname "$0")/../.." && pwd)"
case "$release_output/" in "$release_repo/"*) echo 'Keep build output outside the source workspace.' >&2; exit 1;; esac
mkdir -p "$release_output/YourMove.app/Contents/MacOS"
cd "$release_repo"
python3 release/macos/source_manifest.py > "$release_output/source-manifest.json"
cp release/macos/Info.plist "$release_output/YourMove.app/Contents/Info.plist"
mkdir -p "$release_output/YourMove.app/Contents/Resources"
cp release/macos/YourMove.icns "$release_output/YourMove.app/Contents/Resources/YourMove.icns"
/usr/bin/plutil -lint "$release_output/YourMove.app/Contents/Info.plist"
# The initial qualification matrix is intentionally the OS and architecture
# actually tested. Lowering this floor requires a separate compatibility run.
export GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=26.0
export CGO_CFLAGS='-mmacosx-version-min=26.0'
export CGO_CXXFLAGS='-mmacosx-version-min=26.0'
export CGO_LDFLAGS='-mmacosx-version-min=26.0'
GOPROXY=off go build -trimpath -buildvcs=false -tags desktop,production -ldflags='-s -w' -o "$release_output/YourMove.app/Contents/MacOS/YourMove" .
python3 release/macos/source_manifest.py > "$release_output/source-manifest-after.json"
cmp "$release_output/source-manifest.json" "$release_output/source-manifest-after.json"
go version > "$release_output/toolchain.txt"
xcrun clang --version >> "$release_output/toolchain.txt"
xcrun --show-sdk-version >> "$release_output/toolchain.txt"
go version -m "$release_output/YourMove.app/Contents/MacOS/YourMove" > "$release_output/build-info.txt"
xcrun vtool -show-build "$release_output/YourMove.app/Contents/MacOS/YourMove" > "$release_output/deployment-target.txt"
# Stage the self-contained Playwright runtime (bridge, Node, browsers) when
# its sources are available. YOURMOVE_BUNDLE_PLAYWRIGHT controls this:
#   auto (default) — stage when complete, otherwise continue without it and
#     record the absence; the bundle still runs with the Chromium CDP default.
#   required       — fail if staging is impossible (use for release candidates).
#   skip           — never stage.
bundle_mode="${YOURMOVE_BUNDLE_PLAYWRIGHT:-auto}"
case "$bundle_mode" in
  skip)
    echo 'Skipping Playwright staging (YOURMOVE_BUNDLE_PLAYWRIGHT=skip).'
    ;;
  auto|required)
    if bash "$release_repo/release/macos/stage-playwright.sh" "$release_output/YourMove.app"; then
      echo 'Playwright runtime staged.'
    elif [[ "$bundle_mode" == required ]]; then
      echo 'Playwright staging is required but failed.' >&2
      exit 1
    else
      echo 'Playwright runtime NOT staged; bundle runs with the Chromium CDP default only.'
    fi
    ;;
  *)
    echo "Unknown YOURMOVE_BUNDLE_PLAYWRIGHT=$bundle_mode (expected auto, required, or skip)." >&2
    exit 1
    ;;
esac
/usr/bin/codesign --force --sign - --deep "$release_output/YourMove.app"
/usr/bin/codesign --verify --strict "$release_output/YourMove.app"
(cd "$release_output" && shasum -a 256 source-manifest.json YourMove.app/Contents/Info.plist YourMove.app/Contents/MacOS/YourMove > SHA256SUMS.local)
echo "Local ad-hoc candidate: $release_output/YourMove.app"
echo 'Not notarized; do not distribute this local candidate to beta testers.'
