import { useCallback, useEffect, useMemo, useState } from "react";
import {
  addOpportunity,
  deleteOpportunities,
  fetchOpportunities,
  importOpportunities,
  openPosting,
  type BatchResult,
  type OpportunitiesResponse,
  type OpportunityJob,
} from "./api";

type StageFilter = "active" | "all" | "review" | "ready" | "applied" | "closed";
type SortKey = "priority" | "newest" | "company";

const STAGE_FILTERS: { value: StageFilter; label: string }[] = [
  { value: "active", label: "Still to work on" },
  { value: "all", label: "All stages" },
  { value: "review", label: "Review" },
  { value: "ready", label: "Marked ready" },
  { value: "applied", label: "Applied" },
  { value: "closed", label: "Closed" },
];

function priorityRank(priority: string): number {
  if (priority === "high") return 0;
  if (priority === "low") return 2;
  return 1;
}

function stageRank(stage: string): number {
  if (stage === "ready") return 0;
  if (stage === "review") return 1;
  if (stage === "applied") return 2;
  return 3;
}

function isApplied(job: OpportunityJob): boolean {
  return job.submissionCount > 0 || job.stage === "applied";
}

function isClosed(job: OpportunityJob): boolean {
  return job.stage === "closed";
}

export function OpportunitiesPage(): React.JSX.Element {
  const [data, setData] = useState<OpportunitiesResponse | null>(null);
  const [loadError, setLoadError] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);

  const [query, setQuery] = useState<string>("");
  const [stage, setStage] = useState<StageFilter>("active");
  const [sort, setSort] = useState<SortKey>("priority");

  const [singleUrl, setSingleUrl] = useState<string>("");
  const [singleTitle, setSingleTitle] = useState<string>("");
  const [singleError, setSingleError] = useState<string>("");
  const [singleNotice, setSingleNotice] = useState<string>("");
  const [singleBusy, setSingleBusy] = useState<boolean>(false);

  const [bulkText, setBulkText] = useState<string>("");
  const [bulkError, setBulkError] = useState<string>("");
  const [bulkResult, setBulkResult] = useState<BatchResult | null>(null);
  const [bulkBusy, setBulkBusy] = useState<boolean>(false);

  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [deletePhrase, setDeletePhrase] = useState<string>("");
  const [deleteConfirm, setDeleteConfirm] = useState<boolean>(false);
  const [deleteError, setDeleteError] = useState<string>("");
  const [deleteBusy, setDeleteBusy] = useState<boolean>(false);

  const [actionError, setActionError] = useState<string>("");

  const reload = useCallback(async () => {
    setLoading(true);
    setLoadError("");
    try {
      const next = await fetchOpportunities();
      setData(next);
      setSelected((previous) => {
        const valid = new Set<number>();
        for (const job of next.jobs) {
          if (previous.has(job.id)) valid.add(job.id);
        }
        return valid;
      });
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : "Unable to load jobs");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  const visibleJobs = useMemo(() => {
    if (!data) return [];
    const needle = query.trim().toLocaleLowerCase();
    const filtered = data.jobs.filter((job) => {
      if (stage === "active") {
        if (isApplied(job) || isClosed(job)) return false;
      } else if (stage !== "all" && job.stage !== stage) {
        if (!(stage === "applied" && isApplied(job))) return false;
      }
      if (needle === "") return true;
      const haystack =
        `${job.title} ${job.company} ${job.host} ${job.url}`.toLocaleLowerCase();
      return haystack.includes(needle);
    });
    const ordered = [...filtered];
    ordered.sort((a, b) => {
      if (sort === "newest") return b.id - a.id;
      if (sort === "company") {
        const company = a.companyOrHost.localeCompare(b.companyOrHost);
        if (company !== 0) return company;
        return b.id - a.id;
      }
      const aTerminal = isApplied(a) || isClosed(a) ? 1 : 0;
      const bTerminal = isApplied(b) || isClosed(b) ? 1 : 0;
      if (aTerminal !== bTerminal) return aTerminal - bTerminal;
      const priority = priorityRank(a.priority) - priorityRank(b.priority);
      if (priority !== 0) return priority;
      const stageOrder = stageRank(a.stage) - stageRank(b.stage);
      if (stageOrder !== 0) return stageOrder;
      return b.id - a.id;
    });
    return ordered;
  }, [data, query, stage, sort]);

  const toggleSelected = useCallback((id: number, checked: boolean) => {
    setSelected((previous) => {
      const next = new Set(previous);
      if (checked) next.add(id);
      else next.delete(id);
      return next;
    });
  }, []);

  const handleSingleSubmit = useCallback(
    async (event: React.FormEvent) => {
      event.preventDefault();
      setSingleError("");
      setSingleNotice("");
      setSingleBusy(true);
      try {
        const result = await addOpportunity(singleUrl.trim(), singleTitle.trim());
        if (result.duplicate) {
          setSingleNotice("Already in your workspace; existing details retained.");
        } else {
          setSingleNotice("Saved. Review and refresh when ready.");
          setSingleUrl("");
          setSingleTitle("");
        }
        await reload();
      } catch (error) {
        setSingleError(error instanceof Error ? error.message : "Unable to save job");
      } finally {
        setSingleBusy(false);
      }
    },
    [singleUrl, singleTitle, reload],
  );

  const handleBulkSubmit = useCallback(
    async (event: React.FormEvent) => {
      event.preventDefault();
      setBulkError("");
      setBulkResult(null);
      setBulkBusy(true);
      try {
        const result = await importOpportunities(bulkText);
        setBulkResult(result);
        await reload();
      } catch (error) {
        setBulkError(error instanceof Error ? error.message : "Unable to import jobs");
      } finally {
        setBulkBusy(false);
      }
    },
    [bulkText, reload],
  );

  const handleDelete = useCallback(
    async (event: React.FormEvent) => {
      event.preventDefault();
      setDeleteError("");
      if (selected.size === 0) {
        setDeleteError("Select at least one job to delete.");
        return;
      }
      if (!deleteConfirm || deletePhrase !== "DELETE") {
        setDeleteError("Check confirmation and type DELETE.");
        return;
      }
      setDeleteBusy(true);
      try {
        await deleteOpportunities([...selected]);
        setSelected(new Set());
        setDeletePhrase("");
        setDeleteConfirm(false);
        await reload();
      } catch (error) {
        setDeleteError(error instanceof Error ? error.message : "Unable to delete jobs");
      } finally {
        setDeleteBusy(false);
      }
    },
    [selected, deleteConfirm, deletePhrase, reload],
  );

  const handleOpenPosting = useCallback(async (job: OpportunityJob) => {
    setActionError("");
    try {
      await openPosting(job.id);
      window.location.href = `/jobs/${job.id}`;
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "Could not open posting");
    }
  }, []);

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
            <a className="nav-item is-active" aria-current="page" href="/jobs">
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
              <h1>Opportunities</h1>
            </div>
            <div className="heading-actions">
              <a className="button button-secondary" href="/resumes">
                Document library
              </a>
              <a className="button button-primary" href="#add-opportunity">
                + Add opportunity
              </a>
            </div>
          </div>

          {loading && <p role="status">Loading opportunities…</p>}
          {loadError !== "" && (
            <section className="panel" aria-label="Loading error">
              <p>{loadError}</p>
              <button className="button button-secondary" type="button" onClick={() => void reload()}>
                Retry
              </button>
            </section>
          )}

          {data && (
            <>
              <section className="panel opportunity-dashboard" aria-labelledby="opportunity-progress-title">
                <div className="queue-progress">
                  <h2 className="visually-hidden" id="opportunity-progress-title">
                    Queue progress
                  </h2>
                  <div className="progress-metrics">
                    <button
                      type="button"
                      className="progress-metric progress-remaining"
                      onClick={() => {
                        setStage("active");
                        setQuery("");
                      }}
                    >
                      <strong>{data.remaining}</strong> <span>left to work on</span>
                    </button>
                    <div className="progress-metric">
                      <strong>{data.recordedToday}</strong> <span>recorded today</span>
                    </div>
                    <a className="progress-metric" href="/applications">
                      <strong>{data.applied}</strong> <span>recorded overall</span>
                    </a>
                  </div>
                  <details className="queue-details">
                    <summary>Progress details</summary>
                    {data.queueTotal > 0 ? (
                      <>
                        <div className="queue-progress-caption">
                          <span>Current queue</span>
                          <strong>{data.recordedPercent}% recorded</strong>
                        </div>
                        <progress
                          className="queue-progress-bar"
                          value={data.applied}
                          max={data.queueTotal}
                          aria-label="Applications recorded out of active and applied opportunities"
                        >
                          {data.applied} of {data.queueTotal}
                        </progress>
                        <p className="queue-progress-note">
                          {data.applied} of {data.queueTotal} opportunities in your current queue. Closed
                          jobs excluded.
                        </p>
                      </>
                    ) : (
                      <p className="queue-progress-note">
                        Save an opportunity to start your next application.
                      </p>
                    )}
                  </details>
                </div>
                <div className="dashboard-next">
                  {data.next ? (
                    <>
                      <p className="eyebrow">Suggested next opportunity</p>
                      <h2>{data.next.title}</h2>
                      <p className="dashboard-next-company">{data.next.company}</p>
                      <p className="small-muted">{data.next.reason}</p>
                      <a className="button button-primary" href={data.next.href}>
                        {data.next.label} <span aria-hidden="true">→</span>
                      </a>
                    </>
                  ) : data.applied > 0 ? (
                    <>
                      <p className="eyebrow">CURRENT QUEUE COMPLETE</p>
                      <h2>Your applications are recorded.</h2>
                      <p className="small-muted">
                        Review your history or save another opportunity to keep going.
                      </p>
                      <a className="button button-secondary" href="/applications">
                        Review applications →
                      </a>
                    </>
                  ) : (
                    <>
                      <p className="eyebrow">YOUR NEXT STEP</p>
                      <h2>Start with a job you want.</h2>
                      <a className="button button-primary" href="#add-opportunity">
                        Save an opportunity →
                      </a>
                    </>
                  )}
                </div>
              </section>

              <details className="panel create-panel" id="add-opportunity" open>
                <summary>
                  <span className="summary-icon">+</span>
                  <span>
                    <strong>Add an opportunity</strong>
                    <small>Save a job posting and fetch supported metadata.</small>
                  </span>
                  <span className="summary-chevron" aria-hidden="true">
                    ⌄
                  </span>
                </summary>
                <div className="create-body">
                  <form className="job-form" onSubmit={(event) => void handleSingleSubmit(event)}>
                    <div className="field">
                      <label htmlFor="job_url">Job posting URL</label>
                      <input
                        id="job_url"
                        name="job_url"
                        type="url"
                        placeholder="https://jobs.ashbyhq.com/..."
                        required
                        autoComplete="url"
                        value={singleUrl}
                        onChange={(event) => setSingleUrl(event.target.value)}
                      />
                    </div>
                    <div className="field">
                      <label htmlFor="job_title">
                        Job title <span className="muted">(optional)</span>
                      </label>
                      <input
                        id="job_title"
                        name="job_title"
                        type="text"
                        placeholder="Fetched automatically when available"
                        maxLength={200}
                        value={singleTitle}
                        onChange={(event) => setSingleTitle(event.target.value)}
                      />
                    </div>
                    {singleError !== "" && (
                      <p className="job-form-error" role="alert">
                        {singleError}
                      </p>
                    )}
                    {singleNotice !== "" && <p className="form-notice">{singleNotice}</p>}
                    <div className="form-actions">
                      <button className="button button-primary" type="submit" disabled={singleBusy}>
                        {singleBusy ? "Saving…" : "Save opportunity"}
                      </button>
                    </div>
                  </form>
                  <details className="batch-disclosure">
                    <summary>
                      <strong>Import multiple job URLs</strong>
                      <span className="muted">Up to 50 at once</span>
                    </summary>
                    <form className="job-form batch-import-form" onSubmit={(event) => void handleBulkSubmit(event)}>
                      <div className="field field-full">
                        <label htmlFor="job_urls">One job posting URL per line</label>
                        <textarea
                          id="job_urls"
                          name="job_urls"
                          rows={7}
                          maxLength={128000}
                          placeholder={"https://jobs.ashbyhq.com/...\nhttps://jobs.lever.co/..."}
                          required
                          value={bulkText}
                          onChange={(event) => setBulkText(event.target.value)}
                        />
                        <small>
                          Duplicates are skipped. Batch import saves URLs locally; use Posting
                          checks to retrieve supported details without opening each job.
                        </small>
                      </div>
                      {bulkError !== "" && (
                        <p className="job-form-error" role="alert">
                          {bulkError}
                        </p>
                      )}
                      <div className="form-actions">
                        <button className="button button-secondary" type="submit" disabled={bulkBusy}>
                          {bulkBusy ? "Importing…" : "Import opportunities"}
                        </button>
                      </div>
                    </form>
                    {bulkResult && (
                      <section className="batch-results" aria-label="Import results">
                        <p>
                          Saved {bulkResult.added} · Already saved {bulkResult.duplicates} · Invalid{" "}
                          {bulkResult.invalid}
                        </p>
                        <ul>
                          {bulkResult.items.map((item) => (
                            <li key={`${item.line}-${item.url}`}>
                              <span>
                                Line {item.line} · {item.status}
                              </span>
                              <span className="batch-url">{item.url}</span>
                              <span>{item.detail}</span>
                              {item.jobId > 0 && <a href={`/jobs/${item.jobId}`}>Open workspace →</a>}
                            </li>
                          ))}
                        </ul>
                      </section>
                    )}
                  </details>
                </div>
              </details>

              <section className="panel list-panel" aria-labelledby="saved-jobs-title">
                <div className="section-heading">
                  <div>
                    <h2 id="saved-jobs-title" className="visually-hidden">
                      Saved opportunities
                    </h2>
                    <p className="small-muted" id="results-status" role="status" aria-live="polite">
                      {visibleJobs.length} shown · {data.total} saved in total
                    </p>
                  </div>
                </div>
                <div className="list-toolbar">
                  <div className="field search-field">
                    <label htmlFor="job-search">Search opportunities</label>
                    <input
                      id="job-search"
                      type="search"
                      placeholder="Role, company or source"
                      autoComplete="off"
                      value={query}
                      onChange={(event) => setQuery(event.target.value)}
                    />
                  </div>
                  <div className="field select-field">
                    <label htmlFor="job-stage-filter">Stage</label>
                    <select
                      id="job-stage-filter"
                      value={stage}
                      onChange={(event) => setStage(event.target.value as StageFilter)}
                    >
                      {STAGE_FILTERS.map((option) => (
                        <option key={option.value} value={option.value}>
                          {option.label}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div className="field select-field">
                    <label htmlFor="job-sort">Order</label>
                    <select
                      id="job-sort"
                      value={sort}
                      onChange={(event) => setSort(event.target.value as SortKey)}
                    >
                      <option value="priority">Priority &amp; next steps</option>
                      <option value="newest">Recently saved</option>
                      <option value="company">Company A–Z</option>
                    </select>
                  </div>
                </div>
                {actionError !== "" && (
                  <p className="job-form-error" role="alert">
                    {actionError}
                  </p>
                )}
                <form
                  className="bulk-delete-form"
                  aria-label="Delete selected test or unwanted jobs"
                  onSubmit={(event) => void handleDelete(event)}
                >
                  <div className="job-list">
                    {visibleJobs.map((job) => {
                      const deletable = job.submissionCount === 0;
                      return (
                        <article className="job-row" key={job.id}>
                          <label className="job-select">
                            <input
                              type="checkbox"
                              checked={selected.has(job.id)}
                              disabled={!deletable}
                              title={
                                deletable
                                  ? `Select ${job.displayTitle}`
                                  : "Submitted job: remove individually from its workspace"
                              }
                              aria-label={`Select ${job.displayTitle} for deletion`}
                              onChange={(event) => toggleSelected(job.id, event.target.checked)}
                            />
                          </label>
                          <div className="job-row-body">
                            <div className="job-row-meta">
                              <span className="company-label">{job.companyOrHost}</span>
                              <span className={`status status-${job.stage}`}>{job.stageLabel}</span>
                              {job.priority === "high" && (
                                <span className="priority-flag">High priority</span>
                              )}
                              {job.priority === "low" && (
                                <span className="priority-quiet">Low priority</span>
                              )}
                            </div>
                            <h3>
                              <a className="job-title" href={`/jobs/${job.id}`}>
                                {job.displayTitle}
                              </a>
                            </h3>
                            <p className="job-row-detail">
                              {job.selectedResumeId > 0 ? "Resume selected" : "No resume selected"}{" "}
                              <span className="dot-sep" aria-hidden="true">
                                ·
                              </span>{" "}
                              {job.actionLabel}
                            </p>
                          </div>
                          <div className="job-row-actions">
                            {data.desktop ? (
                              <button
                                type="button"
                                className="text-link row-secondary posting-open-button"
                                onClick={() => void handleOpenPosting(job)}
                              >
                                Posting ↗
                              </button>
                            ) : (
                              <a
                                className="text-link row-secondary"
                                href={job.url}
                                target="_blank"
                                rel="noopener noreferrer"
                              >
                                Posting ↗
                              </a>
                            )}
                            <a className="button button-secondary row-primary" href={`/jobs/${job.id}`}>
                              Open workspace <span aria-hidden="true">→</span>
                            </a>
                          </div>
                        </article>
                      );
                    })}
                    {visibleJobs.length === 0 && (
                      <div className="empty-state">
                        <h3>No matching opportunities</h3>
                        <p>Try a different search or stage.</p>
                        <button
                          className="button button-secondary"
                          type="button"
                          onClick={() => {
                            setQuery("");
                            setStage("active");
                            setSort("priority");
                          }}
                        >
                          Clear filters
                        </button>
                      </div>
                    )}
                  </div>
                  {data.jobs.length > 0 && (
                    <details className="bulk-remove">
                      <summary>
                        Delete selected jobs <span className="muted">(test and unwanted records)</span>
                      </summary>
                      <div className="bulk-remove-body">
                        <p className="confirmation-note">
                          Permanent deletion. Jobs with confirmed submissions are excluded here;
                          remove them individually from their workspace.
                        </p>
                        <div className="field">
                          <label htmlFor="delete_selected_phrase">Type DELETE to confirm</label>
                          <input
                            id="delete_selected_phrase"
                            required
                            pattern="DELETE"
                            autoComplete="off"
                            value={deletePhrase}
                            onChange={(event) => setDeletePhrase(event.target.value)}
                          />
                        </div>
                        <label className="confirm-row">
                          <input
                            type="checkbox"
                            required
                            checked={deleteConfirm}
                            onChange={(event) => setDeleteConfirm(event.target.checked)}
                          />
                          <span>I want to permanently remove the selected jobs.</span>
                        </label>
                        {deleteError !== "" && (
                          <p className="job-form-error" role="alert">
                            {deleteError}
                          </p>
                        )}
                        <button className="button button-danger" type="submit" disabled={deleteBusy}>
                          {deleteBusy ? "Deleting…" : "Delete selected jobs"}
                        </button>
                      </div>
                    </details>
                  )}
                </form>
              </section>
            </>
          )}
        </main>
      </div>
    </div>
  );
}
