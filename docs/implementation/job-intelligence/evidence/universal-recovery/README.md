# Universal Job Recovery — integrated evidence, 2026-10-24

## Disposition

Engineering implementation and controlled recovery/Overview demonstration are complete for the scope below. **Overall requested acceptance remains PARTIAL:** native Wails user interaction is NOT VERIFIED; Taleo/Avature adapters are not qualified; applicant-selected source-sample export and full learning-loop product integration remain unfinished. Do not describe this as a thirteen-provider or native-accepted release. M1 PASS, M2/M3 PARTIAL and deferred VoiceOver YM-017 remain unchanged.

No commit, merge, push or release. All pre-existing modifications, including AGENTS.md, retained. Current GitHub lookup found no PR for `feature/ashby-provider-finalization`; there is no CI claim for this dirty source.

## Before and after

Before: unfamiliar URLs were already saveable. Intake waited for retrieval and returned the list. An empty job opened Prepare, with description entry hidden in Edit details. Manual corrections and failed-refresh retention were already strong. New cover letters required generation.

After: the first-party single-job form saves locally without retrieval, then opens Overview. The description-first editor is already open. Paste and Save produce a readable job with conservative heading/list preservation and section jumps. Explicit Posting checks retrieve supported sources separately. Existing legacy intake callers retain synchronous retrieval; saving through the first-party UI never waits for it. A repeat UI import opens the existing opportunity without fetching or overwriting it.

Prepare retains resume selection/download and explicit external opening. Profile and Answer Library links support manual copying; unsupported forms gain no managed-browser or transfer authority. Applicants can write/paste a new cover letter without AI, then edit/download it through the existing draft lifecycle. Manual completion still requires the existing explicit confirmation and recorded resume review. Next continues the queue.

## Connected demonstration

`TestRecoveryIndependentManualContinuation` exercises production handlers with synthetic applicant data: unsupported URL → save → paste description/correct fields → choose resume → Prepare → explicit external-opener seam → reject managed browser start → failed refresh → reviewed diagnostics → SQLite reopen → confirmed completion → Next. No real employer browser action occurs in this test.

`TestUniversalRecoveryBrowserJourney` uses the actual templates/scripts/POSTs in controlled Chromium: immediate paste editor; focus, paste, Save (three synthetic actions); readable review; Prepare; keyboard Tab to title; no-JavaScript save of corrected title/employer. Screenshots cover empty/review/Prepare at 1280/700/390, light/dark (18 images). Width checks found no horizontal overflow. These are scripted interactions, not human usability timings or native accessibility acceptance.

Visual inspection found and fixed the empty panel pushing the editor below the 390px viewport. Later inspection found plain headings merged into paragraphs; the plain renderer now recognizes only existing conservative section headings and explicit bullet prefixes. Canonical stored text, brief content and generation hashes remain unchanged. HTML pasted as text remains escaped, never executable; arbitrary rich clipboard styling is not retained.

## Recovery acceptance matrix

| Scenario | Observed evidence / limit |
|---|---|
| Unknown valid HTTPS | PASS production save/edit/read/reopen and controlled UI |
| Recognized unsupported variant | PASS synthetic malformed Ashby URL saved/recovered; no closure inferred |
| Temporary failure | PASS deterministic failed refresh retains prior canonical/source |
| Authentication-gated source | Bounded provider 401/403 tests PASS; no authenticated page capture or human journey claimed |
| Invalid/unsafe URL | Existing scheme/credential/control validation tests PASS; storage permission never authorizes retrieval |
| Browser-pasted description | PASS plain content, named section navigation and bullet structure; rich clipboard markup not stored |
| Large plain text | PASS 92000-byte Unicode description through URL-encoded edit POST; body bound accounts for percent expansion |
| Hostile HTML | PASS escaped literal script content; no execution or trusted HTML |
| User-authorized capture | UNAVAILABLE under current exact supported application-browser authority |
| Capture unavailable → manual | PASS paste fallback; no extension or cloud required |
| Prior source → failure | PASS last-success source retained/freshness unconfirmed |
| Correction → refresh | PASS manual primary plus separately retained employer source |
| Restart | PASS SQLite reopen retains content/manual flags/selected resume; no browser authority restored |
| Duplicate/repeat | PASS offline repeat opens existing job; no fetch/overwrite |
| iCIMS/Taleo/Avature recovery | PASS synthetic URLs; narrow classic iCIMS now also qualifies automatic retrieval |

