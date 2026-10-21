# YourMove interaction and composition rules

These are product-design constraints, not a mandate to create components or CSS abstractions. Apply rules to a real user decision, inspect the result in Wails, and revise when use contradicts the rule.

## R1 — Design around the current decision (active)

Organize the screen around the choice the user is making, **not** a collection of available features. The relevant object and next action should precede diagnostics, history, and optional work. The Prepare screen puts the selected resume and employer continuation before optional writing.

**Review:** Could a user determine the next action without scrolling through an explanation? If not, remove competing content before adding color or spacing.

## R2 — Make grouping earn its border (working)

Use section headings, rhythm, or a separator before creating another panel. A bordered surface is justified by an independent object or interaction scope, not merely because a group of data exists.

## R3 — Show meaningful state, not decorative status (working)

A badge, warning, or success state must change interpretation or action. "File selected" is not proof that an employer accepted it. "Submitted" in YourMove means the user recorded submission, not employer receipt.

## R4 — Keep optional capabilities discoverable but subordinate (working)

Use labeled native disclosure for optional or advanced tasks. Errors, consent, and blockers stay visible in the flow where they matter. Without JavaScript, disclosures and underlying forms must still work.

## R5 — One visual primary for each decision (working)

Promote the action that advances the visible task. Secondary actions can remain accessible without looking identical to the primary. Preserve explicit save and consent operations when they represent real boundaries.

## R6 — The existing artifact outranks the means of creating it (active, R2)

When a draft, document, or completed result exists, lead with viewing, editing, or using **that thing**. Put "create another," provider configuration, and source diagnostics after it. When no artifact exists, prioritize creation and show prerequisites.

**Failure exposed:** The cover-letter screen with a saved draft asked the user to configure a model and authorize another generation before showing their letter.

**Applied:** Prepare's saved-draft editor now leads. A labeled "Write another version" disclosure retains the unchanged generation form and consent. An empty writing desk leads with generation settings instead.

## R7 — Controls in a row must share a baseline (active, R2)

Adjacent select boxes, text inputs, and buttons must have consistent height, border treatment, type scale, and label spacing. Avoid relying on platform-native select rendering when it creates inconsistent dimensions. Check the actual macOS WebView, not solely Chrome screenshots.

**Failure exposed:** The provider dropdown was visually half the height of the model field. The result looked accidentally assembled even though both had CSS `min-height`.

**Applied:** Shared `.field` inputs and selects use a 42px height; select rendering uses a consistent chevron, padding, and border. Theme control and file inputs retain their dedicated treatment.

## R8 — Give working text a reading measure (active, R2)

A writing or reading task needs a bounded measure. Do not stretch paragraphs and editable prose to the full available application width. Use available window space to support the task, not to lengthen each line. Never let a control or long filename force the column wider than the viewport.

**Failure exposed:** The letter textarea and provider fields stretched across a wide desktop column, making the editor visually noisy and harder to scan.

**Applied:** Writing area capped around 780px; source evidence remains scrollable; metadata and helper copy use shorter measure. The wider application layout is not globally constrained.

## R9 — Spend words on consequential explanations (active, R2)

Do not repeat a task title in an eyebrow, heading, hero, section title, and first field label. Every sentence should resolve an action, boundary, missing prerequisite, or consequence. Replace decorative slogans with direct labels on operational screens. Important upload and external-provider consent must remain explicit even when lengthy.

**Failure exposed:** "Cover letter" was followed by a second writing-desk introduction, a slogan, numbered steps, and another "Your draft" heading before the actual editor.

**Applied:** The letter starts with "Edit your letter" or "Generate a cover letter". AI Connections uses a standard settings heading; Documents uses a compact import toolbar. Authorization copy is retained.

## R10 — A row should describe an object before exposing its maintenance tools (active, R2)

Document lists lead with filename, state, and the common action. Less frequent routing, revision, archival, and deletion stay available through one plainly labeled disclosure. Never bury a *mandatory* action required to finish the current task.

**Failure exposed:** Every resume row simultaneously showed role family routing, master controls, version import, archive, and deletion. Those controls overpowered the document identity.

**Applied:** Document rows show their content and Download; Manage document exposes the existing forms. No form endpoints or submission semantics changed.

## R2 change boundaries and evidence

- **Changed:** `templates/job.html`, `templates/ai.html`, `templates/resumes.html`, `static/styles.css`, this documentation, and the foundations document.
- **Preserved:** Resume save button and POST, existing draft/save route, independent cover-letter versions, model/provider selection, external consent, pinned resume handoff and explicit employer submission, Wails redirect compatibility.
- **Not changed:** Application backend, AI model capability, active session recovery, global shell/navigation, visual brand assets, document persistence.
- **Must verify in the actual Wails desktop:** Saved and empty cover-letter states, explicit consent, Save my edits, Write another version, select/input height parity, document Manage controls, dark mode, keyboard and 390px layout.

## Screen review worksheet

1. **User's immediate decision** and the artifact involved:
2. **Competing elements** that demand equal visual attention:
3. **Visible object/action first**; what is subordinate:
4. **Text, control, and width consistency** at 1280 and 390px:
5. **Preserved real-path behavior** and evidence actually observed:
