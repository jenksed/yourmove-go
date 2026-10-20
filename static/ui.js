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
