# Questions capture remediation

Source: release branch, based on 11a653c0ef7087535ca202a02cfdc8f935826e45.
Production changes stay in the existing browser handler/session owner and Questions transport/UI. No persistence contract, Fill eligibility, or posting ownership policy changed.

Implemented:
- JSON browser operations return established error objects, including unsupported URLs and malformed forms.
- Launch failure retains the actual cause, including the single managed-session conflict.
- Questions polls and displays the existing capture phase and message. Initial waiting text refers to questions rather than résumé upload.
- Capture continues to use the read-only observer; no new transfer authorization is created.

Verification:
- Focused capture and Answer Library automatic preparation tests pass. The library test now captures its second application's equivalent question through start-only background observation, then checks existing exact/alternative/conflicting/saved-answer behavior.
- Controlled Chromium TestQuestionsCapturePollingPreservesDraft passes against the production frontend. A new inspection and attention status arrive while an editor holds unsaved text; polling preserves that text.
- go test ./... and go test -tags desktop ./... pass.
- pnpm build and git diff --check pass.
- Tests use disposable synthetic workspaces.

Native evidence is PARTIAL, not acceptance:
- Built with the existing qualification script in /private/tmp/yourmove-workflow-native-CB8uKM.
- Executable: YourMoveWorkflow.app/Contents/MacOS/YourMoveWorkflow.
- Executable SHA-256: ff04cbd73d7a9f42466874a540730db80d5e9cf2ea590b2b0a0224b1d7dfb1ab.
- Workspace: /private/tmp/yourmove-workflow-native-CB8uKM/workspace.
- Bundle identifier changed only in this disposable bundle to run.yourmove.questionsrepair to distinguish it from an already-running qualification app, then ad-hoc re-signed.
- Observed the synthetic job, captured employer questions, changing detected/watching status, and Documents activity in Wails. Capture was already opening at first observation, so agent initiation of that capture is not established.
- Repeated computer-use “user changed” guards interrupted interaction. Full Job → Documents → Questions → Capture → Review → Apply and native unsaved-edit recovery remain NOT VERIFIED. No native acceptance claim.
- Existing personal preview processes and their workspaces were not modified.

Original URL limitation:
The failing URL was not supplied. Its end-to-end resolution remains unverified.
Current supported saved identities are HTTPS Ashby /board/UUID; Greenhouse on boards.greenhouse.io or job-boards.greenhouse.io /board/jobs/positive-number; Lever on jobs.lever.co or jobs.eu.lever.co /board/UUID, without credentials, explicit ports, or encoded paths.
Other ATS hosts, custom employer domains, board indexes, incomplete IDs, HTTP URLs, and already-deep application paths as saved posting identities remain unsupported. Observation rejects cross-origin/neighbor-posting redirects; Ashby descendants and Lever /apply are allowed, while Greenhouse requires the same posting path.
Read-only capture for arbitrary destinations could be separated from transfer, but would need its own bounded association policy across browser tab selection, observation, and saved question selection. No broad or guessed policy was introduced without the original failing destination.
Before replacing the personal preview: complete native acceptance and resolve or explicitly accept the original URL limitation.
