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
  revised: "New revision saved. Existing job assignments still point to the original document until you update them.",
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

  const [jobIdText, setJobIdText] = useState<string>("");
  const [jobOptions, setJobOptions] = useState<ResumeOption[]>([]);
  const [jobTitle, setJobTitle] = useState<string>("");
  const [jobSelected, setJobSelected] = useState<number>(0);
  const [jobError, setJobError] = useState<string>("");

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

  const handleLoadJob = useCallback(async () => {
    setJobError("");
    const jobId = Number.parseInt(jobIdText, 10);
    if (!Number.isInteger(jobId) || jobId <= 0) {
      setJobError("Enter a numeric job ID from your opportunities list.");
      return;
    }
    try {
      const result = await fetchResumeOptions(jobId);
      setJobOptions(result.options);
      setJobTitle(result.job.title !== "" ? result.job.title : `Job ${result.job.id}`);
      setJobSelected(result.selectedResumeId);
    } catch (error) {
      setJobError(error instanceof Error ? error.message : "Unable to load job options");
    }
  }, [jobIdText]);

  const handleAssign = useCallback(
    (resumeId: number) => {
      const jobId = Number.parseInt(jobIdText, 10);
      if (!Number.isInteger(jobId) || jobId <= 0) return;
      void runMutation("Assignment", async () => {
        await assignJobResume(jobId, resumeId);
        const result = await fetchResumeOptions(jobId);
        setJobOptions(result.options);
        setJobSelected(result.selectedResumeId);
      }, "Selection saved for that job.");
    },
    [jobIdText, runMutation],
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
            <a className="nav-item is-active" aria-current="page" href="/resumes">
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
          <div className="nav-group">
            <p className="nav-caption">Preferences and tools</p>
            <a className="nav-item" href="/ai">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="m12 2 1.6 5.4L19 9l-5.4 1.6L12 16l-1.6-5.4L5 9l5.4-1.6L12 2Z" />
                <path d="m19 16 .6 2.4L22 19l-2.4.6L19 22l-.6-2.4L16 19l2.4-.6L19 16Z" />
              </svg>
              AI connections
            </a>
            <a className="nav-item" href="/utilities">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M12 3v3m0 12v3m9-9h-3M6 12H3m15.4-6.4-2.1 2.1M7.7 16.3l-2.1 2.1m12.8 0-2.1-2.1M7.7 7.7 5.6 5.6" />
                <circle cx="12" cy="12" r="4" />
              </svg>
              Utilities
            </a>
          </div>
        </nav>
        <div className="sidebar-bottom">
          <span className="local-dot" aria-hidden="true" />
          <span>Local workspace</span>
        </div>
      </aside>
      <div className="app-main">
        <main id="main-content" className="page-container">
          <div className="page-heading">
            <div>
              <h1>Documents</h1>
              <p className="page-description">
                Your exact resume versions, stored locally. Select a document explicitly from each job's Prepare activity.
              </p>
            </div>
            <div className="heading-actions">
              <a className="button button-secondary" href="/families">
                Manage role families →
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
              <section className="panel" aria-labelledby="import-title">
                <div className="library-import-heading">
                  <h2 id="import-title">Import a resume</h2>
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
                  <label className="field">
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

              <section className="panel" aria-labelledby="assign-title">
                <div className="section-heading">
                  <div>
                    <h2 id="assign-title">Assign to a job</h2>
                    <p className="section-subtitle">
                      Loads the same selectable documents as the Prepare activity. Masters are never offered; archived
                      versions appear only when already selected.
                    </p>
                  </div>
                </div>
                <form
                  className="job-form"
                  onSubmit={(event) => {
                    event.preventDefault();
                    void handleLoadJob();
                  }}
                >
                  <div className="field">
                    <label htmlFor="assign-job-id">Job ID</label>
                    <input
                      id="assign-job-id"
                      inputMode="numeric"
                      autoComplete="off"
                      placeholder="e.g. 12"
                      value={jobIdText}
                      onChange={(event) => setJobIdText(event.target.value)}
                    />
                  </div>
                  <div className="form-actions">
                    <button className="button button-secondary" type="submit">
                      Load selectable documents
                    </button>
                  </div>
                </form>
                {jobError !== "" && (
                  <p className="job-form-error" role="alert">
                    {jobError}
                  </p>
                )}
                {jobOptions.length > 0 && (
                  <ul className="resume-options">
                    <li className="resume-option-head">{jobTitle}</li>
                    <li>
                      <label className="confirm-row">
                        <input
                          type="radio"
                          name="job-resume"
                          checked={jobSelected === 0}
                          onChange={() => handleAssign(0)}
                          disabled={busy}
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
                            onChange={() => handleAssign(option.id)}
                            disabled={busy}
                          />
                          <span>
                            {option.name} · {option.kind} · Version {option.version}
                            {option.roleFamilyName !== "" ? ` · ${option.roleFamilyName}` : ""}
                          </span>
                        </label>
                      </li>
                    ))}
                  </ul>
                )}
              </section>

              <section className="panel list-panel" aria-labelledby="resumes-list-title">
                <div className="section-heading">
                  <div>
                    <h2 id="resumes-list-title">Saved documents</h2>
                    <p className="section-subtitle">
                      Master resume = career evidence for cover letters, not an employer upload. Deletion is refused for
                      referenced or master documents; archive them instead.
                    </p>
                  </div>
                </div>
                <div className="list-toolbar">
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
                  <label className="confirm-row">
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
                          {resume.kind} · Version {resume.version} · {resume.displaySize} · Imported {resume.importedAt}
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
