# v0.1.0 source staging map

This map was prepared because no `STAGING_MAP.md` existed in the worktree or the Projects tree. It is the explicit whitelist for this source consolidation. The initial inventory was 67 modified tracked files and 711 untracked files; the new map is a separate, intentional source-document addition.

## Preserved checkpoint

The original checkpoint remains immutable. The worktree copy and durable external copy match by SHA-256:

- `source-before.tar.gz`: `4d5b5f887322bebfef16509f12bd3af27c697c5c143b9439a15738654611a83a`
- `source-before-manifest.json`: `20fd23dc030b371200d3dd14ecea29624d8c72d4aeee23a78031da193e2f9e93`
- Durable copy: `yourmove-review-artifacts/sol61-product-experience-finalization/run-20261025T130038Z/` under the maintainer's Projects folder (outside `/private/tmp`).

## Inclusion rules

- Include all 66 modified tracked application, test, template, static, design, backlog, provider documentation, and release-configuration files, excluding the pre-existing `AGENTS.md` edit.
- Include all 67 untracked files under `internal/`; review confirmed these are Go source/tests and synthetic job-posting fixtures. The public-feed fixtures contain example-only data.
- Include the six implementation/evidence Markdown files listed below. Their screenshot links were removed or replaced with a note that raster captures remain local and are excluded.
- Include the generic `release/macos/WORKSPACE_BACKUP.md` guide; it contains no workspace or applicant data.
- Include this map so the PR retains the exact source manifest and exclusions.

No new application image or binary asset is required. `assets.go` embeds the existing `static/` and `templates/` trees, and the release bundle uses the existing tracked icon assets. All changed embedded templates and static files are included in the manifest.

## Exact staged source manifest

- `STAGING_MAP.md`
- `docs/BACKLOG.md`
- `docs/design/DESIGN_SYSTEM.md`
- `docs/design/RULES.md`
- `docs/implementation/ats-platform-integration-notes.md`
- `docs/implementation/job-intelligence/IMPLEMENTATION_PLAN.md`
- `docs/implementation/job-intelligence/evidence/competitive-ats-coverage.md`
- `docs/implementation/job-intelligence/evidence/m0-overview/README.md`
- `docs/implementation/job-intelligence/evidence/m1-closeout.md`
- `docs/implementation/job-intelligence/evidence/universal-recovery-provider-expansion.md`
- `docs/implementation/job-intelligence/evidence/universal-recovery/README.md`
- `internal/app/app.go`
- `internal/app/app_http.go`
- `internal/app/application_answer_ai_qualification_test.go`
- `internal/app/application_brief.go`
- `internal/app/application_brief_test.go`
- `internal/app/application_handlers.go`
- `internal/app/application_runtime_qualification_test.go`
- `internal/app/application_test.go`
- `internal/app/closed_posting_test.go`
- `internal/app/contact_qualification_test.go`
- `internal/app/cover_letter_independence_qualification_test.go`
- `internal/app/cover_letter_workbench_qualification_test.go`
- `internal/app/cover_letters.go`
- `internal/app/deletion.go`
- `internal/app/desktop_redirect.go`
- `internal/app/desktop_redirect_test.go`
- `internal/app/family_handlers.go`
- `internal/app/handlers.go`
- `internal/app/inspection_handlers.go`
- `internal/app/inspection_test.go`
- `internal/app/intake.go`
- `internal/app/job_description_salary_test.go`
- `internal/app/job_intelligence_competitive_browser_qualification_test.go`
- `internal/app/job_intelligence_competitive_lifecycle_test.go`
- `internal/app/job_intelligence_m1_demo_qualification_test.go`
- `internal/app/job_intelligence_m3_browser_qualification_test.go`
- `internal/app/job_intelligence_m3_lifecycle_test.go`
- `internal/app/job_overview_m0_browser_qualification_test.go`
- `internal/app/job_overview_m0_characterization_test.go`
- `internal/app/job_posting_source_lifecycle_test.go`
- `internal/app/job_posting_sources.go`
- `internal/app/job_posting_sources_test.go`
- `internal/app/job_reading.go`
- `internal/app/job_reading_test.go`
- `internal/app/job_recovery_diagnostics.go`
- `internal/app/job_recovery_review_test.go`
- `internal/app/job_recovery_test.go`
- `internal/app/jobs.go`
- `internal/app/manual_cover_letter.go`
- `internal/app/manual_cover_letter_test.go`
- `internal/app/prepare_entry_points_test.go`
- `internal/app/presentation.go`
- `internal/app/presentation_test.go`
- `internal/app/product_experience_qualification_test.go`
- `internal/app/product_experience_test.go`
- `internal/app/product_finalization_qualification_test.go`
- `internal/app/profile_completion_qualification_test.go`
- `internal/app/public_feed_lifecycle_test.go`
- `internal/app/release_data_test.go`
- `internal/app/submission_recording_qualification_test.go`
- `internal/app/ui_pages.go`
- `internal/app/ui_pages_test.go`
- `internal/app/universal_recovery_browser_qualification_test.go`
- `internal/application/browser/chromium/answer.go`
- `internal/application/browser/chromium/chromium.go`
- `internal/application/browser/chromium/contact.go`
- `internal/application/browser/chromium/contact_ownership_qualification_test.go`
- `internal/application/browser/chromium/cover_letter.go`
- `internal/application/browser/chromium/handoff_integration_test.go`
- `internal/application/browser/chromium/mutation_ownership_qualification_test.go`
- `internal/application/browser/chromium/platform_forms_test.go`
- `internal/application/browser/chromium/qualification.go`
- `internal/application/providers/ashby/ashby.go`
- `internal/application/providers/ashby/ashby_test.go`
- `internal/application/providers/ashby/destination.go`
- `internal/jobdocument/document.go`
- `internal/jobdocument/document_test.go`
- `internal/jobdocument/html.go`
- `internal/jobdocument/html_test.go`
- `internal/jobdocument/recovery_test.go`
- `internal/jobdocument/sections.go`
- `internal/jobdocument/sections_test.go`
- `internal/platforms/ashby.go`
- `internal/platforms/ashby_m0_characterization_test.go`
- `internal/platforms/ashby_test.go`
- `internal/platforms/closed_test.go`
- `internal/platforms/compensation_test.go`
- `internal/platforms/greenhouse.go`
- `internal/platforms/greenhouse_test.go`
- `internal/platforms/icims.go`
- `internal/platforms/jobvite.go`
- `internal/platforms/jobvite_test.go`
- `internal/platforms/lever.go`
- `internal/platforms/lever_test.go`
- `internal/platforms/personio.go`
- `internal/platforms/platforms.go`
- `internal/platforms/posting_source.go`
- `internal/platforms/posting_source_test.go`
- `internal/platforms/public_feed_live_qualification_test.go`
- `internal/platforms/public_feed_providers_test.go`
- `internal/platforms/recruitee.go`
- `internal/platforms/smartrecruiters.go`
- `internal/platforms/smartrecruiters_test.go`
- `internal/platforms/teamtailor.go`
- `internal/platforms/testdata/job-intelligence/README.md`
- `internal/platforms/testdata/job-intelligence/ashby-long-synthetic.json`
- `internal/platforms/testdata/job-intelligence/greenhouse-encoded-posting-synthetic.README.md`
- `internal/platforms/testdata/job-intelligence/greenhouse-encoded-posting-synthetic.json`
- `internal/platforms/testdata/job-intelligence/jobvite-posting-synthetic.README.md`
- `internal/platforms/testdata/job-intelligence/jobvite-posting-synthetic.html`
- `internal/platforms/testdata/job-intelligence/lever-ordered-synthetic-notes.md`
- `internal/platforms/testdata/job-intelligence/lever-ordered-synthetic.json`
- `internal/platforms/testdata/job-intelligence/smartrecruiters-posting-synthetic.README.md`
- `internal/platforms/testdata/job-intelligence/smartrecruiters-posting-synthetic.json`
- `internal/platforms/testdata/job-intelligence/workable-widget-synthetic.README.md`
- `internal/platforms/testdata/job-intelligence/workable-widget-synthetic.json`
- `internal/platforms/testdata/job-intelligence/workday-detail-synthetic.json`
- `internal/platforms/testdata/public-feeds/icims.html`
- `internal/platforms/testdata/public-feeds/personio.xml`
- `internal/platforms/testdata/public-feeds/recruitee.xml`
- `internal/platforms/testdata/public-feeds/teamtailor.xml`
- `internal/platforms/workable.go`
- `internal/platforms/workable_test.go`
- `internal/platforms/workday.go`
- `internal/platforms/workday_test.go`
- `internal/storage/manual_letters.go`
- `internal/storage/manual_letters_test.go`
- `internal/storage/sqlite.go`
- `internal/storage/sqlite_test.go`
- `release/macos/Info.plist`
- `release/macos/WORKSPACE_BACKUP.md`
- `static/styles.css`
- `static/ui.js`
- `templates/answer_library.html`
- `templates/applications.html`
- `templates/candidate.html`
- `templates/career_evidence.html`
- `templates/job.html`
- `templates/jobs.html`
- `templates/resumes.html`

