// Narrow React -> Go JSON transport for the Resumes slice.
// Go remains authoritative for validation, document identity, routing,
// archival, deletion refusals, and persistence.

export interface ResumeItem {
  id: number;
  name: string;
  kind: string;
  size: number;
  displaySize: string;
  importedAt: string;
  roleFamilyId: number;
  roleFamilyName: string;
  isPrimary: boolean;
  isMaster: boolean;
  archived: boolean;
  archivedAt: string;
  seriesId: number;
  version: number;
}

export interface ResumeFamily {
  id: number;
  name: string;
}

export interface ResumesResponse {
  resumes: ResumeItem[];
  families: ResumeFamily[];
  desktop: boolean;
  total: number;
}

export interface ResumeOption extends ResumeItem {
  selected: boolean;
}

export interface ResumeOptionsResponse {
  job: { id: number; title: string; company: string; selectedResumeId: number };
  options: ResumeOption[];
  selectedResumeId: number;
}

async function readError(response: Response): Promise<string> {
  try {
    const data = (await response.json()) as { error?: string };
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

async function readJson<T>(response: Response): Promise<T> {
  if (!response.ok) throw new Error(await readError(response));
  return (await response.json()) as T;
}

function jsonHeaders(): HeadersInit {
  return { "Content-Type": "application/json", Accept: "application/json" };
}

export async function fetchResumes(): Promise<ResumesResponse> {
  const response = await fetch("/api/resumes", { headers: { Accept: "application/json" } });
  return readJson<ResumesResponse>(response);
}

export async function fetchResumeOptions(jobId: number): Promise<ResumeOptionsResponse> {
  const response = await fetch(`/api/jobs/${jobId}/resume-options`, {
    headers: { Accept: "application/json" },
  });
  return readJson<ResumeOptionsResponse>(response);
}

export async function assignJobResume(jobId: number, resumeId: number): Promise<void> {
  const response = await fetch(`/api/jobs/${jobId}/resume`, {
    method: "POST",
    headers: jsonHeaders(),
    body: JSON.stringify({ resume_id: resumeId }),
  });
  if (!response.ok) throw new Error(await readError(response));
}

export async function importResume(file: File): Promise<{ id: number; duplicate: boolean }> {
  const form = new FormData();
  form.append("resume", file, file.name);
  const response = await fetch("/api/resumes/import", { method: "POST", body: form });
  return readJson(response);
}

export async function importResumeNative(): Promise<{ id: number; duplicate: boolean }> {
  const response = await fetch("/api/resumes/import-native", { method: "POST" });
  return readJson(response);
}

export async function setResumeFamily(id: number, familyId: number, primary: boolean): Promise<void> {
  const response = await fetch(`/api/resumes/${id}/family`, {
    method: "POST",
    headers: jsonHeaders(),
    body: JSON.stringify({ family_id: familyId, primary }),
  });
  if (!response.ok) throw new Error(await readError(response));
}

export async function setResumeMaster(id: number, master: boolean): Promise<void> {
  const response = await fetch(`/api/resumes/${id}/master`, {
    method: "POST",
    headers: jsonHeaders(),
    body: JSON.stringify({ master }),
  });
  if (!response.ok) throw new Error(await readError(response));
}

export async function setResumeArchived(id: number, action: "archive" | "restore", confirm: boolean): Promise<void> {
  const response = await fetch(`/api/resumes/${id}/archive`, {
    method: "POST",
    headers: jsonHeaders(),
    body: JSON.stringify({ action, confirm }),
  });
  if (!response.ok) throw new Error(await readError(response));
}

export async function reviseResume(id: number, file: File): Promise<{ id: number }> {
  const form = new FormData();
  form.append("resume", file, file.name);
  const response = await fetch(`/api/resumes/${id}/revise`, { method: "POST", body: form });
  return readJson(response);
}

export async function reviseResumeNative(id: number): Promise<{ id: number }> {
  const response = await fetch(`/api/resumes/${id}/revise-native`, { method: "POST" });
  return readJson(response);
}

export async function deleteResume(id: number): Promise<void> {
  const response = await fetch(`/api/resumes/${id}/delete`, {
    method: "POST",
    headers: jsonHeaders(),
    body: JSON.stringify({ confirm: true, confirmPhrase: "DELETE" }),
  });
  if (!response.ok) throw new Error(await readError(response));
}
