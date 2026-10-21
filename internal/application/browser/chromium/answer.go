package chromium

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"example.com/yourmove-lite/internal/application/browser"
	"github.com/coder/websocket"
)

// FillAnswer reuses the read-only observation as the live identity contract.
// Each interaction resolves afresh; a changed field fails locally, leaving all
// other fields and documents alone.
func (s *Session) FillAnswer(ctx context.Context, answer browser.AnswerValue) (browser.AnswerOutcome, error) {
	out := browser.AnswerOutcome{Label: answer.Field.Label}
	c, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	if err := s.lockContext(c); err != nil {
		return out, err
	}
	defer s.mu.Unlock()
	if s.closed {
		return out, errors.New("browser session closed")
	}
	if answer.Field.Sensitive || answer.Value == "" || !strings.HasPrefix(answer.Field.ID, "browser:") {
		out.Status = "manual"
		out.Detail = "This question needs manual entry."
		return out, nil
	}
	target, err := s.target(c)
	if err != nil {
		return out, err
	}
	ws, err := dialCDP(c, target.WebSocketURL)
	if err != nil {
		return out, err
	}
	defer ws.Close(websocket.StatusNormalClosure, "")
	saved, err := url.Parse(s.postingURL)
	if err != nil {
		return out, err
	}
	args, _ := json.Marshal([]any{saved.Scheme + "://" + saved.Host, strings.TrimRight(saved.Path, "/"), answer})
	const operation = `(async function(origin,base,approved){
 const owns=()=>location.origin===origin&&(location.pathname===base||location.pathname.startsWith(base+'/'));
 const locationField=['location','current location','your location'].includes(approved.field.label.toLowerCase().replace(/\s*\*$/, '').trim())&&!!document.querySelector('.ashby-application-form-container');
 const normOptions=o=>o||[];
 const same=c=>c&&('browser:'+c.id)===approved.field.id&&c.label===approved.field.label&&c.kind===approved.field.kind&&c.required===approved.field.required&&!!c.sensitive===!!approved.field.sensitive&&JSON.stringify(normOptions(c.options))===JSON.stringify(normOptions(approved.field.options));
 const visible=e=>e.getClientRects().length&&getComputedStyle(e).visibility!=='hidden'&&!e.closest('[inert],[aria-hidden=true]');
 const application=e=>{const root=e.closest('form,.ashby-application-form-container');return root&&!root.querySelector('input[type=password]')&&(root.matches('.ashby-application-form-container,.ashby-application-form')||root.querySelector('input[type=file]'))};
 const identity=e=>e.closest('[data-field-entry-id]')?.getAttribute('data-field-entry-id')||e.name||e.id||'';
 const resolve=()=>{
 if(!owns())throw Error('application changed');
 const snapshot=OBSERVATION;
 const observed=snapshot.controls.filter(c=>'browser:'+c.id===approved.field.id);
 if(observed.length!==1||!same(observed[0])||observed[0].sensitive)return null;
 const hits=[...document.querySelectorAll('input,textarea,select')].filter(e=>visible(e)&&'browser:'+identity(e)===approved.field.id&&application(e)&&!e.closest('.awli-application-row,.awli-button-container'));
 if(!hits.length||hits.some(e=>e.disabled||e.readOnly||(e.getAttribute('role')==='combobox'&&!locationField)))return null;
 if(approved.field.kind!=='radio'&&hits.length!==1)return null;
 if(approved.field.kind==='radio'&&hits.some(e=>e.type!=='radio'||e.name!==hits[0].name||e.form!==hits[0].form))return null;
 return hits;
 };
 const result=(status,detail)=>({status,detail});
 let hits=resolve();if(!hits)return result('manual','Question identity or options changed, or the control is unsupported. Review it in Chromium.');
 let e=hits[0];const value=approved.value;let expected=value;
 const label=e=>String(e.labels?.[0]?.innerText||e.getAttribute('aria-label')||e.name||'').replace(/\s+/g,' ').trim().slice(0,350);
 const read=hits=>{
 const e=hits[0];
 if(e.type==='radio')return hits.find(e=>e.checked)?label(hits.find(e=>e.checked)):'';
 if(e.type==='checkbox')return e.checked?'true':'false';
 if(e.tagName==='SELECT')return e.value===''?'':e.selectedOptions[0]?.text||'';
 return e.value;
 };
 if(read(hits)===value&&!locationField)return result('correct','Already matches the approved answer.');
 if(hits.some(e=>document.activeElement===e))return result('manual','Active user entry preserved.');
 if(e.type==='checkbox'){
 if(!['true','false'].includes(value)||e.checked)return result('manual','Existing checkbox choice preserved.');
 e.click();
 }else if(e.type==='radio'){
 if(hits.some(e=>e.checked))return result('manual','Existing radio choice preserved.');
 const choices=hits.filter(e=>label(e)===value);if(choices.length!==1)return result('manual','No unique native option.');
 choices[0].click();
 }else if(e.tagName==='SELECT'){
 if(e.multiple||e.value!=='')return result('manual','Existing selection preserved.');
 const choices=[...e.options].filter(o=>o.text===value&&!o.disabled&&!o.parentElement?.disabled);
 if(choices.length!==1)return result('manual','No unique enabled native option.');
 Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype,'value').set.call(e,choices[0].value);
 e.dispatchEvent(new Event('input',{bubbles:true}));if(e.isConnected&&owns())e.dispatchEvent(new Event('change',{bubbles:true}));
 }else{
 if(!['text','email','tel','url','textarea',...(locationField?['combobox']:[])].includes(approved.field.kind)||!['INPUT','TEXTAREA'].includes(e.tagName))return result('unsupported','Complete this control in Chromium.');
 if(e.value!=='')return result('manual','Existing user answer preserved.');
 if(e.maxLength>=0&&value.length>e.maxLength)return result('manual','Answer exceeds the live field limit.');
 const proto=e.tagName==='TEXTAREA'?HTMLTextAreaElement.prototype:HTMLInputElement.prototype;
 e.focus();
 Object.getOwnPropertyDescriptor(proto,'value').set.call(e,value);
 e.dispatchEvent(new InputEvent('input',{bubbles:true,inputType:'insertText',data:null}));if(e.isConnected&&owns())e.dispatchEvent(new Event('change',{bubbles:true}));
 if(locationField){
   let selected=false;
   const normalized=s=>s.toLowerCase().replace(/united states( of america)?/g,'usa').replace(/[^a-z0-9]/g,'');
   for(let attempt=0;attempt<20;attempt++){
     await new Promise(r=>setTimeout(r,150));
     if(!owns()||!e.isConnected||document.activeElement!==e||e.value!==value)return result('uncertain','Location changed during lookup. Review it in Chromium.');
     const controlled=document.getElementById(e.getAttribute('aria-controls')||e.getAttribute('aria-owns')||'');
     const lists=[...document.querySelectorAll('[role="listbox"]')].filter(visible);
     const root=controlled||(lists.length===1?lists[0]:null);
     const options=root?[...root.querySelectorAll('[role="option"]')].filter(o=>visible(o)&&o.getAttribute('aria-disabled')!=='true'&&normalized(o.innerText)===normalized(value)):[];
     if(options.length===1){options[0].click();if(!e.isConnected||normalized(e.value)!==normalized(value))return result('uncertain','Review the selected location in Chromium.');expected=e.value;selected=true;break;}
   }
   if(!selected){if(e.isConnected)e.blur();return result('uncertain','No unique matching location suggestion. Select your location in Chromium.');}
 }
 if(e.isConnected&&owns())e.blur();
 }
 for(const delay of [120,250]){
 await new Promise(r=>setTimeout(r,delay));hits=resolve();
 if(!hits||read(hits)!==expected||hits.some(e=>e.validity&&!e.validity.valid))return result('uncertain','Answer entry could not be verified after the form changed. Review it yourself; it will not be replayed automatically.');
 }
 return result('filled','Filled and independently observed in the live form.');
 })`
	script := strings.Replace(operation, "OBSERVATION", browser.ReadOnlyScript, 1)
	var response struct {
		Result struct {
			Value browser.AnswerOutcome `json:"value"`
		} `json:"result"`
		Exception json.RawMessage `json:"exceptionDetails"`
	}
	err = cdpCall(c, ws, 1, "Runtime.evaluate", map[string]any{"expression": script + "(..." + string(args) + ")", "awaitPromise": true, "returnByValue": true, "silent": true}, &response)
	if err != nil || len(response.Exception) > 0 {
		return out, errors.New("answer transfer could not be verified; review this field in Chromium")
	}
	out.Status = response.Result.Value.Status
	out.Detail = response.Result.Value.Detail
	return out, nil
}
