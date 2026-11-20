# KAN-16 native evidence — HOLD (native session LOCKED)

## Status

HOLD. Native GUI not qualified. All native scenarios UNVERIFIED. No readiness claim. No push/merge/deploy/Jira changes.

- Worktree: `/Users/jenksed/.t3/worktrees/yourmove-go-kan9-integration/t3-c89fb854`
- Branch: `t3/kan-16-closure-check`
- HEAD at finalize start: `20ac10a7cbfb3ac24e56fd9ba727f700431846ac`
- Baseline: `b0286d3e649e876183210eb65f4d0f85a843ee39` (parent repo HEAD, verified clean at finalize)
- Dirty at finalize start (3 files, 67 insertions): `internal/app/posting_availability.go`, `internal/app/posting_closure_preflight_test.go`, `internal/platforms/greenhouse.go`
- Parent baseline untouched. No broad pkill. Only owned server stopped: PID 61295 (`/private/tmp/kan16-webmode/kan16-web` on 127.0.0.1:3017). Reported native PID 58674 already absent at finalize (no action).

## Actual changes (vs b0286d3)

Committed KAN-16 (`b0286d3..20ac10a7`, 27 files, +863/−19): closed-posting preflight, single-click checking, persistent closure to History, Applied preservation, enrichment/browser guards, Opportunities/Continuation UI wiring.

Focused repair (dirty, +67/−1, committed separately at finalize):

- `internal/platforms/greenhouse.go` (+10): new exported `IsClosedBoardRedirect` wrapping existing `isGreenhouseErrorBoard` (https-only, no user/port, host `job-boards.greenhouse.io`/`boards.greenhouse.io`, single path token, `?error=true`). Comment states it does not extend posting ownership.
- `internal/app/posting_availability.go` (1 line): preflight gate `!browser.OwnsPostingURL(url,url) && !platforms.IsClosedBoardRedirect(url)` → unknown; otherwise falls through to provider fetch mapping `ErrClosed` → closed. Bare board URLs stay unowned for browser/transfer scope.
- `internal/app/posting_closure_preflight_test.go` (+56): `TestClosurePreflightUnretiredBoardRedirectCloses` — unretired `.../kan16redirect?error=true` reports closed/persistent (StageClosed, SourceStatus closed, Attention history, prep preserved); control bare `.../kan16redirect` stays unknown and active.

## Native matrix (all UNVERIFIED — verdict HOLDS)

Native session LOCKED, black captures, no clicks. Prior verdict MUST HOLD.

| Scenario | Result |
| --- | --- |
| job5 closed single-click + résumé1/saved answer/letter preserved | UNVERIFIED native; API-level closed observed pre-finalize |
| job6 Applied submission immune | UNVERIFIED native; API-level applied-preserved observed pre-finalize |
| job2 uncertain unsupported | UNVERIFIED native |
| job3 OPEN (LIVE Greenhouse — nondeterministic, see limitations) | UNVERIFIED native |
| job7 unretired error-board manual link (`ownsPosting=false`), React single-click | NOT demonstrated |
| Same-executable same-workspace native restart durability | NOT observed (browser/API persistence only, not native restart) |

Black capture `/private/tmp/kan16-evidence-01-launch.png` (3420×2014, 2026-11-18) preserved in record ONLY as blocker evidence — never a native pass.

## Independent review (actual final diff)

Reviewer (read-only, fresh on actual final file states; `git diff` arg path blocked by tool allowlist so final-state inspection used): **ACCEPT, no blocking defect.**

- Deterministic gate: `posting_availability.go:61` only bypasses ownership gate for provider error-board redirect; `postingCheckFetch` maps provider `ErrClosed` → closed; only closed retires.
- Provider authority: `greenhouse.go:22-33` strict host/scheme/query checks; consistent with `platforms.go` intake/enrichment `ErrClosed` and `handlers.go fetchPostingDetails` mapping.
- No ownership expansion: `IsClosedBoardRedirect` used only at preflight gate; `browser.OwnsPostingURL` callers unchanged; bare board stays unowned for browser/transfer.
- No silent overwrites / Applied preserved: `jobs.go applyPostingRefresh` guards (`closed` only when not applied and no submissions; manual fields only on `retrieved` with `*_manual=0`); `ensurePostingOpen` early-returns Applied/Closed/submitted; availability API persists only for open unsubmitted jobs.
- Test pins redirect-closes + bare-board-unknown with prep preservation.
- Prior TOCTOU noted nonblocking (each launch/transfer re-invokes `ensurePostingOpen`; worst case surfaces on next preflight, not unsafe transfer).
- Reviewer run hit step cap after delivering verdict + file:line evidence; remaining byte-split archival done at finalize via `git diff --stat` (67 insertions noted above). No code fix required; no second review cycle consumed.

