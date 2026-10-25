# Recovery-driven provider expansion — 2026-10-24

This dated record supplements the previous competitive snapshot; it does not rewrite M1 PASS, M2/M3 PARTIAL or native acceptance. Baseline source archive SHA-256: 4f3b6f9b97af7cacb29733f2fde47a1a07f8ed5eab00ea214cfc29f3e6dc4435. Initial branch: feature/ashby-provider-finalization. All existing worktree changes preserved. No commits, pushes, merges or releases.

## Decisions and claims

| Target | Source / adapter disposition | Exact supported boundary | Recovery remains necessary for |
|---|---|---|---|
| Personio | READY for qualified public XML; implemented | HTTPS single tenant under jobs.personio.de or .com, /job/{numericID}, optional documented language de/en/fr/es/nl/it/pt. Fixed same-host /xml; exact position ID | Disabled feeds, custom domains, unknown languages, missing/incomplete descriptions, over-limit feeds |
| Teamtailor | READY for qualified vendor-hosted RSS; implemented | HTTPS single *.teamtailor.com tenant, /jobs/{numericID}-{slug}; /jobs.rss, two 100-item pages, 8 MiB aggregate and 20-second deadline; returned link host/ID | Branded domains, roles outside first 200 items, unavailable feeds, unqualified URL families |
| Recruitee | READY for qualified vendor-hosted XML; implemented | HTTPS single *.recruitee.com tenant, /o/{slug}; /api/feeds/offers.xml; exact slug AND returned careers URL; numeric ID required | Branded domains, unavailable feeds, application URLs, conflicting identity |
| iCIMS | READY for qualified classic vendor JSON-LD; implemented | HTTPS single *.icims.com tenant, /jobs/{numericID}/{slug}/job or /jobs/{numericID}/job; constructed /jobs/{numericID}/job?in_iframe=1; matching JobPosting URL host/ID | Modern/branded portals, authenticated content, missing/mismatched JSON-LD, expired postings, unqualified paths |
| Taleo | CONDITIONAL research; no adapter | IAEA explicit career-section URL returned exact requisition title/number and encoded description in server-delivered api.fillList script data | All Taleo URLs remain recovery. Dedicated bounded parser needs multiple employers/regions and negative identity fixtures |
| Avature | BLOCKED adapter qualification; no adapter | IBM example redirected to branded Error; Maximus old detail slash variant returned404; current public search200 exposed no server-side JobDetail links | All Avature URLs remain recovery. Need current successful identity-bound detail responses across repeatable tenant patterns |

READY here is source/adapter disposition, not native acceptance. App lifecycle/controlled Overview results belong to the integrated recovery report. Four new adapters bring the possible source portfolio to eleven, not thirteen. Taleo/Avature are not implemented or supported.

## Primary contracts consulted

