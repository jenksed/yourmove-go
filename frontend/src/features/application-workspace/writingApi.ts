// YM-004B React -> Go JSON transport for application writing.
// Go remains authoritative for validation, durable state, evidence policy,
// and provider rules. React owns presentation and disposable drafts.

export interface WritingLibrarySuggestion {
 matchKind: string;
  entryId: number;
  revision: number;
  identity: string;
  question: string;
  employer: string;
  jobTitle: string;
  host: string;
  excerpt: string;
}

export interface WritingEvidenceSuggestion {
  evidenceId: number;
  revision: number;
  identity: string;
  title: string;
  outcomeState: string;
  outcome: string;
  actionsExcerpt: string;
}

export interface WritingQuestion {
  key: string;
  label: string;
  kind: string;
  required: boolean;
  options: string[] | null;
  value: string;
  source: string;
  localStatus: string;
  revision: number;
  editable: boolean;
  libraryEligible: boolean;
  helpEligible: boolean;
  isEarly: boolean;
  discoverySource: string;
  discoveryFingerprint: string;
  sourceQuestionId: string;
  triageCategory: string;
  manualReason: string;
  reusableKey: string;
  reusableRevision: number;
  reusableEligible: boolean;
  reusableAnswer: string;
  reusableAvailable: boolean;
  originEntryId: number;
  originRevision: number;
  originQuestion: string;
  originEmployer: string;
  originRecorded: boolean;
  libraryAutoEntryId: number;
  libraryAutoRevision: number;
  libraryAutoQuestion: string;
  libraryAutoPrepared: boolean;
  librarySuggestions: WritingLibrarySuggestion[];
  evidenceSuggestions: WritingEvidenceSuggestion[];
  reconciliationStatus: string;
  reconciliationDetail: string;
  reconciliationAcceptable: boolean;
  reconciled: boolean;
}

export interface LibraryFullExample {
  revision: number;
  original_question: string;
  answer: string;
  source_employer: string;
  source_job_title: string;
  source_host: string;
  source_kind: string;
  early_discovery_source: string;
  early_source_question_id: string;
  created_at: string;
  updated_at: string;
}

export interface LibraryBrowseEntry {
  id: number;
  revision: number;
  question: string;
  employer: string;
  jobTitle: string;
  host: string;
  excerpt: string;
  sourceKind: string;
  updatedAt: string;
}

export interface LibraryBrowseResult {
  entries: LibraryBrowseEntry[];
  nextCursor: number;
  hasMore: boolean;
  libraryEmpty: boolean;
  query: string;
  bounded: boolean;
  bound?: number;
}

export interface PromotionResult {
  saved: boolean;
  existing: boolean;
  entryId: number;
  revision: number;
}

export interface WritingPrevious {
  key: string;
  label: string;
  value: string;
  revision: number;
  early: boolean;
}

export interface QuestionsSnapshot {
  browserActive: boolean;
  captureState?: { phase: string; message: string };
  available: boolean;
  source: string;
  partial: boolean;
  anchor: string;
  activeContext: string;
  inspection: {
    liveId: number;
    liveSource: string;
    liveCompleteness: string;
    liveUrl: string;
    earlySource: string;
    earlyCompleteness: string;
    earlyFingerprint: string;
  };
  live: WritingQuestion[];
  early: WritingQuestion[];
  // Unified applicant-facing projection: one coherent list merging both
  // contexts. Each row keeps its own save identity; grouping follows
  // triageCategory (answer/profile/employer/review).
  questions: WritingQuestion[];
  unidentifiedCount: number;
  previous: WritingPrevious[];
  profileCount: number;
  employerOnlyCount: number;
  manualReviewCount: number;
  earlyNotice: string;
  returnNotice: string;
  editingAllowed: boolean;
  answerEditingAllowed: boolean;
}

export interface AiChoice {
  Identity: string;
  Label: string;
}

export interface AiProviderOption {
  ID: string;
  Label: string;
  Model: string;
  Help: string;
}

export interface AssistanceSnapshot {
  question: WritingQuestion;
  libraryEmpty: boolean;
  evidenceEmpty: boolean;
  sources: {
    Evidence: AiChoice[] | null;
    Writing: AiChoice[] | null;
    ApplicationResume: AiChoice | null;
    MasterResume: AiChoice | null;
  };
  providers: AiProviderOption[];
  defaultProvider: string;
  defaultModel: string;
}

export interface AnswerTraceClaim {
  claim: string;
  evidence_id: number;
  evidence_revision: number;
  response_id: string;
  quote: string;
}

export interface AnswerTrace {
  answer: string;
  claims: AnswerTraceClaim[];
  unresolved: string[];
}

