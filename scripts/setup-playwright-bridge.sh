#!/bin/sh
# Install the Playwright bridge runtime (Node dependencies only).
# Browser binaries are resolved from the existing Playwright cache and are
# never downloaded by this script or by the bridge at runtime.
set -eu
cd "$(dirname "$0")/../internal/application/browser/playwright/bridge"
npm ci --no-audit --no-fund
node --check driver.mjs
echo "Playwright bridge ready. Browsers resolve via PLAYWRIGHT_BROWSERS_PATH"
echo "(default: ~/Library/Caches/ms-playwright, ~/.cache/ms-playwright)."
