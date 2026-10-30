// YM-004A React Application Workspace: one coherent preparation activity.
// Reads and writes through Go-owned JSON contracts only. The source URL is
// displayed, never edited here; resume bytes never enter this client.

import { useCallback, useEffect, useState } from "react";
import {
  assignWorkspaceResume,
  fetchWorkspace,
  refreshWorkspacePosting,
  saveWorkspaceDetails,
  workspaceIdFromPath,
  type WorkspaceSnapshot,
} from "./api";

function suggestionText(status: string): string {
  switch (status) {
    case "matched":
      return "Suggested for this role family. A suggestion never changes your saved selection until you save it.";
    case "no-resume":
      return "A family matched but has no selectable resume yet.";
    case "choose-primary":
      return "A family matched. Choose a primary resume for it in Documents to get one suggestion.";
    case "ambiguous":
      return "Several families matched equally. Refine title rules or select manually.";
    case "no-title":
      return "Add a job title to enable document suggestions.";
    default:
      return "No confident suggestion yet. Select a document manually or adjust role-family rules.";
  }
}

export function WorkspacePage(): React.JSX.Element {
  const jobId = workspaceIdFromPath(window.location.pathname);
  const [snapshot, setSnapshot] = useState<WorkspaceSnapshot | null>(null);
  const [loadError, setLoadError] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);

  const [title, setTitle] = useState<string>("");
  const [company, setCompany] = useState<string>("");
  const [description, setDescription] = useState<string>("");
  const [salary, setSalary] = useState<string>("");
  const [stage, setStage] = useState<string>("review");
  const [priority, setPriority] = useState<string>("normal");
  const [detailsError, setDetailsError] = useState<string>("");
  const [detailsNotice, setDetailsNotice] = useState<string>("");
  const [detailsBusy, setDetailsBusy] = useState<boolean>(false);

  const [resumeBusy, setResumeBusy] = useState<boolean>(false);
  const [resumeError, setResumeError] = useState<string>("");
  const [resumeNotice, setResumeNotice] = useState<string>("");
  const [refreshBusy, setRefreshBusy] = useState<boolean>(false);
  const [refreshError, setRefreshError] = useState<string>("");

  const reload = useCallback(async () => {
    if (jobId === null) {
      setLoadError("Unknown opportunity. Return to Opportunities and open a saved job.");
      setLoading(false);
      return;
    }
    setLoading(true);
    setLoadError("");
    try {
      const next = await fetchWorkspace(jobId);
      setSnapshot(next);
      setTitle(next.job.title);
      setCompany(next.job.company);
      setDescription(next.job.description);
      setSalary(next.job.salaryText);
      setStage(next.job.stage);
      setPriority(next.job.priority);
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : "Unable to load workspace");
    } finally {
      setLoading(false);
    }
  }, [jobId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const handleSaveDetails = useCallback(
    async (event: React.FormEvent) => {
      event.preventDefault();
      if (jobId === null) return;
      setDetailsError("");
      setDetailsNotice("");
      setDetailsBusy(true);
      try {
        await saveWorkspaceDetails(jobId, {
          title,
          company,
          description,
          salary_text: salary,
          stage,
          priority,
        });
        // Derived suggestions and readiness depend on job details, so reload
        // the complete Go snapshot after the durable write.
        await reload();
        setDetailsNotice("Saved. Your edits are kept across reloads and restarts.");
      } catch (error) {
        setDetailsError(error instanceof Error ? error.message : "Unable to save changes");
      } finally {
        setDetailsBusy(false);
      }
    },
    [jobId, title, company, description, salary, stage, priority, reload],
  );

  const handleAssign = useCallback(
    async (resumeId: number) => {
      if (jobId === null) return;
      setResumeError("");
      setResumeNotice("");
      setResumeBusy(true);
      try {
        await assignWorkspaceResume(jobId, resumeId);
        await reload();
        setResumeNotice(
          resumeId === 0 ? "Selection cleared." : "Selection saved for this opportunity.",
        );
      } catch (error) {
        setResumeError(
          error instanceof Error
            ? `${error.message} Your previous selection is unchanged.`
            : "Unable to save selection. Your previous selection is unchanged.",
        );
      } finally {
        setResumeBusy(false);
      }
    },
    [jobId, reload],
  );

  const handleRefresh = useCallback(async () => {
    if (jobId === null) return;
    setRefreshError("");
    setRefreshBusy(true);
    try {
      await refreshWorkspacePosting(jobId);
      await reload();
    } catch (error) {
      setRefreshError(error instanceof Error ? error.message : "Unable to refresh posting");
    } finally {
      setRefreshBusy(false);
    }
  }, [jobId, reload]);

  const applied = snapshot !== null && (snapshot.job.submissionCount > 0 || snapshot.job.stage === "applied");

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      <aside className="sidebar" aria-label="Primary navigation">
        <a className="brand" href="/jobs" aria-label="YourMove — Opportunities">
          <img
            className="brand-wordmark brand-light"
            src="/static/yourmove-wordmark-primary.svg"
            alt="YourMove"
            width="166"
            height="49"
          />
          <img
            className="brand-wordmark brand-dark"
            src="/static/yourmove-wordmark-reverse.svg"
            alt=""
            width="166"
            height="49"
          />
        </a>
        <nav className="primary-nav" aria-label="Workspace">
          <div className="nav-group">
            <p className="nav-caption">Work</p>
            <a className="nav-item" href="/jobs">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <rect x="3" y="7" width="18" height="14" rx="2" />
                <path d="M8 7V5a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M3 13h18" />
              </svg>
              Opportunities
            </a>
            <a className="nav-item" href="/applications">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="m4 12 5 5L20 6" />
                <path d="M20 13v6a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h10" />
              </svg>
              Submissions
            </a>
          </div>
          <div className="nav-group">
            <p className="nav-caption">Your materials</p>
            <a className="nav-item" href="/resumes">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8zM14 2v6h6M8 13h8M8 17h8" />
              </svg>
              Documents
            </a>
            <a className="nav-item" href="/candidate">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <circle cx="12" cy="8" r="4" />
                <path d="M4 21v-3a8 8 0 0 1 16 0v3" />
              </svg>
              Candidate profile
            </a>
            <a className="nav-item" href="/answer-library">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M5 4h14v16H5zM8 8h8M8 12h8M8 16h5" />
              </svg>
              Answer Library
            </a>
            <a className="nav-item" href="/career-evidence">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M4 19V5m0 14h16M7 15l4-4 3 2 5-6" />
                <circle cx="7" cy="15" r="1" />
                <circle cx="11" cy="11" r="1" />
                <circle cx="14" cy="13" r="1" />
                <circle cx="19" cy="7" r="1" />
              </svg>
              Career Evidence
            </a>
          </div>
        </nav>
        <div className="sidebar-bottom">
          <span className="local-dot" aria-hidden="true" />
          <span>Local workspace</span>
        </div>
      </aside>
      <div className="app-main">
        <main id="main-content" className="page-container workspace-page">
          <nav className="breadcrumbs" aria-label="Breadcrumb">
            <a href="/jobs">Opportunities</a>
            <span aria-hidden="true">/</span>
            <span>{snapshot ? snapshot.job.displayTitle : "Workspace"}</span>
          </nav>

          {loading && <p role="status">Loading application workspace…</p>}
          {loadError !== "" && (
            <section className="panel" aria-label="Loading error">
              <p>{loadError}</p>
              <button className="button button-secondary" type="button" onClick={() => void reload()}>
                Retry
              </button>
            </section>
          )}

          {snapshot && (
            <>
              <div className="page-heading">
                <div>
                  <p className="eyebrow">APPLICATION WORKSPACE</p>
                  <h1>{snapshot.job.displayTitle}</h1>
                  <p className="page-description">
                    {snapshot.job.companyOrHost} <span className="dot-sep" aria-hidden="true">·</span>{" "}
                    {snapshot.job.stageLabel} <span className="dot-sep" aria-hidden="true">·</span>{" "}
                    {snapshot.job.priorityLabel} priority
                  </p>
                </div>
                <div className="heading-actions">
                  <a className="button button-secondary" href="/jobs">
                    Opportunities
                  </a>
                  <a className="button button-secondary" href="/resumes">
                    Documents
                  </a>
                </div>
              </div>

              <section className="panel workspace-next" aria-labelledby="next-action-title">
                <p className="eyebrow">NEXT USEFUL ACTION</p>
                <h2 id="next-action-title">{snapshot.preparation.title}</h2>
                <p>{snapshot.preparation.detail}</p>
              </section>

              <section className="panel" aria-labelledby="opportunity-title">
                <div className="section-heading">
                  <div>
                    <p className="eyebrow">THE OPPORTUNITY</p>
                    <h2 id="opportunity-title">Role context</h2>
                  </div>
                </div>
                <dl className="fact-list workspace-facts">
                  <div>
                    <dt>Employer</dt>
                    <dd>{snapshot.job.company !== "" ? snapshot.job.company : "Not recorded"}</dd>
                  </div>
                  <div>
                    <dt>Compensation</dt>
                    <dd>
                      {snapshot.job.salaryText !== "" ? (
                        <>
                          {snapshot.job.salaryText}
                          <br />
                          <small>{snapshot.job.salaryOriginLabel}</small>
                        </>
                      ) : (
                        "Not available"
                      )}
                    </dd>
                  </div>
                  <div>
                    <dt>Posting</dt>
                    <dd className="workspace-url">{snapshot.job.url}</dd>
                  </div>
                  <div>
                    <dt>Source</dt>
                    <dd>
                      {snapshot.job.sourceStatusLabel}
                      {snapshot.job.sourceCheckedAt !== "" && (
                        <small> · Last checked {snapshot.job.sourceCheckedAt}</small>
                      )}
                      {snapshot.job.sourceNote !== "" && <small> · {snapshot.job.sourceNote}</small>}
                    </dd>
                  </div>
                </dl>
                <div className="workspace-source-actions">
                  <button
                    className="button button-secondary"
                    type="button"
                    disabled={refreshBusy}
                    onClick={() => void handleRefresh()}
                  >
                    {refreshBusy ? "Refreshing…" : "Refresh from posting ↻"}
                  </button>
                  {refreshError !== "" && (
                    <p className="job-form-error" role="alert">
                      {refreshError}
                    </p>
                  )}
                  <p className="small-muted">{snapshot.refreshDisclaimer}</p>
                </div>
                <div className="workspace-description" aria-label="Job description">
                  {snapshot.job.description !== "" ? (
                    <p>{snapshot.job.description}</p>
                  ) : (
                    <div className="empty-state">
                      <h3>No description saved yet</h3>
                      <p>Refresh from the posting or add your own notes in Edit details.</p>
                    </div>
                  )}
                </div>
              </section>

              <details className="panel detail-editor" open>
                <summary>
                  <span>
                    <strong>Edit job and tracking</strong>
                    <small>Title, company, salary, description, stage and priority. The posting URL never changes here.</small>
                  </span>
                  <span className="summary-chevron" aria-hidden="true">
                    ⌄
                  </span>
                </summary>
                <form className="job-edit-form editor-content" onSubmit={(event) => void handleSaveDetails(event)}>
                  <div className="field">
                    <label htmlFor="workspace-title">Job title</label>
                    <input
                      id="workspace-title"
                      value={title}
                      maxLength={200}
                      onChange={(event) => setTitle(event.target.value)}
                    />
                  </div>
                  <div className="field">
                    <label htmlFor="workspace-company">Company</label>
                    <input
                      id="workspace-company"
                      value={company}
                      maxLength={200}
                      onChange={(event) => setCompany(event.target.value)}
                    />
                  </div>
                  {applied ? (
                    <div className="field">
                      <label>Stage</label>
                      <input value="Applied (confirmed)" disabled />
                      <input type="hidden" defaultValue="applied" />
                    </div>
                  ) : (
                    <div className="field">
                      <label htmlFor="workspace-stage">Stage</label>
                      <select
                        id="workspace-stage"
                        value={stage}
                        onChange={(event) => setStage(event.target.value)}
                      >
                        <option value="review">Review</option>
                        <option value="ready">Marked ready</option>
                        <option value="closed">Closed</option>
                      </select>
                    </div>
                  )}
                  <div className="field">
                    <label htmlFor="workspace-priority">Priority</label>
                    <select
                      id="workspace-priority"
                      value={priority}
                      onChange={(event) => setPriority(event.target.value)}
                    >
                      <option value="normal">Normal</option>
                      <option value="high">High</option>
                      <option value="low">Low</option>
                    </select>
                  </div>
                  <div className="field field-full">
                    <label htmlFor="workspace-salary">Salary / compensation (optional)</label>
                    <input
                      id="workspace-salary"
                      value={salary}
                      maxLength={500}
                      onChange={(event) => setSalary(event.target.value)}
                    />
                    <small>Current source: {snapshot.job.salaryOriginLabel}</small>
                  </div>
                  <div className="field field-full">
                    <label htmlFor="workspace-description">Job description</label>
                    <textarea
                      id="workspace-description"
                      rows={8}
                      value={description}
                      onChange={(event) => setDescription(event.target.value)}
                    />
                  </div>
                  <p className="field-full form-note">
                    “Marked ready” is your own planning label; YourMove does not verify application
                    readiness. Applied status comes from confirmed submission history.
                  </p>
                  {detailsError !== "" && (
                    <p className="job-form-error field-full" role="alert">
                      {detailsError}
                    </p>
                  )}
                  {detailsNotice !== "" && (
                    <p className="form-notice field-full" role="status">
                      {detailsNotice}
                    </p>
                  )}
                  <div className="form-actions">
                    <button className="button button-primary" type="submit" disabled={detailsBusy}>
                      {detailsBusy ? "Saving…" : "Save changes"}
                    </button>
                  </div>
                </form>
              </details>

              <section className="panel" aria-labelledby="workspace-resume-title">
                <div className="section-heading">
                  <div>
                    <p className="eyebrow">RESUME</p>
                    <h2 id="workspace-resume-title">Selected document</h2>
                    <p className="section-subtitle">
                      Your choice is saved as soon as you select an option. Suggestions never replace it.
                    </p>
                  </div>
                  <a className="button button-secondary" href="/resumes">
                    Manage documents
                  </a>
                </div>
                {snapshot.suggestion.status === "matched" && (
                  <div className="match-suggestion workspace-suggestion">
                    <div>
                      <p className="eyebrow">SUGGESTION — NOT SAVED</p>
                      <h3>{snapshot.suggestion.resumeName}</h3>
                      <p className="small-muted">
                        Family: {snapshot.suggestion.familyName}
                        {snapshot.suggestion.titleEvidence !== "" &&
                          ` · Title phrase “${snapshot.suggestion.titleEvidence}”`}
                        {snapshot.suggestion.contextEvidence !== "" &&
                          ` · Description phrase “${snapshot.suggestion.contextEvidence}”`}
                      </p>
                      <p className="small-muted">{suggestionText(snapshot.suggestion.status)}</p>
                    </div>
                    <button
                      className="button button-primary"
                      type="button"
                      disabled={resumeBusy}
                      onClick={() => void handleAssign(snapshot.suggestion.resumeId)}
                    >
                      Use suggested resume
                    </button>
                  </div>
                )}
                {snapshot.suggestion.status !== "matched" && (
                  <p className="small-muted workspace-suggestion-note">{suggestionText(snapshot.suggestion.status)}</p>
                )}
                {resumeError !== "" && (
                  <p className="job-form-error" role="alert">
                    {resumeError}
                  </p>
                )}
                {resumeNotice !== "" && (
                  <p className="form-notice" role="status">
                    {resumeNotice}
                  </p>
                )}
                <ul className="resume-options workspace-options">
                  <li>
                    <label className="confirm-row">
                      <input
                        type="radio"
                        name="workspace-resume"
                        checked={snapshot.selectedResumeId === 0}
                        disabled={resumeBusy}
                        onChange={() => void handleAssign(0)}
                      />
                      <span>No resume selected</span>
                    </label>
                  </li>
                  {snapshot.resumeOptions.map((option) => (
                    <li key={option.id}>
                      <label className="confirm-row">
                        <input
                          type="radio"
                          name="workspace-resume"
                          checked={snapshot.selectedResumeId === option.id}
                          disabled={resumeBusy}
                          onChange={() => void handleAssign(option.id)}
                        />
                        <span>
                          {option.name} · {option.kind} · Version {option.version}
                          {option.roleFamilyName !== "" ? ` · ${option.roleFamilyName}` : ""}
                          {option.archived ? " · Archived" : ""}
                          {" · "}
                          {option.transferReady ? (
                            <strong className="transfer-ready">Transfer-ready</strong>
                          ) : (
                            <strong className="transfer-blocked">Not transfer-ready</strong>
                          )}
                          {option.transferNote !== "" && (
                            <small className="transfer-note"> — {option.transferNote}</small>
                          )}
                        </span>
                      </label>
                    </li>
                  ))}
                </ul>
                {snapshot.masterCount > 0 && (
                  <p className="small-muted">
                    A master resume is career evidence for cover letters, never an employer upload. It is
                    not offered above.
                  </p>
                )}
              </section>

              <section className="panel" aria-labelledby="workspace-prep-title">
                <div className="section-heading">
                  <div>
                    <p className="eyebrow">PREPARATION</p>
                    <h2 id="workspace-prep-title">What is ready, what needs attention</h2>
                  </div>
                </div>
                <dl className="readiness-grid">
                  <div>
                    <dt>Resume</dt>
                    <dd>
                      {snapshot.readiness.resumeReady
                        ? "Selected locally"
                        : "Needs attention · choose or repair a resume above"}
                    </dd>
                  </div>
                  <div>
                    <dt>Candidate profile</dt>
                    <dd>
                      {snapshot.readiness.profileApproved ? (
                        "Approved by you"
                      ) : (
                        <>
                          Not approved · contacts excluded · <a href="/candidate">Review candidate information</a>
                        </>
                      )}
                    </dd>
                  </div>
                  <div>
                    <dt>Application answers</dt>
                    <dd>
                      {snapshot.readiness.questionsAvailable
                        ? `${snapshot.readiness.needsAnswerCount} need your input · ${snapshot.readiness.preparedCount} prepared locally`
                        : "Questions not discovered yet"}
                    </dd>
                    {snapshot.readiness.questionsAvailable && (
                      <small>
                        {snapshot.readiness.questionSource}
                        {snapshot.readiness.questionPartial && " · Partial inspection"}
                      </small>
                    )}
                  </div>
                  <div>
                    <dt>Employer-side work</dt>
                    <dd>
                      {snapshot.readiness.questionsAvailable
                        ? snapshot.readiness.manualWorkCount > 0
                          ? `${snapshot.readiness.manualWorkCount} fields need employer-side completion or manual review`
                          : "None identified in this inspection"
                        : "Not assessed"}
                    </dd>
                  </div>
                  <div>
                    <dt>Cover letter</dt>
                    <dd>
                      {snapshot.readiness.coverLetterSaved
                        ? `Saved draft ready to review (${snapshot.letters.count} saved)`
                        : "Optional · no saved draft"}
                    </dd>
                  </div>
                  <div>
                    <dt>Browser session</dt>
                    <dd>{snapshot.readiness.browserStatus}</dd>
                  </div>
                </dl>
                <p className="small-muted">
                  {snapshot.submissions.count} recorded submission{snapshot.submissions.count === 1 ? "" : "s"} ·{" "}
                  {snapshot.preparation.recovery ? "recovery guidance applies" : "no recovery blocker"} ·{" "}
                  {snapshot.handoff.allowed ? snapshot.handoff.reason : snapshot.handoff.reason}
                </p>
              </section>

              <section className="panel" aria-labelledby="workspace-continue-title">
                <div className="section-heading">
                  <div>
                    <p className="eyebrow">CONTINUE IN THE LEGACY WORKSPACE</p>
                    <h2 id="workspace-continue-title">Answer writing, employer steps, and history</h2>
                    <p className="section-subtitle">
                      Writing interaction, browser orchestration, and submission recording stay legacy-owned
                      in this slice. These links return you to a functioning interface.
                    </p>
                  </div>
                </div>
                <ul className="workspace-legacy-links">
                  <li>
                    <a href={`${snapshot.legacyHref}#prepare`}>Legacy Prepare and transfer review →</a>
                  </li>
                  <li>
                    <a href={`${snapshot.legacyHref}#answers`}>Answer writing workspace →</a>
                  </li>
                  <li>
                    <a href={`${snapshot.legacyHref}#cover-letter`}>Cover-letter preparation →</a>
                  </li>
                  <li>
                    <a href={`/jobs/${snapshot.job.id}/inspection`}>Employer browser details and troubleshooting →</a>
                  </li>
                  <li>
                    <a href={`${snapshot.legacyHref}#history`}>
                      Submission history ({snapshot.submissions.count}) →
                    </a>
                  </li>
                </ul>
              </section>
            </>
          )}
        </main>
      </div>
    </div>
  );
}
