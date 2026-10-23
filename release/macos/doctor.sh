#!/usr/bin/env bash
# Local macOS release prerequisite check. Read-only; never signs or publishes.
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo"

failures=0
pass() { printf 'PASS  %s\n' "$1"; }
fail() { printf 'FAIL  %s\n' "$1"; failures=$((failures + 1)); }
check() {
  local description="$1"
  shift
  if "$@" >/dev/null 2>&1; then pass "$description"; else fail "$description"; fi
}

check 'macOS host' test "$(uname -s)" = 'Darwin'
check 'Apple Silicon arm64' test "$(uname -m)" = 'arm64'

if command -v sw_vers >/dev/null 2>&1; then
  os_version="$(sw_vers -productVersion)"
  major="${os_version%%.*}"
  if [[ "$major" =~ ^[0-9]+$ ]] && (( major >= 26 )); then
    pass "macOS $os_version (requires 26+)"
  else
    fail "macOS $os_version (requires 26+)"
  fi
else
  fail 'sw_vers available'
fi

for cmd in git go python3 xcrun xcode-select plutil codesign ditto shasum file iconutil; do
  check "Tool available: $cmd" command -v "$cmd"
done
check 'PlistBuddy available' test -x /usr/libexec/PlistBuddy
check 'Apple Command Line Tools configured' xcode-select -p

expected_go="$(awk '$1 == "go" {print $2; exit}' go.mod)"
if command -v go >/dev/null 2>&1; then
  installed_go="$(go version)"
  if [[ "$installed_go" == *"go${expected_go} darwin/arm64"* ]]; then
    pass "Go $expected_go"
  else
    fail "Expected Go $expected_go for darwin/arm64; found: $installed_go"
  fi
fi

plist='release/macos/Info.plist'
check 'Info.plist parses' /usr/bin/plutil -lint "$plist"
if [[ -f "$plist" ]]; then
  bundle_id="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$plist" 2>/dev/null || true)"
  icon_name="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIconFile' "$plist" 2>/dev/null || true)"
  check 'Expected bundle identifier' test "$bundle_id" = 'com.yourmove.go'
  check 'Expected icon reference' test "$icon_name" = 'YourMove.icns'
fi
check 'App icon present' test -s release/macos/YourMove.icns
check 'PNG icon source present' test -s release/macos/YourMove-icon-1024.png
check 'Build script references bundled icon' grep -Fq 'Contents/Resources/YourMove.icns' release/macos/build.sh
check 'Build script syntax' bash -n release/macos/build.sh
check 'Distribution script syntax' bash -n release/macos/distribute.sh

if [[ -n "$(git status --porcelain)" ]]; then
  printf 'WARN  Working tree has uncommitted changes (prepare.sh requires clean source).\n'
else
  pass 'Clean Git worktree'
fi

if security find-identity -v -p codesigning 2>/dev/null | grep -Fq 'Developer ID Application:'; then
  pass 'Developer ID Application identity present (optional for local candidate)'
else
  printf 'INFO  No Developer ID Application identity; only ad-hoc builds can be prepared.\n'
fi

if (( failures != 0 )); then
  printf '\nPreflight failed: %d required check(s).\n' "$failures" >&2
  exit 1
fi
printf '\nPreflight passed.\n'
