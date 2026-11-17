// Narrow React -> Go JSON transport for the Opportunities slice.
// Go remains authoritative for validation, IDs, stages, and persistence.

export type JobAttention = "focus" | "saved" | "history";

export interface OpportunityJob {
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
  actionLabel: string;
  actionHref: string;
  actionReason: string;
  selectedResumeId: number;
  submissionCount: number;
  sourceStatus: string;
  companyOrHost: string;
  displayTitle: string;
  attention: JobAttention;
  parkedAt: string;
  // Go-owned ATS platform key ("" when unrecognized) and its accessible
  // label. Rendered verbatim; never re-derived here.
  provider: string;
  providerLabel: string;
}

export interface NextOpportunity {
  jobId: number;
  title: string;
  company: string;
  label: string;
  reason: string;
  href: string;
}

export interface OpportunitiesResponse {
  jobs: OpportunityJob[];
  total: number;
  remaining: number;
  focus: number;
  saved: number;
  history: number;
  applied: number;
  recordedToday: number;
  queueTotal: number;
  recordedPercent: number;
  next: NextOpportunity | null;
  desktop: boolean;
}

export interface BatchItem {
  line: number;
  url: string;
  status: string;
  detail: string;
  jobId: number;
}

export interface BatchEnrichment {
  started: boolean;
  total: number;
}

export interface BatchResult {
  items: BatchItem[];
  added: number;
  duplicates: number;
  invalid: number;
  enrichment?: BatchEnrichment | null;
}

// Precise input reconciliation: only lines the server confirms as added or
// duplicate are safe to remove. Failed (invalid), unprocessed (no matching
// item, e.g. truncated response), and blank-skipped lines stay verbatim so
// the user can fix and retry. Unknown outcomes (HTTP/DB failure) never call
// this helper, so the caller preserves the full input including concurrent
// edits. Line numbers are the original 1-based source lines, blanks
// included, matching Go's importJobBatch numbering.
export function reconcileBulkInput(submitted: string, result: BatchResult): string {
  const lines = submitted.split("\n");
  const removable = new Set<number>();
  for (const item of result.items) {
    if (item.status === "added" || item.status === "duplicate") {
      removable.add(item.line);
    }
  }
  const retained: string[] = [];
  for (let index = 0; index < lines.length; index++) {
    const lineNumber = index + 1;
    if (removable.has(lineNumber)) continue;
    if (lines[index].trim() === "") continue;
    retained.push(lines[index]);
  }
  return retained.join("\n");
}

async function readError(response: Response): Promise<string> {
  const text = await response.text();
  if (text.trim() !== "") return text.trim();
  return `Request failed (${response.status})`;
}

export async function fetchOpportunities(): Promise<OpportunitiesResponse> {
  const response = await fetch("/api/opportunities", {
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as OpportunitiesResponse;
}

export async function addOpportunity(
  url: string,
  title: string,
  focus = false,
): Promise<{ id: number; added: boolean; duplicate: boolean }> {
  const response = await fetch("/api/opportunities", {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ url, title, focus }),
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as {
    id: number;
    added: boolean;
    duplicate: boolean;
  };
}

export async function importOpportunities(raw: string): Promise<BatchResult> {
  const response = await fetch("/api/opportunities/import", {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ urls: raw }),
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as BatchResult;
}

export async function moveOpportunities(
  ids: number[],
  attention: "focus" | "saved",
): Promise<{ moved: number; attention: string }> {
  const response = await fetch("/api/opportunities/attention", {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ ids, attention }),
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as { moved: number; attention: string };
}

export async function deleteOpportunities(ids: number[]): Promise<{ deleted: number }> {
  const response = await fetch("/api/opportunities/delete", {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ ids, confirm: true }),
  });
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as { deleted: number };
}

export async function openPosting(id: number): Promise<void> {
  const response = await fetch(`/api/opportunities/${id}/open`, {
    method: "POST",
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new Error(await readError(response));
}
