// S2 previous-writing region: visible related sources with full preview,
// explicit use as starting draft, and in-context library browsing.
// React owns presentation; Go validates every use before text enters the editor.

import { useCallback, useEffect, useRef, useState } from "react";
import {
  browseLibrary,
  fetchLibraryExample,
  fetchManualStartingPoint,
  fetchStartingPoint,
  type LibraryBrowseEntry,
  type LibraryBrowseResult,
  type WritingQuestion,
} from "./writingApi";

export interface PendingSource {
  entryId: number;
  revision: number;
  label: string;
}

function sourceLabel(entry: { question: string; employer: string; jobTitle: string }): string {
  const context = [entry.employer, entry.jobTitle].filter((part) => part !== "").join(" · ");
  return context !== "" ? `${entry.question} — ${context}` : entry.question;
}

export function PreviousWriting({
  jobId,
  destination,
  question,
  baselineRevision,
  disabled,
  onUseAsDraft,
}: {
  jobId: number;
  destination: string;
  question: WritingQuestion;
  baselineRevision: number;
  disabled: boolean;
  onUseAsDraft: (answer: string, origin: PendingSource | null, sourceLabel: string) => void;
}): React.JSX.Element {
  const [previewId, setPreviewId] = useState<number>(0);
  const [previewRevision, setPreviewRevision] = useState<number>(0);
  const [previewTitle, setPreviewTitle] = useState<string>("");
  const [previewBody, setPreviewBody] = useState<string>("");
  const [previewLoading, setPreviewLoading] = useState<boolean>(false);
  const [previewError, setPreviewError] = useState<string>("");
  const [useBusy, setUseBusy] = useState<boolean>(false);
  const [useError, setUseError] = useState<string>("");
  const [browseOpen, setBrowseOpen] = useState<boolean>(false);
  const [browseQuery, setBrowseQuery] = useState<string>("");
  const [browseResult, setBrowseResult] = useState<LibraryBrowseResult | null>(null);
  const [browseLoading, setBrowseLoading] = useState<boolean>(false);
  const [browseError, setBrowseError] = useState<string>("");
  const requestRef = useRef(0);

  // Switching questions clears source selection but preserves the browse query.
  useEffect(() => {
    requestRef.current++;
    setPreviewId(0);
    setPreviewRevision(0);
    setPreviewTitle("");
    setPreviewBody("");
    setPreviewError("");
    setUseError("");
  }, [question.key, question.isEarly]);

  const openPreview = useCallback(
    async (entryId: number, revision: number, title: string) => {
      const request = ++requestRef.current;
      setPreviewId(entryId);
      setPreviewRevision(revision);
      setPreviewTitle(title);
      setPreviewBody("");
      setPreviewError("");
      setUseError("");
      setPreviewLoading(true);
      try {
        const full = await fetchLibraryExample(entryId);
        if (request !== requestRef.current) return;
        setPreviewBody(full.answer);
        setPreviewRevision(full.revision);
        setPreviewTitle(full.original_question);
      } catch (error) {
        if (request !== requestRef.current) return;
        setPreviewError(error instanceof Error ? error.message : "Unable to load the full example");
      } finally {
        if (request === requestRef.current) setPreviewLoading(false);
      }
    },
    [],
  );

  const closePreview = useCallback(() => {
    requestRef.current++;
    setPreviewId(0);
    setPreviewRevision(0);
    setPreviewTitle("");
    setPreviewBody("");
    setPreviewError("");
    setUseError("");
    setUseBusy(false);
  }, []);

  const handleUse = useCallback(async () => {
    if (previewId === 0 || previewBody === "") return;
    setUseBusy(true);
    setUseError("");
    try {
      // Manual questions validate against their application-owned key; the
      // returned text is still an unsaved draft, never a transfer.
      if (question.isManual) {
        const validated = await fetchManualStartingPoint(jobId, question.key, {
          library_entry_id: previewId,
          library_revision: previewRevision,
          revision: baselineRevision,
          destination,
        });
        onUseAsDraft(validated.answer, { entryId: validated.library_entry_id, revision: validated.library_revision, label: previewTitle }, previewTitle);
      } else {
        const validated = await fetchStartingPoint(jobId, {
          library_entry_id: previewId,
          library_revision: previewRevision,
          question_key: question.key,
          revision: baselineRevision,
          destination,
          early: question.isEarly,
          discovery_source: question.discoverySource,
          discovery_fingerprint: question.discoveryFingerprint,
          source_question_id: question.sourceQuestionId,
        });
        onUseAsDraft(validated.answer, { entryId: validated.entry_id, revision: validated.library_revision, label: previewTitle }, previewTitle);
      }
    } catch (error) {
      setUseError(error instanceof Error ? error.message : "Unable to use this example");
    } finally {
      setUseBusy(false);
    }
  }, [previewId, previewBody, previewRevision, previewTitle, jobId, question, baselineRevision, destination, onUseAsDraft]);

  const runBrowse = useCallback(
    async (cursor: number, query: string, append: boolean) => {
      const request = ++requestRef.current;
      setBrowseLoading(true);
      setBrowseError("");
      try {
        const result = await browseLibrary({ query: query.trim() === "" ? undefined : query.trim(), cursor, limit: 20 });
        if (request !== requestRef.current) return;
        setBrowseResult((previous) => {
          if (append && previous && previous.query === result.query) {
            return { ...result, entries: [...previous.entries, ...result.entries] };
          }
          return result;
        });
      } catch (error) {
        if (request !== requestRef.current) return;
        setBrowseError(error instanceof Error ? error.message : "Unable to browse saved writing");
      } finally {
        if (request === requestRef.current) setBrowseLoading(false);
      }
    },
    [],
  );

  useEffect(() => {
    if (browseOpen && browseResult === null && !browseLoading) void runBrowse(0, browseQuery, false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [browseOpen]);

  const suggestions = question.librarySuggestions ?? [];
  const showBrowseButton = question.editable;

  return (
    <div className="previous-writing" aria-label="Previous writing">
      <h4>Previous writing</h4>
      {suggestions.length === 0 && (
        <p className="small-muted">No ranked suggestions for this question. Browse all saved answers to look manually.</p>
      )}
      {suggestions.length > 0 && (
        <ul className="writing-suggestions">
          {suggestions.map((s) => (
            <li key={s.identity}>
              <strong>{s.question}</strong>
              <small>
                {[s.employer, s.jobTitle].filter((part) => part !== "").join(" · ")}
                {s.employer !== "" || s.jobTitle !== "" ? " · " : ""}revision {s.revision} · {s.matchKind || "Related — review for this question"}
              </small>
              <p>{s.excerpt}</p>
              <div className="form-actions">
                <button
                  className="button button-secondary"
                  type="button"
                  disabled={disabled}
                  onClick={() => void openPreview(s.entryId, s.revision, sourceLabel({ question: s.question, employer: s.employer, jobTitle: s.jobTitle }))}
                >
                  Preview full text
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}
      {previewId !== 0 && (
        <div className="writing-preview" aria-label="Full source preview">
          <h5>{previewTitle !== "" ? previewTitle : "Full example"}</h5>
          {previewLoading && <p role="status">Loading the complete example…</p>}
          {previewError !== "" && (
            <p className="job-form-error" role="alert">
              {previewError} The editor is unchanged.
            </p>
          )}
          {!previewLoading && previewError === "" && (
            <>
              <p className="writing-answer-preview">{previewBody}</p>
              <p className="small-muted">Using this places the complete validated text in your editor as an unsaved draft. It does not approve exact reuse.</p>
              {useError !== "" && (
                <p className="job-form-error" role="alert">
                  {useError}
                </p>
              )}
              <div className="form-actions">
                <button className="button button-primary" type="button" disabled={disabled || useBusy || previewBody === ""} onClick={() => void handleUse()}>
                  {useBusy ? "Checking…" : "Use as starting draft"}
                </button>
                <button className="button button-secondary" type="button" onClick={closePreview}>
                  Close preview
                </button>
              </div>
            </>
          )}
        </div>
      )}
      {showBrowseButton && (
        <div className="browse-toggle">
          <button className="button button-secondary" type="button" onClick={() => setBrowseOpen((open) => !open)} aria-expanded={browseOpen}>
            {browseOpen ? "Hide browsing" : "Browse all"}
          </button>
        </div>
      )}
      {browseOpen && (
        <div className="library-browse" aria-label="Browse all saved answers">
          <form
            className="browse-search"
            onSubmit={(event) => {
              event.preventDefault();
              void runBrowse(0, browseQuery, false);
            }}
          >
            <label htmlFor="library-search">Search saved answers</label>
            <div className="browse-search-row">
              <input
                id="library-search"
                value={browseQuery}
                onChange={(event) => setBrowseQuery(event.target.value)}
                placeholder="Employer, role, or question wording"
              />
              <button className="button button-secondary" type="submit" disabled={browseLoading}>
                {browseLoading ? "Searching…" : "Search"}
              </button>
            </div>
          </form>
          {browseError !== "" && (
            <p className="job-form-error" role="alert">
              {browseError} Your editor is unchanged.
            </p>
          )}
          {browseResult && browseResult.libraryEmpty && <p className="small-muted">The Answer Library is empty. Choose “Save to Answer Library” on a narrative question to reuse it here.</p>}
          {browseResult && !browseResult.libraryEmpty && browseResult.entries.length === 0 && (
            <p className="small-muted">
              {browseResult.query !== "" ? `No ranked matches for “${browseResult.query}”. Try different wording or browse the full list.` : "No saved answers yet."}
            </p>
          )}
          {browseResult && browseResult.entries.length > 0 && (
            <>
              {browseResult.bounded && (
                <p className="small-muted" role="note">
                  Showing up to {browseResult.bound ?? 3} ranked matches. Browse without a query to page every saved example.
                </p>
              )}
              <ul className="writing-suggestions">
                {browseResult.entries.map((entry: LibraryBrowseEntry) => (
                  <li key={entry.id}>
                    <strong>{entry.question}</strong>
                    <small>
                      {[entry.employer, entry.jobTitle].filter((part) => part !== "").join(" · ")}
                      {entry.sourceKind === "early" ? " · early example" : ""} · revision {entry.revision}
                    </small>
                    <p>{entry.excerpt}</p>
                    <div className="form-actions">
                      <button
                        className="button button-secondary"
                        type="button"
                        disabled={disabled}
                        onClick={() => void openPreview(entry.id, entry.revision, sourceLabel({ question: entry.question, employer: entry.employer, jobTitle: entry.jobTitle }))}
                      >
                        Preview full text
                      </button>
                    </div>
                  </li>
                ))}
              </ul>
              {browseResult.hasMore && (
                <button
                  className="button button-secondary"
                  type="button"
                  disabled={browseLoading}
                  onClick={() => void runBrowse(browseResult.nextCursor, browseResult.query, true)}
                >
                  {browseLoading ? "Loading…" : "Show older examples"}
                </button>
              )}
            </>
          )}
        </div>
      )}
    </div>
  );
}
