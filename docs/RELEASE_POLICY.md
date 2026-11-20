# YourMove release policy

**Policy established:** October 23, 2026. Applies to desktop release candidates.

## Purpose

A target date can prompt a review. The release owner may authorize shipment when the frozen candidate meets the applicable gates for its advertised scope and audience. A scheduled date never overrides data integrity, applicant authority, privacy, installability, or a blocked core workflow.

The [strategic roadmap](ROADMAP.md) sets product direction and release sequence. [Linear](BACKLOG.md) tracks execution. Use the [release process](RELEASE_PROCESS.md) for commands, the [release checklist](RELEASE_CHECKLIST.md) for readiness checks, and `release/decisions/` for the owner’s decision on a specific artifact.

## Versions and audiences

Define the intended outcome, audience and supported configuration for each candidate. Wider distribution requires evidence for that audience. The [October 23 plan](roadmaps/archive/2026-10-23-initial-release-roadmap.md) records the original v0.1–v0.2 targets.

Before v1.0, the version numbers describe intended scope rather than compatibility promises. Increment the `CFBundleVersion` build number for each external candidate even when the public version is unchanged. Maintain the installed-app identity and test upgrades against existing workspaces.

## Review and release timing

Schedule a review when there is evidence for a decision. Two-week planning cycles and Friday reviews are optional. Hold when an essential gate fails. An urgent hotfix may be reviewed as soon as the relevant regression checks pass and requires its own recorded release decision.

**Scope reduction is the preferred alternative to guessing.** If an ATS workflow or optional AI provider is not qualified, exclude its claim or disable/omit that offering. Do not claim that passing one provider fixture proves the others.

## Release gates

Record each gate as **PASS**, **RESTRICTED**, or **BLOCKED**, with an evidence link or explicit reason. A restricted gate must name the excluded platform, provider, feature, or audience and describe a viable fallback. A blocker prevents the affected release promise from shipping.

| Gate | Required question | Minimum evidence |
| --- | --- | --- |
| 1. Frozen candidate | Do we know exactly what was built? | Git SHA, version/build, platform, artifact SHA-256, passing CI |
| 2. Data integrity | Will saved work survive, remain correctly associated, and migrate safely? | Restart, resume/document hashes, answer/letter/history association, upgrade check |
| 3. Applicant authority and privacy | Did only explicitly authorized material/actions leave YourMove? | Controlled scope tests, manual-value preservation, no automatic submission, source/consent checks |
| 4. Core workflow | Can a job seeker prepare, inspect, submit personally, and record an application? | Native user-path acceptance on the advertised platform, with honest manual fallbacks |
| 5. Distribution | Can the advertised audience actually install and open this artifact? | Real downloaded-archive test; for normal public Mac installation, Developer ID/notarization and Gatekeeper acceptance |
| 6. Recovery | Can the user continue after an expected failure without losing local work? | Cancel/error/relaunch scenarios, clear failure state, no unsafe automatic replay |

### Non-negotiable blockers

- Silent loss, corruption, or misassociation of applicant documents, answers, or submission history.
- Browser mutation or employer submission outside explicit applicant authorization.
- Unexpected disclosure of sensitive applicant material.
- Failure to install or open under the advertised distribution experience.
- A repeatable primary-path failure with no usable recovery or honest scope restriction.

Cosmetic imperfections, unsupported fields with a clear manual fallback, or an optional slow feature can be acceptable in an invited beta **if users can still complete the promised journey**. Do not convert these categories into a weighted score that could outweigh a safety blocker.

## Historical evidence minimum for v0.1.0

- Both CI jobs passing for the exact candidate source.
- At least two distinct application-preparation journeys reaching a reviewable employer form, with supported scope documented.
- Persistence and correct material association after restart; include a version-upgrade check before the first patch release.
- Controlled tests confirming no unapproved overwrite, duplicate upload, or automatic employer submission.
- Downloaded-install acceptance for the chosen distribution path.
- A release decision with known defects, excluded functionality, and owner sign-off.

Two journeys are an acceptance sample, **not universal ATS support**. An optional AI task that makes the interactive application unusable must be isolated or omitted from the advertised v0.1.0 experience.

## Release health and feedback

For early testers, use voluntary reports or explicitly opt-in, sanitized diagnostics. Avoid recording private resumes, employer-form values, credentials, cookies, or session material.

Review preparation completion, time to reviewable form, manual corrections, blocked-workflow rate, data-integrity incidents, and install success. Define denominators and collect real observations before setting quantitative thresholds; a confirmed data-loss event can outweigh a favorable aggregate metric.

## Decision and authority

The release owner records a decision per frozen artifact using `release/decisions/v<version>.md`: **SHIP**, **SHIP RESTRICTED**, or **HOLD**. Include the supported audience, evidence, known defects, exclusions, and date. Distribution requires the release owner’s explicit authorization for that artifact.