export interface AiTicket {
  ticket: string;
  state: string;
  question_key: string;
  question: string;
  revision: number;
  answer?: string;
  trace?: AnswerTrace | null;
  adoptable?: boolean;
  notice?: string;
  expires_at?: string;
  sources?: unknown;
  execution?: string;
  prompt?: string;
  instructions?: string;
  code?: string;
  message?: string;
}

export interface LetterDraftSummary {
  id: number;
  provider: string;
  model: string;
  createdAt: string;
  editedAt: string;
  current: boolean;
}

export interface LetterEvidenceRef {
  id: number;
  revision: number;
  created_at: string;
  title: string;
}

export interface LetterDraft {
  id: number;
  jobId: number;
  provider: string;
  model: string;
  body: string;
  createdAt: string;
  editedAt: string;
  evidenceRefs: LetterEvidenceRef[];
  evidenceStale: boolean;
  contentHash: string;
  masterResumeId: number;
  masterSha: string;
  masterName: string;
  masterVersion: number;
  roleResumeId: number;
  roleSha: string;
  roleName: string;
  roleVersion: number;
  manualSource: boolean;
  originRecorded: boolean;
  originSourceLetterId: number;
  originSourceJobId: number;
  originEmployer: string;
  originJobTitle: string;
}

export interface LetterCatalogEntry {
  id: number;
  jobId: number;
  employer: string;
  jobTitle: string;
  provider: string;
  model: string;
  createdAt: string;
  editedAt: string;
  preview: string;
  masterName: string;
  masterVersion: number;
  roleName: string;
  roleVersion: number;
  manual: boolean;
  current?: boolean;
}

export interface LetterHistoryResult {
  entries: LetterCatalogEntry[];
  nextCursor: number;
  hasMore: boolean;
  selectedId: number;
}

export interface LetterCatalogResult {
  entries: LetterCatalogEntry[];
  nextCursor: number;
  hasMore: boolean;
  libraryEmpty: boolean;
  query: string;
  bounded: boolean;
}

export interface LetterSource {
  name: string;
  version: number;
  ready: boolean;
  message: string;
  excerpt: string;
  truncated: boolean;
}

export interface LettersSnapshot {
  drafts: LetterDraftSummary[];
  current: LetterDraft | null;
  sources: {
    master: LetterSource;
    role: LetterSource;
    jobDescriptionReady: boolean;
    ready: boolean;
  };
  evidenceChoices: { Identity: string; Label: string }[];
  providers: AiProviderOption[];
  defaultProvider: string;
  defaultModel: string;
  editingAllowed: boolean;
}

async function readError(response: Response): Promise<string> {
  const text = await response.text();
  if (text.trim() !== "") {
    try {
      const data = JSON.parse(text) as { error?: string; message?: string };
      if (typeof data.error === "string" && data.error.trim() !== "") return data.error;
      if (typeof data.message === "string" && data.message.trim() !== "") return data.message;
    } catch {
      return text.trim();
    }
  }
  return `Request failed (${response.status})`;
}

export async function fetchBrief(jobId: number, scope: string): Promise<string> {
  const normalized = scope === "all" ? "all" : "unanswered";
  const response = await fetch(`/jobs/${jobId}/brief?format=brief&scope=${encodeURIComponent(normalized)}`, {
    headers: { Accept: "text/plain" },
  });
  if (!response.ok) throw new Error(await readError(response));
  return await response.text();
}