## Checks

- Original (pre-repair, per prior agent): full `internal/app` tests PASS, typecheck PASS, frontend build PASS.
- Repair (per prior agent): full `internal/app` tests ~25s PASS, `platforms` tests PASS, `go vet` touched PASS.
- Finalize focused (this run, worktree): `go test ./internal/app/ -run TestClosurePreflightUnretiredBoardRedirectCloses -count=1` → PASS (`ok 0.557s`); `go vet ./internal/app/ ./internal/platforms/` → clean (no output).
- API observations carried forward (not re-run per bounded budget, not native): job5 closed with résumé/saved answer/letter preserved; job6 Applied immune; job2 uncertain unsupported; job3 OPEN via LIVE posting (nondeterministic); job7 unretired error-board manual link `ownsPosting=false`.
- Baseline verify: parent repo HEAD still `b0286d3`, `git status` clean (only ahead-branch marker, no dirty entries).

## Artifacts

- Candidate build (dirty fixed source): `/private/tmp/kan16-candidate-002/YourMove.app`, ad-hoc signed, binary SHA256 `fd29d2c3968c422e10b78e2644a39ac26b0e5c22883deb865a87b3bc5000a6d6`.
- Stable owner copy (outside Git, no binary committed): `/Users/jenksed/YourMove-KAN16/candidate-002/YourMove.app` (ditto copy, SHA256 verified identical).
- Build-source relationship: built from dirty fixed source (committed `20ac10a7` + 67-line dirty repair). Do NOT claim embedded SHA equals final evidence commit — re-sign/rebuild from clean tree for any distribution claim.
- Synthetic workspace (isolated, no real data): `/private/tmp/kan16-workspace-001` (`jobs.db`, `resumes/kan16-synthetic-resume.pdf`).
- Owner launcher: `docs/implementation/kan16-native-evidence/launch-kan16-synthetic.sh` (guards port/writer, forces synthetic dir, refuses real data).
- This report: `docs/implementation/kan16-native-evidence/REPORT.md`.

## Fixture job mapping (`/private/tmp/kan16-workspace-001/jobs.db` at finalize)

1. CLOSED fixture, tailscale board closure — `closed/closed` — `https://job-boards.greenhouse.io/tailscale?error=true`
2. UNCERTAIN fixture, unsupported site — `review/unsupported` — `https://example.com/jobs/kan16-uncertain-001`
3. OPEN fixture, tailscale posting — `review/retrieved` — `https://job-boards.greenhouse.io/tailscale/jobs/4750001005` (LIVE — nondeterministic)
4. CLICK-CLOSED fixture, fresh unretired — `closed/closed` — `https://job-boards.greenhouse.io/acme?error=true`
5. CLICK-CLOSED deterministic fixture — `closed/closed` — `https://job-boards.greenhouse.io/acme?error=true` (saved answer + cover letter rows present; résumé association per prior API observation)
6. APPLIED preserved fixture — `applied/closed` — `https://job-boards.greenhouse.io/acme?error=true` (submission row present → immune)
7. CLICK-CLOSED UI demo, unretired — `review/not_checked` — `https://job-boards.greenhouse.io/kan16click?error=true` (`ownsPosting=false`, React single-click NOT demonstrated)

## Limitations (must stay open — no readiness claim)

- Unresolved manual-path external View Application bypass needs investigation.
- Nondeterministic open fixture (job3 uses LIVE Greenhouse posting) needs a deterministic owned-posting fixture.
- Native GUI core paths (single-click closed check, Applied immunity display, uncertain/open display, restart durability) need an unlocked console + deterministic owned-posting fixtures.
- Browser/API persistence was same-binary same-workspace only — not a native quit/reopen observation.
- Candidate-002 hash identifies the dirty-source build only; final clean-tree commits have different SHAs by construction.

## Owner next (smallest)

1. Unlocked console + deterministic owned-posting fixtures (replace LIVE job3).
2. Run `launch-kan16-synthetic.sh`, single-click job5/job7, confirm History/preservation without external submit.
3. Investigate View Application bypass, then re-qualify native core paths + same-executable/same-workspace restart.
