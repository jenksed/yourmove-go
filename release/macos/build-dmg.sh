#!/bin/sh
# Build the branded YourMove macOS DMG from an exact accepted candidate ZIP.
#
# The application itself is never rebuilt or modified: the candidate ZIP is
# checksum-verified first, and the embedded .app keeps its executable,
# resources, bundle identity, and ad-hoc signature. This script only stages
# the installer presentation (background, Applications link, assistance
# shortcuts) and wraps it in a compressed disk image.
#
# Usage:
#   bash release/macos/build-dmg.sh [published-zip] [output-dir]
#
# Defaults match the accepted v0.1.0 build 3 candidate. Both arguments must be
# absolute paths when supplied; the output directory must not exist yet.
# Override the frozen-candidate identity for another accepted version/build:
#   YOURMOVE_DMG_EXPECTED_SHA=<zip sha256> \
#   YOURMOVE_DMG_VERSION=<x.y.z> YOURMOVE_DMG_BUILD=<n> \
#   [YOURMOVE_DMG_NAME=<custom.dmg>] \
#   bash release/macos/build-dmg.sh /absolute/candidate.zip /absolute/new/outdir
set -eu

EXPECTED_ZIP_SHA="${YOURMOVE_DMG_EXPECTED_SHA:-d8d4baf30f65b233f2671459925d17d4b0f94a77e29521e42a54ccc90fdf2ad1}"
VOLUME_NAME="YourMove"
DMG_VERSION="${YOURMOVE_DMG_VERSION:-0.1.0}"
DMG_BUILD="${YOURMOVE_DMG_BUILD:-3}"
DMG_NAME="${YOURMOVE_DMG_NAME:-YourMove-$DMG_VERSION-macOS-arm64.dmg}"
case "$DMG_VERSION" in
	[0-9]*.[0-9]*.[0-9]*) ;;
	*) echo "Invalid YOURMOVE_DMG_VERSION: $DMG_VERSION" >&2; exit 2 ;;
esac
case "$DMG_BUILD" in
	''|*[!0-9]*) echo "Invalid YOURMOVE_DMG_BUILD: $DMG_BUILD" >&2; exit 2 ;;
