// YM-004B React -> Go JSON transport for application writing.
// Go remains authoritative for validation, durable state, evidence policy,
// and provider rules. React owns presentation and disposable drafts.

export interface WritingLibrarySuggestion {
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
  librarySuggestions: WritingLibrarySuggestion[];
  evidenceSuggestions: WritingEvidenceSuggestion[];
  reconciliationStatus: string;
  reconciliationDetail: string;
  reconciliationAcceptable: boolean;
  reconciled: boolean;
}

export interface WritingPrevious {
  key: string;
  label: string;
  value: string;
  revision: number;
  early: boolean;
}

export interface QuestionsSnapshot {
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
  provider: string;
  model: string;
  body: string;
  createdAt: string;
  editedAt: string;
  evidenceRefs: LetterEvidenceRef[];
  evidenceStale: boolean;
  contentHash: string;
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

export async function fetchQuestions(jobId: number): Promise<QuestionsSnapshot> {
  const response = await fetch(`/api/jobs/${jobId}/questions`, {
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as QuestionsSnapshot;
}

export interface SaveAnswerInput {
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

export async function saveLetterDraft(jobId: number, draftId: number, body: string): Promise<void> {
  const response = await fetch(`/api/jobs/${jobId}/letters/${draftId}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ body }),
  });
  if (!response.ok) throw new Error(await readError(response));
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
