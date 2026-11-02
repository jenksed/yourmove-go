# M0 current Overview baseline

**Scope:** JI-001 current app-path characterization at baseline source `5da7ff214ae1e9ace05d47eed9183d37f7a8c754`.

**Data:** The characterization test uses only a synthetic Northstar Analytics posting, synthetic compensation, and synthetic applicant edits. It does not open an employer page, read the maintainer's data directory, or use applicant materials.

## Repository baseline

At M0 start, the repository was on `feature/ashby-provider-finalization` at `5da7ff214ae1e9ace05d47eed9183d37f7a8c754`. The initial worktree had a pre-existing `AGENTS.md` modification and an untracked `docs/implementation/job-intelligence/` directory containing the governing implementation plan and swarm playbook. The agent change to `AGENTS.md` was left untouched (SHA-256 `2fefb7c2cafc0c78632278a5d2a493472c5660bce0d4ed2b7c2132c41cf161a2`). The governing documents were not edited (plan SHA-256 `27fbc5eec6c4c5774ded9386d30474fa4ce91c6de03d5b180fc015bbacf251e7`; playbook SHA-256 `c6ba317e6e260d93e803c129e97f3ae102e8cf966ffd0cfd97cd5683b53dd834`).

## Current source-to-Overview path

1. `POST /jobs` in `internal/app/handlers.go` bounds the form, normalizes the saved URL, checks duplicates before fetching, calls `fetchPostingDetails` / `platforms.FetchJob`, then stores the provider's five canonical metadata strings and source-check state through `storeJobIfNew`. The title supplied by the applicant is marked manual. This path has no per-app fetch injection seam; a successful real Ashby intake could not be exercised here because network access to `api.ashbyhq.com` is blocked.
2. `GET /jobs/{id}` in `internal/app/handlers.go` reads the stored job and composes `jobDetail`'s workspace. `templates/job.html` renders `Job.DisplayDescription()` into the escaped `#job-description` element. The current page presents one text block; it does not render provider HTML.
3. The copy button starts hidden in the template and `static/ui.js` reveals it when JavaScript runs. Its handler copies `#job-description.innerText.trim()`. Without JavaScript, the visible description text remains selectable. `TestTrialCopyDescription` verifies the clipboard payload on a controlled synthetic page. `TestJobOverviewM0ResponsiveCapture` also opened the actual saved-job Overview and clicked its copy button with a stub clipboard; the exact saved description was copied and the UI reported success.
4. Edit details is a native disclosure containing `POST /jobs/{id}/edit`. `editJob` delegates to `updateJob`, which records applicant edits, including empty values, as manual. The Overview's source check is a `POST /jobs/{id}/refresh`; `refreshJob` reloads the saved job and calls `refreshPosting`. The characterization test supplies a synthetic fetch result through the existing `enrichmentFetch` seam and confirms the edited description and salary survive while a nonmanual title refreshes.

## Existing canonical consumers

- `internal/app/application_brief.go::writeJobDetails` exports the complete stored `Job.Description`; it does not use a future rich or focused view.
- `internal/app/cover_letters.go::letterEvidenceForJob` places that stored description in the generation prompt and computes SHA-256 over its exact bytes. The canonical fixture hash in the provider tests pins that description input; this M0 characterization made no generation request.
- `internal/app/application_answers.go::questionKey` hashes the destination, provider, and serialized question `Field`; the description is not part of that identity. M0 changed neither question fields nor saved-answer identities.
- `static/ui.js` copies `#job-description.innerText.trim()` and falls back to manual selection when clipboard writing fails. Controlled Chromium verified both the standalone copy fixture and the actual saved-job Overview button. The stub clipboard captured all 1,310 canonical description bytes and the page reported `Job description copied.`; this did not write to the operating-system clipboard.

## Observed current behavior and usability gap

The controlled Chromium captures show the actual saved-job Overview at both reference widths. At 1280px, the description and role facts appear side by side; its headings and bullets remain plain text without structural emphasis or link targets. At 390px, the long description continues below the viewport and the Overview card header compresses its three items into one row: “Job description” wraps one syllable per line beside the copy and edit controls. The document width equals the viewport at both sizes, so these captures show no horizontal document overflow. The description is 1,310 bytes and the full document heights are 1,281px at 1280px and 2,746px at 390px. Raster captures are retained in the local review archive and excluded from the source PR.