## Explicit exclusions

- `AGENTS.md` (one modified tracked file): preserved byte-for-byte from this review; not part of the application PR.
- `review-runs/` (570 untracked files): generated review package, screenshots, logs, source archives, executable, and diagnostic artifacts. The source checkpoint archive and manifest are preserved separately as noted above.
- The remaining 63 untracked files under `docs/implementation/job-intelligence/`: screenshot PNGs and their checksum manifests, plus `SWARM_PLAYBOOK.md` (agent workflow notes). No screenshots are staged.
- Four remaining untracked `release/macos/acceptance/` files: pre-candidate baseline/packaging evidence tied to the older dirty source identity. They remain local and are not represented as candidate acceptance.
- No applicant databases, personal documents, browser profiles, credentials, or backup archives are staged.

## Verification performed before commit

- The staged manifest contained exactly the 141 paths listed above: 66 approved modified tracked files, 67 reviewed Go source/test/fixture files under `internal/`, six implementation/evidence Markdown documents, `release/macos/WORKSPACE_BACKUP.md`, and this map. `AGENTS.md` remains unstaged. The remaining 637 originally untracked files are excluded.
- `git diff --cached --check` passed. The staged source contains no binary files and no file exceeds 10,000 added lines.
- The staged scan found no private keys, provider tokens, credential assignments, or home-directory paths. Email-like matches were confined to provider/test fixtures; no applicant documents or databases are included. Local evidence references use `$PROJECTS/yourmove-review-artifacts/` without exposing the home-directory username.
- Embedded `static/` and `templates/` changes are both included. Existing tracked app icons are unchanged; no new embedded asset is required.
- Browser ownership and submission paths were reviewed. Posting ownership is constrained by provider origin and allowed path; resume, contact, answer, and letter transfer remain explicit guarded actions. The existing `/jobs/{id}/submit` handler and its explicit confirmation are unchanged; there is no automatic employer submission endpoint.
- `release/macos/Info.plist` contains the intended `CFBundleVersion` increment from `1` to `2`.

The manifest and checks will be repeated after reconciliation if the merge changes any listed path.
