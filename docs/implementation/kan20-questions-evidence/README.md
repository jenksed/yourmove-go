# KAN-20 Questions-selector experiment — evidence record (Outcome A)

Owner decision (2026-11-19): **ACCEPT, Outcome A — existing tooling sufficient.**
No `QuestionsObserver` is retained; no production implementation was added.

## Source

- Accepted baseline: `3a30abc1d182944551998520508bc34ec7e3ca3d` (`v0.3/develop`).
- Experiment branch: `experiment/kan-20-questions`, reviewed commit `502d697`.
- Experiment diff vs baseline: exactly one file,
  `internal/app/questions_selector_qualification_test.go` (176 lines,
  `//go:build qualification`, opt-in via `YOURMOVE_QUALIFY_BROWSER=1`).

## Synthetic scenario (`TestQuestionsSelectorDiscardRecovery`)

Seeded KAN-12 fixture, synthetic opportunity with two saved answers (A, B).
Through the real rendered combobox (CDP mouse events at measured coordinates):

1. Initial selection B verified (position, label, editor value, saved-revision status).
2. Select A through the rendered UI; clean selection of A verified.
3. Edit A without saving; dirty alert (`[role="alert"]`) and Discard control appear.
4. Attempt to select B; the dirty guard refuses: A stays selected, draft intact,
   fresh refusal notice shown.
5. Discard restores saved A; selecting B afterwards succeeds.
6. Every step re-queries SQLite `application_answers`; any change to saved
   answers or revisions fails the test.

## Results

| Check | Outcome |
| --- | --- |
| Three clean controlled-Chromium runs | PASS / PASS / PASS |
| Independently repeated run (orchestrator, separate invocation) | PASS |
| Deliberate guard regression (removed 4-line dirty check in `WritingSection.tsx`) | FAIL at the `refused B selection` assertion, as required |
| Restored guard | PASS |
| Focused suite (`TestKan12QuestionsDirtyGuardAndPreviewContext`, selector test, `TestReactQuestionsUnifiedList`) | PASS |

## Boundaries and cost

- No observer/bridge code exists in the tree; no production file changed.
- Native Wails execution is NOT VERIFIED by this experiment (controlled browser
  only); prior native evidence, if any, is separately attributed.
- Maintenance cost: one 176-line qualification-tagged test; no new harness.
- Owner active verification minutes were not measured; the provisional ~30%
  improvement threshold is not demonstrated and no productivity claim is made.
- Future agent-facing UI hypotheses remain out of scope.
