// YM-004B application answers inside the React Application Workspace.
// Reads the Go-owned question inventory, saves through Go-owned validation,
// and surfaces existing Career Evidence, saved writing, and grounded AI
// generation as suggestions. Saved text is never presented as transferred.

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  aiAdoptCheck,
  aiCancel,
  aiGenerate,
  aiReview,
  fetchAssistance,
  fetchQuestions,
  saveAnswer,
  type AssistanceSnapshot,
  type AiTicket,
  type QuestionsSnapshot,
  type WritingQuestion,
} from "./writingApi";

function questionId(q: WritingQuestion): string {
  return `${q.isEarly ? "early" : "live"}:${q.key}`;
}

function QuestionEditor({
  question,
  value,
  onChange,
  disabled,
}: {
  question: WritingQuestion;
  value: string;
  onChange: (next: string) => void;
  disabled: boolean;
}): React.JSX.Element {
  const kind = question.kind.toLowerCase();
  if ((kind === "select" || kind === "radio" || kind === "multi_value_single_select") && question.options && question.options.length > 0) {
    return (
      <div className="field">
        <label htmlFor="writing-answer">Your answer — choose an observed option</label>
        <select id="writing-answer" disabled={disabled} value={value} onChange={(event) => onChange(event.target.value)}>
          <option value="">No answer yet</option>
          {question.options.map((option) => (
            <option key={option} value={option}>
              {option}
            </option>
          ))}
        </select>
      </div>
    );
  }
  if (kind === "checkbox") {
    return (
      <div className="field">
        <label htmlFor="writing-answer">Your answer</label>
        <select id="writing-answer" disabled={disabled} value={value} onChange={(event) => onChange(event.target.value)}>
          <option value="">No answer yet</option>
          <option value="true">Yes</option>
          <option value="false">No</option>
        </select>
      </div>
    );
  }
  const rows = kind === "textarea" ? 8 : 4;
  return (
    <div className="field field-full">
      <label htmlFor="writing-answer">Your answer</label>
      <textarea id="writing-answer" rows={rows} readOnly={disabled} value={value} onChange={(event) => onChange(event.target.value)} />
    </div>
  );
}

