// Narrow React -> Go JSON transport for the YM-003A career discovery slice.
// Go remains authoritative for input validation, provider policy,
// authorization binding, interview state, and promotion into Career Evidence.

export interface CareerLead {
  id: string;
  title: string;
  why: string;
}

export interface CareerSource {
  id: string;
  kind: string;
  text: string;
}

export interface CareerResponse {
  question: string;
  source: CareerSource;
}

export interface CareerProposalSupport {
  source_id: string;
  quote: string;
}

export interface CareerProposal {
  title: string;
  experience: string;
  claim: string;
  source_id: string;
  quote: string;
  supports: CareerProposalSupport[];
  unresolved: string[];
}

export interface CareerResumeRef {
  id: number;
  name: string;
  removed: boolean;
  sha256: string;
  excerpt_hash: string;
}

export interface CareerNext {
  kind: "leads" | "interview" | "none";
  input_hash: string;
  policy_name: string;
  bytes: number;
}

export interface CareerFailure {
  code: string;
  detail: string;
}

export interface CareerSession {
  id: string;
  status: "leads" | "interview" | "promoted";
  revision: number;
  created_at: string;
  updated_at: string;
  expires_at: string;
  resume: CareerResumeRef | null;
  excerpt_hash: string;
  excerpt_bytes: number;
  leads: CareerLead[];
  selected_lead_id: string;
  question: string;
  responses: CareerResponse[];
  proposal: CareerProposal | null;
  next: CareerNext;
  provider: string;
  model: string;
  failure: CareerFailure | null;
  promoted_evidence_id: number | null;
}

export interface CareerSessionSummary {
  id: string;
  status: string;
  resume_id: number;
  resume_name: string;
  resume_removed: boolean;
  leads_count: number;
  selected_lead_id: string;
  responses_count: number;
  question_pending: boolean;
  has_proposal: boolean;
  provider: string;
  model: string;
  failure_code: string;
  promoted_evidence_id: number | null;
  revision: number;
  created_at: string;
  updated_at: string;
  expires_at: string;
}

export interface CareerProvider {
  id: string;
  default_model: string;
  execution: string;
  configured: boolean;
}

export interface CareerPromoteResult {
  evidence_id: number;
  evidence_revision: number;
  path: string;
  notice: string;
}

export interface CareerResumeChoice {
  id: number;
  name: string;
  archived: boolean;
}

export class CareerApiError extends Error {
  code: string;
  status: number;

  constructor(code: string, message: string, status: number) {
    super(message);
    this.code = code;
    this.status = status;
  }
}

async function readError(response: Response): Promise<CareerApiError> {
  const status = response.status;
  try {
    const data = (await response.json()) as { code?: string; message?: string; error?: string };
    if (data && typeof data.message === "string" && data.message.trim() !== "") {
      const code = typeof data.code === "string" && data.code !== "" ? data.code : "failed";
      return new CareerApiError(code, data.message, status);
    }
    if (data && typeof data.error === "string" && data.error.trim() !== "") {
      return new CareerApiError("failed", data.error, status);
    }
  } catch {
    // Fall through to text.
  }
  const text = await response.text();
  if (text.trim() !== "") return new CareerApiError("failed", text.trim(), status);
  return new CareerApiError("failed", `Request failed (${status})`, status);
}

async function readJson<T>(response: Response): Promise<T> {
  if (!response.ok) throw await readError(response);
  return (await response.json()) as T;
}

function jsonHeaders(): HeadersInit {
  return { "Content-Type": "application/json", Accept: "application/json" };
}

export async function fetchCareerProviders(): Promise<CareerProvider[]> {
  const response = await fetch("/api/career/discovery/providers", {
    headers: { Accept: "application/json" },
  });
  const data = await readJson<{ providers: CareerProvider[] }>(response);
  return data.providers;
}

export async function fetchCareerSessions(): Promise<CareerSessionSummary[]> {
  const response = await fetch("/api/career/discovery/sessions", {
    headers: { Accept: "application/json" },
  });
  const data = await readJson<{ sessions: CareerSessionSummary[] }>(response);
  return data.sessions;
}

export async function fetchCareerSession(id: string): Promise<CareerSession> {
  const response = await fetch(`/api/career/discovery/sessions/${id}`, {
    headers: { Accept: "application/json" },
  });
  return readJson<CareerSession>(response);
}

export async function createCareerSession(input: {
  resume_id?: number;
  excerpt?: string;
  excerpt_end?: number;
}): Promise<CareerSession> {
  const response = await fetch("/api/career/discovery/sessions", {
    method: "POST",
    headers: jsonHeaders(),
    body: JSON.stringify(input),
  });
  return readJson<CareerSession>(response);
}

export async function authorizeCareerInference(
  id: string,
  input: { provider: string; model: string; input_hash: string },
): Promise<CareerSession> {
  const response = await fetch(`/api/career/discovery/sessions/${id}/authorize`, {
    method: "POST",
    headers: jsonHeaders(),
    body: JSON.stringify({
      provider: input.provider,
      model: input.model,
      authorize_generation: "yes",
      authorize_external: "yes",
      input_hash: input.input_hash,
    }),
  });
  return readJson<CareerSession>(response);
}

export async function selectCareerLead(id: string, leadId: string, revision: number): Promise<CareerSession> {
  const response = await fetch(`/api/career/discovery/sessions/${id}/select`, {
    method: "POST",
    headers: jsonHeaders(),
    body: JSON.stringify({ lead_id: leadId, revision }),
  });
  return readJson<CareerSession>(response);
}

export async function answerCareerQuestion(id: string, answer: string, revision: number): Promise<CareerSession> {
  const response = await fetch(`/api/career/discovery/sessions/${id}/answer`, {
    method: "POST",
    headers: jsonHeaders(),
    body: JSON.stringify({ answer, revision }),
  });
  return readJson<CareerSession>(response);
}

export async function promoteCareerProposal(id: string, revision: number): Promise<CareerPromoteResult> {
  const response = await fetch(`/api/career/discovery/sessions/${id}/promote`, {
    method: "POST",
    headers: jsonHeaders(),
    body: JSON.stringify({ revision }),
  });
  return readJson<CareerPromoteResult>(response);
}

// Minimal resume picker for the discovery start panel. This reads the same
// library the Resumes slice owns; selection here never changes documents.
export async function fetchCareerResumeChoices(): Promise<CareerResumeChoice[]> {
  const response = await fetch("/api/resumes", { headers: { Accept: "application/json" } });
  const data = await readJson<{
    resumes: { id: number; name: string; archived: boolean }[];
  }>(response);
  return data.resumes.map((resume) => ({ id: resume.id, name: resume.name, archived: resume.archived }));
}
