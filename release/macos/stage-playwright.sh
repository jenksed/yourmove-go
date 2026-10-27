#!/usr/bin/env bash
# Stage the self-contained Playwright runtime into a built YourMove.app.
#
# Copies into <app>/Contents/Resources:
#   playwright-bridge/  driver.mjs, package files, installed node_modules
#   nodejs/bin/node     official Node.js macOS arm64 binary (vendored)
#   ms-playwright/      pinned browser builds for playwright-core 1.58.2
# and writes Contents/Resources/playwright-manifest.json.
#
# Usage: bash release/macos/stage-playwright.sh /path/to/YourMove.app [vendor-dir]
#
# The bridge never downloads at runtime; every byte it needs is staged here.
set -euo pipefail

test "$#" -ge 1 || { echo 'Usage: bash release/macos/stage-playwright.sh /path/to/YourMove.app [vendor-dir]' >&2; exit 2; }
app="$1"
vendor="${2:-${YOURMOVE_PLAYWRIGHT_VENDOR:-$HOME/Projects/yourmove-builds/vendor/playwright}}"
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

test -d "$app/Contents/MacOS" || { echo "Not a built YourMove.app: $app" >&2; exit 1; }

bridge_src="$repo/internal/application/browser/playwright/bridge"
test -f "$bridge_src/driver.mjs" || { echo "Bridge driver missing: $bridge_src/driver.mjs" >&2; exit 1; }
core_pkg="$bridge_src/node_modules/playwright-core/package.json"
if [[ ! -f "$core_pkg" ]]; then
  echo "Bridge dependencies are not installed; run scripts/setup-playwright-bridge.sh first." >&2
  exit 1
fi

# Pinned revisions for the vendored playwright-core (browsers.json).
need_dirs=(chromium-1208 chromium_headless_shell-1208 firefox-1509 webkit-2248 ffmpeg-1011)
browser_cache="${PLAYWRIGHT_BROWSERS_PATH:-$HOME/Library/Caches/ms-playwright}"
if [[ ! -d "$browser_cache" && -d "$HOME/.cache/ms-playwright" ]]; then
  browser_cache="$HOME/.cache/ms-playwright"
fi
for d in "${need_dirs[@]}"; do
  test -d "$browser_cache/$d" || { echo "Pinned browser missing: $browser_cache/$d" >&2; exit 1; }
done

node_bin="$vendor/nodejs/bin/node"
test -x "$node_bin" || { echo "Vendored Node is not executable: $node_bin (run scripts/setup-playwright-bundle.sh)" >&2; exit 1; }

resources="$app/Contents/Resources"
echo "Staging Playwright runtime into $resources ..."

rm -rf "$resources/playwright-bridge" "$resources/nodejs" "$resources/ms-playwright"
mkdir -p "$resources/playwright-bridge" "$resources/nodejs/bin" "$resources/ms-playwright"

cp "$bridge_src/driver.mjs" "$bridge_src/package.json" "$bridge_src/package-lock.json" "$resources/playwright-bridge/"
cp -R "$bridge_src/node_modules" "$resources/playwright-bridge/node_modules"
chmod -R a+rX "$resources/playwright-bridge"

cp "$node_bin" "$resources/nodejs/bin/node"
chmod 0755 "$resources/nodejs/bin/node"

for d in "${need_dirs[@]}"; do
  cp -R "$browser_cache/$d" "$resources/ms-playwright/$d"
done
chmod -R a+rX "$resources/ms-playwright"

# Drop macOS AppleDouble sidecars (._*) carried in from the browser cache:
# ditto --sequesterRsrc folds them into zip metadata on archive and does not
# restore them as files on extract, which breaks the code seal ("a sealed
# resource is missing or invalid"). They carry no executable content.
find "$resources/playwright-bridge" "$resources/nodejs" "$resources/ms-playwright" -name '._*' -delete

core_version="$(python3 -c 'import json;print(json.load(open("'"$core_pkg"'"))["version"])')"
node_version="$("$resources/nodejs/bin/node" --version)"
{
  printf '{\n'
  printf '  "playwright_core": "%s",\n' "$core_version"
  printf '  "browser_revisions": ["%s", "%s", "%s", "%s", "%s"],\n' "${need_dirs[0]}" "${need_dirs[1]}" "${need_dirs[2]}" "${need_dirs[3]}" "${need_dirs[4]}"
  printf '  "node": "%s"\n' "$node_version"
  printf '}\n'
} > "$resources/playwright-manifest.json"

# Ad-hoc seal the staged executables so the final bundle-wide signature
# covers them; the caller re-signs the whole .app afterwards.
codesign --force --sign - "$resources/nodejs/bin/node" >/dev/null
for d in "${need_dirs[@]}"; do
  codesign --force --sign - "$resources/ms-playwright/$d" >/dev/null 2>&1 || true
done

echo "Playwright runtime staged (core $core_version, node $node_version)."
