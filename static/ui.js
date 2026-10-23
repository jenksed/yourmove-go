/* Small progressive enhancements only; every mutating form remains server-owned. */
(() => {
  'use strict';
  const writeClipboardText = async (text) => {
    let clipboardError;
    if (navigator.clipboard?.writeText) {
      try { await navigator.clipboard.writeText(text); return; }
      catch (error) { clipboardError = error; }
    }
    if (document.execCommand) {
      const field = document.createElement('textarea');
      field.value = text;
      field.setAttribute('readonly', '');
      field.style.position = 'fixed';
      field.style.insetInlineStart = '-9999px';
      document.body.append(field);
      field.select();
      let copied = false;
      try { copied = document.execCommand('copy'); }
      finally { field.remove(); }
      if (copied) return;
    }
    throw clipboardError || new Error('Clipboard unavailable');
  };

  const copyDescription = document.querySelector('[data-copy-description]');
  if (copyDescription) {
    copyDescription.hidden = false;
    copyDescription.addEventListener('click', async () => {
      const status = document.querySelector('[data-copy-description-status]');
      const text = document.getElementById('job-description').innerText.trim();
      try {
        await writeClipboardText(text);
        status.textContent = 'Job description copied.';
      } catch (_) { status.textContent = 'Could not copy. Select the description and copy it manually.'; }
    });
  }

  const exportButtons = document.querySelectorAll('[data-copy-export]');
  const exportStatus = document.querySelector('[data-copy-export-status]');
  const exportFallback = document.querySelector('[data-copy-export-fallback]');
  const exportText = document.querySelector('[data-copy-export-text]');
  const exportScope = document.querySelector('[data-brief-scope]');
  const exportViewLink = document.querySelector('[data-brief-view]');
  const updateExportViewLink = () => {
    if (!exportViewLink) return;
    const endpoint = new URL(exportViewLink.dataset.briefViewUrl, window.location.origin);
    endpoint.searchParams.set('scope', exportScope?.value || 'unanswered');
    exportViewLink.href = endpoint.pathname + endpoint.search;
  };
  exportScope?.addEventListener('change', updateExportViewLink);
  updateExportViewLink();
  for (const button of exportButtons) {
    button.hidden = false;
    button.addEventListener('click', async () => {
      const format = button.dataset.exportFormat;
      const scope = document.querySelector('[data-brief-scope]')?.value || 'unanswered';
      const endpoint = new URL(button.dataset.exportUrl, window.location.origin);
      endpoint.searchParams.set('format', format);
      endpoint.searchParams.set('scope', scope);
      let value;
      try {
        const response = await fetch(endpoint, { credentials: 'same-origin', headers: { Accept: 'text/plain' } });
        if (!response.ok) throw new Error('Brief unavailable');
        value = await response.text();
      } catch (_) {
        if (exportStatus) exportStatus.textContent = 'Could not prepare the text. Use the plain-text brief below.';
        return;
      }
      try {
        await writeClipboardText(value);
        if (exportStatus) exportStatus.textContent = format === 'brief' ? 'Application brief copied.' : format === 'questions' ? 'Questions copied.' : 'Job details copied.';
        if (exportFallback) { exportFallback.hidden = true; exportFallback.open = false; }
        if (exportText) exportText.value = '';
      } catch (_) {
        if (exportStatus) exportStatus.textContent = 'Clipboard unavailable. Select and copy the text below.';
        if (exportText) {
          exportText.value = value;
          if (exportFallback) { exportFallback.hidden = false; exportFallback.open = true; }
          const options = exportFallback?.closest('.brief-export-options');
          if (options) options.open = true;
          exportText.focus();
          exportText.select();
        }
      }
    });
  }
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

  // Compact navigation is a nonmodal disclosure; desktop and no-JS retain links.
  const navigation = document.getElementById('workspace-navigation');
  const navigationToggle = document.querySelector('[data-navigation-toggle]');
  const navigationClose = document.querySelector('[data-navigation-close]');
  if (navigation && navigationToggle) {
    document.documentElement.classList.add('navigation-enabled');
    navigationToggle.hidden = false;
    navigationClose.hidden = false;
    const closeNavigation = (restore = true) => {
      document.documentElement.classList.remove('navigation-open');
      navigationToggle.setAttribute('aria-expanded', 'false');
      if (restore) navigationToggle.focus();
    };
    navigationToggle.addEventListener('click', () => {
      if (navigationToggle.getAttribute('aria-expanded') === 'true') closeNavigation();
      else {
        document.documentElement.classList.add('navigation-open');
        navigationToggle.setAttribute('aria-expanded', 'true');
        navigation.querySelector('a[aria-current="page"], a')?.focus();
      }
    });
    navigationClose?.addEventListener('click', () => closeNavigation());
    document.addEventListener('keydown', event => {
      if (event.key === 'Escape' && navigationToggle.getAttribute('aria-expanded') === 'true' && !document.querySelector('dialog[open]')) {
        event.preventDefault(); closeNavigation();
      }
    });
    window.matchMedia('(min-width: 820px)').addEventListener('change', () => closeNavigation(false));
  }

  const addDisclosure = document.getElementById('add-opportunity');
  document.querySelectorAll('[data-open-add]').forEach((link) => link.addEventListener('click', () => {
    if (addDisclosure) { addDisclosure.open = true; requestAnimationFrame(() => document.getElementById('job_url')?.focus()); }
  }));
  if (addDisclosure && location.hash === '#add-opportunity') addDisclosure.open = true;
  document.querySelectorAll('[data-open-details]').forEach((link) => link.addEventListener('click', (event) => {
    const target = document.getElementById(link.dataset.openDetails);
    if (target) {
      target.open = true;
      if (target.id === 'letter-editor') {
        event.preventDefault();
        try { history.replaceState(null, '', link.hash); } catch (_) {}
        requestAnimationFrame(() => {
          const heading = target.querySelector('#letter-studio-title');
          (heading || target).scrollIntoView({block:'start', behavior:'instant'});
          heading?.focus({preventScroll:true});
        });
      }
      if (target.id === 'record-submission') {
        document.querySelector('[data-tab-target=prepare]')?.click();
        requestAnimationFrame(() => target.scrollIntoView({block:'center'}));
      }
    }
  }));

  // Writing activities are native exclusive disclosures. This small bridge
  // restores the last activity in this tab and reveals existing deep links.
  const writingActivities = Array.from(document.querySelectorAll('[data-writing-activity]'));
  if (writingActivities.length) {
    const activityStorageKey = `yourmove:writing-activity:${location.pathname}`;
    const activityByKey = key => writingActivities.find(activity => activity.dataset.writingActivityKey === key);
    const activityForTarget = target => target?.closest?.('[data-writing-activity]') || null;
    const fragmentTarget = () => {
      if (!location.hash) return null;
      try { return document.getElementById(decodeURIComponent(location.hash.slice(1))); }
      catch (_) { return null; }
    };
    const routeActivity = () => {
      const query = new URLSearchParams(location.search);
      const coverResult = query.get('cover');
      if (coverResult === 'saved' || coverResult === 'transferring') return activityByKey('answers');
      if (query.has('early_question') || query.has('question')) return activityByKey('answers');
      if (coverResult || query.has('draft')) return activityByKey('letter');
      return activityForTarget(fragmentTarget());
    };
    const openActivity = (activity, target = activity) => {
      if (!activity) return;
      for (let node = target; node && node !== activity.parentElement; node = node.parentElement) {
        if (node.matches?.('details')) node.open = true;
      }
      activity.open = true;
      try { sessionStorage.setItem(activityStorageKey, activity.dataset.writingActivityKey); } catch (_) {}
    };
    const revealFragment = () => {
      const target = fragmentTarget();
      const activity = activityForTarget(target);
      if (!activity) return;
      const panel = activity.closest('[data-workspace-panel]');
      if (panel?.hidden) {
        document.querySelector(`.workspace-tab[data-tab-target="${panel.dataset.workspacePanel}"]`)?.click();
        try { history.replaceState(null, '', location.pathname + location.search + location.hash); } catch (_) {}
      }
      openActivity(activity, target);
    };
    const requested = routeActivity();
    let remembered = null;
    try { remembered = activityByKey(sessionStorage.getItem(activityStorageKey)); } catch (_) {}
    const selected = requested || remembered || writingActivities.find(activity => activity.open) || activityByKey('answers') || writingActivities[0];
    openActivity(selected);
    document.addEventListener('toggle', event => {
      const activity = event.target;
      if (activity?.matches?.('[data-writing-activity]') && activity.open) {
        try { sessionStorage.setItem(activityStorageKey, activity.dataset.writingActivityKey); } catch (_) {}
      }
    }, true);
    writingActivities.forEach(activity => {
      const summary = activity.querySelector(':scope > summary');
      summary?.addEventListener('click', event => {
        if (!event.isTrusted) return;
        requestAnimationFrame(() => {
          if (!activity.open) return;
          const hash = `#${activity.id}`;
          if (location.hash !== hash) {
            try { history.pushState(history.state, '', location.pathname + location.search + hash); } catch (_) {}
          }
          try { sessionStorage.setItem(activityStorageKey, activity.dataset.writingActivityKey); } catch (_) {}
        });
      });
    });
    window.addEventListener('hashchange', revealFragment);
    window.addEventListener('popstate', revealFragment);
    revealFragment();
  }

  // Client-side presentation filtering. Server output stays complete without JS.
  const list = document.querySelector('[data-job-list]');
  if (list) {
    const rows = Array.from(list.querySelectorAll('[data-job-row]'));
    const search = document.querySelector('[data-job-search]');
    const stage = document.querySelector('[data-job-stage]');
    const sort = document.querySelector('[data-job-sort]');
    const status = document.getElementById('results-status');
    const empty = document.getElementById('filtered-empty');
    const storageKey = 'yourmove:opportunities:filters:v2';
    try {
      const saved = JSON.parse(sessionStorage.getItem(storageKey) || '{}');
      if (search && typeof saved.query === 'string') search.value = saved.query;
      if (stage && ['active','all','review','ready','applied','closed'].includes(saved.stage)) stage.value = saved.stage;
      if (sort && ['priority','newest','company'].includes(saved.sort)) sort.value = saved.sort;
    } catch (_) {}
    const stateOrder = {ready: 0, review: 1, applied: 2, closed: 3};
    const priorityOrder = {high: 0, normal: 1, low: 2};
    const render = () => {
      const query = (search?.value || '').toLocaleLowerCase().trim();
      const currentStage = stage?.value || 'active';
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
        const stageMatches = currentStage === 'all' ||
          (currentStage === 'active' ? !['applied','closed'].includes(row.dataset.stage) : row.dataset.stage === currentStage);
        row.hidden = !stageMatches || !!(query && !row.dataset.search.toLocaleLowerCase().includes(query));
        // A filtered-out selection must not become an invisible destructive action.
        if (row.hidden) { const picked = row.querySelector('input[name="job_ids"]'); if (picked) picked.checked = false; }
        if (!row.hidden) visible++;
      });
      if (status) status.textContent = `${visible} shown · ${rows.length} saved in total`;
      if (empty) empty.hidden = visible !== 0;
      try { sessionStorage.setItem(storageKey, JSON.stringify({query: search?.value || '', stage: currentStage, sort: ordering})); } catch (_) {}
    };
    [search,stage,sort].forEach((el) => el?.addEventListener(el === search ? 'input' : 'change', render));
    document.querySelector('[data-remaining-count]')?.addEventListener('click', () => {
      if (stage) stage.value = 'active';
      if (search) search.value = '';
      render();
    });
    document.querySelector('[data-clear-filters]')?.addEventListener('click', () => {
      if (search) search.value = '';
      if (stage) stage.value = 'active';
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
    const defaultTab = document.querySelector('[data-workspace-tabs]')?.dataset.defaultTab || 'overview';
    const panelForHash = hash => {
      try { return document.getElementById(decodeURIComponent(hash.slice(1)))?.closest('[data-workspace-panel]')?.dataset.workspacePanel; }
      catch (_) { return null; }
    };
    const current = () => {
      const query = new URLSearchParams(location.search);
      if (['saved','transferring'].includes(query.get('cover'))) return 'prepare';
      if (query.has('question') || query.has('early_question')) return 'answers';
      if (query.has('cover') || query.has('draft')) return panelForHash(location.hash) === 'prepare' ? 'cover-letter' : (panelForHash(location.hash) || 'cover-letter');
      return panelForHash(location.hash) || defaultTab;
    };
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
      if (target === 'answers') document.getElementById('application-answers-workspace').open = true;
      if (target === 'cover-letter') document.getElementById('letter-editor').open = true;
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
    window.addEventListener('hashchange', () => show(panelForHash(location.hash) || current()));
    document.addEventListener('click', event => {
      const link = event.target.closest('a[href^="#"]');
      if (!link) return;
      const target = panelForHash(link.hash);
      if (target) show(target);
    });
    document.querySelectorAll('[data-tab-target]:not(.workspace-tab)').forEach((link) => link.addEventListener('click', () => {
      const target = link.dataset.tabTarget;
      if (names.has(target)) show(target);
    }));
  }

  document.querySelectorAll('form[method="POST"]').forEach((form) => {
    form.addEventListener('submit', () => {
      document.documentElement.dataset.pendingAction = 'yes';
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
    const update = () => { const words = draft.value.trim() ? draft.value.trim().split(/\s+/).length : 0; const feedback = words < 120 ? "below target" : words <= 160 ? "within target" : words <= 180 ? "above target" : "over editorial ceiling"; count.textContent = `${words} words · ${feedback} · target 120–160; ceiling 180 · edit freely`; };
    update(); draft.addEventListener('input', update);
  }
})();

// Question workbench: visibility and navigation over the authoritative Go forms.
(() => {
  const workbenches = [...document.querySelectorAll('[data-question-workbench]')];
  workbenches.forEach(workbench => {
  if (workbench.querySelector('[data-question-panel]')) {
    const questionParam = workbench.dataset.questionParam || 'question';
    const returning = new URLSearchParams(location.search).has(questionParam);
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
      panels[selected].dispatchEvent(new CustomEvent('question:selected'));
      links.forEach((link, panel) => {
        if (panel === panels[selected]) link.setAttribute('aria-current', 'true');
        else link.removeAttribute('aria-current');
      });
      chooser.value = panels[selected].dataset.questionKey;
      previous.disabled = selected === 0;
      next.disabled = selected === panels.length - 1;
      position.textContent = `Question ${selected + 1} of ${panels.length}`;
      if (focus) {
        panels[selected].focus({preventScroll:true});
        const bounds = panels[selected].getBoundingClientRect();
        if (bounds.top < 0 || bounds.top >= innerHeight) panels[selected].scrollIntoView({block:'start', behavior:'instant'});
      }
    };
    const statusGroups = questionParam === 'early_question' ? [
      {label: 'Needs your answer', matches: status => status === 'No early answer saved'},
      {label: 'Prepared locally', matches: status => status.startsWith('Early answer saved locally')}
    ] : [
      {label: 'Needs your answer', matches: status => status === 'Needs your answer'},
      {label: 'Prepared locally', matches: status => status.startsWith('Answer saved locally.') || status.startsWith('Answer available locally')}
    ];
    for (const {label: status, matches} of statusGroups) {
      const members = panels.filter(panel => matches(panel.dataset.questionStatus || ''));
      if (!members.length) continue;
      const group = document.createElement('div');
      const heading = document.createElement('h4');
      heading.textContent = status;
      group.append(heading);
      const options = document.createElement('optgroup');
      options.label = status;
      members.forEach(panel => {
        const link = document.createElement('a');
        link.href = `#${panel.id}`;
        link.textContent = panel.dataset.questionLabel;
        link.title = panel.dataset.questionLabel;
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
    chooser.addEventListener('change', () => show(panels.findIndex(panel => panel.dataset.questionKey === chooser.value)));
    previous.addEventListener('click', () => show(selected - 1));
    next.addEventListener('click', () => show(selected + 1));
    workbench.addEventListener('question:reveal', event => show(panels.indexOf(event.target.closest('[data-question-panel]'))));
    workbench.querySelector('[data-review-ready]')?.addEventListener('click', event => {
      event.preventDefault();
      document.querySelector('.workspace-tab[data-tab-target=prepare]')?.click();
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
  });

  // Keep the selected answer editor in place and bring that workbench into view.
  document.querySelectorAll('[data-prepare-question-entry]').forEach(link => {
    link.addEventListener('click', event => {
      const target = document.getElementById(link.hash.slice(1));
      if (!target) return;
      event.preventDefault();
      for (let parent = target; parent; parent = parent.parentElement) {
        if (parent.matches?.('details')) parent.open = true;
      }
      try { history.replaceState(null, '', link.hash); } catch (_) {}
      requestAnimationFrame(() => {
        const selected = target.querySelector('[data-question-panel]:not([hidden])');
        const destination = selected || target;
        destination.focus({preventScroll:true});
        destination.scrollIntoView({block:'start', behavior:'instant'});
      });
    });
  });

  // Explicit server saves remain authoritative. Independent material actions
  // carry unsaved edits across their same-job redirect in this tab only.
  const editors = Array.from(document.querySelectorAll('form[action$="/answers/save"], form[action$="/answers/early-save"], form[action$="/cover-letter/save"]'));
  const recovery = () => document.querySelector('[data-answer-recovery], [data-letter-recovery], [data-answer-ai-unprotected="true"]');
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
    const workbench = panel?.closest('[data-question-workbench]');
    const link = workbench?.querySelector(`[data-question-link="${panel.dataset.questionKey}"]`);
    if (link) link.textContent = panel.dataset.questionLabel + (dirty ? ' · Unsaved changes' : '');
    const option = workbench?.querySelector(`option[value="${panel.dataset.questionKey}"]`);
    if (option) option.textContent = option.textContent.replace(/ · Unsaved changes$/, '') + (dirty ? ' · Unsaved changes' : '');
    const excluded = document.querySelector('[data-transfer-unsaved]');
    if (excluded) {
      const labels = dirtyForms().map(editor => editor.closest('[data-question-panel]')?.dataset.questionLabel || 'Cover letter');
      excluded.hidden = labels.length === 0;
      excluded.textContent = labels.length ? 'Unsaved edits not transferred: ' + labels.join('; ') + '. Only the saved scope shown above is authorized.' : '';
    }
  };
  // Related writing stays read-only until a user opens it. Inserting an
  // example changes only the current textarea; the existing save form owns
  // persistence and revision handling.
  document.addEventListener('toggle', async event => {
      const details = event.target.closest?.('[data-related-answer]');
      if (!details) return;
      if (!details.open || details.dataset.loaded === 'true') return;
      const entryID = details.dataset.entryId;
      const content = details.querySelector('[data-related-content]');
      const useButton = details.querySelector('[data-use-library-answer]');
      if (!entryID || !content) return;
      try {
        const response = await fetch(`/answer-library/${encodeURIComponent(entryID)}/content`, {headers: {'Accept': 'application/json'}});
        if (!response.ok) throw new Error('Unable to load this previous example. Open the full library entry to review it.');
        const example = await response.json();
        if (example.revision !== Number(details.dataset.libraryRevision)) {
          content.textContent = 'This example changed after the recommendation was shown. Reload the question before using it.';
          details.dataset.stale = 'true';
          return;
        }
        const answer = document.createElement('pre');
        answer.className = 'evidence-text related-writing-full-answer';
        answer.setAttribute('aria-label', 'Complete previous answer');
        answer.textContent = typeof example.answer === 'string' ? example.answer : '';
        content.replaceChildren(answer);
        details.dataset.loaded = 'true';
        if (useButton) useButton.hidden = false;
      } catch (_) {
        content.textContent = 'Unable to load this previous example. Open the full library entry to review and copy it.';
      }
  }, true);
  document.addEventListener('click', async event => {
      const button = event.target.closest('[data-use-library-answer]');
      if (!button) return;
      const panel = button.closest('[data-question-panel]');
      const form = panel?.querySelector('form[action$="/answers/save"], form[action$="/answers/early-save"]');
      const key = form?.querySelector('[name="question_key"]')?.value;
      const revision = form?.querySelector('[name="revision"]')?.value;
      const editor = form?.querySelector('[name="answer"]');
      const status = button.closest('[data-related-answer]')?.querySelector('[data-related-status]');
      const entryID = button.dataset.entryId;
      const libraryRevision = button.dataset.libraryRevision;
      if (!panel || !form || !key || revision === undefined || !editor || !entryID || !libraryRevision || !status) return;
      const beforeValue = editor.value;
      const beforeKey = key;
      const beforeRevision = revision;
      if (panel.hidden) return;
      if ((form.dataset.dirty === 'true' || beforeValue.trim() !== '') &&
          !window.confirm('Replace the current editor text with this previous writing? It will remain unsaved until you save this answer locally.')) return;
      button.disabled = true;
      status.textContent = 'Checking the current question…';
      try {
        const body = new URLSearchParams({library_entry_id: entryID, library_revision: libraryRevision, question_key: beforeKey, revision: beforeRevision, destination: form.querySelector('[name="destination"]')?.value || ''});
        if (form.action.endsWith('/answers/early-save')) {
          body.set('early','yes');
          for (const name of ['discovery_source','discovery_fingerprint','source_question_id']) body.set(name,form.querySelector(`[name="${name}"]`)?.value || '');
        }
        const response = await fetch(button.dataset.startpointUrl, {
          method: 'POST', headers: {'Content-Type': 'application/x-www-form-urlencoded;charset=UTF-8', 'Accept': 'application/json'}, body
        });
        if (!response.ok) throw new Error(await response.text() || 'The current question changed. Review it before using previous writing.');
        const selected = await response.json();
        if (selected.entry_id !== Number(entryID) || selected.library_revision !== Number(libraryRevision) || selected.question_key !== beforeKey ||
            String(selected.revision) !== beforeRevision || form.querySelector('[name="question_key"]')?.value !== beforeKey ||
            form.querySelector('[name="revision"]')?.value !== beforeRevision || editor.value !== beforeValue || panel.hidden) {
          throw new Error('The current editor changed while the example was loading. Review the question and try again.');
        }
        editor.value = typeof selected.answer === 'string' ? selected.answer : '';
        editor.dispatchEvent(new Event('input', {bubbles: true}));
        status.textContent = 'Previous writing copied into this unsaved answer. Edit it for this question, then save locally.';
      } catch (error) {
        status.textContent = error instanceof Error ? error.message : 'Unable to use this previous example. Review the current question and try again.';
      } finally {
        button.disabled = false;
      }
  });
  for (const type of ['input', 'change']) document.addEventListener(type, event => {
    const form = event.target.closest('form');
    if (initial.has(form)) update(form);
  });
  const editKey = 'yourmove:pending-edits:' + location.pathname;
  const identity = form => form.getAttribute('action') + ':' + (form.querySelector('[name=question_key], [name=draft_id]')?.value || '');
  const carryEdits = ownForm => {
    const others = dirtyForms().filter(form => form !== ownForm);
    if (recovery()) return false;
    try {
      const pending = others.map(form => ({identity: identity(form), baseline: initial.get(form), fields: [...new FormData(form)]}));
      if (pending.length) sessionStorage.setItem(editKey, JSON.stringify(pending));
      else sessionStorage.removeItem(editKey);
      return true;
    } catch (_) { return false; }
  };
  // Restore only against the same saved revision and exact original contents.
  // If another save changed them, expose the text for recovery, never transfer it.
  try {
    const pending = JSON.parse(sessionStorage.getItem(editKey) || '[]');
    sessionStorage.removeItem(editKey);
    for (const saved of pending) {
      const form = editors.find(form => identity(form) === saved.identity);
      if (form && initial.get(form) === saved.baseline) {
        for (const [name, value] of saved.fields) {
          const control = form.elements.namedItem(name);
          if (!control || !['answer','body','reusable'].includes(name)) continue;
          if (control.type === 'checkbox') control.checked = true;
          else control.value = value;
          control.dispatchEvent(new Event(control.type === 'checkbox' ? 'change' : 'input', {bubbles:true}));
        }
        update(form);
      } else {
        const notice = document.createElement('section');
        notice.dataset.answerRecovery = '';
        const heading = document.createElement('p');
        heading.textContent = 'Unsaved edits preserved below. The saved source changed; copy your text into the current editor before saving. This text is excluded from transfers.';
        const text = document.createElement('textarea');
        text.readOnly = true;
        text.setAttribute('aria-label', 'Preserved unsaved edits');
        text.value = saved.fields.filter(([name]) => ['answer','body'].includes(name)).map(([,value]) => value).join('\n');
        notice.append(heading,text);
        document.getElementById('application-action')?.append(notice);
      }
    }
  } catch (_) {}
  // Own this decision in the document: WebView native confirms can be
  // unavailable. Keep one immutable destination until Stay or Leave resolves it.
  let pendingDeparture = null;
  const dialog = document.createElement('dialog');
  dialog.className = 'leave-materials-dialog';
  dialog.setAttribute('aria-labelledby', 'leave-materials-title');
  dialog.setAttribute('aria-describedby', 'leave-materials-description');
  dialog.innerHTML = '<h2 id="leave-materials-title">Leave application materials?</h2><p id="leave-materials-description">You have unsaved changes. Anything already saved locally will remain available, but unsaved changes will be lost if you leave.</p><div class="leave-materials-actions"><button type="button" class="button button-secondary" data-stay>Stay and edit</button><button type="button" class="button button-primary" data-leave>Leave without saving</button></div>';
  document.body.append(dialog);
  const stay = dialog.querySelector('[data-stay]');
  const leave = dialog.querySelector('[data-leave]');
  const cancelDeparture = () => {
    const pending = pendingDeparture;
    pendingDeparture = null;
    dialog.close();
    pending?.focus?.focus({preventScroll:true});
  };
  stay.addEventListener('click', cancelDeparture);
  dialog.addEventListener('cancel', event => { event.preventDefault(); cancelDeparture(); });
  dialog.addEventListener('keydown', event => {
    if (event.key !== 'Tab') return;
    // WebKit can skip buttons when macOS full keyboard access is disabled.
    // Own both directions instead of relying on platform tab preferences.
    event.preventDefault();
    (document.activeElement === stay ? leave : stay).focus();
  });
  leave.addEventListener('click', () => {
    const pending = pendingDeparture;
    if (!pending) return;
    pendingDeparture = null;
    // Suppress beforeunload only for this acknowledged transition. Do not clear
    // saved data or unrelated AI recovery entries from other workspaces.
    leaving = true;
    for (const form of pending.discard) { form.reset(); update(form); }
    try {
      sessionStorage.removeItem(editKey);
      const currentPreviews = [];
      for (let i = 0; i < sessionStorage.length; i++) {
        const key = sessionStorage.key(i);
        if (key?.startsWith(editKey + ':ai:')) currentPreviews.push(key);
      }
      currentPreviews.forEach(key => sessionStorage.removeItem(key));
    } catch (_) {}
    dialog.close();
    pending.run();
  });
  const allowLeaving = (ownForm, run) => {
    const discard = dirtyForms().filter(form => form !== ownForm);
    if (!discard.length && !recovery()) return true;
    if (!pendingDeparture) {
      pendingDeparture = {run, focus: document.activeElement, discard};
      dialog.showModal();
      stay.focus();
    }
    return false;
  };
  const completionForm = document.querySelector('form[data-record-completion]');
  const completionNotes = completionForm?.querySelector('[name="notes"]');
  const completionKey = completionForm && `yourmove:submission-note:${completionForm.dataset.jobId}`;
  if (completionForm && completionKey) {
    try {
      if (document.querySelector('#history .submission-success')) {
        sessionStorage.removeItem(completionKey);
      } else {
        const savedNotes = sessionStorage.getItem(completionKey);
        if (savedNotes !== null && completionNotes && !completionNotes.value) {
          completionNotes.value = savedNotes;
          const restored = completionForm.querySelector('[data-submission-note-restored]');
          if (restored) restored.hidden = false;
        }
      }
    } catch (_) {}
  }
  document.addEventListener('click', event => {
    const link = event.target.closest('a[href]');
    if (!link || event.defaultPrevented || link.target === '_blank' || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    const target = new URL(link.href, location.href);
    // Hash/tab navigation keeps the same inputs alive in this document.
    if (target.origin === location.origin && target.pathname === location.pathname && target.search === location.search && target.hash && target.hash !== location.hash) return;
    if (link.getAttribute('href').startsWith('#')) return;
    if (link.closest('[data-new-questions]') && target.origin === location.origin && target.pathname === location.pathname && carryEdits(null)) { event.preventDefault(); leaving = true; location.reload(); return; }
    // External operations and downloads do not replace this editor document.
    if (target.origin !== location.origin || link.hasAttribute('download') || /\/download$/.test(target.pathname)) return;
    const destination = target.href;
    if (!allowLeaving(null, () => location.assign(destination))) { event.preventDefault(); event.stopImmediatePropagation(); }
    else leaving = true;
  }, true);
  let authorizedSubmit = null;
  document.addEventListener('submit', event => {
    if (authorizedSubmit === event.target) { authorizedSubmit = null; leaving = true; return; }
    // Recording is independent of question completion. Keep Cancel at this
    // decision instead of revealing/focusing an unrelated question editor.
    const recording = event.target.matches('form[data-record-completion]');
    const independent = event.target.matches('form[data-ai-generate], form[data-ai-draft], form[action$="/continue"], form[action$="/cover-letter/transfer"], form[action$="/browser/observe"], form[action$="/browser/start"], form[action$="/open"]');
    const allowed = independent && carryEdits(event.target) ? true : recording
      ? (!(dirtyForms().length || recovery()) || window.confirm('You have unsaved answer or letter edits. Record completion anyway? Choose OK to record your confirmed employer submission and leave these unsaved edits behind; they will be lost. Choose Cancel to stay and save or copy them first.'))
      : allowLeaving(event.target, () => {
        authorizedSubmit = event.target;
        event.target.requestSubmit(event.submitter || undefined);
        // Invalid forms stay on the page and must retain unload protection.
        if (authorizedSubmit) { authorizedSubmit = null; leaving = false; }
      });
    if (!allowed) { event.preventDefault(); event.stopImmediatePropagation(); }
    else {
      if (recording) {
        try {
          const note = event.target.querySelector('[name="notes"]')?.value || '';
          if (note) sessionStorage.setItem(`yourmove:submission-note:${event.target.dataset.jobId}`, note);
          else sessionStorage.removeItem(`yourmove:submission-note:${event.target.dataset.jobId}`);
        } catch (_) {}
      }
      leaving = true;
    }
  }, true);
  window.addEventListener('beforeunload', event => {
    if (!leaving && (dirtyForms().length || recovery())) { event.preventDefault(); event.returnValue = ''; }
  });
  window.addEventListener('pageshow', () => { leaving = false; delete document.documentElement.dataset.pendingAction; });
})();

// A letter result belongs at the writing desk; saved edits return to transfer scope.
(() => {
  const result = new URLSearchParams(location.search).get('cover');
  if (!result) return;
  if (['saved','transferring'].includes(result)) {
    const scope = document.getElementById('application-action');
    if (!scope) return;
    const restoreScope = () => {
      scope.focus({preventScroll:true});
      scope.scrollIntoView({block:'start',behavior:'instant'});
    };
    if (document.readyState === 'complete') restoreScope();
    else window.addEventListener('load', () => requestAnimationFrame(restoreScope), {once:true});
    return;
  }
  const editor = document.getElementById('letter-editor');
  if (!editor) return;
  const show = () => {
    editor.open = true;
    document.querySelector('[data-tab-target=cover-letter]')?.click();
    editor.scrollIntoView({block:'start',behavior:'instant'});
  };
  if (document.readyState === 'complete') show();
  else window.addEventListener('load', () => requestAnimationFrame(show), {once:true});
})();

// Narrative drafting owns only review tickets and read-only previews. The
// existing answer form remains the sole local save operation.
(() => {
  'use strict';
  const prefix = 'yourmove:pending-edits:' + location.pathname + ':ai:';
  const api = async (root, action, fields, signal) => {
    const response = await fetch(`/jobs/${encodeURIComponent(root.dataset.job)}/answers/ai/${action}`, {
      method: 'POST', headers: {'Content-Type': 'application/x-www-form-urlencoded;charset=UTF-8', Accept: 'application/json'},
      body: fields instanceof URLSearchParams ? fields : new URLSearchParams(fields), signal
    });
    const value = await response.json();
    if (!response.ok) throw new Error(value.message || 'This drafting request could not complete. Review sources again.');
    return value;
  };
  const sourceReference = source => {
    let name = '';
    try { const content = JSON.parse(source.content); name = content.Title || [content.Employer, content.Job].filter(Boolean).join(' · '); } catch (_) {}
    return `${source.category}${name ? ` · ${name.slice(0, 200)}` : ''}${source.id ? ` · ID ${source.id} · revision ${source.revision}` : ''}`;
  };
  const roots = [...document.querySelectorAll('[data-answer-ai]')];
  const initialize = root => {
    if (root.dataset.initialized === 'true') return;
    root.dataset.initialized = 'true';
    const find = name => root.querySelector(`[data-answer-ai-${name}]`);
    const panel = root.closest('[data-question-panel]');
    const form = panel?.querySelector('form[action$="/answers/save"], form[action$="/answers/early-save"]');
    const editor = form?.querySelector('textarea[name="answer"]');
    if (!form || !editor) return;
    find('controls').hidden = false;
    const providerField = root.querySelector('[name="provider"]');
    const modelField = root.querySelector('[name="model"]');
    const providerHelp = find('provider-help');
    if (!providerField || !modelField || !providerHelp) return;
    const modelPreferencesKey = 'yourmove.answer.models.v1';
    const providerPreferenceKey = 'yourmove.answer.provider.v1';
    let modelPreferences = {};
    try { modelPreferences = JSON.parse(localStorage.getItem(modelPreferencesKey) || '{}'); } catch (_) {}
    const availableProvider = value => [...providerField.options].some(option => option.value === value);
    const updateProviderModel = () => {
      const option = providerField.options[providerField.selectedIndex];
      providerHelp.textContent = option?.dataset.help || 'Review AI connections for provider setup and supported models.';
      modelField.value = modelPreferences[providerField.value] || option?.dataset.defaultModel || '';
    };
    try {
      const preferredProvider = localStorage.getItem(providerPreferenceKey);
      if (preferredProvider && availableProvider(preferredProvider)) providerField.value = preferredProvider;
    } catch (_) {}
    updateProviderModel();
    providerField.addEventListener('change', () => {
      try { localStorage.setItem(providerPreferenceKey, providerField.value); } catch (_) {}
      updateProviderModel();
    });
    modelField.addEventListener('input', () => {
      modelPreferences[providerField.value] = modelField.value;
      try { localStorage.setItem(modelPreferencesKey, JSON.stringify(modelPreferences)); } catch (_) {}
    });
    let reviewed = null, preview = null, busy = false, request = null, selectionVersion = 0;
    const key = prefix + root.dataset.key;
    const status = message => { find('status').textContent = message; };
    const matches = () => form.isConnected && root.isConnected && form.querySelector('[name=question_key]')?.value === root.dataset.key && form.querySelector('[name=revision]')?.value === root.dataset.revision;
    const persist = () => {
      try {
        if (preview) {
          const serialized = JSON.stringify(preview);
          if (serialized.length > 22000) throw new Error('Preview recovery limit');
          const existing = [];
          for (let i = 0; i < sessionStorage.length; i++) {
            const candidate = sessionStorage.key(i);
            if (candidate !== key && candidate?.startsWith('yourmove:pending-edits:') && candidate.includes(':ai:')) existing.push(candidate);
          }
          existing.sort((a,b) => {
            const expiry = item => { try { return Date.parse(JSON.parse(sessionStorage.getItem(item)).expires_at) || 0; } catch (_) { return 0; } };
            return expiry(a) - expiry(b);
          });
          while (existing.length >= 16) sessionStorage.removeItem(existing.shift());
          sessionStorage.setItem(key, serialized);
        }
        else sessionStorage.removeItem(key);
        delete root.dataset.answerAiUnprotected;
      } catch (_) {
        if (preview) root.dataset.answerAiUnprotected = 'true';
        status('Preview recovery storage is unavailable. Keep this page open or copy the preview before leaving.');
      }
    };
    const showPreview = (value, recovering = false) => {
      preview = value;
      find('preview').hidden = false;
      find('text').value = value.answer;
      find('question').textContent = value.question;
      find('reference').textContent = value.references || 'Recovered preview; server review required before use.';
      find('adopt').disabled = !matches() || String(value.revision) !== root.dataset.revision || value.question_key !== root.dataset.key || Date.parse(value.expires_at) <= Date.now() || value.adoptable === false;
      find('another').open = false;
      root.open = true;
      status(recovering ? 'Recovered read-only preview. Use requires a fresh server check. Recovery is limited to this tab and is not durable storage.' : value.notice || 'Draft ready for your review. It has not changed the answer editor.');
      if (!recovering) find('heading').focus({preventScroll: true});
      persist();
    };
    const setBusy = value => {
      busy = value;
      find('selection').disabled = value;
      find('review').disabled = value;
      find('generate').disabled = value;
      find('cancel').hidden = !value;
      root.setAttribute('aria-busy', String(value));
    };
    const invalidateReview = event => {
      if (busy) return;
      if (event?.target?.type === 'checkbox' && ['application_resume','master_resume'].includes(event.target.name) && !event.target.checked) {
        find('excerpts').querySelector(`[data-excerpt-role="${event.target.name}"]`)?.remove();
      }
      selectionVersion++;
      const old = reviewed;
      reviewed = null;
      find('review-panel').hidden = true;
      find('authorize').checked = false;
      find('external').checked = false;
      if (old) api(root, 'cancel', {ticket: old.ticket}).catch(() => {});
    };
    find('selection').addEventListener('input', invalidateReview);
    find('selection').addEventListener('change', invalidateReview);
    find('review').addEventListener('click', async () => {
      if (busy) return;
      invalidateReview();
      if (!modelField.value.trim()) {
        status('Enter a model available to the selected provider. See AI connections for setup.');
        modelField.focus();
        return;
      }
      const fields = new URLSearchParams({question_key: root.dataset.key, revision: root.dataset.revision, destination: root.dataset.destination});
      if (root.dataset.early === 'yes') {
        fields.set('early', 'yes');
        fields.set('discovery_source', root.dataset.discoverySource || '');
        fields.set('discovery_fingerprint', root.dataset.discoveryFingerprint || '');
        fields.set('source_question_id', root.dataset.sourceQuestionId || '');
      }
      find('selection').querySelectorAll('[name]').forEach(input => {
        if (input.type === 'checkbox' && !input.checked) return;
        if (input.value) fields.append(input.name, input.value);
      });
      const version = selectionVersion;
      find('review').disabled = true;
      status('Reviewing selected sources locally…');
      try {
        const value = await api(root, 'review', fields);
        if (version !== selectionVersion) { api(root, 'cancel', {ticket: value.ticket}).catch(() => {}); throw new Error('Selections changed during review. Review the current choices again.'); }
        reviewed = value;
        value.sources.filter(source => source.category.startsWith('application_resume:') || source.category.startsWith('master_resume:')).forEach(source => {
          const role = source.category.split(':')[0];
          if (find('excerpts').querySelector(`[data-excerpt-role="${role}"]`)) return;
          const text = source.content.replace(/\n\[Excerpt truncated\]$/, '');
          const characters = [...text];
          const field = document.createElement('div'); field.className = 'field'; field.dataset.excerptRole = role;
          const label = document.createElement('label');
          const input = document.createElement('input'); input.type = 'number'; input.min = '1'; input.max = String(characters.length); input.value = String(characters.length);
          input.id = `ai-excerpt-${root.dataset.key}-${role}`; label.htmlFor = input.id;
          label.textContent = `Shorten ${role === 'application_resume' ? 'application' : 'master'} resume excerpt to first characters (optional)`;
          const boundary = document.createElement('input'); boundary.type = 'hidden'; boundary.name = role + '_end';
          input.addEventListener('input', () => { const count = Number(input.value); boundary.value = Number.isInteger(count) && count > 0 && count <= characters.length ? String(new TextEncoder().encode(characters.slice(0, count).join('')).length) : ''; });
          field.append(label, input, boundary); find('excerpts').append(field);
        });
        find('payload').textContent = `${value.prompt}\n\n${value.instructions}`;
        const questionStatus = value.preparation_kind === 'early' ? 'Published question; not yet checked against the live employer form. ' : 'Current browser-observed question. ';
        find('execution').textContent = `${questionStatus}${value.provider} · ${value.model}. ${value.execution}`;
        find('sources').textContent = value.sources.map(sourceReference).join('\n');
        find('review-panel').hidden = false;
        status('Local review complete. No model has been called. Inspect the exact text before authorizing.');
      } catch (error) { status(error.message); }
      finally { find('review').disabled = false; }
    });
    find('generate').addEventListener('click', async () => {
      if (busy || !reviewed) return;
      if (!find('authorize').checked || !find('external').checked) { status('Authorize this generation and acknowledge possible external transmission first.'); return; }
      const authorized = reviewed;
      request = new AbortController();
      setBusy(true);
      status('Generating one proposed answer. Your editor remains available.');
      try {
        const value = await api(root, 'generate', {ticket: authorized.ticket, authorize_generation: 'yes', authorize_external: 'yes'}, request.signal);
        if (value.state === 'complete' && typeof value.answer === 'string') {
          find('preview-payload').textContent = authorized.source_payload;
          find('preview-sources').hidden = false;
          showPreview({...value, references: `${authorized.provider} · ${authorized.model}. ` + authorized.sources.map(sourceReference).join('; ')});
        } else status(value.notice || `Generation state: ${value.state}. Review sources again for a new request.`);
      } catch (error) { status(error.name === 'AbortError' ? 'Generation canceled. Your existing text is unchanged.' : error.message); }
      finally {
        setBusy(false); request = null; reviewed = null;
        find('review-panel').hidden = true;
        find('authorize').checked = false; find('external').checked = false;
      }
    });
    find('cancel').addEventListener('click', async () => {
      if (!reviewed) return;
      const ticket = reviewed.ticket;
      try { await api(root, 'cancel', {ticket}); } catch (_) {}
      request?.abort();
    });
    find('adopt').addEventListener('click', async () => {
      if (!preview || find('adopt').disabled) return;
      const value = preview;
      const before = editor.value;
      const current = () => matches() && editor.value === before && preview === value && !panel.hidden;
      find('adopt').disabled = true;
      try {
        const checked = await api(root, 'adopt-check', {ticket: value.ticket, question_key: root.dataset.key, revision: root.dataset.revision});
        if (!checked.adoptable || checked.ticket !== value.ticket || checked.question_key !== root.dataset.key || String(checked.revision) !== root.dataset.revision || typeof checked.answer !== 'string' || checked.answer !== value.answer || !current()) throw new Error('The editor or review changed during the check. Your text was preserved.');
        if (before !== '' && !window.confirm('Replace the current editor text with this generated draft? It will remain unsaved until you choose Save answer locally.')) return;
        if (!current()) throw new Error('The editor changed during confirmation. Your text was preserved.');
        editor.value = checked.answer;
        editor.dispatchEvent(new Event('input', {bubbles: true}));
        editor.focus();
        status(root.dataset.early === 'yes' ? 'Added to your unsaved early answer. Review and edit it, then choose Save early answer locally.' : 'Added to your unsaved answer. Review and edit it, then choose Save answer locally.');
      } catch (error) {
        preview.adoptable = false; persist(); status(error.message + ' You can still copy this preview.');
      } finally { find('adopt').disabled = preview?.adoptable === false; }
    });
    find('copy').addEventListener('click', async () => {
      try {
        if (navigator.clipboard?.writeText) await navigator.clipboard.writeText(find('text').value);
        else { find('text').select(); if (!document.execCommand('copy')) throw new Error(); }
        status('Preview copied.');
      } catch (_) { find('text').focus(); find('text').select(); status('Select and copy the preview text.'); }
    });
    find('discard').addEventListener('click', () => {
      if (preview?.ticket) api(root, 'cancel', {ticket: preview.ticket}).catch(() => {});
      preview = null; find('text').value = ''; find('preview-payload').textContent = ''; find('preview-sources').hidden = true; find('preview').hidden = true; find('another').open = true; persist(); status('Preview discarded. Your editor is unchanged.');
    });
    try {
      const stored = sessionStorage.getItem(key);
      if (stored && stored.length <= 22000) {
        const value = JSON.parse(stored);
        if (value.question_key === root.dataset.key && typeof value.answer === 'string' && new TextEncoder().encode(value.answer).length <= 6000 && typeof value.question === 'string') showPreview(value, true);
      }
    } catch (_) {}
    // Best effort cancellation, without invoking generation or storing sources.
    window.addEventListener('pagehide', () => {
      if (busy && reviewed) fetch(`/jobs/${encodeURIComponent(root.dataset.job)}/answers/ai/cancel`, {method: 'POST', headers: {'Content-Type': 'application/x-www-form-urlencoded'}, body: new URLSearchParams({ticket: reviewed.ticket}), keepalive: true}).catch(() => {});
    });
  };
  roots.forEach(initialize);
  window.initializeAnswerAI = initialize;
  // If an observed question disappeared, retain copyable text without adoption.
  try {
    for (let i = 0, count = 0; i < sessionStorage.length && count < 16; i++) {
      const key = sessionStorage.key(i);
      if (!key?.startsWith(prefix) || roots.some(root => key === prefix + root.dataset.key)) continue;
      const stored = sessionStorage.getItem(key);
      if (!stored || stored.length > 22000) continue;
      const value = JSON.parse(stored);
      if (typeof value.answer !== 'string' || new TextEncoder().encode(value.answer).length > 6000) continue;
      const section = document.createElement('section');
      const notice = document.createElement('p'); notice.textContent = 'Recovered preview for an unavailable question. Copy this text for reference; adoption is unavailable.';
      const text = document.createElement('textarea'); text.readOnly = true; text.value = value.answer; text.setAttribute('aria-label', 'Recovered generated preview');
      const discard = document.createElement('button'); discard.type = 'button'; discard.textContent = 'Discard recovered preview'; discard.className = 'button button-secondary';
      discard.addEventListener('click', () => { sessionStorage.removeItem(key); section.remove(); });
      section.append(notice, text, discard); document.querySelector('.application-questions, .early-application-questions')?.append(section); count++;
    }
  } catch (_) {}
})();

// Load the combined, question-bound writing assistance only after it is opened.
(() => {
  document.querySelectorAll('[data-question-help]').forEach(help => {
    const panel = help.closest('[data-question-panel]');
    const restorePreviewDisclosure = () => {
      if (!panel || panel.hidden) return;
      try {
        const previewKey = `yourmove:pending-edits:${location.pathname}:ai:${help.dataset.key}`;
        if (sessionStorage.getItem(previewKey)) help.open = true;
      } catch (_) {}
    };
    panel?.addEventListener('question:selected', restorePreviewDisclosure);
    restorePreviewDisclosure();
    help.addEventListener('toggle', async () => {
      if (!help.open || help.dataset.loaded === 'true' || help.dataset.loading === 'true') return;
      const panel = help.closest('[data-question-panel]');
      const content = help.querySelector('[data-question-help-content]');
      const form = panel?.querySelector('form[action$="/answers/save"],form[action$="/answers/early-save"]');
      if (!panel || !form || panel.hidden) return;
      help.dataset.loading = 'true';
      const prior = content.innerHTML;
      content.replaceChildren(Object.assign(document.createElement('p'), {textContent:'Loading relevant writing and experience…', role:'status'}));
      try {
        const fields = new URLSearchParams({question_key:help.dataset.key, revision:help.dataset.revision, destination:help.dataset.destination});
        if (help.dataset.early === 'yes') {
          fields.set('early','yes');
          fields.set('discovery_source',help.dataset.discoverySource || '');
          fields.set('discovery_fingerprint',help.dataset.discoveryFingerprint || '');
          fields.set('source_question_id',help.dataset.sourceQuestionId || '');
        }
        const response = await fetch(help.dataset.helpUrl,{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded;charset=UTF-8','Accept':'application/json'},body:fields});
        const raw = await response.text();
        let result = {};
        try { result = JSON.parse(raw); } catch (_) { result.message = raw.replace(/<[^>]*>/g,' ').trim(); }
        if (!response.ok) throw new Error(result.message || 'Assistance could not be loaded. Review this question and try again.');
        if (panel.hidden || panel.dataset.questionKey !== help.dataset.key || form.querySelector('[name="revision"]').value !== help.dataset.revision) throw new Error('The question changed while assistance was loading. Open Help me answer again.');
        content.innerHTML = result.html;
        content.querySelectorAll('[data-answer-ai]').forEach(root => window.initializeAnswerAI?.(root));
        help.dataset.loaded = 'true';
      } catch (error) {
        content.innerHTML = prior;
        const notice = document.createElement('p'); notice.setAttribute('role','status'); notice.textContent = error.message || 'Assistance could not be loaded.'; content.append(notice);
      } finally { delete help.dataset.loading; }
    });
  });
})();
