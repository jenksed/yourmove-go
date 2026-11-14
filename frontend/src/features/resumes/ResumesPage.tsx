import { AppNavigation, AppTopbar } from "../../shared/AppNavigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  assignJobResume,
  deleteResume,
  fetchResumeOptions,
  fetchResumes,
  importResume,
  importResumeNative,
  reviseResume,
  reviseResumeNative,
  setResumeArchived,
  setResumeFamily,
  setResumeMaster,
  type ResumeItem,
  type ResumeOption,
  type ResumesResponse,
} from "./api";
import {
  fetchOpportunities,
  type OpportunityJob,
} from "../opportunities/api";

function statusLabel(resume: ResumeItem): string {
  if (resume.archived) return `Archived · Version ${resume.version}`;
  if (resume.isMaster) return `Master resume · Version ${resume.version}`;
  if (resume.roleFamilyName !== "") {
    return resume.isPrimary
      ? `${resume.roleFamilyName} · Primary resume`
      : resume.roleFamilyName;
  }
  return "No role family · available for manual selection";
}

const legacyNotices: Record<string, string> = {
  "in-use": "This resume is referenced by a job, submission, or saved cover letter. Archive it to remove it from active choices without losing application evidence.",
  master: "Remove the master designation or upload a new master revision before deleting this document.",
  archived: "Resume archived. Existing job assignments and submission history are unchanged.",
  restored: "Resume restored. Assign it to a role family or designate it as master if needed.",
  revised: "New revision saved. Existing job assignments still point to the previous document until you update them.",
  duplicate: "That file is already in your document library. A revision must contain different document bytes.",
  failed: "YourMove could not complete that document change. The current library remains available.",
};

