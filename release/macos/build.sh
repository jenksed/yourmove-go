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
# Build the embedded Opportunities frontend reproducibly before embedding.
# The Go executable serves frontend/dist; never rely on a Vite dev server.
if [ -f "$release_repo/frontend/package.json" ]; then
  if ! command -v pnpm >/dev/null 2>&1; then
    echo 'pnpm is required to build the embedded frontend (see frontend/README.md).' >&2
    exit 1
  fi
  (cd "$release_repo/frontend" && pnpm install --frozen-lockfile && pnpm run build)
  (cd "$release_repo/frontend" && pnpm list react react-dom)
fi
# The initial qualification matrix is intentionally the OS and architecture
# actually tested. Lowering this floor requires a separate compatibility run.
# Pin the binary version from the bundle metadata so the updater's
# CurrentVersion always matches Info.plist by construction.
app_version="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$release_output/YourMove.app/Contents/Info.plist")"
app_build="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' "$release_output/YourMove.app/Contents/Info.plist")"
[[ "$app_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ && "$app_build" =~ ^[0-9]+$ ]] || {
  echo "Invalid app version/build in Info.plist: $app_version ($app_build)" >&2; exit 1; }
ldflags="-s -w -X github.com/jenksed/yourmove-go/internal/app.AppVersion=${app_version} -X github.com/jenksed/yourmove-go/internal/app.AppBuild=${app_build}"

# Default production channel and public key are pinned below. Overrides must
# provide both values; disabling both requires an explicit local-build opt-out.
manifest_url="${YOURMOVE_UPDATE_MANIFEST_URL-https://github.com/jenksed/yourmove-go/releases/download/update-channel/stable.json}"
pubkey_file="${YOURMOVE_UPDATE_PUBLIC_KEY_FILE-$release_repo/release/macos/update-public-key.pem}"
if [[ -n "$manifest_url" || -n "$pubkey_file" ]]; then
  [[ -n "$manifest_url" && -n "$pubkey_file" ]] || {
    echo 'Set both YOURMOVE_UPDATE_MANIFEST_URL and YOURMOVE_UPDATE_PUBLIC_KEY_FILE, or neither.' >&2; exit 1; }
  [[ "$manifest_url" =~ ^https://[A-Za-z0-9._~:/%+-]+$ ]] || { echo 'Update manifest URL must be a safe HTTPS URL.' >&2; exit 1; }
  test -s "$pubkey_file" || { echo "Update public key file missing or empty: $pubkey_file" >&2; exit 1; }
  grep -q -- '-----BEGIN PUBLIC KEY-----' "$pubkey_file" || { echo "Update public key file is not a PEM public key: $pubkey_file" >&2; exit 1; }
  grep -q -- '-----END PUBLIC KEY-----' "$pubkey_file" || { echo "Update public key file is not a PEM public key: $pubkey_file" >&2; exit 1; }
  openssl pkey -pubin -in "$pubkey_file" -text -noout 2>/dev/null | grep -q "ED25519 Public-Key" || { echo 'Expected a valid Ed25519 public key.' >&2; exit 1; }
  # Collapse the PEM to literal \n escapes for -X injection; the updater
  # expands them back at startup (see desktop_updater.go).
  pubkey_escaped="$(awk '{printf "%s\\n", $0}' "$pubkey_file")"
  test -n "$pubkey_escaped" || { echo "Update public key file unreadable: $pubkey_file" >&2; exit 1; }
  # Single-quote the -X values: go splits ldflags quote-aware, and the PEM
  # header contains spaces. PEM never contains single quotes.
  ldflags="$ldflags -X 'main.updaterManifestURLEmbedded=${manifest_url}' -X 'main.updaterPublicKeyEmbedded=${pubkey_escaped}'"
  printf 'Pinning update channel %s with key %s\n' "$manifest_url" "$pubkey_file"
else
  [[ "${YOURMOVE_ALLOW_DISABLED_UPDATER:-}" == 1 ]] || {
    echo 'Set the update channel and key; for an explicitly disabled local build set YOURMOVE_ALLOW_DISABLED_UPDATER=1.' >&2; exit 1; }
  echo 'Explicit local build with automatic updates disabled.'
fi
export GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=26.0
export CGO_CFLAGS='-mmacosx-version-min=26.0'
export CGO_CXXFLAGS='-mmacosx-version-min=26.0'
export CGO_LDFLAGS='-mmacosx-version-min=26.0'
GOPROXY=off go build -trimpath -buildvcs=false -tags desktop,production -ldflags="$ldflags" -o "$release_output/YourMove.app/Contents/MacOS/YourMove" .
if [[ -n "$manifest_url" ]]; then
  /usr/bin/grep -qF -- "$manifest_url" "$release_output/YourMove.app/Contents/MacOS/YourMove" || {
    echo 'Pinned manifest URL not found in built executable; refusing candidate.' >&2; exit 1; }
  echo 'Pinned manifest URL verified inside built executable.'
fi
python3 release/macos/source_manifest.py > "$release_output/source-manifest-after.json"
cmp "$release_output/source-manifest.json" "$release_output/source-manifest-after.json"
go version > "$release_output/toolchain.txt"
xcrun clang --version >> "$release_output/toolchain.txt"
xcrun --show-sdk-version >> "$release_output/toolchain.txt"
go version -m "$release_output/YourMove.app/Contents/MacOS/YourMove" > "$release_output/build-info.txt"
xcrun vtool -show-build "$release_output/YourMove.app/Contents/MacOS/YourMove" > "$release_output/deployment-target.txt"
/usr/bin/codesign --force --sign - "$release_output/YourMove.app"
/usr/bin/codesign --verify --strict "$release_output/YourMove.app"
(cd "$release_output" && shasum -a 256 source-manifest.json YourMove.app/Contents/Info.plist YourMove.app/Contents/MacOS/YourMove > SHA256SUMS.local)
echo "Local ad-hoc candidate: $release_output/YourMove.app"
echo 'Not notarized; do not distribute this local candidate to beta testers.'
