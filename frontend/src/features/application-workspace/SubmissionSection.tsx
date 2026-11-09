import { useCallback, useEffect, useRef, useState } from "react";
import {
  fetchContinuation,
  fetchSubmissions,
  mintSubmissionReview,
  moveNext,
  recordSubmission,
  unsubmit,
  StaleSubmissionError,
  type SubmissionsSnapshot,
} from "./continuationApi";
import { hasUnsaved } from "../../shared/unsaved";

export function SubmissionSection({ jobId, onRecorded, onCorrected }: { jobId: number; onRecorded: () => void; onCorrected: () => void }): React.JSX.Element {
  const [history, setHistory] = useState<SubmissionsSnapshot | null>(null);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [confirmNext, setConfirmNext] = useState(false);
  const [confirmRecord, setConfirmRecord] = useState(false);
  const [confirmCorrect, setConfirmCorrect] = useState(false);
  const token = useRef("");

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      setHistory(await fetchSubmissions(jobId));
    } finally {
      setLoading(false);
    }
  }, [jobId]);

  useEffect(() => {
    token.current = "";
    setConfirmNext(false);
    setConfirmRecord(false);
    setConfirmCorrect(false);
    setNotice("");
    void reload().catch(() => setError("Unable to load submission history."));
  }, [reload]);

  const submit = async () => {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      // Reuse the same token on retry; Go makes repeated submissions idempotent.
      if (!token.current) token.current = (await mintSubmissionReview(jobId)).token;
      await recordSubmission(jobId, "", token.current);
      setConfirmRecord(false);
      await reload();
      onRecorded();
    } catch (err) {
      if (err instanceof StaleSubmissionError) token.current = "";
      try {
        const current = await fetchSubmissions(jobId);
        setHistory(current);
        if (current.count > 0) {
          setConfirmRecord(false);
          onRecorded();
          return;
        }
        setError(err instanceof Error ? err.message : "Could not record submission. You may retry.");
      } catch {
        setError("Could not check submission history. Check it before trying again.");
        setHistory(null);
      }
    } finally {
      setBusy(false);
    }
  };

  const correct = async () => {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const result = await unsubmit(jobId);
      setConfirmCorrect(false);
      await reload();
      if (result.remaining === 0) {
        setNotice("Recording removed. This opportunity is back to review and editable; your saved answers, letters, and documents are unchanged.");
        onCorrected();
      } else {
        setNotice("Most recent recording removed. Older history remains, so this application stays recorded.");
        onRecorded();
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not correct this record. Check submission history before trying again.");
    } finally {
      setBusy(false);
    }
  };

  const next = async (proceed = false) => {
    setBusy(true);
    setError("");
    try {
      // Full-page navigation would discard answer and letter drafts, so
      // refuse while this page holds unsaved edits. Tab switches keep
      // their state and need no guard.
      if (hasUnsaved()) {
        setError("Save or discard your edits before moving on.");
        return;
      }
      const state = await fetchContinuation(jobId);
      const active = state.browser.active;
      // The macOS Wails host does not present window.confirm(), so moving
      // on with an open browser uses this inline confirmation instead.
      // Canceling leaves the session open; confirming closes it and moves on.
      if (active && !proceed) {
        setConfirmNext(true);
        return;
      }
      setConfirmNext(false);
      const result = await moveNext(jobId, active);
      window.location.href = result.target;
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not move to the next opportunity.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="panel" id="submission" aria-labelledby="submission-title">
      <h2 id="submission-title">Application status</h2>
      {loading ? (
        <p role="status">Loading application status…</p>
      ) : history === null ? (
        <>
          <p>Unable to confirm submission history.</p>
          <button type="button" onClick={() => void reload().catch(() => setError("Unable to load submission history."))}>Check history</button>
        </>
      ) : history.count > 0 ? (
        <>
          <p role="status"><strong>Submission recorded locally.</strong> Employer receipt is not independently verified.</p>
          <details>
            <summary>Submission history</summary>
            <ol>
              {history.submissions.map((item) => (
                <li key={item.id}>{item.submittedAt} · Résumé: {item.resumeName}{item.notes && ` · ${item.notes}`}</li>
              ))}
            </ol>
          </details>
          {!confirmCorrect ? (
            <div className="form-actions">
              <button className="button button-secondary" type="button" disabled={busy} onClick={() => { setError(""); setNotice(""); setConfirmCorrect(true); }}>
                Correct an accidental recording
              </button>
            </div>
          ) : (
            <div role="alert" aria-label="Confirm correcting the submission record">
              <p className="writing-notice">
                This removes the most recent local record only. Older history stays, and your saved answers,
                letters, and documents are unchanged. With no records left, this opportunity returns to review.
              </p>
              <div className="form-actions">
                <button className="button button-secondary" type="button" disabled={busy} onClick={() => void correct()}>Remove most recent record</button>
                <button className="button button-secondary" type="button" onClick={() => setConfirmCorrect(false)}>Keep history</button>
              </div>
            </div>
          )}
        </>
      ) : !confirmRecord ? (
        <>
          <p>After submitting on the employer website, record it here.</p>
          <button className="button button-primary" type="button" disabled={busy} onClick={() => { setError(""); setNotice(""); setConfirmRecord(true); }}>
            I submitted this application
          </button>
        </>
      ) : (
        <div role="alert" aria-label="Confirm recording the submission">
          <p className="writing-notice">
            Record that you personally submitted this application on the employer website?
            This keeps only a local note — employer receipt is not verified — and pauses editing here.
          </p>
          <div className="form-actions">
            <button className="button button-primary" type="button" disabled={busy} onClick={() => void submit()}>
              {busy ? "Recording…" : "Record submission"}
            </button>
            <button className="button button-secondary" type="button" onClick={() => setConfirmRecord(false)}>Not yet</button>
          </div>
        </div>
      )}
      {notice && <p className="form-notice" role="status">{notice}</p>}
      {error && <p className="job-form-error" role="alert">{error}</p>}
      {history && !confirmNext && (
        <div className="form-actions">
          <button className="button button-secondary" type="button" disabled={busy} onClick={() => void next()}>
            {history.next ? `Next opportunity → ${history.next.title}` : "Back to opportunities"}
          </button>
        </div>
      )}
      {history && confirmNext && (
        <div role="alert" aria-label="Confirm moving to the next opportunity">
          <p className="writing-notice">
            Moving on will close the employer browser. Unsaved work in that form may be lost. Close it and continue?
          </p>
          <div className="form-actions">
            <button className="button button-secondary" type="button" disabled={busy} onClick={() => void next(true)}>Close and continue</button>
            <button className="button button-secondary" type="button" onClick={() => setConfirmNext(false)}>Stay here</button>
          </div>
        </div>
      )}
    </section>
  );
}
