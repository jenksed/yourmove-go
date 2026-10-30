// Feature-local formatting shared by the career discovery and studio pages.

export function statusLabel(status: string): string {
  switch (status) {
    case "leads":
      return "Finding leads";
    case "interview":
      return "Interviewing";
    case "promoted":
      return "Draft saved";
    case "draft":
      return "Draft";
    case "reviewed":
      return "Reviewed";
    default:
      return status;
  }
}

export function formatDateTime(value: string): string {
  if (value === "") return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

export function shortHash(value: string): string {
  if (value === "") return "—";
  return value.length > 12 ? `${value.slice(0, 12)}…` : value;
}
