// Wails/browser status only. Every application mutation stays on server-owned forms.
(() => {
  const host = document.querySelector('[data-browser-watch]');
  if (!host) return;
  const id = host.dataset.jobId;
  const title = host.querySelector('[data-handoff-title]');
  const detail = host.querySelector('[data-handoff-detail]');
  let last = Number(host.dataset.observedId || 0);
  let resumeStatus = host.dataset.resumeStatus || '';
  let contactStatus = host.dataset.contactStatus || '';
  let letterStatus = host.dataset.letterStatus || '';
  let answerStatus = host.dataset.answerStatus || '';
  const preparation = host.hasAttribute('data-preparation-watch');
  let phase = host.dataset.phase || '';
  const historical = new URLSearchParams(location.search).has('browser');
  let pending = false;
  let unsavedEdits = false;
  for (const event of ['input', 'change']) document.addEventListener(event, e => {
    if (e.target.closest('form[action$="/answers/save"], form[action$="/cover-letter/save"]')) unsavedEdits = true;
  });
  function offerRefresh(message) {
    if (host.querySelector('[data-new-questions]')) return;
    const notice = document.createElement('p');
    notice.dataset.newQuestions = '';
    const link = document.createElement('a');
    link.href = location.href;
    link.textContent = message;
    notice.append(link, ' · Save any edits here first. Your employer browser will stay open.');
    host.append(notice);
  }
  async function poll() {
    if (pending || document.documentElement.dataset.pendingAction === 'yes') return;
    pending = true;
    try {
      const response = await fetch(`/jobs/${id}/browser-status`, {cache:'no-store'});
      if (!response.ok) throw new Error('status unavailable');
      const state = await response.json();
      if (title && state.handoff_title) title.textContent = state.handoff_title;
      if (detail && state.handoff_detail) detail.textContent = state.handoff_detail;
      // Refresh the server-authored action controls when a browser closes,
      // an upload reaches an actionable outcome, or the recovery state changes.
      const changed = (state.answer_status || '') !== answerStatus || (state.letter_status || '') !== letterStatus || (state.resume_status || '') !== resumeStatus || (state.contact_status || '') !== contactStatus || ((!preparation || state.phase === 'attention' || phase === 'attention') && (state.phase || '') !== phase);
      if (!state.active || changed || (!preparation && !historical && state.latest_id > 0 && state.latest_id !== last)) {
        if (unsavedEdits || document.querySelector('form[data-dirty="true"], [data-answer-recovery]')) {
          offerRefresh('Updated transfer results are available');
          return;
        }
        location.reload();
        return;
      }
      if (preparation && state.latest_id > 0 && state.latest_id !== last) offerRefresh('Refresh discovered questions');
      resumeStatus = state.resume_status || '';
      contactStatus = state.contact_status || '';
      letterStatus = state.letter_status || '';
      answerStatus = state.answer_status || '';
      phase = state.phase || '';
      last = state.latest_id || 0;
    } catch (_) {
      if (detail) detail.textContent = 'Live status unavailable. Use Recheck this page, or continue in Chromium.';
    } finally { pending = false; }
  }
  document.addEventListener('visibilitychange', () => { if (!document.hidden) void poll(); });
  setInterval(() => { if (!document.hidden) void poll(); }, 2500);
})();
