// Narrow React -> Go JSON transport for the YM-003B Career Studio slice.
// Go remains authoritative for evidence validation and review, role context
// derivation, inference binding, persisted selections, and staleness. React
// owns presentation, selection, authorization gestures, and disposable drafts.

import { CareerApiError } from "./api";

export type StudioEvidenceStatus = "draft" | "reviewed";

export interface StudioEvidenceRecord {
  id: number;
  title: string;
  situation: string;
  actions: string;
  employer_context: string;
  role_context: string;
  technologies: string;
  outcome: string;
  outcome_state: string;
  period: string;
  source_notes: string;
  source_library_entry_id: number;
  source_question: string;
  source_writing: string;
  source_employer: string;
  source_job_title: string;
  source_host: string;
  status: StudioEvidenceStatus;
  revision: number;
  created_at: string;
  updated_at: string;
}

export interface StudioEvidenceContent {
  title: string;
  situation: string;
  actions: string;
  employer_context: string;
  role_context: string;
  technologies: string;
  outcome: string;
  outcome_state: string;
  period: string;
  source_notes: string;
}

export interface StudioEvidenceRef {
  id: number;
  revision: number;
  title: string;
  bytes: number;
  content_hash: string;
}

export interface StudioRoleItem {
  evidence_id: number;
  evidence_revision: number;
  relevant: boolean;
  capabilities: string[];
  gaps: string[];
  explanation: string;
}

export interface StudioRoleAnalysis {
  role_key: string;
  items: StudioRoleItem[];
  unmet: string[];
}

export interface StudioSavedRole {
  role_hash: string;
  evidence_hash: string;
  notice: string;
  provider: string;
  model: string;
  updated_at: string;
  has_analysis: boolean;
  stale: boolean;
  stale_evidence: boolean;
  stale_role: boolean;
  stale_evidence_ids: number[] | null;
  missing_job: boolean;
  analysis: StudioRoleAnalysis | null;
}

export interface StudioRole {
  job_id: number;
  key: string;
  title: string;
  company: string;
  usable: boolean;
  unusable_reason: string;
  focus: string;
  requirements: string[];
  requirements_count: number;
  role_hash: string;
  saved: StudioSavedRole | null;
}

export interface StudioRolesResponse {
  roles: StudioRole[];
  reviewed: StudioEvidenceRef[];
  policy_name: string;
  max_evidences: number;
}

export interface StudioRoleProfile {
  key: string;
  title: string;
  focus: string;
  requirements: string[];
}

export interface StudioRoleSnapshot {
  job_id: number;
  key: string;
  title: string;
  company: string;
  profile: StudioRoleProfile;
  role_hash: string;
  evidences: StudioEvidenceRef[];
  policy_name: string;
  input_hash: string;
  bytes: number;
  saved: StudioSavedRole | null;
}

export interface StudioConnectionSupport {
  evidence_id: number;
  evidence_revision: number;
  quote: string;
  action: string;
}

export interface StudioConnection {
  id: string;
  capability: string;
  interpretation: string;
  relationship: string;
  supports: StudioConnectionSupport[];
  limitations: string[];
  candidate_use: string;
  follow_up: string;
}

export interface StudioConnectionView {
  connection: StudioConnection;
  challenge: string;
  note: string;
  stale: boolean;
}

export interface StudioSavedSynthesis {
  evidence_hash: string;
  notice: string;
  provider: string;
  model: string;
  updated_at: string;
  has_synthesis: boolean;
  stale: boolean;
  integrity_ok: boolean;
  synthesis: { connections: StudioConnection[] } | null;
  connections: StudioConnectionView[];
  challenges_count: number;
}

export interface StudioSynthesisResponse {
  eligible: StudioEvidenceRef[];
  eligible_overflow: number;
  policy_name: string;
  input_hash: string;
  bytes: number;
  current_hash: string;
  max_evidences: number;
  max_connections: number;
  saved: StudioSavedSynthesis | null;
}

async function readJson<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const status = response.status;
    try {
      const data = (await response.json()) as { code?: string; message?: string };
      if (data && typeof data.message === "string" && data.message.trim() !== "") {
        const code = typeof data.code === "string" && data.code !== "" ? data.code : "failed";
        throw new CareerApiError(code, data.message, status);
      }
    } catch (error) {
      if (error instanceof CareerApiError) throw error;
    }
    throw new CareerApiError("failed", `Request failed (${status})`, status);
  }
  return (await response.json()) as T;
}

function jsonHeaders(): HeadersInit {
  return { "Content-Type": "application/json", Accept: "application/json" };
}

export async function fetchStudioEvidence(): Promise<StudioEvidenceRecord[]> {
  const response = await fetch("/api/career/evidence", { headers: { Accept: "application/json" } });
  const data = await readJson<{ evidence: StudioEvidenceRecord[] }>(response);
  return data.evidence;
}

