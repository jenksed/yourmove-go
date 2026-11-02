#!/bin/bash
# Development qualification only. Never reads or replaces a release bundle.
set -euo pipefail

if [[ "$(uname -s)" != Darwin ]]; then
  echo "Native workflow qualification requires macOS." >&2
  exit 1
fi
if [[ "${1:-}" != "" && "${1:-}" != --launch ]]; then
  echo "Usage: $0 [--launch]" >&2
  exit 1
fi
source_root="$(cd "$(dirname "$0")/.." && pwd)"
output_root="$(mktemp -d /private/tmp/yourmove-workflow-native-XXXXXX)"
bundle="$output_root/YourMoveWorkflow.app"
executable="$bundle/Contents/MacOS/YourMoveWorkflow"
workspace="$output_root/workspace"
evidence="$output_root/evidence"
signing_identity=run.yourmove.workflowqualification
mkdir -p "$bundle/Contents/MacOS" "$bundle/Contents/Resources" "$evidence"
exec > >(tee "$evidence/harness.log") 2>&1
cd "$source_root"
echo "Source commit: $(git rev-parse HEAD)"
git status --short
echo "Source changes (empty for a clean source commit):"
git diff HEAD --binary | shasum -a 256
echo "Bundle: $bundle"
echo "Workspace: $workspace"
echo "Evidence: $evidence"

cat > "$bundle/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>YourMoveWorkflow</string>
<key>CFBundleIdentifier</key><string>run.yourmove.workflowqualification</string>
<key>CFBundleName</key><string>YourMove Workflow Qualification</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleVersion</key><string>1</string>
<key>NSHighResolutionCapable</key><true/>
</dict></plist>
PLIST

# The fresh mktemp destination prevents overwriting a previously executed
# Mach-O vnode (and reusing its cached signature). Go's build cache is allowed;
# no prior bundle or executable output is copied into this bundle.
go build -tags desktop -o "$executable" .
YOURMOVE_NATIVE_FIXTURE_DIR="$workspace" go test -count=1 -tags qualification ./internal/app -run '^TestPrepareNativeWorkflowFixture$' -v

# This development bundle has exactly one executable, no frameworks/helpers.
# Sign the inner executable first, then the enclosing bundle. Never use --deep
# to sign: future nested components must be explicitly signed before this step.
echo "Signing: codesign --force --sign - --timestamp=none --identifier $signing_identity $executable"
codesign --force --sign - --timestamp=none --identifier "$signing_identity" "$executable"
echo "Signing: codesign --force --sign - --timestamp=none --identifier $signing_identity $bundle"
codesign --force --sign - --timestamp=none --identifier "$signing_identity" "$bundle"
echo "Verifying executable and complete bundle before launch:"
codesign --verify --strict --verbose=4 "$executable"
codesign --verify --strict --verbose=4 "$bundle"
codesign --display --verbose=4 "$bundle"
shasum -a 256 "$executable" "$bundle/Contents/Info.plist"
echo "Signature verification: PASS (ad-hoc development identity)."

# No bundle content is written after signing. Runtime storage and logs are
# siblings of the bundle. This is not Developer ID/notarization qualification.
if [[ "${1:-}" == --launch ]]; then
  echo "Launching verified Wails executable at $(date -u +%FT%TZ)"
  # Use the same fail-closed identity checks as frozen-candidate acceptance.
  python3 -c 'import pathlib,secrets,sys; p=pathlib.Path(sys.argv[1]); p.write_text(secrets.token_hex(16)+"\n"); p.chmod(0o600)' "$workspace/.yourmove-synthetic-marker"
  swiftc -module-cache-path "$output_root/swift-cache" "$source_root/scripts/native-window-owner.swift" -o "$output_root/window-owner"
  python3 "$source_root/scripts/native-candidate.py" launch --executable "$executable" --sha256 "$(shasum -a 256 "$executable" | cut -d ' ' -f1)" --workspace "$workspace" --evidence "$output_root/launch-evidence" --helper "$output_root/window-owner"
  echo "Process/window identity verified; native interaction NOT YET VERIFIED."
  echo "Before and after every transition run native-candidate.py verify, then check the matching AX window title."

else
  echo "Launch: NOT ATTEMPTED; native interaction NOT VERIFIED."
fi
