/* Small progressive enhancements only; every mutating form remains server-owned. */
(() => {
  'use strict';
  const key = 'yourmove:theme';
  const themeControl = document.querySelector('[data-theme-choice]');
  const setTheme = (value) => {
    const theme = value === 'dark' || value === 'light' ? value : 'system';
    document.documentElement.dataset.theme = theme === 'system' ? '' : theme;
    if (theme === 'system') document.documentElement.removeAttribute('data-theme');
    const dark = theme === 'dark' || (theme === 'system' && window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches);
    document.querySelector('meta[name="theme-color"]')?.setAttribute('content', dark ? '#141a19' : '#f7f7f3');
  };
  let theme = 'system';
  try { const previous = localStorage.getItem(key); if (previous === 'dark' || previous === 'light') theme = previous; } catch (_) {}
  setTheme(theme);
  if (themeControl) {
    themeControl.value = theme;
    themeControl.addEventListener('change', () => {
      setTheme(themeControl.value);
      try { localStorage.setItem(key, themeControl.value); } catch (_) {}
    });
    if (window.matchMedia) window.matchMedia('(prefers-color-scheme: dark)').addEventListener?.('change', () => {
      if (themeControl.value === 'system') setTheme('system');
    });
  }

  const addDisclosure = document.getElementById('add-opportunity');
  document.querySelectorAll('[data-open-add]').forEach((link) => link.addEventListener('click', () => {
    if (addDisclosure) { addDisclosure.open = true; requestAnimationFrame(() => document.getElementById('job_url')?.focus()); }
  }));
  if (addDisclosure && location.hash === '#add-opportunity') addDisclosure.open = true;
  document.querySelectorAll('[data-open-details]').forEach((link) => link.addEventListener('click', () => {
    const target = document.getElementById(link.dataset.openDetails);
    if (target) target.open = true;
  }));

  // Client-side presentation filtering. Server output stays complete without JS.
  const list = document.querySelector('[data-job-list]');
  if (list) {
    const rows = Array.from(list.querySelectorAll('[data-job-row]'));
    const search = document.querySelector('[data-job-search]');
    const stage = document.querySelector('[data-job-stage]');
    const sort = document.querySelector('[data-job-sort]');
    const status = document.getElementById('results-status');
    const empty = document.getElementById('filtered-empty');
    const storageKey = 'yourmove:opportunities:filters';
    try {
      const saved = JSON.parse(sessionStorage.getItem(storageKey) || '{}');
      if (search && typeof saved.query === 'string') search.value = saved.query;
      if (stage && ['all','review','ready','applied','closed'].includes(saved.stage)) stage.value = saved.stage;
      if (sort && ['priority','newest','company'].includes(saved.sort)) sort.value = saved.sort;
    } catch (_) {}
    const stateOrder = {ready: 0, review: 1, applied: 2, closed: 3};
    const priorityOrder = {high: 0, normal: 1, low: 2};
    const render = () => {
      const query = (search?.value || '').toLocaleLowerCase().trim();
      const currentStage = stage?.value || 'all';
      const ordering = sort?.value || 'priority';
      const ordered = rows.slice().sort((a,b) => {
        if (ordering === 'newest') return Number(b.dataset.order) - Number(a.dataset.order);
        if (ordering === 'company') return (a.dataset.company || a.dataset.search).localeCompare(b.dataset.company || b.dataset.search) || Number(b.dataset.order) - Number(a.dataset.order);
        const aClosed = ["applied", "closed"].includes(a.dataset.stage) ? 1 : 0;
        const bClosed = ["applied", "closed"].includes(b.dataset.stage) ? 1 : 0;
        return aClosed - bClosed ||
          (priorityOrder[a.dataset.priority] ?? 9) - (priorityOrder[b.dataset.priority] ?? 9) ||
          (stateOrder[a.dataset.stage] ?? 9) - (stateOrder[b.dataset.stage] ?? 9) || Number(b.dataset.order) - Number(a.dataset.order);
      });
      ordered.forEach((row) => list.appendChild(row));
      let visible = 0;
      rows.forEach((row) => {
        row.hidden = (currentStage !== 'all' && row.dataset.stage !== currentStage) || !!(query && !row.dataset.search.toLocaleLowerCase().includes(query));
        // A filtered-out selection must not become an invisible destructive action.
        if (row.hidden) { const picked = row.querySelector('input[name="job_ids"]'); if (picked) picked.checked = false; }
        if (!row.hidden) visible++;
      });
      if (status) status.textContent = `${visible} of ${rows.length} opportunities shown`;
      if (empty) empty.hidden = visible !== 0;
      try { sessionStorage.setItem(storageKey, JSON.stringify({query: search?.value || '', stage: currentStage, sort: ordering})); } catch (_) {}
    };
    [search,stage,sort].forEach((el) => el?.addEventListener(el === search ? 'input' : 'change', render));
    document.querySelector('[data-clear-filters]')?.addEventListener('click', () => {
      if (search) search.value = '';
      if (stage) stage.value = 'all';
      if (sort) sort.value = 'priority';
      render(); search?.focus();
    });
    render();
  }

  // Hash-addressable tabs. Without JavaScript, all three sections remain readable.
  const tabs = Array.from(document.querySelectorAll('.workspace-tab[data-tab-target]'));
  const panels = Array.from(document.querySelectorAll('[data-workspace-panel]'));
  if (tabs.length && panels.length) {
    const names = new Set(panels.map((panel) => panel.dataset.workspacePanel));
    const current = () => names.has(location.hash.slice(1)) ? location.hash.slice(1) : 'overview';
    const show = (name, focus = false) => {
      const target = names.has(name) ? name : 'overview';
      document.documentElement.classList.add('tabs-enabled');
      tabs.forEach((tab) => {
        const selected = tab.dataset.tabTarget === target;
        tab.classList.toggle('is-active', selected);
        tab.setAttribute('aria-selected', String(selected));
        tab.tabIndex = selected ? 0 : -1;
      });
      panels.forEach((panel) => panel.hidden = panel.dataset.workspacePanel !== target);
      if (focus) document.getElementById(target)?.focus({preventScroll:true});
    };
    tabs.forEach((tab) => {
      tab.setAttribute('role', 'tab');
      tab.id = `workspace-tab-${tab.dataset.tabTarget}`;
      tab.setAttribute('aria-controls', tab.dataset.tabTarget);
      tab.addEventListener('click', (event) => {
        event.preventDefault();
        const target = tab.dataset.tabTarget;
        try { history.replaceState(null, '', `#${target}`); } catch (_) { location.hash = target; }
        show(target);
      });
      tab.addEventListener('keydown', (event) => {
        if (!['ArrowRight', 'ArrowLeft', 'Home', 'End'].includes(event.key)) return;
        event.preventDefault();
        const index = tabs.indexOf(tab);
        const next = event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : (index + (event.key === 'ArrowRight' ? 1 : tabs.length - 1)) % tabs.length;
        tabs[next].focus(); tabs[next].click();
      });
    });
    const holder = document.querySelector('[data-workspace-tabs]');
    holder?.setAttribute('role', 'tablist');
    panels.forEach((panel) => {
      panel.setAttribute('role','tabpanel');
      panel.setAttribute('aria-labelledby', `workspace-tab-${panel.dataset.workspacePanel}`);
    });
    show(current());
    window.addEventListener('hashchange', () => show(current()));
    document.querySelectorAll('[data-tab-target]:not(.workspace-tab)').forEach((link) => link.addEventListener('click', () => {
      const target = link.dataset.tabTarget;
      if (names.has(target)) show(target);
    }));
  }

  document.querySelectorAll('form[method="POST"]').forEach((form) => {
    form.addEventListener('submit', () => {
      // Native browser validation already ran; prevent accidental double clicks while loading.
      const button = form.querySelector('button[type="submit"]');
      if (!button) return;
      button.disabled = true;
      button.setAttribute('aria-busy', 'true');
    });
  });
})();

