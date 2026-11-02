package browser

// ContactResolver is engine-agnostic DOM JavaScript owned by the browser
// package and evaluated by every driver. Contact resolution stays local to
// this operation. It uses exact machine semantics plus compatible labels
// within an application form, never positions or question wording alone.
// Every write resolves again in the current DOM.
const ContactResolver = `(function(origin, base) {
 const owns = () => location.origin === origin && (location.pathname === base || location.pathname.startsWith(base + '/'));
 const visible = el => !el.closest('[inert],[aria-hidden="true"]') && getComputedStyle(el).visibility !== 'hidden' && el.getClientRects().length > 0;
 const norm = s => String(s || '').toLowerCase().replace(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}__(systemfield_[a-z_]+)$/, '$1').replace(/[\s_-]/g, '');
 const ids = {name:'name',fullname:'name',systemfieldname:'name',firstname:'first_name',givenname:'first_name',systemfieldfirstname:'first_name',lastname:'last_name',familyname:'last_name',surname:'last_name',systemfieldlastname:'last_name',email:'email',emailaddress:'email',systemfieldemail:'email',phone:'phone',phonenumber:'phone',mobilephone:'phone',systemfieldphone:'phone'};
 const auto = {'name':'name','given-name':'first_name','family-name':'last_name','email':'email','tel':'phone'};
 const labels = {name:['name','full name','full legal name','your name'],first_name:['first name','given name'],last_name:['last name','family name','surname'],email:['email','email address'],phone:['phone','phone number','mobile phone','mobile number']};
 const forms = () => [...document.querySelectorAll('form,.ashby-application-form-container')].filter(f => visible(f) && !f.querySelector('input[type=password]') && !f.parentElement?.closest('form,.ashby-application-form-container') && (f.classList.contains('ashby-application-form-container') || f.classList.contains('ashby-application-form') || f.querySelector('input[type=file]')));
 const identity = el => {
   if (!['text','email','tel'].includes(el.type) || !visible(el) || el.closest('.awli-application-row,.awli-button-container')) return '';
   const group = el.closest('[data-field-entry-id]');
   const signals = [el.name,el.id,group?.getAttribute('data-field-entry-id')].map(x=>ids[norm(x)]).filter(Boolean);
   const ac = (el.getAttribute('autocomplete') || '').trim().toLowerCase().split(/\s+/);
   if (ac.length === 1 || (ac.length === 2 && ac[0].startsWith('section-'))) {
     if (auto[ac.at(-1)]) signals.push(auto[ac.at(-1)]);
   }
   if (!signals.length && el.type === 'tel') signals.push('phone');
   if (!signals.length || new Set(signals).size !== 1) return '';
   const key = signals[0];
   const rawLabels = [group?.querySelector('.ashby-application-form-question-title')?.textContent, ...[...(el.labels || [])].map(x=>x.textContent), el.getAttribute('aria-label')].filter(x=>x?.trim());
   const clean = s => s.toLowerCase().replace(/\((required|optional)\)/g,'').replace(/[*✱]/g,'').replace(/\s+/g,' ').trim();
   if ((el.type === 'tel' && !rawLabels.length) || rawLabels.some(x=>!labels[key].includes(clean(x)))) return '';
   if ((key === 'email' && !['text','email'].includes(el.type)) || (key === 'phone' && !['text','tel'].includes(el.type))) return '';
   return key;
 };
 const hits = key => forms().flatMap(f=>[...f.querySelectorAll('input')].filter(el=>identity(el) === key));
 const resolve = key => {
   if (!owns()) throw Error('application changed');
   if ((key === 'name' && (hits('first_name').length || hits('last_name').length)) || (['first_name','last_name'].includes(key) && hits('name').length)) return {status:'ambiguous',detail:'Both full-name and separate-name controls are present. Complete the name yourself.'};
   const matches = hits(key);
   if (!matches.length) return {status:'missing',detail:'No supported control on this step. Complete it in the employer form if required.'};
   if (matches.length !== 1) return {status:'ambiguous',detail:'Multiple matching controls. Complete this field yourself.'};
   const el = matches[0];
   if (el.matches(':disabled') || el.readOnly || el.getAttribute('role') === 'combobox') return {status:'manual',detail:'This control needs manual entry.'};
   return {el};
 };
 return {
   plan() {
     if (!owns()) throw Error('application changed');
     return {ready:forms().length > 0,keys:hits('name').length ? ['name','email','phone'] : ['first_name','last_name','email','phone']};
   },
   apply(key, value) {
     if (!value) return {status:'skipped',detail:'No approved value. Complete this field if required.'};
     const found = resolve(key); if (!found.el) return found;
     const el = found.el;
     if (el.value === value) return {status:'correct',detail:'Already matches the approved value.'};
     if (el.value !== '' || document.activeElement === el) return {status:'manual',detail:'Existing or active user entry preserved.'};
     if (el.maxLength >= 0 && value.length > el.maxLength) return {status:'manual',detail:'Approved value exceeds this control’s limit.'};
     if (!el.isConnected || !owns()) throw Error('application changed');
     el.focus();
     Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(el,value);
     el.dispatchEvent(new InputEvent('input',{bubbles:true,inputType:'insertText',data:null}));
     if (el.isConnected && owns()) el.dispatchEvent(new Event('change',{bubbles:true}));
     if (el.isConnected && owns()) el.blur();
     return {status:'pending',detail:''};
   },
   verify(key, value) {
     const found = resolve(key);
     if (found.el && found.el.value === value && found.el.validity.valid) return {status:'filled',detail:'Filled and verified in the live form.'};
     return {status:'uncertain',detail:'The form changed after entry. Review this field yourself; it will not be replayed.'};
   }
 };
})`
