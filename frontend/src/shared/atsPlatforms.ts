// Display-only ATS platform recognition for the locally bundled icons.
// Go remains authoritative for validation, saved state, and browser
// behavior: when a Go-owned provider key is supplied it is rendered
// verbatim and never re-derived here. The URL/host mapping below is a
// narrow, explicit allowlist used only where no Go provider field exists
// (the workspace snapshot), and it never touches managed-browser rules.

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

function normalizeProvider(value: string | undefined): string {
  if (value === undefined) return "";
  const key = value.trim().toLowerCase();
  return PLATFORM_LABEL[key] !== undefined ? key : "";
}

function normalizeHost(value: string): string {
  const host = value.trim().toLowerCase().replace(/\.$/, "");
  if (host === "" || host.length > 253 || /[\s@/]/.test(host)) return "";
  if (!/^[a-z0-9_.-]+$/.test(host)) return "";
  return host;
}

// token mirrors Go's safeToken for single tenant labels.
function isToken(value: string): boolean {
  return value !== "" && /^[A-Za-z0-9_-]+$/.test(value);
}

function tenantOf(host: string, suffix: string): string {
  if (!host.endsWith(suffix)) return "";
  const tenant = host.slice(0, host.length - suffix.length);
  if (tenant === "" || tenant.includes(".")) return "";
  return isToken(tenant) ? tenant : "";
}

function isWorkdayHost(host: string): boolean {
  const labels = host.split(".");
  if (
    labels.length === 4 &&
    labels[2] === "myworkdayjobs" &&
    labels[3] === "com" &&
    /^wd[0-9]+$/.test(labels[1] ?? "") &&
    /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(labels[0] ?? "")
  ) {
    return true;
  }
  return (
    labels.length === 3 &&
    labels[1] === "myworkdaysite" &&
    labels[2] === "com" &&
    /^wd[0-9]+$/.test(labels[0] ?? "")
  );
}

function platformFromHost(host: string): string {
  switch (host) {
    case "jobs.ashbyhq.com":
    case "www.ashbyhq.com":
      return "ashby";
    case "job-boards.greenhouse.io":
    case "boards.greenhouse.io":
      return "greenhouse";
    case "jobs.lever.co":
    case "jobs.eu.lever.co":
      return "lever";
    case "jobs.smartrecruiters.com":
      return "smartrecruiters";
    case "apply.workable.com":
      return "workable";
    case "jobs.jobvite.com":
      return "jobvite";
    default:
      break;
  }
  if (tenantOf(host, ".recruitee.com") !== "") return "recruitee";
  if (tenantOf(host, ".teamtailor.com") !== "") return "teamtailor";
  if (tenantOf(host, ".icims.com") !== "") return "icims";
  if (tenantOf(host, ".jobs.personio.de") !== "" || tenantOf(host, ".jobs.personio.com") !== "") {
    return "personio";
  }
  if (isWorkdayHost(host)) return "workday";
  return "";
}

export interface ATSIdentity {
  provider?: string;
  url?: string;
  host?: string;
}

// resolveATSPlatform reports the recognized platform key or "".
// An explicitly supplied provider (even "") is trusted verbatim: it is the
// Go-owned verdict. Derivation from URL/host only applies when no provider
// was supplied at all.
export function resolveATSPlatform(identity: ATSIdentity): string {
  if (identity.provider !== undefined) return normalizeProvider(identity.provider);
  let host = identity.host !== undefined ? normalizeHost(identity.host) : "";
  if (host === "" && identity.url !== undefined) {
    try {
      host = normalizeHost(new URL(identity.url).hostname);
    } catch {
      host = "";
    }
  }
  if (host === "") return "";
  return platformFromHost(host);
}

// atsPlatformLabel is the accessible name for a resolved platform key.
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
