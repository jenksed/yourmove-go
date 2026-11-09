// In-app unsaved-work registry for the React workspace.
//
// Editors report dirty drafts per scope; navigation guards read the
// aggregate synchronously, and the desktop host is notified so native
// window close and Cmd+Q can confirm instead of silently discarding.
// Browser tabs keep their beforeunload listeners as a fallback.
//
// A fresh module instance means a fresh page load: reset the host flag so
// a previous page cannot leave a stale prompt behind. No framework.
const scopes = new Map<string, boolean>();
let lastReported = false;

function aggregate(): boolean {
  for (const dirty of scopes.values()) {
    if (dirty) return true;
  }
  return false;
}

function postHost(dirty: boolean): void {
  if (dirty === lastReported) return;
  lastReported = dirty;
  // Fire-and-forget: the desktop asset handler records the flag for its
  // native close guards. Failures leave beforeunload as the fallback.
  void fetch("/api/desktop/unsaved", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ dirty }),
  }).catch(() => undefined);
}

export function setUnsaved(scope: string, dirty: boolean): void {
  if (dirty) scopes.set(scope, true);
  else scopes.delete(scope);
  postHost(aggregate());
}

export function hasUnsaved(): boolean {
  return aggregate();
}

// A fresh page has no drafts yet. Clear any flag left by the previous
// page; its editors report explicitly if edits are in progress.
void fetch("/api/desktop/unsaved", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ dirty: false }),
}).catch(() => undefined);
