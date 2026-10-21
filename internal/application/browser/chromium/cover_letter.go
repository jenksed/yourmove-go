package chromium

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// FillCoverLetter handles only an explicitly labeled native cover-letter text
// control. Existing entries are authoritative; no upload fallback is attempted.
func (s *Session) FillCoverLetter(ctx context.Context, body string) (AttachmentResult, error) {
	c, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	if err := s.lockContext(c); err != nil {
		return AttachmentResult{}, err
	}
	defer s.mu.Unlock()
	if s.closed {
		return AttachmentResult{}, errors.New("browser session closed")
	}
	target, err := s.target(c)
	if err != nil {
		return AttachmentResult{}, err
	}
	ws, err := dialCDP(c, target.WebSocketURL)
	if err != nil {
		return AttachmentResult{}, err
	}
	defer ws.Close(websocket.StatusNormalClosure, "")
	saved, err := url.Parse(s.postingURL)
	if err != nil {
		return AttachmentResult{}, err
	}
	args, _ := json.Marshal([]string{saved.Scheme + "://" + saved.Host, strings.TrimRight(saved.Path, "/"), body})
	// Re-resolve after events, preserving React rerenders and conditional changes.
	const operation = `(function(origin,base,body){
 const owns=()=>location.origin===origin&&(location.pathname===base||location.pathname.startsWith(base+'/'));
 const resolve=()=>{
 if(!owns()) throw Error('application changed');
 const hits=[...document.querySelectorAll('textarea,input[type=text]')].filter(e=>{
 const group=e.closest('[data-field-entry-id]');
 const labels=[group?.querySelector('.ashby-application-form-question-title')?.textContent,...[...(e.labels||[])].map(l=>l.textContent),e.getAttribute('aria-label')].filter(x=>x?.trim());
 return e.closest('form,.ashby-application-form-container')&&!e.closest('[inert],[aria-hidden=true],.awli-application-row,.awli-button-container')&&e.getClientRects().length&&getComputedStyle(e).visibility!=='hidden'&&labels.length&&labels.every(x=>/^cover[\s_-]*letter\s*(?:\((?:optional|required)\))?\s*\*?$/i.test(x.trim()));
 });
 return hits.length===1?hits[0]:null;
 };
 const e=resolve();
 if(!e||e.disabled||e.readOnly||e.getAttribute('role')==='combobox') return {selected:false,detail:'No unique supported cover-letter text control. Complete it in Chromium.'};
 if(e.value===body) return {selected:true,verified:true,detail:'Saved cover-letter text already matches the live form.'};
 if(e.value!==''||document.activeElement===e) return {selected:false,detail:'Existing or active cover-letter entry preserved. Review it in Chromium.'};
 if(e.maxLength>=0&&body.length>e.maxLength) return {selected:false,detail:'Saved letter exceeds this field’s limit. Complete it in Chromium.'};
 const proto=e.tagName==='TEXTAREA'?HTMLTextAreaElement.prototype:HTMLInputElement.prototype;
 Object.getOwnPropertyDescriptor(proto,'value').set.call(e,body);
 e.dispatchEvent(new InputEvent('input',{bubbles:true,inputType:'insertText',data:null}));
 if(e.isConnected&&owns())e.dispatchEvent(new Event('change',{bubbles:true}));
 return new Promise(resolveResult=>setTimeout(()=>{
 const current=resolve();const ok=!!current&&current.value===body;
 resolveResult({selected:true,verified:ok,detail:ok?'Saved cover-letter text filled and observed. Review in Chromium.':'Letter entry was attempted but could not be verified. Review it manually; it will not be replayed.'});
 },350));
 })`
	var remote struct {
		Result struct {
			Value AttachmentResult `json:"value"`
		} `json:"result"`
		Exception json.RawMessage `json:"exceptionDetails"`
	}
	err = cdpCall(c, ws, 1, "Runtime.evaluate", map[string]any{"expression": operation + "(..." + string(args) + ")", "returnByValue": true, "awaitPromise": true, "silent": true}, &remote)
	if err != nil || len(remote.Exception) > 0 {
		return AttachmentResult{}, errors.New("cover-letter transfer could not be verified; inspect the employer form before continuing")
	}
	return remote.Result.Value, nil
}
