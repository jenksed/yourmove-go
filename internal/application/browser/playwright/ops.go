package playwright

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser"
)

// asFunction wraps an immediately invoked page expression (such as the
// shared read-only inspection script) as a nullary function source for the
// bridge, which only invokes function sources. Trimming the trailing
// semicolon keeps the parenthesized body a valid expression.
func asFunction(iife string) string {
	return "()=>{return (" + strings.TrimRight(strings.TrimSpace(iife), ";") + ")}"
}

// evaluate runs one authorized page function and decodes its JSON result.
func (s *Session) evaluate(ctx context.Context, expression string, args []any, out any) error {
	var raw json.RawMessage
	err := s.call(ctx, "evaluate", map[string]any{"expression": expression, "args": args}, &raw)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if len(raw) == 0 || string(raw) == "null" {
		return errors.New("browser page did not return an inspection result")
	}
	return json.Unmarshal(raw, out)
}

// lockContext serializes live observation and mutation without letting
// callers wait past cancellation.
func (s *Session) lockContext(ctx context.Context) error {
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for !s.opMu.TryLock() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
	if err := ctx.Err(); err != nil {
		s.opMu.Unlock()
		return err
	}
	return nil
}

// Observe captures the read-only projection of the current employer page.
func (s *Session) Observe(ctx context.Context) (browser.Observation, error) {
	c, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	if err := s.lockContext(c); err != nil {
		return browser.Observation{}, err
	}
	defer s.opMu.Unlock()
	if s.isClosed() {
		return browser.Observation{}, errors.New("inspection browser is closed")
	}
	// The shared script is an immediately invoked function expression; wrap
	// it so the bridge invokes a function source.
	expression := asFunction(browser.LocalizeScript(browser.ReadOnlyScript, s.label()))
	var snapshot browser.Observation
	if err := s.evaluate(c, expression, nil, &snapshot); err != nil {
		return browser.Observation{}, err
	}
	if snapshot.URL == "about:blank" {
		var current struct {
			URL string `json:"url"`
		}
		if err := s.call(c, "pageUrl", nil, &current); err != nil {
			return browser.Observation{}, err
		}
		if s.owns(current.URL) {
			return snapshot, nil
		}
	}
	if !s.owns(snapshot.URL) {
		return browser.Observation{}, fmt.Errorf("current browser page changed to another job or origin (%s); nothing saved", safeObservedAddress(snapshot.URL))
	}
	return snapshot, nil
}

