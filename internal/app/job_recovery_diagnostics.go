package app

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"unicode"
)

// recoveryDiagnostic is deliberately a metadata-only projection. In particular,
// URL paths, queries, raw errors and applicant-authored content never leave the
// local job through this endpoint. Hostnames still require applicant inspection.
type recoveryDiagnostic struct {
	Version           int    `json:"version"`
	Host              string `json:"hostname_review_before_sharing"`
	PlatformFamily    string `json:"platform_family"`
	URLShape          string `json:"url_path_shape_values_omitted"`
	PathSegments      int    `json:"path_segments"`
	HasQuery          bool   `json:"query_omitted"`
	Outcome           string `json:"retrieval_outcome"`
	DescriptionOrigin string `json:"description_origin"`
	HasDescription    bool   `json:"has_description"`
	RecoverySucceeded bool   `json:"recovered_locally"`
}

func diagnosticForJob(job Job) recoveryDiagnostic {
	d := recoveryDiagnostic{Version: 1, PlatformFamily: "unknown", Outcome: "not_checked", DescriptionOrigin: "unknown", HasDescription: strings.TrimSpace(job.Description) != ""}
	if u, err := url.Parse(job.URL); err == nil {
		d.Host = u.Hostname()
		for _, family := range []struct{ suffix, name string }{
			{".icims.com", "icims"}, {".taleo.net", "taleo"}, {".avature.net", "avature"}, {".jobs.personio.de", "personio"}, {".jobs.personio.com", "personio"}, {".teamtailor.com", "teamtailor"}, {".recruitee.com", "recruitee"},
			{"jobs.ashbyhq.com", "ashby"}, {"boards.greenhouse.io", "greenhouse"}, {"job-boards.greenhouse.io", "greenhouse"}, {"jobs.lever.co", "lever"},
		} {
			if strings.HasSuffix(strings.ToLower(d.Host), family.suffix) {
				d.PlatformFamily = family.name
				break
			}
		}

		for _, part := range strings.Split(u.Path, "/") {
			if part != "" {
				d.PathSegments++
				shape := ":text"
				if strings.TrimFunc(part, unicode.IsDigit) == "" {
					shape = ":number"
				}
				d.URLShape += "/" + shape
			}
		}
		d.HasQuery = u.RawQuery != ""
	}
	switch job.SourceStatus {
	case "unsupported":
		d.Outcome = "unknown_platform"
		if d.PlatformFamily != "unknown" {
			d.Outcome = "unsupported_url_variant"
		}
	case "failed":
		d.Outcome = "retrieval_unavailable"
	case "closed":
		d.Outcome = "posting_closed"
	case "retrieved":
		d.Outcome = "provider_retrieved"
	}
	if job.DescriptionManual {
		d.DescriptionOrigin = "applicant_supplied_or_corrected"
		d.RecoverySucceeded = d.HasDescription
	} else if job.SourceStatus == "retrieved" && d.HasDescription {
		d.DescriptionOrigin = "provider_source"
	}
	return d
}

var recoveryDiagnosticPreview = template.Must(template.New("diagnostic").Parse(`<!doctype html><html lang="en"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Review posting diagnostics</title><link rel="stylesheet" href="/static/styles.css"><main><h1>Review posting diagnostics</h1><p>This stays on your device. Downloading does not send it to anyone. Inspect the hostname before sharing; it may identify an employer or private career site.</p><p>This package omits the posting path, query values, descriptions, raw errors, browser content, resumes and answers. No source samples are included.</p><pre style="white-space:pre-wrap;overflow-wrap:anywhere">{{.JSON}}</pre><form method="post"><label><input type="checkbox" name="reviewed" value="yes" required> I have reviewed this metadata.</label><button type="submit">Download metadata</button></form><p><a href="/jobs/{{.ID}}#overview">Return to job</a></p></main></html>`))

func (a *App) jobRecoveryDiagnostics(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	job, err := a.jobByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	data, err := json.MarshalIndent(diagnosticForJob(job), "", "  ")
	if err != nil {
		http.Error(w, "Unable to prepare diagnostics", 500)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodPost {
		if !parseLocalForm(w, r, 1024) {
			return
		}
		if r.PostForm.Get("reviewed") != "yes" {
			http.Error(w, "Review the metadata before downloading", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="yourmove-posting-diagnostics.json"`)
		w.Write(data)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := recoveryDiagnosticPreview.Execute(w, struct {
		JSON string
		ID   int64
	}{string(data), id}); err != nil {
		fmt.Printf("diagnostic preview: %v\n", err)
	}
}
