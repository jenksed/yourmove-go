# YourMove Go — v0.1 release readiness and critical path

**Assessment date:** October 20, 2026, America/Detroit.
**Audience:** publicly recruited beta testers, following the owner's clarification that recruitment will extend beyond a private development group.
**Release decision:** **NO-GO for public beta distribution today.** The production macOS application is buildable and basic native operation works. A distributable release candidate, reliable native file export, local-data safeguards, and complete native acceptance still need work.
**Implementation boundary:** assessment only. The accepted focused-question and Scope-First Continuation implementations remain closed. No product code, dependencies, existing tests, or existing documents were changed. No commit was created.

## 1. What is established now

| Question | Result | Evidence and limit |
| --- | --- | --- |
| Can the real desktop executable build? | PASS on this Mac | `GOPROXY=off go build -tags desktop` built the current working tree without a source overlay or qualification tag. Go 1.27.2, macOS 26.5.2, arm64, CGO enabled. |
| Can it run away from the repository? | PASS for a staged assessment bundle | The production executable was copied byte-for-byte into a temporary `.app` and launched with an empty working directory. Templates, CSS, logo and navigation rendered. Current [assets.go](../../assets.go#L5) embeds templates and static files; older repository-relative-asset warnings in `WAILS_COMPAT.md`, `UI_WORKBENCH.md`, and `desktop.go` are stale. |
| Is there a qualified public installer/download? | NOT ESTABLISHED | No checked-in release recipe, distribution bundle, release identity/version policy, signing/notarization result, or clean-machine installation evidence was found. The temporary assessment bundle is not a release artifact. |
| Can native forms and the picker operate? | PASS for tested local paths | Native add opportunity, document picker/import, explicit resume assignment, and job-edit POST/GET navigation worked. |
| Do tested local records survive native restart? | PASS | Command-Q exited the identified process. A deliberate relaunch retained the synthetic job, company/description edits, selected resume, and document identity. Imported bytes matched their stored SHA-256 and size; SQLite `quick_check` returned `ok`. |
| Can native download/export be relied on? | UNQUALIFIED; release gate | Two activations of the synthetic DOCX Download link produced no observable save dialog, UI acknowledgement, navigation, or expected `Downloads/Resume*.docx` output. This does not establish where every possible download destination would be, nor qualify PDF export. Passing HTTP byte-response tests do not establish native file delivery. |
| Is the complete native employer journey accepted? | **BLOCKED** | This assessment did not execute native Fill application, generation, letter editing/saving, answer filling, continuation, employer navigation, or submission recording. The older isolated Chromium connectivity/window-targeting blocker remains in force. |

The build's Mach-O `LC_BUILD_VERSION` declares **macOS 26.0 minimum** and **arm64 only**. Its signature is linker-generated **ad hoc**, identifier `a.out`, with no Developer ID team and no bound Info.plist. Those are observed artifact properties, not a proposed compatibility policy. Choose and test a supported OS/architecture matrix before recruiting testers; do not assume this build works on older macOS or Intel.

For direct public beta distribution, the recommended acceptance gate is a Developer ID-signed, hardened, notarized candidate with a stapled ticket, installed through the normal macOS path. Apple documents the requirements in [Notarizing macOS software before distribution](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution). A ZIP containing the properly signed app is sufficient as a container; a custom installer is unnecessary for v0.1. See [Packaging Mac software for distribution](https://developer.apple.com/documentation/xcode/packaging-mac-software-for-distribution). Do not make removal of quarantine or disabled Gatekeeper part of tester onboarding.

## 2. Fresh native evidence and isolation

Evidence root: `/private/tmp/yourmove-v01-readiness-zbvtxmlk/evidence/`.

The unmodified production executable SHA-256 is `5a208704cfc74627b3a57080ca2c7a2f240c4f9982e21409c7bc4bd9c971cfd0`. The installed copy has the same hash. Its source baseline is the existing dirty working tree, recorded in `source-before.json`; the Git revision alone does not identify these accepted uncommitted changes.

The temporary bundle is `/private/tmp/yourmove-v01-readiness-zbvtxmlk/installed/YourMoveReadiness.app`, identifier `local.yourmove.release-readiness-zbvtxmlk`. A small launcher forces its own synthetic data directory and empty CWD on every launch, including a UI-tool relaunch. It does not inject jobs, providers, browser sessions, or alternate Go handlers. This storage-only launcher is a qualification precaution, not a proposed production startup policy.

Native actions actually performed:

1. Launch the staged app; inspect its empty Opportunities screen.
2. Save `https://example.com/yourmove-v01-readiness-synthetic` with a synthetic title through the real native form. This unsupported URL uses the local unsupported-source branch; no employer page was opened.
3. Open Documents, invoke the real macOS picker, and import the isolated synthetic `role.docx`.
4. Open Prepare, choose that document, and explicitly save its assignment. The current resume-save handler returns to Overview; improving that navigation is deferred polish.
5. Save synthetic company and description changes through the native job editor.
6. Activate the library's read-only DOCX Download link twice and inspect the unchanged native state. Export remains unqualified.
7. Quit with Command-Q; confirm the exact PID exited; deliberately relaunch. Inspect the persisted opportunity and resume-selected indication.
8. Quit the isolated app again and confirm it exited. Older YourMove applications were neither operated nor terminated.

Native screenshots inspected: `01-native-empty.png`, `02-native-import.png`, `03-native-download-result.png`, `04-native-before-restart.png`, `05-native-restarted.png`. The restart accessibility tree is `05-native-restarted.ax.txt`. Native request evidence is in `native.log`; exact process paths are in `native-process.txt`, `native-quit-process.txt`, and `native-final-process.txt`.

The synthetic native database ended with one job, one imported document, the saved resume assignment, zero cover letters and zero submissions. This is a local smoke test, not a native transfer-counter result. No employer browser operation was requested. The six-file normal-storage byte manifest was unchanged: zero added, removed, or modified files. Private normal-storage manifests remain outside the repository. The earlier normal-storage incident was not repaired or reinterpreted; its authoritative prior values remain unresolved.

## 3. Defects, severity and uncertainty

**No new P0 product defect was demonstrated.** Here, P0 means demonstrated unintended employer transmission/submission, destructive or cross-workspace data corruption, or a fundamental launch/journey failure across the declared supported beta configuration. Missing acceptance evidence is a release blocker, but it is not proof of such a defect. The existing test-harness normal-storage incident remains a serious unresolved qualification incident, distinct from a reproduced production defect.

### A. P1 — automatic migration can produce an incomplete workspace

[applicationStorage](../../storage.go#L15) automatically considers CWD `jobs.db` when the destination is absent, even with an explicit data-directory override. [migrateLegacyJobs](../../storage.go#L63) snapshots the database only. If that database contains resume records, their stored files are not copied to the destination's new `resumes` directory.

The isolated probe created a synthetic workspace with an imported DOCX, migrated its database, and loaded the copied resume row successfully. `readResumeBytes` then failed because the destination lacked the file. The original file remained intact. Reimporting the identical DOCX returned the existing row through [importResume's hash deduplication](../../resumes.go#L119), reported success, and still did not repair the missing file.

**Impact:** affected migrated workspaces cannot use their recorded documents for download, generation evidence, or authorized filling. This is not proof that ordinary current App Support workspaces lose documents on every launch. A genuine old jobs-only database has no document bytes to copy. The defect is that automatic migration accepts a document-bearing database without completing or explicitly refusing that migration.

**Smallest safe correction:** detect and refuse an incomplete document-bearing migration before installing the destination, with an explicit recovery instruction, or implement a verified whole-workspace copy. Keep the original untouched. Reimport must either restore exact verified missing bytes safely or report that recovery is required. Do not silently fabricate success. Verify the supported jobs-only migration separately.

### B. P1 investigation — native file export has no observed completion

The native DOCX download observation above is repeatable at the UI boundary, but the request/destination failure mechanism is not established. [downloadResume](../../application_handlers.go#L432) and [downloadCoverLetter](../../attachment_downloads.go#L10) currently return HTTP attachment responses; [desktop.go](../../desktop.go#L27) configures an import picker, without a corresponding application save-file seam.

**Release gate:** establish an observable native Save/Cancel path and compare the exported bytes with the exact selected saved document. Test resume PDF/DOCX and saved-letter PDF independently. A native export failure blocks the manual fallback for unsupported employer forms. Do not label all exports broken or select a dependency upgrade from this limited observation.

### C. P1 for the advertised input range — valid letter text can exceed the encoded request limit

The isolated handler probe submitted 5,000 `é` characters: **10,000 UTF-8 bytes**, accepted by `saveCoverLetterBody`'s 12,000-byte body validator. URL encoding produces **30,016 request bytes**, exceeding the **24 KiB** limit at [saveCoverLetterHandler](../../cover_letters.go#L300). The response was generic HTTP 400, without the submitted-text recovery editor. The previously saved revision remained unchanged. Browser Back recovery was not established by this probe.

This is a bounded save/error-path defect, not evidence of premature transfer or overwritten persisted data. Align accepted body size, form encoding limits, and recovery behavior before advertising the full input range. General multilingual PDF support is a separate capability and can be deferred; valid local text saving should not depend on PDF glyph support.

### D. Acceptance concern — one fresh question qualification timeout

The combined fresh controlled run passed the contact journey, combined continuation, and scope-state tests, then timed out at `question_workbench_qualification_test.go:372`: the final no-JavaScript answer save did not reach the expected revision in that run. The test passed unchanged when rerun alone. Both logs are retained. This does not establish a reproducible P0 or justify reopening the accepted question UI for cosmetic work. It does prevent reporting this assessment's combined qualification command as uniformly green. Investigate with captured request, DOM and revision evidence if it recurs; do not weaken the assertion.

## 4. Acceptance strategy independent of the old temporary native harness

Use complementary gates. The browser fixture remains useful; it is not the sole release authority.

| Gate | How to qualify | Required record |
| --- | --- | --- |
| Production artifact | Build the frozen accepted source, package stable identity/version metadata, sign/notarize/staple; download the actual candidate to a different Mac or clean macOS account without the repository or development caches. Install and launch through Finder with normal macOS protections enabled. | Candidate hash, source manifest/revision, build recipe, signing/notarization result, OS/architecture, installation and launch result. |
| Native local workflow | A human follows a short script in the production candidate: add/edit job, import/cancel picker, save/change resume, approve synthetic profile, save/reopen local materials, export/cancel export, restart, and inspect persistence. No UI automation is required for acceptance. | Screenshots plus exact saved identities/hashes, native POST outcomes and persistence readback. This assessment supplies a partial pass only. |
| Mutation authority | Retain the controlled localhost Chromium suites for exact bytes/values, one resume upload, independent failures, manual-value preservation, conditional questions, no duplicate/uncertain replay, no save-triggered mutations, and zero automatic submissions. | Fixture-side counters and hashes, independent outcomes, complete logs including failures. |
| Complete native application | A human operates the exact packaged candidate and a positively identified employer window. Use an explicitly controlled QA/test posting on a supported ATS, or an independently identified isolated fixture with its existing narrow test seam. Observe counters independently from window-control automation. Exercise first fill → manual correction → generate/edit/save → review exact scope → late/combined continuation → review → local recording semantics. | One complete fresh native journey, correct browser ownership, actual per-item effects, no repeat resume, retained manual correction, no premature transfer, and human-controlled submission. No live candidate/employer test was authorized or performed here. |
| Upgrade and recovery | Use synthetic prior-version workspaces with documents, drafts, answers and history. Test repeat opening, interrupted migration, backup/restore, missing/corrupt files, and replacement of the app bundle while retaining data. | Original untouched, verified destination bytes/records, explicit refusal on unsupported recovery, and restoration proof. |

For complete-native acceptance, first try **human operation** rather than repairing the failing window-automation infrastructure. An existing controlled ATS test posting avoids generalizing production ownership rules. If only the fixture path is available, fix the narrow test setup separately and keep its qualification identity explicit. Never disable destination validation or claim a test-seam binary is the shipping artifact. A release may be accepted manually; no browser screenshot, Go suite, or unavailable counter can replace an unobserved native step.

## 5. v0.1 scope and deferrals

**Must complete for publicly recruited beta:**

- A downloadable, versioned, signed/notarized macOS candidate with a declared hardware/OS floor and clean-machine Finder installation/launch evidence. Test the hardened/signed artifact, not only the current ad hoc build.
- Native import, exact saved-document export, deliberate transfer, honest uncertainty/manual fallback, local persistence/restart, and manual submission-record semantics. Preserve both accepted Workbench slices.
- Close the reproduced migration/reimport and valid-text save-limit defects; verify a practical whole-workspace backup/restore procedure. A documented procedure is sufficient; a new backup UI is unnecessary.
- A complete native journey with a recorded manual signoff. Resolve or explicitly explain recurring controlled-test failures before signing off.
- Runtime dependency instructions: existing supported Chrome/Chromium for assisted filling; the GUI must remain useful if it is absent. Test Finder launch without Playwright caches or a developer shell. The current detector searches installed Chrome/Chromium and caches; a developer-cache success is not a fresh-user dependency test.
- Keep AI generation optional. Qualify at least one advertised setup path from a Finder-launched app, including unavailable-provider behavior and existing generation consent. Current API instructions rely on shell environment variables; passing CLI/provider unit tests does not establish that Finder inherits that setup. Document a workable path or narrow the beta claim before adding more providers.
- A brief beta guide: supported sites and limits, what is saved locally, when browser-only progress can be lost, what leaves the device for generation/transfer, manual submission responsibility, backup/update instructions, and a feedback route requesting version/OS/steps with personal data removed. No new telemetry system is required.

**Defer:** cosmetic Workbench changes; manual-letter creation/schema redesign; AI question answering; question-bank infrastructure; new ATS adapters; a browser recovery redesign; durable employer receipts; multilingual PDF font expansion; every AI-provider integration; automatic updates; cloud sync/accounts; Windows/Linux; Intel support unless explicitly recruited; broad architecture refactoring. Manual download of a new version can serve beta updates once data-retention behavior is tested. Do not market deferred capabilities as supported.

## 6. The next three engineering tasks

| Order | Bounded task | Likely scope | Done when |
| --- | --- | --- | --- |
| 1 | **Close the reproduced local-data failures.** | `storage.go`, `resumes.go`, the letter form parser/error boundary, and focused tests. Preserve the current schema and transfer authority. | Document-bearing migration is complete or explicitly refused before installation; originals remain intact; identical-file recovery never reports false success; valid encoded letter text saves or remains recoverable; existing jobs-only migration and saved revisions still pass. |
| 2 | **Qualify and complete native saved-document export.** | Native desktop save-file capability plus existing resume/letter download boundaries and native acceptance cases. Start by capturing whether the GET is handled and where output is intended. | A human chooses a destination, receives exact saved resume/PDF bytes, can cancel without writing, and returns to the workspace. Browser downloads and document identity checks retain their behavior. No Wails/Chromium redesign. |
| 3 | **Produce the public-beta release candidate.** | Checked-in macOS packaging recipe and metadata, chosen deployment target, Developer ID/hardened signing, notarization/stapling, version/hash record, and the short clean-machine/manual acceptance script above. | An external tester can download, install and Finder-launch the candidate without Go, the repo, disabled Gatekeeper or development caches. Required native, dependency, persistence and complete-journey gates have explicit results against that candidate. |

Signing-account/certificate readiness can be checked while tasks 1–2 are being scoped; credentials need not enter source, logs, or this report. Final publication remains a separate authorized action. The earlier personal-data recovery review also remains separate; do not infer or overwrite unknown historical profile/selection values while building the beta.

## 7. Verification and handoff

Fresh passing commands:

```sh
GOPROXY=off go build -tags desktop -o <assessment>/build/YourMove .
go test ./... -count=1
go test -tags desktop ./... -count=1
go test -race ./... -count=1
git diff --check
```

Fresh controlled results: `TestControlledContactJourney`, `TestControlledCombinedContinuation`, and `TestControlledScopeFirstWorkbench` passed in the combined run. `TestControlledQuestionWorkbench` failed once at its last no-JS save and passed the isolated unchanged rerun. Logs: `controlled-browser.log`, `questions-recheck.log`. The passing journeys retained one resume upload, preserved manual corrections, no save-triggered employer mutation, and zero submissions. Responsive captures are under `scope/`, `states/`, and `questions/`; these remain browser evidence, distinct from the native screenshots above.

The two temporary assessment probes passed by **observing the expected defects**, not by validating correct migration/save behavior. Their source and overlay remain at `/private/tmp/yourmove-v01-readiness-zbvtxmlk/release_assessment_test.go` and `overlay.json`; results are in `assessment-probes.log`. They are not added to the ordinary passing repository test suite.

Other evidence: `desktop-build.log`, `build-info.txt`, `build-manifest.json`, `macos-build-target.txt`, `go-test.log`, `desktop-test.log`, `race-test.log`, `native-first-write.json`, `native-import.json`, `native-before-restart.json`, `native-after-restart.json`, and `normal-storage-verification.json`.

The only repository addition is this assessment. Source manifests confirmed that all pre-existing tracked and untracked files were unchanged. No commit, reset, stash, cleanup, dependency update, native qualification repair, normal-storage recovery, employer submission, distribution upload, or publication was performed. The isolated native process is stopped; its synthetic artifacts are retained for review.