export async function fetchQuestions(jobId: number): Promise<QuestionsSnapshot> {
  const response = await fetch(`/api/jobs/${jobId}/questions`, {
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new Error(await readError(response));
  const snapshot = (await response.json()) as QuestionsSnapshot;
  // Go encodes empty lists as null; the workspace treats them as empty.
  snapshot.questions ??= [];
  snapshot.live ??= [];
  snapshot.early ??= [];
  snapshot.previous ??= [];
  return snapshot;
}

export async function discoverQuestions(jobId: number): Promise<string> {
  const response = await fetch(`/api/jobs/${jobId}/questions/discover`, { method: "POST", headers: { Accept: "application/json" } });
  if (!response.ok) throw new Error(await readError(response));
  return ((await response.json()) as { status: string }).status;
}

export interface SaveAnswerInput {
 save_to_library?: boolean;
  question_key: string;
  revision: number;
  destination: string;
  answer: string;
  reusable: boolean;
  reusable_revision: number;
  early: boolean;
  discovery_source: string;
  discovery_fingerprint: string;
  source_question_id: string;
  origin_entry_id?: number;
  origin_revision?: number;
  origin_clear?: boolean;
}

export async function saveAnswer(
  jobId: number,
  input: SaveAnswerInput,
): Promise<{ saved: boolean; key: string; revision: number }> {
  const response = await fetch(`/api/jobs/${jobId}/answers`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as { saved: boolean; key: string; revision: number };
}

export async function fetchLibraryExample(entryId: number): Promise<LibraryFullExample> {
  const response = await fetch(`/answer-library/${entryId}/content`, {
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as LibraryFullExample;
}

export interface StartingPointResult {
  entry_id: number;
  library_revision: number;
  question_key: string;
  revision: number;
  answer: string;
}

export async function fetchStartingPoint(
  jobId: number,
  input: {
    library_entry_id: number;
    library_revision: number;
    question_key: string;
    revision: number;
    destination: string;
    early: boolean;
    discovery_source: string;
    discovery_fingerprint: string;
    source_question_id: string;
  },
): Promise<StartingPointResult> {
  const body = new URLSearchParams();
  body.set("library_entry_id", String(input.library_entry_id));
  body.set("library_revision", String(input.library_revision));
  body.set("question_key", input.question_key);
  body.set("revision", String(input.revision));
  body.set("destination", input.destination);
  if (input.early) {
    body.set("early", "yes");
    body.set("discovery_source", input.discovery_source);
    body.set("discovery_fingerprint", input.discovery_fingerprint);
    body.set("source_question_id", input.source_question_id);
  }
  const response = await fetch(`/jobs/${jobId}/answers/starting-point`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded", Accept: "application/json" },
    body: body.toString(),
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as StartingPointResult;
}

export async function browseLibrary(
  input: { query?: string; cursor?: number; limit?: number },
): Promise<LibraryBrowseResult> {
  const params = new URLSearchParams();
  if (input.query !== undefined && input.query !== "") params.set("query", input.query);
  if (input.cursor !== undefined && input.cursor > 0) params.set("cursor", String(input.cursor));
  if (input.limit !== undefined) params.set("limit", String(input.limit));
  const suffix = params.toString() !== "" ? `?${params.toString()}` : "";
  const response = await fetch(`/api/answer-library${suffix}`, {
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as LibraryBrowseResult;
}

export async function promoteToLibrary(
  jobId: number,
  input: { question_key: string; revision: number; early: boolean },
): Promise<PromotionResult> {
  const body = new URLSearchParams();
  body.set("question_key", input.question_key);
  body.set("revision", String(input.revision));
  const path = input.early
    ? `/jobs/${jobId}/answers/early-library`
    : `/jobs/${jobId}/answers/library`;
  const response = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded", Accept: "application/json" },
    body: body.toString(),
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as PromotionResult;
}

export async function fetchAssistance(
  jobId: number,
  input: Omit<SaveAnswerInput, "answer" | "reusable" | "reusable_revision">,
): Promise<AssistanceSnapshot> {
  const response = await fetch(`/api/jobs/${jobId}/assistance`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as AssistanceSnapshot;
}

function formBody(params: Record<string, string | string[]>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (Array.isArray(value)) {
      for (const item of value) search.append(key, item);
    } else {
      search.append(key, value);
    }
  }
  return search.toString();
}

async function postAiForm(jobId: number, action: string, params: Record<string, string | string[]>): Promise<AiTicket> {
  const response = await fetch(`/jobs/${jobId}/answers/ai/${action}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
      Accept: "application/json",
    },
    body: formBody(params),
  });
  const data = (await response.json()) as AiTicket;
  if (!response.ok) throw new Error(data.message ?? data.code ?? `AI request failed (${response.status})`);
  return data;
}

export interface AiReviewInput {
  question_key: string;
  revision: string;
  destination: string;
  provider: string;
  model: string;
  career_evidence: string[];
  library_entry: string;
  application_resume: string;
  master_resume: string;
  include_job_context: string;
  early: string;
  discovery_source: string;
  discovery_fingerprint: string;
  source_question_id: string;
}

export function aiReview(jobId: number, input: AiReviewInput): Promise<AiTicket> {
  const params: Record<string, string | string[]> = {
    question_key: input.question_key,
    revision: input.revision,
    destination: input.destination,
    provider: input.provider,
    model: input.model,
  };
  if (input.career_evidence.length > 0) params["career_evidence"] = input.career_evidence;
  if (input.library_entry !== "") params["library_entry"] = input.library_entry;
  if (input.application_resume !== "") params["application_resume"] = input.application_resume;
  if (input.master_resume !== "") params["master_resume"] = input.master_resume;
  if (input.include_job_context !== "") params["include_job_context"] = input.include_job_context;
  if (input.early !== "") {
    params["early"] = input.early;
    params["discovery_source"] = input.discovery_source;
    params["discovery_fingerprint"] = input.discovery_fingerprint;
    params["source_question_id"] = input.source_question_id;
  }
  return postAiForm(jobId, "review", params);
}

export function aiGenerate(jobId: number, ticket: string): Promise<AiTicket> {
  return postAiForm(jobId, "generate", {
    ticket,
    authorize_generation: "yes",
    authorize_external: "yes",
  });
}

export function aiAdoptCheck(jobId: number, ticket: string, questionKey: string, revision: string): Promise<AiTicket> {
  return postAiForm(jobId, "adopt-check", {
    ticket,
    question_key: questionKey,
    revision,
  });
}

export function aiCancel(jobId: number, ticket: string): Promise<AiTicket> {
  return postAiForm(jobId, "cancel", { ticket });
}

export async function fetchLetters(jobId: number): Promise<LettersSnapshot> {
  const response = await fetch(`/api/jobs/${jobId}/letters`, {
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as LettersSnapshot;
}

export async function fetchLetterDraft(jobId: number, draftId: number): Promise<LetterDraft> {
  const response = await fetch(`/api/jobs/${jobId}/letters/${draftId}`, {
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as LetterDraft;
}

export async function saveLetterDraft(jobId: number, draftId: number, body: string, expectedHash?: string): Promise<void> {
  const response = await fetch(`/api/jobs/${jobId}/letters/${draftId}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify(expectedHash ? { body, expected_hash: expectedHash } : { body }),
  });
  if (!response.ok) throw new Error(await readError(response));
}

export async function selectLetterDraft(jobId: number, draftId: number): Promise<void> {
  const response = await fetch(`/api/jobs/${jobId}/letters/select`, {
    method: "POST", headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ draftId }),
  });
  if (!response.ok) throw new Error(await readError(response));
}

export async function createManualLetter(
  jobId: number,
  body: string,
  prior?: { prior_letter_id: number; prior_job_id: number; prior_body_hash: string },
): Promise<{ draftId: number }> {
  const response = await fetch(`/api/jobs/${jobId}/letters/manual`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify(prior ? { body, ...prior } : { body }),
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as { draftId: number };
}

export async function fetchLetterHistory(jobId: number, cursor?: number, limit?: number): Promise<LetterHistoryResult> {
  const params = new URLSearchParams();
  if (cursor !== undefined && cursor > 0) params.set("cursor", String(cursor));
  if (limit !== undefined) params.set("limit", String(limit));
  const suffix = params.toString() !== "" ? `?${params.toString()}` : "";
  const response = await fetch(`/api/jobs/${jobId}/letters/history${suffix}`, {
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as LetterHistoryResult;
}

export async function browseCoverLetters(input: {
  query?: string;
  cursor?: number;
  limit?: number;
  exclude_job?: number;
}): Promise<LetterCatalogResult> {
  const params = new URLSearchParams();
  if (input.query !== undefined && input.query !== "") params.set("query", input.query);
  if (input.cursor !== undefined && input.cursor > 0) params.set("cursor", String(input.cursor));
  if (input.limit !== undefined) params.set("limit", String(input.limit));
  if (input.exclude_job !== undefined && input.exclude_job > 0) params.set("exclude_job", String(input.exclude_job));
  const suffix = params.toString() !== "" ? `?${params.toString()}` : "";
  const response = await fetch(`/api/cover-letters${suffix}`, {
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as LetterCatalogResult;
}

export async function fetchCatalogLetter(letterId: number): Promise<{
  letter: LetterDraft;
  job: { id: number; employer: string; host: string; title: string };
}> {
  const response = await fetch(`/api/cover-letters/${letterId}`, {
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as {
    letter: LetterDraft;
    job: { id: number; employer: string; host: string; title: string };
  };
}

export interface GenerateLetterInput {
  provider: string;
  model: string;
  letter_resume_id: number;
  career_evidence: string[];
  confirm_external: string;
  confirm_local: string;
}

export async function generateLetter(jobId: number, input: GenerateLetterInput): Promise<{ generated: boolean; draftId: number }> {
  const response = await fetch(`/api/jobs/${jobId}/letters`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as { generated: boolean; draftId: number };
}

// Starts only the existing read-only managed observer; never a transfer.
export async function captureQuestions(jobId: number): Promise<void> {
  const response = await fetch(`/jobs/${jobId}/browser/start`, {
    method: "POST",
    headers: { Accept: "application/json", "Content-Type": "application/x-www-form-urlencoded" },
    body: "",
  });
  if (!response.ok) {
    const text = await response.text();
    let message = text;
    try { message = (JSON.parse(text) as { error?: string }).error || text; } catch { /* Legacy transport error. */ }
    throw new Error(message);
  }
}
