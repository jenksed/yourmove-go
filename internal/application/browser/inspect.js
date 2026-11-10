// Read-only DOM observation. Never read input.value, cookie, storage, page
// bodies, attached file bytes, or trigger clicks/network requests.
(() => {
  const clip = (value, n = 220) => String(value || '').trim().slice(0, n);
  const visible = el => {
    const style = getComputedStyle(el);
    return style.display !== 'none' && style.visibility !== 'hidden' &&
      !el.closest('[inert],[aria-hidden="true"]') && el.getClientRects().length > 0;
  };
  // Lever's optional LinkedIn authentication is not an application step.
  const socialAuth = el => !!el.closest('.awli-application-row,.awli-button-container');
  const labelText = label => {
    if (!label) return '';
    const copy = label.cloneNode(true);
    copy.querySelectorAll('input,textarea,select,button,[role="combobox"],[role="listbox"],[role="option"],[role="radio"],[role="checkbox"],[role="group"],[role="radiogroup"]').forEach(el => el.remove());
    return String(copy.textContent || '').replace(/\s+/g, ' ').trim();
  };
  const sensitive = value =>
    /password|ssn|social security|birth|gender|race|ethnic|disabilit|veteran|consent|authorization|sponsorship|salary|criminal|citizenship|visa|work permit|right to work|eligible.*work|legally|nationality|security clearance|background check|compensation|pay expectation|relocat|medical|health|voluntary|terms|agree|acknowledg|certif|attest|submit/i.test(value);
  const token = value => encodeURIComponent(String(value || '').trim());
  const entryRoot = el => el.closest('[data-field-entry-id]');
  const entryValue = root => root?.closest('[data-field-entry-id]')?.getAttribute('data-field-entry-id')?.trim() || '';
  const questionTitle = root =>
    root?.querySelector('.ashby-application-form-question-title')?.innerText ||
    root?.querySelector('legend')?.innerText ||
    root?.getAttribute('aria-label') || '';
  const ariaText = el => {
    const labelledBy = (el.getAttribute('aria-labelledby') || '').split(/\s+/).filter(Boolean);
    const referenced = labelledBy.map(id => document.getElementById(id)?.innerText || '').filter(Boolean).join(' ');
    return clip(el.getAttribute('aria-label') || referenced || el.innerText || el.textContent, 350);
  };
  const labelledByText = el => {
    if (!el || typeof el.getAttribute !== 'function') return '';
    const ids = (el.getAttribute('aria-labelledby') || '').split(/\s+/).filter(Boolean);
    return clip(ids.map(id => document.getElementById(id)?.innerText || '').filter(Boolean).join(' '), 350);
  };
  // Hosted application forms (for example Lever's .application-question
  // items) often render the employer's question as plain text in a container
  // heading rather than in a <label>, <legend>, or ARIA relationship. The
  // container heading is authoritative DOM semantics for the enclosed
  // control; input placeholders, default values, and machine names below
  // are not.
  const containerQuestionLabel = el => {
    const container = el?.closest?.('.application-question');
    if (!container) return '';
    return clip(labelText(container.querySelector('.application-label')), 350);
  };
  const describe = el => {
    const root = entryRoot(el);
    const title = questionTitle(root) || el.closest('fieldset')?.querySelector('legend')?.innerText;
    const labelFromId = el.id ? document.querySelector('label[for="' + CSS.escape(el.id) + '"]') : null;
    // Custom structured controls are often labelled through an accessible
    // relationship (aria-labelledby), a wrapping label, or a semantic group
    // container rather than a directly associated <label>.
    const group = el.closest('[role="group"],[role="radiogroup"]');
    const groupLabel = group && group !== el
      ? clip(group.getAttribute('aria-label') || labelledByText(group), 350) : '';
    // Placeholders, default values, and machine names are deliberately not
    // question wording: they hint at or identify the control but never state
    // the employer's question. An empty result keeps the honest
    // unidentified-control notice instead of a fabricated label.
    return clip(title || labelText(el.labels?.[0]) || labelText(labelFromId) ||
      labelledByText(el) || labelText(el.closest('label')) || groupLabel ||
      el.getAttribute('aria-label') || containerQuestionLabel(el) || '', 350);
  };
  // Sensitivity still considers hint and machine text: a control whose only
  // wording is a placeholder or name keeps its privacy handling even though
  // that text never becomes its displayed question.
  const sensitiveHints = el =>
    sensitive((el.getAttribute('aria-label') || '') + ' ' + (el.getAttribute('placeholder') || '') + ' ' + (el.name || ''));
  // A label element that encloses every member of a group names the group;
  // a label that encloses one member names that option and is never used
  // as the group question.
  const sharedGroupLabel = members => {
    if (members.length < 2 || !members[0].labels) return '';
    for (const label of members[0].labels) {
      if (members.every(member => label.contains(member))) return labelText(label);
    }
    return '';
  };
  const optionLabel = el => clip(labelText(el.labels?.[0]) || ariaText(el) || el.name || 'Unlabeled option', 350);
  const kindOf = el => {
    if (el.type === 'file') return 'file';
    if (el.tagName.toLowerCase() === 'textarea') return 'textarea';
    if (el.tagName.toLowerCase() === 'select') return 'select';
    if (el.getAttribute('role') === 'combobox') {
      return el.tagName.toLowerCase() === 'input' ? 'combobox' : 'custom_combobox';
    }
    if (el.getAttribute('role') === 'listbox') {
      return 'listbox';
    }
    if (el.getAttribute('role') === 'checkbox') return 'custom_checkbox';
    if (el.getAttribute('role') === 'radio') return 'custom_radio';
    if (el.type === 'checkbox') return 'checkbox';
    if (el.type === 'radio') return 'radio';
    return el.type || 'text';
  };
  const raw = [...document.querySelectorAll('input,textarea,select,[role="combobox"],[role="listbox"],[role="checkbox"],[role="radio"]')]
    .filter(el => visible(el) && !socialAuth(el) &&
      !['hidden', 'password', 'submit', 'button', 'reset'].includes(el.type || ''));
  const candidates = raw.slice(0, 200).filter(el => {
    const roleRoot = el.closest('[role="combobox"],[role="checkbox"],[role="radio"]');
    if (!roleRoot || roleRoot === el || el.tagName !== 'INPUT') return true;
    if (roleRoot.getAttribute('role') === 'combobox') return false;
    return (roleRoot.getAttribute('role') === 'checkbox' && el.type === 'checkbox') ||
      (roleRoot.getAttribute('role') === 'radio' && el.type === 'radio');
  });
  // A listbox referenced as the popup of an observed combobox (via
  // aria-controls/aria-owns) belongs to that control: its visible options
  // are collected onto the combobox record instead of reported twice.
  const popupOwners = new Map();
  for (const el of candidates) {
    if (!['combobox', 'custom_combobox'].includes(kindOf(el))) continue;
    for (const attr of ['aria-controls', 'aria-owns']) {
      const id = (el.getAttribute(attr) || '').split(/\s+/).filter(Boolean)[0] || '';
      if (id && !popupOwners.has(id)) popupOwners.set(id, el);
    }
  }
  const ownedPopup = el => {
    if (kindOf(el) !== 'listbox' || !el.id || !popupOwners.has(el.id)) return null;
    return popupOwners.get(el.id);
  };
  const records = [];
  const unknown = [];
  const add = (record, members) => {
    if (!record.id) {
      unknown.push(...members);
      return;
    }
    records.push({ ...record, order: Math.min(...members.map(el => candidates.indexOf(el))) });
  };
  const rootIdentity = (root, members, suffix = '') => {
    const entry = entryValue(root);
    const sharedName = members.length && members.every(el => el.name && el.name === members[0].name)
      ? members[0].name : '';
    const control = suffix || sharedName;
    if (entry) return entry;
    if (control) return control;
    if (root?.id) return token(root.id);
    const ids = members.map(el => el.name || el.id);
    return ids.length === members.length && ids.every(Boolean) ? 'group__' + ids.map(token).sort().join('__') : '';
  };
  const fieldIdentity = (el, root = entryRoot(el)) => {
    const entry = entryValue(root);
    const control = el.name || el.id || '';
    if (entry) return entry;
    return control;
  };
  const makeRecord = (members, kind, root, options = [], identity = '') => {
    const labelledBy = (members[0]?.getAttribute('aria-labelledby') || '')
      .split(/\s+/).filter(Boolean).map(id => document.getElementById(id)?.innerText || '').filter(Boolean).join(' ');
    // The group question comes only from reliable DOM semantics: the entry or
    // structural container heading, an accessible name, the enclosing
    // application-question heading, or one label shared by every member.
    // Per-option wrapping labels describe one choice, never the group, and
    // machine names are never question wording, so neither is used here.
    // Without semantics the record keeps the honest unidentified-control
    // notice.
    const groupLabel = root?.getAttribute('aria-label') || labelledBy ||
      containerQuestionLabel(members[0]) || sharedGroupLabel(members);
    const label = clip(questionTitle(root) || groupLabel || (members.length === 1 ? describe(members[0]) : ''), 350);
    return {
      id: identity || (members.length === 1 ? fieldIdentity(members[0], root) : rootIdentity(root, members)),
      label: label || 'Unlabeled control',
      kind,
      required: members.some(el => !!(el.required || el.getAttribute('aria-required') === 'true')),
      sensitive: sensitive(label) || options.some(sensitive) || members.some(sensitiveHints),
      options
    };
  };
  const groupBy = (members, keyFor) => {
    const groups = new Map();
    for (const el of members) {
      const key = keyFor(el);
      if (!groups.has(key)) groups.set(key, []);
      groups.get(key).push(el);
    }
    return [...groups.values()];
  };
  const optionLabels = members => members.map(optionLabel);
  const equalOptions = (left, right) =>
    left.length === right.length && left.every((value, index) => value === right[index]);
  const mergeOptions = groups => {
    if (groups.length > 1 && groups.every(group => equalOptions(group, groups[0]))) return groups[0];
    return groups.flat();
  };
  // Read-only option evidence for ARIA structured controls. Only options
  // already rendered in the visible DOM are collected; nothing is opened,
  // clicked, or otherwise interacted with to discover choices. Options from
  // a referenced popup (aria-controls/aria-owns) are collected only for
  // controls without a stable control identity: a named control may be a
  // Fill or save candidate, so its record must stay identical across
  // transient popup states (for example location suggestions that appear
  // only after typing), while an identity-less custom control is
  // display-only and keeps its visible choices as read-only evidence.
  const ariaOptions = el => {
    const seen = new Set();
    const out = [];
    const push = node => {
      const value = clip(node.innerText || node.textContent, 110);
      if (value !== '' && !seen.has(value) && out.length < 60) {
        seen.add(value);
        out.push(value);
      }
    };
    el.querySelectorAll('[role="option"]').forEach(opt => { if (visible(opt)) push(opt); });
    if (!el.name && !el.id) {
      for (const attr of ['aria-controls', 'aria-owns']) {
        const id = (el.getAttribute(attr) || '').split(/\s+/).filter(Boolean)[0] || '';
        const target = id ? document.getElementById(id) : null;
        if (target && target !== el) {
          target.querySelectorAll('[role="option"]').forEach(opt => { if (visible(opt)) push(opt); });
        }
      }
    }
    return out;
  };
  // Relationship-scoped display identity for recognizable custom controls
  // without a stable control identity. It derives only from real DOM
  // references (group containers, owned popups, labelling elements), never
  // from label wording, and only custom kinds that are never Fill-eligible
  // receive it, so it cannot become a transfer identity.
  const containerReference = el => {
    const group = el.closest('[role="group"],[role="radiogroup"],fieldset');
    const groupRef = group && group !== el ? (group.id || group.getAttribute('aria-labelledby') || '') : '';
    if (groupRef) return 'aria_group_' + groupRef;
    for (const attr of ['aria-controls', 'aria-owns']) {
      const owned = (el.getAttribute(attr) || '').split(/\s+/).filter(Boolean)[0] || '';
      if (owned && document.getElementById(owned)) return 'aria_group_' + owned;
    }
    const labelled = (el.getAttribute('aria-labelledby') || '').split(/\s+/).filter(Boolean)[0] || '';
    if (labelled && document.getElementById(labelled)) return 'aria_labelled_' + labelled;
    return '';
  };
  // Custom structured controls can never be operated automatically, so a
  // relationship-scoped display identity keeps their real question visible
  // instead of collapsing it into the unidentified-control notice. Native
  // kinds keep the existing rule: no stable identity means no display row.
  const customDisplayKind = kind =>
    ['combobox', 'custom_combobox', 'listbox', 'custom_radio_group', 'custom_checkbox_group', 'unsupported_ambiguous'].includes(kind);
  const selectishOptions = el => {
    const kind = kindOf(el);
    if (kind === 'select') return [...el.options].slice(0, 60).map(option => clip(option.text, 110));
    if (['combobox', 'custom_combobox', 'listbox'].includes(kind)) return ariaOptions(el);
    return [];
  };
  // One record for one observed control. A referenced listbox popup is
  // reported through its combobox when that control is itself displayed
  // under the same question label; otherwise it stands as its own record.
  const singleRecord = (el, root) => {
    const kind = kindOf(el);
    const options = selectishOptions(el);
    let identity = fieldIdentity(el, root);
    if (!identity && customDisplayKind(kind)) identity = containerReference(el);
    const record = makeRecord([el], kind, root, options, identity);
    const owner = ownedPopup(el);
    if (owner) {
      const ownerKind = kindOf(owner);
      let ownerId = fieldIdentity(owner, entryRoot(owner));
      if (!ownerId && customDisplayKind(ownerKind)) ownerId = containerReference(owner);
      if (ownerId) {
        const ownerRecord = makeRecord([owner], ownerKind, entryRoot(owner), selectishOptions(owner), ownerId);
        if (ownerRecord.label === record.label) return null;
      }
    }
    return record;
  };
  const processScope = (root, members, ashbyEntry) => {
    const units = [];
    const handled = new Set();
    const radios = members.filter(el => ['radio', 'custom_radio'].includes(kindOf(el)));
    if (radios.length) {
      const native = radios.filter(el => kindOf(el) === 'radio');
      const custom = radios.filter(el => kindOf(el) === 'custom_radio');
      if (native.length) {
        const groups = groupBy(native, el => el.name || '__unnamed_radio_group');
        for (const group of groups) {
          const name = group[0].name;
          const id = rootIdentity(root, group, name);
          units.push({ members: group, family: 'radio', representation: 'native',
            record: makeRecord(group, name ? 'radio' : 'unsupported_ambiguous', root, optionLabels(group), id) });
        }
      }
      if (custom.length) {
        const groups = groupBy(custom, el => el.closest('[role="group"],[role="radiogroup"]') || root || el.parentElement);
        for (const group of groups) {
          const groupRoot = group[0].closest('[role="group"],[role="radiogroup"]') || root;
          const options = group.map(ariaText);
          const groupReference = groupRoot?.id || groupRoot?.getAttribute('aria-labelledby') ||
            group[0].getAttribute('aria-labelledby') || '';
          units.push({ members: group, family: 'radio', representation: 'custom',
            record: makeRecord(group, 'custom_radio_group', groupRoot, options,
              rootIdentity(groupRoot, group, entryValue(root) ? 'aria_radio' : (groupReference ? 'aria_group_' + groupReference : ''))) });
        }
      }
      radios.forEach(el => handled.add(el));
    }

    const checkboxes = members.filter(el => ['checkbox', 'custom_checkbox'].includes(kindOf(el)));
    if (checkboxes.length) {
      const native = checkboxes.filter(el => kindOf(el) === 'checkbox');
      const custom = checkboxes.filter(el => kindOf(el) === 'custom_checkbox');
      if (native.length) {
        const byName = groupBy(native, el => el.name ? 'name:' + el.name : el);
        const namedGroups = byName.filter(group => group[0].name);
        const unnamed = byName.filter(group => !group[0].name).flat();
        const checkboxGroups = ashbyEntry ? [native] : namedGroups;
        for (const group of checkboxGroups) {
          const grouped = group.length > 1;
          const kind = grouped ? 'checkbox_group' : 'checkbox';
          const identity = grouped ? rootIdentity(root, group, ashbyEntry ? 'checkbox_group' : group[0].name) : fieldIdentity(group[0], root);
          units.push({ members: group, family: 'checkbox', representation: 'native',
            record: makeRecord(group, kind, root, grouped ? optionLabels(group) : [], identity) });
        }
        if (!ashbyEntry && unnamed.length > 1 && questionTitle(root)) {
          const identity = rootIdentity(root, unnamed);
          units.push({ members: unnamed, family: 'checkbox', representation: 'native',
            record: makeRecord(unnamed, 'unsupported_ambiguous', root, optionLabels(unnamed), identity) });
        } else {
          for (const checkbox of unnamed) {
            units.push({ members: [checkbox], family: 'checkbox', representation: 'native',
              record: makeRecord([checkbox], 'checkbox', root, [], fieldIdentity(checkbox, root)) });
          }
        }
      }
      if (custom.length) {
        const groups = groupBy(custom, el => {
          const semanticRoot = el.closest('[role="group"]');
          if (ashbyEntry) return root;
          if (semanticRoot) return semanticRoot;
          const labelledBy = el.getAttribute('aria-labelledby') || '';
          return labelledBy ? 'labelled:' + labelledBy : el;
        });
        for (const group of groups) {
          const groupRoot = group[0].closest('[role="group"]') || root;
          const grouped = group.length > 1 || !!group[0].closest('[role="group"]');
          const reference = groupRoot?.id || groupRoot?.getAttribute('aria-labelledby') || group[0].getAttribute('aria-labelledby') || '';
          const kind = grouped ? 'custom_checkbox_group' : 'custom_checkbox';
          const identity = grouped
            ? rootIdentity(groupRoot, group, entryValue(root) ? 'aria_checkbox' : (reference ? 'aria_group_' + reference : ''))
            : fieldIdentity(group[0], root);
          units.push({ members: group, family: 'checkbox', representation: 'custom',
            record: makeRecord(group, kind, groupRoot, grouped ? group.map(ariaText) : [], identity) });
        }
      }
      checkboxes.forEach(el => handled.add(el));
    }

    for (const el of members) {
      if (handled.has(el)) continue;
      const kind = kindOf(el);
      const record = singleRecord(el, root);
      if (!record) continue;
      units.push({ members: [el], family: kind === 'select' || kind === 'combobox' || kind === 'custom_combobox' || kind === 'listbox' ? 'selectish' : kind,
        representation: kind === 'combobox' || kind === 'custom_combobox' || kind === 'listbox' ? 'custom' : 'native', record });
    }

    // One Ashby field-entry ID is the question boundary. If native and custom
    // presentations both appear under that boundary, merge only their displayed
    // option lists and mark the result ambiguous/manual-only.
    if (ashbyEntry) {
      const families = new Set(units.map(unit => unit.family));
      for (const family of families) {
        const related = units.filter(unit => unit.family === family);
        const representations = new Set(related.map(unit => unit.representation));
        if (related.length < 2 || representations.size < 2) continue;
        const options = mergeOptions(related.map(unit => unit.record.options || []));
        const membersInFamily = related.flatMap(unit => unit.members);
        const native = related.find(unit => unit.representation === 'native');
        const id = native?.record.id || rootIdentity(root, membersInFamily, family + '_group');
        const merged = makeRecord(membersInFamily, 'unsupported_ambiguous', root, options, id);
        for (let i = units.length - 1; i >= 0; i--) {
          if (units[i].family === family) units.splice(i, 1);
        }
        units.push({ members: membersInFamily, family, representation: 'ambiguous', record: merged });
      }
      if (units.length > 1) {
        const allMembers = units.flatMap(unit => unit.members);
        const allOptions = units.flatMap(unit => unit.record.options || []);
        const ambiguous = makeRecord(allMembers, 'unsupported_ambiguous', root, allOptions, entryValue(root));
        units.splice(0, units.length, { members: allMembers, family: 'ambiguous', representation: 'ambiguous', record: ambiguous });
      }
    }
    for (const unit of units) add(unit.record, unit.members);
  };

  const entryRoots = [...new Set(candidates.map(entryRoot).filter(Boolean))];
  const assigned = new Set();
  for (const root of entryRoots) {
    const members = candidates.filter(el => entryRoot(el) === root);
    members.forEach(el => assigned.add(el));
    processScope(root, members, true);
  }
  const remaining = candidates.filter(el => !assigned.has(el));
  const structuralRoots = [...new Set(remaining.map(el => el.closest('fieldset,[role="group"],[role="radiogroup"]')).filter(Boolean))];
  for (const root of structuralRoots) {
    const members = remaining.filter(el => el.closest('fieldset,[role="group"],[role="radiogroup"]') === root);
    members.forEach(el => assigned.add(el));
    processScope(root, members, false);
  }
  const looseRadios = candidates.filter(el => !assigned.has(el) && kindOf(el) === 'radio' && el.name);
  const formGroups = new Map();
  for (const el of looseRadios) {
    const form = el.form || document;
    if (!formGroups.has(form)) formGroups.set(form, new Map());
    const byName = formGroups.get(form);
    if (!byName.has(el.name)) byName.set(el.name, []);
    byName.get(el.name).push(el);
  }
  for (const groups of formGroups.values()) {
    for (const members of groups.values()) {
      members.forEach(el => assigned.add(el));
      processScope(null, members, false);
    }
  }
  const looseCustomRadioGroups = new Map();
  for (const el of candidates.filter(el => !assigned.has(el) && kindOf(el) === 'custom_radio')) {
    const reference = el.getAttribute('aria-labelledby') || '';
    if (!reference) continue;
    const key = (el.form?.id || '') + '|' + reference;
    if (!looseCustomRadioGroups.has(key)) looseCustomRadioGroups.set(key, []);
    looseCustomRadioGroups.get(key).push(el);
  }
  for (const members of looseCustomRadioGroups.values()) {
    if (members.length < 2) continue;
    members.forEach(el => assigned.add(el));
    processScope(null, members, false);
  }
  for (const el of candidates) {
    if (assigned.has(el)) continue;
    const record = singleRecord(el, null);
    if (!record) continue;
    add(record, [el]);
  }
  if (unknown.length) {
    records.push({
      id: 'unidentified-controls',
      label: 'Controls without a stable field identity',
      kind: 'unsupported_ambiguous',
      required: unknown.some(el => !!(el.required || el.getAttribute('aria-required') === 'true')),
      sensitive: unknown.some(el => sensitive(describe(el)) || sensitiveHints(el)),
      options: [],
      order: Math.min(...unknown.map(el => candidates.indexOf(el)))
    });
  }
  records.sort((left, right) => left.order - right.order);
  const controls = records.slice(0, 200).map(({ order, ...record }) => ({
    id: clip(record.id, 180),
    label: clip(record.label, 350),
    kind: clip(record.kind, 100),
    required: record.required,
    sensitive: record.sensitive,
    options: record.options.slice(0, 60).map(option => clip(option, 110))
  }));

  const rawActions = [...document.querySelectorAll('button,input[type="submit"],a[href],[role="button"]')]
    .filter(el => visible(el) && !socialAuth(el)).slice(0, 60);
  const actions = rawActions.map(el => ({
    label: clip(el.innerText || el.getAttribute('aria-label') ||
      (el.matches('input[type="submit"]') ? 'Submit application' : '') || el.textContent, 150),
    method: clip(el.form?.getAttribute('method') || '', 16),
    destination: clip(el.getAttribute('href') || el.form?.getAttribute('action') || '', 300)
  })).filter(action => action.label);
  const rawFrames = [...document.querySelectorAll('iframe')];
  const frames = rawFrames.slice(0, 20).map(frame => ({
    host: (() => { try { return new URL(frame.src, location.href).hostname; } catch { return 'unknown'; } })(),
    title: clip(frame.title, 100)
  }));
  return {
    url: location.href,
    title: clip(document.title, 180),
    heading: clip(document.querySelector('h1,h2,[role="heading"]')?.innerText, 180),
    readyState: document.readyState,
    forms: document.forms.length,
    rawControlCount: raw.length,
    controls,
    actions,
    frames,
    truncated: raw.length > 200 || records.length > 200 || rawActions.length > 60 || rawFrames.length > 20
  };
})();
