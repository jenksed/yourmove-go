// S2 cover letters: blank, prior-letter, or generated starts share one
// destination editor. Prior-letter reuse creates a new manual identity;
// preview never selects or saves.

import { useCallback, useEffect, useState } from "react";
import {
  browseCoverLetters,
  createManualLetter,
  fetchCatalogLetter,
  fetchLetterDraft,
  fetchLetterHistory,
  fetchLetters,
  generateLetter,
  saveLetterDraft,
  selectLetterDraft,
  type LetterCatalogEntry,
  type LetterDraft,
  type LettersSnapshot,
} from "./writingApi";
import { setUnsaved } from "../../shared/unsaved";

export interface LetterResumeOption {
  id: number;
  name: string;
  kind: string;
  version: number;
  archived: boolean;
}

function resumeLabel(draft: LetterDraft): string {
  if (draft.manualSource || (draft.masterResumeId === 0 && draft.roleResumeId === 0)) return "Manual — no résumé source";
  const parts: string[] = [];
  if (draft.roleName !== "") parts.push(`${draft.roleName} · v${draft.roleVersion}`);
  else if (draft.roleResumeId > 0) parts.push(`Résumé ${draft.roleResumeId}`);
  if (draft.masterName !== "") parts.push(`master ${draft.masterName} · v${draft.masterVersion}`);
  return parts.length > 0 ? parts.join(" · ") : "Résumé binding recorded";
}

function entryLabel(entry: LetterCatalogEntry): string {
  const origin = entry.manual ? "Manual" : entry.provider;
  const resume = entry.roleName !== "" ? `${entry.roleName} · v${entry.roleVersion}` : entry.manual ? "no résumé source" : "résumé binding recorded";
  return `${entry.employer !== "" ? entry.employer : "Saved opportunity"} · ${entry.jobTitle} · ${origin} · ${resume}`;
}

