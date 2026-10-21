# First trial run fixes

The job overview has a Copy job description control. It copies the entire rendered description as plain text, with paragraph breaks preserved. Clipboard failure is reported rather than claiming success. The existing edit textarea remains available without JavaScript.

Contact and free-text answer transfer now focuses before setting the native input value and dispatching input/change, then blurs before the delayed verification reads. This permits focus- and blur-driven employer handlers to run. Verification also checks native validity. This does not prove an employer's private application state accepted the value; live qualification remains necessary.

Approved candidate profile facts recognize ordinary LinkedIn labels with opaque employer identifiers, including LinkedIn Profile. Transfer excludes the existing LinkedIn authentication containers. Location combines the approved city, region and country; it does not hardcode or change the saved profile. Ashby location combobox transfer waits for a unique matching visible suggestion and clicks it. United States and USA are treated as equivalent for this exact match. Missing or ambiguous options remain uncertain. Active or nonempty manual entries are preserved. Controls that replace the input or show a selection chip can still require manual review.

## Evidence

- `go test ./...` passes with loopback listeners enabled.
- Opt-in Chromium qualification: `YOURMOVE_QUALIFY_BROWSER=1 go test -tags qualification ./internal/application/browser/chromium -run 'TestTrialBlurAndLocation|TestHostedPlatformForms' -count=1` passes. This runs the production CDP filler against local forms, checking values committed on blur, native form validity, an actual autocomplete option click, canonical location text, preservation of manual location, resume/letter attachments, and zero submissions.
- Real Ashby qualification remains **unresolved**: the browser runtime reported Chrome unavailable and no connected browsers. Local fixtures do not establish compatibility with a live tenant's React state, location markup, or validation. No application was submitted.
- Chromium clipboard test verifies that the full multi-paragraph plain-text payload reaches the clipboard API, including bullets and decoded ampersands. It uses a clipboard API stub; OS paste was not tested.
- Native Wails visual review and OS clipboard paste verification remain outstanding.

## CoreText warning investigation

Application styling uses the existing `-apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif` stack (`static/styles.css`). Application native sources and the installed Wails v3 beta.32 Darwin application sources contain no explicit `.SFNS`/`fontWithName` font lookup. No application-owned private font name was found, so no font infrastructure or CSS substitution was added.

The warning is most consistent with underlying macOS/WebKit font resolution, but its precise emitter and harmlessness have **not** been established without a reproducing native log/backtrace. WebKit documents that `.SFNS-*` system fonts cannot be reconstructed through `+[NSFont fontWithName:size:]` on macOS 10.15: https://bugs.webkit.org/show_bug.cgi?id=199788 . This supports framework provenance as an inference, not proof for this exact warning. A fallback warning alone is not evidence of an application failure, but a claim that it is wholly harmless would require checking native rendering. If reproduced, inspect the caller at `CTFontLogSystemFontNameRequest` before changing app styles.
