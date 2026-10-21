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

// Contact resolution stays local to this operation. It uses exact machine
// semantics plus compatible labels within an application form, never positions
// or question wording alone. Every write resolves again in the current DOM.
const contactResolver = `(function(origin, base) {
 const owns = () => location.origin === origin && (location.pathname === base || location.pathname.startsWith(base + '/'));
 const visible = el => !el.closest('[inert],[aria-hidden="true"]') && getComputedStyle(el).visibility !== 'hidden' && el.getClientRects().length > 0;
 const norm = s => String(s || '').toLowerCase().replace(/[\s_-]/g, '');
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
     Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(el,value);
     el.dispatchEvent(new InputEvent('input',{bubbles:true,inputType:'insertText',data:null}));
     if (el.isConnected && owns()) el.dispatchEvent(new Event('change',{bubbles:true}));
     return {status:'pending',detail:''};
   },
   verify(key, value) {
     const found = resolve(key);
     if (found.el && found.el.value === value) return {status:'filled',detail:'Filled and verified in the live form.'};
     return {status:'uncertain',detail:'The form changed after entry. Review this field yourself; it will not be replayed.'};
   }
 };
})`

// FillContacts shares the existing session lock with observation and attachment.
// It never navigates, submits, selects files, or overwrites a nonempty value.
func (s *Session) FillContacts(ctx context.Context, approved browser.ContactValues) (browser.ContactResult, error) {
	c, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	// Waiting for another operation is also cancellable and bounded.
	if err := s.lockContext(c); err != nil {
		return browser.ContactResult{}, err
	}
	defer s.mu.Unlock()
	if s.closed {
		return browser.ContactResult{}, errors.New("browser session closed")
	}
	target, err := s.target(c)
	if err != nil {
		return browser.ContactResult{}, err
	}
	ws, err := dialCDP(c, target.WebSocketURL)
	if err != nil {
		return browser.ContactResult{}, err
	}
	defer ws.Close(websocket.StatusNormalClosure, "")
	saved, err := url.Parse(s.postingURL)
	if err != nil {
		return browser.ContactResult{}, err
	}
	boundary, _ := json.Marshal([]string{saved.Scheme + "://" + saved.Host, strings.TrimRight(saved.Path, "/")})
	var remote struct {
		Result struct {
			ObjectID string `json:"objectId"`
		} `json:"result"`
	}
	if err := cdpCall(c, ws, 1, "Runtime.evaluate", map[string]any{"expression": contactResolver + "(..." + string(boundary) + ")", "silent": true}, &remote); err != nil {
		return browser.ContactResult{}, err
	}
	if remote.Result.ObjectID == "" {
		return browser.ContactResult{}, errors.New("contact controls are unavailable")
	}
	id := 1
	call := func(method, key, value string, out any) error {
		id++
		var response struct {
			Result struct {
				Value json.RawMessage `json:"value"`
			} `json:"result"`
			Exception json.RawMessage `json:"exceptionDetails"`
		}
		err := cdpCall(c, ws, id, "Runtime.callFunctionOn", map[string]any{
			"objectId": remote.Result.ObjectID, "functionDeclaration": "function(method,key,value){return this[method](key,value)}",
			"arguments": []any{map[string]any{"value": method}, map[string]any{"value": key}, map[string]any{"value": value}}, "returnByValue": true, "silent": true,
		}, &response)
		if err != nil {
			return err
		}
		if len(response.Exception) > 0 || len(response.Result.Value) == 0 {
			return errors.New("contact transfer could not verify the current application")
		}
		return json.Unmarshal(response.Result.Value, out)
	}
	var plan struct {
		Ready bool     `json:"ready"`
		Keys  []string `json:"keys"`
	}
	if err := call("plan", "", "", &plan); err != nil {
		return browser.ContactResult{}, err
	}
	result := browser.ContactResult{Ready: plan.Ready}
	if !plan.Ready {
		return result, nil
	}
	for i, key := range plan.Keys {
		outcome := browser.ContactOutcome{Key: key}
		err := call("apply", key, approved.Value(key), &outcome)
		if err == nil && outcome.Status == "pending" {
			// Allow input handlers and replacement controls to settle, then read twice.
			for _, delay := range []time.Duration{120 * time.Millisecond, 250 * time.Millisecond} {
				timer := time.NewTimer(delay)
				select {
				case <-c.Done():
					timer.Stop()
					err = c.Err()
				case <-timer.C:
					err = call("verify", key, approved.Value(key), &outcome)
				}
				if err != nil || outcome.Status != "filled" {
					break
				}
			}
		}
		if err != nil {
			outcome.Status, outcome.Detail = "uncertain", "Could not verify this field. Check it in the employer form."
			result.Fields = append(result.Fields, outcome)
			for _, remaining := range plan.Keys[i+1:] {
				result.Fields = append(result.Fields, browser.ContactOutcome{Key: remaining, Status: "manual", Detail: "Not attempted. Complete this field yourself."})
			}
			return result, err
		}
		result.Fields = append(result.Fields, outcome)
	}
	return result, nil
}
