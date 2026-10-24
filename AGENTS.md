<!-- graft:start -->
## Graft — repo context graph

This repo is indexed in `graft/`: small linked markdown nodes that explain each
system and carry exact file:line spans, kept in sync with the code through git.

For ANY task here — understanding how something works, finding where code lives,
or scoping a change — get context from the graph before grepping or opening
source files. Re-ask freely (it's cheap) and reuse literal identifiers you
already have (symbol, error string, file name) as the query. New to this repo?
Run `graft map` first — a token-budgeted orientation (dir clusters, hubs,
hotspots), no LLM, no key.

- Run `graft ask "<your question>" --source` → ranked nodes with the relevant
  code spans inlined (each hit's ≤8-line crux by default; `--full` for whole
  definitions when the crux isn't enough). Match the tool to the task shape:
  for understanding or editing, the top node IS the answer — cite its
  `covers:` file:line spans and edit straight from `--source`. For
  exhaustive tasks ("every occurrence / every caller of this pattern"), ranked
  results are top-N, not complete — run `graft grep "<literal>"` instead
  (exhaustive over indexed files, grouped by enclosing symbol), falling back
  to raw `grep -rn` only for unindexed files.
- `graft skeleton <file>` → every definition's signature + span, ~10× cheaper
  than reading the file; use it to skim an API surface.
- `graft callers <symbol>` gives precomputed, exact edges — who calls this.
  Add `--direction out` for what it calls, or `--depth N` to walk
  transitively for the full blast radius. For structural questions, skip
  ranking and use this directly.
- Or browse: `graft/INDEX.md` lists every node; follow the links.
- Monorepos and folders of multiple repos rank fairly across sub-projects —
  hits carry `[scope/]` labels naming which one they're from. Narrow with
  `graft ask "<task>" --in <scope>/` once you know where you're working.

If a returned span is truncated ("+N more lines"), open the file at that exact
range before finalizing. Only open source files when a node genuinely lacks a
needed detail, and then at the exact file:line the node points to — never
re-read whole files.

After big code changes, refresh the graph with `graft build` (deterministic,
no API key, $0).
<!-- graft:end -->

## Application UI design decisions

Before changing a user-facing template or its styles, read `docs/design/DESIGN_SYSTEM.md` and `docs/design/RULES.md`. Start with R1 (the current decision) and R6 (existing artifacts before their generators); apply R7–R10 where control parity, prose width, duplicated copy, or overcrowded object lists are relevant. Prefer scoped composition changes over a new component library. Preserve the existing Go POST handlers, Wails redirect behavior, consent boundaries, and explicit submission confirmation.

## Backlog intake and prioritization

The maintained engineering backlog is [`docs/BACKLOG.md`](docs/BACKLOG.md). It records unfinished Phase 2 items, PR closeout, refactoring, beta-release gates and deliberately deferred native automation work. **Read it before selecting work whenever the user asks for backlog items, priorities, next tasks, cleanup/refactors or release readiness.** Also read `docs/RELEASE_POLICY.md` / `docs/RELEASE_CHECKLIST.md` for release choices, and the linked acceptance evidence for any proposed gate.

1. Verify the current branch, PR/CI and relevant implementation first: backlog priorities and snapshot SHAs can become stale. Respect unrelated worktree modifications.
2. Establish the user's **current objective** (PR closeout, refactoring, beta qualification or later research). Filter by stage, dependencies and actionable status; recommend the top **2–3** with user impact, evidence and acceptance conditions. P0 safety/data integrity defects outrank all work. Do not blindly sort by P1 or expand a focused request into an unrelated release effort.
3. Distinguish demonstrated failures, code-review hypotheses and **NOT VERIFIED** host testing. Do not present a blocked automation harness as a YourMove product defect or convert Chromium/Go tests into native Wails acceptance. Unverified core workflow remains a beta release gate, even if separately accepted for source merge.
4. Work on one bounded, owner-authorized item at a time, in a suitable branch. Preserve existing Go handlers/SQLite, consent and explicit manual employer submission boundaries. Do not autonomously merge, release, delete user data or submit employer applications.
5. After completing an item, update `docs/BACKLOG.md` with its status, commit/PR and tests/evidence; keep deferred work discoverable. Use a separate PR for unrelated refinements rather than growing a closeout diff.

If the user asks to **refactor**, favor the maintainability items after Phase 2 is merged and avoid creating a new framework or browser stack. If the user asks for a **beta release**, evaluate the actual frozen artifact against `docs/RELEASE_POLICY.md` regardless of a green CI or source-merge decision.
