// YM-004B cover letters inside the React Application Workspace.
// Generation, validation, and persistence stay Go-owned. Each generation
// creates a new draft; editing a draft never rewrites another version.
// Failures leave every saved draft untouched.

import { useCallback, useEffect, useState } from "react";
import {
  createManualLetter,
  fetchLetterDraft,
  fetchLetters,
  generateLetter,
  saveLetterDraft,
  selectLetterDraft,
  type LettersSnapshot,
} from "./writingApi";

export interface LetterResumeOption {
  id: number;
  name: string;
  kind: string;
  version: number;
  archived: boolean;
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
        } else {
          setDraft("");
        }
        setDirty(false);
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
    if (!dirty && manualDraft === "") return;
    const guard = (event: BeforeUnloadEvent) => {
      event.preventDefault();
    };
    window.addEventListener("beforeunload", guard);
    return () => window.removeEventListener("beforeunload", guard);
  }, [dirty, manualDraft]);

  const openDraft = useCallback(
    async (id: number) => {
      if (id === selectedId) return;
      if (dirty) {
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
        setDirty(false);
        setSnapshot((prev) => {
          if (!prev) return prev;
          return { ...prev, current: opened };
        });
      } catch (error) {
        setSaveError(error instanceof Error ? error.message : "Unable to open the draft");
      }
    },
    [dirty, selectedId, jobId, onMaterialSaved, readOnly],
  );

  const handleSave = useCallback(async () => {
    if (selectedId === 0) return;
    setSaveError("");
    setSaveNotice("");
    setSaving(true);
    try {
      await saveLetterDraft(jobId, selectedId, draft);
      setDirty(false);
      setSaveNotice("Draft saved exactly as edited.");
      onMaterialSaved();
      await reload(selectedId);
    } catch (error) {
      setSaveError(error instanceof Error ? error.message : "Unable to save the draft");
    } finally {
      setSaving(false);
    }
  }, [selectedId, jobId, draft, reload, onMaterialSaved]);

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
  }, [jobId, genProvider, genModel, genResumeId, genEvidence, confirmExternal, confirmLocal, reload, onMaterialSaved]);

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

  return (
    <section className="panel" aria-labelledby="letters-title" id="cover-letters">
      <div className="section-heading">
        <div>
          <p className="eyebrow">COVER LETTERS</p>
          <h2 id="letters-title">Prepare your cover letter</h2>
          <p className="section-subtitle">
            Write your own letter or optionally generate a draft, then review and save the exact text.
          </p>
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

      {snapshot.drafts.length > 0 && (
        <div className="field">
          <label htmlFor="letter-draft">Selected cover letter</label>
          <select id="letter-draft" value={selectedId} onChange={(event) => void openDraft(Number(event.target.value))}>
            <option value={0} disabled>Choose a draft…</option>
            {snapshot.drafts.map((item) => (
              <option key={item.id} value={item.id}>
                Draft {item.id} · {item.provider} · {item.editedAt !== "" ? item.editedAt : item.createdAt}
              </option>
            ))}
          </select>
        </div>
      )}

      {snapshot.current && (
        <div className="writing-editor">
          <h3>
            Draft {snapshot.current.id} · {snapshot.current.provider}
          </h3>
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

      {snapshot.drafts.length === 0 && (
        <div className="empty-state">
          <h3>No cover-letter drafts yet</h3>
          <p>Write one below or return later.</p>
        </div>
      )}

      {snapshot.editingAllowed && !readOnly && (
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
            <p className="small-muted">
              Sources: master {snapshot.sources.master.ready ? "readable" : "unavailable"} · role résumé{" "}
              {snapshot.sources.role.ready ? "readable" : "unavailable"}.
            </p>
            <label className="confirm-row">
              <input
                type="checkbox"
                checked={confirmExternal}
                onChange={(event) => setConfirmExternal(event.target.checked)}
              />
              <span>I authorize transmitting my materials to the provider for this specific letter</span>
            </label>
            {genProvider === "ollama" && (
              <label className="confirm-row">
                <input type="checkbox" checked={confirmLocal} onChange={(event) => setConfirmLocal(event.target.checked)} />
                <span>Local-only handling acknowledged for Ollama</span>
              </label>
            )}
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