export function WritingSection({
  jobId,
  destination,
  legacyHref,
  onMaterialSaved,
  readOnly,
}: {
  jobId: number;
  destination: string;
  legacyHref: string;
  onMaterialSaved: () => void;
  readOnly: boolean;
}): React.JSX.Element {
  const [snapshot, setSnapshot] = useState<QuestionsSnapshot | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [loadError, setLoadError] = useState<string>("");
  const [selectedId, setSelectedId] = useState<string>("");
  const [draft, setDraft] = useState<string>("");
  const [dirty, setDirty] = useState<boolean>(false);
  const [saveError, setSaveError] = useState<string>("");
  const [saveNotice, setSaveNotice] = useState<string>("");
  const [saving, setSaving] = useState<boolean>(false);
  const [reusable, setReusable] = useState<boolean>(false);
  const [assistance, setAssistance] = useState<AssistanceSnapshot | null>(null);
  const [assistBusy, setAssistBusy] = useState<boolean>(false);
  const [assistError, setAssistError] = useState<string>("");
  const [assistOpen, setAssistOpen] = useState<boolean>(false);
  const [aiEvidence, setAiEvidence] = useState<string[]>([]);
  const [aiWriting, setAiWriting] = useState<string>("");
  const [aiAppResume, setAiAppResume] = useState<boolean>(false);
  const [aiMasterResume, setAiMasterResume] = useState<boolean>(false);
  const [aiJobContext, setAiJobContext] = useState<boolean>(false);
  const [aiProvider, setAiProvider] = useState<string>("");
  const [aiModel, setAiModel] = useState<string>("");
  const [confirmExternal, setConfirmExternal] = useState<boolean>(false);
  const [ticket, setTicket] = useState<AiTicket | null>(null);
  const [aiBusy, setAiBusy] = useState<boolean>(false);
  const [aiError, setAiError] = useState<string>("");

  const reload = useCallback(async () => {
    setLoading(true);
    setLoadError("");
    try {
      const next = await fetchQuestions(jobId);
      setSnapshot(next);
      setAssistance(null);
      setAssistOpen(false);
      setTicket(null);
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : "Unable to load questions");
    } finally {
      setLoading(false);
    }
  }, [jobId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const allQuestions = useMemo(() => {
    if (!snapshot) return [];
    return [...snapshot.live, ...snapshot.early];
  }, [snapshot]);

  const selected: WritingQuestion | null = useMemo(() => {
    return allQuestions.find((q) => questionId(q) === selectedId) ?? null;
  }, [allQuestions, selectedId]);

  useEffect(() => {
    if (selected) {
      setDraft(selected.value);
      setDirty(false);
      setSaveError("");
      setSaveNotice("");
      setReusable(false);
      setAssistance(null);
      setAssistOpen(false);
      setTicket(null);
      setAiError("");
      setAiEvidence([]);
      setAiWriting("");
      setAiAppResume(false);
      setAiMasterResume(false);
      setAiJobContext(false);
    }
  }, [selectedId]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!dirty) return;
    const guard = (event: BeforeUnloadEvent) => {
      event.preventDefault();
    };
    window.addEventListener("beforeunload", guard);
    return () => window.removeEventListener("beforeunload", guard);
  }, [dirty]);

  const pickQuestion = useCallback(
    (next: WritingQuestion) => {
      const id = questionId(next);
      if (id === selectedId) return;
      if (dirty) {
        setSaveError("Save or discard your answer before opening another question.");
        return;
      }
      setSelectedId(id);
    },
    [dirty, selectedId],
  );

  const handleSave = useCallback(async () => {
    if (!selected || !snapshot?.editingAllowed || readOnly) return;
    setSaveError("");
    setSaveNotice("");
    setSaving(true);
    try {
      const result = await saveAnswer(jobId, {
        question_key: selected.key,
        revision: selected.revision,
        destination,
        answer: draft,
        reusable,
        reusable_revision: selected.reusableRevision,
        early: selected.isEarly,
        discovery_source: selected.discoverySource,
        discovery_fingerprint: selected.discoveryFingerprint,
        source_question_id: selected.sourceQuestionId,
      });
      setDirty(false);
      setSaveNotice("Saved locally. It has not been filled on the employer website.");
      onMaterialSaved();
      await reload();
      void result;
    } catch (error) {
      setSaveError(error instanceof Error ? error.message : "Unable to save the answer");
    } finally {
      setSaving(false);
    }
  }, [selected, snapshot, jobId, destination, draft, reusable, reload, onMaterialSaved, readOnly]);

  const openAssistance = useCallback(async () => {
    if (!selected) return;
    setAssistBusy(true);
    setAssistError("");
    try {
      const next = await fetchAssistance(jobId, {
        question_key: selected.key,
        revision: selected.revision,
        destination,
        early: selected.isEarly,
        discovery_source: selected.discoverySource,
        discovery_fingerprint: selected.discoveryFingerprint,
        source_question_id: selected.sourceQuestionId,
      });
      setAssistance(next);
      setAssistOpen(true);
      setAiProvider(next.defaultProvider);
      setAiModel(next.defaultModel);
    } catch (error) {
      setAssistError(error instanceof Error ? error.message : "Unable to load suggestions");
    } finally {
      setAssistBusy(false);
    }
  }, [selected, jobId, destination]);

  const handleReview = useCallback(async () => {
    if (!selected) return;
    setAiBusy(true);
    setAiError("");
    try {
      const reviewed = await aiReview(jobId, {
        question_key: selected.key,
        revision: String(selected.revision),
        destination,
        provider: aiProvider,
        model: aiModel,
        career_evidence: aiEvidence,
        library_entry: aiWriting,
        application_resume: aiAppResume && assistance?.sources.ApplicationResume ? assistance.sources.ApplicationResume.Identity : "",
        master_resume: aiMasterResume && assistance?.sources.MasterResume ? assistance.sources.MasterResume.Identity : "",
        include_job_context: aiJobContext ? "yes" : "",
        early: selected.isEarly ? "yes" : "",
        discovery_source: selected.discoverySource,
        discovery_fingerprint: selected.discoveryFingerprint,
        source_question_id: selected.sourceQuestionId,
      });
      setTicket(reviewed);
    } catch (error) {
      setAiError(error instanceof Error ? error.message : "Unable to prepare generation");
    } finally {
      setAiBusy(false);
    }
  }, [selected, jobId, destination, aiProvider, aiModel, aiEvidence, aiWriting, aiAppResume, aiMasterResume, aiJobContext, assistance]);

  const handleGenerate = useCallback(async () => {
    if (!ticket) return;
    setAiBusy(true);
    setAiError("");
    try {
      const result = await aiGenerate(jobId, ticket.ticket);
      setTicket(result);
    } catch (error) {
      setAiError(error instanceof Error ? error.message : "Generation failed. Your existing writing is unchanged.");
    } finally {
      setAiBusy(false);
    }
  }, [ticket, jobId]);

  const handleCancelTicket = useCallback(async () => {
    if (!ticket) return;
    try {
      await aiCancel(jobId, ticket.ticket);
    } catch {
      // Cancellation is best-effort; the ticket expires server-side.
    }
    setTicket(null);
  }, [ticket, jobId]);

  const handleUseSuggestion = useCallback(async () => {
    if (!ticket?.answer || !selected) return;
    setAiError("");
    try {
      await aiAdoptCheck(jobId, ticket.ticket, selected.key, String(selected.revision));
      setDraft(ticket.answer);
      setDirty(true);
      setSaveNotice("Suggestion placed in your editor. Review it, then save to keep it.");
    } catch (error) {
      setAiError(
        error instanceof Error
          ? `${error.message} The question or sources changed; review the current material before copying text manually.`
          : "The preview is stale; review the current material before copying text manually.",
      );
    }
  }, [ticket, selected, jobId]);

  if (loading) return <p role="status">Loading application questions…</p>;
  if (loadError !== "") {
    return (
      <section className="panel" aria-label="Questions loading error">
        <p>{loadError}</p>
        <button className="button button-secondary" type="button" onClick={() => void reload()}>
          Retry
        </button>
      </section>
    );
  }
  if (!snapshot) return <></>;

  const needsAnswer = allQuestions.filter((q) => q.editable && q.value === "");
  const prepared = allQuestions.filter((q) => q.editable && q.value !== "");

  return (
    <section className="panel" aria-labelledby="writing-title" id="writing">
      <div className="section-heading">
        <div>
          <p className="eyebrow">APPLICATION ANSWERS</p>
          <h2 id="writing-title">Answer the employer's questions</h2>
          <p className="section-subtitle">
            {snapshot.available
              ? snapshot.activeContext === "live"
                ? `${snapshot.source}. Your text saves locally in YourMove; nothing is filled on the employer site from here.`
                : `${snapshot.source}. These definitions are published by the employer but not yet verified on the live form; save locally and reconcile later.` + " Nothing is filled on the employer site from here."
              : allQuestions.length > 0
                ? "These questions came from a previous inspection. Review the current employer form before transferring any answers."
                : "Questions have not been discovered for this posting yet. Saved writing below is preserved."}
          </p>
        </div>
        <div className="heading-actions">
          <button className="button button-secondary" type="button" disabled={dirty} onClick={() => void reload()}>
            Refresh questions
          </button>
          <a className="button button-secondary" href={`${legacyHref}#prepare`}>
            Employer transfer review
          </a>
        </div>
      </div>

      {dirty && <p className="small-muted">Save your answer before refreshing the question list.</p>}

      {snapshot.partial && (
        <p className="writing-notice" role="note">
          The latest inspection is partial; later or conditional questions may not be represented.
        </p>
      )}
      {(snapshot.returnNotice !== "" || snapshot.earlyNotice !== "") && (
        <p className="writing-notice" role="note">
          {[snapshot.returnNotice, snapshot.earlyNotice].filter((part) => part !== "").join(" ")}
        </p>
      )}

      {(!snapshot.editingAllowed || readOnly) && (
        <p className="writing-notice" role="note">
          This application is no longer active. Answers are read-only; history below is preserved.
        </p>
      )}

      {!snapshot.available && allQuestions.length === 0 && snapshot.previous.length === 0 && (
        <div className="empty-state">
          <h3>No questions discovered yet</h3>
          <p>
            Run an inspection from the <a href={`/jobs/${jobId}/inspection`}>employer browser details</a> or the legacy
            Prepare section, then return here to answer.
          </p>
        </div>
      )}

      {allQuestions.length > 0 && (
        <div className="writing-layout">
          <ul className="writing-question-list" aria-label="Application questions">
            {needsAnswer.map((q) => (
              <li key={questionId(q)}>
                <button
                  type="button"
                  className={selectedId === questionId(q) ? "writing-question is-selected" : "writing-question"}
                  onClick={() => pickQuestion(q)}
                >
                  <strong>{q.label}</strong>
                  <small>
                    {q.isEarly ? "Published definition" : "Live form"} · Needs your answer
                    {q.required ? " · Required" : ""}
                  </small>
                </button>
              </li>
            ))}
            {prepared.map((q) => (
              <li key={questionId(q)}>
                <button
                  type="button"
                  className={selectedId === questionId(q) ? "writing-question is-selected" : "writing-question"}
                  onClick={() => pickQuestion(q)}
                >
                  <strong>{q.label}</strong>
                  <small>
                    {q.isEarly ? "Published definition" : "Live form"} · Prepared locally
                    {q.reconciled ? " · Reconciled" : ""}
                  </small>
                </button>
              </li>
            ))}
            {allQuestions
              .filter((q) => !q.editable)
              .map((q) => (
                <li key={questionId(q)}>
                  <button
                    type="button"
                    className={selectedId === questionId(q) ? "writing-question is-selected" : "writing-question"}
                    onClick={() => pickQuestion(q)}
                  >
                    <strong>{q.label}</strong>
                    <small>Manual or employer-only · read-only</small>
                  </button>
                </li>
              ))}
          </ul>

          <div className="writing-editor" aria-live="polite">
            {selected ? (
              <>
                <h3>{selected.label}</h3>
                <p className="small-muted">
                  {selected.isEarly ? `Published definition (${selected.discoverySource})` : selected.localStatus}
                  {selected.revision > 0 ? ` · Saved revision ${selected.revision}` : " · Not saved yet"}
                  {selected.required ? " · Required by employer" : " · Optional"}
                </p>
                {selected.source !== "" && !selected.isEarly && (
                  <p className="small-muted">Source: {selected.source}</p>
                )}
                {selected.isEarly && selected.reconciliationStatus !== "" && (
                  <p className="small-muted">
                    Reconciliation: {selected.reconciliationStatus}
                    {selected.reconciliationDetail !== "" ? ` — ${selected.reconciliationDetail}` : ""}
                  </p>
                )}
                {!selected.editable && (
                  <p className="writing-notice" role="note">
                    {selected.manualReason !== "" ? selected.manualReason : "This question requires manual completion in Chromium."}
                  </p>
                )}
                <QuestionEditor
                  question={selected}
                  value={draft}
                  disabled={!selected.editable || !snapshot.editingAllowed || readOnly}
                  onChange={(next) => {
                    setDraft(next);
                    setDirty(next !== selected.value);
                  }}
                />
                {selected.libraryEligible && snapshot.editingAllowed && !readOnly && (
                  <label className="confirm-row">
                    <input type="checkbox" checked={reusable} onChange={(event) => setReusable(event.target.checked)} />
                    <span>Offer as reusable phrasing for this employer when saved</span>
                  </label>
                )}
                {saveError !== "" && (
                  <p className="job-form-error" role="alert">
                    {saveError}
                  </p>
                )}
                {saveNotice !== "" && (
                  <p className="form-notice" role="status">
                    {saveNotice}
                  </p>
                )}
                <div className="form-actions">
                  {dirty && <button className="button button-secondary" type="button" onClick={() => { setDraft(selected.value); setDirty(false); setSaveError(""); }}>Discard edits</button>}
                  <button
                    className="button button-primary"
                    type="button"
                    disabled={saving || !dirty || !selected.editable || (!snapshot.editingAllowed || readOnly)}
                    onClick={() => void handleSave()}
                  >
                    {saving ? "Saving…" : "Save answer locally"}
                  </button>
                  {selected.helpEligible && snapshot.editingAllowed && !readOnly && (
                    <button
                      className="button button-secondary"
                      type="button"
                      disabled={assistBusy}
                      onClick={() => void openAssistance()}
                    >
                      {assistBusy ? "Loading…" : "Use existing experience"}
                    </button>
                  )}
                </div>
                {dirty && <p className="small-muted">Unsaved edits — they stay on this screen until you save.</p>}
                {assistError !== "" && (
                  <p className="job-form-error" role="alert">
                    {assistError}
                  </p>
                )}

                {assistOpen && assistance && (
                  <div className="writing-assist" aria-label="Experience and generation">
                    <h4>Your relevant experience</h4>
                    {assistance.libraryEmpty && assistance.evidenceEmpty ? (
                      <p className="small-muted">
                        No reviewed Career Evidence or saved writing is available yet. Write directly, or add
                        experience in Career Evidence first.
                      </p>
                    ) : (
                      <>
                        {assistance.question.evidenceSuggestions.length > 0 && (
                          <div>
                            <h5>Reviewed Career Evidence</h5>
                            <ul className="writing-suggestions">
                              {assistance.question.evidenceSuggestions.map((s) => (
                                <li key={s.identity}>
                                  <strong>{s.title}</strong>
                                  <small>
                                    {s.outcomeState === "recorded" && s.outcome !== ""
                                      ? `Recorded outcome: ${s.outcome}`
                                      : "Outcome not established as a result."}{" "}
                                    Reviewed revision {s.revision}.
                                  </small>
                                  {s.actionsExcerpt !== "" && <p>{s.actionsExcerpt}</p>}
                                </li>
                              ))}
                            </ul>
                          </div>
                        )}
                        {assistance.question.librarySuggestions.length > 0 && (
                          <div>
                            <h5>Previous writing (phrasing reference only)</h5>
                            <ul className="writing-suggestions">
                              {assistance.question.librarySuggestions.map((s) => (
                                <li key={s.identity}>
                                  <strong>{s.question}</strong>
                                  <small>
                                    {s.employer !== "" ? `${s.employer} · ` : ""}revision {s.revision}
                                  </small>
                                  <p>{s.excerpt}</p>
                                  <button
                                    className="button button-secondary"
                                    type="button"
                                    onClick={() => {
                                      setDraft(s.excerpt);
                                      setDirty(s.excerpt !== selected.value);
                                    }}
                                  >
                                    Start from this phrasing
                                  </button>
                                </li>
                              ))}
                            </ul>
                          </div>
                        )}
                      </>
                    )}

                    <h4>Grounded generation</h4>
                    <p className="small-muted">
                      Generation uses only the sources you select below, through the Go-owned provider path. The
                      result is a suggestion until you save it. A failure never changes your saved writing.
                    </p>
                    <fieldset className="writing-sources">
                      <legend>Applicant-reviewed Career Evidence (up to 2)</legend>
                      {(assistance.sources.Evidence ?? []).map((choice) => (
                        <label key={choice.Identity} className="confirm-row">
                          <input
                            type="checkbox"
                            checked={aiEvidence.includes(choice.Identity)}
                            disabled={aiEvidence.length >= 2 && !aiEvidence.includes(choice.Identity)}
                            onChange={(event) =>
                              setAiEvidence((prev) =>
                                event.target.checked ? [...prev, choice.Identity] : prev.filter((id) => id !== choice.Identity),
                              )
                            }
                          />
                          <span>{choice.Label}</span>
                        </label>
                      ))}
                      {(assistance.sources.Evidence ?? []).length === 0 && (
                        <p className="small-muted">No reviewed evidence available for selection.</p>
                      )}
                    </fieldset>
                    <fieldset className="writing-sources">
                      <legend>Previous writing (structure reference only)</legend>
                      {(assistance.sources.Writing ?? []).map((choice) => (
                        <label key={choice.Identity} className="confirm-row">
                          <input
                            type="radio"
                            name="ai-writing"
                            checked={aiWriting === choice.Identity}
                            onChange={() => setAiWriting(choice.Identity)}
                          />
                          <span>{choice.Label}</span>
                        </label>
                      ))}
                      {(assistance.sources.Writing ?? []).length === 0 && (
                        <p className="small-muted">No saved writing available for selection.</p>
                      )}
                      {aiWriting !== "" && (
                        <button className="button button-secondary" type="button" onClick={() => setAiWriting("")}>
                          Clear writing selection
                        </button>
                      )}
                    </fieldset>
                    <fieldset className="writing-sources">
                      <legend>Résumé context and job requirements</legend>
                      {assistance.sources.ApplicationResume && (
                        <label className="confirm-row">
                          <input type="checkbox" checked={aiAppResume} onChange={(event) => setAiAppResume(event.target.checked)} />
                          <span>Selected application résumé ({assistance.sources.ApplicationResume.Label})</span>
                        </label>
                      )}
                      {assistance.sources.MasterResume && (
                        <label className="confirm-row">
                          <input type="checkbox" checked={aiMasterResume} onChange={(event) => setAiMasterResume(event.target.checked)} />
                          <span>Master résumé ({assistance.sources.MasterResume.Label})</span>
                        </label>
                      )}
                      <label className="confirm-row">
                        <input type="checkbox" checked={aiJobContext} onChange={(event) => setAiJobContext(event.target.checked)} />
                        <span>Include job title, employer, and description as requirements context</span>
                      </label>
                    </fieldset>
                    <div className="field">
                      <label htmlFor="ai-provider">Provider</label>
                      <select id="ai-provider" value={aiProvider} onChange={(event) => setAiProvider(event.target.value)}>
                        {assistance.providers.map((option) => (
                          <option key={option.ID} value={option.ID}>
                            {option.Label}
                          </option>
                        ))}
                      </select>
                      <small>
                        {(assistance.providers.find((option) => option.ID === aiProvider)?.Help ?? "").toString()}
                      </small>
                    </div>
                    <div className="field">
                      <label htmlFor="ai-model">Model</label>
                      <input id="ai-model" value={aiModel} onChange={(event) => setAiModel(event.target.value)} />
                    </div>
                    <label className="confirm-row">
                      <input
                        type="checkbox"
                        checked={confirmExternal}
                        onChange={(event) => setConfirmExternal(event.target.checked)}
                      />
                      <span>I authorize transmitting the selected text to the provider for this specific review</span>
                    </label>
                    <div className="form-actions">
                      <button className="button button-secondary" type="button" disabled={aiBusy} onClick={() => void handleReview()}>
                        {aiBusy ? "Preparing…" : "Review sources"}
                      </button>
                    </div>
                    {ticket && (
                      <div className="writing-ticket" aria-label="Generation review">
                        <p className="small-muted">
                          {ticket.execution ?? "Review the selected sources, then authorize generation."}
                        </p>
                        {ticket.notice !== "" && ticket.notice !== undefined && (
                          <p className="writing-notice" role="note">
                            {ticket.notice}
                          </p>
                        )}
                        {ticket.state === "reviewed" && (
                          <button className="button button-primary" type="button" disabled={aiBusy} onClick={() => void handleGenerate()}>
                            {aiBusy ? "Generating…" : "Authorize and generate"}
                          </button>
                        )}
                        {(ticket.state === "complete" || ticket.state === "insufficient") && ticket.answer && (
                          <>
                            <h5>Suggested answer</h5>
                            <p className="writing-answer-preview">{ticket.answer}</p>
                            {ticket.trace && ticket.trace.claims.length > 0 && (
                              <div>
                                <h5>Source trace</h5>
                                <ul className="writing-suggestions">
                                  {ticket.trace.claims.map((claim, index) => (
                                    <li key={index}>
                                      <p>“{claim.quote}”</p>
                                      <small>Supports: {claim.claim}</small>
                                    </li>
                                  ))}
                                </ul>
                              </div>
                            )}
                            {ticket.trace && ticket.trace.unresolved.length > 0 && (
                              <div>
                                <h5>Unsupported qualifications</h5>
                                <ul className="writing-suggestions">
                                  {ticket.trace.unresolved.map((item, index) => (
                                    <li key={index}>
                                      <small>{item}</small>
                                    </li>
                                  ))}
                                </ul>
                              </div>
                            )}
                            {ticket.state === "insufficient" && (
                              <p className="writing-notice" role="note">
                                The selected evidence cannot reasonably support an answer. The suggestion was withheld;
                                your saved writing is unchanged.
                              </p>
                            )}
                            <div className="form-actions">
                              <button
                                className="button button-primary"
                                type="button"
                                disabled={!ticket.adoptable}
                                onClick={() => void handleUseSuggestion()}
                              >
                                Use suggestion in editor
                              </button>
                              <button className="button button-secondary" type="button" onClick={() => void handleCancelTicket()}>
                                Discard
                              </button>
                            </div>
                            {!ticket.adoptable && (
                              <p className="small-muted">This preview is stale; copy it for reference or discard it.</p>
                            )}
                          </>
                        )}
                      </div>
                    )}
                    {aiError !== "" && (
                      <p className="job-form-error" role="alert">
                        {aiError}
                      </p>
                    )}
                  </div>
                )}
              </>
            ) : (
              <p className="small-muted">Select a question to review and answer it.</p>
            )}
          </div>
        </div>
      )}

      {(snapshot.profileCount > 0 || snapshot.employerOnlyCount > 0 || snapshot.manualReviewCount > 0) && (
        <p className="small-muted">
          {snapshot.profileCount > 0 && `${snapshot.profileCount} approved candidate facts are ready. `}
          {snapshot.employerOnlyCount > 0 && `${snapshot.employerOnlyCount} fields need employer-side completion. `}
          {snapshot.manualReviewCount > 0 && `${snapshot.manualReviewCount} fields need manual review. `}
          <a href={`${legacyHref}#prepare`}>See the full breakdown</a>.
        </p>
      )}

      {snapshot.previous.length > 0 && (
        <div className="writing-previous">
          <h3>Preserved writing for changed questions</h3>
          <ul className="writing-suggestions">
            {snapshot.previous.map((item) => (
              <li key={`${item.early ? "early" : "live"}:${item.key}`}>
                <strong>{item.label}</strong>
                <small>{item.early ? "Early answer" : "Previous live answer"} · revision {item.revision} · preserved</small>
                <p>{item.value}</p>
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}
