// Narrow React -> Go JSON transport for the YM-004A Application Workspace.
// Go remains authoritative for validation, durable state, resume
// eligibility, source metadata, and preparation/readiness. React owns
// presentation and disposable drafts; it keeps no applicant records in
// browser storage.

export interface WorkspaceJob {
  id: number;
  url: string;
  host: string;
  title: string;
  company: string;
  description: string;
  stage: string;
  stageLabel: string;
  priority: string;
  priorityLabel: string;
  salaryText: string;
  salaryOrigin: string;
  salaryOriginLabel: string;
  sourceStatus: string;
  sourceStatusLabel: string;
  sourceNote: string;
  sourceCheckedAt: string;
  selectedResumeId: number;
  submissionCount: number;
  lastSubmittedAt: string;
  companyOrHost: string;
  displayTitle: string;
  missingDetails: string[];
  titleManual: boolean;
  companyManual: boolean;
  descriptionManual: boolean;
  salaryManual: boolean;
}

export interface WorkspaceResumeOption {
  id: number;
  name: string;
  kind: string;
  size: number;
  displaySize: string;
  importedAt: string;
  roleFamilyId: number;
  roleFamilyName: string;
  isPrimary: boolean;
  archived: boolean;
  version: number;
  selected: boolean;
  transferReady: boolean;
  transferNote: string;
}

export interface WorkspaceSuggestion {
  status: string;
  familyName: string;
  resumeId: number;
  resumeName: string;
  titleEvidence: string;
  contextEvidence: string;
}

export interface WorkspaceReadiness {
  resumeReady: boolean;
  resumeUnavailable: boolean;
  profileApproved: boolean;
  questionsAvailable: boolean;
  needsAnswerCount: number;
  preparedCount: number;
  profileCount: number;
  employerOnlyCount: number;
  manualReviewCount: number;
  manualWorkCount: number;
  questionSource: string;
  questionPartial: boolean;
  questionAnchor: string;
  coverLetterSaved: boolean;
  browserStatus: string;
}

export interface WorkspacePreparation {
  title: string;
  detail: string;
  recovery: boolean;
  hasScope: boolean;
  external: boolean;
}

export interface WorkspaceNextAction {
  activity: "job" | "documents" | "questions" | "apply" | "applied";
  label: string;
}

export interface WorkspaceSnapshot {
  job: WorkspaceJob;
  resumeOptions: WorkspaceResumeOption[];
  selectedResumeId: number;
  suggestion: WorkspaceSuggestion;
  handoff: { allowed: boolean; reason: string };
  resumeValid: boolean;
  readiness: WorkspaceReadiness;
  preparation: WorkspacePreparation;
  nextAction: WorkspaceNextAction;
  submissions: { count: number };
  letters: { count: number; saved: boolean };
  masterCount: number;
  answerEditingAllowed: boolean;
  desktop: boolean;
  legacyHref: string;
  refreshDisclaimer: string;
}

export interface WorkspaceDetailsInput {
  title: string;
  company: string;
  description: string;
  salary_text: string;
  stage: string;
  priority: string;
}

async function readError(response: Response): Promise<string> {
  try {
    const data = (await response.clone().json()) as { error?: string };
    if (data && typeof data.error === "string" && data.error.trim() !== "") {
      return data.error;
    }
  } catch {
    // Fall through to text.
  }
  const text = await response.text();
  if (text.trim() !== "") return text.trim();
  return `Request failed (${response.status})`;
}

export async function fetchWorkspace(jobId: number): Promise<WorkspaceSnapshot> {
  const response = await fetch(`/api/jobs/${jobId}/workspace`, {
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as WorkspaceSnapshot;
}

export async function saveWorkspaceDetails(
  jobId: number,
  input: WorkspaceDetailsInput,
): Promise<{ saved: boolean; job: WorkspaceJob }> {
  const response = await fetch(`/api/jobs/${jobId}/details`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as { saved: boolean; job: WorkspaceJob };
}

export async function refreshWorkspacePosting(
  jobId: number,
): Promise<{ job: WorkspaceJob; disclaimer: string }> {
  const response = await fetch(`/api/jobs/${jobId}/refresh`, {
    method: "POST",
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as { job: WorkspaceJob; disclaimer: string };
}

// Resume selection reuses the established endpoint also used by Documents:
// POST /api/jobs/{id}/resume (assignResume). Ineligible documents are
// refused server-side without replacing the prior association.
export async function assignWorkspaceResume(jobId: number, resumeId: number): Promise<void> {
  const response = await fetch(`/api/jobs/${jobId}/resume`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ resume_id: resumeId }),
  });
  if (!response.ok) throw new Error(await readError(response));
}

export function workspaceIdFromPath(pathname: string): number | null {
  const match = pathname.match(/^\/jobs\/(\d+)(?:\/|\?|#|$)/);
  if (!match) return null;
  const id = Number.parseInt(match[1] ?? "", 10);
  return Number.isInteger(id) && id > 0 ? id : null;
}
