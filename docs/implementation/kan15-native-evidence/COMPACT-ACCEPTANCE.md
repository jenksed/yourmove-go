# KAN-15 Compact native acceptance

**READY FOR OWNER VERIFICATION.** Agent-observed on macOS 27.2.1 (arm64), Wails 3.1.0. The application source was clean at `63adb6e338930802a6ff5280b5829017cd2d8751`. The evidence commit adds documentation and screenshots.

| Artifact | Identity |
| --- | --- |
| Ad-hoc signed application | `/private/tmp/yourmove-workflow-native-pXIhf4/YourMoveWorkflow.app` |
| Executable SHA-256 | `c3ff286dece429ec98e0de67ad0cb5ba512b1c7d5aa4fb2d11320562c65c41b2` |
| Synthetic workspace | `/private/tmp/yourmove-workflow-native-ojOXIu/compact-workspace` |
| Launch and restart | Signed executable path, clean source revision and SQLite file ownership verified before UI work. Same executable and workspace used after full exit. |

## Native acceptance

| Check | Result | Observation |
| --- | --- | --- |
| Compact list and action hierarchy | PASS | Title leads each row, followed by company, source, stage and résumé status. Open workspace is the primary button. Direct move and Posting controls remain visible. Normal list showed zero checkboxes. [Focus](compact-focus.jpg) · [Saved for Later](compact-saved.jpg) |
| Shared title resolution | PASS | Suggested next and the Focus row displayed Screenshot and linked to job 43. The existing posting-slug display fallback leaves the stored title empty. [Native title evidence](compact-title-consistency.jpg) |
| Single and bulk movement | PASS | Prepared job 1 moved Focus → Saved for Later → Focus at Review. Select multiple now sits with the list filters. Two selected fixtures moved together in both directions and their stages remained Review. [Bulk selection](compact-bulk.jpg) |
| Protected deletion and terminal jobs | PASS | In History, selecting the Closed record disabled both move buttons. The separate deletion disclosure required typing DELETE and checking confirmation. Changing selection cleared both. No deletion was submitted. |
| Mixed import and retry | PASS | One new URL, one duplicate and one invalid line produced a compact 1/1/1 summary. The duplicate stayed in Focus. The textarea retained just the invalid line and cleared on successful retry. [Native summary](compact-import.jpg) |
| Search and sorting | PASS | The populated Saved list reached 41 rows. Search matched the prepared job by title and ten fixture rows by posting slug. Recently saved showed the newest imported item first. Search uses stored text and URL, so a space-separated rendered fallback such as Fixture 3 still requires the corresponding URL slug fixture-3. |
| Populated History | PASS | Applied and Closed synthetic records appeared at their recorded stages. The local submission count remained one. [History](compact-history.jpg) |
| Prepared workspace and restart | PASS | After a full quit and reopen without reseeding, job 1 was in Focus at Review with résumé ID 1, the same revision-1 answer and the selected letter. A read-only SQLite check matched those records and quick check returned ok. [Preparation after restart](compact-prepared-after-restart.jpg) |
| Narrow and keyboard | PASS | At the left-tiled native size, row actions wrapped below metadata and remained visible. Option-Tab traversed the list controls and Space entered bulk mode and opened the deletion disclosure. The exact minimum width and screen-reader speech were not separately exercised. [Narrow rows](compact-narrow.jpg) |

TypeScript and Vite production build passed. Focused Go tests passed for title resolution, triage, attention moves, selection guards, batch reconciliation and Opportunities. Independent review found a possible fixed-width icon overlap, which was corrected before signing the verified build. Its focused test rerun passed after an intermittent batch-test temporary-directory cleanup error. This was development signing. The question marked **[Synthetic fixture] Why this team?** comes from the qualification fixture.

## Owner walkthrough

Quit other YourMove instances and launch this candidate from Terminal with the prepared fixture.

```sh
YOURMOVE_DATA_DIR=/private/tmp/yourmove-workflow-native-ojOXIu/compact-workspace \
  /private/tmp/yourmove-workflow-native-pXIhf4/YourMoveWorkflow.app/Contents/MacOS/YourMoveWorkflow
```

Open Opportunities. Inspect the Compact rows and try **Save for Later** and **Work on this** on **Synthetic native workflow**, labeled **Fixture only**. Try **Select multiple** on two fixture rows, then **Done selecting**. History already has one synthetic Applied and one synthetic Closed record. Quit with Command-Q and use the same command to verify that the fixture job retained its placement and preparation.