export async function fetchStudioEvidenceRecord(id: number): Promise<StudioEvidenceRecord> {
  const response = await fetch(`/api/career/evidence/${id}`, { headers: { Accept: "application/json" } });
  const data = await readJson<{ record: StudioEvidenceRecord }>(response);
  return data.record;
}

export async function createStudioEvidence(): Promise<StudioEvidenceRecord> {
  const response = await fetch("/api/career/evidence", { method: "POST", headers: jsonHeaders() });
  const data = await readJson<{ record: StudioEvidenceRecord }>(response);
  return data.record;
}

export async function saveStudioEvidence(
  id: number,
  revision: number,
  content: StudioEvidenceContent,
): Promise<StudioEvidenceRecord> {
  const response = await fetch(`/api/career/evidence/${id}/save`, {
    method: "POST",
    headers: jsonHeaders(),
    body: JSON.stringify({ revision, ...content }),
  });
  const data = await readJson<{ record: StudioEvidenceRecord }>(response);
  return data.record;
}

export async function reviewStudioEvidence(
  id: number,
  revision: number,
  content: StudioEvidenceContent,
): Promise<StudioEvidenceRecord> {
  const response = await fetch(`/api/career/evidence/${id}/review`, {
    method: "POST",
    headers: jsonHeaders(),
    body: JSON.stringify({ revision, confirm_review: "yes", ...content }),
  });
  const data = await readJson<{ record: StudioEvidenceRecord }>(response);
  return data.record;
}

export async function reopenStudioEvidence(id: number, revision: number): Promise<StudioEvidenceRecord> {
  const response = await fetch(`/api/career/evidence/${id}/reopen`, {
    method: "POST",
    headers: jsonHeaders(),
    body: JSON.stringify({ revision, confirm_reopen: "yes" }),
  });
  const data = await readJson<{ record: StudioEvidenceRecord }>(response);
  return data.record;
}

export async function deleteStudioEvidence(id: number, revision: number): Promise<void> {
  const response = await fetch(`/api/career/evidence/${id}/delete`, {
    method: "POST",
    headers: jsonHeaders(),
    body: JSON.stringify({ revision, confirm: "yes" }),
  });
  await readJson<{ deleted: number }>(response);
}

export async function fetchStudioRoles(): Promise<StudioRolesResponse> {
  const response = await fetch("/api/career/roles", { headers: { Accept: "application/json" } });
  return readJson<StudioRolesResponse>(response);
}

export async function fetchStudioRoleSnapshot(key: string, evidenceIds: number[]): Promise<StudioRoleSnapshot> {
  const params = new URLSearchParams({ evidence_ids: evidenceIds.join(",") });
  const response = await fetch(`/api/career/roles/${key}?${params.toString()}`, {
    headers: { Accept: "application/json" },
  });
  return readJson<StudioRoleSnapshot>(response);
}

export async function authorizeStudioRole(
  key: string,
  input: { provider: string; model: string; evidence_ids: number[]; input_hash: string },
): Promise<{ job_id: number; key: string; saved: StudioSavedRole }> {
  const response = await fetch(`/api/career/roles/${key}/authorize`, {
    method: "POST",
    headers: jsonHeaders(),
    body: JSON.stringify({
      provider: input.provider,
      model: input.model,
      evidence_ids: input.evidence_ids,
      input_hash: input.input_hash,
      authorize_generation: "yes",
      authorize_external: "yes",
    }),
  });
  return readJson<{ job_id: number; key: string; saved: StudioSavedRole }>(response);
}

export async function fetchStudioSynthesis(): Promise<StudioSynthesisResponse> {
  const response = await fetch("/api/career/synthesis", { headers: { Accept: "application/json" } });
  return readJson<StudioSynthesisResponse>(response);
}

export async function authorizeStudioSynthesis(input: {
  provider: string;
  model: string;
  input_hash: string;
}): Promise<StudioSynthesisResponse> {
  const response = await fetch("/api/career/synthesis/authorize", {
    method: "POST",
    headers: jsonHeaders(),
    body: JSON.stringify({
      provider: input.provider,
      model: input.model,
      input_hash: input.input_hash,
      authorize_generation: "yes",
      authorize_external: "yes",
    }),
  });
  return readJson<StudioSynthesisResponse>(response);
}

export async function challengeStudioConnection(
  id: string,
  state: string,
  note: string,
): Promise<StudioSynthesisResponse> {
  const response = await fetch(`/api/career/synthesis/challenges/${id}`, {
    method: "POST",
    headers: jsonHeaders(),
    body: JSON.stringify({ state, note }),
  });
  return readJson<StudioSynthesisResponse>(response);
}
