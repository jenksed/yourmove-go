(() => {
  const shell = document.getElementById('prototype-shell');
  const params = new URLSearchParams(window.location.search);
  const shellChoices = new Set(['baseline', 'refined', 'collapsible', 'adaptive']);
  const views = new Set(['jobs', 'prepare', 'letter', 'documents', 'candidate', 'ai', 'type', 'library', 'evidence', 'applications', 'utilities']);
  const typeChoices = new Set(['system', 'selective', 'expanded']);
  const selectedShell = shellChoices.has(params.get('shell')) ? params.get('shell') : 'baseline';
  const selectedView = views.has(params.get('view')) ? params.get('view') : 'jobs';
  const selectedType = typeChoices.has(params.get('type')) ? params.get('type') : 'system';
  const headingWeight = ['300', '400', '500', '600', '700'].includes(params.get('weight')) ? params.get('weight') : '600';
  const requestedMode = ['expanded', 'compact'].includes(params.get('mode')) ? params.get('mode') : '';
  const isManualMode = Boolean(requestedMode);
  const query = new URLSearchParams(params);

  shell.dataset.shell = selectedShell;
  shell.dataset.view = selectedView;
  shell.dataset.type = selectedType;
  shell.dataset.headingWeight = headingWeight;
  if (selectedShell === 'adaptive') {
    shell.dataset.navMode = requestedMode || (window.matchMedia('(max-width: 920px)').matches ? 'compact' : 'expanded');
  } else {
    shell.dataset.navMode = requestedMode || (selectedShell === 'collapsible' ? 'expanded' : 'expanded');
  }
  if (isManualMode) shell.dataset.navManual = 'true';
  if (params.get('theme') === 'light' || params.get('theme') === 'dark') {
    document.documentElement.dataset.theme = params.get('theme');
  }

  document.querySelectorAll('[data-view-section]').forEach(section => {
    section.hidden = section.dataset.viewSection !== selectedView;
  });
  const activeDestination = selectedView === 'prepare' || selectedView === 'letter' ? 'jobs' : selectedView;
  document.querySelectorAll('[data-destination]').forEach(link => {
    const isActive = link.dataset.destination === activeDestination;
    link.classList.toggle('is-active', isActive);
    if (isActive) link.setAttribute('aria-current', 'page');
    else link.removeAttribute('aria-current');
  });

  const focusTarget = params.get('focus');
  const focusSelector = focusTarget === 'answer' ? '#q1' : focusTarget === 'letter' ? '.letter-editor-demo' : '';
  if (focusSelector && selectedView !== 'type') {
    window.requestAnimationFrame(() => document.querySelector(focusSelector)?.scrollIntoView({block: 'start'}));
  }

  const appearance = document.getElementById('appearance');
  appearance.value = params.get('theme') || 'system';
  appearance.addEventListener('change', () => {
    const next = new URLSearchParams(window.location.search);
    if (appearance.value === 'system') next.delete('theme');
    else next.set('theme', appearance.value);
    window.location.search = next.toString();
  });

  const toggle = document.getElementById('nav-toggle');
  const updateToggle = () => {
    const expanded = shell.dataset.navMode !== 'compact';
    toggle.setAttribute('aria-expanded', String(expanded));
    toggle.querySelector('.toggle-glyph').textContent = expanded ? '‹' : '›';
    toggle.querySelector('.toggle-label').textContent = expanded ? 'Collapse navigation' : 'Expand navigation';
  };
  updateToggle();
  toggle.addEventListener('click', () => {
    shell.dataset.navManual = 'true';
    shell.dataset.navMode = shell.dataset.navMode === 'compact' ? 'expanded' : 'compact';
    updateToggle();
    query.set('shell', selectedShell);
    query.set('view', selectedView);
    query.set('type', selectedType);
    query.set('weight', headingWeight);
    query.set('mode', shell.dataset.navMode);
    window.history.replaceState(null, '', `${window.location.pathname}?${query.toString()}`);
  });

  document.querySelectorAll('a[href*="view="]').forEach(link => {
    link.addEventListener('click', event => {
      const target = new URL(link.href, window.location.href);
      const nextView = target.searchParams.get('view');
      if (!nextView) return;
      event.preventDefault();
      const next = new URLSearchParams(window.location.search);
      next.set('shell', selectedShell);
      next.set('view', nextView);
      next.set('type', selectedType);
      next.set('weight', headingWeight);
      if (params.get('theme')) next.set('theme', params.get('theme'));
      if (shell.dataset.navManual) next.set('mode', shell.dataset.navMode);
      window.location.search = next.toString();
    });
  });

  if (selectedShell === 'adaptive' && !shell.dataset.navManual) {
    const media = window.matchMedia('(max-width: 920px)');
    media.addEventListener('change', event => {
      shell.dataset.navMode = event.matches ? 'compact' : 'expanded';
      updateToggle();
    });
  }
})();
