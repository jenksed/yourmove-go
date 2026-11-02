# YourMove v0.1.0 integrated candidate — agent qualification record

Candidate built 2026-11-02 from branch `feat/v01-integrated-qualification`
(no source changes; all evidence synthetic, isolated workspaces only).

## Exact artifact and environment

| Item | Value |
| --- | --- |
| Release/version/build | 0.1.0 / 1 |
| Source commit SHA | 4952f9d0b980a62f35770d33b653dcb16024e835 |
| Candidate binary ZIP SHA-256 | 88ee80131e868e7b8bea336429b0965789cbb2028cf90e6e412d577fe95783dd |
| Matching source ZIP SHA-256 | 3668fc0e79043b7a83f3f616e94eafa2622c33e63518b566f2a9766c380 |
| Candidate bundle | Built to `/tmp/ym-candidate-v01-001/candidate/YourMove.app` (outside source tree; not retained as the deliverable — rebuild via `release/macos/prepare.sh`) |
| macOS / arch | 26.5.2 / arm64, Go 1.27.2 |
| Browser resolution | No stable Chrome; Playwright-cached Chromium `chromium-1200` present; `YOURMOVE_CHROMIUM` override supported |
| Distribution audience | Controlled local candidate only (ad-hoc signature, NOT notarized) |
| Tester and date | Spark agent, 2026-11-02 |

Build: unmodified `release/macos/prepare.sh` (runs `doctor.sh`, both Go
suites on an isolated workspace, `build.sh`, ZIP packaging, extraction +
signature re-verification). Frontend `tsc --noEmit && vite build` clean
(52 modules; `index-DeDb9ruZ.js` 379,698 B, `index-BasTmfff.css` 37,130 B).

## Agent-observed evidence (same-commit Go behavior, synthetic data)

- Candidate process launches independently of the repo/dev server, stays
  alive, creates `jobs.db` + `resumes/` in the isolated workspace, logs
  `YourMove Wails compatibility run starting at /jobs`. No GUI interaction
  was observable by the agent.
- Embedded bundle confirmed two ways: candidate Mach-O contains the exact
  Vite asset hash (`index-DeDb9ruZ.js`) and root marker; same-commit server
  returns HTTP 200 for `/jobs` (React index), `/app/assets/*.js` (379,698 B)
  and `/app/assets/*.css` (37,130 B).
- Intake → résumé → association → review → submit → history all pass:
  `POST /api/opportunities` 201 then duplicate 200 (`duplicate:true`);
  docx import 201; association 200; `submit` without `confirm:"yes"` → 400;
  review token → submit 200 `recorded:true`; history count 1 with résumé
  SHA-256 `cfdeb463…cc1b8e2df3f`. Loopback-only URLs; no employer contacted.
- Restart on the same workspace: job, résumé association, `stage=applied`,
  submission count, history entry, and notes byte-identical.
-Absent-provider behavior is clean: AI letter generation with no provider →
  422 naming explicit consent/transmission; managed inspection of a
  non-ATS synthetic URL → 422 scope message; submit on an applied job's
  browser path → 409. Routine organization use unaffected.
- `go test ./...` and `go test -tags desktop ./...` pass (isolated
  `YOURMOVE_DATA_DIR`), including the existing stop/restart persistence and
  Chromium-control suites.
- Distribution: `file` → Mach-O arm64; `codesign --verify --strict` PASS
  (ad-hoc, `com.yourmove.go`); ZIP extract + signature re-verify PASS via
  `prepare.sh`. `spctl --assess --type execute` → **rejected** (expected for
  ad-hoc; Developer ID identity absent on this machine).

## NOT VERIFIED — owner walkthrough required

1. Rebuild: `bash release/macos/prepare.sh /absolute/new/output-dir` from
   commit `4952f9d`; confirm `SHA256SUMS` matches the table above.
2. Copy `YourMove.app` to `/Applications` on a clean Mac account; open via
   Finder and record the actual security prompt (ad-hoc quarantine).
3. In the Wails window with an isolated synthetic workspace: add a job from
   a URL, review the imported opportunity, import a synthetic résumé via the
   native picker (also exercise Cancel), associate it, save an answer and a
   cover letter.
4. Open a supported employer posting (Ashby/Greenhouse/Lever) in managed
   Chromium, authorize one transfer, submit personally on the employer site,
   record completion in YourMove; confirm YourMove never auto-submits.
5. Quit and reopen the same `.app` on the same workspace; verify the job,
   association, answers, letter, and history are intact.
6. Record outcomes in a copy of `release/macos/ACCEPTANCE.md` and file the
   `SHIP RESTRICTED` / `HOLD` decision in `release/decisions/v0.1.0.md`.

## Gate disposition (agent-proposed, owner to confirm)

| Gate | Disposition | Evidence / exclusion |
| --- | --- | --- |
| Frozen candidate | PASS | Commit + ZIP SHA-256 above; `prepare.sh` manifest cmp + `SHA256SUMS.local` |
| Data integrity | PASS (Go-state level) | Restart byte-identity; duplicate/intake guards; existing restart suites green |
| Applicant authority/privacy | PASS (Go-state level) | confirm guard 400; review-token flow; local-only declaration; no employer contact |
| Core workflow | RESTRICTED | Go-state journey passes; **native Wails/Chromium interaction NOT observed by agent** |
| Distribution | RESTRICTED | Local ad-hoc install only; Gatekeeper rejects; no Developer ID/notarization |
| Recovery | PASS (partial) | Clean 4xx/422/409 on absent AI, unsupported inspection, completed-job browser open |

No source changes were required: no incompatibilities between the React
bundle, Job Intelligence providers, managed Chromium path, and macOS
packaging were found, and no release-blocking defect was observed.

## Hotfix (post-candidate): Documents row layout

Owner-reported broken Documents screen: long underscore filenames rendered
one character per line. Cause: `.resume-row` was a non-wrapping flex row
while `details.document-manage` carried `flex-basis: 100%`, crushing
`.resume-info` to ~zero width so `overflow-wrap: anywhere` broke every
character. Fix (commit below): `flex-wrap: wrap` on `.resume-row`, so the
manage disclosure takes its own full-width row. Reproduced before and
verified after in an isolated render with the production class set
(horizontal name, right-aligned Download, full-width Manage row).
Frontend `tsc + vite build` clean (CSS `index-Dn7B4p-b.css`). CSS-only;
no Go behavior changed. The candidate above must be rebuilt via
`release/macos/prepare.sh` before owner walkthrough.
