# KAN-13 native Apply → Fill evidence

Owner accepted this bounded evidence and the profile-save error display fix on 2026-11-17. The native journey was observed on the clean source revision `168f05fa5688809c0cd8ee84715d8dbcef82401c`, before the separate display fix `7a164b44ed8f6e7bd0e9dac68c3d0e1bca708cc0`. The fix has focused test evidence; the complete native journey was not repeated on the fix. This report and screenshots are a subsequent evidence-only commit.

The ad-hoc-signed macOS Wails qualification executable `YourMoveReactFixture.app/Contents/MacOS/YourMoveReactFixture` was built by `scripts/build-react-native-fixture.sh` with `desktop,qualification`, `vcs.revision=168f05fa`, `vcs.modified=false`; SHA-256 `bf487205411b93213298d433e7706e47b48057f9b0959407ab435313024a3310`. The recorded environment was macOS 27.2.1 arm64, Wails 3.1.0 and Go 1.27.3. The workspace and employer form were synthetic and isolated. Screenshots are cropped to the relevant windows to omit unrelated desktop content; they show synthetic data only.

## Direct native observations

In the actual Wails window, a synthetic profile was approved, a résumé selected through the native dialog, a saved letter used, and a captured question answered. The applicant reviewed included materials and explicitly pressed Fill Application; the UI stated that Fill sends included saved information and does not submit. [Scope](scope.png), [feedback](fill-feedback.png), [employer form](employer-form.png).

- First name, last name and email: reported filled and verified; Alex, Fixture and alex@example.test were visible in the controlled employer form.
- Phone: reported not found on this page, with no supported control on the fixture step.
- Résumé and cover letter: named files observed in browser file inputs; employer upload acceptance remained unverified.
- Saved “Why this team?” answer: reported filled and independently observed in the form.
- The fixture displayed local submissions: 0; no submission receipt was created, local submissions remained zero, and the job stayed in review.
- After quitting and reopening the same executable against the same synthetic workspace without reseeding, the résumé, approved contact profile, answer and letter remained saved. [Restart](restart.png).

## Controlled tests and limitations

The qualification-tagged `TestControlledProfileFactFill` passed against a controlled managed browser: Website from Portfolio, separate GitHub, LinkedIn, Location autocomplete, Pronouns, ordinary answer, attachments, preservation of a prefilled employer last name, exclusion of work authorization, and zero submissions. These profile fields and manual-value preservation were not demonstrated in the native fixture, which lacks those controls or a prefilled value. KAN-10 focused rule tests also passed. The fix's `TestRejectedProfileSaveShowsPersistedApproval` passed with the fix and failed without it; neighboring profile tests, `go vet ./internal/app/`, and `git diff --check` passed in the prior run. The fix changes the error page to show persisted approval, while retaining rejected form inputs; failure to read the persisted profile falls back to Not approved, a nonblocking observation.

Narrow-screen and complete keyboard-only journeys were not verified. No employer submission, release/distribution acceptance, or real employer upload acceptance was performed. KAN-17 remains responsible for final integrated release qualification; this evidence does not establish it.