Five synthetic unsupported URL families saved and recovered in the dedicated test; one controlled browser recovery at six width/theme combinations. These are bounded samples, not population percentages. Manual fallback is measured; capture recovery is unavailable. Prepared answer identities are unchanged. Creating arbitrary new per-job questions for unknown forms is not implemented; use existing Answer Library/profile content manually.

## Provenance and persistence

Existing manual flags distinguish applicant-supplied/corrected canonical text from employer source. The UI labels it “Your saved description” and states it is unverified. Successful provider refresh retains source separately for comparison and never replaces manual corrections; failed checks preserve last usable values/source. Page-capture provenance is unnecessary until capture exists.

Manual cover letters store `manual / user-authored`, NULL resume source IDs and empty generation hashes. A transactional legacy table migration removes only source-ID NOT NULL constraints, preserving draft IDs/body/evidence/timestamps. Readers coalesce absent source IDs into the existing in-memory zero representation. Tests cover legacy preservation, reopen/integrity, and manual insert with foreign keys enabled. Errors retain escaped submitted letter text for copying/retry. No fake resume evidence or parallel letter model.

## Local diagnostic workflow

Overview → Posting details and source checks → Review posting diagnostics → inspect metadata → check confirmation → Download metadata. Declining changes nothing. Default package includes reviewed hostname, value-free path shape, path count/query-presence, recognized vendor family, coarse outcome and recovery/origin booleans. It excludes paths/query values, source notes/raw errors, descriptions, browser state, profiles, resumes, answers and letters. No upload or telemetry exists. A hostname can itself identify a private employer/tenant, so inspection is explicit. Source failures are deliberately reported as unavailable when authentication/network/parser cause is not proven; no speculative classifier invents certainty.

This compact pattern record aids grouping, but is **not a full developer fixture package**. Optional source samples and exact public URLs are not exported. Engineers obtain separately approved public examples, establish anonymous accessibility and exact identity across employers, build invented-content fixtures, then validate source/canonical/app lifecycle/Overview. The [provider report](../universal-recovery-provider-expansion.md) demonstrates this investigation for classic iCIMS: earlier shell-only failure → same-posting public iframe → two employers → identity-bound JSON-LD fixture → live production retrieval → controlled lifecycle/Overview. Applicant edits never establish API facts. Full applicant-to-fixture sample export remains an explicit follow-up.

## Provider scorecard

Six targets investigated. Four new adapters source-qualified on eight live postings/two employers per adapter and qualified through production controlled Overview lifecycle: Personio, Teamtailor, Recruitee, classic iCIMS. Eleven implemented controlled-supported providers total including the original seven. Taleo is CONDITIONAL research/no adapter; Avature BLOCKED qualification/no adapter. New native-accepted providers: zero. Exact supported and blocked variants, primary documentation, dated live digests and fixtures are in the [provider expansion report](../universal-recovery-provider-expansion.md).

The extended competitive Overview test checks all eleven through intake, identity, structured reading, canonical copy/brief, manual edit, changed source, freshness and SQLite restart. New provider screenshots at 1280/390 are included here. Native support is separately unverified.

## Verification and artifact handling

Logs and candidate identity live outside source under `/private/tmp/yourmove-recovery-final-20261024`. Candidate source is recorded by the build's before/after SHA-256 file manifest. The pre-task recoverable source is `/private/tmp/yourmove-recovery-baseline-20261024/source.tar`, SHA-256 `4f3b6f9b97af7cacb29733f2fde47a1a07f8ed5eab00ea214cfc29f3e6dc4435`, alongside HEAD/status/binary tracked patch. It matches the supplied historical source identity.

Native UI tools are unavailable in this runtime. Build/launch evidence must not become a native interaction PASS. Use the final isolated launcher and synthetic directory; perform save/paste/review, resume/answers/letter manual continuation, default-browser return, confirmation/Next, light/dark/narrow keyboard and same-directory restart in the actual Wails app. Native download via WebView attachment response also needs verification; no claim that Chromium download establishes native Save dialog behavior.

Independent read-only review found no concrete correctness/privacy/SSRF/migration defect in the reviewed recovery/provider paths and reran focused packages PASS. Remaining risks by user impact: native continuation/export/restart unverified; no managed capture for unknown pages; arbitrary per-job question drafting absent; metadata-only learning export; Taleo/Avature and branded/custom feed domains remain recovery; rich clipboard emphasis/links are not retained. Next milestone: native acceptance of the connected recovery journey on the frozen candidate, then a bounded reviewed source-sample learning package. Do not expand provider count before these close.
