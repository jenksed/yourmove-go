//go:build qualification

package app

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
)

// TestJobOverviewM0ResponsiveCapture records the existing saved-job Overview
// with synthetic data at the two M0 reference widths. It is opt-in because it
// launches a real browser and writes screenshots to an explicit evidence dir.
func TestJobOverviewM0ResponsiveCapture(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_M0_OVERVIEW") != "1" {
		t.Skip("opt-in synthetic Ashby Overview Chromium qualification")
	}
	evidenceDir := os.Getenv("YOURMOVE_M0_EVIDENCE_DIR")
	if evidenceDir == "" || !filepath.IsAbs(evidenceDir) {
		t.Fatal("YOURMOVE_M0_EVIDENCE_DIR must be an absolute directory")
	}
	if err := os.MkdirAll(evidenceDir, 0700); err != nil {
		t.Fatal(err)
	}

	a, _ := testWorkspace(t)
	description := strings.Join([]string{
		"About Northstar Analytics",
		"Northstar builds data tools for distributed public-interest teams. This synthetic posting describes a long role so the current Overview can be characterized without candidate information.",
		"",
		"About the role",
		"You will help a small platform group improve reliability, documentation, and the day-to-day work of product engineers. The team values clear writing and careful handoffs.",
		"",
		"What you'll do",
		"• Design dependable service boundaries and explain tradeoffs.",
		"• Improve operational tooling and developer workflows.",
		"  • Add useful metrics and test failure recovery.",
		"  • Keep runbooks current and easy to follow.",
		"• Work with product and support on issues that cross team boundaries.",
		"",
		"What we're looking for",
		"• Experience maintaining production services.",
		"• Ability to investigate unfamiliar systems and communicate clearly.",
		"• Interest in mentoring colleagues and improving team practices.",
		"",
		"How we work",
		"The role works with a remote-first team across several time zones. Written plans and recorded design decisions help colleagues contribute asynchronously.",
		"",
		"Additional information",
		"Read the synthetic team guide at https://guide.example.test/engineering before interviews. The hiring team will discuss accommodations and interview timing with candidates.",
		"",
		"End of synthetic posting.",
	}, "\n")
	postingURL := "https://jobs.ashbyhq.com/northstar/11111111-1111-4111-8111-111111111111"
	id, added, err := a.storeJobIfNew(Job{
		URL:             postingURL,
		Host:            "jobs.ashbyhq.com",
		Title:           "Senior Platform Engineer",
		Company:         "Northstar Analytics",
		Description:     description,
		SalaryText:      "$145,000–$185,000 USD per year",
		SalaryOrigin:    "employer",
		Stage:           StageReview,
		Priority:        PriorityNormal,
		SourceStatus:    "retrieved",
		SourceNote:      "Synthetic source fixture; application availability is not verified.",
		SourceCheckedAt: "2026-10-24T12:00:00Z",
	})
	if err != nil || !added {
		t.Fatalf("store synthetic Ashby job: id=%d added=%v err=%v", id, added, err)
	}

	path := "/jobs/" + strconv.FormatInt(id, 10)
	base := newMux(a)
	router := http.NewServeMux()
	router.HandleFunc("GET "+path+"/application", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, path+"#overview", http.StatusSeeOther)
	})
	router.Handle("/", base)
	server := httptest.NewServer(router)
	defer server.Close()
	pageURL := server.URL + path

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	chrome, err := chromium.LaunchFixture(ctx, pageURL)
	if err != nil {
		t.Fatal(err)
	}
	defer chrome.Close()
	for deadline := time.Now().Add(12 * time.Second); ; {
		var ready struct {
			Result struct {
				Value bool `json:"value"`
			} `json:"result"`
		}
		err := chrome.FixtureCall(ctx, pageURL, "Runtime.evaluate", map[string]any{
			"expression":    "document.readyState==='complete' && !!document.querySelector('#job-description')",
			"returnByValue": true,
		}, &ready)
		if err == nil && ready.Result.Value {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("saved-job Overview did not settle: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	page, err := chrome.FixturePage(ctx, pageURL)
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	for _, width := range []int{1280, 390} {
		const height = 900
		if err := page.Call(ctx, "Emulation.setDeviceMetricsOverride", map[string]any{
			"width": width, "height": height, "deviceScaleFactor": 1, "mobile": false,
		}, nil); err != nil {
			t.Fatalf("set %dx%d viewport: %v", width, height, err)
		}
		var view struct {
			Result struct {
				Value struct {
					InnerWidth         int    `json:"innerWidth"`
					ScrollWidth        int    `json:"scrollWidth"`
					ScrollHeight       int    `json:"scrollHeight"`
					Description        string `json:"description"`
					Overview           bool   `json:"overview"`
					DescriptionVisible bool   `json:"descriptionVisible"`
					Hash               string `json:"hash"`
				} `json:"value"`
			} `json:"result"`
		}
		if err := page.Call(ctx, "Runtime.evaluate", map[string]any{
			"expression":    `(()=>{window.scrollTo(0,0);document.documentElement.style.scrollBehavior='auto';document.activeElement?.blur();const d=document.querySelector('#job-description');const o=document.querySelector('[data-workspace-panel="overview"]');return {innerWidth,scrollWidth:document.documentElement.scrollWidth,scrollHeight:document.documentElement.scrollHeight,description:d?.textContent||'',overview:document.querySelector('[data-tab-target="overview"]')?.getAttribute('aria-selected')==='true'&&o?.hidden===false,descriptionVisible:!!d&&d.getClientRects().length>0,hash:location.hash}})()`,
			"returnByValue": true,
		}, &view); err != nil {
			t.Fatalf("inspect %dpx Overview: %v", width, err)
		}
		if !view.Result.Value.Overview || !view.Result.Value.DescriptionVisible {
			t.Fatalf("%dpx capture is not showing the Overview: overview=%v description_visible=%v hash=%q", width, view.Result.Value.Overview, view.Result.Value.DescriptionVisible, view.Result.Value.Hash)
		}
		if view.Result.Value.Description != description {
			t.Fatalf("%dpx rendered description differs from the saved canonical text", width)
		}
		var screenshot struct {
			Data string `json:"data"`
		}
		if err := page.Call(ctx, "Page.captureScreenshot", map[string]any{
			"format": "png", "captureBeyondViewport": false,
		}, &screenshot); err != nil {
			t.Fatalf("capture %dpx Overview: %v", width, err)
		}
		png, err := base64.StdEncoding.DecodeString(screenshot.Data)
		if err != nil {
			t.Fatalf("decode %dpx Overview screenshot: %v", width, err)
		}
		path := filepath.Join(evidenceDir, "current-overview-"+strconv.Itoa(width)+".png")
		if err := os.WriteFile(path, png, 0600); err != nil {
			t.Fatalf("write %dpx Overview screenshot: %v", width, err)
		}
		t.Logf("captured %s viewport=%dx%d document=%dx%d canonical_description_bytes=%d", path, view.Result.Value.InnerWidth, height, view.Result.Value.ScrollWidth, view.Result.Value.ScrollHeight, len(view.Result.Value.Description))
	}
	var clicked struct {
		Result struct {
			Value bool `json:"value"`
		} `json:"result"`
	}
	if err := page.Call(ctx, "Runtime.evaluate", map[string]any{
		"expression":    `(()=>{const b=document.querySelector('[data-copy-description]');if(!b||b.hidden)return false;Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>{window.__m0Copied=text}}});b.click();return true})()`,
		"returnByValue": true,
	}, &clicked); err != nil || !clicked.Result.Value {
		t.Fatalf("click the actual saved-job description copy control: clicked=%v err=%v", clicked.Result.Value, err)
	}
	for deadline := time.Now().Add(3 * time.Second); ; {
		var copied struct {
			Result struct {
				Value struct {
					Text   string `json:"text"`
					Status string `json:"status"`
				} `json:"value"`
			} `json:"result"`
		}
		err := page.Call(ctx, "Runtime.evaluate", map[string]any{
			"expression":    `(()=>({text:window.__m0Copied||'',status:document.querySelector('[data-copy-description-status]')?.textContent||''}))()`,
			"returnByValue": true,
		}, &copied)
		if err == nil && copied.Result.Value.Text == strings.TrimSpace(description) && copied.Result.Value.Status == "Job description copied." {
			t.Logf("actual Overview copy control PASS: %d canonical bytes copied; status=%q", len(copied.Result.Value.Text), copied.Result.Value.Status)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("actual Overview clipboard payload/status: copied_bytes=%d status=%q err=%v", len(copied.Result.Value.Text), copied.Result.Value.Status, err)
		}
		time.Sleep(25 * time.Millisecond)
	}
}
