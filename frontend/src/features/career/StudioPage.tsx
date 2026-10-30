import { useCallback, useEffect, useMemo, useState } from "react";
import { CareerApiError, fetchCareerProviders, type CareerProvider } from "./api";
import { CareerNav } from "./CareerNav";
import { formatDateTime, shortHash, statusLabel } from "./format";
import {
  authorizeStudioRole,
  authorizeStudioSynthesis,
  challengeStudioConnection,
  createStudioEvidence,
  deleteStudioEvidence,
  fetchCareerActivation,
  fetchStudioEvidence,
  fetchStudioEvidenceRecord,
  fetchStudioRoles,
  fetchStudioRoleSnapshot,
  fetchStudioSynthesis,
  reopenStudioEvidence,
  reviewStudioEvidence,
  saveStudioEvidence,
  type StudioActivationRecord,
  type StudioEvidenceContent,
  type StudioEvidenceRecord,
  type StudioRolesResponse,
  type StudioRoleSnapshot,
  type StudioSynthesisResponse,
} from "./studioApi";

const emptyContent: StudioEvidenceContent = {
  title: "",
  situation: "",
  actions: "",
  employer_context: "",
  role_context: "",
  technologies: "",
  outcome: "",
  outcome_state: "not_recorded",
  period: "",
  source_notes: "",
};

function contentFromRecord(record: StudioEvidenceRecord): StudioEvidenceContent {
  return {
    title: record.title,
    situation: record.situation,
    actions: record.actions,
    employer_context: record.employer_context,
    role_context: record.role_context,
    technologies: record.technologies,
    outcome: record.outcome,
    outcome_state: record.outcome_state,
    period: record.period,
    source_notes: record.source_notes,
  };
}

