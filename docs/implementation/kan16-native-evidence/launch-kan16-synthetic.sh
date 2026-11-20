#!/bin/sh
# KAN-16 owner launcher — synthetic workspace only, no setup, no real data.
# Guards: conflicting writer (port 3017 + jobs.db lock), real-data refusal.
set -eu

SYNTH_DIR="/private/tmp/kan16-workspace-001"
APP_CANDIDATE="/Users/jenksed/YourMove-KAN16/candidate-002/YourMove.app"
APP_FALLBACK="/private/tmp/kan16-candidate-002/YourMove.app"

die() { echo "KAN-16 launcher: $*" >&2; exit 1; }

[ -d "$SYNTH_DIR" ] || die "synthetic workspace missing: $SYNTH_DIR (refusing to create/seed; no real data touched)"
[ -f "$SYNTH_DIR/jobs.db" ] || die "synthetic jobs.db missing in $SYNTH_DIR"

case "${YOURMOVE_DATA_DIR:-$SYNTH_DIR}" in
  "$SYNTH_DIR") DATA_DIR="$SYNTH_DIR" ;;
  *) die "YOURMOVE_DATA_DIR points outside synthetic path (${YOURMOVE_DATA_DIR:-unset}); unset it or set exactly $SYNTH_DIR. Real applicant data is never launched by this script." ;;
esac

if lsof -iTCP:3017 -sTCP:LISTEN -Pn >/dev/null 2>&1; then
  die "port 3017 already in use (another writer/server running). Stop it first; refusing second writer."
fi
if [ -f "$DATA_DIR/jobs.db" ] && lsof "$DATA_DIR/jobs.db" >/dev/null 2>&1; then
  die "jobs.db appears open by another process; refusing second writer."
fi

if [ -d "$APP_CANDIDATE" ]; then APP="$APP_CANDIDATE"; elif [ -d "$APP_FALLBACK" ]; then APP="$APP_FALLBACK"; else die "no candidate app found (checked $APP_CANDIDATE and $APP_FALLBACK)"; fi

echo "Launching KAN-16 synthetic build:"
echo "  app: $APP"
echo "  data: $DATA_DIR (synthetic only)"
echo "  note: ad-hoc signed qualification build; not distribution. HOLD — native paths UNVERIFIED."
YOURMOVE_DATA_DIR="$DATA_DIR" open -a "$APP" --args -YOURMOVE_DATA_DIR "$DATA_DIR"
