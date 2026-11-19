import { AppNavigation, AppTopbar } from "../../shared/AppNavigation";
import { ATSPlatformIcon } from "../../shared/ATSPlatformIcon";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  addOpportunity,
  checkPostingAvailability,
  deleteOpportunities,
  fetchOpportunities,
  importOpportunities,
  isPostingClosedBlock,
  moveOpportunities,
  openPosting,
  reconcileBulkInput,
  type BatchResult,
  type OpportunitiesResponse,
  type OpportunityJob,
} from "./api";

type StageFilter = "active" | "all" | "review" | "ready" | "applied" | "closed";
type AttentionFilter = "focus" | "saved" | "history";
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
  const [attention, setAttention] = useState<AttentionFilter>("focus");
  const [stage, setStage] = useState<StageFilter>("all");
  const [sort, setSort] = useState<SortKey>("priority");
  const [createOpen, setCreateOpen] = useState<boolean>(
    () => typeof window !== "undefined" && window.location.hash === "#add-opportunity",
  );

  const [singleUrl, setSingleUrl] = useState<string>("");
  const [singleTitle, setSingleTitle] = useState<string>("");
  const [singleError, setSingleError] = useState<string>("");
  const [singleNotice, setSingleNotice] = useState<string>("");
  const [singleBusy, setSingleBusy] = useState<boolean>(false);

  const [bulkText, setBulkText] = useState<string>("");
  const [bulkError, setBulkError] = useState<string>("");
  const [bulkResult, setBulkResult] = useState<BatchResult | null>(null);
  const [bulkBusy, setBulkBusy] = useState<boolean>(false);

  // One bulk selection per row. Move and deletion share this set so there
  // is exactly one visually clear control per opportunity; deletion itself
  // stays deliberately invoked inside the collapsed confirmation below.
  const [selectingMultiple, setSelectingMultiple] = useState(false);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [deleteOpen, setDeleteOpen] = useState<boolean>(false);
  const [deletePhrase, setDeletePhrase] = useState<string>("");
  const [deleteConfirm, setDeleteConfirm] = useState<boolean>(false);
  const [deleteError, setDeleteError] = useState<string>("");
  const [deleteBusy, setDeleteBusy] = useState<boolean>(false);
  const [moveBusy, setMoveBusy] = useState<boolean>(false);
  const [moveNotice, setMoveNotice] = useState<string>("");

  const [actionError, setActionError] = useState<string>("");
  // KAN-16 closure preflight for every Posting entry. A confirmed closure
  // stays in this workspace as a persistent notice with History actions and
  // never navigates away; unknown keeps the manual path with a warning.
  const [checkingId, setCheckingId] = useState<number | null>(null);
  const [closureNotice, setClosureNotice] = useState<string>("");
  const [closedJobId, setClosedJobId] = useState<number | null>(null);
  const [closedJobTitle, setClosedJobTitle] = useState<string>("");
  const [postingWarning, setPostingWarning] = useState<string>("");
  const [manualFallback, setManualFallback] = useState<{ id: number; url: string } | null>(null);

  const reload = useCallback(async () => {
    setLoading(true);
    setLoadError("");
    try {
      const next = await fetchOpportunities();
      setData(next);
      const existing = new Set(next.jobs.map((job) => job.id));
      setSelected((previous) => {
        const valid = new Set<number>();
        for (const id of previous) {
          if (existing.has(id)) valid.add(id);
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

  useEffect(() => {
    const applyPlacementHash = () => {
      // Closure-notice navigation targets: the workspace links here so a
      // retired job is one click away without auto-navigation.
      if (window.location.hash === "#history-closed") {
        setAttention("history");
        setStage("closed");
      } else if (window.location.hash === "#focus") {
        setAttention("focus");
      }
    };
    applyPlacementHash();
    const onHashChange = () => {
      if (window.location.hash === "#add-opportunity") setCreateOpen(true);
      applyPlacementHash();
    };
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);

  const focusCount = data?.focus ?? data?.remaining ?? 0;
  const savedCount = data?.saved ?? 0;
  const historyCount = data?.history ?? 0;

  const visibleJobs = useMemo(() => {
    if (!data) return [];
    const needle = query.trim().toLocaleLowerCase();
    const filtered = data.jobs.filter((job) => {
      const placement = job.attention === "focus" || job.attention === "saved" || job.attention === "history"
        ? job.attention
        : isApplied(job) || isClosed(job)
          ? "history"
          : "saved";
      if (placement !== attention) return false;
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
  }, [data, query, attention, stage, sort]);

  // Selections never leak across views: only jobs in the current visible
  // rows/view/filter stay selected, so hidden jobs are never moved or
  // deleted by accident. One shared set keeps the single per-row control
  // unambiguous for both bulk actions.
  const visibleIds = useMemo(() => new Set(visibleJobs.map((job) => job.id)), [visibleJobs]);

  useEffect(() => {
    setSelected((previous) => {
      if (previous.size === 0) return previous;
      const scoped = new Set<number>();
      for (const id of previous) {
        if (visibleIds.has(id)) scoped.add(id);
      }
      return scoped.size === previous.size ? previous : scoped;
    });
  }, [visibleIds]);

  const toggleSelected = useCallback((id: number, checked: boolean) => {
    setSelected((previous) => {
      const next = new Set(previous);
      if (checked) next.add(id);
      else next.delete(id);
      return next;
    });
  }, []);

  // Deletion authorization must never go stale. The typed DELETE phrase and
  // confirmation checkbox authorize exactly the selection that was reviewed
  // when they were entered, so any selection change clears both. Closing the
  // collapsed confirmation also clears both; reopening starts clean.
  // Filter/view pruning and async reload pruning flow through `selected`,
  // so they clear authorization the same way.
  const selectedKey = useMemo(
    () => [...selected].sort((a, b) => a - b).join(","),
    [selected],
  );

  useEffect(() => {
    setDeletePhrase("");
    setDeleteConfirm(false);
  }, [selectedKey]);

  const jobById = useMemo(
    () => new Map((data?.jobs ?? []).map((job) => [job.id, job] as const)),
    [data],
  );
  const selectedJobs = useMemo(
    () =>
      [...selected]
        .map((id) => jobById.get(id))
        .filter((job): job is OpportunityJob => job !== undefined),
    [selected, jobById],
  );
  // Go (moveJobsAttention) refuses any batch containing an applied/closed
  // job or a job with recorded submissions: the whole batch moves nothing.
  // Disable bulk moves for such selections instead of letting them fail.
  // Closed History jobs without submissions stay deletable below.
  const selectedHasUnmovable = useMemo(
    () => selectedJobs.some((job) => isApplied(job) || isClosed(job)),
    [selectedJobs],
  );

  const handleSingleSave = useCallback(
    async (focus: boolean) => {
      setSingleError("");
      setSingleNotice("");
      setSingleBusy(true);
      try {
        const result = await addOpportunity(singleUrl.trim(), singleTitle.trim(), focus);
        if (result.duplicate) {
          setSingleNotice("Already in your workspace; existing details and placement retained.");
        } else if (focus) {
          setSingleNotice("Saved to Focus. It now counts toward your suggested next opportunity.");
          setSingleUrl("");
          setSingleTitle("");
          setAttention("focus");
        } else {
          setSingleNotice("Saved for Later. Move it to Focus when you are ready to work on it.");
          setSingleUrl("");
          setSingleTitle("");
          setAttention("saved");
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

  const handleSingleSubmit = useCallback(
    async (event: React.FormEvent) => {
      event.preventDefault();
      await handleSingleSave(false);
    },
    [handleSingleSave],
  );

  const handleSingleSaveFocus = useCallback(async () => {
    await handleSingleSave(true);
  }, [handleSingleSave]);

  const handleMoveSelected = useCallback(
    async (next: "focus" | "saved") => {
      setActionError("");
      setMoveNotice("");
      if (selected.size === 0) {
        setActionError("Select at least one job to move.");
        return;
      }
      const count = selected.size;
      setMoveBusy(true);
      try {
        await moveOpportunities([...selected], next);
        setMoveNotice(
          next === "focus"
            ? `Moved ${count} to Focus without changing stages or preparation.`
            : `Moved ${count} to Saved for Later without changing stages or preparation.`,
        );
        setSelected(new Set());
        setAttention(next);
        await reload();
      } catch (error) {
        setActionError(error instanceof Error ? error.message : "Unable to move selected jobs");
      } finally {
        setMoveBusy(false);
      }
    },
    [selected, reload],
  );

  const handleMoveJob = useCallback(
    async (job: OpportunityJob, next: "focus" | "saved") => {
      setActionError("");
      try {
        await moveOpportunities([job.id], next);
        await reload();
      } catch (error) {
        setActionError(error instanceof Error ? error.message : "Unable to move job");
      }
    },
    [reload],
  );

  const handleBulkSubmit = useCallback(
    async (event: React.FormEvent) => {
      event.preventDefault();
      const submitted = bulkText;
      setBulkError("");
      setBulkResult(null);
      setBulkBusy(true);
      try {
        const result = await importOpportunities(submitted);
        setBulkResult(result);
        await reload();
        // Reconcile only on a confirmed result: drop added/duplicate lines,
        // keep failed/unprocessed lines for fixing and retry. Preserve
        // concurrent edits made while the request was in flight.
        const retained = reconcileBulkInput(submitted, result);
        setBulkText((previous) => (previous === submitted ? retained : previous));
      } catch (error) {
        // Unknown outcome (network/server failure): preserve the full input
        // including any concurrent edits; nothing was confirmed saved.
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

  // Desktop Posting: Go availability preflight first, then the Go-owned
  // system-browser open (which re-checks). A 409 closure between the two
  // refreshes the list and persists the closure notice with History actions;
  // success navigates to the workspace. Unknown never blocks.
  const handleOpenPosting = useCallback(async (job: OpportunityJob) => {
    setActionError("");
    setPostingWarning("");
    setManualFallback(null);
    setCheckingId(job.id);
    try {
      const availability = await checkPostingAvailability(job.id);
      if (availability.state === "closed") {
        setClosureNotice(availability.notice !== "" ? availability.notice : availability.detail);
        setClosedJobId(job.id);
        setClosedJobTitle(job.displayTitle);
        await reload();
        return;
      }
      if (availability.state === "unknown" && availability.detail !== "") {
        setPostingWarning(availability.detail);
      }
      try {
        await openPosting(job.id);
      } catch (error) {
        const message = error instanceof Error ? error.message : "Could not open posting";
        await reload();
        if (isPostingClosedBlock(message)) {
          setClosureNotice(message);
          setClosedJobId(job.id);
          setClosedJobTitle(job.displayTitle);
        } else {
          setActionError(message);
        }
        return;
      }
      window.location.href = `/jobs/${job.id}`;
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "Could not open posting");
    } finally {
      setCheckingId(null);
    }
  }, [reload]);

  // Browser-mode Posting: the same Go availability gate guards the manual
  // external open. The tab is pre-opened synchronously in the click gesture
  // so the async gate does not trigger popup blockers; it is closed on
  // closure/failure and navigated noopener-safe on success. A blocked popup
  // surfaces a truthful inline fallback link and never leaks a blank window.
  // Unknown keeps the manual path with a warning.
  const handleExternalOpen = useCallback((job: OpportunityJob) => {
    setActionError("");
    setPostingWarning("");
    setManualFallback(null);
    let pending: Window | null = null;
    try {
      pending = window.open("about:blank", "_blank");
    } catch {
      pending = null;
    }
    if (pending) {
      try {
        pending.opener = null;
      } catch {
        // about:blank is same-origin; ignore hardening failures.
      }
    }
    setCheckingId(job.id);
    void (async () => {
      try {
        const availability = await checkPostingAvailability(job.id);
        if (availability.state === "closed") {
          try {
            if (pending && !pending.closed) pending.close();
          } catch {
            // Ignore close failures; the tab is already gone.
          }
          setClosureNotice(availability.notice !== "" ? availability.notice : availability.detail);
          setClosedJobId(job.id);
          setClosedJobTitle(job.displayTitle);
          await reload();
          return;
        }
        if (availability.state === "unknown" && availability.detail !== "") {
          setPostingWarning(availability.detail);
        }
        if (pending && !pending.closed) {
          pending.location.href = job.url;
        } else {
          setManualFallback({ id: job.id, url: job.url });
          setPostingWarning((previous) =>
            previous !== ""
              ? previous
              : "Your browser blocked the new tab. Use the manual link below to open the posting.",
          );
        }
      } catch (error) {
        try {
          if (pending && !pending.closed) pending.close();
        } catch {
          // Ignore close failures here as well.
        }
        setActionError(error instanceof Error ? error.message : "Could not check the posting.");
      } finally {
        setCheckingId(null);
      }
    })();
  }, [reload]);

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      <AppNavigation active="opportunities" />
      <div className="app-main">
        <AppTopbar />
        <main id="main-content" className="page-container">
          <div className="page-heading">
            <div>
              <h1>Opportunities</h1>
              <p className="page-description">Choose an opportunity to continue.</p>
            </div>
            <div className="heading-actions">
              <a className="text-link" href="/resumes">
                Document library
              </a>
              <a
                className={data?.next ? "button button-secondary" : "button button-primary"}
                href="#add-opportunity"
                onClick={() => setCreateOpen(true)}
              >
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
              {data.total > 0 && (
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
                        setAttention("focus");
                        setStage("all");
                        setQuery("");
                      }}
                    >
                      <strong>{focusCount}</strong> <span>in Focus</span>
                    </button>
                    <button
                      type="button"
                      className="progress-metric"
                      onClick={() => {
                        setAttention("saved");
                        setStage("all");
                        setQuery("");
                      }}
                    >
                      <strong>{savedCount}</strong> <span>saved for later</span>
                    </button>
                    <button
                      type="button"
                      className="progress-metric"
                      onClick={() => {
                        setAttention("history");
                        setStage("all");
                        setQuery("");
                      }}
                    >
                      <strong>{historyCount}</strong> <span>in history</span>
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
                          aria-label="Applications recorded out of Focus plus recorded submissions"
                        >
                          {data.applied} of {data.queueTotal}
                        </progress>
                        <p className="queue-progress-note">
                          {data.applied} of {data.queueTotal} opportunities in Focus plus recorded
                          submissions. Saved for Later and other History never count as outstanding
                          work.
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
                      <div className="dashboard-next-identity">
                        <p className="eyebrow">Suggested next opportunity</p>
                        <h2>{data.next.title}</h2>
                        <p className="dashboard-next-company">{data.next.company}</p>
                      </div>
                      <p className="dashboard-next-reason">{data.next.reason}</p>
                      <a className="button button-primary" href={data.next.href}>
                        {data.next.label} <span aria-hidden="true">→</span>
                      </a>
                    </>
                  ) : data.applied > 0 ? (
                    <>
                      <div className="dashboard-next-identity">
                        <p className="eyebrow">CURRENT QUEUE COMPLETE</p>
                        <h2>Your applications are recorded.</h2>
                        <p className="small-muted">
                          Review your history or save another opportunity to keep going.
                        </p>
                      </div>
                      <a className="button button-secondary" href="/applications">
                        Review applications →
                      </a>
                    </>
                  ) : null}
                </div>
              </section>
              )}

              <details
                className="panel create-panel"
                id="add-opportunity"
                open={createOpen}
                onToggle={(event) => setCreateOpen(event.currentTarget.open)}
              >
                <summary>
                  <span className="create-summary-text">
                    <strong>Add an opportunity</strong>
                  </span>
                  <span className="summary-chevron" aria-hidden="true">
                    ⌄
                  </span>
                </summary>
                <div className="create-body">
                  <p className="small-muted create-helper">
                    Paste a job posting link to save it locally. New postings land in Saved for
                    Later; move one to Focus when you are ready to work on it.
                  </p>
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
                      <button className="button button-secondary" type="submit" disabled={singleBusy}>
                        {singleBusy ? "Saving…" : "Save for Later"}
                      </button>
                      <button
                        className="button button-primary"
                        type="button"
                        disabled={singleBusy}
                        onClick={() => void handleSingleSaveFocus()}
                      >
                        {singleBusy ? "Saving…" : "Save & Focus"}
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
                          New imports land in Saved for Later without changing existing records.
                          After import, YourMove checks supported public job boards for
                          titles, employers, descriptions, and other available posting details.
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
                        <p role="status" className="batch-summary">
                          Saved {bulkResult.added} · Already saved {bulkResult.duplicates} · Needs
                          fixing {bulkResult.invalid}
                        </p>
                        {bulkResult.invalid > 0 && (
                          <p role="alert" className="batch-failures">
                            {bulkResult.invalid}{" "}
                            {bulkResult.invalid === 1 ? "line needs fixing" : "lines need fixing"}.
                            Check the details below, correct the input, and retry.
                          </p>
                        )}
                        {bulkResult.enrichment && bulkResult.enrichment.total > 0 && (
                          <p role="status">
                            {bulkResult.enrichment.total} {bulkResult.enrichment.total === 1 ? "opportunity is" : "opportunities are"} checking public posting details
                            {bulkResult.enrichment.started ? " automatically." : " — another metadata check is already running or paused, so these stay unchecked for now."}{" "}
                            <a href="/enrichment">Posting checks →</a>
                          </p>
                        )}
                        <details className="batch-details">
                          <summary>
                            Details ({bulkResult.items.length}{" "}
                            {bulkResult.items.length === 1 ? "line" : "lines"}) — expand for workspace
                            links
                          </summary>
                          <ul>
                            {bulkResult.items.map((item) => (
                              <li
                                key={`${item.line}-${item.url}`}
                                className={
                                  item.status === "invalid"
                                    ? "batch-item batch-item-invalid"
                                    : item.status === "duplicate"
                                      ? "batch-item batch-item-duplicate"
                                      : "batch-item batch-item-added"
                                }
                              >
                                <span>
                                  Line {item.line} · {item.status}
                                </span>
                                <span className="batch-url">{item.url}</span>
                                <span>{item.detail}</span>
                                {item.jobId > 0 && <a href={`/jobs/${item.jobId}`}>Open workspace →</a>}
                              </li>
                            ))}
                          </ul>
                        </details>
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
                    <div className="attention-tabs" role="group" aria-label="Opportunity views">
                      <button
                        type="button"
                        className={attention === "focus" ? "button button-primary" : "button button-secondary"}
                        aria-pressed={attention === "focus"}
                        onClick={() => setAttention("focus")}
                      >
                        Focus · {focusCount}
                      </button>
                      <button
                        type="button"
                        className={attention === "saved" ? "button button-primary" : "button button-secondary"}
                        aria-pressed={attention === "saved"}
                        onClick={() => setAttention("saved")}
                      >
                        Saved for Later · {savedCount}
                      </button>
                      <button
                        type="button"
                        className={attention === "history" ? "button button-primary" : "button button-secondary"}
                        aria-pressed={attention === "history"}
                        onClick={() => setAttention("history")}
                      >
                        History · {historyCount}
                      </button>
                    </div>
                    <p className="visually-hidden" id="results-status" role="status" aria-live="polite">
                      {visibleJobs.length} opportunities shown
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
                  <button
                    className="button button-secondary list-select-toggle"
                    type="button"
                    onClick={() => {
                      if (selectingMultiple) {
                        setSelected(new Set());
                        setDeleteOpen(false);
                        setDeletePhrase("");
                        setDeleteConfirm(false);
                        setDeleteError("");
                      }
                      setSelectingMultiple(!selectingMultiple);
                    }}
                  >
                    {selectingMultiple ? "Done selecting" : "Select multiple"}
                  </button>
                </div>
                {actionError !== "" && (
                  <p className="job-form-error" role="alert">
                    {actionError}
                  </p>
                )}
                {moveNotice !== "" && <p className="form-notice">{moveNotice}</p>}
                {closureNotice !== "" && (
                  <div className="writing-notice" role="alert" aria-label="Posting closed">
                    <p>
                      {closedJobTitle !== "" && <strong>{closedJobTitle} — </strong>}
                      {closureNotice}
                    </p>
                    <div className="form-actions">
                      <button
                        className="button button-secondary"
                        type="button"
                        onClick={() => {
                          setAttention("history");
                          setStage("closed");
                        }}
                      >
                        View in History → Closed
                      </button>
                      <button
                        className="button button-secondary"
                        type="button"
                        onClick={() => {
                          setAttention("focus");
                        }}
                      >
                        Return to Focus
                      </button>
                      {closedJobId !== null && (
                        <a className="button button-secondary" href={`/jobs/${closedJobId}`}>
                          Open workspace →
                        </a>
                      )}
                    </div>
                  </div>
                )}
                {postingWarning !== "" && closureNotice === "" && checkingId === null && (
                  <p className="writing-notice" role="status">
                    {postingWarning}
                  </p>
                )}
                {manualFallback !== null && closureNotice === "" && (
                  <p className="writing-notice" role="status">
                    Your browser blocked the new tab.{" "}
                    <a href={manualFallback.url} target="_blank" rel="noopener noreferrer">
                      Open the posting manually
                    </a>{" "}
                    — viewing sends no saved material.
                  </p>
                )}
                {selectingMultiple && (
                  <div className="bulk-move-actions">
                    <p className="small-muted bulk-select-hint">
                      Tick Select on each row for bulk move or deletion ({selected.size} selected).
                      Jobs with confirmed submissions cannot be bulk deleted; applied,
                      closed, or submitted jobs stay in History when moving.
                    </p>
                    {selectedHasUnmovable && selected.size > 0 && (
                      <p className="job-form-error" role="alert">
                        Selection includes a History job (applied, closed, or with recorded
                        submissions). It stays in History; uncheck it to move the rest. No
                        move was attempted. Eligible History jobs without submissions can
                        still be deleted below.
                      </p>
                    )}
                    <button
                      className="button button-secondary"
                      type="button"
                      disabled={moveBusy || selected.size === 0 || selectedHasUnmovable}
                      title={
                        selectedHasUnmovable
                          ? "Uncheck History jobs to enable moving"
                          : undefined
                      }
                      onClick={() => void handleMoveSelected("focus")}
                    >
                      {moveBusy ? "Moving…" : `Move selected to Focus (${selected.size})`}
                    </button>
                    <button
                      className="button button-secondary"
                      type="button"
                      disabled={moveBusy || selected.size === 0 || selectedHasUnmovable}
                      title={
                        selectedHasUnmovable
                          ? "Uncheck History jobs to enable moving"
                          : undefined
                      }
                      onClick={() => void handleMoveSelected("saved")}
                    >
                      {moveBusy ? "Moving…" : `Move selected to Saved (${selected.size})`}
                    </button>
                  </div>
                )}
                <form
                  className="bulk-delete-form"
                  aria-label="Delete selected test or unwanted jobs"
                  onSubmit={(event) => void handleDelete(event)}
                >
                  <div className="job-list">
                    {visibleJobs.map((job) => {
                      const deletable = job.submissionCount === 0;
                      const unmovable = isApplied(job) || isClosed(job);
                      return (
                        <article className="job-row" key={job.id}>
                          {selectingMultiple && (
                            <label className="job-select">
                              <input
                                type="checkbox"
                                checked={selected.has(job.id)}
                                // Submitted jobs are never bulk-deleted here; they
                                // are removed individually from their workspace.
                                // Closed History jobs without submissions stay
                                // selectable so they can still be deleted below.
                                disabled={!deletable}
                                title={
                                  !deletable
                                    ? "Submitted job: remove individually from its workspace"
                                    : unmovable
                                      ? "Select for deletion (this History job stays in History when moving)"
                                      : `Select ${job.displayTitle} for bulk move or deletion`
                                }
                                aria-label={`Select ${job.displayTitle} for bulk move or deletion`}
                                onChange={(event) => toggleSelected(job.id, event.target.checked)}
                              />
                              <span aria-hidden="true">Select</span>
                            </label>
                          )}
                          <div className="job-row-body">
                            <h3>
                              <a className="job-title" href={`/jobs/${job.id}`}>
                                {job.displayTitle}
                              </a>
                            </h3>
                            <div className="job-row-meta">
                              <span className="company-label">{job.companyOrHost}</span>
                              <ATSPlatformIcon
                                provider={job.provider}
                                providerLabel={job.providerLabel}
                              />
                              {job.host && job.host.toLowerCase() !== job.companyOrHost.toLowerCase() && (
                                <span className="job-row-detail">{job.host}</span>
                              )}
                              <span className={`status status-${job.stage}`}>{job.stageLabel}</span>
                              <span className="job-row-detail">
                                {job.selectedResumeId > 0 ? "Résumé selected" : "No résumé selected"}
                              </span>
                              {job.priority === "high" && (
                                <span className="priority-flag">High priority</span>
                              )}
                              {job.priority === "low" && (
                                <span className="priority-quiet">Low priority</span>
                              )}
                            </div>
                          </div>
                          <div className="job-row-actions">
                            {job.attention === "focus" && (
                              <button
                                type="button"
                                className="button button-secondary row-primary"
                                onClick={() => void handleMoveJob(job, "saved")}
                              >
                                Save for Later
                              </button>
                            )}
                            {job.attention === "saved" && (
                              <button
                                type="button"
                                className="button button-secondary row-primary"
                                onClick={() => void handleMoveJob(job, "focus")}
                              >
                                Work on this
                              </button>
                            )}
                            {data.desktop ? (
                              <button
                                type="button"
                                className="text-link row-secondary posting-open-button"
                                disabled={checkingId === job.id}
                                onClick={() => void handleOpenPosting(job)}
                              >
                                {checkingId === job.id ? "Checking…" : "Posting ↗"}
                              </button>
                            ) : (
                              <button
                                type="button"
                                className="text-link row-secondary posting-open-button"
                                disabled={checkingId === job.id}
                                onClick={() => handleExternalOpen(job)}
                              >
                                {checkingId === job.id ? "Checking…" : "Posting ↗"}
                              </button>
                            )}
                            <a className="button button-primary row-primary" href={`/jobs/${job.id}`}>
                              Open workspace <span aria-hidden="true">→</span>
                            </a>
                          </div>
                        </article>
                      );
                    })}
                    {visibleJobs.length === 0 && (
                      <div className="empty-state">
                        <h3>{data.total === 0 ? "Save your first opportunity" : "No matching opportunities"}</h3>
                        <p>{data.total === 0 ? "Add a job posting to begin." : "Try a different search or stage."}</p>
                        {data.total > 0 && <button
                          className="button button-secondary"
                          type="button"
                          onClick={() => {
                            setQuery("");
                            setStage("all");
                            setSort("priority");
                          }}
                        >
                          Clear filters
                        </button>}
                      </div>
                    )}
                  </div>
                  {selectingMultiple && data.jobs.length > 0 && (
                    <details
                      className="bulk-remove"
                      open={deleteOpen}
                      onToggle={(event) => {
                        const open = event.currentTarget.open;
                        setDeleteOpen(open);
                        if (!open) {
                          setDeletePhrase("");
                          setDeleteConfirm(false);
                        }
                      }}
                    >
                      <summary>
                        Delete selected jobs ({selected.size} selected){" "}
                        <span className="muted">(test and unwanted records)</span>
                      </summary>
                      <div className="bulk-remove-body">
                        <p className="confirmation-note">
                          Permanent deletion. This removes the jobs ticked with Select above
                          with their saved answers, manual questions, and cover-letter drafts.
                          Reusable Answer Library examples and imported resumes are retained.
                          Jobs with confirmed submissions are excluded here; remove them
                          individually from their workspace. History jobs without submissions
                          can be deleted but cannot be moved. Only jobs shown in the current
                          view stay selected; hidden jobs are never included. Deletion needs
                          all three steps here: open this section, type DELETE, and confirm.
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
                          {deleteBusy ? "Deleting…" : `Delete selected jobs (${selected.size})`}
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
