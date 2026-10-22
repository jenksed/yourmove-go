# YourMove Go — Checkpoint 3 qualification

Starting state: clean `main` at `592731be3b27f59a364657cf5f647c2901bf57fe`; fetched `origin/main` matched before edits. The repository has no standalone Checkpoint 2 report. Relevant implementation records read were the Checkpoint 1 report, the AI-answer implementation and design specification, the application-experience report, the trial-run fixes, the codebase guide, and historical Wails compatibility notes.

## Early-answer reconciliation

The accepted-state shortcut trusted the saved live inspection fingerprint and question key. It did not check the current `application_answers` row. An early draft could therefore continue to display as accepted after the live saved answer was changed, cleared, or removed. The accept form also did not carry the live answer revision, so a stale form could replace a newer live edit.

Reconciliation now checks the current saved answer for the mapped live question. It labels the early draft accepted only while the current live answer text still matches. Changed, cleared, and removed answers get distinct states; the current live answer remains untouched and the stale early copy remains available for review. The acceptance POST checks the live answer revision captured when the review form rendered, in addition to the existing early-answer revision and inspection identity checks.

Regression coverage verifies all three changed/cleared/removed states, preserves the independent early and live records, checks that no acceptance action is offered for stale copies, and rejects a stale acceptance POST after a newer live answer was saved.

## Ashby read-only preview

`chromium.Preview()` launches the saved Ashby destination through the existing Chromium session code. The process receives a fresh `os.MkdirTemp` profile with mode `0700` and an explicit `--user-data-dir`; no existing browser profile or managed application session is reused. Preview now adds `--headless=new`, avoiding a transient employer window. It runs the read-only DOM observation script once and exposes no fill, upload, or submit operation.

`Preview()` closes its own Chromium process and removes its temporary directory on success, failure, or cancellation. Cancellation is returned as cancellation even if a DOM snapshot was already observed. Cleanup errors are surfaced. The controlled loopback qualification verified an existing managed-session fixture remained open and unchanged, observed only GET requests, created no answer or submission records, and removed the temporary profile after both completion and cancellation. The fixture rejected non-GET requests. No candidate profile, answer, or file was supplied to the page.

Opening an employer page can still cause ordinary website requests, analytics, or other network activity. Preview is not network-invisible.

## Desktop and Chromium qualification

The production-tagged Wails binary built and was launched once with an isolated workspace at `/private/tmp/yourmove-go-checkpoint3-native/data` and a working directory outside the repository. Wails logged its startup line. WebKit then reported `Code=513 / Operation not permitted` while creating app-specific cache and website-data directories under `~/Library/Caches` and `~/Library/WebKit`, alongside repeated LaunchServices/font messages. The process did not report a Go fatal error and remained alive until that isolated process was stopped.

The Computer Use app list did not expose the launched binary, so the bridge could not attach. No second app copy was opened. The cache errors were reproduced, but their effect on usable native rendering could not be established; they are not treated as proof of application failure or proof that the UI worked. Native Wails navigation and interaction remain unverified.

DesktopHandler HTTP tests verified Wails-origin request handling and safe navigation behavior. Separate controlled Chromium tests used temporary SQLite/resume storage, fake generation, and loopback employer pages. They passed for:

- Greenhouse early question discovery, answer editing, local save, fake AI source review/generation, and restart recovery.
- Ashby read-only question preview, managed-session isolation, no field mutations or submissions, and cancellation cleanup.
- Live question reconciliation and authorized answer transfer, resume/document handoff, and zero employer submissions.
- Local completion recording, Applied/History, next-job navigation, and persistence after restart.

These are controlled browser and HTTP results, not native WebView results and not real employer submissions.

## Verification run

- `go test -count=1 ./...` — passed.
- `go test -count=1 -tags desktop ./...` — passed.
- `go build ./...` — passed.
- `go build -tags desktop,production -o /private/tmp/yourmove-go-checkpoint3 .` — passed.
- `node --check static/ui.js` — passed.
- `git diff --check` — passed.
- Focused reconciliation tests — passed, including changed, cleared, removed, and stale-submit cases.
- Race tests for reconciliation and Chromium preview cancellation/cleanup — passed.
- Opt-in controlled Chromium AI, Greenhouse early-answer, Ashby preview, combined continuation, handoff, submission/history/next-job, and restart qualifications — passed with synthetic data.

Focused controlled commands used:

```sh
YOURMOVE_QUALIFY_BROWSER=1 YOURMOVE_QUALIFY_EARLY=1 YOURMOVE_QUALIFY_PREVIEW=1 \
  go test -count=1 -tags qualification ./internal/app \
  -run 'TestControlled(NarrativeAnswerWorkbench|EarlyPreparationWorkbench|AshbyPreview(IsReadOnlyAndSessionIsolated|CancellationRemovesIsolatedProfile)|CombinedContinuation|SubmissionRecordingWorkflow|RuntimeRestart)$'

YOURMOVE_QUALIFY_BROWSER=1 go test -count=1 -tags qualification \
  ./internal/application/browser/chromium -run '^TestControlledApplicationHandoff$'

go test -race -count=1 ./internal/app \
  -run 'TestGreenhouseEarlyAnswerNeedsExplicitLiveAcceptance|TestAcceptedEarlyAnswerReflectsCurrentLiveSavedAnswer|TestEarlyAcceptanceRejectsChangedLiveAnswerRevision'
YOURMOVE_QUALIFY_PREVIEW=1 go test -race -count=1 -tags qualification ./internal/app \
  -run 'TestControlledAshbyPreview(IsReadOnlyAndSessionIsolated|CancellationRemovesIsolatedProfile)$'
go test -race -count=1 ./internal/application/browser/chromium
```

