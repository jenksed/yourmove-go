# PR #7 native manual smoke record

**Purpose:** Minimum owner-run Wails smoke for PR #7 source review. This record
is a blank execution form; no result below has been observed yet.

**Current status:** NOT RUN. Do not change an item to PASS without performing
that exact step in the native Wails app. A browser fixture or a successful
bundle launch does not count as native interaction evidence.

## Run identity

- Date and local time:
- Owner/operator:
- macOS version and hardware:
- PR #7 source commit (full SHA):
- Build/bundle path and SHA-256:
- Fresh synthetic workspace path:
- Wails launch result (record observed output):
- Any permissions or setup changed for this run:

Use a fresh isolated workspace and synthetic content only. Never open a real
employer account or submit an application. Do not use applicant data.

## Results

For each row, replace `NOT RUN` with `PASS` or `FAIL` and record what was
observed. If the step cannot be performed, leave it `NOT VERIFIED` and explain
the specific blocker. Attach screenshots or short logs where they help; ensure
they contain synthetic data only.

| Step | Status | Observation / evidence |
|---|---|---|
| Launch the exact PR source build into the fresh synthetic workspace; verify the expected local workspace opens. | NOT RUN | |
| Open the synthetic opportunity’s Answers activity, edit one synthetic answer, save it, navigate away and back, and verify the saved value. | NOT RUN | |
| Edit an answer and letter without saving, request an unrelated departure, choose **Stay and edit**, and verify both exact values remain and focus returns to the initiating control. Verify no local completion/history record was created. | NOT RUN | |
| Repeat the dirty departure and press **Escape**; verify both exact values remain, the dialog closes, and focus returns. | NOT RUN | |
| Repeat the dirty departure, choose **Leave without saving**, and verify the exact destination opens once with no dialog loop; confirm the saved values remain intact. | NOT RUN | |
| Reopen the opportunity, make fresh dirty answer/letter edits, open the completion confirmation and choose **Cancel and stay**; verify exact values, focus return, and zero new History records. | NOT RUN | |
| Reopen the completion confirmation and press **Escape**; verify exact values, focus return, and zero new History records. | NOT RUN | |
| After a completion cancellation, request ordinary navigation again and verify the dialog has its ordinary **Stay and edit / Leave without saving** copy. Cancel and verify focus returns. | NOT RUN | |
| With an answer and letter dirty, explicitly confirm the synthetic employer-submission declaration and choose **Record completion**. Verify one local record, the expected selected-resume association, unchanged saved answer/letter values, and no employer submission action. | NOT RUN | |
| Inspect History for the synthetic record and its resume association; use **Next job** and verify the next synthetic opportunity opens. | NOT RUN | |
| Quit Wails fully, relaunch the same synthetic workspace, and verify saved answer/letter values and the local History/resume association persist. | NOT RUN | |

## Completion summary

- Failed steps and exact reproduction:
- Steps not run and reason:
- Does any failure block PR #7 source review? Explain:
- Owner decision: `PENDING` / `APPROVE SOURCE MERGE` / `HOLD`
- Owner comments and follow-up issue/PR:

Native transfer, VoiceOver, zoom, reduced motion, export-dialog behavior, and
public release readiness are outside this minimum PR closeout smoke. Keep their
separate acceptance statuses unchanged unless those exact gates are performed.
