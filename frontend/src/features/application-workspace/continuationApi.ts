// YM-004C React -> Go JSON transport for employer continuation.
// Go remains authoritative for browser operations, transfer eligibility,
// authorization, submission records, and durable state. React owns
// presentation and disposable interaction state; it never controls CDP.

export interface ContinuationAnswer {
  key: string;
  label: string;
  value: string;
  source: string;
}

export interface ContinuationPreparation {
  title: string;
  detail: string;
  recovery: boolean;
  action: string;
  external: boolean;
  includeResume: boolean;
  includeLetter: boolean;
  includeAnswers: boolean;
  answerSummary: string;
  answerReviewHash: string;
}

export interface BrowserOutcome {
  label?: string;
  key?: string;
  status: string;
  detail?: string;
}

export interface ContinuationBrowser {
  active: boolean;
  engine: string;
  phase: string;
  message: string;
  latest_id: number;
  answer_status: string;
  answer_outcomes?: BrowserOutcome[] | null;
  letter_status: string;
  letter_message: string;
  resume_status: string;
  resume_retryable: boolean;
  resume_message: string;
  resume_name: string;
  contact_status: string;
  contact_summary: string;
  contact_outcomes?: BrowserOutcome[] | null;
}

export interface ContinuationResume {
  id: number;
  name: string;
  kind: string;
  version: number;
  sha256: string;
  valid: boolean;
}

export interface ContinuationProfile {
  approved: boolean;
  revision: number;
  fullName: string;
  email: string;
  phone: string;
}

export interface ContinuationLetter {
  id: number;
  provider: string;
  model: string;
  contentHash: string;
  editedAt: string;
  hasBody: boolean;
}

export interface ContinuationOther {
  id: number;
  title: string;
  url: string;
  stage: string;
}

export interface ContinuationSaved {
  checkedAt: string;
  fieldCount: number;
  url: string;
}

export interface ContinuationSubmission {
  id: number;
  submittedAt: string;
  resumeName: string;
  resumeSha256: string;
  notes: string;
}

export interface ContinuationSnapshot {
  destination: string;
  ownsPosting: boolean;
  desktop: boolean;
  applied: boolean;
  closed: boolean;
  editingAllowed: boolean;
  preparation: ContinuationPreparation;
  handoff: { allowed: boolean; reason: string };
  browser: ContinuationBrowser;
  otherBrowser: ContinuationOther | null;
  savedBrowser: ContinuationSaved | null;
  resume: ContinuationResume | null;
  profile: ContinuationProfile;
  letter: ContinuationLetter | null;
  pendingAnswers: ContinuationAnswer[];
  previousCount: number;
  awaitingReconciliation: number;
  questionsAvailable: boolean;
  submissions: ContinuationSubmission[];
  submissionCount: number;
  legacyHref: string;
}

export interface ContinueItem {
  item: string;
  ok: boolean;
  status: number;
  detail: string;
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

export async function fetchContinuation(jobId: number): Promise<ContinuationSnapshot> {
  const response = await fetch(`/api/jobs/${jobId}/continuation`, {
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as ContinuationSnapshot;
}

export type BrowserAction = "start" | "observe" | "restart" | "retry-resume" | "stop";

export async function browserAction(
  jobId: number,
  action: BrowserAction,
  confirmRestart = false,
): Promise<{ ok: boolean; message?: string; browser: ContinuationBrowser }> {
  const response = await fetch(`/api/jobs/${jobId}/browser`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ action, confirm_restart: confirmRestart ? "yes" : "" }),
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as { ok: boolean; message?: string; browser: ContinuationBrowser };
}

export interface ContinueInput {
  destination: string;
  include_resume: boolean;
  resume_id: string;
  resume_hash: string;
  transfer_contacts: boolean;
  profile_revision: string;
  include_letter: boolean;
  draft_id: string;
  body_hash: string;
  mode: string;
  include_answers: boolean;
  review_hash: string;
}

export async function authorizeContinue(jobId: number, input: ContinueInput): Promise<{ items: ContinueItem[]; browser: ContinuationBrowser }> {
  const response = await fetch(`/api/jobs/${jobId}/continue`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as { items: ContinueItem[]; browser: ContinuationBrowser };
}

export interface SubmissionReview {
  token: string;
  jobId: number;
  resumeId: number;
  resumeName: string;
  resumeHash: string;
}

export async function mintSubmissionReview(jobId: number): Promise<SubmissionReview> {
  const response = await fetch(`/api/jobs/${jobId}/submission-review`, {
    method: "POST",
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as SubmissionReview;
}

export async function recordSubmission(jobId: number, notes: string, token: string): Promise<void> {
  const response = await fetch(`/api/jobs/${jobId}/submit`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ confirm: "yes", notes, submission_token: token }),
  });
  if (!response.ok) throw new Error(await readError(response));
}

export interface SubmissionsSnapshot {
  submissions: ContinuationSubmission[];
  count: number;
  next: { JobID: number; Title: string; Href: string; Label: string; Reason: string } | null;
}

export async function fetchSubmissions(jobId: number): Promise<SubmissionsSnapshot> {
  const response = await fetch(`/api/jobs/${jobId}/submissions`, {
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as SubmissionsSnapshot;
}

export async function moveNext(jobId: number): Promise<{ target: string }> {
  const response = await fetch(`/api/jobs/${jobId}/next`, {
    method: "POST",
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as { target: string };
}
