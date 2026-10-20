# G5.5 — Ashby resume handoff

## User flow

1. In a saved active Ashby job, choose a resume or accept an explainable role-family recommendation; **save** the selection.
2. In Prepare, review the exact document name and approve **Prepare resume handoff**. This approval authorizes selecting that exact file into this employer form, which may upload it to the employer immediately. No other candidate facts are transmitted.
3. A separate managed Chromium window opens on that saved Ashby job. Navigate its Apply, Next and conditional steps manually. YourMove's existing read-only watcher monitors pages and looks for one unambiguous, empty resume file control.
4. When found, it selects the SHA-256-verified resume from a private, temporary file bearing the human-readable saved filename. One attempt only: unknown, ambiguous, populated or failed controls yield a manual attachment message. The file-selection event may cause the employer page to upload or parse immediately.
5. Review the actual employer attachment and submission requirements. You handle all other questions, Continue, consent, final Submit and verification. YourMove does not infer application submission from browser activity.
6. Closing the managed browser removes temporary document copies and ends the authorization. The selected job resume remains in the YourMove Documents library.

## Limits and known unknowns

- Ashby only in this iteration. Browser identification requires a matching saved posting URL. Other ATS remain manual.
- Does not fill text, answer questions, click page navigation, or submit an application. Cross-origin embedded file controls, custom dropzones without file inputs, unsupported MIME formats, ambiguous controls, or a browser that failed to open require manual attachment.
- `DOM.setFileInputFiles` and browser `files` metadata show local selection, **not verified acceptance by Ashby**. A final confirmation in the employer UI remains essential.
- Automatic retry is deliberately disabled following attempted selection. Refreshing an Ashby form or closing Chromium can lose browser-only progress. New sessions require a new explicit approval.
- The Go tests are synthetic/local. The exact native WebView → Chromium → Ashby path requires Mac acceptance. See commands in the delivered response.