// Writing desk: presentation state only. The Go POST endpoint is authoritative.
(() => {
  const provider = document.getElementById('letter_provider');
  const model = document.getElementById('letter_model');
  const modelHint = document.getElementById('letter_model_hint');
  const form = document.querySelector('[data-ai-generate]');
  if (!provider || !model || !form) return;
  const storageKey = 'yourmove.writer.provider.v1';
  const modelKey = 'yourmove.writer.models.v1';
  const defaults = Object.fromEntries([...provider.options].filter(o => o.value).map(o => [o.value, o.dataset.defaultModel || '']));
  let models = {};
  try { models = JSON.parse(localStorage.getItem(modelKey) || '{}'); } catch (_) {}
  const applyProvider = () => {
    model.value = models[provider.value] || defaults[provider.value] || '';
    model.placeholder = 'Enter the model name';
    if (modelHint) {
      modelHint.textContent = ({
        codex: 'Use default for the model selected in Codex, or enter a supported model.',
        'claude-cli': 'Choose sonnet, haiku, opus (when available), or another Claude model.',
        ollama: 'Choose a model installed in Ollama (check ollama list).',
        openai: 'Choose an OpenAI API model. Usage is billed separately.',
        anthropic: 'Choose an Anthropic API model. Usage is billed separately.',
        openrouter: 'Use the provider/model ID from OpenRouter.',
        compatible: 'Use the model supported by your configured endpoint.'
      })[provider.value] || 'Enter a model supported by this provider.';
    }
  };
  try {
    const preferred = localStorage.getItem(storageKey);
    if (preferred && defaults[preferred] !== undefined) provider.value = preferred;
  } catch (_) {}
  applyProvider();
  const resumeChoice = document.getElementById('letter_resume_id');
  const resumeStatus = document.querySelector('[data-ai-role-check]');
  const resumeWarning = document.querySelector('[data-ai-role-warning]');
  if (resumeChoice && resumeStatus) {
    const initialLabel = resumeStatus.innerHTML;
    const updateResume = () => {
      const alternate = resumeChoice.value && resumeChoice.value !== resumeChoice.dataset.applicationResume;
      resumeStatus.classList.toggle('is-missing', !alternate && resumeStatus.getAttribute('aria-label') === 'Selected resume unavailable');
      resumeStatus.innerHTML = alternate ? '<span aria-hidden="true">✓</span> Alternate writing resume selected' : initialLabel;
      if (resumeWarning) resumeWarning.hidden = Boolean(alternate);
    };
    resumeChoice.addEventListener('change', updateResume);
    updateResume();
  }
  provider.addEventListener('change', () => {
    try { localStorage.setItem(storageKey, provider.value); } catch (_) {}
    applyProvider();
  });
  model.addEventListener('change', () => {
    models[provider.value] = model.value;
    try { localStorage.setItem(modelKey, JSON.stringify(models)); } catch (_) {}
  });
  form.addEventListener('submit', () => {
    const progress = form.querySelector('[data-ai-progress]');
    const button = form.querySelector('[data-ai-submit]');
    if (progress) progress.textContent = 'Writing with ' + provider.options[provider.selectedIndex].text.split(' · ')[0] + '…';
    if (button) button.textContent = 'Writing your draft…';
  });
  const draft = document.getElementById('letter-body');
  const count = document.querySelector('[data-ai-word-count]');
  if (draft && count) {
    const update = () => { const words = draft.value.trim() ? draft.value.trim().split(/\s+/).length : 0; count.textContent = `${words} words · editable draft`; };
    update(); draft.addEventListener('input', update);
  }
})();

