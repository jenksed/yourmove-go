import { useCallback, useEffect, useState } from "react";
import {
  CareerApiError,
  answerCareerQuestion,
  authorizeCareerInference,
  createCareerSession,
  fetchCareerProviders,
  fetchCareerResumeChoices,
  fetchCareerSession,
  fetchCareerSessions,
  promoteCareerProposal,
  selectCareerLead,
  type CareerProvider,
  type CareerResumeChoice,
  type CareerSession,
  type CareerSessionSummary,
} from "./api";
import { CareerNav } from "./CareerNav";
import { formatDateTime, statusLabel } from "./format";

export function CareerPage(): React.JSX.Element {
  const [sessions, setSessions] = useState<CareerSessionSummary[]>([]);
  const [providers, setProviders] = useState<CareerProvider[]>([]);
  const [resumeChoices, setResumeChoices] = useState<CareerResumeChoice[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [loadError, setLoadError] = useState<string>("");

  const [activeId, setActiveId] = useState<string>("");
  const [session, setSession] = useState<CareerSession | null>(null);
  const [sessionError, setSessionError] = useState<string>("");

  const [notice, setNotice] = useState<string>("");
  const [actionError, setActionError] = useState<string>("");
  const [busy, setBusy] = useState<boolean>(false);

  const [startMode, setStartMode] = useState<"resume" | "excerpt">("resume");
  const [startResumeId, setStartResumeId] = useState<string>("");
  const [startExcerpt, setStartExcerpt] = useState<string>("");
  const [offerExcerptEnd, setOfferExcerptEnd] = useState<boolean>(false);

  const [leadChoice, setLeadChoice] = useState<string>("");
  const [providerId, setProviderId] = useState<string>("");
  const [model, setModel] = useState<string>("");
  const [ackGeneration, setAckGeneration] = useState<boolean>(false);
  const [ackExternal, setAckExternal] = useState<boolean>(false);
  const [answer, setAnswer] = useState<string>("");
  const [promotePath, setPromotePath] = useState<string>("");

  const reloadList = useCallback(async () => {
    setSessions(await fetchCareerSessions());
  }, []);

  const openSession = useCallback(async (id: string) => {
    setSessionError("");
    setActiveId(id);
    setLeadChoice("");
    setAnswer("");
    setPromotePath("");
    try {
      const view = await fetchCareerSession(id);
      setSession(view);
      if (view.selected_lead_id !== "") setLeadChoice(view.selected_lead_id);
    } catch (error) {
      setSession(null);
      setSessionError(error instanceof Error ? error.message : "Unable to load the interview");
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      setLoadError("");
      try {
        const [loadedProviders, loadedSessions, loadedResumes] = await Promise.all([
          fetchCareerProviders(),
          fetchCareerSessions(),
          fetchCareerResumeChoices(),
        ]);
        if (cancelled) return;
        setProviders(loadedProviders);
        setSessions(loadedSessions);
        setResumeChoices(loadedResumes);
        const firstConfigured = loadedProviders.find((provider) => provider.configured);
        const initial = firstConfigured ?? loadedProviders[0];
        if (initial) {
          setProviderId(initial.id);
          setModel(initial.default_model);
        }
        const params = new URLSearchParams(window.location.search);
        const requested = params.get("session");
        if (requested) await openSession(requested);
      } catch (error) {
        if (!cancelled) setLoadError(error instanceof Error ? error.message : "Unable to load career discovery");
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [openSession]);

  const handleProviderChange = useCallback(
    (id: string) => {
      setProviderId(id);
      const provider = providers.find((candidate) => candidate.id === id);
      if (provider) setModel(provider.default_model);
    },
    [providers],
  );

  const runMutation = useCallback(
    async (label: string, work: () => Promise<CareerSession | null>, done?: string): Promise<void> => {
      setActionError("");
      setNotice("");
      setBusy(true);
      try {
        const result = await work();
        if (result) {
          setSession(result);
          setActiveId(result.id);
          if (result.selected_lead_id !== "") setLeadChoice(result.selected_lead_id);
        }
        if (done) setNotice(done);
        await reloadList();
      } catch (error) {
        if (error instanceof CareerApiError && error.code === "complete_no_proposal" && activeId !== "") {
          try {
            await openSession(activeId);
            setNotice(`Interview ${activeId} reached its response limit without a proposal. Start a new interview to continue.`);
            return;
          } catch {
            // Fall through to the original error if the session cannot be reloaded.
          }
        }
        if (error instanceof CareerApiError && error.code === "changed" && activeId !== "") {
          try {
            await openSession(activeId);
            setNotice("The interview changed elsewhere. Reloaded the current state; review it before continuing.");
            return;
          } catch {
            // Fall through to the original error.
          }
        }
        setActionError(error instanceof Error ? `${label}: ${error.message}` : `${label} failed`);
      } finally {
        setBusy(false);
      }
    },
    [activeId, openSession, reloadList],
  );

  const handleStart = useCallback(
    (excerptEnd?: number) => {
      setOfferExcerptEnd(false);
      void (async () => {
        setActionError("");
        setNotice("");
        setBusy(true);
        try {
          let created: CareerSession;
          if (startMode === "resume") {
            const resumeId = Number.parseInt(startResumeId, 10);
            if (!Number.isInteger(resumeId) || resumeId <= 0) {
              setActionError("Start: choose a resume from your library.");
              return;
            }
            created = await createCareerSession(
              excerptEnd ? { resume_id: resumeId, excerpt_end: excerptEnd } : { resume_id: resumeId },
            );
          } else {
            if (startExcerpt.trim() === "") {
              setActionError("Start: paste the resume excerpt to investigate.");
              return;
            }
            created = await createCareerSession({ excerpt: startExcerpt });
          }
          setSession(created);
          setActiveId(created.id);
          setAnswer("");
          setLeadChoice("");
          setPromotePath("");
          setNotice("Interview started. No external request has been made yet.");
          await reloadList();
        } catch (error) {
          if (error instanceof CareerApiError && error.code === "excerpt_required") {
            setOfferExcerptEnd(true);
          }
          setActionError(error instanceof Error ? `Start: ${error.message}` : "Start failed");
        } finally {
          setBusy(false);
        }
      })();
    },
    [reloadList, startExcerpt, startMode, startResumeId],
  );

  const handleAuthorize = useCallback(() => {
    if (!session || session.next.kind === "none") return;
    if (providerId === "" || model.trim() === "") {
      setActionError("Authorize: choose a provider and model.");
      return;
    }
    if (!ackGeneration || !ackExternal) {
      setActionError("Authorize: confirm both statements before the external request.");
      return;
    }
    const inputHash = session.next.input_hash;
    void runMutation(
      "Authorize",
      () => authorizeCareerInference(session.id, { provider: providerId, model: model.trim(), input_hash: inputHash }),
      session.next.kind === "leads"
        ? "Leads found. Choose one investigation to continue."
        : "Response received. Review the question or proposal.",
    );
  }, [ackExternal, ackGeneration, model, providerId, runMutation, session]);

  const handleSelect = useCallback(() => {
    if (!session || leadChoice === "") return;
    void runMutation("Select", () => selectCareerLead(session.id, leadChoice, session.revision), "Lead selected.");
  }, [leadChoice, runMutation, session]);

  const handleAnswer = useCallback(() => {
    if (!session) return;
    if (answer.trim() === "") {
      setActionError("Answer: write your response before saving.");
      return;
    }
    const id = session.id;
    const revision = session.revision;
    const text = answer;
    void runMutation("Answer", () => answerCareerQuestion(id, text, revision), "Answer saved.").then(() =>
      setAnswer(""),
    );
  }, [answer, runMutation, session]);

  const handlePromote = useCallback(() => {
    if (!session || !session.proposal) return;
    void (async () => {
      setActionError("");
      setNotice("");
      setBusy(true);
      try {
        const result = await promoteCareerProposal(session.id, session.revision);
        setPromotePath(result.path);
        setNotice(result.notice);
        await openSession(session.id);
        await reloadList();
      } catch (error) {
        setActionError(error instanceof Error ? `Save draft: ${error.message}` : "Save draft failed");
      } finally {
        setBusy(false);
      }
    })();
  }, [openSession, reloadList, session]);

  const activeProvider = providers.find((provider) => provider.id === providerId);
  const selectableResumes = resumeChoices.filter((resume) => !resume.archived);
  const isCompleteNoProposal = session !== null && session.responses.length >= 6 && session.proposal === null;

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      <CareerNav active="discovery" />
      <div className="app-main">
        <main id="main-content" className="page-container">
          <div className="page-heading">
            <div>
              <h1>Career discovery</h1>
              <p className="page-description">
                Turn resume material into interview-backed Career Evidence drafts. Every external request needs your
                explicit authorization, and nothing is approved until you review it.
              </p>
            </div>
            <div className="heading-actions">
              <a className="button button-secondary" href="/career-evidence">
                Open Career Evidence
              </a>
              <span className="summary-count">
                {sessions.length} {sessions.length === 1 ? "interview" : "interviews"}
              </span>
            </div>
          </div>

          {loading && (
            <section className="panel" aria-label="Loading">
              <p className="section-subtitle">Loading interviews…</p>
            </section>
          )}
          {loadError !== "" && (
            <section className="panel" aria-label="Loading error">
              <p className="job-form-error" role="alert">
                {loadError}
              </p>
              <button
                className="button button-secondary"
                type="button"
                onClick={() => window.location.reload()}
                disabled={busy}
              >
                Reload
              </button>
            </section>
          )}
          {notice !== "" && (
            <section className="panel career-notice" role="status">
              <p>{notice}</p>
            </section>
          )}
          {actionError !== "" && (
            <p className="job-form-error" role="alert">
              {actionError}
            </p>
          )}

          {!loading && loadError === "" && (
            <>
              <section className="panel" aria-labelledby="start-title">
                <div className="career-section-heading">
                  <h2 id="start-title">Start an interview</h2>
                  <p className="section-subtitle">
                    From a saved resume or a pasted excerpt. Starting saves locally only; lead finding waits for your
                    authorization.
                  </p>
                </div>
                <div className="career-start-grid">
                  <div className="field">
                    <label htmlFor="career-start-mode">Source</label>
                    <select
                      id="career-start-mode"
                      value={startMode}
                      onChange={(event) => setStartMode(event.target.value === "excerpt" ? "excerpt" : "resume")}
                      disabled={busy}
                    >
                      <option value="resume">Saved resume</option>
                      <option value="excerpt">Pasted excerpt</option>
                    </select>
                  </div>
                  {startMode === "resume" ? (
                    <div className="field">
                      <label htmlFor="career-start-resume">Resume</label>
                      <select
                        id="career-start-resume"
                        value={startResumeId}
                        onChange={(event) => setStartResumeId(event.target.value)}
                        disabled={busy}
                      >
                        <option value="">Choose a resume…</option>
                        {selectableResumes.map((resume) => (
                          <option key={resume.id} value={String(resume.id)}>
                            {resume.name}
                          </option>
                        ))}
                      </select>
                    </div>
                  ) : (
                    <div className="field field-full">
                      <label htmlFor="career-start-excerpt">Excerpt (up to 8000 bytes)</label>
                      <textarea
                        id="career-start-excerpt"
                        rows={5}
                        value={startExcerpt}
                        onChange={(event) => setStartExcerpt(event.target.value)}
                        placeholder="Paste the resume passage to investigate."
                        disabled={busy}
                      />
                    </div>
                  )}
                </div>
                <div className="form-actions">
                  <button
                    className="button button-primary"
                    type="button"
                    onClick={() => handleStart()}
                    disabled={busy}
                  >
                    Start interview
                  </button>
                  {offerExcerptEnd && startMode === "resume" && (
                    <button
                      className="button button-secondary"
                      type="button"
                      onClick={() => handleStart(8000)}
                      disabled={busy}
                    >
                      Use first 8000 bytes instead
                    </button>
                  )}
                </div>
              </section>

              {session && (
                <section className="panel" aria-labelledby="session-title">
                  <div className="career-section-heading">
                    <h2 id="session-title">
                      {statusLabel(session.status)} · revision {session.revision}
                    </h2>
                    <p className="section-subtitle">
                      {session.resume
                        ? session.resume.removed
                          ? `Resume removed after import · excerpt kept · updated ${formatDateTime(session.updated_at)}`
                          : `${session.resume.name} · updated ${formatDateTime(session.updated_at)}`
                        : `Pasted excerpt · updated ${formatDateTime(session.updated_at)}`}
                    </p>
                  </div>

                  {session.failure && (
                    <p className="job-form-error" role="alert">
                      {session.failure.code}: not recorded. Fix the provider if needed, then authorize the same step
                      again — no silent retry happened.
                    </p>
                  )}
                  {isCompleteNoProposal && (
                    <p className="career-notice" role="status">
                      Interview {session.id} reached its response limit without a proposal. Start a new interview to continue.
                    </p>
                  )}
                  {session.status === "promoted" && session.promoted_evidence_id !== null && (
                    <p className="career-promoted" role="status">
                      Draft saved as{" "}
                      <a className="text-link" href={`/career-evidence/${session.promoted_evidence_id}`}>
                        Career Evidence #{session.promoted_evidence_id}
                      </a>
                      . Correct it there and complete your explicit review.
                    </p>
                  )}

                  <details className="career-details">
                    <summary>Interview source (redacted)</summary>
                    <p className="section-subtitle">
                      Excerpt {session.excerpt_bytes} bytes · sha256 {session.excerpt_hash}. The full text stays on
                      this device and is never displayed here; what leaves the app on authorization is summarized
                      below.
                    </p>
                  </details>

                  {session.leads.length > 0 && session.responses.length === 0 && session.status !== "promoted" && (
                    <div className="career-block">
                      <h3>Investigation leads</h3>
                      <p className="section-subtitle">
                        Model interpretations, not established facts. Choose one investigation before answering.
                      </p>
                      <div className="career-leads" role="radiogroup" aria-label="Investigation leads">
                        {session.leads.map((lead) => (
                          <label key={lead.id} className="career-lead">
                            <input
                              type="radio"
                              name="career-lead"
                              value={lead.id}
                              checked={leadChoice === lead.id || session.selected_lead_id === lead.id}
                              onChange={() => setLeadChoice(lead.id)}
                              disabled={busy || session.selected_lead_id !== ""}
                            />
                            <span>
                              <strong>{lead.title}</strong>
                              <span className="career-lead-why">{lead.why}</span>
                            </span>
                          </label>
                        ))}
                      </div>
                      {session.selected_lead_id === "" && (
                        <div className="form-actions">
                          <button
                            className="button button-primary"
                            type="button"
                            onClick={handleSelect}
                            disabled={busy || leadChoice === ""}
                          >
                            Select this lead
                          </button>
                        </div>
                      )}
                    </div>
                  )}

                  {session.next.kind !== "none" && !isCompleteNoProposal && (
                    <div className="career-block">
                      <h3>
                        {session.next.kind === "leads" ? "Authorize lead finding" : "Authorize the next question"}
                      </h3>
                      <p className="section-subtitle">
                        {activeProvider ? activeProvider.execution : "Choose a provider."} No request runs without both
                        confirmations below.
                      </p>
                      <div className="career-start-grid">
                        <div className="field">
                          <label htmlFor="career-provider">Provider</label>
                          <select
                            id="career-provider"
                            value={providerId}
                            onChange={(event) => handleProviderChange(event.target.value)}
                            disabled={busy}
                          >
                            {providers.map((provider) => (
                              <option key={provider.id} value={provider.id}>
                                {provider.id}
                                {provider.configured ? "" : " (not configured)"}
                              </option>
                            ))}
                          </select>
                        </div>
                        <div className="field">
                          <label htmlFor="career-model">Model</label>
                          <input
                            id="career-model"
                            value={model}
                            onChange={(event) => setModel(event.target.value)}
                            disabled={busy}
                          />
                        </div>
                      </div>
                      <details className="career-details">
                        <summary>
                          What will be transmitted ({session.next.bytes} bytes · {session.next.policy_name})
                        </summary>
                        <p className="section-subtitle">
                          {session.next.kind === "leads"
                            ? `Resume excerpt (${session.excerpt_bytes} bytes) for lead finding.`
                            : `Resume excerpt (${session.excerpt_bytes} bytes), ${session.responses.length} recorded ${session.responses.length === 1 ? "response" : "responses"}${session.selected_lead_id !== "" ? `, focus ${session.selected_lead_id}` : ""} for the next interview turn.`}{" "}
                          Exact prompts and source text are never displayed; the byte count and policy name identify
                          this specific request.
                        </p>
                      </details>
                      <div className="career-checks">
                        <label>
                          <input
                            type="checkbox"
                            checked={ackGeneration}
                            onChange={(event) => setAckGeneration(event.target.checked)}
                            disabled={busy}
                          />{" "}
                          I authorize this specific inference.
                        </label>
                        <label>
                          <input
                            type="checkbox"
                            checked={ackExternal}
                            onChange={(event) => setAckExternal(event.target.checked)}
                            disabled={busy}
                          />{" "}
                          I understand the summarized material above leaves this app for external inference.
                        </label>
                      </div>
                      <div className="form-actions">
                        <button
                          className="button button-primary"
                          type="button"
                          onClick={handleAuthorize}
                          disabled={busy || !ackGeneration || !ackExternal}
                        >
                          Authorize and run
                        </button>
                      </div>
                    </div>
                  )}

                  {session.question !== "" && session.status !== "promoted" && !isCompleteNoProposal && (
                    <div className="career-block">
                      <h3>Pending question</h3>
                      <p className="career-question">{session.question}</p>
                      <div className="field">
                        <label htmlFor="career-answer">Your answer (saved before any further inference)</label>
                        <textarea
                          id="career-answer"
                          rows={4}
                          value={answer}
                          onChange={(event) => setAnswer(event.target.value)}
                          placeholder="Answer in your own words. Unknowns stay unknown."
                          disabled={busy}
                        />
                      </div>
                      <div className="form-actions">
                        <button
                          className="button button-primary"
                          type="button"
                          onClick={handleAnswer}
                          disabled={busy || answer.trim() === ""}
                        >
                          Save answer
                        </button>
                      </div>
                    </div>
                  )}

                  {session.responses.length > 0 && (
                    <div className="career-block">
                      <h3>Recorded responses ({session.responses.length})</h3>
                      <ol className="career-transcript">
                        {session.responses.map((response) => (
                          <li key={response.source.id}>
                            <p className="career-transcript-question">{response.question}</p>
                            <p>
                              <span className="career-source-id">[{response.source.id}]</span> {response.source.text}
                            </p>
                          </li>
                        ))}
                      </ol>
                    </div>
                  )}

                  {session.proposal && (
                    <div className="career-block">
                      <h3>Proposal under inspection</h3>
                      <p className="section-subtitle">
                        A suggestion grounded in your recorded answers. Inspect every citation, then save it as a draft
                        you will correct — nothing here is approved.
                      </p>
                      <dl className="career-proposal">
                        <dt>Title</dt>
                        <dd>{session.proposal.title}</dd>
                        <dt>Experience</dt>
                        <dd>{session.proposal.experience}</dd>
                        <dt>Claim</dt>
                        <dd>{session.proposal.claim}</dd>
                      </dl>
                      <h4>Citations</h4>
                      <ul className="career-citations">
                        {session.proposal.supports.map((support, index) => (
                          <li key={`${support.source_id}-${index}`}>
                            <span className="career-source-id">[{support.source_id}]</span> “{support.quote}”
                          </li>
                        ))}
                      </ul>
                      <h4>Unresolved</h4>
                      <ul className="career-unresolved">
                        {session.proposal.unresolved.map((item, index) => (
                          <li key={index}>{item}</li>
                        ))}
                      </ul>
                      {session.status !== "promoted" && (
                        <div className="form-actions">
                          <button
                            className="button button-primary"
                            type="button"
                            onClick={handlePromote}
                            disabled={busy}
                          >
                            Save as Career Evidence draft
                          </button>
                          {promotePath !== "" && (
                            <a className="button button-secondary" href={promotePath}>
                              Open the draft
                            </a>
                          )}
                        </div>
                      )}
                    </div>
                  )}
                </section>
              )}
              {sessionError !== "" && (
                <p className="job-form-error" role="alert">
                  {sessionError}
                </p>
              )}

              <section className="panel" aria-labelledby="sessions-title">
                <div className="career-section-heading">
                  <h2 id="sessions-title">Previous interviews</h2>
                  <p className="section-subtitle">
                    Persisted locally across quit and relaunch. Reopen one to continue where you left off.
                  </p>
                </div>
                {sessions.length === 0 ? (
                  <p className="section-subtitle">No interviews yet.</p>
                ) : (
                  <ul className="career-session-list">
                    {sessions.map((summary) => (
                      <li key={summary.id} className={summary.id === activeId ? "is-active" : ""}>
                        <div>
                          <strong>{statusLabel(summary.status)}</strong>
                          <span className="career-session-meta">
                            {summary.resume_name !== ""
                              ? summary.resume_removed
                                ? "removed resume"
                                : summary.resume_name
                              : "pasted excerpt"}{" "}
                            · {summary.responses_count}{" "}
                            {summary.responses_count === 1 ? "response" : "responses"}
                            {summary.has_proposal ? " · proposal ready" : ""}
                            {summary.failure_code !== "" ? ` · failed (${summary.failure_code})` : ""} · updated{" "}
                            {formatDateTime(summary.updated_at)}
                          </span>
                        </div>
                        <button
                          className="button button-secondary"
                          type="button"
                          onClick={() => void openSession(summary.id)}
                          disabled={busy || summary.id === activeId}
                        >
                          {summary.id === activeId ? "Current" : "Resume"}
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
              </section>
            </>
          )}
        </main>
      </div>
    </div>
  );
}