Go printed non-fatal module stat-cache write warnings because the default module cache is outside the writable workspace. The requested builds and tests exited successfully using a temporary writable `GOCACHE`.

No five-posting live pilot was run. The user has not selected the actual Greenhouse, Ashby, and Lever/partially supported postings. No live employer result or throughput improvement is claimed. Use the applicant-run procedure and blank capture sheet below when those postings are selected.

## Native Wails manual acceptance checklist

Run this with a dedicated persistent data directory. Do not use the normal YourMove data directory for synthetic checks. Keep the employer browser and YourMove actions under the applicant's control; YourMove must never submit an employer application.

1. Build and launch the verified binary using the safe command in the Checkpoint 3 handoff. Confirm `/jobs` renders and navigation to an opportunity and back works.
2. Add `https://example.com/wails-smoke` with title `Wails form test` through the native form. It is an unsupported synthetic source, so no ATS fetch is expected. Confirm the saved row and its detail page. Edit a local title or priority, save, and verify the persisted value. Do not count this smoke-test row as one of the five pilot applications.
3. For an applicant-selected Greenhouse posting, explicitly discover questions. Open one early question, edit it, save it locally, navigate away, and confirm the text returns unchanged.
4. Review AI source selections and the exact payload. The controlled Chromium qualification used an injected fake generator and passed. For a native-only check, use a local fake compatible-provider endpoint if one is already available; otherwise mark native generation unverified. Do not configure a paid or remote provider solely for this check. Confirm preview does not save until the ordinary Save action.
5. For an applicant-selected Ashby posting, run the read-only preview. Confirm no employer window appears, no form value changes, and no upload or submission occurs. The page request may still create normal network traffic.
6. Open the applicant-selected employer form manually. Inspect live fields, reconcile one unchanged question, and verify a changed/cleared/removed live answer is never labeled as the current accepted answer.
7. Authorize a saved answer or resume transfer only after reviewing its exact scope. Check the observed result in the employer browser. Do not treat selection or a local “filled” result as employer acceptance.
8. Complete the employer application manually only when it is an actual application the applicant intends to submit. Then, and only then, record completion in YourMove. Verify Applied and History.
9. Use Next job and confirm it opens the next saved opportunity without changing either answer record.
10. Quit this test instance, relaunch with the same `YOURMOVE_DATA_DIR`, and verify the opportunity, local answers, and any applicant-confirmed completion remain present.
11. Record the visible result for every step. If WebKit cache errors recur, capture the terminal output and separately note whether the window rendered and accepted local navigation/forms; the log alone does not decide that result.

## Five-application supervised pilot

The applicant selects five real postings for actual use: two Greenhouse, two Ashby, and one Lever or otherwise partially supported posting. Do not duplicate an application to create a baseline. Before the pilot, choose recent comparable applications completed without YourMove for a personal baseline, or mark the baseline unknown. A self-estimated baseline must be labeled as an estimate.

For each applicant-selected posting, use one row in [`2026-10-22-checkpoint3-pilot-template.csv`](2026-10-22-checkpoint3-pilot-template.csv). Capture applicant-controlled timestamps for:

`Opportunity saved → questions available → answers prepared → employer browser opened → live fields reconciled → material transferred → manual submission → completion recorded`.

Use the app's local inspection/answer/submission timestamps where displayed. Use the applicant's own clock or stopwatch for browser opening, mismatch work, employer submission, and any event without a local timestamp. Record mismatch time as the total active time spent resolving differences, not idle time. Record blocked transitions and manual interventions even when the application is not completed.

Count early questions before the live browser opens; count saved answers by distinct question; count “accepted for live use” only after explicit reconciliation; count “verified filled” only for a Chromium outcome labeled `filled`; track already-correct fields separately. Never infer employer acceptance from local verification. Compare total time and preparation time with the applicant's labeled baseline after all five rows are complete. If a posting is blocked or abandoned, keep its row and reason. Do not upload the filled capture sheet or employer data to GitHub; keep it local.

## Safe persistent launch

The tested binary is `/private/tmp/yourmove-go-checkpoint3`. To rebuild it and launch with persistent isolated storage:

```sh
cd /Users/jenksed/Projects/yourmove-go
GOCACHE=/private/tmp/yourmove-go-checkpoint3-build \
  go build -tags desktop,production -o /private/tmp/yourmove-go-checkpoint3 .

mkdir -p "$HOME/Library/Application Support/YourMove-Checkpoint3-Pilot-data"
mkdir -p "$HOME/Library/Application Support/YourMove-Checkpoint3-Pilot-runner"
cd "$HOME/Library/Application Support/YourMove-Checkpoint3-Pilot-runner"
YOURMOVE_DATA_DIR="$HOME/Library/Application Support/YourMove-Checkpoint3-Pilot-data" \
  /private/tmp/yourmove-go-checkpoint3
```

Keep the same data directory on later runs and do not run a second YourMove instance against it. Launching from the empty runner directory avoids importing a repository-local legacy database.
