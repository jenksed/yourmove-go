import { AppNavigation, AppTopbar } from "../../shared/AppNavigation";
import { setUnsaved } from "../../shared/unsaved";
// YM-004A React Application Workspace: one coherent preparation activity.
// Reads and writes through Go-owned JSON contracts only. The source URL is
// displayed, never edited here; resume bytes never enter this client.

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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

function suggestionNote(status: string, familyName: string, familyCount: number): React.JSX.Element | null {
  switch (status) {
    case "no-resume":
      return (
        <p className="small-muted">
          The {familyName} family has no résumé yet. <a href="/resumes">Add one in Documents</a>.
        </p>
      );
    case "choose-primary":
      return (
        <p className="small-muted">
          The {familyName} family needs a primary résumé. <a href="/resumes">Set one in Documents</a>.
        </p>
      );
    case "ambiguous":
      return <p className="small-muted">Several families matched equally — select manually.</p>;
    case "no-title":
      return <p className="small-muted">Add a job title to enable suggestions.</p>;
    default:
      if (familyCount === 0) {
        return (
          <p className="small-muted">
            No role families yet. <a href="/families">Organize role families</a> to get suggestions.
          </p>
        );
      }
      return null;
  }
}

type Activity = "job" | "documents" | "questions" | "apply" | "applied";

interface JobBaseline {
  title: string;
  company: string;
  description: string;
  salary: string;
  stage: string;
  priority: string;
}

const emptyBaseline: JobBaseline = {
  title: "",
  company: "",
  description: "",
  salary: "",
  stage: "review",
  priority: "normal",
};