// FillContacts transfers only the approved contact snapshot, preserving
// manual entries and never replaying a changed value.
func (s *Session) FillContacts(ctx context.Context, approved browser.ContactValues) (browser.ContactResult, error) {
	c, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	if err := s.lockContext(c); err != nil {
		return browser.ContactResult{}, err
	}
	defer s.opMu.Unlock()
	if s.isClosed() {
		return browser.ContactResult{}, errors.New("browser session closed")
	}
	origin, base, err := s.contactScope()
	if err != nil {
		return browser.ContactResult{}, err
	}
	scope := []any{origin, base}
	install := `(origin,base)=>{window.__ym_contact=(` + browser.LocalizeScript(browser.ContactResolver, s.label()) + `)(origin,base);return {installed:true};}`
	var installed struct {
		Installed bool `json:"installed"`
	}
	if err := s.evaluate(c, install, scope, &installed); err != nil || !installed.Installed {
		return browser.ContactResult{}, errors.New("contact controls are unavailable")
	}
	call := func(method, key, value string, out any) error {
		expr := `(method,key,value)=>{const r=window.__ym_contact;if(!r)throw Error('application changed');return r[method](key,value);}`
		return s.evaluate(c, expr, []any{method, key, value}, out)
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

// FillAnswer transfers one approved answer after revalidating its live identity.
func (s *Session) FillAnswer(ctx context.Context, answer browser.AnswerValue) (browser.AnswerOutcome, error) {
	out := browser.AnswerOutcome{Label: answer.Field.Label}
	c, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	if err := s.lockContext(c); err != nil {
		return out, err
	}
	defer s.opMu.Unlock()
	if s.isClosed() {
		return out, errors.New("browser session closed")
	}
	if answer.Field.Sensitive || answer.Value == "" || len(answer.Field.ID) < len("browser:") || answer.Field.ID[:len("browser:")] != "browser:" {
		out.Status = "manual"
		out.Detail = "This question needs manual entry."
		return out, nil
	}
	origin, base, err := s.contactScope()
	if err != nil {
		return out, err
	}
	script := s.answerScript()
	args, _ := json.Marshal([]any{origin, base, answer})
	var decoded []any
	if err := json.Unmarshal(args, &decoded); err != nil {
		return out, err
	}
	var outcome browser.AnswerOutcome
	if err := s.evaluate(c, script, decoded, &outcome); err != nil {
		return out, errors.New("answer transfer could not be verified; review this field in the managed browser")
	}
	out.Status = outcome.Status
	out.Detail = outcome.Detail
	return out, nil
}

func (s *Session) answerScript() string {
	script := browser.LocalizeScript(browser.AnswerOperation, s.label())
	// Inline the read-only projection exactly as the CDP driver does.
	if idx := strings.Index(script, "OBSERVATION"); idx >= 0 {
		script = script[:idx] + "(" + strings.TrimRight(strings.TrimSpace(browser.ReadOnlyScript), ";") + ")" + script[idx+len("OBSERVATION"):]
	}
	return script
}

// AttachResume selects the approved resume file into the identified employer input.
func (s *Session) AttachResume(ctx context.Context, path string) (browser.AttachmentResult, error) {
	return s.attachDocument(ctx, path, false, "")
}

// AttachCoverLetter selects the approved cover-letter PDF after verifying its digest.
func (s *Session) AttachCoverLetter(ctx context.Context, path, digest string) (browser.AttachmentResult, error) {
	return s.attachDocument(ctx, path, true, digest)
}

// FillCoverLetter transfers approved cover-letter text into the labeled control.
func (s *Session) FillCoverLetter(ctx context.Context, body string) (browser.AttachmentResult, error) {
	c, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	if err := s.lockContext(c); err != nil {
		return browser.AttachmentResult{}, err
	}
	defer s.opMu.Unlock()
	if s.isClosed() {
		return browser.AttachmentResult{}, errors.New("browser session closed")
	}
	origin, base, err := s.contactScope()
	if err != nil {
		return browser.AttachmentResult{}, err
	}
	var result browser.AttachmentResult
	if err := s.evaluate(c, browser.LocalizeScript(browser.CoverLetterOperation, s.label()), []any{origin, base, body}, &result); err != nil {
		return browser.AttachmentResult{}, errors.New("cover-letter transfer could not be verified; inspect the employer form before continuing")
	}
	return result, nil
}

func (s *Session) attachDocument(ctx context.Context, path string, cover bool, digest string) (browser.AttachmentResult, error) {
	c, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	if err := s.lockContext(c); err != nil {
		return browser.AttachmentResult{}, err
	}
	defer s.opMu.Unlock()
	if s.isClosed() {
		return browser.AttachmentResult{}, errors.New("browser session closed")
	}
	if !filepath.IsAbs(path) {
		return browser.AttachmentResult{}, errors.New("resume file path must be absolute")
	}
	survey := asFunction(browser.LocalizeScript(browser.FileInputSurvey, s.label()))
	var inputs []browser.FileInput
	if err := s.evaluate(c, survey, nil, &inputs); err != nil {
		return browser.AttachmentResult{}, &browser.PreflightError{Cause: err}
	}
	idx, err := browser.ChooseDocumentInput(inputs, filepath.Ext(path), cover)
	if err != nil {
		return browser.AttachmentResult{}, err
	}
	// Reinspect immediately before mutation: rerenders can replace or reorder
	// file inputs between the initial survey and selection.
	var current []browser.FileInput
	if err := s.evaluate(c, survey, nil, &current); err != nil {
		return browser.AttachmentResult{}, &browser.PreflightError{Cause: err}
	}
	if !browser.SameFileInputs(inputs, current) {
		return browser.AttachmentResult{}, &browser.PreflightError{Cause: errors.New("resume controls changed before file selection")}
	}
	var href struct {
		Href string `json:"href"`
	}
	if err := s.evaluate(c, "()=>({href:location.href})", nil, &href); err != nil || !s.owns(href.Href) {
		return browser.AttachmentResult{}, errors.New("application changed before document selection; review the destination")
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return browser.AttachmentResult{}, errors.New("prepared document is unavailable")
	}
	if cover && fmt.Sprintf("%x", sha256.Sum256(data)) != digest {
		return browser.AttachmentResult{}, errors.New("prepared cover letter changed; review it again")
	}
	// Tag the exact surveyed input so file selection cannot drift to a
	// rerendered neighbor. Any error after setInputFiles may be an ambiguous
	// employer upload and is never retried automatically.
	var tagged struct {
		Token string `json:"token"`
	}
	if err := s.call(c, "tag", map[string]any{"index": idx}, &tagged); err != nil {
		return browser.AttachmentResult{}, &browser.PreflightError{Cause: err}
	}
	if tagged.Token == "" {
		return browser.AttachmentResult{}, &browser.PreflightError{Cause: errors.New("resume input disappeared before selection")}
	}
	if err := s.call(c, "setFiles", map[string]any{"token": tagged.Token, "paths": []string{path}}, nil); err != nil {
		return browser.AttachmentResult{}, fmt.Errorf("file selection outcome unverified: %w", err)
	}
	out := browser.AttachmentResult{Selected: true, Detail: "File selected in employer browser; server acceptance not confirmed"}
	verify := coverVerify(idx, cover)
	var confirmed struct {
		Name   string `json:"name"`
		Size   int64  `json:"size"`
		Digest string `json:"digest"`
		URL    string `json:"url"`
	}
	if err := s.evaluate(c, asFunction(verify), nil, &confirmed); err == nil && confirmed.Name == filepath.Base(path) && confirmed.Size == int64(len(data)) && s.owns(confirmed.URL) && (!cover || confirmed.Digest == digest) {
		out.Verified = true
		out.Detail = "Exact named file present in employer input; employer upload acceptance still unverified"
	}
	return out, nil
}

func coverVerify(idx int, cover bool) string {
	// Raw immediately invoked expressions; the caller wraps them with
	// asFunction so the bridge invokes a function source exactly once.
	if cover {
		return fmt.Sprintf(`(async()=>{const e=document.querySelectorAll('input[type="file"]')[%d];const f=e?.files?.[0];if(!f)return {};const hash=await crypto.subtle.digest('SHA-256',await f.arrayBuffer());return {url:location.href,name:f.name,size:f.size,digest:[...new Uint8Array(hash)].map(x=>x.toString(16).padStart(2,'0')).join('')}})()`, idx)
	}
	return fmt.Sprintf(`(()=>{const f=document.querySelectorAll('input[type="file"]')[%d]?.files?.[0];return {url:location.href,name:f?.name||"",size:f?.size||0}})()`, idx)
}
