# KAN-15 native acceptance

**READY FOR OWNER VERIFICATION.** The agent completed the essential workflow on macOS 27.2.1 in Wails 3.1.0. Owner acceptance is pending.

The ad-hoc signed candidate at `/private/tmp/yourmove-workflow-native-1k0NNP/YourMoveWorkflow.app` embeds application source `91704b879a102525ee92b67b1ecce28c86f40268`. Its executable SHA-256 is `047e0e4f4893705f551c1ee7ffa1eb17a7dc2e95ca7106e5dc5cc06a7fad0b55`. The build harness verified the signature. The source build reported a dirty tree because the native screenshots were then untracked. The evidence commit adds only documentation and screenshots.

Use Terminal to launch the signed executable with the prepared synthetic workspace.

```sh
YOURMOVE_DATA_DIR=/private/tmp/yourmove-workflow-native-q5JWXo/workspace \
  /private/tmp/yourmove-workflow-native-1k0NNP/YourMoveWorkflow.app/Contents/MacOS/YourMoveWorkflow
```

1. Open **Opportunities**. **Focus**, **Saved for Later**, and **History** are spaced with counts on the buttons. Choose **Select multiple** for one labeled checkbox per row and explicit move actions. **Done selecting** restores the ordinary view. Permanent deletion has a separate disclosure with a typed phrase and confirmation.
2. Open **Synthetic native workflow**, labeled **Fixture only**. Its stage is **Review**. Documents shows `kan15-synthetic-resume.docx` version 1 and the letter `Synthetic saved letter`. Questions shows **[Synthetic fixture] Why this team?** with the saved answer `Synthetic KAN-15 answer for persistence verification.` at revision 1. This question was inserted by the opt-in fixture into the job's saved inspection. It was never captured from a live Ashby posting.
3. Choose **Save for Later** on that row, find it under **Saved for Later**, and choose **Work on this** to return it to Focus. Quit with Command-Q and repeat the same Terminal command. The prepared job should still be in Focus at Review with the same résumé, answer and letter.
4. Open **History**. **Applied** shows **Synthetic History Applied** and **Closed** shows **Synthetic History Closed**. The Applied record is a synthetic local submission record. Use the existing **Fixture NN** rows to check search, sorting and bulk moves. On this Mac, Option-Tab traversed the web controls and Space activated the mode button and checkbox.

| Requirement | Result | Native observation |
| --- | --- | --- |
| Ordinary and deliberate bulk selection | PASS | No row checkbox in the ordinary view. Select multiple revealed one visibly labeled checkbox per row. Done selecting cleared the mode. |
| Single and multiple moves | PASS | Prepared job moved Focus → Saved → Focus at Review. Two rows moved to Focus together, then returned together to Saved on the final build. |
| Deletion safeguards | PASS | Separate disclosure exposed the typed phrase and explicit checkbox. Changing selection cleared the checked confirmation. No delete was submitted. |
| Dense search, sort and stage filtering | PASS | Searched the 42-job workspace, selected Company A–Z on the 41-row Saved list, and exercised both History stage filters. Sorting was observed at the control level. The bulk rows shared a company, so this run did not prove ordering between different companies. |
| Populated Applied and Closed History | PASS | The two synthetic records appeared in History and each filter showed only its matching record. The local submission count remained one. |
| Mixed batch and duplicate placement | PASS | 42 lines yielded 40 added, one duplicate, one invalid. Only invalid input remained. A corrected retry cleared the input. A separate duplicate of a Focus job left it in Focus. |
| Failed request | PASS | A temporary trigger on the isolated fixture induced a write error. Both URLs stayed in the textarea, no success counts appeared, and neither row existed in SQLite after shutdown. Removed the trigger before handoff. The error still renders as a JSON string. |
| Prepared job and restart | PASS | Same executable and same workspace reopened without reseeding. Job 1 stayed in Review and Focus with résumé ID 1, answer revision 1 and the selected saved letter. SQLite quick check passed. |
| Normal and narrow layout and essential keyboard | PASS | Normal and left-tiled native layouts inspected. Option-Tab and Space entered bulk mode and selected a row. Final spacing and removal of the redundant visible stats were captured at normal width. The final CSS-only spacing change was not re-observed at the narrower width because the window tool timed out. |
| Real employer Capture and Fill | UNVERIFIED | Existing two-job controlled browser test passed. Live employer confidence testing belongs to KAN-17. |

The full workflow used build `8231247fe2ca887bd3c3c6fa658fea76ea74781e`, executable SHA-256 `e283cab11d7570b9dd24bc9f03bc71d9893bc1babd7bf00f50afe3f536f79feb`. Only the view-button gap and duplicate visible stats changed afterward. Normal and bulk screenshots use the final build. Other screenshots use the full-workflow build. The initial attempt opened outside the synthetic workspace and was closed without an acceptance action. A subsequent workspace-reuse attempt surfaced a real-looking résumé filename, so qualification continued on a fresh fixture with the verified synthetic SQLite file open.

Independent focused review covered selection, deletion, imports and placement persistence. It found no new blocker. It identified an opportunity to bind deletion confirmation to the exact selection inside the submit handler as an additional safeguard. The native cancellation check passed. The final presentation-only changes were self-reviewed. TypeScript, the production build and focused Go checks passed. The harness used development signing.

- [Normal Opportunities](opportunities-normal.jpg) and [bulk selection](opportunities-bulk-selection.jpg)
- [Mixed import](import-mixed-result.jpg) and [unexpected request failure](import-request-failure.jpg)
- [Prepared job after restart](prepared-after-restart.jpg), [Applied History](history-applied.jpg) and [left-tiled layout](opportunities-narrow.jpg)