esac
case "$DMG_NAME" in
	*/*) echo "Invalid YOURMOVE_DMG_NAME: must be a bare filename" >&2; exit 2 ;;
esac
WINDOW_W=700
WINDOW_H=600

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
ZIP="${1:-$HOME/Projects/yourmove-builds/mac-candidate-20261104-201712-32006/YourMove-0.1.0-macOS-arm64-unnotarized.zip}"
OUTDIR="${2:-$HOME/Projects/yourmove-builds/mac-dmg-0.1.0-build3}"

if [ ! -f "$ZIP" ]; then
	echo "Published ZIP not found: $ZIP" >&2
	exit 1
fi
if [ -e "$OUTDIR" ]; then
	echo "Output directory already exists (refusing to overwrite): $OUTDIR" >&2
	exit 1
fi
if [ -d "/Volumes/$VOLUME_NAME" ]; then
	echo "A volume named $VOLUME_NAME is already mounted; unmount it first so Finder automation cannot touch the wrong disk." >&2
	exit 1
fi

echo "Verifying published ZIP..."
actual="$(shasum -a 256 "$ZIP" | awk '{print $1}')"
if [ "$actual" != "$EXPECTED_ZIP_SHA" ]; then
	echo "ZIP checksum mismatch: got $actual, want $EXPECTED_ZIP_SHA" >&2
	exit 1
fi
echo "ZIP checksum OK."

WORK="$(mktemp -d /tmp/yourmove-dmg.XXXXXX)"
DEVICE=""
cleanup() {
	if [ -n "$DEVICE" ]; then
		hdiutil detach "$DEVICE" 2>/dev/null || true
	fi
	rm -rf "$WORK"
}
trap cleanup EXIT

STAGE="$WORK/stage"
mkdir -p "$STAGE/.background"
echo "Staging installer contents..."
unzip -q "$ZIP" -d "$STAGE"
rm -rf "$STAGE/__MACOSX"

APP="$STAGE/YourMove.app"
test -d "$APP" || { echo "YourMove.app missing after extraction" >&2; exit 1; }

echo "Verifying embedded application..."
bundle_id=$(/usr/libexec/PlistBuddy -c "Print CFBundleIdentifier" "$APP/Contents/Info.plist")
test "$bundle_id" = "com.yourmove.go" || { echo "bundle id: $bundle_id" >&2; exit 1; }
short_version=$(/usr/libexec/PlistBuddy -c "Print CFBundleShortVersionString" "$APP/Contents/Info.plist")
build_version=$(/usr/libexec/PlistBuddy -c "Print CFBundleVersion" "$APP/Contents/Info.plist")
test "$short_version" = "$DMG_VERSION" || { echo "version: $short_version" >&2; exit 1; }
test "$build_version" = "$DMG_BUILD" || { echo "build: $build_version" >&2; exit 1; }
/usr/bin/file "$APP/Contents/MacOS/YourMove" | grep -q arm64 || { echo "not arm64" >&2; exit 1; }
/usr/bin/codesign --verify --deep --strict "$APP" || { echo "signature check failed" >&2; exit 1; }
echo "App identity OK: $bundle_id $short_version build $build_version arm64, ad-hoc signature verifies."

ln -s /Applications "$STAGE/Applications"

make_webloc() { # $1 = filename, $2 = URL
	python3 - "$STAGE/$1" "$2" <<'EOF'
import plistlib, sys
path, url = sys.argv[1], sys.argv[2]
with open(path, "wb") as f:
    plistlib.dump({"URL": url}, f, fmt=plistlib.FMT_XML)
EOF
}
# Internet-location shortcuts. The Settings pane uses .inetloc because
# Finder will not follow a custom URL scheme from a .webloc file; the
# installation guide is an ordinary https link either way.
make_webloc "Open Privacy & Security.inetloc" \
	"x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension"
make_webloc "Installation Guide.webloc" \
	"https://www.yourmove.run/docs/availability/"
/usr/bin/SetFile -a E "$STAGE/Open Privacy & Security.inetloc" "$STAGE/Installation Guide.webloc"

cp "$REPO_ROOT/release/macos/dmg-background.png" "$STAGE/.background/dmg-background.png"

RW_DMG="$WORK/rw.dmg"
FS_SIZE="$(du -sm "$STAGE" | awk '{print $1}')"
FS_SIZE="$((FS_SIZE + 24))m"
echo "Creating $FS_SIZE writable image..."
hdiutil create -srcfolder "$STAGE" -volname "$VOLUME_NAME" -fs HFS+ \
	-format UDRW -size "$FS_SIZE" -ov "$RW_DMG" >/dev/null

echo "Mounting for Finder layout..."
mkdir -p "$WORK/mnt"
# Fixed mountpoint: hdiutil would otherwise mount a second copy at
# "/Volumes/YourMove 1", and whitespace-splitting that path breaks parsing.
ATTACH_OUT="$(hdiutil attach -readwrite -noverify -noautoopen -mountpoint "$WORK/mnt" "$RW_DMG")"
MOUNT="$WORK/mnt"
DEVICE="$(echo "$ATTACH_OUT" | awk '/\/dev\/disk/ {print $1}' | head -1)"
test -n "$DEVICE" || { echo "mount failed" >&2; exit 1; }

BG_ALIAS="$MOUNT/.background/dmg-background.png"
# Item references use display names (extensions stay hidden via SetFile -a E
# above); Finder resolves them and the positions below are verified on
# macOS 26-seeded windows.
osascript <<EOF
tell application "Finder"
	-- Address our own mount by path, never by volume name: another disk
	-- with the same name must not receive this layout.
	set theFolder to folder (POSIX file "$MOUNT" as alias)
	open theFolder
	set win to Finder window 1
	set current view of win to icon view
	set toolbar visible of win to false
	set statusbar visible of win to false
	set sidebar width of win to 0
	set the bounds of win to {100, 100, $((100 + WINDOW_W)), $((100 + WINDOW_H))}
	set opts to the icon view options of win
	set arrangement of opts to not arranged
	set icon size of opts to 96
	set text size of opts to 12
	set background picture of opts to POSIX file "$BG_ALIAS"
	set position of item "YourMove.app" of win to {180, 205}
	set position of item "Applications" of win to {520, 205}
	set position of item "Open Privacy & Security" of win to {180, 335}
	set position of item "Installation Guide" of win to {520, 335}
	close win
	delay 1
	open theFolder
	set win to Finder window 1
	update theFolder without registering applications
	delay 2
	close win
end tell
EOF

echo "Detaching..."
hdiutil detach "$DEVICE" >/dev/null
DEVICE=""
sync

mkdir -p "$OUTDIR"
FINAL="$OUTDIR/$DMG_NAME"
echo "Compressing final DMG..."
hdiutil convert "$RW_DMG" -format UDZO -imagekey zlib-level=9 -o "$FINAL" >/dev/null
(cd "$OUTDIR" && shasum -a 256 "$DMG_NAME" > "$DMG_NAME.sha256")

echo "DMG ready: $FINAL"
cat "$OUTDIR/$DMG_NAME.sha256"
