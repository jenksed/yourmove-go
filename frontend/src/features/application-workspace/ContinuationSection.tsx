// YM-004C employer continuation inside the React Application Workspace.
// Reviews the exact Go-derived transfer scope, authorizes only selected
// material through the existing guarded commands, and renders truthful
// browser outcomes. Submitting the application stays a manual,
// personally completed act recorded separately.

import { useCallback, useEffect, useRef, useState } from "react";
import {
  authorizeContinue,
  browserAction,
  fetchContinuation,
  type BrowserAction,
  type ContinuationSnapshot,
} from "./continuationApi";

function outcomeText(status: string): string {
  switch (status) {
    case "filled":
      return "Filled and verified in the browser";
    case "correct":
      return "Already present; left unchanged";
    case "uncertain":
      return "Uncertain; inspect this field in Chromium before continuing";
    case "complete":
      return "Complete";
    case "waiting":
      return "Waiting for the current form";
    case "checking":
      return "Checking the current form";
    case "attention":
      return "Requires manual work";
    case "review":
      return "Review in Chromium";
    case "selected":
      return "Selected in browser";
    default:
      return status === "" ? "Not started" : status;
  }
}

export function ContinuationSection({ jobId, materialRevision }: { jobId: number; materialRevision: number }): React.JSX.Element {
  const [snapshot, setSnapshot] = useState<ContinuationSnapshot | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [loadError, setLoadError] = useState<string>("");
  const [wantResume, setWantResume] = useState<boolean>(false);
  const [wantLetter, setWantLetter] = useState<boolean>(false);
  const [wantAnswers, setWantAnswers] = useState<boolean>(false);
  const [letterMode, setLetterMode] = useState<string>("file");
  const [confirmRestart, setConfirmRestart] = useState<boolean>(false);
  const [busy, setBusy] = useState<boolean>(false);
  const [actionError, setActionError] = useState<string>("");
  const [actionNotice, setActionNotice] = useState<string>("");
  const selectionInitialized = useRef({ resume: false, letter: false, answers: false });
  const pollRef = useRef<number | null>(null);
  const requestRef = useRef<number>(0);

  const reload = useCallback(async () => {
    const request = ++requestRef.current;
    try {
      const next = await fetchContinuation(jobId);
      if (request !== requestRef.current) return;
      setSnapshot(next);
      setLoadError("");
    } catch (error) {
      if (request !== requestRef.current) return;
      setLoadError(error instanceof Error ? error.message : "Unable to load employer continuation");
    } finally {
      if (request === requestRef.current) setLoading(false);
    }
  }, [jobId]);

  useEffect(() => {
    selectionInitialized.current = { resume: false, letter: false, answers: false };
    setWantResume(false);
    setWantLetter(false);
    setWantAnswers(false);
    setSnapshot(null);
    setLoading(true);
    void reload();
  }, [reload, materialRevision]);

  useEffect(() => {
    if (!snapshot) return;
    const prep = snapshot.preparation;
    if (prep.includeResume && !selectionInitialized.current.resume) {
      selectionInitialized.current.resume = true;
      setWantResume(true);
    }
    if (prep.includeLetter && !selectionInitialized.current.letter) {
      selectionInitialized.current.letter = true;
      setWantLetter(true);
    }
    if (prep.includeAnswers && !selectionInitialized.current.answers) {
      selectionInitialized.current.answers = true;
      setWantAnswers(true);
    }
  }, [snapshot]);

  const needsPoll = snapshot !== null && (
    snapshot.browser.active ||
    snapshot.browser.phase === "opening" ||
    ["waiting", "checking"].includes(snapshot.browser.answer_status) ||
    ["waiting", "checking"].includes(snapshot.browser.letter_status) ||
    snapshot.browser.contact_status === "waiting" ||
    snapshot.browser.contact_status === "checking"
  );

  useEffect(() => {
    if (!needsPoll) {
      if (pollRef.current !== null) {
        window.clearInterval(pollRef.current);
        pollRef.current = null;
      }
      return;
    }
    if (pollRef.current !== null) return;
    pollRef.current = window.setInterval(() => {
      void reload();
    }, 4000);
    return () => {
      if (pollRef.current !== null) {
        window.clearInterval(pollRef.current);
        pollRef.current = null;
      }
    };
  }, [needsPoll, reload]);

  const runBrowser = useCallback(
    async (action: BrowserAction) => {
      setActionError("");
      setActionNotice("");
      setBusy(true);
      try {
        const result = await browserAction(jobId, action, action === "restart" ? confirmRestart : false);
        if (!result.ok) {
          setActionError(result.message ?? "The browser operation could not complete.");
        } else if (action === "stop") {
          setActionNotice("Employer browser closed. Saved materials in YourMove are unchanged.");
        }
        await reload();
      } catch (error) {
        setActionError(error instanceof Error ? error.message : "The browser operation could not complete.");
      } finally {
        setBusy(false);
      }
    },
    [jobId, confirmRestart, reload],
  );

  const runAuthorize = useCallback(async () => {
    if (!snapshot) return;
    setActionError("");
    setActionNotice("");
    setBusy(true);
    try {
      const result = await authorizeContinue(jobId, {
        destination: snapshot.destination,
        include_resume: wantResume && snapshot.preparation.includeResume,
        resume_id: snapshot.resume ? String(snapshot.resume.id) : "",
        resume_hash: snapshot.resume ? snapshot.resume.sha256 : "",
        transfer_contacts: wantResume && snapshot.preparation.includeResume && snapshot.profile.approved,
        profile_revision: String(snapshot.profile.revision),
        include_letter: wantLetter && snapshot.preparation.includeLetter,
        draft_id: snapshot.letter ? String(snapshot.letter.id) : "",
        body_hash: snapshot.letter ? snapshot.letter.contentHash : "",
        mode: letterMode,
        include_answers: wantAnswers && snapshot.preparation.includeAnswers,
        review_hash: snapshot.preparation.answerReviewHash,
      });
      const failed = result.items.filter((item) => !item.ok);
      if (failed.length === 0) {
        setActionNotice("Authorization recorded. Review the browser outcomes below; nothing is submitted automatically.");
        setWantResume(false);
        setWantLetter(false);
        setWantAnswers(false);
      } else {
        setActionError(failed.map((item) => `${item.item}: ${item.detail}`).join(" "));
      }
      await reload();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "Authorization failed. Nothing was transferred by this attempt.");
    } finally {
      setBusy(false);
    }
  }, [snapshot, jobId, wantResume, wantLetter, wantAnswers, letterMode, reload]);

  if (loading) return <p role="status">Loading employer continuation…</p>;
  if (loadError !== "" && !snapshot) {
    return (
      <section className="panel" aria-label="Continuation loading error">
        <p>{loadError}</p>
        <button className="button button-secondary" type="button" onClick={() => void reload()}>
          Retry
        </button>
      </section>
    );
  }
  if (!snapshot) return <></>;

  const prep = snapshot.preparation;
  const browser = snapshot.browser;
  const activeTransfer = browser.resume_status !== "" || browser.contact_status !== "" || browser.letter_status !== "" || browser.answer_status !== "";

  return (
    <section className="panel" aria-labelledby="continuation-title" id="employer">
      <div className="section-heading">
        <div>
          <p className="eyebrow">EMPLOYER CONTINUATION</p>
          <h2 id="continuation-title">Review, authorize, and observe</h2>
          <p className="section-subtitle">
            YourMove proposes the exact scope below. Only material you select is authorized, and only the
            employer browser carries it out. You still submit personally.
          </p>
        </div>
        <a className="button button-secondary" href={`/jobs/${jobId}/inspection`}>
          Browser details
        </a>
      </div>

      <dl className="fact-list workspace-facts">
        <div>
          <dt>Employer destination</dt>
          <dd className="workspace-url">{snapshot.destination}</dd>
        </div>
        <div>
          <dt>Browser session</dt>
          <dd>
            {browser.active ? `Open in this session${browser.phase !== "" ? ` · ${browser.phase}` : ""}` : browser.phase === "interrupted" ? `Interrupted · ${browser.message}` : "Not open"}
          </dd>
        </div>
      </dl>

      {snapshot.otherBrowser && (
        <p className="writing-notice" role="note">
          Another application browser is open for {snapshot.otherBrowser.title}. Return to that application to
          preserve its browser work, or close its Chromium window before opening this job.
        </p>
      )}
      {snapshot.savedBrowser && !browser.active && (
        <p className="writing-notice" role="note">
          The employer browser disconnected. Your saved materials remain. Last checked {snapshot.savedBrowser.checkedAt} ·{" "}
          {snapshot.savedBrowser.fieldCount} observed controls. Opening again starts a fresh browser; browser-only
          values are not restored and previous uploads are not replayed.
        </p>
      )}
      {browser.phase === "attention" && browser.message !== "" && (
        <p className="writing-notice" role="alert">
          {browser.message}
        </p>
      )}
      {!snapshot.ownsPosting && (
        <p className="writing-notice" role="note">
          Automatic filling is not available for this job site yet. Open the application, attach your selected
          documents manually, and complete the form yourself.
        </p>
      )}

      <div className="workspace-source-actions">
        {!snapshot.editingAllowed ? (
          <small>This application is recorded as complete. Browser assistance is read-only history below.</small>
        ) : !browser.active ? (
          <button className="button button-primary" type="button" disabled={busy} onClick={() => void runBrowser("start")}>
            {busy ? "Opening…" : "Open application"}
          </button>
        ) : (
          <>
            <button className="button button-secondary" type="button" disabled={busy} onClick={() => void runBrowser("observe")}>
              Recheck form
            </button>
            <button className="button button-secondary" type="button" disabled={busy} onClick={() => void runBrowser("stop")}>
              Close browser
            </button>
          </>
        )}
        <small>Opening sends no saved material. Recheck the form before authorizing a transfer.</small>
        {browser.resume_retryable && (
          <button className="button button-secondary" type="button" disabled={busy} onClick={() => void runBrowser("retry-resume")}>
            Retry resume check
          </button>
        )}
        {browser.active && (
          <details>
            <summary>Reopen application</summary>
            <p className="small-muted">
              Reopening starts a fresh browser; browser-only values and attachment state may be lost. Previous
              uploads will not be replayed.
            </p>
            <label className="confirm-row">
              <input type="checkbox" checked={confirmRestart} onChange={(event) => setConfirmRestart(event.target.checked)} />
              <span>I understand reopening may lose browser-only work.</span>
            </label>
            <button className="button button-secondary" type="button" disabled={busy || !confirmRestart} onClick={() => void runBrowser("restart")}>
              Reopen application
            </button>
          </details>
        )}
      </div>

      <div className="continuation-scope" aria-label="Transfer scope">
        <h3>Exact transfer scope</h3>
        {snapshot.resume ? (
          <div className="continuation-item">
            <label className="confirm-row">
              <input type="checkbox" checked={wantResume && prep.includeResume} disabled={!prep.includeResume} onChange={(event) => setWantResume(event.target.checked)} />
              <span>
                <strong>Résumé:</strong> {snapshot.resume.name} · {snapshot.resume.kind} · Version {snapshot.resume.version}
                {snapshot.resume.valid ? "" : " · unavailable"}
              </span>
            </label>
            {!prep.includeResume && <small>{snapshot.handoff.reason}</small>}
            {prep.includeResume && snapshot.profile.approved && (
              <p className="small-muted">
                Contact information to fill: {snapshot.profile.fullName} · {snapshot.profile.email}
                {snapshot.profile.phone !== "" ? ` · ${snapshot.profile.phone}` : ""} (approved revision{" "}
                {snapshot.profile.revision})
              </p>
            )}
            {prep.includeResume && !snapshot.profile.approved && (
              <p className="small-muted">
                Contact filling disabled — <a href="/candidate">complete the candidate profile</a> to include contacts.
                Résumé attachment remains available.
              </p>
            )}
          </div>
        ) : (
          <p className="small-muted">No résumé selected. {snapshot.handoff.reason}</p>
        )}

        {snapshot.letter ? (
          <div className="continuation-item">
            <label className="confirm-row">
              <input type="checkbox" checked={wantLetter && prep.includeLetter} disabled={!prep.includeLetter} onChange={(event) => setWantLetter(event.target.checked)} />
              <span>
                <strong>Cover letter:</strong> Draft {snapshot.letter.id} · {snapshot.letter.provider}
              </span>
            </label>
            {!prep.includeLetter && snapshot.browser.letter_status === "" && (
              <small>A letter transfer is already recorded for this session or no draft is ready.</small>
            )}
            {prep.includeLetter && (
              <div className="field">
                <label htmlFor="letter-mode">Employer cover-letter control</label>
                <select id="letter-mode" value={letterMode} onChange={(event) => setLetterMode(event.target.value)}>
                  <option value="file">Attach saved letter as PDF</option>
                  <option value="text">Fill an explicitly labeled cover-letter text field</option>
                </select>
              </div>
            )}
          </div>
        ) : (
          <p className="small-muted">No saved cover-letter draft. Prepare one above to include it.</p>
        )}

        {snapshot.pendingAnswers.length > 0 ? (
          <div className="continuation-item">
            <label className="confirm-row">
              <input type="checkbox" checked={wantAnswers && prep.includeAnswers} disabled={!prep.includeAnswers} onChange={(event) => setWantAnswers(event.target.checked)} />
              <span>
                <strong>
                  {snapshot.pendingAnswers.length} saved {snapshot.pendingAnswers.length === 1 ? "answer" : "answers"} ready for transfer
                </strong>
              </span>
            </label>
            <dl className="contact-facts pending-answer-scope">
              {snapshot.pendingAnswers.map((answer) => (
                <div key={answer.key}>
                  <dt>{answer.label}</dt>
                  <dd>
                    {answer.value} — {answer.source} · Eligible for transfer
                  </dd>
                </div>
              ))}
            </dl>
            {!prep.includeAnswers && <small>Answer transfer is waiting, checking, or already covered in this session.</small>}
          </div>
        ) : (
          <p className="small-muted">No saved answers pending transfer.</p>
        )}

        {(snapshot.previousCount > 0 || snapshot.awaitingReconciliation > 0) && (
          <p className="small-muted">
            {snapshot.previousCount > 0 && `${snapshot.previousCount} earlier saved answers are excluded from transfer. `}
            {snapshot.awaitingReconciliation > 0 && `${snapshot.awaitingReconciliation} early answers await reconciliation and are excluded.`}
          </p>
        )}
      </div>

      {actionError !== "" && (
        <p className="job-form-error" role="alert">
          {actionError}
        </p>
      )}
      {actionNotice !== "" && (
        <p className="form-notice" role="status">
          {actionNotice}
        </p>
      )}
      <div className="form-actions">
        <button
          className="button button-primary"
          type="button"
          disabled={busy || !browser.active || (!(wantResume && prep.includeResume) && !(wantLetter && prep.includeLetter) && !(wantAnswers && prep.includeAnswers))}
          onClick={() => void runAuthorize()}
        >
          {busy ? "Filling…" : "Fill selected information"}
        </button>
      </div>
      <p className="small-muted">
        The employer may receive the selected material immediately. Unselected material is never transferred.
        Existing manual entries and attachments are preserved. You review and submit personally.
      </p>

      {activeTransfer && (
        <div className="continuation-outcomes" aria-label="Transfer outcomes">
          <h3>{browser.active ? "Observed in Chromium" : "Last observed browser results"}</h3>
          {browser.resume_status !== "" && (
            <p>
              <strong>Résumé · {outcomeText(browser.resume_status)}</strong>
              <br />
              <small>
                {browser.resume_name} · {browser.resume_message}
              </small>
            </p>
          )}
          {browser.contact_summary !== "" && <p>{browser.contact_summary}</p>}
          {(browser.contact_outcomes ?? []).filter((o) => o.status === "uncertain").map((o) => (
            <p key={o.key} role="status" className="inline-hint">
              <strong>{o.key} · Uncertain transfer</strong> · {o.detail}
            </p>
          ))}
          {browser.letter_status !== "" && (
            <p>
              <strong>Cover letter · {outcomeText(browser.letter_status)}</strong> · {browser.letter_message}
              {(browser.letter_status === "complete" || browser.letter_status === "attention") && (
                <small>
                  {" "}
                  The saved local draft may differ from the letter previously sent. Inspect the existing employer
                  attachment or text manually; local edits do not authorize automatic replacement.
                </small>
              )}
            </p>
          )}
          {browser.answer_status !== "" && (
            <details open={browser.answer_status === "complete"}>
              <summary>Answer transfer · {outcomeText(browser.answer_status)}</summary>
              {browser.answer_status === "complete" && prep.answerSummary !== "" && <p>{prep.answerSummary}.</p>}
              <ul className="transfer-contact-outcomes">
                {(browser.answer_outcomes ?? []).map((outcome, index) => (
                  <li key={index}>
                    <strong>{outcome.label}</strong>
                    <span>
                      {outcomeText(outcome.status)}
                      {outcome.detail ? ` · ${outcome.detail}` : ""}
                    </span>
                  </li>
                ))}
              </ul>
            </details>
          )}
          <p className="small-muted">
            Employer acceptance not established. Review attachments, filled text and manual fields in the employer
            browser; submit personally.
          </p>
        </div>
      )}
    </section>
  );
}