The provider's current `descriptionText` tokenizer converts HTML blocks into plain newlines and list bullets. It does not preserve heading hierarchy, nested-list semantics, emphasis, or link targets. This is visible in the captures: section labels and list entries have no document structure or navigation, and the mobile header gives the reading surface little room before the description starts.

## Viewport and native evidence

| Check | Status | Evidence / limitation |
| --- | --- | --- |
| Production handler/template output | PASS | `TestJobOverviewM0Characterization` runs the protected `newMux` through its actual GET/POST handlers using an in-process synthetic database. |
| 1280px browser capture | PASS | `TestJobOverviewM0ResponsiveCapture`; 1280×900 viewport, 1280px document width, 1,281px document height. Raster output is retained locally and excluded from the source PR. |
| 390px browser capture | PASS | Same qualification; 390×900 viewport, 390px document width, 2,746px document height. Raster output is retained locally and excluded from the source PR. |
| Actual Overview copy button | PASS | Same qualification clicked the page's real control and checked the stub clipboard against the complete saved description plus the success status. No system clipboard was changed. |
| Standalone controlled copy fixture | PASS | `TestTrialCopyDescription` with the repository's real `static/ui.js`; full exact expected payload and status matched. |
| Actual Wails Overview baseline | NOT VERIFIED | No JI Overview interaction was run in Wails. `scripts/qualify-native-workflow.sh` is a separate macOS development flow: it builds a desktop-tag app bundle in a unique `/private/tmp` root, seeds an isolated synthetic workspace through `YOURMOVE_NATIVE_FIXTURE_DIR`, signs and verifies the bundle, and only launches it with `--launch` while setting `YOURMOVE_DATA_DIR` to that workspace. Reading, interaction and same-executable/same-directory restart are manual steps. Existing `docs/implementation/product-experience-implementation/ACCEPTANCE.md` records prior product-experience native evidence and gaps, not this Job Intelligence screen at this candidate. |
| Fresh public Ashby endpoint fetch | NOT VERIFIED | DNS/network access to `api.ashbyhq.com` was unavailable in this run. Official API documentation was reviewed, and the dated Ashby/Linear observations already recorded in the implementation plan were reviewed; no fresh response body was retained. |

The browser qualification serves only the synthetic saved job through a temporary loopback server and uses a separate temporary Chrome profile. It does not contact an employer, read the maintainer's database, or write candidate data. The first sandboxed browser attempt was denied at listener creation; the final opt-in runs used the authorized local-only qualification command below and passed. The actual Wails screen remains unverified.

## Reproduction

```sh
GOCACHE=/private/tmp/yourmove-m0-go-cache go test -count=1 ./internal/app -run '^TestJobOverviewM0Characterization$'
GOCACHE=/private/tmp/yourmove-go-m0-browser-cache YOURMOVE_QUALIFY_BROWSER=1 YOURMOVE_CHROMIUM="/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary" go test -count=1 -tags qualification ./internal/application/browser/chromium -run '^TestTrialCopyDescription$' -v
GOCACHE=/private/tmp/yourmove-go-m0-browser-cache YOURMOVE_QUALIFY_M0_OVERVIEW=1 YOURMOVE_M0_EVIDENCE_DIR="$PWD/docs/implementation/job-intelligence/evidence/m0-overview" YOURMOVE_CHROMIUM="/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary" go test -count=1 -tags qualification ./internal/app -run '^TestJobOverviewM0ResponsiveCapture$' -v
GOCACHE=/private/tmp/yourmove-go-m0-final-cache go test -count=1 ./internal/platforms ./internal/app
```

All four commands passed with exit status 0. The browser commands require a local loopback listener; they were run with permission for this synthetic qualification. The Go build cache was placed under `/private/tmp` because the default user cache path is not writable in the sandbox. Browser: Google Chrome 158.0.8121.0 Canary; retained screenshots: `5cef57ef7000d54e9fa6b8abf4fd3cebffd124ec66c03bba53436c02eba7477e` and `cc3ef1e85aff88f77ca076acd409b189068f7b65d9a538fdc9178dcfd2540807`.
