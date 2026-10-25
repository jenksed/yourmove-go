# YourMove workspace backup and restore

Your workspace contains a SQLite database **and** private resume files. Back up
and restore the complete `YourMove` directory as one unit. A copy of `jobs.db`
alone is not a usable backup. Quit YourMove before copying so the database and
files represent the same saved state. These commands use the default macOS
location; replace `workspace` if the installation uses `YOURMOVE_DATA_DIR`.

## Create and verify a backup

```sh
set -eu
workspace="$HOME/Library/Application Support/YourMove"
backup="$HOME/Documents/YourMove-backup-$(date +%Y%m%d-%H%M%S).tar.gz"

test -f "$workspace/jobs.db"
tar -czf "$backup" -C "$(dirname "$workspace")" "$(basename "$workspace")"
shasum -a 256 "$backup" | tee "$backup.sha256"
tar -tzf "$backup" | sed -n '1,20p'
```

Keep the archive and its `.sha256` file together in a private location. The
archive contains applicant data; do not email it, attach it to an issue, or
include it in source control.

## Restore a backup

Quit YourMove first. Verify the archive, then move the current workspace aside
instead of merging old and new files. The restore command below requires that
the destination path be empty after the move.

```sh
set -eu
workspace="$HOME/Library/Application Support/YourMove"
backup="$HOME/Documents/YourMove-backup-YYYYMMDD-HHMMSS.tar.gz"

shasum -a 256 -c "$backup.sha256"
if test -e "$workspace"; then
  mv "$workspace" "$workspace.before-restore-$(date +%Y%m%d-%H%M%S)"
fi
tar -xzf "$backup" -C "$(dirname "$workspace")"
test -f "$workspace/jobs.db"
sqlite3 "$workspace/jobs.db" 'PRAGMA integrity_check;'
```

The final command must print `ok`. Reopen YourMove and verify a few jobs,
resume selections, documents, saved answers, letters, and completion records
before removing the prior workspace. Keep both directories until that review
passes. If a restore or migration reports a missing/damaged document, stop and
restore the complete matching workspace; do not re-import the same resume over
the registered record.

## Legacy working-directory data

When starting from an older working-directory `jobs.db`, first back up both
that database and its sibling `resumes/` directory. The automatic database
migration intentionally refuses a legacy database containing resume records:
it cannot safely move document files on its own. Restore the complete workspace
to the configured application-support location before launching the new app.
The original legacy files are left in place when this safety check refuses the
migration.
