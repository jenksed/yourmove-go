# Checkpoint 3 — Application workspace and clipboard brief

## Workspace composition

The Prepare page now opens with a compact readiness summary for the selected resume, approved profile, questions needing input, locally prepared answers, employer-side/manual work, cover letter, and browser session. The summary does not repeat profile values or resume metadata. It links directly to the selected question workbench and to a saved letter when one exists.

The existing combined Fill/Continue form remains server-owned and keeps its POST route, hidden exact-scope fields, hash/revision checks, authorization text, and final action. The transfer review is shorter: contact facts are grouped, the saved cover letter text is a named disclosure, and diagnostics remain on the inspection/troubleshooting surface. The Fill action remains after its transfer review. Submission recording remains a separate confirmation after employer-side submission.

The question editors and answer assistance remain the Checkpoint 2 workbenches. No second editor or answer-storage path was added.

## Clipboard formats and selection

`Copy application brief` produces:

```text
JOB APPLICATION BRIEF

Company: …
Position: …
Compensation: …
Compensation source: …
Posting: …

JOB DESCRIPTION

[Full saved description]

QUESTIONS NEEDING MY INPUT

Source: …
[Partial inspection note, when applicable]

1. [Original employer wording]
   Required|Optional
   Published options:
   - [Observed option]

END OF BRIEF
```

Compensation and description come from the saved job row. Missing salary is “Not specified”; missing description is explicitly unavailable. The formatter copies the full description without a truncation cap.

`Copy questions` returns the numbered question section only. `Copy job details` returns employer, title, saved compensation/source, posting URL, and full description. The collapsed options disclose a scope selector: unanswered actionable questions by default, or all actionable question wording without answers. Supported published select/radio options are included. The plain-text view link follows the selected scope.

Question selection uses one inventory. A live inspection is selected only while the managed browser session is active for the saved job and its latest valid observation ID matches the prepared live snapshot. Otherwise, the latest valid early discovery for that posting is used. The inventories are never merged or deduplicated by label. The output names the source as a current browser-observed form, read-only application preview, or published question definitions; partial inspection warns that later or conditional questions may be absent.

## Privacy and operational behavior

The export is a read-only `GET` from saved job and inspection data. It never calls a provider, opens/reloads an employer page, writes a database record, records submission, or transmits text externally. Clipboard writes happen only after a user click. The brief is not placed in a URL. The formatter takes question wording, required status, safe published options, and a boolean local-answer state; saved answers and unsaved editor contents are never passed to it. The existing Checkpoint 1 question triage excludes profile-resolved, sensitive, consent, employer-only, and unsupported controls from the actionable export.

Clipboard success and failure are announced. If Clipboard API and `execCommand` are both unavailable, the response text is placed in a selectable textarea. A plain-text server view and download form remain available without JavaScript.

## Qualification and screenshot evidence

The opt-in controlled Chromium test uses only synthetic loopback data. It exercised clipboard success, denied clipboard with selectable fallback, question and details formats, scope changes, saved-cover-letter navigation, exact Fill action presence, partial live inspection, uncertain transfer reporting, missing resume, synthetic local history recording, and the no-JavaScript text view. It verified that no provider, inspection, browser observation, or submission record was produced by copying. No employer submission occurred.

Before screenshots come from the verified starting commit `9eb0f7bdbf0dcd572b82f1bbdee07ba2e25d3f17`. After screenshots come from the final synthetic workspace. All images are 900 pixels high.

| State | Evidence |
|---|---|
| Before, 1280 light | [prepare-1280-light.png](checkpoint3-visuals/before/prepare-1280-light.png) |
| Before, 1440 dark | [prepare-1440-dark.png](checkpoint3-visuals/before/prepare-1440-dark.png) |
| Before, 390 light/dark | [light](checkpoint3-visuals/before/prepare-390-light.png) · [dark](checkpoint3-visuals/before/prepare-390-dark.png) |
| Ready, 1280 light / 1440 dark | [1280 light](checkpoint3-visuals/after/prepare-ready-1280-light.png) · [1440 dark](checkpoint3-visuals/after/prepare-ready-1440-dark.png) |
| Ready, 390 light/dark | [light](checkpoint3-visuals/after/prepare-ready-390-light.png) · [dark](checkpoint3-visuals/after/prepare-ready-390-dark.png) |
| Clipboard denied, selectable fallback | [prepare-copy-fallback-390-light.png](checkpoint3-visuals/after/prepare-copy-fallback-390-light.png) |
| Missing resume | [prepare-missing-resume-1280-light.png](checkpoint3-visuals/after/prepare-missing-resume-1280-light.png) |
| Transfer requires manual review | [prepare-manual-review-1280-dark.png](checkpoint3-visuals/after/prepare-manual-review-1280-dark.png) |
| Application recorded locally | [history-recorded-1280-dark.png](checkpoint3-visuals/after/history-recorded-1280-dark.png) |

## Performance measurements

Single local synthetic `httptest` samples, not a statistical benchmark:

| Measurement | Starting commit | Checkpoint 3 |
|---|---:|---:|
| Prepare HTML, 50 preview questions and 19.6 KB description | 201,992 bytes · 9.43 ms | 204,614 bytes · 9.97 ms |
| Change | — | +2,622 bytes (+1.30%); +0.54 ms in this sample |
| Representative full brief | Not available | 23,948 bytes · 3.52 ms |
| Missing description | Not available | 354 bytes · 0.39 ms; explicit unavailable marker |
| 290 KB saved description | Not available | 290,323 bytes · 1.30 ms; complete description marker present |

Copying uses no employer-browser activity. The timing variation is small but these single samples should not be read as a performance guarantee.

## Native Wails qualification and remaining friction

The desktop production binary built and launched with a synthetic `YOURMOVE_DATA_DIR`. The Wails process did not appear in the Mac accessibility app registry available to this session, so I could not inspect the native window or verify its system clipboard interaction. Browser and no-JavaScript paths were controlled and verified in Chromium instead.

Manual native smoke steps:

1. Build with `go build -tags desktop,production -o /private/tmp/yourmove-go-checkpoint3 .`.
2. Start with an empty isolated directory: `mkdir -p /private/tmp/yourmove-wails-checkpoint3-manual && YOURMOVE_DATA_DIR=/private/tmp/yourmove-wails-checkpoint3-manual /private/tmp/yourmove-go-checkpoint3`.
3. In Opportunities, use **+ Add opportunity** → **Import multiple job URLs** with `https://example.test/careers/123`. This path saves only the URL; do not use Posting checks or Open employer site.
4. Open the saved job, use **Overview** → **Edit job and tracking**, and enter synthetic company, title, salary, and a multi-paragraph description. Save the details.
5. Open **Prepare**, click **Copy application brief**, paste into a local text editor, and check that the saved job details and full description are present. Verify missing questions are described as not discovered. Do not click Fill or submit.

Employer-only/manual questions still need completion on the employer site. Partial inspections still cannot represent unseen conditional or later-step questions. These limits are shown in readiness and remain subject to the existing manual review and submission safeguards.
