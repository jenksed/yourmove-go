package chromium

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/jenksed/yourmove-go/internal/application/browser"
)

// Contact resolution stays local to this operation. It uses exact machine
// semantics plus compatible labels within an application form, never positions
// or question wording alone. Every write resolves again in the current DOM.
const contactResolver = browser.ContactResolver

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
