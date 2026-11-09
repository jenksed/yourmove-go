// S2 question desk: one working question, a comfortable editor, and visible
// previous writing at the point of need. Go stays authoritative for
// validation, durable saves, and reuse eligibility.

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  aiAdoptCheck,
  aiCancel,
  aiGenerate,
  aiReview,
  fetchAssistance,
  fetchBrief,
  fetchQuestions,
  captureQuestions,
  saveAnswer,
  type AssistanceSnapshot,
  type AiTicket,
  type QuestionsSnapshot,
  type WritingQuestion,
} from "./writingApi";
import { PreviousWriting, type PendingSource } from "./PreviousWriting";

function questionId(q: WritingQuestion): string {
  return `${q.isEarly ? "early" : "live"}:${q.key}`;
}

function triageOf(q: WritingQuestion): string {
  if (q.triageCategory !== "") return q.triageCategory;
  return q.editable ? "answer" : "review";
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
  onMaterialSaved,
  readOnly,
  active,
  discoveryStatus,
  onRetryDiscovery,
}: {
  jobId: number;
  destination: string;
  onMaterialSaved: () => void;
  readOnly: boolean;
  active: boolean;
  discoveryStatus: string;
  onRetryDiscovery: () => void;
}): React.JSX.Element {
  const [snapshot, setSnapshot] = useState<QuestionsSnapshot | null>(null);
  const [captureBusy, setCaptureBusy] = useState(false);
  const [captureNotice, setCaptureNotice] = useState("");
  const [loading, setLoading] = useState<boolean>(true);
  const [loadError, setLoadError] = useState<string>("");
  const [selectedId, setSelectedId] = useState<string>("");
  const selectedIdRef = useRef("");
  selectedIdRef.current = selectedId;
  const [draft, setDraft] = useState<string>("");
  const [dirty, setDirty] = useState<boolean>(false);
  const [baselineRevision, setBaselineRevision] = useState<number>(0);
  const [pendingOrigin, setPendingOrigin] = useState<PendingSource | null>(null);
  const [pendingSourceLabel, setPendingSourceLabel] = useState<string>("");
  const [replaceCandidate, setReplaceCandidate] = useState<{ answer: string; origin: PendingSource | null; label: string } | null>(null);
  const requestRef = useRef(0);
  const dirtyRef = useRef(false);
  dirtyRef.current = dirty;
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
  const [briefScope, setBriefScope] = useState<string>("unanswered");
  const [briefBusy, setBriefBusy] = useState<boolean>(false);
  const [briefError, setBriefError] = useState<string>("");
  const [briefNotice, setBriefNotice] = useState<string>("");
  const [briefFallback, setBriefFallback] = useState<string>("");

  const reload = useCallback(async () => {
    const request = ++requestRef.current;
    setLoading(true);
    setLoadError("");
    try {
      const next = await fetchQuestions(jobId);
      if (request !== requestRef.current) return;
      setSnapshot(next);
      // Select the first question needing an answer so the desk never opens
      // on a blank selector. Mirrors the legacy workbench's first-unanswered
      // default; an explicit selection is never overridden here.
      if (selectedIdRef.current === "") {
        const tasks = (next.questions ?? []).filter((q) => (q.triageCategory !== "" ? q.triageCategory : q.editable ? "answer" : "review") === "answer");
        const firstOpen = tasks.find((q) => q.value === "") ?? tasks[0];
        if (firstOpen) setSelectedId(`${firstOpen.isEarly ? "early" : "live"}:${firstOpen.key}`);
      }
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

  useEffect(() => {
    if (!active) return;
    let cancelled = false;
    let inFlight = false;
    const refreshObserved = async () => {
      if (inFlight) return;
      inFlight = true;
      const request = ++requestRef.current;
      try {
        const next = await fetchQuestions(jobId);
        if (!cancelled && request === requestRef.current) setSnapshot((previous) => {
          // Retain a vanished selection only while unsaved edits need protection.
          // A clean disappearance (retired triage, context flip) is left to the
          // repair effect below so Next/Previous keep working on a live identity.
          const selected = previous && previous.questions.find((q) => questionId(q) === selectedIdRef.current);
          if (selected && dirtyRef.current && !next.questions.some((q) => questionId(q) === selectedIdRef.current)) {
            next.questions = [...next.questions, selected];
          }
          return next;
        });
      } catch {
        // Keep the last successful inventory; manual refresh reports errors.
      } finally { inFlight = false; }
    };
    void refreshObserved();
    window.addEventListener("focus", refreshObserved);
    const timer = window.setInterval(() => void refreshObserved(), snapshot?.browserActive ? 1000 : 4000);
    return () => { cancelled = true; window.removeEventListener("focus", refreshObserved); window.clearInterval(timer); };
  }, [active, jobId, snapshot?.browserActive, discoveryStatus]);

  const allQuestions = useMemo(() => {
    if (!snapshot) return [];
    if (snapshot.questions) return snapshot.questions;
    return [...snapshot.live, ...snapshot.early];
  }, [snapshot]);

  const selected: WritingQuestion | null = useMemo(() => {
    return allQuestions.find((q) => questionId(q) === selectedId) ?? null;
  }, [allQuestions, selectedId]);

  useEffect(() => {
    if (selected) {
      setDraft(selected.value);
      setDirty(false);
      setBaselineRevision(selected.revision);
      setPendingOrigin(null);
      setPendingSourceLabel("");
      setReplaceCandidate(null);
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
    if (selected && !dirtyRef.current) {
      setDraft(selected.value);
      setBaselineRevision((previous) => (selected.revision !== previous && selected.value !== draft ? selected.revision : previous));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selected?.value, selected?.revision]);

  // Single selection rule, evaluated after every inventory change: without
  // unsaved edits there is always a valid answer-task selection. This covers
  // initial mount (alongside the fast path in reload), raced initial fetches,
  // and inventories that retire the selected question (profile approval,
  // live/early context flip). Drafts under the dirty guard are never touched.
  useEffect(() => {
    if (loading || dirtyRef.current) return;
    const resolved = selectedId === "" ? undefined : allQuestions.find((q) => questionId(q) === selectedId);
    if (resolved && triageOf(resolved) === "answer") return;
    const tasks = allQuestions.filter((q) => triageOf(q) === "answer");
    const firstOpen = tasks.find((q) => q.value === "") ?? tasks[0];
    if (firstOpen && questionId(firstOpen) !== selectedId) setSelectedId(questionId(firstOpen));
  }, [loading, selectedId, allQuestions]);

  useEffect(() => {
    if (!dirty) return;
    const guard = (event: BeforeUnloadEvent) => {
      event.preventDefault();
    };
    window.addEventListener("beforeunload", guard);
    return () => window.removeEventListener("beforeunload", guard);
  }, [dirty]);

  const discardEdits = useCallback(() => {
    if (!selected) return;
    setDraft(selected.value);
    dirtyRef.current = false;
    setDirty(false);
    setSaveError("");
    setPendingOrigin(null);
    setReplaceCandidate(null);
  }, [selected]);

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

  const handleUseAsDraft = useCallback(
    (answer: string, origin: PendingSource | null, label: string) => {
      if (!selected) return;
      if (dirty && draft !== answer) {
        setReplaceCandidate({ answer, origin, label });
        return;
      }
      setDraft(answer);
      dirtyRef.current = answer !== selected.value;
      setDirty(dirtyRef.current);
      setPendingOrigin(origin);
      setPendingSourceLabel(label);
      setSaveNotice(origin ? `Started from ${label}. Review employer-specific wording, then save.` : "");
      setSaveError("");
    },
    [selected, dirty, draft],
  );

  const confirmReplace = useCallback(() => {
    if (!selected || !replaceCandidate) return;
    setDraft(replaceCandidate.answer);
    dirtyRef.current = replaceCandidate.answer !== selected.value;
    setDirty(dirtyRef.current);
    setPendingOrigin(replaceCandidate.origin);
    setPendingSourceLabel(replaceCandidate.label);
    setSaveNotice(`Started from ${replaceCandidate.label}. Review employer-specific wording, then save.`);
    setSaveError("");
    setReplaceCandidate(null);
  }, [selected, replaceCandidate]);

  const handleSave = useCallback(async (toLibrary = false) => {
    if (!selected || !snapshot?.editingAllowed || readOnly) return;
    setSaveError("");
    setSaveNotice("");
    setSaving(true);
    try {
      // Automatic preparation carries provenance when the applicant saves the
      // suggested text without choosing another source. An explicit starting
      // draft takes precedence over the automatic suggestion.
      const autoEntryId = pendingOrigin?.entryId ?? (selected.libraryAutoPrepared ? selected.libraryAutoEntryId : 0);
      const autoRevision = pendingOrigin?.revision ?? (selected.libraryAutoPrepared ? selected.libraryAutoRevision : 0);
      const result = await saveAnswer(jobId, {
        save_to_library: toLibrary,
        question_key: selected.key,
        revision: baselineRevision,
        destination,
        answer: draft,
        reusable,
        reusable_revision: selected.reusableRevision,
        early: selected.isEarly,
        discovery_source: selected.discoverySource,
        discovery_fingerprint: selected.discoveryFingerprint,
        source_question_id: selected.sourceQuestionId,
        origin_entry_id: autoEntryId,
        origin_revision: autoRevision,
      });
      setDirty(false);
      setPendingOrigin(null);
      setReplaceCandidate(null);
      setBaselineRevision(result.revision);
      setSaveNotice(toLibrary ? "Saved for this application and available in the Answer Library. Existing reusable answers remain unchanged; edit them explicitly in the library." : "Saved for this application only. Nothing was filled on the employer website.");
      onMaterialSaved();
      await reload();
      void result;
    } catch (error) {
      setSaveError(error instanceof Error ? error.message : "Unable to save the answer");
    } finally {
      setSaving(false);
    }
  }, [selected, snapshot, jobId, destination, draft, reusable, baselineRevision, pendingOrigin, reload, onMaterialSaved, readOnly]);

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

  const handleCopyBrief = useCallback(async () => {
    setBriefError("");
    setBriefNotice("");
    setBriefFallback("");
    setBriefBusy(true);
    try {
      const text = await fetchBrief(jobId, briefScope);
      if (typeof navigator === "undefined" || !navigator.clipboard?.writeText) {
        setBriefFallback(text);
        setBriefNotice("Clipboard access is unavailable here. Copy the text below manually.");
        return;
      }
      try {
        await navigator.clipboard.writeText(text);
        setBriefNotice("Copied. Paste it into any text editor.");
      } catch {
        setBriefFallback(text);
        setBriefNotice("Clipboard access was denied. Copy the text below manually.");
      }
    } catch (error) {
      setBriefError(error instanceof Error ? error.message : "Unable to copy the application brief");
    } finally {
      setBriefBusy(false);
    }
  }, [jobId, briefScope]);

  const handleUseSuggestion = useCallback(async () => {
    if (!ticket?.answer || !selected) return;
    setAiError("");
    try {
      await aiAdoptCheck(jobId, ticket.ticket, selected.key, String(selected.revision));
      setDraft(ticket.answer);
      dirtyRef.current = true;
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

  const answerTasks = allQuestions.filter((q) => triageOf(q) === "answer");
  const profileItems = allQuestions.filter((q) => triageOf(q) === "profile");
  const employerItems = allQuestions.filter((q) => triageOf(q) === "employer");
  const reviewItems = allQuestions.filter((q) => triageOf(q) === "review");

  const selectedIndex = selected ? answerTasks.findIndex((q) => questionId(q) === selectedId) : -1;
  const staleBaseline = selected !== null && selected.revision !== baselineRevision && dirty;
  const canSave = selected !== null && (dirty || (reusable && selected.reusableEligible)) && selected.editable && snapshot.editingAllowed && !readOnly;

  return (
    <section className="panel" aria-labelledby="writing-title" id="writing">
      <div className="section-heading">
        <div>
          <h2 id="writing-title">Questions</h2>
          <p className="section-subtitle">
            {snapshot.available
              ? snapshot.activeContext === "live"
                ? `${snapshot.source}. Your text saves locally in YourMove; nothing is filled on the employer site from here.`
                : `${snapshot.source}. These questions have not yet been verified on your live form.`
              : "Saved writing below is preserved."}
            {snapshot.partial ? " Partial inspection; later questions may be missing." : ""}
          </p>
        </div>
        <div className="heading-actions">
          <button className="button" type="button" disabled={captureBusy || readOnly || !snapshot.editingAllowed} onClick={async () => {
            setCaptureBusy(true);
            setCaptureNotice("");
            try {
              await captureQuestions(jobId);
              setCaptureNotice("Navigate the employer form yourself. Capture observes question definitions; it does not authorize any transfer. Current capture status appears below.");
            } catch (error) {
              setCaptureNotice(error instanceof Error ? error.message : "Unable to start question capture.");
            } finally { setCaptureBusy(false); }
          }}>
            {captureBusy ? "Opening capture browser…" : "Capture application questions"}
          </button>
          <button className="button button-secondary" type="button" disabled={dirty} onClick={() => void reload()}>
            Refresh questions
          </button>
        </div>
      </div>

      {captureNotice && <p role="status">{captureNotice}</p>}
      <p role="status" aria-label="Question capture status">
        {snapshot.captureState?.phase
          ? `Capture — ${snapshot.captureState.phase}: ${snapshot.captureState.message}`
          : "Capture is not running. Open the managed browser to observe application questions."}
      </p>
      {dirty && <p className="small-muted">Save your answer before refreshing the question list.</p>}

      {discoveryStatus === "checking" && <p role="status">Checking for application questions…</p>}
      {discoveryStatus === "unsupported" && answerTasks.length > 0 && <p className="writing-notice">This employer does not offer an early question preview. Select Capture application questions to check managed inspection support.</p>}
      {discoveryStatus === "failed" && <p className="writing-notice">We could not check questions right now. Your saved work is safe. <button className="button button-secondary" type="button" onClick={onRetryDiscovery}>Retry question check</button> You can also select Capture application questions.</p>}

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
            {discoveryStatus === "checking" ? "Checking this posting for questions." : "No early questions are available. Select Capture application questions, then navigate the employer form in the managed browser."}
          </p>
        </div>
      )}

      {answerTasks.length > 0 && (
        <>
          <div className="question-navigator" aria-label="Question navigation">
            <button
              className="button button-secondary"
              type="button"
              disabled={selectedIndex <= 0 || dirty}
              onClick={() => { if (selectedIndex > 0) pickQuestion(answerTasks[selectedIndex - 1]); }}
            >
              ← Previous
            </button>
            <label className="question-selector">
              <span className="small-muted">Question {selectedIndex + 1} of {answerTasks.length}</span>
              <select value={selectedId} onChange={(event) => {
                const target = answerTasks.find((q) => questionId(q) === event.target.value);
                if (target) pickQuestion(target);
              }}>
                {answerTasks.map((q) => (
                  <option key={questionId(q)} value={questionId(q)}>
                    {q.label.length > 80 ? `${q.label.slice(0, 80)}…` : q.label}{q.value !== "" ? " · prepared" : " · needs answer"}
                  </option>
                ))}
              </select>
            </label>
          <button
            className="button button-secondary"
            type="button"
            disabled={selectedIndex < 0 || selectedIndex >= answerTasks.length - 1 || dirty}
            onClick={() => { if (selectedIndex >= 0 && selectedIndex < answerTasks.length - 1) pickQuestion(answerTasks[selectedIndex + 1]); }}
          >
            Next →
          </button>
          {dirty && selected && (
            <div className="question-dirty-note">
              <p className="small-muted" role="note">
                Unsaved edits on this question — save or discard them to move to another question.
              </p>
              <div className="form-actions">
                <button className="button button-secondary" type="button" disabled={saving || !canSave} onClick={() => void handleSave()}>
                  {saving ? "Saving…" : "Save for this application only"}
                </button>
                <button className="button button-secondary" type="button" onClick={discardEdits}>
                  Discard edits
                </button>
              </div>
            </div>
          )}
          </div>

          <div className="writing-desk">
            <div className="writing-editor" aria-live="polite">
              {selected ? (
                <>
                  <h3>{selected.label}</h3>
                  <p className="small-muted">
                    {selected.isEarly ? `Published definition (${selected.discoverySource})` : selected.localStatus}
                    {selected.revision > 0 ? ` · Saved revision ${selected.revision}` : " · Not saved yet"}
                    {selected.required ? " · Required by employer" : " · Optional"}
                  </p>
                  {selected.originRecorded && (
                    <p className="small-muted">
                      Started from {selected.originQuestion !== "" ? `“${selected.originQuestion}”` : "a saved example"}
                      {selected.originEmployer !== "" ? ` (${selected.originEmployer})` : ""}. Edits since then are yours.
                    </p>
                  )}
                  {pendingOrigin && (
                    <p className="small-muted" role="status">
                      Starting from {pendingSourceLabel}. Review employer-specific wording, then save.
                    </p>
                  )}
                  {selected.libraryAutoPrepared && !pendingOrigin && (
                    <p className="small-muted" role="status">
                      Automatically prepared from Answer Library{selected.libraryAutoQuestion !== "" && selected.libraryAutoQuestion !== selected.label ? ` (“${selected.libraryAutoQuestion}”)` : ""}. Review employer-specific wording, then save. Saving keeps this application independent; the library copy is unchanged.
                    </p>
                  )}
                  {staleBaseline && (
                    <p className="writing-notice" role="note">
                      The saved answer changed in another window (now revision {selected.revision}). Your edits are kept; saving uses your original revision {baselineRevision} and will ask you to review first.
                    </p>
                  )}
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
                      dirtyRef.current = next !== selected.value;
                      setDirty(dirtyRef.current);
                    }}
                  />
                  {selected.reusableAvailable && (
                    <div className="exact-reuse" aria-label="Exact reusable answer">
                      <h4>Exact answer available</h4>
                      <p className="small-muted">Go approved this exact wording for this employer (memory revision {selected.reusableRevision}). Editing below saves an application-only override.</p>
                      <p className="writing-answer-preview">{selected.reusableAnswer}</p>
                    </div>
                  )}
                  {selected.reusableEligible && snapshot.editingAllowed && !readOnly && (
                    <label className="confirm-row">
                      <input type="checkbox" checked={reusable} onChange={(event) => setReusable(event.target.checked)} />
                      <span>Approve exact reuse for this employer when saved</span>
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
                    {selected.libraryEligible && <button className="button button-primary" type="button" disabled={saving || !selected.editable || !snapshot.editingAllowed || readOnly || !draft.trim()} onClick={() => void handleSave(true)}>{saving ? "Saving…" : "Save to Answer Library"}</button>}
                    {dirty && <button className="button button-secondary" type="button" onClick={discardEdits}>Discard edits</button>}
                    <button
                      className="button button-secondary"
                      type="button"
                      disabled={saving || !canSave}
                      onClick={() => void handleSave()}
                    >
                      {saving ? "Saving…" : "Save for this application only"}
                    </button>
                  </div>
                  {replaceCandidate && (
                    <div className="writing-replace" role="alert">
                      <p><strong>Replace your unsaved edits?</strong> “{replaceCandidate.label}” would replace the current editor text. This stays unsaved until you save.</p>
                      <div className="form-actions">
                        <button className="button button-primary" type="button" onClick={confirmReplace}>Replace editor text</button>
                        <button className="button button-secondary" type="button" onClick={() => setReplaceCandidate(null)}>Keep my draft</button>
                      </div>
                    </div>
                  )}
                  {dirty && <p className="small-muted">Unsaved edits — they stay on this screen until you save.</p>}
                  {assistError !== "" && (
                    <p className="job-form-error" role="alert">
                      {assistError}
                    </p>
                  )}
                </>
              ) : (
                <p className="small-muted">Select a question to review and answer it.</p>
              )}
            </div>

            {selected && (
              <aside className="previous-column" aria-label="Previous writing column">
                <PreviousWriting
                  jobId={jobId}
                  destination={destination}
                  question={selected}
                  baselineRevision={baselineRevision}
                  disabled={!selected.editable || !snapshot.editingAllowed || readOnly}
                  onUseAsDraft={handleUseAsDraft}
                />
              </aside>
            )}
          </div>

          <details className="panel detail-editor">
            <summary><strong>Use existing experience and grounded generation</strong></summary>
            <div className="editor-content">
              {selected && selected.helpEligible && snapshot.editingAllowed && !readOnly && (
                <button className="button button-secondary" type="button" disabled={assistBusy} onClick={() => void openAssistance()}>
                  {assistBusy ? "Loading…" : "Load experience and generation"}
                </button>
              )}
              {assistOpen && assistance && (
                <div className="writing-assist" aria-label="Experience and generation">
                  <h4>Your relevant experience</h4>
                  {assistance.libraryEmpty && assistance.evidenceEmpty ? (
                    <p className="small-muted">No reviewed Career Evidence or saved writing is available yet.</p>
                  ) : (
                    <>
                      {assistance.question.evidenceSuggestions.length > 0 && (
                        <div>
                          <h5>Reviewed Career Evidence</h5>
                          <ul className="writing-suggestions">
                            {assistance.question.evidenceSuggestions.map((s) => (
                              <li key={s.identity}>
                                <strong>{s.title}</strong>
                                <small>{s.outcomeState === "recorded" && s.outcome !== "" ? `Recorded outcome: ${s.outcome}` : "Outcome not established as a result."} Reviewed revision {s.revision}.</small>
                                {s.actionsExcerpt !== "" && <p>{s.actionsExcerpt}</p>}
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}
                    </>
                  )}
                  <h4>Grounded generation</h4>
                  <p className="small-muted">Generation uses only the sources you select below. The result is a suggestion until you save it.</p>
                  <fieldset className="writing-sources">
                    <legend>Applicant-reviewed Career Evidence (up to 2)</legend>
                    {(assistance.sources.Evidence ?? []).map((choice) => (
                      <label key={choice.Identity} className="confirm-row">
                        <input type="checkbox" checked={aiEvidence.includes(choice.Identity)} disabled={aiEvidence.length >= 2 && !aiEvidence.includes(choice.Identity)} onChange={(event) => setAiEvidence((prev) => event.target.checked ? [...prev, choice.Identity] : prev.filter((id) => id !== choice.Identity))} />
                        <span>{choice.Label}</span>
                      </label>
                    ))}
                    {(assistance.sources.Evidence ?? []).length === 0 && <p className="small-muted">No reviewed evidence available for selection.</p>}
                  </fieldset>
                  <fieldset className="writing-sources">
                    <legend>Previous writing (structure reference only)</legend>
                    {(assistance.sources.Writing ?? []).map((choice) => (
                      <label key={choice.Identity} className="confirm-row">
                        <input type="radio" name="ai-writing" checked={aiWriting === choice.Identity} onChange={() => setAiWriting(choice.Identity)} />
                        <span>{choice.Label}</span>
                      </label>
                    ))}
                    {(assistance.sources.Writing ?? []).length === 0 && <p className="small-muted">No saved writing available for selection.</p>}
                    {aiWriting !== "" && <button className="button button-secondary" type="button" onClick={() => setAiWriting("")}>Clear writing selection</button>}
                  </fieldset>
                  <div className="field">
                    <label htmlFor="ai-provider">Provider</label>
                    <select id="ai-provider" value={aiProvider} onChange={(event) => setAiProvider(event.target.value)}>
                      {assistance.providers.map((option) => (
                        <option key={option.ID} value={option.ID}>{option.Label}</option>
                      ))}
                    </select>
                  </div>
                  <div className="field">
                    <label htmlFor="ai-model">Model</label>
                    <input id="ai-model" value={aiModel} onChange={(event) => setAiModel(event.target.value)} />
                  </div>
                  <label className="confirm-row">
                    <input type="checkbox" checked={confirmExternal} onChange={(event) => setConfirmExternal(event.target.checked)} />
                    <span>I authorize transmitting the selected text to the provider for this specific review</span>
                  </label>
                  <div className="form-actions">
                    <button className="button button-secondary" type="button" disabled={aiBusy} onClick={() => void handleReview()}>Review sources</button>
                  </div>
                  {ticket && (
                    <div className="writing-ticket" aria-label="Generation review">
                      <p className="small-muted">{ticket.execution ?? "Review the selected sources, then authorize generation."}</p>
                      {ticket.state === "reviewed" && <button className="button button-primary" type="button" disabled={aiBusy} onClick={() => void handleGenerate()}>Authorize and generate</button>}
                      {(ticket.state === "complete" || ticket.state === "insufficient") && ticket.answer && (
                        <>
                          <h5>Suggested answer</h5>
                          <p className="writing-answer-preview">{ticket.answer}</p>
                          <div className="form-actions">
                            <button className="button button-primary" type="button" disabled={!ticket.adoptable} onClick={() => void handleUseSuggestion()}>Use suggestion in editor</button>
                            <button className="button button-secondary" type="button" onClick={() => void handleCancelTicket()}>Discard</button>
                          </div>
                        </>
                      )}
                    </div>
                  )}
                  {aiError !== "" && <p className="job-form-error" role="alert">{aiError}</p>}
                </div>
              )}
            </div>
          </details>

          <details className="panel detail-editor">
            <summary><strong>Application brief</strong><small>Plain-text job details for reference</small></summary>
            <div className="editor-content">
              <div className="field">
                <label htmlFor="brief-scope">Questions to include</label>
                <select id="brief-scope" value={briefScope} onChange={(event) => setBriefScope(event.target.value)}>
                  <option value="unanswered">Unanswered actionable questions only</option>
                  <option value="all">All actionable questions · wording only</option>
                </select>
              </div>
              <button className="button button-secondary" type="button" disabled={briefBusy} onClick={() => void handleCopyBrief()}>{briefBusy ? "Copying…" : "Copy application brief"}</button>
              {briefError !== "" && <p className="job-form-error" role="alert">{briefError}</p>}
              {briefNotice !== "" && <p className="form-notice" role="status">{briefNotice}</p>}
              {briefFallback !== "" && <textarea rows={12} readOnly value={briefFallback} onFocus={(event) => event.target.select()} aria-label="Application brief" />}
            </div>
          </details>
        </>
      )}

      {(snapshot.unidentifiedCount > 0) && (
        <p className="writing-notice" role="note">
          {snapshot.unidentifiedCount === 1 ? "One employer control could not be stably identified; inspect it directly in the employer browser." : `${snapshot.unidentifiedCount} employer controls could not be stably identified; inspect them directly in the employer browser.`}
        </p>
      )}

      {(profileItems.length > 0 || employerItems.length > 0 || reviewItems.length > 0) && (
        <div className="writing-previous" aria-label="Fields outside the writing tasks">
          {profileItems.length > 0 && <p className="small-muted">Candidate Profile fields are handled automatically when authorized.</p>}
          {employerItems.length > 0 && (
            <div>
              <h3>Complete on the employer site</h3>
              <ul className="writing-suggestions">
                {employerItems.map((q) => (
                  <li key={questionId(q)}><strong>{q.label}</strong><small>{q.required ? "Required by employer" : "Optional"}</small></li>
                ))}
              </ul>
            </div>
          )}
          {reviewItems.length > 0 && (
            <div>
              <h3>Needs manual completion</h3>
              <ul className="writing-suggestions">
                {reviewItems.map((q) => (
                  <li key={questionId(q)}><strong>{q.label}</strong><small>{q.required ? "Required by employer · " : ""}{q.manualReason !== "" ? q.manualReason : "This question requires manual completion in Chromium."}</small></li>
                ))}
              </ul>
            </div>
          )}
        </div>
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