export function WorkspacePage(): React.JSX.Element {
  const jobId = workspaceIdFromPath(window.location.pathname);
  const [activity, setActivity] = useState<Activity>("job");
  const [snapshot, setSnapshot] = useState<WorkspaceSnapshot | null>(null);
  const [loadError, setLoadError] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);
  const [summaryError, setSummaryError] = useState<string>("");

  const [title, setTitle] = useState<string>("");
  const [company, setCompany] = useState<string>("");
  const [description, setDescription] = useState<string>("");
  const [salary, setSalary] = useState<string>("");
  const [stage, setStage] = useState<string>("review");
  const [priority, setPriority] = useState<string>("normal");
  const [baseline, setBaseline] = useState<JobBaseline>(emptyBaseline);
  const [detailsError, setDetailsError] = useState<string>("");
  const [detailsNotice, setDetailsNotice] = useState<string>("");
  const [detailsBusy, setDetailsBusy] = useState<boolean>(false);

  const [resumeBusy, setResumeBusy] = useState<boolean>(false);
  const [resumeError, setResumeError] = useState<string>("");
  const [chooserOpen, setChooserOpen] = useState<boolean>(false);
  const [chooserQuery, setChooserQuery] = useState<string>("");
  const [chooserFamily, setChooserFamily] = useState<string>("all");
  const chooserRef = useRef<HTMLDialogElement | null>(null);
  const chooserSearchRef = useRef<HTMLInputElement | null>(null);
  const chooserPrevSelected = useRef<number | null>(null);
  const chooserWasOpen = useRef<boolean>(false);
  const [refreshBusy, setRefreshBusy] = useState<boolean>(false);
  const [refreshError, setRefreshError] = useState<string>("");
  const initialNavDone = useRef(false);
  const [materialRevision, setMaterialRevision] = useState<number>(0);
  const [discoveryStatus, setDiscoveryStatus] = useState<string>("");
  const discoveryStarted = useRef(false);
  // One parent generation guards overlapping workspace GETs (material saves,
  // discovery, explicit reloads, restored-page refreshes). Obsolete
  // responses never touch snapshot, drafts, or baseline.
  const requestRef = useRef(0);
  const prevActivity = useRef<Activity>("job");

  const refreshSummary = useCallback(async () => {
    if (jobId === null) return;
    const request = ++requestRef.current;
    try {
      const next = await fetchWorkspace(jobId);
      if (request !== requestRef.current) return;
      // Background reads update the snapshot only: drafts and the saved
      // baseline stay intact, and child editors keep their state.
      setSnapshot(next);
      setSummaryError("");
    } catch {
      if (request !== requestRef.current) return;
      setSummaryError("Summary could not refresh; showing last loaded information.");
    }
  }, [jobId]);

  const materialSaved = useCallback(() => {
    setMaterialRevision((value) => value + 1);
    void refreshSummary();
  }, [refreshSummary]);

  const retryDiscovery = useCallback(() => {
    if (jobId === null) return;
    discoveryStarted.current = true;
    setDiscoveryStatus("checking");
    void discoverQuestions(jobId)
      .then((status) => {
        setDiscoveryStatus(status);
        // Refresh the parent summary without switching activity so newly
        // observed inventory appears beside the current editors.
        void refreshSummary();
      })
      .catch(() => setDiscoveryStatus("failed"));
  }, [jobId, refreshSummary]);

  const openChooser = useCallback(() => {
    setChooserQuery("");
    setChooserFamily("all");
    setChooserOpen(true);
  }, []);

  // The native dialog is imperative: opening focuses search, and a changed
  // saved selection closes it. Failures leave it open with the error shown.
  useEffect(() => {
    const dialog = chooserRef.current;
    if (!dialog) return;
    if (chooserOpen && !chooserWasOpen.current) {
      chooserPrevSelected.current = snapshot?.selectedResumeId ?? null;
      if (!dialog.open) dialog.showModal();
      chooserSearchRef.current?.focus();
    } else if (!chooserOpen && chooserWasOpen.current && dialog.open) {
      dialog.close();
    }
    chooserWasOpen.current = chooserOpen;
  }, [chooserOpen, snapshot]);
  useEffect(() => {
    if (
      chooserOpen &&
      snapshot &&
      chooserPrevSelected.current !== null &&
      snapshot.selectedResumeId !== chooserPrevSelected.current
    ) {
      setChooserOpen(false);
    }
  }, [chooserOpen, snapshot]);

  // First arrival at Questions — by explicit tab move or by initial landing
  // on a workspace that suggests it — starts the established discovery
  // operation once per page load. All other navigation stays local and never
  // triggers a remote question check.
  const handleActivitySelect = useCallback(
    (next: Activity) => {
      setActivity(next);
      if (next === "questions" && !discoveryStarted.current) retryDiscovery();
    },
    [retryDiscovery],
  );

  const detailsDirty =
    title !== baseline.title ||
    company !== baseline.company ||
    description !== baseline.description ||
    salary !== baseline.salary ||
    stage !== baseline.stage ||
    priority !== baseline.priority;

  useEffect(() => {
    // Report job-detail edits to the desktop close guard; saving stays
    // explicit through the details form.
    setUnsaved("details", detailsDirty);
    if (!detailsDirty) return;
    const guard = (event: BeforeUnloadEvent) => { event.preventDefault(); };
    window.addEventListener("beforeunload", guard);
    return () => window.removeEventListener("beforeunload", guard);
  }, [detailsDirty]);

  const initialLoad = useCallback(async () => {
    if (jobId === null) {
      setLoadError("Unknown opportunity. Return to Opportunities and open a saved job.");
      setLoading(false);
      return;
    }
    const request = ++requestRef.current;
    setLoading(true);
    setLoadError("");
    try {
      const next = await fetchWorkspace(jobId);
      if (request !== requestRef.current) return;
      setSnapshot(next);
      setTitle(next.job.title);
      setCompany(next.job.company);
      setDescription(next.job.description);
      setSalary(next.job.salaryText);
      setStage(next.job.stage);
      setPriority(next.job.priority);
      setBaseline({
        title: next.job.title,
        company: next.job.company,
        description: next.job.description,
        salary: next.job.salaryText,
        stage: next.job.stage,
        priority: next.job.priority,
      });
    } catch (error) {
      if (request !== requestRef.current) return;
      setLoadError(error instanceof Error ? error.message : "Unable to load workspace");
    } finally {
      if (request === requestRef.current) setLoading(false);
    }
  }, [jobId]);

  useEffect(() => {
    void initialLoad();
  }, [initialLoad]);

  // Initial navigation: explicit hashes win; otherwise follow the Go
  // navigation suggestion once. Never re-navigate on later updates.
  // Landing on Questions runs the same once-per-load question check as an
  // explicit tab move, so opening a workspace never needs a manual refresh
  // to discover its questions. Other landings stay local.
  useEffect(() => {
    if (!snapshot || initialNavDone.current) return;
    initialNavDone.current = true;
    const hash = window.location.hash.slice(1);
    let target: Activity = "job";
    if (["prepare", "workspace-resume-title", "cover-letters", "documents"].includes(hash)) {
      target = "documents";
    } else if (["writing", "questions"].includes(hash)) {
      target = "questions";
    } else if (["employer", "apply"].includes(hash)) target = "apply";
    else if (["submission", "history", "applied"].includes(hash)) target = "applied";
    else if (hash === "overview" || hash === "job") target = "job";
    else if (snapshot.nextAction) target = snapshot.nextAction.activity;
    else if (snapshot.job.submissionCount > 0 || snapshot.job.stage === "applied") target = "applied";
    else if (snapshot.selectedResumeId > 0) target = "questions";
    handleActivitySelect(target);
  }, [snapshot, handleActivitySelect]);

  // Restored pages (back/forward after a Documents assignment) refresh the
  // summary without resetting drafts, baseline, or activity.
  useEffect(() => {
    const onPageShow = (event: PageTransitionEvent) => {
      if (event.persisted) void refreshSummary();
    };
    window.addEventListener("pageshow", onPageShow);
    return () => window.removeEventListener("pageshow", onPageShow);
  }, [refreshSummary]);

  // Leaving Apply surfaces observed browser/question changes in preparation.
  // Live browser polling stays inside ContinuationSection.
  useEffect(() => {
    const previous = prevActivity.current;
    prevActivity.current = activity;
    if (previous === "apply" && activity !== "apply") void refreshSummary();
  }, [activity, refreshSummary]);

  const handleSaveDetails = useCallback(
    async (event: React.FormEvent) => {
      event.preventDefault();
      if (jobId === null) return;
      requestRef.current++;
      setDetailsError("");
      setDetailsNotice("");
      setDetailsBusy(true);
      try {
        const result = await saveWorkspaceDetails(jobId, {
          title,
          company,
          description,
          salary_text: salary,
          stage,
          priority,
        });
        // The canonical write response updates both draft and baseline;
        // the derived summary follows without resetting other editors.
        setTitle(result.job.title);
        setCompany(result.job.company);
        setDescription(result.job.description);
        setSalary(result.job.salaryText);
        setStage(result.job.stage);
        setPriority(result.job.priority);
        setBaseline({
          title: result.job.title,
          company: result.job.company,
          description: result.job.description,
          salary: result.job.salaryText,
          stage: result.job.stage,
          priority: result.job.priority,
        });
        await refreshSummary();
        setDetailsNotice("Saved. Your edits are kept across reloads and restarts.");
      } catch (error) {
        setDetailsError(error instanceof Error ? error.message : "Unable to save changes");
      } finally {
        setDetailsBusy(false);
      }
    },
    [jobId, title, company, description, salary, stage, priority, refreshSummary],
  );

  const handleAssign = useCallback(
    async (resumeId: number) => {
      if (jobId === null) return;
      if (detailsDirty) {
        setResumeError("Save your job edits before changing the selected document.");
        return;
      }
      requestRef.current++;
      setResumeError("");
      setResumeBusy(true);
      try {
        await assignWorkspaceResume(jobId, resumeId);
        // Assignment needs only a summary refresh; editor drafts stay intact.
        // A changed selection closes the chooser via the snapshot effect below.
        await refreshSummary();
        materialSaved();
      } catch (error) {
        setResumeError(
          error instanceof Error
            ? `${error.message} Could not confirm the change. Reload the workspace to check the saved selection.`
            : "Unable to save selection. Could not confirm the change. Reload the workspace to check the saved selection.",
        );
      } finally {
        setResumeBusy(false);
      }
    },
    [jobId, refreshSummary, detailsDirty, materialSaved],
  );

  const handleRefresh = useCallback(async () => {
    if (jobId === null) return;
    if (detailsDirty) {
      setRefreshError("Save your job edits before refreshing the posting.");
      return;
    }
    requestRef.current++;
    setRefreshError("");
    setRefreshBusy(true);
    try {
      const result = await refreshWorkspacePosting(jobId);
      setTitle(result.job.title);
      setCompany(result.job.company);
      setDescription(result.job.description);
      setSalary(result.job.salaryText);
      setStage(result.job.stage);
      setPriority(result.job.priority);
      setBaseline({
        title: result.job.title,
        company: result.job.company,
        description: result.job.description,
        salary: result.job.salaryText,
        stage: result.job.stage,
        priority: result.job.priority,
      });
      await refreshSummary();
    } catch (error) {
      setRefreshError(error instanceof Error ? error.message : "Unable to refresh posting");
    } finally {
      setRefreshBusy(false);
    }
  }, [jobId, refreshSummary, detailsDirty]);

  const applied = snapshot !== null && (snapshot.job.submissionCount > 0 || snapshot.job.stage === "applied");
  const completionRecorded = useCallback(() => {
    if (jobId === null) return;
    requestRef.current++;
    // Confirmed history updates only the stage draft/baseline; all other
    // unsaved job edits and child drafts are preserved.
    setStage("applied");
    setBaseline((previous) => ({ ...previous, stage: "applied" }));
    void refreshSummary();
    materialSaved();
  }, [jobId, refreshSummary, materialSaved]);

  const materials = useMemo(() => {
    if (!snapshot) return null;
    const { job, readiness } = snapshot;
    const option = snapshot.resumeOptions.find((row) => row.id === snapshot.selectedResumeId);
    let resumeText = "No résumé selected";
    if (snapshot.selectedResumeId !== 0) {
      if (option) {
        resumeText = `${option.name} · Version ${option.version}`;
        if (!option.transferReady && option.transferNote !== "") resumeText += ` — ${option.transferNote}`;
      } else {
        resumeText = "Selected résumé unavailable";
      }
    }
    let answersText: string;
    let answersQualifier = "";
    if (!readiness.questionsAvailable) {
      answersText = discoveryStatus === "checking" ? "Checking available questions…" : "Questions not yet established";
    } else {
      if (readiness.questionPartial) {
        answersQualifier = "Partial inspection — later questions may be missing.";
      }
      if (!readiness.questionPartial && readiness.needsAnswerCount === 0 && readiness.preparedCount === 0) {
        answersText = "No questions needing your input were found in this inspection.";
      } else if (readiness.needsAnswerCount === 0 && readiness.preparedCount > 0) {
        answersText = `${readiness.preparedCount} of ${readiness.preparedCount} answers saved. Nothing left unanswered.`;
      } else if (readiness.needsAnswerCount > 0) {
        const total = readiness.needsAnswerCount + readiness.preparedCount;
        answersText =
          readiness.preparedCount > 0
            ? `${readiness.needsAnswerCount} of ${total} questions still need answers (${readiness.preparedCount} saved).`
            : `${readiness.needsAnswerCount} questions need answers.`;
      } else {
        answersText = "Review available questions.";
      }
      if (readiness.manualWorkCount > 0) {
        answersQualifier += (answersQualifier !== "" ? " " : "") +
          `Plus ${readiness.manualWorkCount} employer-side items to complete on their site.`;
      }
    }
    const letterText = readiness.coverLetterSaved ? "Saved letter available" : "No saved letter";
    let attention = "";
    if (job.submissionCount > 0) attention = "Submission recorded locally.";
    else if (job.stage === "applied") attention = "Applied — no local submission record.";
    else if (job.stage === "closed") attention = "This opportunity is closed.";
    else if (snapshot.selectedResumeId === 0) attention = "No résumé selected. Choose one to continue.";
    else if (!snapshot.resumeValid) attention = "Selected résumé unavailable. Review the selected document.";
    else if (!readiness.questionsAvailable) attention = "";
    else if (readiness.needsAnswerCount > 0) attention = `${readiness.needsAnswerCount} available questions without saved answers.`;
    return { resumeText, answersText, answersQualifier, letterText, attention };
  }, [snapshot, discoveryStatus]);

  const selectedOption = snapshot?.resumeOptions.find((row) => row.id === snapshot.selectedResumeId) ?? null;

  const chooserFamilies = useMemo(() => {
    if (!snapshot) return [];
    const seen = new Map<number, string>();
    for (const option of snapshot.resumeOptions) {
      if (option.roleFamilyId > 0 && !seen.has(option.roleFamilyId)) {
        seen.set(option.roleFamilyId, option.roleFamilyName);
      }
    }
    return [...seen.entries()]
      .map(([id, name]) => ({ id, name }))
      .sort((a, b) => a.name.localeCompare(b.name));
  }, [snapshot]);

  const chooserGroups = useMemo(() => {
    if (!snapshot) return [];
    const needle = chooserQuery.trim().toLocaleLowerCase();
    const matches = (name: string, family: string) =>
      needle === "" || `${name} ${family}`.toLocaleLowerCase().includes(needle);
    const inFilter = (option: { roleFamilyId: number }) =>
      chooserFamily === "all" ||
      (chooserFamily === "none" ? option.roleFamilyId === 0 : option.roleFamilyId === Number(chooserFamily));
    const rows = snapshot.resumeOptions.filter(
      (option) => inFilter(option) && matches(option.name, option.roleFamilyName),
    );
    if (chooserFamily !== "all") return [{ key: chooserFamily, label: null as string | null, rows }];
    const groups: { key: string; label: string | null; rows: typeof rows }[] = [];
    for (const family of chooserFamilies) {
      const familyRows = rows.filter((option) => option.roleFamilyId === family.id);
      if (familyRows.length > 0) groups.push({ key: `family-${family.id}`, label: family.name, rows: familyRows });
    }
    const ungrouped = rows.filter((option) => option.roleFamilyId === 0);
    if (ungrouped.length > 0) groups.push({ key: "family-none", label: "No family", rows: ungrouped });
    return groups;
  }, [snapshot, chooserQuery, chooserFamily, chooserFamilies]);

  const showSuggested =
    snapshot !== null &&
    snapshot.nextAction !== undefined &&
    snapshot.nextAction.activity !== activity &&
    activity !== "apply";

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      <AppNavigation active="opportunities" />
      <div className="app-main">
        <AppTopbar />
        <main id="main-content" className={`page-container workspace-page workspace-activity-${activity}`}>
          <nav className="breadcrumbs" aria-label="Breadcrumb">
            <a href="/jobs">Opportunities</a>
            <span aria-hidden="true">/</span>
            <span>Workspace</span>
          </nav>

          {loading && <p role="status">Loading application workspace…</p>}
          {loadError !== "" && (
            <section className="panel" aria-label="Loading error">
              <p>{loadError}</p>
              <button className="button button-secondary" type="button" onClick={() => void initialLoad()}>
                Retry
              </button>
            </section>
          )}

          {snapshot && materials && (
            <>
              <div className="page-heading" id="overview">
                <div>
                  <h1>{snapshot.job.displayTitle}</h1>
                  <p className="page-description">
                    {snapshot.job.companyOrHost} <span className="dot-sep" aria-hidden="true">·</span>{" "}
                    {snapshot.job.stageLabel}
                  </p>
                </div>
                <div className="heading-actions">
                  <a className="text-link" href="/resumes">
                    Document library
                  </a>
                </div>
              </div>

              <section className="panel workspace-materials" aria-label="Current materials">
                <dl className="workspace-materials-grid">
                  <div>
                    <dt>Résumé</dt>
                    <dd>{materials.resumeText}</dd>
                  </div>
                  <div>
                    <dt>Answers</dt>
                      <dd>
                        {materials.answersText}
                        {!snapshot.readiness.questionsAvailable && activity !== "questions" && (
                        <>
                          {" "}
                          <button
                            className="text-link workspace-inline-action"
                            type="button"
                            onClick={() => handleActivitySelect("questions")}
                          >
                            Check available questions
                          </button>
                        </>
                      )}
                      {materials.answersQualifier !== "" && (
                        <small className="workspace-materials-qualifier"> {materials.answersQualifier}</small>
                      )}
                    </dd>
                  </div>
                  <div>
                    <dt>Letter</dt>
                    <dd>{materials.letterText}</dd>
                  </div>
                </dl>
                {(materials.attention !== "" || showSuggested) && (
                  <div className="workspace-attention-row">
                    {materials.attention !== "" && (
                      <p className="workspace-attention" role="status">
                        {materials.attention}
                      </p>
                    )}
                    {showSuggested && (
                      <button
                        className="button button-primary"
                        type="button"
                        onClick={() => handleActivitySelect(snapshot.nextAction.activity)}
                      >
                        {snapshot.nextAction.label} <span aria-hidden="true">→</span>
                      </button>
                    )}
                  </div>
                )}
                {summaryError !== "" && (
                  <div className="workspace-summary-error">
                    <p role="alert">{summaryError}</p>
                    <button className="button button-secondary" type="button" onClick={() => void refreshSummary()}>
                      Retry
                    </button>
                  </div>
                )}
              </section>
              {detailsDirty && activity !== "job" && <p className="workspace-unsaved" role="status">Job edits are not saved. <button type="button" onClick={() => handleActivitySelect("job")}>Return to job details</button> to save them.</p>}
              <nav className="workspace-tabs" aria-label="Application activities">
                {(["job", "documents", "questions", "apply", "applied"] as const).map((tab) => (
                  <button
                    key={tab}
                    type="button"
                    aria-current={activity === tab ? "page" : undefined}
                    onClick={() => handleActivitySelect(tab)}
                  >
                    {tab === "job" ? "Review job" : tab === "documents" ? "Documents" : tab === "questions" ? "Questions" : tab === "apply" ? "Apply" : "Submission history"}
                  </button>
                ))}
              </nav>

              <div hidden={activity !== "job"}>
              <section className="panel" aria-labelledby="opportunity-title">
                <div className="section-heading">
                  <div>
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
                    <dd>
                      <span className="workspace-source-status">{snapshot.job.host !== "" ? snapshot.job.host : snapshot.job.url}</span>
                      <span className="workspace-url">{snapshot.job.url}</span>
                    </dd>
                  </div>
                  <div>
                    <dt>Source</dt>
                    <dd>
                      <span className="workspace-source-status">{snapshot.job.sourceStatusLabel}</span>
                      {(snapshot.job.sourceCheckedAt !== "" || snapshot.job.sourceNote !== "") && (
                        <small className="workspace-source-meta">
                          {snapshot.job.sourceCheckedAt !== "" ? `Last checked ${snapshot.job.sourceCheckedAt}` : ""}
                          {snapshot.job.sourceCheckedAt !== "" && snapshot.job.sourceNote !== "" ? " · " : ""}
                          {snapshot.job.sourceNote}
                        </small>
                      )}
                    </dd>
                  </div>
                </dl>
                <div className="workspace-source-actions">
                  <button
                    className="button button-secondary"
                    type="button"
                    disabled={refreshBusy || detailsBusy}
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
                      disabled={detailsBusy}
                      onChange={(event) => setTitle(event.target.value)}
                    />
                  </div>
                  <div className="field">
                    <label htmlFor="workspace-company">Company</label>
                    <input
                      id="workspace-company"
                      value={company}
                      maxLength={200}
                      disabled={detailsBusy}
                      onChange={(event) => setCompany(event.target.value)}
                    />
                  </div>
                  {applied ? (
                    <div className="field">
                      <label>Stage</label>
                      <input value={snapshot.job.submissionCount > 0 ? "Applied (confirmed)" : "Applied"} disabled />
                      <input type="hidden" defaultValue="applied" />
                    </div>
                  ) : (
                    <div className="field">
                      <label htmlFor="workspace-stage">Stage</label>
                      <select
                        id="workspace-stage"
                        value={stage}
                        disabled={detailsBusy}
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
                      disabled={detailsBusy}
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
                      disabled={detailsBusy}
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
                      disabled={detailsBusy}
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
              </div>

              <div hidden={activity !== "documents"}>
              <section className="panel" id="prepare" aria-labelledby="workspace-resume-title">
                <div className="section-heading">
                  <div>
                    <h2 id="workspace-resume-title">Résumé</h2>
                  </div>
                  <div className="heading-actions">
                    <a className="text-link" href="/families">
                      Role families
                    </a>
                    <a className="button button-secondary" href="/resumes">
                      Manage documents
                    </a>
                  </div>
                </div>
                <div className="resume-current">
                  <div className="resume-current-text">
                    {selectedOption ? (
                      <>
                        <p>
                          <strong>{selectedOption.name}</strong> · Version {selectedOption.version}
                          {selectedOption.roleFamilyName !== "" && (
                            <span className="resume-family">
                              {" "}· {selectedOption.roleFamilyName}
                              {selectedOption.isPrimary ? " · Primary" : ""}
                            </span>
                          )}
                        </p>
                        {!selectedOption.transferReady && (
                          <p className="small-muted">Unavailable for transfer. Choose an available document.</p>
                        )}
                      </>
                    ) : snapshot.selectedResumeId !== 0 ? (
                      <p>Selected résumé unavailable. Review the selected document.</p>
                    ) : (
                      <p className="small-muted">No résumé selected.</p>
                    )}
                  </div>
                  <button
                    className={selectedOption ? "button button-secondary" : "button button-primary"}
                    type="button"
                    disabled={resumeBusy}
                    onClick={openChooser}
                  >
                    {selectedOption ? "Change résumé" : "Choose résumé"}
                  </button>
                </div>
                {snapshot.suggestion.status === "matched" &&
                  snapshot.suggestion.resumeId !== snapshot.selectedResumeId && (
                    <p className="resume-suggestion">
                      Suggested for {snapshot.suggestion.familyName}:{" "}
                      <strong>{snapshot.suggestion.resumeName}</strong>{" "}
                      <button
                        className="button button-primary"
                        type="button"
                        disabled={resumeBusy}
                        onClick={() => void handleAssign(snapshot.suggestion.resumeId)}
                      >
                        Use suggested resume
                      </button>
                    </p>
                  )}
                {snapshot.suggestion.status === "matched" &&
                  snapshot.suggestion.resumeId === snapshot.selectedResumeId && (
                    <p className="small-muted">
                      Using the suggested résumé for {snapshot.suggestion.familyName}.
                    </p>
                  )}
                {snapshot.suggestion.status !== "matched" &&
                  suggestionNote(
                    snapshot.suggestion.status,
                    snapshot.suggestion.familyName,
                    snapshot.familyCount,
                  )}
                {resumeError !== "" && (
                  <p className="job-form-error" role="alert">
                    {resumeError}
                  </p>
                )}
                {snapshot.resumeOptions.length === 0 && <p className="workspace-empty-documents">No selectable résumé yet. Add one in Documents, then return here to select it for this opportunity.</p>}
                <dialog
                  ref={chooserRef}
                  className="resume-chooser"
                  aria-labelledby="resume-chooser-title"
                  onClose={() => setChooserOpen(false)}
                >
                  <div className="resume-chooser-head">
                    <h3 id="resume-chooser-title">Choose a résumé</h3>
                    <button
                      className="button button-secondary"
                      type="button"
                      onClick={() => setChooserOpen(false)}
                    >
                      Close
                    </button>
                  </div>
                  <div className="resume-chooser-tools">
                    <div className="field">
                      <label htmlFor="resume-chooser-search">Search</label>
                      <input
                        ref={chooserSearchRef}
                        id="resume-chooser-search"
                        type="search"
                        placeholder="Name or family"
                        autoComplete="off"
                        value={chooserQuery}
                        onChange={(event) => setChooserQuery(event.target.value)}
                      />
                    </div>
                    {chooserFamilies.length > 0 && (
                      <div className="field">
                        <label htmlFor="resume-chooser-family">Family</label>
                        <select
                          id="resume-chooser-family"
                          value={chooserFamily}
                          onChange={(event) => setChooserFamily(event.target.value)}
                        >
                          <option value="all">All families</option>
                          {chooserFamilies.map((family) => (
                            <option key={family.id} value={String(family.id)}>
                              {family.name}
                            </option>
                          ))}
                          <option value="none">No family</option>
                        </select>
                      </div>
                    )}
                  </div>
                  {resumeError !== "" && (
                    <p className="job-form-error" role="alert">
                      {resumeError}
                    </p>
                  )}
                  <div className="resume-chooser-list">
                    <label className="confirm-row chooser-row">
                      <input
                        type="radio"
                        name="chooser-resume"
                        checked={snapshot.selectedResumeId === 0}
                        disabled={resumeBusy}
                        onChange={() => void handleAssign(0)}
                      />
                      <span>No résumé — clear selection</span>
                    </label>
                    {chooserGroups.map((group) => (
                      <fieldset key={group.key} className="chooser-group">
                        {group.label !== null && <legend>{group.label}</legend>}
                        {group.rows.map((option) => (
                          <label key={option.id} className="confirm-row chooser-row">
                            <input
                              type="radio"
                              name="chooser-resume"
                              checked={snapshot.selectedResumeId === option.id}
                              disabled={resumeBusy}
                              onChange={() => void handleAssign(option.id)}
                            />
                            <span className="chooser-row-text">
                              {option.name} · Version {option.version}
                              {option.isPrimary ? " · Primary" : ""}
                              {option.archived ? " · Archived" : ""}
                              {!option.transferReady ? " · Unavailable" : ""}
                              {snapshot.selectedResumeId === option.id ? " · Current" : ""}
                            </span>
                          </label>
                        ))}
                      </fieldset>
                    ))}
                    {chooserGroups.length === 0 && (
                      <p className="small-muted" role="status">
                        No matching résumés.
                      </p>
                    )}
                  </div>
                  {resumeBusy && <p role="status">Saving…</p>}
                </dialog>
                {snapshot.masterCount > 0 && (
                  <p className="small-muted">
                    Master resumes are career evidence for cover letters, never employer uploads.
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
              </div>

              <div hidden={activity !== "questions"}>
                {snapshot.job.id > 0 && <WritingSection jobId={snapshot.job.id} destination={snapshot.job.url} onMaterialSaved={materialSaved} readOnly={applied} active={activity === "questions"} discoveryStatus={discoveryStatus} onRetryDiscovery={retryDiscovery} />}
                {!showSuggested && (
                  <div className="workspace-secondary-action">
                    <button className="button button-secondary" type="button" onClick={() => handleActivitySelect("apply")}>
                      Review application options →
                    </button>
                  </div>
                )}
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