export function CoverLetterSection({
  jobId,
  resumeOptions,
  selectedResumeId,
  onMaterialSaved,
  readOnly,
}: {
  jobId: number;
  resumeOptions: LetterResumeOption[];
  selectedResumeId: number;
  onMaterialSaved: () => void;
  readOnly: boolean;
}): React.JSX.Element {
  const [snapshot, setSnapshot] = useState<LettersSnapshot | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [loadError, setLoadError] = useState<string>("");
  const [selectedId, setSelectedId] = useState<number>(0);
  const [manualDraft, setManualDraft] = useState<string>("");
  const [draft, setDraft] = useState<string>("");
  const [draftHash, setDraftHash] = useState<string>("");
  const [dirty, setDirty] = useState<boolean>(false);
  const [saveError, setSaveError] = useState<string>("");
  const [saveNotice, setSaveNotice] = useState<string>("");
  const [saving, setSaving] = useState<boolean>(false);
  const [genProvider, setGenProvider] = useState<string>("");
  const [genModel, setGenModel] = useState<string>("");
  const [genResumeId, setGenResumeId] = useState<string>("");
  const [genEvidence, setGenEvidence] = useState<string[]>([]);
  const [confirmExternal, setConfirmExternal] = useState<boolean>(false);
  const [confirmLocal, setConfirmLocal] = useState<boolean>(false);
  const [genBusy, setGenBusy] = useState<boolean>(false);
  const [genError, setGenError] = useState<string>("");
  const [genNotice, setGenNotice] = useState<string>("");
  const [historyEntries, setHistoryEntries] = useState<LetterCatalogEntry[]>([]);
  const [historyCursor, setHistoryCursor] = useState<number>(0);
  const [historyHasMore, setHistoryHasMore] = useState<boolean>(false);
  const [historySelectedId, setHistorySelectedId] = useState<number>(0);
  const [pickerOpen, setPickerOpen] = useState<boolean>(false);
  const [catalogQuery, setCatalogQuery] = useState<string>("");
  const [catalogEntries, setCatalogEntries] = useState<LetterCatalogEntry[]>([]);
  const [catalogCursor, setCatalogCursor] = useState<number>(0);
  const [catalogHasMore, setCatalogHasMore] = useState<boolean>(false);
  const [catalogEmpty, setCatalogEmpty] = useState<boolean>(false);
  const [catalogBusy, setCatalogBusy] = useState<boolean>(false);
  const [catalogError, setCatalogError] = useState<string>("");
  const [previewLetter, setPreviewLetter] = useState<LetterDraft | null>(null);
  const [previewJob, setPreviewJob] = useState<{ employer: string; title: string } | null>(null);
  const [previewBusy, setPreviewBusy] = useState<boolean>(false);
  const [previewError, setPreviewError] = useState<string>("");
  const [newDraft, setNewDraft] = useState<string>("");
  const [newDraftSource, setNewDraftSource] = useState<{ id: number; jobId: number; hash: string; label: string } | null>(null);
  const [newDraftError, setNewDraftError] = useState<string>("");
  const [newDraftNotice, setNewDraftNotice] = useState<string>("");

  const reload = useCallback(
    async (selectId?: number) => {
      setLoading(true);
      setLoadError("");
      try {
        const next = await fetchLetters(jobId);
        setSnapshot(next);
        setGenProvider(next.defaultProvider);
        const wanted = selectId ?? next.current?.id ?? 0;
        setSelectedId(wanted);
        if (wanted > 0) {
          const opened = await fetchLetterDraft(jobId, wanted);
          setDraft(opened.body);
          setDraftHash(opened.contentHash);
        } else {
          setDraft("");
          setDraftHash("");
        }
        setDirty(false);
        const history = await fetchLetterHistory(jobId, 0, 20);
        setHistoryEntries(history.entries);
        setHistoryCursor(history.nextCursor);
        setHistoryHasMore(history.hasMore);
        setHistorySelectedId(history.selectedId);
      } catch (error) {
        setLoadError(error instanceof Error ? error.message : "Unable to load cover letters");
      } finally {
        setLoading(false);
      }
    },
    [jobId],
  );

  useEffect(() => {
    void reload();
  }, [reload]);

  useEffect(() => {
    setGenResumeId(String(selectedResumeId));
  }, [selectedResumeId]);

  useEffect(() => {
    // Report letter drafts to the desktop close guard; the editor keeps
    // its explicit save and revision checks.
    setUnsaved("letter", dirty || manualDraft !== "" || newDraft !== "");
    if (!dirty && manualDraft === "" && newDraft === "") return;
    const guard = (event: BeforeUnloadEvent) => {
      event.preventDefault();
    };
    window.addEventListener("beforeunload", guard);
    return () => window.removeEventListener("beforeunload", guard);
  }, [dirty, manualDraft, newDraft]);

  const openDraft = useCallback(
    async (id: number) => {
      if (id === selectedId) return;
      if (dirty || newDraft !== "") {
        setSaveError("Save or discard your edits before opening another draft.");
        return;
      }
      setSaveError("");
      setSaveNotice("");
      try {
        const opened = await fetchLetterDraft(jobId, id);
        if (!readOnly) {
          await selectLetterDraft(jobId, id);
          onMaterialSaved();
        }
        setSelectedId(id);
        setDraft(opened.body);
        setDraftHash(opened.contentHash);
        setDirty(false);
        setHistorySelectedId(id);
        setSnapshot((prev) => {
          if (!prev) return prev;
          return { ...prev, current: opened };
        });
      } catch (error) {
        setSaveError(error instanceof Error ? error.message : "Unable to open the draft");
      }
    },
    [dirty, newDraft, selectedId, jobId, onMaterialSaved, readOnly],
  );

  const previewHistoryDraft = useCallback(
    async (id: number) => {
      setPreviewError("");
      setPreviewBusy(true);
      try {
        const opened = await fetchLetterDraft(jobId, id);
        setPreviewLetter(opened);
        setPreviewJob(null);
      } catch (error) {
        setPreviewError(error instanceof Error ? error.message : "Unable to preview this draft");
      } finally {
        setPreviewBusy(false);
      }
    },
    [jobId],
  );

  const handleSave = useCallback(async () => {
    if (selectedId === 0) return;
    setSaveError("");
    setSaveNotice("");
    setSaving(true);
    try {
      await saveLetterDraft(jobId, selectedId, draft, draftHash);
      setDirty(false);
      setSaveNotice("Draft saved exactly as edited.");
      onMaterialSaved();
      await reload(selectedId);
    } catch (error) {
      setSaveError(error instanceof Error ? error.message : "Unable to save the draft");
    } finally {
      setSaving(false);
    }
  }, [selectedId, jobId, draft, draftHash, reload, onMaterialSaved]);

  const handleManual = useCallback(async () => {
    setSaveError("");
    setSaving(true);
    try {
      const result = await createManualLetter(jobId, manualDraft);
      setManualDraft("");
      onMaterialSaved();
      await reload(result.draftId);
      setSaveNotice("Your letter was saved.");
    } catch (error) {
      setSaveError(error instanceof Error ? error.message : "Unable to save the letter");
    } finally { setSaving(false); }
  }, [jobId, manualDraft, onMaterialSaved, reload]);

  const handleGenerate = useCallback(async () => {
    if (dirty || newDraft !== "") {
      setGenError("Save or discard your edits before generating a new draft. Generation never replaces unsaved text.");
      return;
    }
    setGenError("");
    setGenNotice("");
    setGenBusy(true);
    try {
      const result = await generateLetter(jobId, {
        provider: genProvider,
        model: genModel,
        letter_resume_id: genResumeId === "" ? 0 : Number(genResumeId),
        career_evidence: genEvidence,
        confirm_external: confirmExternal ? "yes" : "",
        confirm_local: confirmLocal ? "yes" : "",
      });
      try {
        await selectLetterDraft(jobId, result.draftId);
        setGenNotice("New draft generated and selected. Your other drafts are unchanged.");
      } catch {
        setGenNotice("The draft was generated, but could not be selected. Choose it from saved drafts.");
        await reload();
        return;
      }
      onMaterialSaved();
      setConfirmExternal(false);
      setConfirmLocal(false);
      await reload(result.draftId);
    } catch (error) {
      setGenError(
        error instanceof Error
          ? `${error.message} Prior drafts are unchanged.`
          : "Generation failed. Prior drafts are unchanged.",
      );
    } finally {
      setGenBusy(false);
    }
  }, [jobId, genProvider, genModel, genResumeId, genEvidence, confirmExternal, confirmLocal, reload, onMaterialSaved, dirty, newDraft]);

  const runCatalog = useCallback(
    async (cursor: number, query: string, append: boolean) => {
      setCatalogBusy(true);
      setCatalogError("");
      try {
        const result = await browseCoverLetters({ query: query.trim() === "" ? undefined : query.trim(), cursor, limit: 20 });
        setCatalogEmpty(result.libraryEmpty);
        setCatalogCursor(result.nextCursor);
        setCatalogHasMore(result.hasMore);
        setCatalogEntries((previous) => (append ? [...previous, ...result.entries] : result.entries));
      } catch (error) {
        setCatalogError(error instanceof Error ? error.message : "Unable to browse saved letters");
      } finally {
        setCatalogBusy(false);
      }
    },
    [],
  );

  const openPicker = useCallback(() => {
    if (dirty || newDraft !== "") {
      setNewDraftError("Save or discard your current edits before starting from a prior letter.");
      return;
    }
    setPickerOpen(true);
    setNewDraftError("");
    if (catalogEntries.length === 0 && !catalogBusy) void runCatalog(0, catalogQuery, false);
  }, [dirty, newDraft, catalogEntries.length, catalogBusy, runCatalog, catalogQuery]);

  const previewCatalogLetter = useCallback(async (entry: LetterCatalogEntry) => {
    setPreviewError("");
    setPreviewBusy(true);
    try {
      const detail = await fetchCatalogLetter(entry.id);
      setPreviewLetter(detail.letter);
      setPreviewJob({ employer: detail.job.employer !== "" ? detail.job.employer : detail.job.host, title: detail.job.title });
    } catch (error) {
      setPreviewError(error instanceof Error ? error.message : "Unable to preview this letter");
    } finally {
      setPreviewBusy(false);
    }
  }, []);

  const usePreviewAsDraft = useCallback(() => {
    if (!previewLetter) return;
    if (dirty || newDraft !== "" || draft !== (snapshot?.current?.body ?? draft)) {
      // Guard late responses and dirty state explicitly.
    }
    if (dirty) {
      setNewDraftError("Save or discard your selected-draft edits before starting from a prior letter.");
      return;
    }
    setNewDraft(previewLetter.body);
    setNewDraftSource({ id: previewLetter.id, jobId: previewLetter.jobId, hash: previewLetter.contentHash, label: entryLabel({ ...previewLetter, employer: previewJob?.employer ?? "", jobTitle: previewJob?.title ?? "", preview: "", masterName: previewLetter.masterName, masterVersion: previewLetter.masterVersion, roleName: previewLetter.roleName, roleVersion: previewLetter.roleVersion, manual: previewLetter.manualSource } as LetterCatalogEntry) });
    setNewDraftError("");
    setNewDraftNotice("Review employer names and role-specific claims, then save as a new letter for this opportunity.");
  }, [previewLetter, previewJob, dirty, draft, snapshot]);

  const saveNewFromPrior = useCallback(async () => {
    if (newDraft.trim() === "" || !newDraftSource) return;
    setNewDraftError("");
    setSaving(true);
    try {
      const result = await createManualLetter(jobId, newDraft, {
        prior_letter_id: newDraftSource.id,
        prior_job_id: newDraftSource.jobId,
        prior_body_hash: newDraftSource.hash,
      });
      setNewDraft("");
      setNewDraftSource(null);
      setPreviewLetter(null);
      setPickerOpen(false);
      onMaterialSaved();
      await reload(result.draftId);
      setSaveNotice("New letter saved for this opportunity with a new identity. The original is unchanged.");
    } catch (error) {
      setNewDraftError(error instanceof Error ? error.message : "Unable to save the new letter");
    } finally {
      setSaving(false);
    }
  }, [newDraft, newDraftSource, jobId, onMaterialSaved, reload]);

  if (loading) return <p role="status">Loading cover letters…</p>;
  if (loadError !== "") {
    return (
      <section className="panel" aria-label="Cover letter loading error">
        <p>{loadError}</p>
        <button className="button button-secondary" type="button" onClick={() => void reload()}>
          Retry
        </button>
      </section>
    );
  }
  if (!snapshot) return <></>;

  const currentStale = snapshot.current?.evidenceStale ?? false;
  const selectedOption = resumeOptions.find((option) => option.id === selectedResumeId);
  const currentResumeLabel = selectedOption ? `${selectedOption.name} · Version ${selectedOption.version}` : "No résumé selected";

  return (
    <section className="panel" aria-labelledby="letters-title" id="cover-letters">
      <div className="section-heading">
        <div>
          <h2 id="letters-title">Cover letter</h2>
          <p className="section-subtitle">Saved letter for this opportunity, or start a new one from a blank page or a prior letter.</p>
        </div>
      </div>

      {!snapshot.sources.ready && (
        <p className="writing-notice" role="note">
          {!snapshot.sources.jobDescriptionReady
            ? "Add a saved job description before generating a cover letter."
            : "Generation needs a readable master résumé and an active role-specific résumé. " +
              `${snapshot.sources.master.message} ${snapshot.sources.role.message}`}
        </p>
      )}

      <div className="letter-start-choices" aria-label="Starting choices">
        <button className="button button-secondary" type="button" onClick={() => { setPickerOpen(false); setPreviewLetter(null); }}>
          Blank
        </button>
        <button className="button button-secondary" type="button" onClick={openPicker} aria-expanded={pickerOpen}>
          Use prior letter
        </button>
      </div>

      {snapshot.current && (
        <div className="writing-editor">
          <h3>Saved letter for this opportunity</h3>
          <p className="small-muted">
            {snapshot.current.manualSource ? "Manual — no résumé source" : `Saved résumé: ${resumeLabel(snapshot.current)}`} · Current application résumé: {currentResumeLabel}
            {snapshot.current.originRecorded && ` · Started from letter ${snapshot.current.originSourceLetterId}${snapshot.current.originEmployer !== "" ? ` (${snapshot.current.originEmployer})` : ""}`}
          </p>
          {currentStale && (
            <p className="writing-notice" role="note">
              The Career Evidence behind this draft changed since it was written. The text is unchanged; review the
              current records before reusing its claims.
            </p>
          )}
          {snapshot.current.evidenceRefs.length > 0 && (
            <p className="small-muted">
              Written from: {snapshot.current.evidenceRefs.map((ref) => `${ref.title} (revision ${ref.revision})`).join("; ")}
            </p>
          )}
          <div className="field field-full">
            <label htmlFor="letter-body">Letter text</label>
            <textarea
              id="letter-body"
              rows={14}
              value={draft}
              readOnly={!snapshot.editingAllowed || readOnly}
              onChange={(event) => {
                setDraft(event.target.value);
                setDirty(event.target.value !== (snapshot.current?.body ?? ""));
              }}
            />
          </div>
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
            {dirty && <button className="button button-secondary" type="button" onClick={() => { setDraft(snapshot.current?.body ?? ""); setDirty(false); setSaveError(""); }}>Discard edits</button>}
            <button
              className="button button-primary"
              type="button"
              disabled={saving || !dirty || (!snapshot.editingAllowed || readOnly)}
              onClick={() => void handleSave()}
            >
              {saving ? "Saving…" : "Save draft edits"}
            </button>
          </div>
          {(!snapshot.editingAllowed || readOnly) && (
            <p className="small-muted">This application is no longer active. Drafts are read-only.</p>
          )}
          {dirty && <p className="small-muted">Save or discard your edits before opening another draft.</p>}
        </div>
      )}

      <div className="field">
        <label htmlFor="letter-history">Saved letters for this opportunity</label>
        <select id="letter-history" value={historySelectedId} onChange={(event) => void openDraft(Number(event.target.value))}>
          <option value={0} disabled>Choose a saved letter…</option>
          {historyEntries.map((item) => (
            <option key={item.id} value={item.id}>
              Letter {item.id} · {item.provider} · {item.editedAt !== "" ? item.editedAt : item.createdAt}{item.current ? " · selected" : ""}
            </option>
          ))}
        </select>
        <small>Preview is separate from selection. Opening here changes the selected letter for this application.</small>
      </div>
      <div className="form-actions">
        <button className="button button-secondary" type="button" disabled={!historyHasMore} onClick={() => void fetchLetterHistory(jobId, historyCursor, 20).then((page) => { setHistoryEntries((prev) => [...prev, ...page.entries]); setHistoryCursor(page.nextCursor); setHistoryHasMore(page.hasMore); })}>
          Show older letters
        </button>
        {historyEntries.length > 0 && historySelectedId > 0 && !historyEntries.some((entry) => entry.id === historySelectedId) && (
          <small className="small-muted">Selected letter {historySelectedId} is retained even though it is older than this page.</small>
        )}
      </div>
      {historySelectedId > 0 && (
        <div className="form-actions">
          <button className="button button-secondary" type="button" onClick={() => void previewHistoryDraft(historySelectedId)}>
            Preview selected without editing
          </button>
        </div>
      )}

      {snapshot.drafts.length === 0 && !pickerOpen && (
        <div className="empty-state">
          <h3>No cover-letter drafts yet</h3>
          <p>Write one below or start from a prior letter.</p>
        </div>
      )}

      {pickerOpen && (
        <div className="prior-letter-picker" aria-label="Use prior letter">
          <h3>Use prior letter as starting draft</h3>
          <p className="small-muted">Browse every retained letter. Preview is read-only; Use creates an unsaved new draft for this opportunity.</p>
          <form
            onSubmit={(event) => {
              event.preventDefault();
              void runCatalog(0, catalogQuery, false);
            }}
          >
            <label htmlFor="prior-letter-search">Search saved letters</label>
            <div className="browse-search-row">
              <input id="prior-letter-search" value={catalogQuery} onChange={(event) => setCatalogQuery(event.target.value)} placeholder="Employer or role" />
              <button className="button button-secondary" type="submit" disabled={catalogBusy}>{catalogBusy ? "Searching…" : "Search"}</button>
            </div>
          </form>
          {catalogError !== "" && <p className="job-form-error" role="alert">{catalogError}</p>}
          {catalogEmpty && <p className="small-muted">No saved letters yet. Write a blank letter to start.</p>}
          <ul className="writing-suggestions">
            {catalogEntries.map((entry) => (
              <li key={entry.id}>
                <strong>{entry.employer !== "" ? entry.employer : "Saved opportunity"} · {entry.jobTitle}</strong>
                <small>{entryLabel(entry)} · {entry.editedAt !== "" ? entry.editedAt : entry.createdAt}</small>
                <p>{entry.preview}</p>
                <div className="form-actions">
                  <button className="button button-secondary" type="button" onClick={() => void previewCatalogLetter(entry)}>
                    Preview
                  </button>
                </div>
              </li>
            ))}
          </ul>
          {catalogHasMore && (
            <button className="button button-secondary" type="button" disabled={catalogBusy} onClick={() => void runCatalog(catalogCursor, catalogQuery, true)}>
              Show older letters
            </button>
          )}
          {previewBusy && <p role="status">Loading preview…</p>}
          {previewError !== "" && <p className="job-form-error" role="alert">{previewError}</p>}
          {previewLetter && (
            <div className="writing-preview" aria-label="Prior letter preview">
              <h4>Original opportunity and résumé source</h4>
              <p className="small-muted">
                {previewJob ? `${previewJob.employer} · ${previewJob.title}` : `Letter ${previewLetter.id}`} · {resumeLabel(previewLetter)} · {previewLetter.createdAt}
              </p>
              <p className="writing-answer-preview">{previewLetter.body}</p>
              <div className="form-actions">
                <button className="button button-primary" type="button" onClick={usePreviewAsDraft}>Use as starting draft</button>
                <button className="button button-secondary" type="button" onClick={() => { setPreviewLetter(null); setPreviewJob(null); }}>Close preview</button>
              </div>
            </div>
          )}
          {newDraftSource && (
            <div className="writing-editor" aria-label="New letter draft">
              <h4>New draft for this opportunity</h4>
              <p className="small-muted">From letter {newDraftSource.id} ({newDraftSource.label}). Current résumé stays {currentResumeLabel}. Saving creates a new manual letter; the original is unchanged.</p>
              <div className="field field-full">
                <label htmlFor="prior-new-body">New letter text</label>
                <textarea id="prior-new-body" rows={14} value={newDraft} onChange={(event) => setNewDraft(event.target.value)} readOnly={!snapshot.editingAllowed || readOnly} />
              </div>
              {newDraftError !== "" && <p className="job-form-error" role="alert">{newDraftError}</p>}
              {newDraftNotice !== "" && <p className="form-notice" role="status">{newDraftNotice}</p>}
              <div className="form-actions">
                <button className="button button-secondary" type="button" onClick={() => { setNewDraft(""); setNewDraftSource(null); }}>Discard new draft</button>
                <button className="button button-primary" type="button" disabled={saving || newDraft.trim() === ""} onClick={() => void saveNewFromPrior()}>
                  {saving ? "Saving…" : "Save new letter for this opportunity"}
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {snapshot.editingAllowed && !readOnly && !pickerOpen && (
        <details className="panel detail-editor" open={snapshot.drafts.length === 0}>
          <summary><strong>Write a letter</strong></summary>
          <div className="editor-content field">
            <label htmlFor="manual-letter">Your letter</label>
            <textarea id="manual-letter" rows={10} value={manualDraft} onChange={(event) => setManualDraft(event.target.value)} />
            <button className="button button-primary" type="button" disabled={saving || dirty || manualDraft.trim() === ""} onClick={() => void handleManual()}>Save new letter</button>
            {saveError !== "" && <p role="alert">{saveError}</p>}
          </div>
        </details>
      )}

      {snapshot.editingAllowed && !readOnly && (
        <details className="panel detail-editor">
          <summary>
            <span>
              <strong>Generate a new draft</strong>
              <small>Uses your selected résumé, master résumé, and up to two reviewed experiences.</small>
            </span>
            <span className="summary-chevron" aria-hidden="true">
              ⌄
            </span>
          </summary>
          <div className="editor-content">
            <div className="field">
              <label htmlFor="letter-provider">Provider</label>
              <select id="letter-provider" value={genProvider} onChange={(event) => setGenProvider(event.target.value)}>
                {snapshot.providers.map((option) => (
                  <option key={option.ID} value={option.ID}>
                    {option.Label}
                  </option>
                ))}
              </select>
              <small>{snapshot.providers.find((option) => option.ID === genProvider)?.Help}</small>
            </div>
            <div className="field">
              <label htmlFor="letter-model">Model (blank for the provider default)</label>
              <input
                id="letter-model"
                value={genModel}
                placeholder={snapshot.defaultModel}
                onChange={(event) => setGenModel(event.target.value)}
              />
            </div>
            <div className="field">
              <label htmlFor="letter-resume">Role résumé for this letter (defaults to the workspace selection)</label>
              <select
                id="letter-resume"
                value={genResumeId}
                onChange={(event) => setGenResumeId(event.target.value)}
              >
                {resumeOptions
                  .filter((option) => !option.archived)
                  .map((option) => (
                    <option key={option.id} value={option.id}>
                      {option.name} · {option.kind} · Version {option.version}
                    </option>
                  ))}
              </select>
            </div>
            <fieldset className="writing-sources">
              <legend>Reviewed Career Evidence (up to 2, optional)</legend>
              {snapshot.evidenceChoices.map((choice) => (
                <label key={choice.Identity} className="confirm-row">
                  <input
                    type="checkbox"
                    checked={genEvidence.includes(choice.Identity)}
                    disabled={genEvidence.length >= 2 && !genEvidence.includes(choice.Identity)}
                    onChange={(event) =>
                      setGenEvidence((prev) =>
                        event.target.checked ? [...prev, choice.Identity] : prev.filter((id) => id !== choice.Identity),
                      )
                    }
                  />
                  <span>{choice.Label}</span>
                </label>
              ))}
              {snapshot.evidenceChoices.length === 0 && (
                <p className="small-muted">No reviewed evidence yet — the letter will use your résumés only.</p>
              )}
            </fieldset>
            {genError !== "" && (
              <p className="job-form-error" role="alert">
                {genError}
              </p>
            )}
            {genNotice !== "" && (
              <p className="form-notice" role="status">
                {genNotice}
              </p>
            )}
            <div className="form-actions">
              <button className="button button-primary" type="button" disabled={genBusy} onClick={() => void handleGenerate()}>
                {genBusy ? "Generating…" : "Generate new draft"}
              </button>
            </div>
          </div>
        </details>
      )}
    </section>
  );
}
