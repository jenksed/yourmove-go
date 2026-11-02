#!/bin/sh
# Fetch the self-contained Playwright distribution vendor inputs.
#
# Downloads the pinned official Node.js macOS arm64 binary into a vendor
# directory OUTSIDE the repository, and verifies the pinned Playwright
# browser builds (matching playwright-core 1.58.2) are installed in the
# local Playwright cache, installing any missing ones.
#
# Usage: scripts/setup-playwright-bundle.sh [vendor-dir]
# Default vendor dir: $HOME/Projects/yourmove-builds/vendor/playwright
#
# Nothing here touches the release bundle; release/macos/stage-playwright.sh
# copies these inputs into YourMove.app at build time.
set -eu

NODE_VERSION="${YOURMOVE_NODE_VERSION:-v22.23.4}"
PLAYWRIGHT_CORE="${YOURMOVE_PLAYWRIGHT_CORE:-1.58.2}"
VENDOR="${1:-${YOURMOVE_PLAYWRIGHT_VENDOR:-$HOME/Projects/yourmove-builds/vendor/playwright}}"
BROWSERS_PATH="${PLAYWRIGHT_BROWSERS_PATH:-$HOME/Library/Caches/ms-playwright}"

# Pinned revisions for playwright-core 1.58.2 (see bridge/node_modules
# playwright-core/browsers.json).
NEED_DIRS="chromium-1208 chromium_headless_shell-1208 firefox-1509 webkit-2248 ffmpeg-1011"

mkdir -p "$VENDOR/nodejs/bin"

NODE_BIN="$VENDOR/nodejs/bin/node"
if [ -x "$NODE_BIN" ]; then
  echo "Node already vendored: $("$NODE_BIN" --version) ($NODE_BIN)"
else
  echo "Downloading Node.js $NODE_VERSION for darwin-arm64..."
  TMPD="$(mktemp -d "${TMPDIR:-/tmp/}yourmove-node.XXXXXX")"
  trap 'rm -rf "$TMPD"' EXIT
  BASE="https://nodejs.org/dist/$NODE_VERSION"
  TARBALL="node-$NODE_VERSION-darwin-arm64.tar.gz"
  curl -fsSL -o "$TMPD/$TARBALL" "$BASE/$TARBALL"
  curl -fsSL -o "$TMPD/SHASUMS256.txt" "$BASE/SHASUMS256.txt"
  (cd "$TMPD" && grep -F "$TARBALL" SHASUMS256.txt | shasum -a 256 -c -)
  tar -xzf "$TMPD/$TARBALL" -C "$TMPD"
  cp "$TMPD/node-$NODE_VERSION-darwin-arm64/bin/node" "$NODE_BIN"
  chmod 0755 "$NODE_BIN"
  trap - EXIT
  rm -rf "$TMPD"
  echo "Vendored Node: $("$NODE_BIN" --version) ($NODE_BIN)"
fi

missing=""
for d in $NEED_DIRS; do
  if [ ! -d "$BROWSERS_PATH/$d" ]; then
    missing="$missing $d"
  fi
done

if [ -n "$missing" ]; then
  echo "Installing missing Playwright browsers:$missing"
  bridge_dir="$(cd "$(dirname "$0")/../internal/application/browser/playwright/bridge" && pwd)"
  (cd "$bridge_dir" && npx --yes "playwright-core@$PLAYWRIGHT_CORE" install chromium firefox webkit)
  for d in $NEED_DIRS; do
    if [ ! -d "$BROWSERS_PATH/$d" ]; then
      echo "Browser still missing after install: $BROWSERS_PATH/$d" >&2
      exit 1
    fi
  done
else
  echo "Pinned browsers present in $BROWSERS_PATH"
fi

echo "Vendor ready: $VENDOR"
