// YM-004C manual submission recording inside the React workspace.
// Recording is the applicant's explicit declaration of a personally
// completed submission. It never submits to an employer, and browser
// activity never marks the job Applied on its own.

import { useCallback, useEffect, useState } from "react";
import {
  fetchSubmissions,
  mintSubmissionReview,
  moveNext,
  recordSubmission,
  type SubmissionReview,
  type SubmissionsSnapshot,
} from "./continuationApi";

export function SubmissionSection({ jobId }: { jobId: number }): React.JSX.Element {
  const [history, setHistory] = useState<SubmissionsSnapshot | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [loadError, setLoadError] = useState<string>("");
  const [review, setReview] = useState<SubmissionReview | null>(null);
  const [reviewBusy, setReviewBusy] = useState<boolean>(false);
  const [reviewError, setReviewError] = useState<string>("");
  const [notes, setNotes] = useState<string>("");
  const [confirmed, setConfirmed] = useState<boolean>(false);
  const [submitBusy, setSubmitBusy] = useState<boolean>(false);
  const [submitError, setSubmitError] = useState<string>("");
  const [submitNotice, setSubmitNotice] = useState<string>("");
  const [nextBusy, setNextBusy] = useState<boolean>(false);

  const reload = useCallback(async () => {
    setLoading(true);
    setLoadError("");
    try {
      setHistory(await fetchSubmissions(jobId));
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : "Unable to load submission history");
    } finally {
      setLoading(false);
    }
  }, [jobId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const startReview = useCallback(async () => {
    setReviewBusy(true);
    setReviewError("");
    try {
      const next = await mintSubmissionReview(jobId);
      setReview(next);
      setConfirmed(false);
      setSubmitNotice("");
    } catch (error) {
      setReviewError(error instanceof Error ? error.message : "Unable to prepare submission confirmation");
    } finally {
      setReviewBusy(false);
    }
  }, [jobId]);

  const submit = useCallback(async () => {
    if (!review) return;
    setSubmitBusy(true);
    setSubmitError("");
    try {
      await recordSubmission(jobId, notes, review.token);
      setSubmitNotice("Recorded. Employer receipt is not independently verified.");
      setReview(null);
      setNotes("");
      setConfirmed(false);
      await reload();
    } catch (error) {
      setSubmitError(
        error instanceof Error
          ? `${error.message} Check submission history before retrying.`
          : "Recording failed. Check submission history before retrying.",
      );
    } finally {
      setSubmitBusy(false);
    }
  }, [review, jobId, notes, reload]);

  const next = useCallback(async () => {
    setNextBusy(true);
    try {
      const result = await moveNext(jobId);
      window.location.href = result.target;
    } catch {
      setNextBusy(false);
    }
  }, [jobId]);

  if (loading) return <p role="status">Loading submission history…</p>;
  if (loadError !== "" && !history) {
    return (
      <section className="panel" aria-label="History loading error">
        <p>{loadError}</p>
        <button className="button button-secondary" type="button" onClick={() => void reload()}>
          Retry
        </button>
      </section>
    );
  }

  return (
    <section className="panel" aria-labelledby="submission-title" id="submission">
      <div className="section-heading">
        <div>
          <p className="eyebrow">APPLICATION TIMELINE</p>
          <h2 id="submission-title">Submission history</h2>
        </div>
        {history && history.count > 0 && <span className="summary-count">{history.count} recorded</span>}
      </div>

      {history && history.count > 0 ? (
        <>
          <p className="submission-success" role="status">
            <strong>Submission recorded locally.</strong> You confirmed submission; employer receipt is not
            independently verified.
          </p>
          <ol className="timeline">
            {history.submissions.map((item) => (
              <li key={item.id}>
                <span className="timeline-dot" aria-hidden="true" />
                <div className="timeline-content">
                  <strong>Submission recorded</strong>
                  <time>{item.submittedAt}</time>
                  <p>Résumé: {item.resumeName}</p>
                  {item.notes !== "" && <p className="timeline-note">{item.notes}</p>}
                </div>
              </li>
            ))}
          </ol>
          {history.next && (
            <div className="form-actions">
              <button className="button button-primary" type="button" disabled={nextBusy} onClick={() => void next()}>
                {nextBusy ? "Moving…" : `Next job → ${history.next.Title}`}
              </button>
            </div>
          )}
          <p className="small-muted">Closes this completed application&apos;s managed browser. Saved records stay available.</p>
          <p>
            <a href="/jobs">Back to Opportunities</a>
          </p>
        </>
      ) : (
        <>
          <div className="empty-state">
            <h3>No submissions recorded</h3>
            <p>After submitting on the employer site, record it here. Your application record will appear here.</p>
          </div>

          <details className="submission-disclosure" open={review !== null}>
            <summary>
              <span>Record completed application</span>
              <span className="summary-chevron" aria-hidden="true">
                ⌄
              </span>
            </summary>
            <div className="submission-disclosure-body">
              <div className="confirmation-note">
                This action records your confirmation. It does not submit an application or prove receipt by the
                employer.
              </div>
              {!review ? (
                <>
                  <p className="small-muted">
                    Reviewing binds this confirmation to your currently selected résumé. If the selection changes,
                    review again.
                  </p>
                  {reviewError !== "" && (
                    <p className="job-form-error" role="alert">
                      {reviewError}
                    </p>
                  )}
                  <button className="button button-secondary" type="button" disabled={reviewBusy} onClick={() => void startReview()}>
                    {reviewBusy ? "Preparing…" : "Review current selection for recording"}
                  </button>
                </>
              ) : (
                <>
                  <p>
                    <strong>Selected résumé:</strong> {review.resumeName !== "" ? review.resumeName : "No résumé assigned"}
                  </p>
                  <div className="field">
                    <label htmlFor="submission-notes">
                      Submission notes <span className="muted">(optional)</span>
                    </label>
                    <input
                      id="submission-notes"
                      value={notes}
                      maxLength={1000}
                      placeholder="Confirmation number or follow-up"
                      onChange={(event) => setNotes(event.target.value)}
                    />
                  </div>
                  <label className="confirm-row">
                    <input type="checkbox" checked={confirmed} onChange={(event) => setConfirmed(event.target.checked)} />
                    <span>I confirm that I submitted this application on the employer website.</span>
                  </label>
                  {submitError !== "" && (
                    <p className="job-form-error" role="alert">
                      {submitError}
                    </p>
                  )}
                  {submitNotice !== "" && (
                    <p className="form-notice" role="status">
                      {submitNotice}
                    </p>
                  )}
                  <div className="form-actions">
                    <button
                      className="button button-primary"
                      type="button"
                      disabled={submitBusy || !confirmed}
                      onClick={() => void submit()}
                    >
                      {submitBusy ? "Recording…" : "Record completed application"}
                    </button>
                    <button
                      className="button button-secondary"
                      type="button"
                      onClick={() => {
                        setReview(null);
                        setConfirmed(false);
                      }}
                    >
                      Discard review
                    </button>
                  </div>
                </>
              )}
            </div>
          </details>
        </>
      )}
    </section>
  );
}
