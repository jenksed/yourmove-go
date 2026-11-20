import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { coverLetterStatusText } from "./coverLetterStatus.ts";

test("Apply renders the transfer-derived letter status instead of inferring handling from selection", async () => {
  const component = await readFile(new URL("./ContinuationSection.tsx", import.meta.url), "utf8");
  assert.match(component, /coverLetterStatusText\(browser\.letter_status, snapshot\.ownsPosting\)/);
  const letterCardStart = component.indexOf("{snapshot.letter ? (");
  const letterCardEnd = component.indexOf("Cover letter not included — none selected.", letterCardStart);
  assert.ok(letterCardStart >= 0 && letterCardEnd > letterCardStart, "cover-letter card is present");
  assert.doesNotMatch(component.slice(letterCardStart, letterCardEnd), /Already handled in this session\./);
});

test("saved letter with no transfer is described as local preparation", () => {
  const text = coverLetterStatusText("", true);
  assert.match(text, /Saved locally/);
  assert.match(text, /no transfer/);
  assert.doesNotMatch(text, /handled in this session/i);
});

test("unsupported posting requires manual letter handling", () => {
  assert.match(coverLetterStatusText("", false), /unavailable.*Attach.*manually/i);
});

test("verified transfer retains accurate per-item feedback", () => {
  const text = coverLetterStatusText("complete", true);
  assert.match(text, /Selected and verified in the employer browser/);
  assert.match(text, /inspect it before submitting personally/);
});

test("attempted and pending transfers do not claim success", () => {
  assert.match(coverLetterStatusText("attention", true), /attempted.*uncertain/i);
  assert.match(coverLetterStatusText("waiting", true), /pending.*no result has been verified/i);
});