// Question workbench: visibility and navigation over the authoritative Go forms.
(() => {
  const workbench = document.querySelector('[data-question-workbench]');
  if (workbench) {
    const returning = new URLSearchParams(location.search).has('question');
    const panels = Array.from(workbench.querySelectorAll('[data-question-panel]'));
    const index = workbench.querySelector('[data-question-index]');
    const chooser = workbench.querySelector('[data-question-chooser]');
    const previous = workbench.querySelector('[data-question-previous]');
    const next = workbench.querySelector('[data-question-next]');
    const position = workbench.querySelector('[data-question-position]');
    const groups = new Map();
    const links = new Map();
    let selected = Math.max(0, panels.findIndex(panel => panel.dataset.questionKey === workbench.dataset.selectedQuestion));
    const show = (number, focus = true) => {
      if (number < 0 || number >= panels.length) return;
      selected = number;
      panels.forEach((panel, i) => panel.hidden = i !== selected);
      links.forEach((link, panel) => {
        if (panel === panels[selected]) link.setAttribute('aria-current', 'true');
        else link.removeAttribute('aria-current');
      });
      chooser.value = panels[selected].dataset.questionKey;
      previous.disabled = selected === 0;
      next.disabled = selected === panels.length - 1;
      position.textContent = `Question ${selected + 1} of ${panels.length} · observed order`;
      if (focus) {
        panels[selected].focus({preventScroll:true});
        const bounds = panels[selected].getBoundingClientRect();
        if (bounds.top < 0 || bounds.top >= innerHeight) panels[selected].scrollIntoView({block:'start', behavior:'instant'});
      }
    };
    for (const status of ['Needs local answer', 'Saved locally or provided by an approved source', 'Requires manual completion in Chromium']) {
      const members = panels.filter(panel => panel.dataset.questionStatus === status);
      if (!members.length) continue;
      const group = document.createElement('div');
      const heading = document.createElement('h4');
      heading.textContent = `${status} (${members.length})`;
      group.append(heading);
      const options = document.createElement('optgroup');
      options.label = status;
      members.forEach(panel => {
        const link = document.createElement('a');
        link.href = `#${panel.id}`;
        link.textContent = panel.dataset.questionLabel;
        link.dataset.questionLink = panel.dataset.questionKey;
        link.addEventListener('click', event => { event.preventDefault(); show(panels.indexOf(panel)); });
        group.append(link);
        links.set(panel, link);
        const option = document.createElement('option');
        option.value = panel.dataset.questionKey;
        option.textContent = `${panels.indexOf(panel) + 1}. ${panel.dataset.questionLabel}`;
        options.append(option);
      });
      index.append(group);
      chooser.append(options);
      groups.set(status, members.length);
    }
    workbench.querySelector('[data-question-summary]').textContent = `${groups.get('Needs local answer') || 0} need a local answer · ${groups.get('Saved locally or provided by an approved source') || 0} prepared · ${groups.get('Requires manual completion in Chromium') || 0} manual`;
    chooser.addEventListener('change', () => show(panels.findIndex(panel => panel.dataset.questionKey === chooser.value)));
    previous.addEventListener('click', () => show(selected - 1));
    next.addEventListener('click', () => show(selected + 1));
    workbench.addEventListener('question:reveal', event => show(panels.indexOf(event.target.closest('[data-question-panel]'))));
    workbench.querySelector('[data-review-ready]').addEventListener('click', event => {
      event.preventDefault();
      const scope = document.getElementById('application-action');
      scope.focus({preventScroll:true});
      scope.scrollIntoView({block:'start', behavior:'instant'});
    });
    index.hidden = false;
    chooser.closest('.question-chooser').hidden = false;
    position.hidden = false;
    workbench.querySelector('.question-navigation').hidden = false;
    workbench.classList.add('questions-enabled');
    show(selected, false);
    // Save-return focus is explicit; opening Prepare normally doesn't steal focus.
    if (returning && location.hash === '#prepare') {
      // The browser applies #prepare after deferred scripts. Restore the
      // question after that native fragment navigation has finished.
      const restore = () => {
        panels[selected].focus({preventScroll:true});
        workbench.scrollIntoView({block:'start', behavior:'instant'});
      };
      if (document.readyState === 'complete') restore();
      else window.addEventListener('load', () => requestAnimationFrame(restore), {once:true});
    }
  }

  // No autosave or storage: guards protect the current document, including Wails
  // links and POST navigation where beforeunload alone is insufficient.
  const editors = Array.from(document.querySelectorAll('form[action$="/answers/save"], form[action$="/cover-letter/save"]'));
  const recovery = document.querySelector('[data-answer-recovery], [data-letter-recovery]');
  const initial = new Map(editors.map(form => [form, new URLSearchParams(new FormData(form)).toString()]));
  const dirtyForms = () => editors.filter(form => form.dataset.dirty === 'true');
  let leaving = false;
  const update = form => {
    const dirty = new URLSearchParams(new FormData(form)).toString() !== initial.get(form);
    form.dataset.dirty = String(dirty);
    const status = form.querySelector('[data-answer-status]');
    if (status) {
      status.dataset.savedText ||= status.textContent;
      status.textContent = dirty ? 'Unsaved changes' : status.dataset.savedText;
    }
    const panel = form.closest('[data-question-panel]');
    const link = panel && workbench?.querySelector(`[data-question-link="${panel.dataset.questionKey}"]`);
    if (link) link.textContent = panel.dataset.questionLabel + (dirty ? ' · Unsaved changes' : '');
    const option = panel && workbench?.querySelector(`option[value="${panel.dataset.questionKey}"]`);
    if (option) option.textContent = option.textContent.replace(/ · Unsaved changes$/, '') + (dirty ? ' · Unsaved changes' : '');
  };
  for (const type of ['input', 'change']) document.addEventListener(type, event => {
    const form = event.target.closest('form');
    if (initial.has(form)) update(form);
  });
  const allowLeaving = ownForm => {
    const others = dirtyForms().filter(form => form !== ownForm);
    if (!others.length && !recovery) return true;
    if (window.confirm('Unsaved answer or letter changes will be lost. Leave and discard them? Choose Cancel to stay and save or copy your work first.')) return true;
    const first = others[0];
    if (first) {
      first.dispatchEvent(new CustomEvent('question:reveal', {bubbles:true}));
      first.closest('details')?.setAttribute('open', '');
      first.querySelector('textarea, select')?.focus();
    }
    return false;
  };
  document.addEventListener('click', event => {
    const link = event.target.closest('a[href]');
    if (!link || event.defaultPrevented || link.target === '_blank' || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    const target = new URL(link.href, location.href);
    // Hash/tab navigation keeps the same inputs alive in this document.
    if (target.origin === location.origin && target.pathname === location.pathname && target.search === location.search && target.hash && target.hash !== location.hash) return;
    if (link.getAttribute('href').startsWith('#')) return;
    if (!allowLeaving(null)) { event.preventDefault(); event.stopImmediatePropagation(); }
    else leaving = true;
  }, true);
  document.addEventListener('submit', event => {
    if (!allowLeaving(event.target)) { event.preventDefault(); event.stopImmediatePropagation(); }
    else leaving = true;
  }, true);
  window.addEventListener('beforeunload', event => {
    if (!leaving && (dirtyForms().length || recovery)) { event.preventDefault(); event.returnValue = ''; }
  });
  window.addEventListener('pageshow', () => { leaving = false; });
})();
