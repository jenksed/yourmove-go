// Display-only ATS platform support for the locally bundled icons.
// Go remains authoritative for validation, saved state, and provider
// identity: both the Opportunities list and the workspace snapshot carry
// the Go-owned provider key, and the component renders it verbatim. No
// URL/host inference lives here.

// Platforms with a locally bundled icon in frontend/public/ats.
const BUNDLED_ICON_FILE: Record<string, string> = {
  ashby: "ashby.png",
  greenhouse: "greenhouse.svg",
  icims: "icims.svg",
  personio: "personio.svg",
  recruitee: "recruitee.svg",
  smartrecruiters: "smartrecruiters.svg",
  teamtailor: "teamtailor.svg",
  workable: "workable.svg",
  workday: "workday.svg",
};

// Applicant-facing names. Mirrors the Go provider labels, including the two
// recognized platforms without bundled artwork (lever, jobvite) so the
// neutral fallback still announces a meaningful name.
const PLATFORM_LABEL: Record<string, string> = {
  ashby: "Ashby",
  greenhouse: "Greenhouse",
  icims: "iCIMS",
  jobvite: "Jobvite",
  lever: "Lever",
  personio: "Personio",
  recruitee: "Recruitee",
  smartrecruiters: "SmartRecruiters",
  teamtailor: "Teamtailor",
  workable: "Workable",
  workday: "Workday",
};

// normalizeATSProvider accepts only a known Go-owned provider key.
export function normalizeATSProvider(value: string | undefined): string {
  if (value === undefined) return "";
  const key = value.trim().toLowerCase();
  return PLATFORM_LABEL[key] !== undefined ? key : "";
}

// atsPlatformLabel is the accessible name for a provider key.
export function atsPlatformLabel(platform: string): string {
  return PLATFORM_LABEL[platform] ?? "";
}

// atsIconSrc is the bundled icon URL for platforms with approved artwork,
// or null when the neutral fallback applies (unknown sources plus the
// recognized platforms without bundled art).
export function atsIconSrc(platform: string): string | null {
  const file = BUNDLED_ICON_FILE[platform];
  if (file === undefined) return null;
  const env = (import.meta as unknown as { env?: { BASE_URL?: string } }).env;
  const base = env?.BASE_URL ?? "/app/";
  return `${base}ats/${file}`;
}
