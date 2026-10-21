#!/bin/bash
# Run only after the owner authorizes signing and upload to Apple's notary service.
# The keychain profile must already exist; never pass account passwords here.
set -euo pipefail
test "$#" = 1 && [[ "$1" = /* ]] || { echo 'Usage: bash release/macos/distribute.sh /absolute/candidate-directory' >&2; exit 1; }
: "${YOURMOVE_SIGN_IDENTITY:?Set a Developer ID Application identity}"
: "${YOURMOVE_NOTARY_PROFILE:?Set an existing notarytool keychain profile name}"
case "$YOURMOVE_SIGN_IDENTITY" in 'Developer ID Application: '*) ;; *) echo 'Developer ID Application identity required.' >&2; exit 1;; esac
release_candidate="$1"
release_app="$release_candidate/YourMove.app"
test -f "$release_candidate/source-manifest.json" && test -f "$release_app/Contents/MacOS/YourMove"
(cd "$release_candidate" && shasum -a 256 -c SHA256SUMS.local)
test ! -e "$release_candidate/notary-submit.zip" && test ! -e "$release_candidate/YourMove-0.1.0-macos-arm64.zip"
/usr/bin/codesign --force --options runtime --timestamp --sign "$YOURMOVE_SIGN_IDENTITY" "$release_app"
/usr/bin/codesign --verify --deep --strict --verbose=2 "$release_app"
/usr/bin/codesign -dv --verbose=4 "$release_app" 2> "$release_candidate/signing.txt"
/usr/bin/ditto -c -k --keepParent "$release_app" "$release_candidate/notary-submit.zip"
xcrun notarytool submit "$release_candidate/notary-submit.zip" --keychain-profile "$YOURMOVE_NOTARY_PROFILE" --wait --output-format json > "$release_candidate/notary-result.json"
python3 -c 'import json,sys; assert json.load(open(sys.argv[1]))["status"] == "Accepted", "Notarization was not accepted"' "$release_candidate/notary-result.json"
xcrun stapler staple "$release_app"
xcrun stapler validate "$release_app"
/usr/sbin/spctl --assess --type execute --verbose=4 "$release_app" 2> "$release_candidate/gatekeeper.txt"
/usr/bin/ditto -c -k --keepParent "$release_app" "$release_candidate/YourMove-0.1.0-macos-arm64.zip"
(cd "$release_candidate" && shasum -a 256 source-manifest.json YourMove.app/Contents/Info.plist YourMove.app/Contents/MacOS/YourMove YourMove-0.1.0-macos-arm64.zip > SHA256SUMS.release)
echo 'Signed and notarized artifact prepared locally. Clean-machine acceptance is still required; nothing was published.'
