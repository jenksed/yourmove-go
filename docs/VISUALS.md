# YourMove v0.1 visual direction and image slots

Use YourMove's **existing brand assets**, not a new AI-generated logo:

- `static/yourmove-wordmark-primary.svg` — light-background wordmark
- `static/yourmove-wordmark-reverse.svg` — dark-background wordmark
- `static/yourmove-symbol-primary.svg` — brand symbol
- `static/yourmove-logo.png` — existing raster asset

All paths above are relative to the repository root. If preparing social graphics, place the official logo manually in a design editor *after* creating the background. AI image tools routinely distort lettering.

## Visual language

- **Mood:** capable, modern, focused, hopeful without being motivational-poster cheesy.
- **Palette:** charcoal, warm off-white, graphite, and restrained cobalt blue highlights; use the existing logo as authored. Keep greens out of the primary visual system.
- **Design language:** editorial typography, generous negative space, subtle layered shapes and directional movement. Human-scale job-search artifacts, not sci-fi dashboards.
- **Avoid:** robots, glowing AI brains, faceless crowds, fake customer testimonials, made-up product UI, fake employer logos, imaginary metrics, and screenshots containing personal information.
- **Truth requirement:** product screenshots must come from the real application. AI can generate supporting editorial art or backgrounds, *not* evidence of functioning features.

## Slot 01 — Product preview (README, after introduction)

**Deliverable:** `docs/assets/opportunities.png`, ideally 1600–2000px wide.

**Source:** a real capture of the actual Opportunities page, loaded with synthetic jobs. Crop browser chrome as needed, but do not manipulate the application UI or invent states. Prefer a clean light appearance and a legible viewport.

**AI use:** optional decorative background only; never generate the UI itself.

**Alt text:** “YourMove Opportunities workspace showing saved jobs, priorities, and next actions.”

**README replacement:** replace the Slot 01 HTML comment and placeholder quote with:

```md
![YourMove Opportunities workspace showing saved jobs and next actions](docs/assets/opportunities.png)
```

## Slot 02 — Wails + local browser (README, under two modes)

**Deliverable:** `docs/assets/desktop-browser.png`, ideally 1600x900 or 1800x1000.

Use **two real screenshots** showing the same job workspace, one from the Wails desktop window and one from `127.0.0.1:3000`. Composite them manually on a subtle background, with two small factual labels added in the editor: “Wails desktop” and “Local browser.” Do not suggest the browser can access a remote hosted service.

**Optional AI background prompt:**

> Minimal premium editorial technology backdrop for a two-panel software product comparison. Warm off-white canvas, quiet graphite shapes and a restrained cobalt accent that suggests forward motion. Two large clear empty areas for real screenshots to be composited later. No text, no logos, no interface elements, no computers, no people, no gradients that reduce contrast. Flat and clean, 16:9 composition.

**Alt text:** “The same YourMove job workspace shown in a Wails desktop window and a local web browser.”

## Slot 03 — Feedback invitation (README, near feedback CTA)

**Deliverable:** `docs/assets/feedback-invite.png`, preferably 1600x900.

**AI image prompt:**

> Editorial graphic for an early-access job application organizer named YourMove. A small ordered stack of paper-like application cards transforms into one clear forward path. Elegant geometry, charcoal and graphite on an off-white field, one restrained cobalt blue line marking the next step. Considerable negative space on the right for later typography and official branding. Mature, approachable, authentic, no humans, no robot or AI imagery, no fake UI, no logos, no text. 16:9 landscape.

Overlay this copy *manually* in an editor: **“Try YourMove. Tell us where it gets in the way.”** Add the existing YourMove wordmark. Do not add invented reviews or counts.

**Alt text:** “YourMove invitation to try the early preview and share feedback.”

## Slot 04 — Public sharing / social launch image

**Deliverable:** `docs/assets/social-launch.png`, 1200x630 for link previews or social posts (can be adapted to square/4:5 crops). Not required in the README until verified.

**AI image prompt:**

> Clean editorial social media background for a local-first job application organizer. A single strong diagonal movement of layered charcoal paper forms toward a cobalt blue opening; subtle tangible texture, confident whitespace, elegant restrained composition. Dark charcoal left third reserved for an existing brand wordmark to be overlaid later; no generated typography, no logos, no glowing technology symbols, no dashboard mockups, no people. Designed for readable social-media thumbnail crops at 1200x630.

Overlay manually: **“Your search. Your decisions. YourMove.”** Keep a second line optional: **“v0.1 early access — testers wanted.”** Include only links and availability claims that are true when posting.

## Asset acceptance and publishing

1. Capture actual working product screens from the candidate release build using synthetic data.
2. Generate only supporting visuals; add all logos and text using the real assets in an editor.
3. Export optimized PNG or WebP with readable text at phone widths.
4. Add meaningful alt text in the README. Do not ship a broken image link.
5. Remove visible placeholder quotes and comments when each slot is populated.
6. Recheck that the depicted UI and advertised features match the released commit.

A screenshot is more persuasive than an AI render pretending to be one.
