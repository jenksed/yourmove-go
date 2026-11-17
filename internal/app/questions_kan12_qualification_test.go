//go:build qualification

package app

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// KAN-12 Approach 1 functional qualification for the Questions desk. Uses the
// controlled headless-Chromium fixture with the real React UI and real Go
// handlers against synthetic data only. This is functional evidence, not a
// substitute for native Wails acceptance: selectors here are DOM queries,
// and layout is the fixture viewport, not the macOS window.
func seedKan12Questions(t *testing.T, a *App) {
	t.Helper()
	// The recognized Ashby posting identity keeps the seeded browser
	// inspection on the live, editable path. Early preview discovery is
	// neutralized by the caller so the desk cannot flip to read-only preview.
	const posting = testAshbyPosting
	if err := a.saveJob(Job{URL: posting, Host: "jobs.ashbyhq.com", Title: "Synthetic KAN-12 role", Company: "Fixtureworks"}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveCandidateProfile(CandidateProfile{
		FirstName: "Alex", LastName: "Fixture", Email: "alex@example.test", Approved: true,
	}); err != nil {
		t.Fatal(err)
	}
	fields := []schema.Field{
		{ID: "browser:why", Label: "Why do you want to join Fixtureworks? This is a deliberately long employer question to exercise wrapping in the selector, the editor heading, and narrow layouts.", Kind: "textarea"},
		{ID: "browser:challenge", Label: "Describe a challenging technical problem you solved recently and what you learned from the experience in detail.", Kind: "textarea"},
	}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: posting + "/application", Fields: fields}); err != nil {
		t.Fatal(err)
	}
	for i, answer := range []string{
		"I want to join Fixtureworks because the team owns reliability end to end, which matches my on-call experience.",
		"I debugged a midnight queue backlog by tracing one poison message, then added a dead-letter queue and alerting.",
	} {
		if _, err := a.db.Exec(`INSERT INTO application_answers(job_id,destination,provider,question_key,label,answer,revision) VALUES(1,?, 'ashby',?,?,?,1)`,
			posting, questionKey(posting, "ashby", fields[i]), fields[i].Label, answer); err != nil {
			t.Fatal(err)
		}
	}
	libraries := []struct {
		question, answer, employer, title string
	}{
		{
			"Why do you want to join a new engineering team?",
			"I look for teams that own reliability end to end, review incidents openly, and keep on-call sustainable for everyone involved.",
			"Acme Robotics", "Backend Engineer",
		},
		{
			"Describe a hard production incident you resolved",
			"A poison message stalled the queue at midnight; I quarantined it, replayed the backlog, and added dead-letter alerting afterward.",
			"", "",
		},
	}
	for i, entry := range libraries {
		if _, err := a.db.Exec(`INSERT INTO answer_library(original_question,answer,source_employer,source_job_title,source_host,
			source_job_id,source_destination,source_provider,source_question_key,source_revision,source_inspection_id,
			source_inspection_fingerprint,source_field_id,source_field_kind,source_kind,question_terms,answer_terms,
			search_index_version,created_at,updated_at,revision)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1)`,
			entry.question, entry.answer, entry.employer, entry.title, "jobs.ashbyhq.com",
			900+i, posting, "ashby",
			"kan12-question", 1, 900+i, "synthetic", "browser:library", "textarea", "live",
			answerLibraryIndexTerms(entry.question), answerLibraryIndexTerms(entry.answer), answerLibrarySearchIndexVersion,
			answerLibraryTimestamp(), answerLibraryTimestamp()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestKan12QuestionsDirtyGuardAndPreviewContext(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in controlled browser")
	}
	a, _ := testWorkspace(t)
	seedKan12Questions(t, a)
	// Keep the desk on the seeded live inspection: early preview discovery
	// stays failed without launching a browser or touching the network.
	a.previewApplication = func(context.Context, string) (schema.Result, error) {
		return schema.Result{}, errors.New("synthetic: no early preview in the KAN-12 fixture")
	}
	app, ui, ctx, _ := shotServe(t, a)
	page := app.URL + "/jobs/1"

	// Reach the Questions desk through the workspace tabs.
	shotWait(t, ctx, ui, page, `document.body.innerText.includes('Synthetic KAN-12 role')`)
	shotEval(t, ctx, ui, page, `Array.from(document.querySelectorAll('button, a')).find(x=>x.textContent.trim()==="Questions" && x.offsetParent!==null).click()`, nil)
	shotWait(t, ctx, ui, page, `document.body.innerText.includes('Question 1 of 2')`)

	// Dirty the first answer through the real editor event path, using the
	// native value setter so React observes the keystroke-equivalent input.
	var typeResult string
	shotEval(t, ctx, ui, page, `(()=>{const area=document.querySelector('#writing-answer');if(!area) return 'NO_AREA';if(area.readOnly||area.disabled) return 'READONLY';const proto=area instanceof HTMLTextAreaElement?HTMLTextAreaElement.prototype:HTMLInputElement.prototype;Object.getOwnPropertyDescriptor(proto,'value').set.call(area,area.value+' Extra unsaved sentence.');area.dispatchEvent(new Event('input',{bubbles:true}));return 'OK:'+area.value.slice(-30)})()`, &typeResult)
	if !strings.HasPrefix(typeResult, "OK:") {
		t.Fatalf("could not type into the answer editor: %q", typeResult)
	}
	shotWait(t, ctx, ui, page, `document.querySelector('.question-dirty-note')?.innerText.includes('Unsaved edits')`)

	var draftBefore string
	shotEval(t, ctx, ui, page, `document.querySelector('#writing-answer').value`, &draftBefore)
	if !strings.Contains(draftBefore, "Extra unsaved sentence.") {
		t.Fatalf("editor draft missing typed text: %q", draftBefore)
	}

	// Dirty Next must not save, discard, or navigate: the same question,
	// draft, and nearby guidance stay until the applicant acts. The blocked
	// attempt is announced next to the navigator (new wording), not as a
	// save error in the editor area (previous behavior).
	shotClick(t, ctx, ui, page, "Next →")
	shotWait(t, ctx, ui, page, `document.querySelector('.question-dirty-note')?.innerText.includes('Unsaved edits on this question. Save or discard')`)
	var navText, bodyText string
	shotEval(t, ctx, ui, page, `document.querySelector('.question-dirty-note').innerText`, &navText)
	shotEval(t, ctx, ui, page, `document.body.innerText`, &bodyText)
	if !strings.Contains(navText, "Unsaved edits on this question. Save or discard them to move to another question.") {
		t.Fatalf("blocked navigation lacks nearby guidance: %q", navText)
	}
	if strings.Contains(bodyText, "before opening another question") {
		t.Fatalf("blocked navigation still reports through the editor save error")
	}
	var position, draftAfter string
	shotEval(t, ctx, ui, page, `document.querySelector('#question-position').textContent`, &position)
	shotEval(t, ctx, ui, page, `document.querySelector('#writing-answer').value`, &draftAfter)
	if position != "Question 1 of 2" {
		t.Fatalf("dirty Next navigated away: %q", position)
	}
	if draftAfter != draftBefore {
		t.Fatalf("dirty Next changed the draft")
	}

	// The selector shares the same guard and the same nearby guidance.
	shotEval(t, ctx, ui, page, `document.querySelector('.question-select-trigger').click()`, nil)
	shotWait(t, ctx, ui, page, `document.querySelector('.question-select-item') !== null`)
	shotEval(t, ctx, ui, page, `Array.from(document.querySelectorAll('.question-select-item'))[1].click()`, nil)
	shotWait(t, ctx, ui, page, `document.querySelector('#question-position').textContent === 'Question 1 of 2' && document.querySelector('.question-dirty-note')?.innerText.includes('Save or discard them to move')`)

	// Blocked navigation must not touch the durable answer or its revision.
	var revision int
	var saved string
	if err := a.db.QueryRow(`SELECT revision,answer FROM application_answers WHERE job_id=1 AND label LIKE 'Why do you want%'`).Scan(&revision, &saved); err != nil {
		t.Fatal(err)
	}
	if revision != 1 || strings.Contains(saved, "Extra unsaved sentence.") {
		t.Fatalf("blocked navigation persisted draft: revision=%d answer=%q", revision, saved)
	}

	// Explicit save, then clean navigation works and the revision advances.
	shotClick(t, ctx, ui, page, "Save for this application only")
	shotWait(t, ctx, ui, page, `document.body.innerText.includes('Saved revision 2')`)
	shotClick(t, ctx, ui, page, "Next →")
	shotWait(t, ctx, ui, page, `document.body.innerText.includes('Question 2 of 2')`)

	// Returning must restore the explicitly saved answer.
	shotClick(t, ctx, ui, page, "← Previous")
	shotWait(t, ctx, ui, page, `document.querySelector('#writing-answer')?.value.includes('Extra unsaved sentence.')`)
	shotEval(t, ctx, ui, page, `document.querySelector('#writing-answer').value`, &draftAfter)
	if draftAfter != draftBefore {
		t.Fatal("return lost saved answer")
	}
	shotClick(t, ctx, ui, page, "Next →")
	shotWait(t, ctx, ui, page, `document.body.innerText.includes('Question 2 of 2')`)

	// Previous Writing keeps working: browse every saved example, preview the
	// Acme entry with its full employer/role source context, and reuse it only
	// through explicit Use as Draft consent.
	shotClick(t, ctx, ui, page, "Browse all")
	shotWait(t, ctx, ui, page, `document.body.innerText.includes('Acme Robotics')`)
	shotEval(t, ctx, ui, page, `Array.from(document.querySelectorAll('.library-browse .writing-suggestions li')).find(li=>li.innerText.includes('Acme Robotics')).querySelector('button').click()`, nil)
	shotWait(t, ctx, ui, page, `document.querySelector('.writing-preview')?.innerText.includes('Saved from Acme Robotics')`)
	var preview string
	shotEval(t, ctx, ui, page, `document.querySelector('.writing-preview').innerText`, &preview)
	if !strings.Contains(preview, "Backend Engineer") || !strings.Contains(preview, "revision 1") {
		t.Fatalf("full preview lost source context: %q", preview)
	}
	shotClick(t, ctx, ui, page, "Use as starting draft")
	shotWait(t, ctx, ui, page, `document.body.innerText.includes('Review employer-specific wording')`)
	var draftD1 string
	shotEval(t, ctx, ui, page, `document.querySelector('#writing-answer').value`, &draftD1)

	// A second example on a dirty draft asks for Replace/Keep consent instead
	// of overwriting, and entries without source context show no context line.
	shotEval(t, ctx, ui, page, `Array.from(document.querySelectorAll('.library-browse .writing-suggestions li')).find(li=>!li.innerText.includes('Acme Robotics')).querySelector('button').click()`, nil)
	shotWait(t, ctx, ui, page, `document.querySelector('.writing-preview')?.innerText.includes('poison message stalled')`)
	shotEval(t, ctx, ui, page, `document.querySelector('.writing-preview').innerText`, &preview)
	if strings.Contains(preview, "Saved from") {
		t.Fatalf("context-free preview shows a source line: %q", preview)
	}
	shotClick(t, ctx, ui, page, "Use as starting draft")
	shotWait(t, ctx, ui, page, `document.body.innerText.includes('Replace your unsaved edits?')`)
	shotClick(t, ctx, ui, page, "Keep my draft")
	shotWait(t, ctx, ui, page, `!document.body.innerText.includes('Replace your unsaved edits?')`)
	var kept string
	shotEval(t, ctx, ui, page, `document.querySelector('#writing-answer').value`, &kept)
	if kept != draftD1 {
		t.Fatalf("Keep my draft did not preserve the editor text: %q", kept)
	}

	// Dirty Previous uses the same guard and preserves the current draft.
	shotClick(t, ctx, ui, page, "← Previous")
	shotWait(t, ctx, ui, page, `document.querySelector('.question-dirty-note')?.innerText.includes('Save or discard them to move')`)
	shotEval(t, ctx, ui, page, `document.querySelector('#question-position').textContent`, &position)
	shotEval(t, ctx, ui, page, `document.querySelector('#writing-answer').value`, &kept)
	if position != "Question 2 of 2" || kept != draftD1 {
		t.Fatal("dirty Previous lost position or draft")
	}

	shotClick(t, ctx, ui, page, "Use as starting draft")
	shotWait(t, ctx, ui, page, `document.body.innerText.includes('Replace your unsaved edits?')`)
	shotClick(t, ctx, ui, page, "Replace editor text")
	shotWait(t, ctx, ui, page, `document.querySelector('#writing-answer')?.value.startsWith('A poison message stalled')`)
	if err := a.db.QueryRow(`SELECT revision FROM application_answers WHERE job_id=1 AND label LIKE 'Describe a challenging%'`).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if revision != 1 {
		t.Fatalf("Replace unexpectedly saved revision %d", revision)
	}

	shotCaptureAt(t, ctx, ui, page, 1280, 900, ".question-navigator", "dark", "kan12-dirty-desktop-dark")
	shotCaptureAt(t, ctx, ui, page, 700, 550, ".question-navigator", "dark", "kan12-dirty-narrow-dark")
}
