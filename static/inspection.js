// Wails/browser status only. Every application mutation stays on server-owned forms.
(() => {
  const host = document.querySelector('[data-browser-watch]');
  if (!host) return;
  const id = host.dataset.jobId;
  const title = host.querySelector('[data-handoff-title]');
  const detail = host.querySelector('[data-handoff-detail]');
  let last = Number(host.dataset.observedId || 0);
  let resumeStatus = host.dataset.resumeStatus || '';
  let phase = host.dataset.phase || '';
  const historical = new URLSearchParams(location.search).has('browser');
  let pending = false;
  async function poll() {
    if (pending) return;
    pending = true;
    try {
      const response = await fetch(`/jobs/${id}/browser-status`, {cache:'no-store'});
      if (!response.ok) throw new Error('status unavailable');
      const state = await response.json();
      if (title && state.handoff_title) title.textContent = state.handoff_title;
      if (detail && state.handoff_detail) detail.textContent = state.handoff_detail;
      // Refresh the server-authored action controls when a browser closes,
      // an upload reaches an actionable outcome, or the recovery state changes.
      const changed = (state.resume_status || '') !== resumeStatus || (state.phase || '') !== phase;
      if (!state.active || changed || (!historical && state.latest_id > 0 && state.latest_id !== last)) {
        location.reload();
        return;
      }
      resumeStatus = state.resume_status || '';
      phase = state.phase || '';
      last = state.latest_id || 0;
    } catch (_) {
      if (detail) detail.textContent = 'Live status unavailable. Use Recheck this page, or continue in Chromium.';
    } finally { pending = false; }
  }
  document.addEventListener('visibilitychange', () => { if (!document.hidden) void poll(); });
  setInterval(() => { if (!document.hidden) void poll(); }, 2500);
})();
