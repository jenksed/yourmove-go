// Read-only DOM observation. Never read input.value, cookie, storage, page
// bodies, attached file bytes, or trigger clicks/network requests.
(() => {
 const clip=(value,n=220)=>String(value||'').trim().slice(0,n);
 const visible=(el)=>{const s=getComputedStyle(el);return s.display!=='none'&&s.visibility!=='hidden'&&el.getClientRects().length>0};
 // Lever's optional LinkedIn authentication is not an application step.
 const socialAuth=(el)=>!!el.closest('.awli-application-row,.awli-button-container');
 const labelText=(label)=>{
   if(!label)return '';
   const copy=label.cloneNode(true);
   copy.querySelectorAll('input,textarea,select,button,[role="combobox"]').forEach(e=>e.remove());
   return String(copy.textContent||'').replace(/\s+/g,' ').trim();
 };
 const describe=(el)=>{
   const title=el.closest('[data-field-entry-id]')?.querySelector('.ashby-application-form-question-title')?.innerText;
   const labelFromId=el.id?document.querySelector('label[for="'+CSS.escape(el.id)+'"]'):null;
   return clip(title||labelText(el.labels?.[0])||labelText(labelFromId)||el.getAttribute('aria-label')||el.getAttribute('placeholder')||el.name||'Unnamed control',350);
 };
 const sensitive=(s)=>/password|ssn|social security|birth|gender|race|ethnic|disabilit|veteran|consent|authorization|sponsorship|salary|criminal|citizenship|visa|work permit|right to work|eligible.*work|legally|nationality|security clearance|background check|compensation|pay expectation|relocat|medical|health|voluntary|terms|agree|acknowledg|certif|attest|submit/i.test(s);
 const raw=[...document.querySelectorAll('input,textarea,select,[role="combobox"],[role="checkbox"],[role="radio"]')].filter(el=>visible(el)&&!socialAuth(el)&&!['hidden','password','submit','button','reset'].includes(el.type||''));
 const radioSeen=new Set();
 const controls=raw.slice(0,200).map((el,i)=>{
    const kind=el.type==='file'?'file':el.tagName.toLowerCase()==='textarea'?'textarea':el.tagName.toLowerCase()==='select'?'select':el.type==='checkbox'?'checkbox':el.type==='radio'?'radio':el.getAttribute('role')||el.type||'text';
    let label=describe(el);
    let radioOptions=[];
    if(kind==='radio'){
      const group=el.closest('[data-field-entry-id],fieldset');
      if(group&&el.name){
        const groupId=group.getAttribute('data-field-entry-id')||el.name;
        if(radioSeen.has(groupId))return null;
        radioSeen.add(groupId);
        label=clip(group.querySelector('.ashby-application-form-question-title,legend')?.innerText,350)||label;
        radioOptions=[...group.querySelectorAll('input[type=radio]')].filter(e=>e.name===el.name&&visible(e)).map(e=>clip(labelText(e.labels?.[0])||e.getAttribute('aria-label')||e.name,350));
      }
    }
    return {id:clip(el.closest('[data-field-entry-id]')?.getAttribute('data-field-entry-id')||el.getAttribute('name')||el.id||('unidentified-'+i),180),label,kind,required:!!(el.required||el.getAttribute('aria-required')==='true'),sensitive:sensitive(label),options:el.tagName.toLowerCase()==='select'?[...el.options].slice(0,60).map(o=>clip(o.text,110)):radioOptions};
 }).filter(Boolean);
 const rawActions=[...document.querySelectorAll('button,input[type="submit"],a[href],[role="button"]')].filter(el=>visible(el)&&!socialAuth(el)).slice(0,60);
 const actions=rawActions.map(el=>({label:clip(el.innerText||el.value||el.getAttribute('aria-label')||el.textContent,150),method:clip(el.form?.getAttribute('method')||'',16),destination:clip(el.getAttribute('href')||el.form?.getAttribute('action')||'',300)})).filter(a=>a.label);
 const rawFrames=[...document.querySelectorAll('iframe')];
 const frames=rawFrames.slice(0,20).map(f=>({host:(()=>{try{return new URL(f.src,location.href).hostname}catch{return 'unknown'}})(),title:clip(f.title,100)}));
 return {url:location.href,title:clip(document.title,180),heading:clip(document.querySelector('h1,h2,[role="heading"]')?.innerText,180),readyState:document.readyState,forms:document.forms.length,controls,actions,frames,truncated:raw.length>200||rawActions.length>60||rawFrames.length>20};
})()
