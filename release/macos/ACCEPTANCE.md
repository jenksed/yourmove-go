# YourMove macOS release acceptance record

Fill in this record for the exact candidate under test. Passing Go tests or
building a signed bundle does **not** prove the graphical or downloaded install.
Use synthetic candidate/job data; never commit personal resumes, employer forms,
credentials, or unredacted screenshots.

- Source commit:
- Candidate ZIP SHA-256:
- App version/build:
- macOS version and architecture:
- Chrome version / browser prerequisite:
- Test method (local executable / extracted ZIP / browser-downloaded ZIP):
- Tester/date:

## Manual checks

- [ ] Extract ZIP and verify app name/icon and ordinary Finder launch.
- [ ] Confirm setup with no developer tools, repository, or cached Chromium dependency.
- [ ] Confirm default workspace location; use synthetic test records.
- [ ] Add and edit an opportunity, quit, relaunch, and confirm persistence.
- [ ] Import PDF and DOCX documents; cancel native picker once.
- [ ] Save a resume selection; restart; verify the exact selection and stored bytes.
- [ ] Export PDF/DOCX and a saved-letter PDF; verify the bytes and Save/Cancel.
- [ ] Confirm configured AI generation requires consent, preserves edits, and handles failures.
- [ ] Confirm supported Chromium handoff with a controlled fixture, preserving manual values and avoiding duplicate uploads.
- [ ] Confirm YourMove does not submit employer applications automatically.
- [ ] Confirm behavior when Chrome or AI provider is not configured.
- [ ] Confirm an actual browser-downloaded archive opens under ordinary Gatekeeper protections (public distribution gate; requires Developer ID/notarization for normal UX).

## Disposition

- Result: PASS / CONDITIONAL / FAIL
- Failures, exclusions and evidence (sanitized):
- Release authorization and date:

**Current limit:** local ad-hoc signing and a working ZIP are not Developer ID
signing/notarization. Do not describe an unnotarized build as a normal public
macOS installer.
