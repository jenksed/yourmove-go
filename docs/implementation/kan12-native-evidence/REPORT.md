# KAN-12 native observations — 2026-11-17

> Prior sections record the earlier partial run (dirty pre-decision build).
> The completion pass below qualifies the committed revision 5ff1641 and
> resolves the remaining gaps. Still NOT Done or OWNER-ACCEPTED; the owner
> decides acceptance.

## Identity and chronology

Assigned branch t3/kan12-questions-workflow, HEAD 177e46fc09a8357004c804cc09a7f8c45154e496. Initially clean. Final HEAD unchanged; dirty qualification-only fixture modification, new qualification seed helper, and this evidence directory. Parent checkout was not modified. No push, merge, external tracker writes or subagents.

Current build: existing scripts/build-react-native-fixture.sh, tags desktop,qualification, ad-hoc signed and codesign verified. Binary `/private/tmp/yourmove-react-native-QwCaSv/YourMoveReactFixture.app/Contents/MacOS/YourMoveReactFixture`, SHA256 `86faf4b26d127f876c9cd6622ed6bc340b7a24a12dfbe2ce2858f68f9dce7014`. Workspace `/private/tmp/yourmove-react-native-QwCaSv`. Source is HEAD plus the two qualification-only changes retained here, not a pristine production build. macOS reported 27.2.1, arm64, Go 1.27.3, Wails 3.1.0.

Retrospective baseline comparison: source archive d46a2d1f943395dd3ff43e9d622dfb3b89dadfd8 at `/private/tmp/kan12-baseline-source`, same qualification adjustments, baseline frontend rebuilt via npm run build. Existing qualification script built/signed binary `/private/tmp/yourmove-react-native-9WkG5Z/YourMoveReactFixture.app/Contents/MacOS/YourMoveReactFixture`, SHA256 `cc129b83b9895d870fd9a13d184cc96cf8b1fa28b6c1c270f8bfe089ba10b0b0`; workspace its enclosing root. Archive has no .git, so script's revision output is blank; source provenance comes from git archive command. An earlier intermediate baseline build containing copied current dist was never launched and is excluded. Baseline was observed AFTER initial current interactions; this does not satisfy a chronological before-code baseline. Current binary was reopened afterward on its same workspace.

## Agent-observed native behavior

All interactions used actual Wails window via CUA native accessibility and keyboard. Screenshots are native window captures, not browser previews.

- Edited saved first answer using keyboard. Dirty Next preserved question and draft, showed shared guidance. Explicit navigator Save changed displayed revision 1 to 2. Next then Previous restored exact edited saved text.
- Current full Previous Writing preview displayed Acme Robotics, Backend Engineer, revision 1. Use as starting draft populated unsaved editor. Added unsaved sentence; Keep my draft preserved it, and a subsequent Replace editor text removed it in favor of source text.
- Edited saved second answer. Dirty Previous preserved question 2 and text. Selector Up/Return also retained question 2 and draft with shared guidance.
- Baseline dirty Next was disabled; current dirty Next accepted attempted navigation and displayed guidance. Baseline full preview lacked source context; current included it.
- Controlled View Application changed to Return to Application and watching capture state. Existing qualification factory binds this to loopback; no real employer action or submission performed. Review job workspace navigation worked.
- Capture subsequently replaced current question list with fixture's actual single Why this team? control. On reopening the same current executable/workspace, original answers remained under Preserved writing for changed questions, including revision 2 and exact edited first answer. This is expected fixture mismatch, not evidence of lost answers.
- Long answer keyboard entry wrapped inside editor. Screenshot native-competing-save.png shows navigator Save and editor Save simultaneously, both with identical Save for this application only labels; no Save to Answer Library on this newly captured unsaved question. Earlier saved questions also exposed Save to Answer Library. This establishes competing native controls for C discussion; no styling intervention or user confusion claim is justified by this alone.
- Offscreen AX clicks sometimes scrolled first; repeated after fresh observation. One stale click produced AXError.invalidUIElement after successful save; refreshed tree confirmed revision 2. Native automation was available.

## Corroboration and limitations

Read-only SQLite query after dirty interactions and again at end: first answer revision 2 with only explicit saved suffix ` Native unsaved edit.`; second revision 1 with original text. No library draft, replacement text, or second-answer unsaved suffix persisted. No concurrent SQLite writer introduced.

Focused changed-fixture check `go test -tags qualification ./internal/app -run '^TestNativeReactFixtureGuardAndRestart$' -count=1` passed. Build/signature passed. No redundant full suite rerun. Earlier automated suites/review were user-reported, not rerun evidence here.

NOT VERIFIED / unresolved:

- Ranked Previous Writing search: browsing showed both entries, but searches Acme and reliability returned No ranked matches (Return and Search button attempted). Seed/search compatibility needs investigation; no production regression attribution established.
- Narrow native window: two corner-drag attempts left screenshot dimensions unchanged. Thus narrow layout is NOT VERIFIED. Long question wrapping observed at normal size; long answer scroll/caret precise positioning only partially exercised (CARET appeared at end despite Home attempt).
- No pristine current committed-build qualification (qualification seed changes are uncommitted); no release/distribution acceptance.
- Controlled browser session launch observed from Wails; employer browser contents not independently captured. No automated Fill or submission requested/performed.

