//go:build qualification

package app

// Compact résumé chooser over a 13-document library: matched, ambiguous,
// choose-primary and no-family suggestions stay advisory; explicit assignment
// through the established endpoint is the only thing that changes selection.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
)

func TestResumeChooserTwelveResumeLibrary(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in controlled browser")
	}
	a, _ := testWorkspace(t)
	a.resources.frontendDist = os.DirFS("../..")
	a.resources.frontendIndex = loadFrontendIndex(a.resources.frontendDist)
	if len(a.resources.frontendIndex) == 0 {
		t.Fatal("build frontend first")
	}
	for _, family := range []RoleFamily{
		{Name: "Support", TitleTerms: "support engineer"},
		{Name: "Linux", TitleTerms: "linux administrator"},
		{Name: "Operations", TitleTerms: "data entry"},
		{Name: "Admin", TitleTerms: "desk entry"},
		{Name: "Success", TitleTerms: "customer success manager"},
	} {
		if err := a.saveFamily(family); err != nil {
			t.Fatal(err)
		}
	}
	families, err := a.allFamilies()
	if err != nil || len(families) != 5 {
		t.Fatalf("families=%+v err=%v", families, err)
	}
	familyID := map[string]int64{}
	for _, f := range families {
		familyID[f.Name] = f.ID
	}
	importNamed := func(name, tag string) Resume {
		t.Helper()
		// Identical bytes would register as the same document; each fixture
		// resume carries a unique trailer after the shared PDF body.
		body := append(bytes.Clone(mustQualificationPDF(t)), []byte("\n% fixture "+tag)...)
		resume, err := a.importResume(name, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		return resume
	}
	// 13 documents: four Support (one primary), three Linux (one primary),
	// single Operations/Admin candidates, two Success resumes without a
	// primary, one unassigned, and one archived.
	support := []Resume{importNamed("Support_A.pdf", "sa"), importNamed("Support_B.pdf", "sb"), importNamed("Support_C.pdf", "sc"), importNamed("Support_D.pdf", "sd")}
	linux := []Resume{importNamed("Linux_A.pdf", "la"), importNamed("Linux_B.pdf", "lb"), importNamed("Linux_C.pdf", "lc")}
	ops := importNamed("Ops_Logger.pdf", "ops")
	admin := importNamed("Admin_Filer.pdf", "adm")
	success := []Resume{importNamed("Success_A.pdf", "sua"), importNamed("Success_B.pdf", "sub")}
	lone := importNamed("Generalist.pdf", "gen")
	stale := importNamed("Old_Draft.pdf", "old")
	for i, resume := range support {
		if err := a.assignResumeFamily(resume.ID, familyID["Support"], i == 1); err != nil {
			t.Fatal(err)
		}
	}
	for i, resume := range linux {
		if err := a.assignResumeFamily(resume.ID, familyID["Linux"], i == 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.assignResumeFamily(ops.ID, familyID["Operations"], false); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResumeFamily(admin.ID, familyID["Admin"], false); err != nil {
		t.Fatal(err)
	}
	for _, resume := range success {
		if err := a.assignResumeFamily(resume.ID, familyID["Success"], false); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.setResumeArchived(stale.ID, true); err != nil {
		t.Fatal(err)
	}
	jobs := []Job{
		{URL: "https://jobs.ashbyhq.com/render/e1f2d6a2-b9e2-4f16-b863-3ca228c83e75", Host: "jobs.ashbyhq.com", Title: "Support Engineer"},
		{URL: "https://example.com/data-clerk", Host: "example.com", Title: "Data entry desk entry clerk"},
		{URL: "https://example.com/astronaut", Host: "example.com", Title: "Astronaut"},
		{URL: "https://example.com/success", Host: "example.com", Title: "Customer Success Manager"},
	}
	for _, job := range jobs {
		if err := a.saveJob(job); err != nil {
			t.Fatal(err)
		}
	}
	// Job 1 starts with an explicit non-primary selection so the suggestion
	// stays advisory until the applicant acts.
	if err := a.assignResume(1, support[0].ID); err != nil {
		t.Fatal(err)
	}

	router := http.NewServeMux()
	router.HandleFunc("GET /application", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/jobs/1", http.StatusSeeOther) })
	router.Handle("/", newMux(a))
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	page := server.URL + "/jobs/1"
	ui, err := chromium.LaunchFixture(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer ui.Close()
	eval := func(expression string, out any) error {
		var reply struct {
			Result struct {
				Value json.RawMessage `json:"value"`
			} `json:"result"`
			Exception json.RawMessage `json:"exceptionDetails"`
		}
		if err := ui.FixtureCall(ctx, page, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true, "awaitPromise": true}, &reply); err != nil {
			return err
		}
		if len(reply.Exception) > 0 {
			return fmt.Errorf("JS %s", reply.Exception)
		}
		if out != nil {
			return json.Unmarshal(reply.Result.Value, out)
		}
		return nil
	}
	run := func(expression string) {
		t.Helper()
		if err := eval(expression, nil); err != nil {
			t.Fatal(err)
		}
	}
	wait := func(expression string) {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			var ok bool
			if eval(expression, &ok) == nil && ok {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		var body string
		_ = eval(`document.body.innerText`, &body)
		t.Fatalf("timeout %s\n%s", expression, body)
	}
	clickButton := func(label string) {
		t.Helper()
		run(fmt.Sprintf(`Array.from(document.querySelectorAll('button')).find(x=>x.textContent.trim()===%q&&x.offsetParent!==null)?.click()`, label))
	}
	// Clicks after a full-page load can land on a transient render, so every
	// consequential click retries until its visible effect lands.
	openChooser := func() {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			var open bool
			if eval(`document.querySelector('.resume-chooser')?.open===true`, &open) == nil && open {
				return
			}
			_ = eval(`Array.from(document.querySelectorAll('button')).find(x=>x.textContent.trim()==='Change résumé'&&x.offsetParent!==null)?.click()`, nil)
			time.Sleep(200 * time.Millisecond)
		}
		var probe string
		_ = eval(`(()=>{const d=document.querySelector('.resume-chooser');const b=Array.from(document.querySelectorAll('button')).find(x=>x.textContent.trim()==='Change résumé');const chain=[];let e=b;while(e&&chain.length<6){const cs=getComputedStyle(e);chain.push(e.tagName+'.'+e.className+' display='+cs.display);e=e.parentElement}return 'dialog:'+(d?'open='+d.open:'MISSING')+' btn:'+(b?'found visible='+(b.offsetParent!==null)+' disabled='+b.disabled:'MISSING')+' chain='+chain.join(' < ')})()`, &probe)
		t.Fatalf("chooser did not open (%s)", probe)
	}
	chooseInChooser := func(fragment string) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			var open bool
			if eval(`document.querySelector('.resume-chooser')?.open===true`, &open) == nil && !open {
				return
			}
			_ = eval(fmt.Sprintf(`Array.from(document.querySelectorAll('.resume-chooser input[name=chooser-resume]')).find(x=>x.parentElement.innerText.includes(%q))?.click()`, fragment), nil)
			time.Sleep(300 * time.Millisecond)
		}
		var body string
		_ = eval(`document.body.innerText`, &body)
		t.Fatalf("chooser choice %q did not save\n%s", fragment, body)
	}
	acceptSuggestion := func(name string) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			var summary string
			if eval(`document.querySelector('.resume-current')?.innerText ?? ''`, &summary) == nil && strings.Contains(summary, name) {
				return
			}
			_ = eval(`Array.from(document.querySelectorAll('.resume-suggestion button')).find(x=>x.textContent.trim()==='Use suggested resume')?.click()`, nil)
			time.Sleep(300 * time.Millisecond)
		}
		t.Fatalf("suggestion %q not accepted", name)
	}

	wait(`document.body.innerText.includes('Support Engineer')`)
	clickButton("Documents")
	// Compact by default: one summary row plus a chooser action, no
	// unbounded radio list.
	wait(`Array.from(document.querySelectorAll('button')).some(x=>x.textContent.trim()==='Change résumé')`)
	var radios int
	if err := eval(`document.querySelectorAll('input[name=workspace-resume]').length`, &radios); err != nil || radios != 0 {
		t.Fatalf("unbounded selector still rendered: radios=%d err=%v", radios, err)
	}
	var summary string
	if err := eval(`document.querySelector('.resume-current')?.innerText ?? ''`, &summary); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Support_A.pdf", "Version 1", "Support"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("selected summary %q missing %q", summary, want)
		}
	}
	// The matched suggestion names the primary without applying it.
	wait(`document.querySelector('.resume-suggestion')?.innerText.includes('Support_B.pdf')`)
	var suggestionText string
	if err := eval(`document.querySelector('.resume-suggestion')?.innerText ?? ''`, &suggestionText); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(suggestionText, "support engineer") {
		t.Fatalf("suggestion leaks title-match diagnostics: %q", suggestionText)
	}

	// One click accepts the suggestion through the established endpoint.
	acceptSuggestion("Support_B.pdf")
	var using bool
	if err := eval(`document.body.innerText.includes('Using the suggested résumé for Support')`, &using); err != nil || !using {
		t.Fatalf("accepted suggestion not acknowledged: %v %v", using, err)
	}

	// The chooser stays bounded with search and family filtering.
	openChooser()
	var focused string
	if err := eval(`document.activeElement?.id ?? ''`, &focused); err != nil || focused != "resume-chooser-search" {
		t.Fatalf("search not focused on open: %q err=%v", focused, err)
	}
	var openRows int
	if err := eval(`document.querySelectorAll('.resume-chooser input[name=chooser-resume]').length`, &openRows); err != nil {
		t.Fatal(err)
	}
	// 12 selectable documents plus the clear-selection option.
	if openRows != 13 {
		t.Fatalf("chooser rows=%d want 13", openRows)
	}
	run(`(()=>{const e=document.querySelector('#resume-chooser-search');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(e,'linux');e.dispatchEvent(new Event('input',{bubbles:true}))})()`)
	wait(`document.querySelectorAll('.resume-chooser input[name=chooser-resume]').length===4`)
	run(`(()=>{const e=document.querySelector('#resume-chooser-family');e.value='none';e.dispatchEvent(new Event('change',{bubbles:true}))})()`)
	wait(`document.querySelectorAll('.resume-chooser input[name=chooser-resume]').length===1`)
	run(`(()=>{const e=document.querySelector('#resume-chooser-search');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(e,'');e.dispatchEvent(new Event('input',{bubbles:true}));const f=document.querySelector('#resume-chooser-family');f.value='all';f.dispatchEvent(new Event('change',{bubbles:true}))})()`)
	wait(`document.querySelectorAll('.resume-chooser input[name=chooser-resume]').length===13`)
	// Archived and primary states are labeled; the archived row stays out.
	var labels string
	if err := eval(`document.querySelector('.resume-chooser-list')?.innerText ?? ''`, &labels); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Primary", "OPERATIONS", "NO FAMILY"} {
		if !strings.Contains(labels, want) {
			t.Fatalf("chooser labels %q missing %q", labels, want)
		}
	}
	if strings.Contains(labels, "Old_Draft.pdf") {
		t.Fatalf("archived document offered in chooser")
	}
	// Keyboard: Escape closes the dialog without changing anything.
	escapeDeadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(escapeDeadline) {
		var open bool
		if eval(`document.querySelector('.resume-chooser')?.open===true`, &open) == nil && !open {
			break
		}
		if err := ui.FixtureCall(ctx, page, "Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": "Escape", "code": "Escape", "windowsVirtualKeyCode": 27}, nil); err != nil {
			t.Fatal(err)
		}
		if err := ui.FixtureCall(ctx, page, "Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "key": "Escape", "code": "Escape", "windowsVirtualKeyCode": 27}, nil); err != nil {
			t.Fatal(err)
		}
		time.Sleep(200 * time.Millisecond)
	}
	wait(`document.querySelector('.resume-chooser')?.open===false`)
	wait(`document.querySelector('.resume-current')?.innerText.includes('Support_B.pdf')`)

	// Manual choice inside the chooser overrides the suggestion.
	openChooser()
	chooseInChooser("Generalist.pdf")
	wait(`document.querySelector('.resume-current')?.innerText.includes('Generalist.pdf')`)
	if saved, err := a.jobByID(1); err != nil || saved.SelectedResumeID != lone.ID {
		t.Fatalf("manual choice not persisted: %+v %v", saved, err)
	}

	// Ambiguous, unmatched, and choose-primary jobs abstain with one quiet line.
	for _, tc := range []struct {
		job, want string
		quiet    bool
	}{
		{"/jobs/2", "Several families matched equally", false},
		{"/jobs/3", "", true},
		{"/jobs/4", "needs a primary", false},
	} {
		run(fmt.Sprintf(`location.href=%q`, server.URL+tc.job))
		page = server.URL + tc.job
		wait(`document.body.innerText.includes('Résumé')`)
		clickButton("Documents")
		wait(`document.querySelector('#prepare')?.innerText.includes('Change résumé')`)
		var note string
		if err := eval(`document.querySelector('#prepare')?.innerText ?? ''`, &note); err != nil {
			t.Fatal(err)
		}
		if tc.quiet {
			// No match and families exist elsewhere: no suggestion line at
			// all, only the persistent Role families management link.
			for _, banned := range []string{"Use suggested resume", "Suggested for", "Using the suggested", "needs a primary", "has no résumé yet", "matched equally", "Add a job title"} {
				if strings.Contains(note, banned) {
					t.Fatalf("job %s not quiet: found %q", tc.job, banned)
				}
			}
			var hasFamiliesLink bool
			if err := eval(`!!document.querySelector('#prepare a[href="/families"]')`, &hasFamiliesLink); err != nil || !hasFamiliesLink {
				t.Fatalf("job %s missing Role families link", tc.job)
			}
		} else {
			if !strings.Contains(note, tc.want) {
				t.Fatalf("job %s note %q missing %q", tc.job, note, tc.want)
			}
			if strings.Contains(note, "Use suggested resume") {
				t.Fatalf("job %s abstention broken by a suggestion button", tc.job)
			}
		}
		if err := eval(`document.querySelectorAll('input[name=workspace-resume]').length`, &radios); err != nil || radios != 0 {
			t.Fatalf("unbounded selector on %s", tc.job)
		}
	}
	// Job 3 has no families matching, yet families exist: the quiet path is
	// covered by job 2/4 abstention and the familyCount payload contract.
	var status string
	if err := eval(`(async()=>{const r=await fetch('/api/jobs/3/workspace');const j=await r.json();return j.suggestion.status+':'+j.familyCount})()`, &status); err != nil || status != "no-family:5" {
		t.Fatalf("no-family payload=%q err=%v", status, err)
	}

	// Returning to job 1 preserves the manual assignment; Apply agrees.
	run(fmt.Sprintf(`location.href=%q`, server.URL+"/jobs/1"))
	page = server.URL + "/jobs/1"
	wait(`document.body.innerText.includes('Generalist.pdf')`)
	clickButton("Documents")
	wait(`document.querySelector('#prepare .resume-current')?.innerText.includes('Generalist.pdf')`)
	var continuation struct {
		Resume *struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"resume"`
	}
	if err := eval(`(async()=>{const r=await fetch('/api/jobs/1/continuation');return await r.json()})()`, &continuation); err != nil {
		t.Fatal(err)
	}
	if continuation.Resume == nil || continuation.Resume.ID != lone.ID {
		t.Fatalf("apply identity diverged: %+v", continuation.Resume)
	}

	// Evidence: closed and open chooser at desktop and compact widths.
	// A fixed directory keeps the rendered evidence retrievable after the run.
	evidenceDir := "/tmp/chooser-evidence"
	if err := os.MkdirAll(evidenceDir, 0700); err != nil {
		t.Fatal(err)
	}
	shoot := func(name string, width, height int, theme string) {
		t.Helper()
		encoded, err := ui.FixtureScreenshotViewport(ctx, page, width, height, "#prepare", theme)
		if err != nil {
			t.Fatal(err)
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(evidenceDir, name)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		t.Log("screenshot", path)
	}
	shoot("chooser-compact-1280x900-light.png", 1280, 900, "light")
	shoot("chooser-compact-1280x900-dark.png", 1280, 900, "dark")
	shoot("chooser-compact-700x550-light.png", 700, 550, "light")
	shoot("chooser-compact-700x550-dark.png", 700, 550, "dark")
	openChooser()
	shoot("chooser-open-1280x900-light.png", 1280, 900, "light")
	shoot("chooser-open-1280x900-dark.png", 1280, 900, "dark")
	shoot("chooser-open-700x550-light.png", 700, 550, "light")
	shoot("chooser-open-700x550-dark.png", 700, 550, "dark")

	// With no families configured at all, the workspace offers the
	// management path instead of a match verdict.
	for _, id := range familyID {
		if err := a.deleteFamily(id); err != nil {
			t.Fatal(err)
		}
	}
	run(`location.reload()`)
	wait(`document.body.innerText.includes('Generalist.pdf')`)
	clickButton("Documents")
	wait(`document.querySelector('#prepare')?.innerText.includes('No role families yet')`)
	var zeroCount string
	if err := eval(`(async()=>{const r=await fetch('/api/jobs/1/workspace');const j=await r.json();return ''+j.familyCount})()`, &zeroCount); err != nil || zeroCount != "0" {
		t.Fatalf("familyCount after delete=%q err=%v", zeroCount, err)
	}
}