export function ResumesPage(): React.JSX.Element {
  const [data, setData] = useState<ResumesResponse | null>(null);
  const [loadError, setLoadError] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);
  const [notice, setNotice] = useState<string>("");
  const [actionError, setActionError] = useState<string>("");
  const [busy, setBusy] = useState<boolean>(false);

  const [query, setQuery] = useState<string>("");
  const [showArchived, setShowArchived] = useState<boolean>(true);

  // Searchable association state. Destination selection never writes;
  // only an explicit resume radio POSTs an assignment.
  const [assocOpen, setAssocOpen] = useState<boolean>(false);
  const [oppSearch, setOppSearch] = useState<string>("");
  const [oppList, setOppList] = useState<OpportunityJob[] | null>(null);
  const [oppLoading, setOppLoading] = useState<boolean>(false);
  const [oppError, setOppError] = useState<string>("");
  const [selectedJobId, setSelectedJobId] = useState<number | null>(null);
  const [jobOptions, setJobOptions] = useState<ResumeOption[]>([]);
  const [jobTitle, setJobTitle] = useState<string>("");
  const [jobCompany, setJobCompany] = useState<string>("");
  const [jobSelected, setJobSelected] = useState<number>(0);
  const [jobError, setJobError] = useState<string>("");
  const [jobLoading, setJobLoading] = useState<boolean>(false);
  const [assignBusy, setAssignBusy] = useState<boolean>(false);
  const [assignNotice, setAssignNotice] = useState<string>("");
  const [assignError, setAssignError] = useState<string>("");

  const [deleteTarget, setDeleteTarget] = useState<number | null>(null);
  const [deletePhrase, setDeletePhrase] = useState<string>("");
  const [deleteConfirm, setDeleteConfirm] = useState<boolean>(false);

  const reload = useCallback(async () => {
    setLoading(true);
    setLoadError("");
    try {
      setData(await fetchResumes());
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : "Unable to load resumes");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  useEffect(() => {
    const code = new URLSearchParams(window.location.search).get("notice");
    if (code && legacyNotices[code]) setNotice(legacyNotices[code]);
  }, []);

  const visible = useMemo(() => {
    if (!data) return [];
    const needle = query.trim().toLocaleLowerCase();
    return data.resumes.filter((resume) => {
      if (!showArchived && resume.archived) return false;
      if (needle === "") return true;
      return `${resume.name} ${resume.kind} ${resume.roleFamilyName}`.toLocaleLowerCase().includes(needle);
    });
  }, [data, query, showArchived]);

  const runMutation = useCallback(
    async (label: string, work: () => Promise<unknown>, done?: string | ((result: unknown) => string)) => {
      setActionError("");
      setNotice("");
      setBusy(true);
      try {
        const result = await work();
        if (done) setNotice(typeof done === "string" ? done : done(result));
        await reload();
      } catch (error) {
        setActionError(error instanceof Error ? `${label}: ${error.message}` : `${label} failed`);
      } finally {
        setBusy(false);
      }
    },
    [reload],
  );

  const handleImportFile = useCallback(
    (file: File | undefined) => {
      if (!file) return;
      void runMutation("Import", () => importResume(file), (result) => {
        const imported = result as { duplicate: boolean };
        return imported.duplicate
          ? "That file is already in your document library. Existing bytes and document identity were kept."
          : "Resume imported. Stored locally with exact bytes preserved.";
      });
    },
    [runMutation],
  );

  const handleReviseFile = useCallback(
    (id: number, file: File | undefined) => {
      if (!file) return;
      void runMutation(
        "Revision",
        () => reviseResume(id, file),
        "New revision saved. Existing job assignments still point to the previous document until you change them.",
      );
    },
    [runMutation],
  );

  const loadOpportunities = useCallback(async () => {
    setOppLoading(true);
    setOppError("");
    try {
      const next = await fetchOpportunities();
      setOppList(next.jobs);
    } catch (error) {
      setOppList(null);
      setOppError(error instanceof Error ? error.message : "Unable to load opportunities");
    } finally {
      setOppLoading(false);
    }
  }, []);

  useEffect(() => {
    if (assocOpen) void loadOpportunities();
  }, [assocOpen, loadOpportunities]);

  const filteredOpportunities = useMemo(() => {
    if (!oppList) return [];
    const needle = oppSearch.trim().toLocaleLowerCase();
    if (needle === "") return oppList;
    return oppList.filter((job) =>
      `${job.displayTitle} ${job.title} ${job.company} ${job.companyOrHost} ${job.host} ${job.url}`.toLocaleLowerCase().includes(needle),
    );
  }, [oppList, oppSearch]);

  const selectedJob = useMemo(
    () => oppList?.find((job) => job.id === selectedJobId) ?? null,
    [oppList, selectedJobId],
  );

  // The selected destination stays visible even when the search text
  // filters its row out of the results.
  const visibleOpportunities = useMemo(() => {
    if (selectedJob && !filteredOpportunities.some((job) => job.id === selectedJob.id)) {
      return [selectedJob, ...filteredOpportunities];
    }
    return filteredOpportunities;
  }, [filteredOpportunities, selectedJob]);

  const handleSelectJob = useCallback(
    async (jobId: number) => {
      if (jobLoading || assignBusy) return;
      setSelectedJobId(jobId);
      setJobOptions([]);
      setJobSelected(0);
      setJobTitle("");
      setJobCompany("");
      setJobError("");
      setAssignNotice("");
      setAssignError("");
      setJobLoading(true);
      try {
        const result = await fetchResumeOptions(jobId);
        setJobOptions(result.options);
        setJobTitle(result.job.title !== "" ? result.job.title : `Job ${result.job.id}`);
        setJobCompany(result.job.company);
        setJobSelected(result.selectedResumeId);
      } catch (error) {
        setJobError(error instanceof Error ? error.message : "Unable to load document choices");
      } finally {
        setJobLoading(false);
      }
    },
    [jobLoading, assignBusy],
  );

  const handleRetryJobOptions = useCallback(async () => {
    if (selectedJobId === null || jobLoading || assignBusy) return;
    setJobError("");
    setJobLoading(true);
    try {
      const result = await fetchResumeOptions(selectedJobId);
      setJobOptions(result.options);
      setJobTitle(result.job.title !== "" ? result.job.title : `Job ${result.job.id}`);
      setJobCompany(result.job.company);
      setJobSelected(result.selectedResumeId);
    } catch (error) {
      setJobError(error instanceof Error ? error.message : "Unable to load document choices");
    } finally {
      setJobLoading(false);
    }
  }, [selectedJobId, jobLoading, assignBusy]);

  // An explicit resume radio is the save action. The write result and a
  // later options refresh are reported separately: a committed write
  // followed by a failed read stays a saved selection, never a failure.
  const handleAssign = useCallback(
    async (resumeId: number) => {
      const jobId = selectedJobId;
      if (jobId === null || jobLoading || assignBusy) return;
      setAssignBusy(true);
      setAssignError("");
      setAssignNotice("");
      const destination = jobTitle !== "" ? `${jobTitle}${jobCompany !== "" ? ` — ${jobCompany}` : ""}` : `Job ${jobId}`;
      try {
        await assignJobResume(jobId, resumeId);
        setJobSelected(resumeId);
        const chosen = jobOptions.find((option) => option.id === resumeId);
        const chosenLabel =
          resumeId === 0 ? "No resume selected" : chosen ? `${chosen.name} · Version ${chosen.version}` : `Resume ${resumeId}`;
        setAssignNotice(`Selection saved: ${chosenLabel} for ${destination}.`);
        try {
          const result = await fetchResumeOptions(jobId);
          setJobOptions(result.options);
          setJobSelected(result.selectedResumeId);
        } catch {
          setAssignError("Selection saved; document choices could not refresh.");
        }
      } catch (error) {
        setAssignError(
          error instanceof Error
            ? `${error.message} Could not confirm the change. Reload choices to check the saved selection.`
            : "Could not confirm the change. Reload choices to check the saved selection.",
        );
      } finally {
        setAssignBusy(false);
      }
    },
    [selectedJobId, jobLoading, assignBusy, jobTitle, jobCompany, jobOptions],
  );

  const handleDelete = useCallback(
    (id: number) => {
      if (deleteTarget !== id || !deleteConfirm || deletePhrase !== "DELETE") {
        setActionError("Delete: check confirmation and type DELETE.");
        return;
      }
      void runMutation("Delete", () => deleteResume(id), "Resume permanently deleted.").finally(() => {
        setDeleteTarget(null);
        setDeletePhrase("");
        setDeleteConfirm(false);
      });
    },
    [deleteTarget, deleteConfirm, deletePhrase, runMutation],
  );

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      <AppNavigation active="documents" />
      <div className="app-main">
        <AppTopbar />
        <main id="main-content" className="page-container">
          <div className="page-heading">
            <div>
              <h1>Documents</h1>
              <p className="page-description">
                Keep your résumé versions and choose which one each opportunity uses.
              </p>
            </div>
            <div className="heading-actions">
              <a className="text-link" href="/families">
                Manage role families
              </a>
              <span className="summary-count">
                {data ? `${data.total} ${data.total === 1 ? "resume" : "resumes"}` : "…"}
              </span>
            </div>
          </div>

          {loading && <p role="status">Loading documents…</p>}
          {loadError !== "" && (
            <section className="panel" aria-label="Loading error">
              <p>{loadError}</p>
              <button className="button button-secondary" type="button" onClick={() => void reload()}>
                Retry
              </button>
            </section>
          )}
          {notice !== "" && (
            <section className="panel library-notice" role="status">
              <p>{notice}</p>
            </section>
          )}
          {actionError !== "" && (
            <p className="job-form-error" role="alert">
              {actionError}
            </p>
          )}

          {data && (
            <>
              <section className="panel docs-import-toolbar" aria-label="Import a resume">
                <div className="docs-import-text">
                  <h2>Import a resume</h2>
                  <p className="section-subtitle">PDF or DOCX, up to 10 MiB. Stored locally.</p>
                </div>
                {data.desktop ? (
                  <button
                    className="button button-primary"
                    type="button"
                    disabled={busy}
                    onClick={() =>
                      void runMutation("Native import", () => importResumeNative(), (result) => {
                        const imported = result as { duplicate: boolean };
                        return imported.duplicate
                          ? "That file is already in your document library. Existing bytes and document identity were kept."
                          : "Resume imported from the native picker.";
                      })
                    }
                  >
                    Choose PDF or DOCX <span aria-hidden="true">↗</span>
                  </button>
                ) : (
                  <label className="field docs-import-file">
                    <span>Select a file</span>
                    <input
                      type="file"
                      accept=".pdf,.docx"
                      disabled={busy}
                      onChange={(event) => {
                        handleImportFile(event.target.files?.[0]);
                        event.target.value = "";
                      }}
                    />
                  </label>
                )}
              </section>

              <details
                className="panel docs-assoc"
                open={assocOpen}
                onToggle={(event) => setAssocOpen(event.currentTarget.open)}
              >
                <summary>
                  <span>
                    <strong>Associate a résumé with an opportunity</strong>
                  </span>
                  <span className="summary-chevron" aria-hidden="true">
                    ⌄
                  </span>
                </summary>
                <div className="docs-assoc-body">
                  <div className="docs-assoc-grid">
                    <div className="docs-assoc-destination">
                      <h3>Choose an opportunity</h3>
                      <div className="field">
                        <label htmlFor="assoc-opportunity-search">Search saved opportunities</label>
                        <input
                          id="assoc-opportunity-search"
                          type="search"
                          placeholder="Title, company, or posting source"
                          autoComplete="off"
                          value={oppSearch}
                          disabled={oppLoading || jobLoading || assignBusy}
                          onChange={(event) => setOppSearch(event.target.value)}
                        />
                      </div>
                      {oppLoading && <p role="status">Loading opportunities…</p>}
                      {oppError !== "" && (
                        <>
                          <p className="job-form-error" role="alert">
                            {oppError}
                          </p>
                          <button className="button button-secondary" type="button" onClick={() => void loadOpportunities()}>
                            Retry
                          </button>
                        </>
                      )}
                      {!oppLoading && oppError === "" && oppList !== null && (
                        <>
                          {oppList.length === 0 ? (
                            <p className="small-muted">
                              No saved opportunities. <a href="/jobs#add-opportunity">Save an opportunity</a> to
                              associate a résumé.
                            </p>
                          ) : visibleOpportunities.length === 0 ? (
                            <p className="small-muted" role="status">
                              No matching opportunities.
                            </p>
                          ) : (
                            <ul className="assoc-results" aria-label="Saved opportunities">
                              {visibleOpportunities.map((job) => (
                                <li key={job.id} className={job.id === selectedJobId ? "assoc-result is-selected" : "assoc-result"}>
                                  <label>
                                    <input
                                      type="radio"
                                      name="assoc-opportunity"
                                      checked={job.id === selectedJobId}
                                      disabled={jobLoading || assignBusy}
                                      onChange={() => void handleSelectJob(job.id)}
                                    />
                                    <span className="assoc-result-text">
                                      <strong>{job.displayTitle}</strong>
                                      <small>
                                        {job.companyOrHost} · {job.stageLabel}
                                      </small>
                                      <small className="assoc-result-url">{job.url}</small>
                                    </span>
                                  </label>
                                </li>
                              ))}
                            </ul>
                          )}
                        </>
                      )}
                    </div>
                    <div className="docs-assoc-choice">
                      {selectedJobId === null ? (
                        <p className="small-muted" role="status">
                          Choose an opportunity to see its selected résumé and eligible alternatives.
                        </p>
                      ) : (
                        <>
                          <h3>
                            {jobTitle !== "" ? jobTitle : `Job ${selectedJobId}`}
                            {jobCompany !== "" ? ` — ${jobCompany}` : ""}
                          </h3>
                          {jobLoading && <p role="status">Loading document choices…</p>}
                          {jobError !== "" && (
                            <>
                              <p className="job-form-error" role="alert">
                                {jobError}
                              </p>
                              <button
                                className="button button-secondary"
                                type="button"
                                onClick={() => void handleRetryJobOptions()}
                                disabled={jobLoading || assignBusy}
                              >
                                Retry
                              </button>
                            </>
                          )}
                          {!jobLoading && jobError === "" && (
                            <>
                              <p className="section-subtitle">
                                Selecting a résumé saves it immediately for {jobTitle !== "" ? jobTitle : `Job ${selectedJobId}`}
                                {jobCompany !== "" ? ` — ${jobCompany}` : ""}.
                              </p>
                              {jobOptions.length === 0 && (
                                <p className="small-muted">No selectable résumés for this opportunity.</p>
                              )}
                              <ul className="resume-options">
                                <li>
                                  <label className="confirm-row">
                                    <input
                                      type="radio"
                                      name="job-resume"
                                      checked={jobSelected === 0}
                                      onChange={() => void handleAssign(0)}
                                      disabled={assignBusy || jobLoading}
                                    />
                                    <span>No resume (clear selection)</span>
                                  </label>
                                </li>
                                {jobOptions.map((option) => (
                                  <li key={option.id}>
                                    <label className="confirm-row">
                                      <input
                                        type="radio"
                                        name="job-resume"
                                        checked={jobSelected === option.id}
                                        onChange={() => void handleAssign(option.id)}
                                        disabled={assignBusy || jobLoading}
                                      />
                                      <span>
                                        {option.name} · {option.kind} · Version {option.version}
                                        {option.roleFamilyName !== "" ? ` · ${option.roleFamilyName}` : ""}
                                      </span>
                                    </label>
                                  </li>
                                ))}
                              </ul>
                              {assignBusy && <p role="status">Saving…</p>}
                              {assignNotice !== "" && (
                                <p className="form-notice" role="status">
                                  {assignNotice} <a href={`/jobs/${selectedJobId}`}>Open workspace</a>
                                </p>
                              )}
                              {assignError !== "" && (
                                <>
                                  <p className="job-form-error" role="alert">
                                    {assignError}
                                  </p>
                                  <button
                                    className="button button-secondary"
                                    type="button"
                                    onClick={() => void handleRetryJobOptions()}
                                    disabled={jobLoading || assignBusy}
                                  >
                                    Reload choices
                                  </button>
                                </>
                              )}
                            </>
                          )}
                        </>
                      )}
                    </div>
                  </div>
                </div>
              </details>

              <section className="panel list-panel" aria-labelledby="resumes-list-title">
                <div className="section-heading">
                  <div>
                    <h2 id="resumes-list-title">Saved documents</h2>
                  </div>
                </div>
                <div className="list-toolbar docs-toolbar">
                  <div className="field search-field">
                    <label htmlFor="resume-search">Search documents</label>
                    <input
                      id="resume-search"
                      type="search"
                      placeholder="Name, kind, or family"
                      autoComplete="off"
                      value={query}
                      onChange={(event) => setQuery(event.target.value)}
                    />
                  </div>
                  <label className="confirm-row docs-archived-toggle">
                    <input
                      type="checkbox"
                      checked={showArchived}
                      onChange={(event) => setShowArchived(event.target.checked)}
                    />
                    <span>Show archived</span>
                  </label>
                </div>
                <div className="resume-list">
                  {visible.map((resume) => (
                    <article key={resume.id} className="resume-row document-row">
                      <div className="resume-file-icon" aria-hidden="true">
                        {resume.kind === ".pdf" ? "PDF" : "DOCX"}
                      </div>
                      <div className="resume-info">
                        <h3>{resume.name}</h3>
                        <p>
                          {resume.kind} · Version {resume.version} · {resume.displaySize}
                        </p>
                        <p className={resume.archived ? "document-unassigned" : "document-family"}>{statusLabel(resume)}</p>
                      </div>
                      <a className="button button-secondary" href={`/resumes/${resume.id}/download`}>
                        Download ↓
                      </a>
                      <details className="document-manage">
                        <summary>
                          <span>Manage document</span>
                          <span className="summary-chevron" aria-hidden="true">
                            ⌄
                          </span>
                        </summary>
                        <div className="document-manage-body">
                          <p className="small-muted">
                            Imported {resume.importedAt}. Master resumes provide career evidence for cover letters,
                            not employer uploads. Deletion is refused for referenced or master documents; archive
                            them instead.
                          </p>
                          {!resume.isMaster && !resume.archived && (
                            <form
                              className="document-routing-form"
                              onSubmit={(event) => {
                                event.preventDefault();
                                const form = new FormData(event.currentTarget);
                                const familyId = Number.parseInt(String(form.get("family_id") ?? "0"), 10);
                                const primary = form.get("primary") === "yes";
                                void runMutation("Routing", () => setResumeFamily(resume.id, familyId, primary), "Routing saved.");
                              }}
                            >
                              <div className="field">
                                <label htmlFor={`family-for-${resume.id}`}>Role family</label>
                                <select id={`family-for-${resume.id}`} name="family_id" defaultValue={String(resume.roleFamilyId)}>
                                  <option value="0">Unassigned</option>
                                  {data.families.map((family) => (
                                    <option key={family.id} value={String(family.id)}>
                                      {family.name}
                                    </option>
                                  ))}
                                </select>
                              </div>
                              <label className="confirm-row">
                                <input type="checkbox" name="primary" value="yes" defaultChecked={resume.isPrimary} />
                                <span>Primary resume for this family</span>
                              </label>
                              <button className="button button-secondary" type="submit" disabled={busy}>
                                Save routing
                              </button>
                            </form>
                          )}
                          {!resume.archived && (
                            <button
                              className="button button-secondary"
                              type="button"
                              disabled={busy}
                              onClick={() =>
                                void runMutation(
                                  "Master",
                                  () => setResumeMaster(resume.id, !resume.isMaster),
                                  resume.isMaster ? "Master designation removed." : "Designated as master resume.",
                                )
                              }
                            >
                              {resume.isMaster ? "Remove master designation" : "Set as master resume"}
                            </button>
                          )}
                          {!resume.archived ? (
                            <>
                              {data.desktop ? (
                                <button
                                  className="button button-secondary"
                                  type="button"
                                  disabled={busy}
                                  onClick={() =>
                                    void runMutation("Native revision", () => reviseResumeNative(resume.id), "New revision saved.")
                                  }
                                >
                                  Import new revision (native picker)
                                </button>
                              ) : (
                                <label className="field">
                                  <span>Revised PDF or DOCX</span>
                                  <input
                                    type="file"
                                    accept=".pdf,.docx"
                                    disabled={busy}
                                    onChange={(event) => {
                                      handleReviseFile(resume.id, event.target.files?.[0]);
                                      event.target.value = "";
                                    }}
                                  />
                                </label>
                              )}
                              <form
                                className="document-routing-form"
                                onSubmit={(event) => {
                                  event.preventDefault();
                                  void runMutation("Archive", () => setResumeArchived(resume.id, "archive", true), "Resume archived. Job assignments and history are unchanged.");
                                }}
                              >
                                <p className="confirmation-note">
                                  Archive removes this document from current choices while preserving application evidence.
                                </p>
                                <button className="button button-secondary" type="submit" disabled={busy}>
                                  Archive
                                </button>
                              </form>
                            </>
                          ) : (
                            <button
                              className="button button-secondary"
                              type="button"
                              disabled={busy}
                              onClick={() =>
                                void runMutation("Restore", () => setResumeArchived(resume.id, "restore", false), "Resume restored to the active library.")
                              }
                            >
                              Restore to active library
                            </button>
                          )}
                          <details className="danger-zone">
                            <summary>Delete resume</summary>
                            <div className="danger-zone-body">
                              <p className="confirmation-note">
                                Permanent removal is allowed only for unreferenced, non-master files. Archiving preserves the
                                exact submitted document and never changes a job assignment.
                              </p>
                              {deleteTarget === resume.id ? (
                                <form
                                  className="job-form"
                                  onSubmit={(event) => {
                                    event.preventDefault();
                                    handleDelete(resume.id);
                                  }}
                                >
                                  <label className="confirm-row">
                                    <input
                                      type="checkbox"
                                      checked={deleteConfirm}
                                      onChange={(event) => setDeleteConfirm(event.target.checked)}
                                    />
                                    <span>I understand this document will be permanently deleted.</span>
                                  </label>
                                  <div className="field">
                                    <label htmlFor={`delete-resume-${resume.id}`}>Type DELETE to confirm</label>
                                    <input
                                      id={`delete-resume-${resume.id}`}
                                      autoComplete="off"
                                      value={deletePhrase}
                                      onChange={(event) => setDeletePhrase(event.target.value)}
                                    />
                                  </div>
                                  <button className="button button-danger" type="submit" disabled={busy}>
                                    Delete this resume
                                  </button>
                                </form>
                              ) : (
                                <button
                                  className="button button-secondary"
                                  type="button"
                                  onClick={() => {
                                    setDeleteTarget(resume.id);
                                    setDeletePhrase("");
                                    setDeleteConfirm(false);
                                  }}
                                >
                                  Delete…
                                </button>
                              )}
                            </div>
                          </details>
                        </div>
                      </details>
                    </article>
                  ))}
                  {visible.length === 0 && (
                    <div className="empty-state">
                      <div className="empty-mark" aria-hidden="true">
                        ▤
                      </div>
                      <h3>{data.total === 0 ? "Your document library is empty" : "No matching documents"}</h3>
                      <p>
                        {data.total === 0
                          ? "Import a PDF or DOCX to start preparing applications."
                          : "Try a different search or show archived documents."}
                      </p>
                    </div>
                  )}
                </div>
              </section>
            </>
          )}
        </main>
      </div>
    </div>
  );
}