- [Personio XML reference](https://developer.personio.de/v1.0/reference/get_xml) documents career-site feed and seven languages. Actual anonymous availability was separately observed.
- [Teamtailor RSS guide](https://support.teamtailor.com/en/articles/11171756-rss-feed-how-to-guide) documents descriptions, IDs, locations and offset/per_page; default100. No credentialed API used.
- [Recruitee feed routes](https://support.recruitee.com/en/articles/8213076-faq-api) documents formatted XML /api/feeds/offers.xml. Observed formatted root is offers/offer; do not substitute separate jobs/job schema.
- [Recruitee authentication notice](https://docs.recruitee.com/reference/authentication-1) announces10 February2027 token enforcement for Careers endpoints and explicitly exempts XML feeds. No anonymous JSON API dependency or employer token.
- [iCIMS Job Portal API](https://developer-community.icims.com/applications/applicant-tracking/job-portal) examples require Basic authentication. Adapter uses public applicant-facing HTML only.
- [Oracle career-section identity](https://docs.oracle.com/en/cloud/saas/taleo-enterprise/20d/otcug/career-sections.html) distinguishes career-section codes, requisition numbers and public/private configuration. Documentation alone does not qualify all deployments.

## Actual public-source qualification

Read-only GETs on2026-10-24 America/Detroit (2026-10-25 UTC). Sandbox DNS was unavailable; public retrieval ran through approved network escalation. No applicant profiles, resumes, answers, browser storage, credentials or application mutations. Raw research stayed in /private/tmp/yourmove-provider-research; headers (which may contain server-issued session cookies) are deliberately excluded from repository evidence.

Actual command: YOURMOVE_QUALIFY_PUBLIC_FEEDS=1 go test ./internal/platforms -run '^TestPublicFeedLiveSourceQualification$' -count=1 -v

Result: **PASS**,8/8 live imports across two employers for each of four providers,3.83s. Exact posting URLs/assertions are in internal/platforms/public_feed_live_qualification_test.go. Every import ran production FetchJob and sealed-source validation; claims are not fixture-only.

| Employer / source | Bytes / records | Response body SHA-256 |
|---|---:|---|
| personio.jobs.personio.de/xml?language=en | 8,550 /1 | d3fc39d51dadf5ce5372d59ad6d02353d17f8fa20d597a5e3bcfa457403478de |
| knime.jobs.personio.de/xml?language=en | 43,793 /5 | 9a590ee2ee77fc0e3a36f2ba3baee80809d3c2b5c59063c7f82766d0b40273d3 |
| career.teamtailor.com/jobs.rss | 101,490 /14 | bddc56f08656b9b05ea5b5c0d93a7811b867cdd14dd6d49042c1547851d934d0 |
| hiire.teamtailor.com/jobs.rss | 175,293 /27 | dd028b9928d952fd66134ad012328d958060ac086aa6297ff522e3bb8b4c1214 |
| make.recruitee.com/api/feeds/offers.xml | 17,159 /3 | e323d16afabd942550bfc7dce6be0528e00304e3b0d9d2d7405375a3972a075c |
| rewire.recruitee.com/api/feeds/offers.xml | 123,707 /13 | 3823ce77cd39d4b8a4bea68234b23330487d81dee1fe7695f56a0a4b72dabc8b |

Classic iCIMS pages initially returned wrapper HTML. Their server-delivered iframe URLs identified the same public posting with in_iframe=1, returning identity-bound JSON-LD for Origami Risk4645 and i3 5272. Numeric-only route was separately confirmed200 for i3. Both production live tests passed. Old Element19208 returned410; that does not qualify universal closure handling.

Live canonical lengths: Personio6,743/4,764 bytes; Teamtailor4,500/3,434; Recruitee3,169/6,089; iCIMS10,358/6,134. All retained rich structure. No eligibility, compensation or requirements were invented. Ambiguous Recruitee workplace flags remain unspecified. Personio currently displays the direct working schedule, without inferring contract permanence.

## Retention and qualification contracts

Adapters preserve canonical text and ordered HTML in the existing PostingSource. Personio section names become escaped headings; Recruitee requirements follow description. Tracking queries are not transmitted: only validated tenant, posting identity and Personio language construct endpoints. Every request drops cookie jars, sends no credentials, refuses redirects and bounds bytes/time. Missing items and feed404/410 are retrieval failures, not individual closure proof. Existing transactional persistence owns last-success retention and manual precedence.

Synthetic fixtures in internal/platforms/testdata/public-feeds mirror observed structures using invented employer/role prose. Tests cover exact identity, wrong host/ID, duplicate candidates, malformed XML/entities, encoded separators, unsafe hosts/credentials/ports, redirects,401/403/404/410/503, byte/title bounds and bounded pagination. Existing jobdocument escaping/sanitization remains the sole renderer. No trusted HTML or application authority added.

## Evidence-to-adapter loop

Prior competitive evidence recorded classic iCIMS shells as BLOCKED. This investigation retained that failure distinction, observed the same-posting public iframe variant, checked two employers, built an invented-content JSON-LD fixture and strict host/ID binding, then exercised production FetchJob against both live examples. This eliminates a demonstrated retrieval need for this narrow family. Integrated lifecycle tests must separately prove recovered manual content survives adapter refresh and restart. Applicant-edited text is never employer-source evidence.

Engineers should reproduce anonymous accessibility, confirm full identity, compare multiple employers, build sanitized realistic fixtures, then qualify canonical/source/app-lifecycle/Overview behavior. Keep optional source samples separate and reviewable. No automatic upload, telemetry backend, browser-history collection or credentialed endpoint discovery was introduced.

## Selected live source identities

These source-content digests identify only the selected retained content/facts, not whole response bodies or immutable vendor fixtures.

| Public posting | Retained source SHA-256 |
|---|---|
| [Personio1834171](https://personio.jobs.personio.de/job/1834171?language=en) | 2113707ff19f71d1460af4aed5c0e358493721677258999ef69e71f45e581bdc |
| [KNIME2696582](https://knime.jobs.personio.de/job/2696582?language=en) | e88a1c5047e0cf8bfea9b889712f1fe57306bee640a9bd7b5ed628df32f2778e |
| [Teamtailor8118064](https://career.teamtailor.com/jobs/8118064-uk-account-executive-smb) | a2000d6ecbc39bbc058ddcc91d904bcedd3c71ae51368432c0f2cf1698376e33 |
| [Hiire8099372](https://hiire.teamtailor.com/jobs/8099372-senior-backend-engineer) | 10f4185f5431decbed37d2230e569ef44544788e62e35bdb0e600bd58879be6b |
| [Make backend engineer](https://make.recruitee.com/o/backend-engineer-freelance-1) | 988cbfbb380b7732c77137d9f0721db96667040e9a54f6bc7a405a45b3ddb2a4 |
| [Rewire recruiting lead](https://rewire.recruitee.com/o/head-of-recruiting-people-development-mwd) | 8aed67233c8946572f6640fd490bb5a93dd4f4059c4ebcdaac2b65826b23779e |
| [Origami Risk4645](https://careers-origamirisk.icims.com/jobs/4645/software-engineer%2C-modernization/job) | f8be14239f0030ed815e4ca77a60bb88a9467efd970bb4f960fb47d3ce1ae314 |
| [i3 5272](https://careers-i3-corps.icims.com/jobs/5272/software-engineer/job) | 6087045e9e6f4e8cec8bf7920529c4a7acc07d7a6d352997e4b3a61ce68ae09a |

Taleo research URL: [IAEA requisition2026/0442 (016041)](https://iaea.taleo.net/careersection/ex/jobdetail.ftl?job=2026%2F0442%20%28016041%29). This is evidence of one exact server-delivered script structure, not a Taleo support claim. Avature research: [IBM example](https://ibmglobal.avature.net/en_US/careers/JobDetail/Software-Engineer-Backend/53086), [Maximus expired detail](https://maximus.avature.net/careers/JobDetail/Senior-Full-Stack-Developer/40004/), [Maximus public search](https://maximus.avature.net/careers/SearchJobs).

Provider production code frozen after focused tests and whole internal/platforms suite PASS. Command: GOCACHE=/private/tmp/yourmove-go-cache go test ./internal/platforms -count=1 (0.460s). Diff whitespace check PASS. Shared dispatch/identity changes belong to the orchestrator. Native acceptance remains NOT VERIFIED here.

The integrated local evidence workflow must exclude automatic source samples by default. This lane supplies a repeatable engineering investigation and source-to-fixture example; applicant review/export and connected recovery proof are separately tracked. Do not label the entire learning gate PASS from adapter research alone.