export function StudioPage(): React.JSX.Element {
  const [evidence, setEvidence] = useState<StudioEvidenceRecord[]>([]);
  const [filter, setFilter] = useState<"all" | "draft" | "reviewed">("all");
  const [record, setRecord] = useState<StudioEvidenceRecord | null>(null);
  const [fields, setFields] = useState<StudioEvidenceContent>(emptyContent);
  const [confirmReview, setConfirmReview] = useState<boolean>(false);
  const [confirmReopen, setConfirmReopen] = useState<boolean>(false);
  const [confirmDelete, setConfirmDelete] = useState<boolean>(false);

  const [rolesData, setRolesData] = useState<StudioRolesResponse | null>(null);
  const [roleKey, setRoleKey] = useState<string>("");
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [snapshot, setSnapshot] = useState<StudioRoleSnapshot | null>(null);

  const [synthesis, setSynthesis] = useState<StudioSynthesisResponse | null>(null);
  const [challengeDrafts, setChallengeDrafts] = useState<Record<string, { state: string; note: string }>>({});
  const [activation, setActivation] = useState<StudioActivationRecord[]>([]);

  const [providers, setProviders] = useState<CareerProvider[]>([]);
  const [providerId, setProviderId] = useState<string>("");
  const [model, setModel] = useState<string>("");
  const [roleAckGen, setRoleAckGen] = useState<boolean>(false);
  const [roleAckExt, setRoleAckExt] = useState<boolean>(false);
  const [synAckGen, setSynAckGen] = useState<boolean>(false);
  const [synAckExt, setSynAckExt] = useState<boolean>(false);

  const [loading, setLoading] = useState<boolean>(true);
  const [loadError, setLoadError] = useState<string>("");
  const [notice, setNotice] = useState<string>("");
  const [actionError, setActionError] = useState<string>("");
  const [busy, setBusy] = useState<boolean>(false);

  const refreshAll = useCallback(async () => {
    const [loadedEvidence, loadedRoles, loadedSynthesis, loadedActivation] = await Promise.all([
      fetchStudioEvidence(),
      fetchStudioRoles(),
      fetchStudioSynthesis(),
      fetchCareerActivation(),
    ]);
    setEvidence(loadedEvidence);
    setRolesData(loadedRoles);
    setSynthesis(loadedSynthesis);
    setActivation(loadedActivation);
  }, []);

  const openRecord = useCallback(async (id: number) => {
    const loaded = await fetchStudioEvidenceRecord(id);
    setRecord(loaded);
    setFields(contentFromRecord(loaded));
    setConfirmReview(false);
    setConfirmReopen(false);
    setConfirmDelete(false);
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      setLoadError("");
      try {
        const loadedProviders = await fetchCareerProviders();
        if (cancelled) return;
        setProviders(loadedProviders);
        const firstConfigured = loadedProviders.find((provider) => provider.configured);
        const initial = firstConfigured ?? loadedProviders[0];
        if (initial) {
          setProviderId(initial.id);
          setModel(initial.default_model);
        }
        await refreshAll();
        if (cancelled) return;
        const params = new URLSearchParams(window.location.search);
        const requestedRole = params.get("role");
        if (requestedRole) setRoleKey(requestedRole);
        const requestedEvidence = params.get("evidence");
        if (requestedEvidence) {
          const id = Number.parseInt(requestedEvidence, 10);
          if (Number.isInteger(id) && id > 0) await openRecord(id);
        }
      } catch (error) {
        if (!cancelled) setLoadError(error instanceof Error ? error.message : "Unable to load Career Studio");
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [openRecord, refreshAll]);

  const handleProviderChange = useCallback(
    (id: string) => {
      setProviderId(id);
      const provider = providers.find((candidate) => candidate.id === id);
      if (provider) setModel(provider.default_model);
    },
    [providers],
  );

  const fail = useCallback((label: string, error: unknown) => {
    setActionError(error instanceof Error ? `${label}: ${error.message}` : `${label} failed`);
  }, []);

  const evidenceTitles = useMemo(() => {
    const map = new Map<number, string>();
    for (const entry of evidence) map.set(entry.id, entry.title === "" ? `Record #${entry.id}` : entry.title);
    for (const ref of rolesData?.reviewed ?? []) {
      if (!map.has(ref.id)) map.set(ref.id, ref.title);
    }
    return map;
  }, [evidence, rolesData]);

  const synthesisTitles = useMemo(() => {
    const map = new Map(evidenceTitles);
    for (const ref of synthesis?.eligible ?? []) {
      if (!map.has(ref.id)) map.set(ref.id, ref.title);
    }
    return map;
  }, [evidenceTitles, synthesis]);

  const visibleEvidence = evidence.filter((entry) => filter === "all" || entry.status === filter);
  const reviewedCount = evidence.filter((entry) => entry.status === "reviewed").length;
  const activeRole = rolesData?.roles.find((role) => role.key === roleKey) ?? null;
  const activeProvider = providers.find((provider) => provider.id === providerId);
  const maxEvidence = rolesData?.max_evidences ?? 4;

  const setField = useCallback((key: keyof StudioEvidenceContent, value: string) => {
    setFields((current) => ({ ...current, [key]: value }));
  }, []);

  const toggleSelectedId = useCallback(
    (id: number) => {
      setSelectedIds((current) => {
        if (current.includes(id)) return current.filter((candidate) => candidate !== id);
        if (current.length >= maxEvidence) return current;
        return [...current, id].sort((a, b) => a - b);
      });
      setSnapshot(null);
    },
    [maxEvidence],
  );

  const handleCreate = useCallback(() => {
    setActionError("");
    setNotice("");
    setBusy(true);
    void (async () => {
      try {
        const created = await createStudioEvidence();
        await openRecord(created.id);
        await refreshAll();
        setNotice(`Draft #${created.id} created. Add what you remember, then save.`);
      } catch (error) {
        fail("Create", error);
      } finally {
        setBusy(false);
      }
    })();
  }, [fail, openRecord, refreshAll]);

  const handleSave = useCallback(
    (kind: "save" | "review") => {
      if (!record) return;
      if (kind === "review" && !confirmReview) {
        setActionError("Review: confirm the record accurately describes your experience.");
        return;
      }
      const id = record.id;
      const revision = record.revision;
      const content = fields;
      setActionError("");
      setNotice("");
      setBusy(true);
      void (async () => {
        try {
          const updated =
            kind === "review"
              ? await reviewStudioEvidence(id, revision, content)
              : await saveStudioEvidence(id, revision, content);
          setRecord(updated);
          setFields(contentFromRecord(updated));
          setConfirmReview(false);
          await refreshAll();
          setNotice(
            kind === "review"
              ? "Marked as reviewed by you. This is your confirmation, not independent verification."
              : updated.status === "draft" && record.status === "reviewed"
                ? "Saved. Editing returned this record to draft; review it again before comparing."
                : "Draft saved.",
          );
        } catch (error) {
          if (error instanceof CareerApiError && error.code === "changed") {
            try {
              await openRecord(id);
              await refreshAll();
              setNotice("This record changed elsewhere. Reloaded the current revision; review it before saving.");
              return;
            } catch {
              // Fall through to the original error.
            }
          }
          fail(kind === "review" ? "Review" : "Save", error);
        } finally {
          setBusy(false);
        }
      })();
    },
    [confirmReview, fail, fields, openRecord, record, refreshAll],
  );

  const handleReopen = useCallback(() => {
    if (!record || record.status !== "reviewed") return;
    if (!confirmReopen) {
      setActionError("Reopen: confirm this record needs more work.");
      return;
    }
    const id = record.id;
    const revision = record.revision;
    setActionError("");
    setNotice("");
    setBusy(true);
    void (async () => {
      try {
        const updated = await reopenStudioEvidence(id, revision);
        setRecord(updated);
        setFields(contentFromRecord(updated));
        setConfirmReopen(false);
        await refreshAll();
        setNotice("Reopened as a draft. Saved interpretations citing the reviewed revision are now flagged stale.");
      } catch (error) {
        if (error instanceof CareerApiError && error.code === "changed") {
          try {
            await openRecord(id);
            await refreshAll();
            setNotice("This record changed elsewhere. Reloaded the current revision.");
            return;
          } catch {
            // Fall through to the original error.
          }
        }
        fail("Reopen", error);
      } finally {
        setBusy(false);
      }
    })();
  }, [confirmReopen, fail, openRecord, record, refreshAll]);

  const handleDelete = useCallback(() => {
    if (!record) return;
    if (!confirmDelete) {
      setActionError("Delete: check the confirmation to delete this record.");
      return;
    }
    const id = record.id;
    const revision = record.revision;
    setActionError("");
    setNotice("");
    setBusy(true);
    void (async () => {
      try {
        await deleteStudioEvidence(id, revision);
        setRecord(null);
        setFields(emptyContent);
        setConfirmDelete(false);
        await refreshAll();
        setNotice(`Record #${id} deleted. Saved interpretations citing it are now flagged stale.`);
      } catch (error) {
        fail("Delete", error);
      } finally {
        setBusy(false);
      }
    })();
  }, [confirmDelete, fail, record, refreshAll]);

  const handleSnapshot = useCallback(() => {
    if (roleKey === "" || selectedIds.length === 0) {
      setActionError("Compare: choose a role and at least one reviewed record.");
      return;
    }
    setActionError("");
    setNotice("");
    setBusy(true);
    void (async () => {
      try {
        setSnapshot(await fetchStudioRoleSnapshot(roleKey, selectedIds));
        setRoleAckGen(false);
        setRoleAckExt(false);
      } catch (error) {
        fail("Review selection", error);
      } finally {
        setBusy(false);
      }
    })();
  }, [fail, roleKey, selectedIds]);

  const handleAuthorizeRole = useCallback(() => {
    if (!snapshot) return;
    if (providerId === "" || model.trim() === "") {
      setActionError("Authorize: choose a provider and model.");
      return;
    }
    if (!roleAckGen || !roleAckExt) {
      setActionError("Authorize: confirm both statements before the external request.");
      return;
    }
    const key = snapshot.key;
    const request = {
      provider: providerId,
      model: model.trim(),
      evidence_ids: snapshot.evidences.map((ref) => ref.id),
      input_hash: snapshot.input_hash,
    };
    setActionError("");
    setNotice("");
    setBusy(true);
    void (async () => {
      try {
        const result = await authorizeStudioRole(key, request);
        setSnapshot((current) => (current && current.key === key ? { ...current, saved: result.saved } : current));
        await refreshAll();
        setRoleAckGen(false);
        setRoleAckExt(false);
        setNotice("Comparison saved. It is a hypothesis for your judgment; nothing was adopted.");
      } catch (error) {
        if (error instanceof CareerApiError && error.code === "changed") {
          setSnapshot(null);
          await refreshAll();
          setNotice("The role or evidence changed. Review the current selection before authorizing again.");
          return;
        }
        fail("Authorize", error);
      } finally {
        setBusy(false);
      }
    })();
  }, [fail, model, providerId, refreshAll, roleAckExt, roleAckGen, snapshot]);

  const handleAuthorizeSynthesis = useCallback(() => {
    if (!synthesis || synthesis.input_hash === "") return;
    if (providerId === "" || model.trim() === "") {
      setActionError("Authorize: choose a provider and model.");
      return;
    }
    if (!synAckGen || !synAckExt) {
      setActionError("Authorize: confirm both statements before the external request.");
      return;
    }
    const request = { provider: providerId, model: model.trim(), input_hash: synthesis.input_hash };
    setActionError("");
    setNotice("");
    setBusy(true);
    void (async () => {
      try {
        setSynthesis(await authorizeStudioSynthesis(request));
        setSynAckGen(false);
        setSynAckExt(false);
        await refreshAll();
        setNotice("Synthesis saved. Every connection is a hypothesis; challenge any interpretation you dispute.");
      } catch (error) {
        if (error instanceof CareerApiError && error.code === "changed") {
          await refreshAll();
          setNotice("The reviewed set changed. Review the current set before authorizing again.");
          return;
        }
        fail("Authorize", error);
      } finally {
        setBusy(false);
      }
    })();
  }, [fail, model, providerId, refreshAll, synAckExt, synAckGen, synthesis]);

  const handleChallenge = useCallback(
    (id: string) => {
      const draft = challengeDrafts[id] ?? { state: "", note: "" };
      setActionError("");
      setNotice("");
      setBusy(true);
      void (async () => {
        try {
          setSynthesis(await challengeStudioConnection(id, draft.state, draft.note));
          setChallengeDrafts((current) => {
            const next = { ...current };
            delete next[id];
            return next;
          });
          setNotice(
            draft.state === ""
              ? "Challenge cleared. The interpretation stands unchallenged."
              : "Challenge recorded. The interpretation stays visible with your judgment attached.",
          );
        } catch (error) {
          fail("Challenge", error);
        } finally {
          setBusy(false);
        }
      })();
    },
    [challengeDrafts, fail],
  );

  const savedRoleView = snapshot?.saved ?? activeRole?.saved ?? null;

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      <CareerNav active="studio" />
      <div className="app-main">
        <main id="main-content" className="page-container">
          <div className="page-heading">
            <div>
              <h1>Career Studio</h1>
              <p className="page-description">
                Review your experiences, compare them against saved Opportunities, and inspect what Muse infers across
                them. Interpretations are hypotheses: your corrections flag them stale, and your challenges stay
                attached.
              </p>
            </div>
            <div className="heading-actions">
              <a className="button button-secondary" href="/career">
                Open discovery interviews
              </a>
              <span className="summary-count">
                {reviewedCount} {reviewedCount === 1 ? "record" : "records"} reviewed
              </span>
            </div>
          </div>

          {loading && (
            <section className="panel" aria-label="Loading">
              <p className="section-subtitle">Loading Career Studio…</p>
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
              <section className="panel" aria-labelledby="library-title">
                <div className="career-section-heading">
                  <h2 id="library-title">Evidence library</h2>
                  <p className="section-subtitle">
                    Browse drafts and reviewed records. Only reviewed records with a title, situation, and actions can
                    be compared or synthesized.
                  </p>
                </div>
                <div className="studio-library-bar">
                  <div className="field">
                    <label htmlFor="studio-filter">Show</label>
                    <select
                      id="studio-filter"
                      value={filter}
                      onChange={(event) => setFilter(event.target.value as "all" | "draft" | "reviewed")}
                      disabled={busy}
                    >
                      <option value="all">All records</option>
                      <option value="draft">Drafts</option>
                      <option value="reviewed">Reviewed</option>
                    </select>
                  </div>
                  <div className="form-actions">
                    <button className="button button-primary" type="button" onClick={handleCreate} disabled={busy}>
                      New draft
                    </button>
                  </div>
                </div>
                {visibleEvidence.length === 0 ? (
                  <p className="section-subtitle">No records here yet.</p>
                ) : (
                  <ul className="career-session-list">
                    {visibleEvidence.map((entry) => (
                      <li key={entry.id} className={record?.id === entry.id ? "is-active" : ""}>
                        <div>
                          <strong>{entry.title === "" ? `Record #${entry.id}` : entry.title}</strong>
                          <span className="career-session-meta">
                            {statusLabel(entry.status)} · revision {entry.revision} · updated{" "}
                            {formatDateTime(entry.updated_at)}
                          </span>
                        </div>
                        <button
                          className="button button-secondary"
                          type="button"
                          onClick={() => void openRecord(entry.id).catch((error: unknown) => fail("Open", error))}
                          disabled={busy || record?.id === entry.id}
                        >
                          {record?.id === entry.id ? "Current" : "Open"}
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
              </section>

              {record && (
                <section className="panel" aria-labelledby="editor-title">
                  <div className="career-section-heading">
                    <h2 id="editor-title">
                      {record.title === "" ? `Record #${record.id}` : record.title} · {statusLabel(record.status)} ·
                      revision {record.revision}
                    </h2>
                    <p className="section-subtitle">
                      Saved locally on every action. Saving changes to a reviewed record returns it to draft.
                    </p>
                  </div>
                  <div className="career-start-grid">
                    <div className="field field-full">
                      <label htmlFor="studio-title">Title</label>
                      <input
                        id="studio-title"
                        value={fields.title}
                        onChange={(event) => setField("title", event.target.value)}
                        disabled={busy}
                      />
                    </div>
                    <div className="field field-full">
                      <label htmlFor="studio-situation">Situation</label>
                      <textarea
                        id="studio-situation"
                        rows={4}
                        value={fields.situation}
                        onChange={(event) => setField("situation", event.target.value)}
                        disabled={busy}
                      />
                    </div>
                    <div className="field field-full">
                      <label htmlFor="studio-actions">Actions</label>
                      <textarea
                        id="studio-actions"
                        rows={4}
                        value={fields.actions}
                        onChange={(event) => setField("actions", event.target.value)}
                        placeholder="What you personally did. Unknowns stay unknown."
                        disabled={busy}
                      />
                    </div>
                    <div className="field">
                      <label htmlFor="studio-employer">Employer context</label>
                      <input
                        id="studio-employer"
                        value={fields.employer_context}
                        onChange={(event) => setField("employer_context", event.target.value)}
                        disabled={busy}
                      />
                    </div>
                    <div className="field">
                      <label htmlFor="studio-role-context">Role context</label>
                      <input
                        id="studio-role-context"
                        value={fields.role_context}
                        onChange={(event) => setField("role_context", event.target.value)}
                        disabled={busy}
                      />
                    </div>
                    <div className="field">
                      <label htmlFor="studio-technologies">Technologies</label>
                      <input
                        id="studio-technologies"
                        value={fields.technologies}
                        onChange={(event) => setField("technologies", event.target.value)}
                        disabled={busy}
                      />
                    </div>
                    <div className="field">
                      <label htmlFor="studio-period">Period</label>
                      <input
                        id="studio-period"
                        value={fields.period}
                        onChange={(event) => setField("period", event.target.value)}
                        disabled={busy}
                      />
                    </div>
                    <div className="field">
                      <label htmlFor="studio-outcome-state">Outcome status</label>
                      <select
                        id="studio-outcome-state"
                        value={fields.outcome_state}
                        onChange={(event) => setField("outcome_state", event.target.value)}
                        disabled={busy}
                      >
                        <option value="not_recorded">Not recorded</option>
                        <option value="unknown">Unknown</option>
                        <option value="recorded">Recorded</option>
                      </select>
                    </div>
                    <div className="field">
                      <label htmlFor="studio-outcome">Outcome</label>
                      <input
                        id="studio-outcome"
                        value={fields.outcome}
                        onChange={(event) => setField("outcome", event.target.value)}
                        disabled={busy}
                      />
                    </div>
                    <div className="field field-full">
                      <label htmlFor="studio-source-notes">Source notes</label>
                      <textarea
                        id="studio-source-notes"
                        rows={2}
                        value={fields.source_notes}
                        onChange={(event) => setField("source_notes", event.target.value)}
                        disabled={busy}
                      />
                    </div>
                  </div>

                  {(record.source_question !== "" ||
                    record.source_writing !== "" ||
                    record.source_notes !== "") && (
                    <details className="career-details">
                      <summary>Inspect source and supporting notes</summary>
                      {record.source_library_entry_id > 0 && (
                        <p className="section-subtitle">
                          Snapshot from Answer Library
                          {record.source_employer !== ""
                            ? ` · ${record.source_employer}`
                            : record.source_host !== ""
                              ? ` · ${record.source_host}`
                              : ""}
                          {record.source_job_title !== "" ? ` · ${record.source_job_title}` : ""}
                        </p>
                      )}
                      {record.source_question !== "" && (
                        <>
                          <h4 className="studio-source-heading">Original employer question</h4>
                          <pre className="career-pre">{record.source_question}</pre>
                        </>
                      )}
                      {record.source_writing !== "" && (
                        <>
                          <h4 className="studio-source-heading">Original writing</h4>
                          <pre className="career-pre">{record.source_writing}</pre>
                        </>
                      )}
                      {record.source_notes !== "" && (
                        <>
                          <h4 className="studio-source-heading">Supporting notes</h4>
                          <pre className="career-pre">{record.source_notes}</pre>
                        </>
                      )}
                    </details>
                  )}

                  {(() => {
                    const use = activation.find((entry) => entry.id === record.id);
                    if (!use) return null;
                    const staleAnswers = use.answers.filter((item) => item.state === "needs-review");
                    const staleLetters = use.letters.filter((item) => item.stale);
                    return (
                      <details className="career-block" open={staleAnswers.length > 0 || staleLetters.length > 0}>
                        <summary>
                          Application use
                          {staleAnswers.length + staleLetters.length > 0
                            ? ` · review ${staleAnswers.length + staleLetters.length} stale`
                            : use.answers.length + use.letters.length > 0
                              ? ` · ${use.answers.length + use.letters.length} citing`
                              : " · unused"}
                        </summary>
                        {use.interview ? (
                          <p className="section-subtitle">
                            Recorded interview:{" "}
                            <a href={`/career?session=${use.interview.session_id}`}>
                              {use.interview.responses} recorded response(s)
                            </a>
                            . Answer traces quote these responses verbatim.
                          </p>
                        ) : (
                          <p className="section-subtitle">
                            No recorded interview is linked. This record can draft ordinary answers but cannot
                            advertise an interview evidence trace.
                          </p>
                        )}
                        {use.answers.length === 0 && use.letters.length === 0 ? (
                          <p className="section-subtitle">No saved answers or letter drafts cite this record yet.</p>
                        ) : (
                          <ul className="career-session-list">
                            {use.answers.map((item) => (
                              <li key={`answer-${item.job_id}-${item.question_key}`}>
                                <div>
                                  <strong>
                                    <a href={`/jobs/${item.job_id}#prepare`}>
                                      {item.job_title === "" ? `Application #${item.job_id}` : item.job_title}
                                    </a>
                                  </strong>
                                  <span className="career-session-meta">
                                    Saved answer · {item.label === "" ? "early draft" : item.label} · {item.state}
                                  </span>
                                  <span className="career-session-meta">{item.detail}</span>
                                </div>
                              </li>
                            ))}
                            {use.letters.map((item) => (
                              <li key={`letter-${item.job_id}-${item.draft_id}`}>
                                <div>
                                  <strong>
                                    <a href={`/jobs/${item.job_id}?draft=${item.draft_id}#letter-editor`}>
                                      {item.job_title === "" ? `Application #${item.job_id}` : item.job_title}
                                    </a>
                                  </strong>
                                  <span className="career-session-meta">
                                    Letter draft {item.draft_id} · pinned revision {item.pinned_revision} ·{" "}
                                    {item.stale ? "supporting evidence changed; text unchanged" : "supporting evidence unchanged"}
                                  </span>
                                </div>
                              </li>
                            ))}
                          </ul>
                        )}
                      </details>
                    );
                  })()}

                  <div className="career-block">
                    <div className="career-checks">
                      <label>
                        <input
                          type="checkbox"
                          checked={confirmReview}
                          onChange={(event) => setConfirmReview(event.target.checked)}
                          disabled={busy}
                        />{" "}
                        This record accurately describes my experience.
                      </label>
                      {record.status === "reviewed" && (
                        <label>
                          <input
                            type="checkbox"
                            checked={confirmReopen}
                            onChange={(event) => setConfirmReopen(event.target.checked)}
                            disabled={busy}
                          />{" "}
                          This record needs more work; reopen it as a draft.
                        </label>
                      )}
                      <label>
                        <input
                          type="checkbox"
                          checked={confirmDelete}
                          onChange={(event) => setConfirmDelete(event.target.checked)}
                          disabled={busy}
                        />{" "}
                        Delete this record permanently.
                      </label>
                    </div>
                    <div className="form-actions studio-actions">
                      <button
                        className="button button-primary"
                        type="button"
                        onClick={() => handleSave("save")}
                        disabled={busy}
                      >
                        Save
                      </button>
                      <button
                        className="button button-primary"
                        type="button"
                        onClick={() => handleSave("review")}
                        disabled={busy || !confirmReview}
                      >
                        Mark as reviewed
                      </button>
                      {record.status === "reviewed" && (
                        <button
                          className="button button-secondary"
                          type="button"
                          onClick={handleReopen}
                          disabled={busy || !confirmReopen}
                        >
                          Reopen as draft
                        </button>
                      )}
                      <button
                        className="button button-secondary"
                        type="button"
                        onClick={handleDelete}
                        disabled={busy || !confirmDelete}
                      >
                        Delete
                      </button>
                    </div>
                  </div>
                </section>
              )}

              <section className="panel" aria-labelledby="roles-title">
                <div className="career-section-heading">
                  <h2 id="roles-title">Role comparison</h2>
                  <p className="section-subtitle">
                    Compare one to {maxEvidence} reviewed experiences against a saved Opportunity. The role context is
                    derived from the posting itself; unsupported requirements are named explicitly, never hidden.
                  </p>
                </div>
                {rolesData && rolesData.roles.length === 0 && (
                  <p className="section-subtitle">
                    No Opportunities saved yet. <a href="/jobs">Save one first</a>, then compare.
                  </p>
                )}
                {rolesData && rolesData.roles.length > 0 && (
                  <>
                    <div className="career-start-grid">
                      <div className="field">
                        <label htmlFor="studio-role">Opportunity</label>
                        <select
                          id="studio-role"
                          value={roleKey}
                          onChange={(event) => {
                            setRoleKey(event.target.value);
                            setSnapshot(null);
                          }}
                          disabled={busy}
                        >
                          <option value="">Choose an Opportunity…</option>
                          {rolesData.roles.map((role) => (
                            <option key={role.key} value={role.key}>
                              {(role.title === "" ? `Posting #${role.job_id}` : role.title) +
                                (role.company === "" ? "" : ` — ${role.company}`) +
                                (role.usable ? "" : " (needs description)")}
                            </option>
                          ))}
                        </select>
                      </div>
                    </div>
                    {activeRole && !activeRole.usable && (
                      <p className="job-form-error" role="alert">
                        {activeRole.unusable_reason}{" "}
                        <a className="text-link" href={`/jobs/${activeRole.job_id}`}>
                          Edit the posting
                        </a>
                        , then compare.
                      </p>
                    )}
                    {activeRole && activeRole.usable && (
                      <div className="career-block">
                        <h3>Role context from the posting</h3>
                        <p className="section-subtitle">{activeRole.focus}</p>
                        <h4>Requirements under comparison ({activeRole.requirements_count})</h4>
                        <ul className="career-unresolved">
                          {activeRole.requirements.map((requirement, index) => (
                            <li key={index}>{requirement}</li>
                          ))}
                        </ul>
                      </div>
                    )}
                    {activeRole && activeRole.usable && (
                      <div className="career-block">
                        <h3>Reviewed experiences ({selectedIds.length} of {maxEvidence})</h3>
                        {rolesData.reviewed.length === 0 ? (
                          <p className="section-subtitle">
                            No reviewed records yet. Review at least one record in the library above first.
                          </p>
                        ) : (
                          <div className="career-leads" role="group" aria-label="Reviewed experiences">
                            {rolesData.reviewed.map((ref) => (
                              <label key={ref.id} className="career-lead">
                                <input
                                  type="checkbox"
                                  checked={selectedIds.includes(ref.id)}
                                  onChange={() => toggleSelectedId(ref.id)}
                                  disabled={busy || (!selectedIds.includes(ref.id) && selectedIds.length >= maxEvidence)}
                                />
                                <span>
                                  <strong>{ref.title}</strong>
                                  <span className="career-lead-why">
                                    revision {ref.revision} · {ref.bytes} bytes · sha {shortHash(ref.content_hash)}
                                  </span>
                                </span>
                              </label>
                            ))}
                          </div>
                        )}
                        <div className="form-actions">
                          <button
                            className="button button-primary"
                            type="button"
                            onClick={handleSnapshot}
                            disabled={busy || selectedIds.length === 0}
                          >
                            Review selection
                          </button>
                        </div>
                      </div>
                    )}
                    {snapshot && (
                      <div className="career-block">
                        <h3>Authorize this comparison</h3>
                        <p className="section-subtitle">
                          {activeProvider ? activeProvider.execution : "Choose a provider."} {snapshot.bytes} bytes ·{" "}
                          {snapshot.policy_name} · binding {shortHash(snapshot.input_hash)}
                        </p>
                        <div className="career-start-grid">
                          <div className="field">
                            <label htmlFor="studio-role-provider">Provider</label>
                            <select
                              id="studio-role-provider"
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
                            <label htmlFor="studio-role-model">Model</label>
                            <input
                              id="studio-role-model"
                              value={model}
                              onChange={(event) => setModel(event.target.value)}
                              disabled={busy}
                            />
                          </div>
                        </div>
                        <div className="career-checks">
                          <label>
                            <input
                              type="checkbox"
                              checked={roleAckGen}
                              onChange={(event) => setRoleAckGen(event.target.checked)}
                              disabled={busy}
                            />{" "}
                            I authorize this specific inference.
                          </label>
                          <label>
                            <input
                              type="checkbox"
                              checked={roleAckExt}
                              onChange={(event) => setRoleAckExt(event.target.checked)}
                              disabled={busy}
                            />{" "}
                            I understand the summarized material above leaves this app for external inference.
                          </label>
                        </div>
                        <div className="form-actions">
                          <button
                            className="button button-primary"
                            type="button"
                            onClick={handleAuthorizeRole}
                            disabled={busy || !roleAckGen || !roleAckExt}
                          >
                            Authorize and run
                          </button>
                        </div>
                      </div>
                    )}
                    {savedRoleView && (
                      <div className="career-block">
                        <h3>Saved comparison</h3>
                        {savedRoleView.stale && (
                          <p className="studio-stale" role="alert">
                            Stale —{" "}
                            {savedRoleView.stale_role && savedRoleView.stale_evidence
                              ? "the posting and the evidence both changed since this comparison."
                              : savedRoleView.stale_role
                                ? "the posting changed since this comparison."
                                : "a cited record was corrected, reopened, or deleted since this comparison."}{" "}
                            The prior reading stays below for reference; authorize again to refresh it.
                          </p>
                        )}
                        <p className="section-subtitle">
                          {savedRoleView.notice} Saved {formatDateTime(savedRoleView.updated_at)}
                          {savedRoleView.provider !== "" ? ` · ${savedRoleView.provider}/${savedRoleView.model}` : ""}.
                        </p>
                        {savedRoleView.has_analysis && savedRoleView.analysis && (
                          <>
                            <div className="studio-cards">
                              {savedRoleView.analysis.items.map((item) => (
                                <article key={item.evidence_id} className="studio-card">
                                  <h4>
                                    {evidenceTitles.get(item.evidence_id) ?? `Record #${item.evidence_id}`}{" "}
                                    <span className={item.relevant ? "studio-badge studio-badge-hit" : "studio-badge"}>
                                      {item.relevant ? "Relevant" : "Not relevant"}
                                    </span>{" "}
                                    {savedRoleView.stale_evidence_ids?.includes(item.evidence_id) && (
                                      <span className="studio-badge studio-badge-stale">Stale</span>
                                    )}
                                  </h4>
                                  <p className="section-subtitle">
                                    Cited revision {item.evidence_revision} ·{" "}
                                    {evidenceTitles.get(item.evidence_id) ? "" : "record no longer reviewed · "}
                                    {item.explanation}
                                  </p>
                                  {item.capabilities.length > 0 && (
                                    <>
                                      <h5>Supported capabilities</h5>
                                      <ul>
                                        {item.capabilities.map((capability, index) => (
                                          <li key={index}>{capability}</li>
                                        ))}
                                      </ul>
                                    </>
                                  )}
                                  {item.gaps.length > 0 && (
                                    <>
                                      <h5>Gaps</h5>
                                      <ul>
                                        {item.gaps.map((gap, index) => (
                                          <li key={index}>{gap}</li>
                                        ))}
                                      </ul>
                                    </>
                                  )}
                                </article>
                              ))}
                            </div>
                            <h4>Unsupported requirements ({savedRoleView.analysis.unmet.length})</h4>
                            {savedRoleView.analysis.unmet.length === 0 ? (
                              <p className="section-subtitle">
                                Every derived requirement found support in the compared records.
                              </p>
                            ) : (
                              <ul className="career-unresolved">
                                {savedRoleView.analysis.unmet.map((requirement, index) => (
                                  <li key={index}>{requirement}</li>
                                ))}
                              </ul>
                            )}
                          </>
                        )}
                      </div>
                    )}
                  </>
                )}
              </section>

              <section className="panel" aria-labelledby="synthesis-title">
                <div className="career-section-heading">
                  <h2 id="synthesis-title">Cross-experience synthesis</h2>
                  <p className="section-subtitle">
                    What recurs across your reviewed experiences. Connections are hypotheses grounded in quoted records —
                    challenge any interpretation you dispute.
                  </p>
                </div>
                {synthesis && (
                  <>
                    <h3 className="studio-mini-heading">
                      Reviewed set ({synthesis.eligible.length}
                      {synthesis.eligible_overflow > 0 ? `, ${synthesis.eligible_overflow} more beyond the comparison budget` : ""}
                      )
                    </h3>
                    {synthesis.eligible.length < 2 ? (
                      <p className="section-subtitle">
                        Review at least two records with complete title, situation, and actions before looking for
                        connections.
                      </p>
                    ) : (
                      <>
                        <ul className="career-unresolved">
                          {synthesis.eligible.map((ref) => (
                            <li key={ref.id}>
                              {ref.title} · revision {ref.revision} · {ref.bytes} bytes · sha{" "}
                              {shortHash(ref.content_hash)}
                            </li>
                          ))}
                        </ul>
                        <div className="career-block">
                          <h3>Authorize this synthesis</h3>
                          <p className="section-subtitle">
                            {activeProvider ? activeProvider.execution : "Choose a provider."} {synthesis.bytes} bytes ·{" "}
                            {synthesis.policy_name} · binding {shortHash(synthesis.input_hash)}
                          </p>
                          <div className="career-start-grid">
                            <div className="field">
                              <label htmlFor="studio-syn-provider">Provider</label>
                              <select
                                id="studio-syn-provider"
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
                              <label htmlFor="studio-syn-model">Model</label>
                              <input
                                id="studio-syn-model"
                                value={model}
                                onChange={(event) => setModel(event.target.value)}
                                disabled={busy}
                              />
                            </div>
                          </div>
                          <div className="career-checks">
                            <label>
                              <input
                                type="checkbox"
                                checked={synAckGen}
                                onChange={(event) => setSynAckGen(event.target.checked)}
                                disabled={busy}
                              />{" "}
                              I authorize this specific inference.
                            </label>
                            <label>
                              <input
                                type="checkbox"
                                checked={synAckExt}
                                onChange={(event) => setSynAckExt(event.target.checked)}
                                disabled={busy}
                              />{" "}
                              I understand the summarized material above leaves this app for external inference.
                            </label>
                          </div>
                          <div className="form-actions">
                            <button
                              className="button button-primary"
                              type="button"
                              onClick={handleAuthorizeSynthesis}
                              disabled={busy || !synAckGen || !synAckExt}
                            >
                              Authorize and run
                            </button>
                          </div>
                        </div>
                      </>
                    )}
                    {synthesis.saved && (
                      <div className="career-block">
                        <h3>
                          Saved synthesis
                          {synthesis.saved.challenges_count > 0 &&
                            ` · ${synthesis.saved.challenges_count} challenged`}
                        </h3>
                        {synthesis.saved.stale && (
                          <p className="studio-stale" role="alert">
                            Stale — a cited record was corrected, reopened, or deleted since this synthesis. The prior
                            interpretation stays below for reference; authorize again to refresh it.
                          </p>
                        )}
                        {!synthesis.saved.integrity_ok && (
                          <p className="job-form-error" role="alert">
                            A saved quotation no longer resolves against its cited record. Treat this synthesis as
                            unreliable and authorize a fresh one.
                          </p>
                        )}
                        <p className="section-subtitle">
                          {synthesis.saved.notice} Saved {formatDateTime(synthesis.saved.updated_at)}
                          {synthesis.saved.provider !== "" ? ` · ${synthesis.saved.provider}` : ""}
                          {synthesis.saved.model !== "" ? `/${synthesis.saved.model}` : ""}.
                        </p>
                        {synthesis.saved.has_synthesis && synthesis.saved.connections.length === 0 && (
                          <p className="section-subtitle">
                            No defensible cross-experience connection was found. That is an honest result, not a
                            failure.
                          </p>
                        )}
                        <div className="studio-cards">
                          {synthesis.saved.connections.map((view) => {
                            const draft = challengeDrafts[view.connection.id] ?? {
                              state: view.challenge,
                              note: view.note,
                            };
                            return (
                              <article key={view.connection.id} className="studio-card">
                                <h4>
                                  {view.connection.capability}{" "}
                                  {view.stale && <span className="studio-badge studio-badge-stale">Stale</span>}{" "}
                                  {view.challenge !== "" && (
                                    <span className="studio-badge">
                                      {view.challenge === "unsupported" ? "Challenged: unsupported" : "Challenged"}
                                    </span>
                                  )}
                                </h4>
                                <p>{view.connection.interpretation}</p>
                                <h5>How the records connect</h5>
                                <p>{view.connection.relationship}</p>
                                <h5>Grounding</h5>
                                <ul>
                                  {view.connection.supports.map((support, index) => (
                                    <li key={index}>
                                      <strong>
                                        {synthesisTitles.get(support.evidence_id) ??
                                          `Record #${support.evidence_id}`}{" "}
                                        (revision {support.evidence_revision})
                                      </strong>
                                      : “{support.quote}” — {support.action}
                                    </li>
                                  ))}
                                </ul>
                                <h5>Limitations</h5>
                                <ul>
                                  {view.connection.limitations.map((limitation, index) => (
                                    <li key={index}>{limitation}</li>
                                  ))}
                                </ul>
                                <h5>What this lets you credibly say</h5>
                                <p>{view.connection.candidate_use}</p>
                                {view.connection.follow_up !== "" && (
                                  <>
                                    <h5>Follow-up worth answering</h5>
                                    <p>{view.connection.follow_up}</p>
                                  </>
                                )}
                                <div className="career-start-grid">
                                  <div className="field">
                                    <label htmlFor={`studio-challenge-${view.connection.id}`}>Your judgment</label>
                                    <select
                                      id={`studio-challenge-${view.connection.id}`}
                                      value={draft.state}
                                      onChange={(event) =>
                                        setChallengeDrafts((current) => ({
                                          ...current,
                                          [view.connection.id]: { state: event.target.value, note: draft.note },
                                        }))
                                      }
                                      disabled={busy}
                                    >
                                      <option value="">No challenge</option>
                                      <option value="questioned">Questioned</option>
                                      <option value="unsupported">Unsupported</option>
                                    </select>
                                  </div>
                                  <div className="field">
                                    <label htmlFor={`studio-challenge-note-${view.connection.id}`}>
                                      Note (under 500 bytes)
                                    </label>
                                    <input
                                      id={`studio-challenge-note-${view.connection.id}`}
                                      value={draft.note}
                                      onChange={(event) =>
                                        setChallengeDrafts((current) => ({
                                          ...current,
                                          [view.connection.id]: { state: draft.state, note: event.target.value },
                                        }))
                                      }
                                      disabled={busy}
                                    />
                                  </div>
                                </div>
                                <div className="form-actions">
                                  <button
                                    className="button button-secondary"
                                    type="button"
                                    onClick={() => handleChallenge(view.connection.id)}
                                    disabled={busy}
                                  >
                                    Save challenge
                                  </button>
                                </div>
                              </article>
                            );
                          })}
                        </div>
                      </div>
                    )}
                  </>
                )}
              </section>
            </>
          )}
        </main>
      </div>
    </div>
  );
}
