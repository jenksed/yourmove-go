//go:build qualification

package app

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
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
)

// Production pages, real scripts, synthetic SQLite and files only. No employer
// session is started and no generation or external request is authorized.
func TestControlledProductExperience(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_PRODUCT") != "1" {
		t.Skip("opt-in product experience qualification")
	}
	a, _, job, _, key := newAnswerLibraryFixture(t)
	saveLibrarySourceAnswer(t, a, job, key, "I compared synthetic request logs, reproduced the issue locally, and documented the findings. The long-term outcome is unknown.", 0)
	entry, _, err := a.promoteApplicationAnswer(job.ID, key, 1)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := a.createCareerEvidenceFromLibrary(entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.updateCareerEvidence(evidence, 1, careerEvidenceContent{Title: "Investigating intermittent API timeouts", Situation: "A synthetic integration timed out intermittently.", Actions: "Compared logs and reproduced the issue locally.", OutcomeState: "unknown"}, true); err != nil {
		t.Fatal(err)
	}
	resume, err := a.importResume("Alex_Morgan_履歴書.pdf", bytes.NewReader(mustQualificationPDF(t)))
	if err != nil {
		t.Fatal(err)
	}
	if err = a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	if err = a.saveFamily(RoleFamily{Name: "Support engineering", TitleTerms: "support engineer"}); err != nil {
		t.Fatal(err)
	}
	if err = a.assignResumeFamily(resume.ID, 1, true); err != nil {
		t.Fatal(err)
	}
	for i := 2; i <= 5; i++ {
		if err = a.saveJob(Job{URL: fmt.Sprintf("https://example.test/role/%d", i), Host: "example.test", Company: "Synthetic Cooperative", Title: fmt.Sprintf("Implementation Engineer, Secure Data Exchange %d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	if err = a.recordSubmission(5, "Synthetic applicant confirmation", reviewedSubmissionToken(t, a, 5)); err != nil {
		t.Fatal(err)
	}
	if _, err = a.db.Exec(`INSERT INTO cover_letters(job_id,body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at) VALUES(1,?,'fixture','none',1,'master',1,'role','job','2026-10-23')`, "Dear hiring team,\n\nI investigate integration problems through careful reproduction, clear communication, and documented findings. I would welcome the opportunity to bring this approach to your support team.\n\nAlex Morgan"); err != nil {
		t.Fatal(err)
	}
	empty, _ := testWorkspace(t)
	router := http.NewServeMux()
	router.HandleFunc("GET /empty", empty.jobForm)
	router.Handle("/", newMux(a))
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	page := server.URL + "/jobs"
	chrome, err := chromium.LaunchFixture(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer chrome.Close()
	root := os.Getenv("YOURMOVE_PRODUCT_EVIDENCE")
	if root == "" {
		root = filepath.Join(t.TempDir(), "evidence")
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	before := os.Getenv("YOURMOVE_PRODUCT_CAPTURE_BASELINE") == "1"
	t.Log("synthetic evidence:", root)
	waitBriefJS(t, ctx, chrome, page, `document.readyState==='complete' && !!document.querySelector('.page-heading')`)
	navigate := func(path, selector string) {
		t.Helper()
		if err := chrome.FixtureCall(ctx, page, "Page.navigate", map[string]any{"url": server.URL + path}, nil); err != nil {
			t.Fatal(err)
		}
		page = server.URL + path
		waitBriefJS(t, ctx, chrome, page, `document.readyState==='complete' && !!document.querySelector(`+fmt.Sprintf("%q", selector)+`) && document.readyState==='complete'`)
	}
	var measurements []map[string]any
	capture := func(name, selector string, width, height int, theme string) {
		t.Helper()
		conn, err := chrome.FixturePage(ctx, page)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		if err = conn.Call(ctx, "Emulation.setDeviceMetricsOverride", map[string]any{"width": width, "height": height, "deviceScaleFactor": 1, "mobile": false}, nil); err != nil {
			t.Fatal(err)
		}
		var result struct {
			Result struct {
				Value map[string]any `json:"value"`
			} `json:"result"`
		}
		script := fmt.Sprintf(`(()=>{document.documentElement.dataset.theme=%q;document.documentElement.style.scrollBehavior='auto';if(%q==='.page-heading'){window.scrollTo(0,0)}else{document.querySelector(%q).scrollIntoView({block:'start',behavior:'instant'});window.scrollBy(0,-18)};document.getAnimations().forEach(a=>a.finish());return {innerWidth,innerHeight,scrollWidth:document.documentElement.scrollWidth,firstJobTop:document.querySelector('[data-job-row]:not([hidden])')?.getBoundingClientRect().top??null}})()`, theme, selector, selector)
		if err = conn.Call(ctx, "Runtime.evaluate", map[string]any{"expression": script, "returnByValue": true}, &result); err != nil {
			t.Fatal(err)
		}
		var shot struct {
			Data string `json:"data"`
		}
		if err = conn.Call(ctx, "Page.captureScreenshot", map[string]any{"format": "png", "captureBeyondViewport": false}, &shot); err != nil {
			t.Fatal(err)
		}
		data, err := base64.StdEncoding.DecodeString(shot.Data)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(root, name+".png"), data, 0600); err != nil {
			t.Fatal(err)
		}
		m := result.Result.Value
		m["capture"] = name
		measurements = append(measurements, m)
		if m["scrollWidth"].(float64) > m["innerWidth"].(float64) {
			t.Errorf("horizontal overflow: %s %v", name, m)
		}
	}
	for _, s := range []struct{ name, path, selector string }{
		{"empty", "/empty", ".page-heading"}, {"opportunities", "/jobs", ".page-heading"}, {"overview", "/jobs/1#overview", "#overview"}, {"prepare", "/jobs/1#prepare", "#prepare"}, {"answers", "/jobs/1#answers", "#answers"}, {"letter", "/jobs/1#cover-letter", "#cover-letter"}, {"documents", "/resumes", ".page-heading"}, {"candidate", "/candidate", ".page-heading"}, {"library", "/answer-library", ".page-heading"}, {"career-evidence", "/career-evidence", ".page-heading"}, {"history", "/jobs/5#history", "#history"}, {"applications", "/applications", ".page-heading"}, {"recovery", "/jobs/1#transfer-review", "#transfer-review"}, {"ai", "/ai", ".page-heading"}, {"families", "/families", ".page-heading"}, {"inspection", "/jobs/1/inspection", ".page-heading"}, {"utilities", "/utilities", ".page-heading"}, {"posting-checks", "/enrichment", ".page-heading"}, {"requirements", "/requirements", ".page-heading"},
	} {
		if before {
			switch s.name {
			case "answers":
				s.path = "/jobs/1#application-questions"
				s.selector = "#application-questions"
			case "letter":
				s.path = "/jobs/1#letter-editor"
				s.selector = "#letter-editor"
			case "recovery":
				s.path = "/jobs/1#application-action"
				s.selector = "#application-action"
			}
		}
		if s.name == "recovery" {
			a.browsers.setState(1, "interrupted", "Synthetic employer browser interrupted; saved materials remain.", 0, false)
			if err = a.assignResume(1, 0); err != nil {
				t.Fatal(err)
			}
		}
		navigate(s.path, s.selector)
		for _, theme := range []string{"light", "dark"} {
			capture(s.name+"-"+theme, s.selector, 1100, 800, theme)
		}
		if s.name == "recovery" {
			if err = a.assignResume(1, resume.ID); err != nil {
				t.Fatal(err)
			}
		}
		if s.name == "opportunities" {
			capture("opportunities-900-light", s.selector, 900, 700, "light")
		}
	}
	if before {
		report, _ := json.MarshalIndent(measurements, "", "  ")
		if err = os.WriteFile(filepath.Join(root, "measurements.json"), report, 0600); err != nil {
			t.Fatal(err)
		}
		return
	}
	navigate("/jobs/1?draft=1#prepare", "#prepare")
	assertBriefJS(t, ctx, chrome, page, `!document.querySelector('#prepare').hidden && document.querySelector('#cover-letter').hidden`)
	navigate("/jobs/1?question="+key+"#cover-letter", "#cover-letter")
	assertBriefJS(t, ctx, chrome, page, `!document.querySelector('#cover-letter').hidden && document.querySelector('#answers').hidden`)
	navigate("/jobs/1#answers", "#answers")
	for _, size := range [][2]int{{900, 700}, {750, 650}, {700, 550}} {
		capture(fmt.Sprintf("answers-%dx%d", size[0], size[1]), "#answers", size[0], size[1], "dark")
	}
	connection, err := chrome.FixturePage(ctx, page)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err = connection.Call(ctx, "Emulation.setDeviceMetricsOverride", map[string]any{"width": 700, "height": 550, "deviceScaleFactor": 1, "mobile": false}, nil); err != nil {
		t.Fatal(err)
	}
	// Compact navigation: all destinations remain reachable; Escape restores focus.
	clickBriefUI(t, ctx, chrome, page, "[data-navigation-toggle]")
	assertBriefJS(t, ctx, chrome, page, `document.querySelectorAll('#workspace-navigation a.nav-item').length===8 && document.querySelector('[data-navigation-toggle]').getAttribute('aria-expanded')==='true'`)
	if err = chrome.FixtureCall(ctx, page, "Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": "Escape", "code": "Escape", "windowsVirtualKeyCode": 27}, nil); err != nil {
		t.Fatal(err)
	}
	assertBriefJS(t, ctx, chrome, page, `document.activeElement.matches('[data-navigation-toggle]') && document.querySelector('[data-navigation-toggle]').getAttribute('aria-expanded')==='false'`)
	// Activity switching retains the exact dirty node; departure remains guarded.
	assertBriefJS(t, ctx, chrome, page, `(()=>{window.productEditor=document.querySelector('#answers [name=answer]');productEditor.value='Unsaved synthetic adaptation';productEditor.dispatchEvent(new Event('input',{bubbles:true}));return true})()`)
	clickBriefUI(t, ctx, chrome, page, ".workspace-tab[data-tab-target=cover-letter]")
	clickBriefUI(t, ctx, chrome, page, ".workspace-tab[data-tab-target=answers]")
	assertBriefJS(t, ctx, chrome, page, `productEditor===document.querySelector('#answers [name=answer]') && productEditor.value==='Unsaved synthetic adaptation'`)
	clickBriefUI(t, ctx, chrome, page, "[data-navigation-toggle]")
	clickBriefUI(t, ctx, chrome, page, ".nav-item[href='/candidate']")
	assertBriefJS(t, ctx, chrome, page, `document.querySelector('.leave-materials-dialog').open`)
	capture("unsaved-dialog", ".leave-materials-dialog", 700, 550, "dark")
	clickBriefUI(t, ctx, chrome, page, "[data-stay]")
	assertBriefJS(t, ctx, chrome, page, `productEditor.value==='Unsaved synthetic adaptation' && !document.querySelector('.leave-materials-dialog').open`)
	clickBriefUI(t, ctx, chrome, page, ".nav-item[href='/candidate']")
	clickBriefUI(t, ctx, chrome, page, "[data-leave]")
	page = server.URL + "/candidate"
	waitBriefJS(t, ctx, chrome, page, `location.pathname==='/candidate' && !document.querySelector('dialog[open]')`)
	report, err := json.MarshalIndent(measurements, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "measurements.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
	t.Log("PASS: populated production surfaces, both themes, compact layout, eight destinations, Escape/focus return, same-node dirty activity switches, Stay/Leave exact destination; no generation, employer writes or submissions")
}
