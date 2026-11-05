import { AppNavigation } from "../../shared/AppNavigation";
// YM-004A React Application Workspace: one coherent preparation activity.
// Reads and writes through Go-owned JSON contracts only. The source URL is
// displayed, never edited here; resume bytes never enter this client.

import { useCallback, useEffect, useRef, useState } from "react";
import {
  assignWorkspaceResume,
  fetchWorkspace,
  refreshWorkspacePosting,
  saveWorkspaceDetails,
  workspaceIdFromPath,
  type WorkspaceSnapshot,
} from "./api";
import { CoverLetterSection } from "./CoverLetterSection";
import { ContinuationSection } from "./ContinuationSection";
import { SubmissionSection } from "./SubmissionSection";
import { WritingSection } from "./WritingSection";
import { discoverQuestions } from "./writingApi";

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
  const [activity, setActivity] = useState<"job" | "documents" | "questions" | "apply" | "applied">("job");
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
  const initialHashHandled = useRef(false);
  const [materialRevision, setMaterialRevision] = useState<number>(0);
  const [discoveryStatus, setDiscoveryStatus] = useState<string>("");
  const discoveryStarted = useRef(false);
  const retryDiscovery = useCallback(() => {
    if (jobId === null) return;
    discoveryStarted.current = true;
    setDiscoveryStatus("checking");
    void discoverQuestions(jobId).then(setDiscoveryStatus).catch(() => setDiscoveryStatus("failed"));
  }, [jobId]);
  const materialSaved = useCallback(() => setMaterialRevision((value) => value + 1), []);

  useEffect(() => {
    if (jobId === null || !snapshot || discoveryStarted.current || (activity !== "documents" && activity !== "questions")) return;
    retryDiscovery();
  }, [activity, jobId, snapshot, retryDiscovery]);

  const detailsDirty = snapshot !== null && (
    title !== snapshot.job.title || company !== snapshot.job.company ||
    description !== snapshot.job.description || salary !== snapshot.job.salaryText ||
    stage !== snapshot.job.stage || priority !== snapshot.job.priority
  );

  useEffect(() => {
    if (!detailsDirty) return;
    const guard = (event: BeforeUnloadEvent) => { event.preventDefault(); };
    window.addEventListener("beforeunload", guard);
    return () => window.removeEventListener("beforeunload", guard);
  }, [detailsDirty]);

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

  useEffect(() => {
    if (!snapshot || initialHashHandled.current) return;
    initialHashHandled.current = true;
    const hash = window.location.hash.slice(1);
    if (["prepare", "workspace-resume-title", "cover-letters"].includes(hash)) setActivity("documents");
    else if (["writing", "questions"].includes(hash)) setActivity("questions");
    else if (["employer", "apply"].includes(hash)) setActivity("apply");
    else if (["submission", "history", "applied"].includes(hash)) setActivity("applied");
    else if (snapshot.job.submissionCount > 0 || snapshot.job.stage === "applied") setActivity("applied");
    else if (snapshot.selectedResumeId > 0) setActivity("questions");
  }, [snapshot]);

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
      if (detailsDirty) {
        setResumeError("Save your job edits before changing the selected document.");
        return;
      }
      setResumeError("");
      setResumeNotice("");
      setResumeBusy(true);
      try {
        await assignWorkspaceResume(jobId, resumeId);
        await reload();
        materialSaved();
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
    [jobId, reload, detailsDirty, materialSaved],
  );

  const handleRefresh = useCallback(async () => {
    if (jobId === null) return;
    if (detailsDirty) {
      setRefreshError("Save your job edits before refreshing the posting.");
      return;
    }
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
  }, [jobId, reload, detailsDirty]);

  const applied = snapshot !== null && (snapshot.job.submissionCount > 0 || snapshot.job.stage === "applied");
  const completionRecorded = useCallback(() => {
    if (jobId !== null) {
      void fetchWorkspace(jobId).then((next) => {
        setSnapshot(next);
        setStage(next.job.stage);
      }).catch(() => setDetailsError("Completion was recorded, but the opportunity summary could not be refreshed. Reopen the workspace to see its current state."));
    }
    materialSaved();
  }, [jobId, materialSaved]);

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      <AppNavigation active="opportunities" />
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
              <div className="page-heading" id="overview">
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
                    ← All opportunities
                  </a>
                  <a className="button button-secondary" href="/resumes">
                    Documents
                  </a>
                </div>
              </div>

              <div className="workspace-orientation">
                <div><strong>Your application, at your pace</strong><p>Review the role, prepare your materials, then continue on the employer site. You submit the application yourself.</p></div>
                <span>{applied ? "Submission recorded" : snapshot.job.stageLabel}</span>
              </div>
              {detailsDirty && activity !== "job" && <p className="workspace-unsaved" role="status">Job edits are not saved. <button type="button" onClick={() => setActivity("job")}>Return to job details</button> to save them.</p>}
              <nav className="workspace-tabs" aria-label="Application activities">
                {(["job", "documents", "questions", "apply", "applied"] as const).map((tab) => (
                  <button key={tab} type="button" aria-current={activity === tab ? "page" : undefined}
                    onClick={() => setActivity(tab)}>{tab === "job" ? "1. Review job" : tab === "documents" ? "2. Documents" : tab === "questions" ? "3. Questions" : tab === "apply" ? "4. Apply" : "Submission history"}</button>
                ))}
              </nav>

              <div hidden={activity !== "job"}>
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

              <details className="panel detail-editor">
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
                    {detailsDirty && <span className="small-muted">Unsaved job edits</span>}
                    <button className="button button-primary" type="submit" disabled={detailsBusy}>
                      {detailsBusy ? "Saving…" : "Save changes"}
                    </button>
                  </div>
                </form>
              </details>

              <div className="workspace-step-end"><div><strong>Next: prepare your documents</strong><p>Choose a résumé and work on a cover letter before continuing.</p></div><button className="button button-primary" type="button" onClick={() => setActivity("documents")}>Continue to documents →</button></div>
              </div>

              <div hidden={activity !== "documents"}>
              <section className="panel" id="prepare" aria-labelledby="workspace-resume-title">
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
                {snapshot.resumeOptions.length === 0 && <p className="workspace-empty-documents">No selectable résumé yet. Add one in Documents, then return here to select it for this opportunity.</p>}
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

              {snapshot.job.id > 0 && <CoverLetterSection
                jobId={snapshot.job.id}
                selectedResumeId={snapshot.selectedResumeId}
                onMaterialSaved={materialSaved}
                readOnly={applied}
                resumeOptions={snapshot.resumeOptions.map((option) => ({
                  id: option.id, name: option.name, kind: option.kind,
                  version: option.version, archived: option.archived,
                }))}
              />}
              <div className="workspace-step-end"><div><strong>Next: application questions</strong><p>Prepare answers now, or complete them on the employer site.</p></div><button className="button button-primary" type="button" onClick={() => setActivity("questions")}>Continue to questions →</button></div>
              </div>

              <div hidden={activity !== "questions"}>
                <p>Answer any available questions you choose; the rest can be completed on the employer site.</p>
                {snapshot.job.id > 0 && <WritingSection jobId={snapshot.job.id} destination={snapshot.job.url} onMaterialSaved={materialSaved} readOnly={applied} active={activity === "questions"} discoveryStatus={discoveryStatus} onRetryDiscovery={retryDiscovery} />}
                <div className="workspace-step-end"><div><strong>Next: continue with the employer</strong><p>Review what to share before opening or filling the application.</p></div><button className="button button-primary" type="button" onClick={() => setActivity("apply")}>Continue to application →</button></div>
              </div>

              <div hidden={activity !== "apply"}>
                {snapshot.job.id > 0 && <ContinuationSection jobId={snapshot.job.id} materialRevision={materialRevision} active={activity === "apply"} />}
              </div>

              <div hidden={activity !== "apply" && activity !== "applied"} id="history">
                {snapshot.job.id > 0 && <SubmissionSection jobId={snapshot.job.id} onRecorded={completionRecorded} />}
                <a className="button button-secondary" href="/jobs">Back to Opportunities</a>
              </div>


            </>
          )}
        </main>
      </div>
    </div>
  );
}
