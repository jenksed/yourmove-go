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

test("verified transfer remains historical after the saved letter is edited and Apply is reopened", () => {
  const savedLetter = { body: "Saved letter revision present at transfer time." };
  const transfer = { letter_status: "complete" };
  const transferredBody = savedLetter.body;
  savedLetter.body = "Edited saved letter revision after the verified transfer.";
  assert.notEqual(savedLetter.body, transferredBody);

  // The continuation exposes the session outcome but not the transferred
  // revision hash, so the completed status must describe only that history.
  const reopenedApplyStatus = coverLetterStatusText(transfer.letter_status, true);
  assert.match(reopenedApplyStatus, /earlier in this session/);
  assert.match(reopenedApplyStatus, /may differ from the current saved letter/);
  assert.doesNotMatch(reopenedApplyStatus, /current saved letter.{0,20}(selected|verified)/i);
});

test("attempted and pending transfers do not claim success", () => {
  assert.match(coverLetterStatusText("attention", true), /attempted.*uncertain/i);
  assert.match(coverLetterStatusText("waiting", true), /pending.*no result has been verified/i);
});
