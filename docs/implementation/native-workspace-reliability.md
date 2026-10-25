# Native workspace reliability recovery — 2026-10-25

Release status: HOLD pending frozen-candidate native acceptance. Builds 2 and 3 are
superseded by source changes; the next candidate is 0.1.0 (4).

## Established defect and limits

`ApplicationStorage` previously ran working-directory legacy discovery even when
`YOURMOVE_DATA_DIR` explicitly selected an empty workspace. A synthetic regression
on a14e7fb demonstrates legacy jobs appearing in that selected workspace.
An explicit override now suppresses legacy discovery completely. Normal mode
retains the consistent SQLite snapshot migration and refusal to migrate a modern
workspace containing documents without its document directory.

This defect does not establish the cause of either historical window incident.
The latest record reports an initially empty synthetic database before the later
unexpected window. The earlier record explicitly describes display-name app
resolution after restart. Missing per-transition PID/window ownership evidence
prevents choosing between a different process/window and an application defect.
No historical personal-database baseline exists; immutability is not claimed.

## Storage and identity boundaries

- Empty-but-set overrides fail instead of silently selecting default storage.
- Absolute roots are canonicalized once. Filesystem root, invalid ancestors,
  symlinked DB/document members and incompatible member types fail closed.
  Deliberate symlinked root aliases resolve to one canonical workspace.
- DB and document dependencies are selected at startup and retained by the App;
  generation and navigation do not reselect them from the environment.
- Opt-in `YOURMOVE_NATIVE_RUN_ID` requires an explicit marked synthetic workspace
  before storage is opened. Normal launches create no identity record and keep
  the ordinary window title.
- Diagnostic launches write an exclusive mode-0600 identity record and use a
  random run ID plus PID in the native title. Records contain paths, hashes and
  process metadata only, never applicant content or credentials.
- `scripts/native-candidate.py` directly launches the exact hashed executable
  from the synthetic root, captures inventory and process start time, and checks
  the opened database inode and native window owner PID/executable/title.
  `scripts/native-window-owner.swift` performs read-only OS metadata inspection.
  Missing title/ownership permission fails verification; it is not bypassed.
- Before and after each major native transition, run `verify` and compare the
  CUA accessibility window title with the verified title. Do not use display-name
  or bundle-ID discovery as identity. Stop before UI mutation on any mismatch.

These checks protect controlled qualification; they are not a filesystem sandbox
against hostile concurrent local filesystem replacement. No new app architecture,
submission behavior, data migration or normal duplicate-instance policy is added.
Wails single-instance forwarding is not configured by YourMove.

## Regression coverage

Storage tests exercise explicit empty isolation despite a legacy DB, permitted
normal migration, refusal of incomplete modern migration, invalid configuration,
canonical root aliases, redirected members, separate synthetic DBs and repeated
restart. Native identity tests require marked explicit roots and exclusive launch
records. Python guard tests reject unrelated/stale windows, mismatched processes,
changed workspace identity, absent open DB and reused process lifetimes; CI runs
these tests. A blocked synthetic generator test confirms unrelated navigation
works and the result stays in the original DB despite a changed environment,
then verifies its document association after reopen.

## Controlled-browser harness correction

Two repeatable qualification failures checked a target before navigation had
finished. The contact journey incorrectly assumed a redirect even when observation
updated the UI in place and preserved its question query. It now acquires the
verified tab after observation, accepting only the two exact expected fixture
URLs before issuing the next navigation once. The step-two attachment fixture
briefly had no matching target during the page transition. Qualification-only connection
discovery now waits up to three seconds for the exact loopback URL. It never
replays a CDP operation. The attachment test's bounded wait also tolerates the
preflight no-target state; production attachment/ownership behavior is unchanged.

## Qualification and handoff

The private, ignored recovery package under `.quality/native-recovery-20261025/`
contains preserved incident records, the original worktree inventory and patch,
build-2 checksum, baseline reproduction, exact command logs and candidate-specific
acceptance results. Consult its final `NATIVE_ACCEPTANCE.md` and `TEST_RESULTS.md`
for PASS/FAIL/NOT VERIFIED; this source note is not native acceptance.

Two supported-provider controlled Chromium journeys are not native Wails
acceptance. No real employer application may be submitted. Source/CI, native
qualification and downloaded-install/distribution approval remain separate gates.
