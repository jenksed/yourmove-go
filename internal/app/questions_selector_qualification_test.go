//go:build qualification

package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
	"github.com/jenksed/yourmove-go/internal/storage"
)

// The controlled browser drives the rendered combobox and textarea. Go's
// saved answers provide a separate reference for the editor contents.
func TestQuestionsSelectorDiscardRecovery(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in controlled browser")
	}
	dir := t.TempDir()
	t.Setenv("YOURMOVE_DATA_DIR", dir)
	dbPath, resumeDir, err := storage.ApplicationStorage()
	if err != nil {
		t.Fatal(err)
	}
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	a := newTestApp(t, db, resumeDir)
	seedKan12Questions(t, a)
	a.previewApplication = func(context.Context, string) (schema.Result, error) {
		return schema.Result{}, errors.New("synthetic: no early preview in the Questions fixture")
	}
	server, ui, ctx, _ := shotServe(t, a)
	page := server.URL + "/jobs/1"
	shotWait(t, ctx, ui, page, `document.body.innerText.includes('Synthetic KAN-12 role')`)
	shotClick(t, ctx, ui, page, "Questions")
	const trigger = ".question-select-trigger"
	shotWait(t, ctx, ui, page, `document.querySelector('#question-position')?.innerText === 'Question 1 of 2' && !!document.querySelector('#writing-answer')`)
	pointer, err := ui.FixturePage(ctx, page)
	if err != nil {
		t.Fatal(err)
	}
	defer pointer.Close()

	// Coordinates come from the actual rendered control. All selection and
	// editing events below are delivered as browser input.
	click := func(locator string) {
		t.Helper()
		var point struct{ X, Y float64 }
		shotEval(t, ctx, ui, page, `(()=>{
			const element = `+locator+`;
			if (!element?.getClientRects().length) throw Error('Control is unavailable');
			element.scrollIntoView({block:'nearest', behavior:'instant'});
			const rect = element.getBoundingClientRect();
			const X = rect.x + rect.width / 2, Y = rect.y + rect.height / 2;
			if (!element.contains(document.elementFromPoint(X,Y))) throw Error('Control is obscured');
			return {X,Y};
		})()`, &point)
		for _, event := range []string{"mousePressed", "mouseReleased"} {
			if err := pointer.Call(ctx, "Input.dispatchMouseEvent", map[string]any{
				"type": event, "x": point.X, "y": point.Y, "button": "left", "clickCount": 1,
			}, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	find := func(selector, label string) string {
		return fmt.Sprintf(`Array.from(document.querySelectorAll(%q)).find(e=>e.innerText.trim()===%q)`, selector, label)
	}

	type savedAnswer struct {
		Key, Label, Value string
		Revision          int
	}
	const aLabel, bLabel = "Describe a challenging technical problem", "Why do you want to join Fixtureworks?"
	saved := func(label string) savedAnswer {
		t.Helper()
		var answer savedAnswer
		err := a.db.QueryRow(`SELECT question_key,label,answer,revision FROM application_answers WHERE job_id=1 AND label LIKE ?`, label+"%").
			Scan(&answer.Key, &answer.Label, &answer.Value, &answer.Revision)
		if err != nil {
			t.Fatal(err)
		}
		return answer
	}
	// A is the second option, so selecting it proves an actual change from
	// the initial B selection. The fixture supplies distinct saved texts.
	b, questionA := saved(bLabel), saved(aLabel)
	if questionA.Key == b.Key || questionA.Value == b.Value || questionA.Value == "" || b.Value == "" || questionA.Revision != 1 || b.Revision != 1 {
		t.Fatalf("invalid synthetic answers: A=%+v B=%+v", questionA, b)
	}
	t.Logf("workspace=%s page=%s A=%+v B=%+v", dir, page, questionA, b)
	selectQuestion := func(answer savedAnswer) {
		t.Helper()
		click(fmt.Sprintf(`document.querySelector(%q)`, trigger))
		item := find(`[role="option"] .question-select-label`, answer.Label)
		shotWait(t, ctx, ui, page, item+`?.getClientRects().length > 0`)
		t.Logf("clicked selector option %q", answer.Label)
		click(item)
		shotWait(t, ctx, ui, page, `document.querySelector('.question-select-trigger')?.getAttribute('aria-expanded') === 'false'`)
	}

	type rendered struct {
		Position, Selection, Heading, Value, Status, Feedback string
		Discard                                               bool
	}
	check := func(step string, want rendered) {
		t.Helper()
		var got rendered
		shotEval(t, ctx, ui, page, `(()=>{
			const text = selector => {
				const element = document.querySelector(selector);
				return element?.getClientRects().length ? element.innerText.trim() : '';
			};
			return {
				position:text('#question-position'), selection:text('#question-select-value'),
				heading:text('.writing-editor h3'), value:document.querySelector('#writing-answer')?.value,
				status:text('.writing-editor h3 + p'), feedback:text('.question-dirty-note [role="alert"]'),
				discard:!!document.querySelector('.question-dirty-note button:last-child')?.getClientRects().length
			};
		})()`, &got)
		t.Logf("%s UI=%+v", step, got)
		if got != want {
			t.Errorf("%s: want %+v, got %+v", step, want, got)
		}
		if gotA, gotB := saved(aLabel), saved(bLabel); gotA != questionA || gotB != b {
			t.Errorf("%s altered saved answers: A=%+v B=%+v", step, gotA, gotB)
		}
		if t.Failed() {
			t.FailNow()
		}
	}
	const savedStatus = "Answer saved locally. It has not been filled on the employer website. · Saved revision 1 · Optional"
	clean := func(position string, answer savedAnswer) rendered {
		return rendered{Position: position, Selection: answer.Label, Heading: answer.Label, Value: answer.Value, Status: savedStatus}
	}
	cleanB, cleanA := clean("Question 1 of 2", b), clean("Question 2 of 2", questionA)
	check("initial B", cleanB)
	selectQuestion(questionA)
	shotWait(t, ctx, ui, page, fmt.Sprintf(`document.querySelector('#writing-answer')?.value === %q`, questionA.Value))
	check("clean selection A", cleanA)

	click(`document.querySelector('#writing-answer')`)
	const draft = "KAN-20 synthetic unsaved edit."
	// Replace the fixture text via keyboard selection and typed input.
	for _, event := range []string{"keyDown", "keyUp"} {
		if err := pointer.Call(ctx, "Input.dispatchKeyEvent", map[string]any{
			"type": event, "key": "a", "code": "KeyA", "windowsVirtualKeyCode": 65, "commands": []string{"selectAll"},
		}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := pointer.Call(ctx, "Input.insertText", map[string]any{"text": draft}, nil); err != nil {
		t.Fatal(err)
	}
	shotWait(t, ctx, ui, page, `!!document.querySelector('.question-dirty-note [role="alert"]')`)
	dirtyA := cleanA
	dirtyA.Value, dirtyA.Discard = draft, true
	dirtyA.Feedback = "Unsaved edits on this question — save or discard them to move to another question."
	check("dirty A before switch", dirtyA)

	selectQuestion(b)
	dirtyA.Feedback = "Unsaved edits on this question. Save or discard them to move to another question."
	check("refused B selection", dirtyA)
	click(find(".question-dirty-note button", "Discard edits"))
	check("discard restored A", cleanA)

	selectQuestion(b)
	shotWait(t, ctx, ui, page, fmt.Sprintf(`document.querySelector('#writing-answer')?.value === %q`, b.Value))
	check("clean selection B", cleanB)
}