Native apps launched for this run were quit via Cmd-Q. Synthetic workspaces and evidence remain. Remaining gaps are incomplete work, not an established external blocker. Parent should continue targeted search/narrow qualification before declaring verification complete.

## Evidence

- [Current dirty navigation](native-dirty.png)
- [Current controlled View Application](native-view-application.png)
- [Current competing saves and long answer](native-competing-save.png)
- [Retrospective baseline dirty state](baseline-dirty.png)
- [Retrospective baseline preview](baseline-preview.png)
- [Final saved-question hierarchy, app primary](kan12-final-save-hierarchy.png)
- [Final ranked search, poison message](kan12-final-ranked-search.png)
- [Final full preview](kan12-final-preview.png)
- [Final use as starting draft](kan12-final-use-as-draft.png)
- [Final narrow 700px layout](kan12-final-narrow-700.png)
- [Final narrow keyboard entry](kan12-final-narrow-keyboard.png)
- [Final saved revision 2](kan12-final-saved-rev2.png)
- [Final controlled View Application](kan12-final-view-application.png)
- [Final restart preserved writing](kan12-final-restart-preserved.png)

## Completion pass — committed revision 5ff1641

Code commit `5ff16416e21f55b8833ddcf60e26853c3059ee98` (branch
t3/kan12-questions-workflow): WritingSection application-save primary /
Answer Library secondary (both preserved, no behavior change); KAN-12
fixture seed gated behind `YOURMOVE_QUALIFY_KAN12=1` on fresh workspaces
only (no reseed on restart, no silent repopulation of deleted answers);
seed returns errors instead of panicking; guard-test pins default fixture
behavior (no seed, preview untouched); focused opt-in test covers seed,
ranked-search compatibility, restart preservation, and no-repopulation.

Build: scripts/build-react-native-fixture.sh, tags desktop,qualification,
ad-hoc signed and codesign verified by the script. Binary
`/private/tmp/yourmove-react-native-Rz9Ejb/YourMoveReactFixture.app/Contents/MacOS/YourMoveReactFixture`,
SHA256 `1195ea17601986113e16069cdfe01a7ca59d915679c1e026b7c9ef9bf91c07b3`.
Workspace `/private/tmp/yourmove-react-native-Rz9Ejb` (fresh synthetic,
`YOURMOVE_QUALIFY_KAN12=1` at launch). App log reports
vcs.revision=5ff16416e21f55b8833ddcf60e26853c3059ee98,
vcs.modified=true (only untracked evidence files present; no tracked
modification at build). macOS 27.2.1 arm64, Go 1.27.3, Wails 3.1.0.

Before/after for C: the before hierarchy is the committed HEAD state
(library primary at WritingSection.tsx:987, application secondary) plus
the earlier native-competing-save.png showing both save controls
coexisting. No evidence contradicted making application saving primary,
so the edit proceeded. After: the saved-question row shows Save for this
application only as the filled primary action and Save to Answer Library
as secondary; both actions intact, dirty-navigator duplicate app save
unchanged.

Agent-observed on the committed build, all through the real Wails window
via Accessibility AX actions and native keyboard plus screencapture:

- Ranked search: query `poison message` returned a ranked match (top hit
  Describe a hard production incident you resolved, revision 1) where
  `Acme` / `reliability` alone returned none. Root cause is ranking
  semantics, not fixture corruption: single terms score below the
  ranker gates (score < 5, one shared term, no phrase overlap), while
  two-term/phrase queries pass. Fixture compatibility proven by the new
  focused test asserting `poison message`, `reliability end`, and
  `join new engineering team` all top-hit the intended entries.
- Preview then Use as starting draft populated the editor with the
  source text as an unsaved draft with origin/provenance notice.
- Primary Save for this application only saved revision 2 with notice
  Saved for this application only; DB shows answers rev 2 + rev 1 and
  library still 2 rows: no unexpected revisions, no library draft.
- Controlled View Application opened only the loopback synthetic
  employer form (Local submissions: 0, no manual submission); nothing
  filled or submitted. Closing it left a capture observation that
  replaced the live list with the fixture's Why this team? control;
  saved answers moved under Preserved writing, none lost.
- Narrowest supported width: AX set size to 700x750 honored exactly
  (MinWidth 700, MinHeight 550 per desktop composition). At 700px the
  navigator, dirty notice, long question heading, editor, and both save
  buttons stack full-width with no breakage. Textarea shows a visible
  keyboard focus ring; End-then-type appended at the caret with wrapping
  inside the editor.
- Restart: full quit, then reopened the same executable on the same
  workspace without reseeding. Preserved writing shows revision 2 with
  the exact saved text and revision 1 intact; row counts still 2 and 2.

Checks run: frontend `npm run build` (tsc + vite) clean;
`go test -tags qualification ./internal/app` for the fixture guard,
KAN-12 opt-in, and Kan12/Questions focused tests pass;
`go test ./internal/answerlibrary` passes; `go build ./...`
(default tags) clean; `git diff --check` clean. Full suites were
already green per prior independent review and were not rerun.

NOT VERIFIED: distribution/release acceptance; employer-browser Fill and
submission (never attempted, by design); baseline chronological
comparison unchanged from the prior section.

Qualification apps were quit via Cmd-Q (confirmed no remaining
process). Parent's own Chromium fixture session was never touched.
Evidence screenshots in this directory; synthetic workspaces remain.
