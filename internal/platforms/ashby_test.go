package platforms

import "testing"

func TestParseAshbyURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		board   string
		id      string
		wantErr bool
	}{
		{
			name:  "valid Render posting",
			url:   "https://jobs.ashbyhq.com/render/a3f88090-5e37-43a7-b392-04b02056db6d",
			board: "render",
			id:    "a3f88090-5e37-43a7-b392-04b02056db6d",
		},
		{
			name:    "unsupported host",
			url:     "https://example.com/render/123",
			wantErr: true,
		},
		{
			name:    "missing posting ID",
			url:     "https://jobs.ashbyhq.com/render",
			wantErr: true,
		},
		{
			name:    "insecure URL",
			url:     "http://jobs.ashbyhq.com/render/123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			board, id, err := parseAshbyURL(tt.url)

			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tt.wantErr)
			}

			if err != nil {
				return
			}

			if board != tt.board || id != tt.id {
				t.Errorf(
					"got (%q, %q), want (%q, %q)",
					board, id, tt.board, tt.id,
				)
			}
		})
	}
}

func TestExtractAshbyMetadata(t *testing.T) {
	data := []byte(`{
		"jobs": [
			{
				"id": "other-job",
				"title": "Other Position",
				"descriptionPlain": "Wrong description"
			},
			{
				"id": "a3f88090-5e37-43a7-b392-04b02056db6d",
				"title": "Software Engineer, Dev Velocity (all levels)",
				"descriptionPlain": "Build developer tooling and automation.",
                "compensation": {"scrapeableCompensationSalarySummary": "$100K - $140K"}
			}
		]
	}`)

	metadata, err := extractAshbyMetadata(
		data,
		"a3f88090-5e37-43a7-b392-04b02056db6d",
	)
	if err != nil {
		t.Fatal(err)
	}

	if metadata.Title != "Software Engineer, Dev Velocity (all levels)" {
		t.Errorf("unexpected title: %q", metadata.Title)
	}

	if metadata.SalaryText != "$100K - $140K" {
		t.Errorf("salary summary = %q", metadata.SalaryText)
	}
	if metadata.Description != "Build developer tooling and automation." {
		t.Errorf("unexpected description: %q", metadata.Description)
	}
}

func TestAshbyHTMLDescriptionFallback(t *testing.T) {
	data := []byte(`{"jobs":[{"id":"c7f26e3d-7345-44a6-8e5a-3599679bfe9e","title":"Support Operations Lead","descriptionHtml":"<h2>About the role</h2><p>Lead operations &amp; automation.</p>","compensation":{"scrapeableCompensationSalarySummary":"$279K - $310K"}}]}`)
	got, err := extractAshbyMetadata(data, "c7f26e3d-7345-44a6-8e5a-3599679bfe9e")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Support Operations Lead" || got.Description != "About the role\nLead operations & automation." || got.SalaryText != "$279K - $310K" {
		t.Fatalf("Ashby fallback: %+v", got)
	}
}

func TestParseAshbyURLRejectsTruncatedUUID(t *testing.T) {
	if _, _, err := parseAshbyURL("https://jobs.ashbyhq.com/render/a3f88090-5e37-43a7-b392-04b02056d"); err == nil {
		t.Fatal("truncated UUID accepted")
	}
}

func TestAshbyCanMatchDocumentedJobURLWithoutID(t *testing.T) {
	data := []byte(`{"jobs":[{"jobUrl":"https://jobs.ashbyhq.com/openai/c7f26e3d-7345-44a6-8e5a-3599679bfe9e","title":"Support Operations Lead","descriptionPlain":"Team description"}]}`)
	got, err := extractAshbyMetadata(data, "c7f26e3d-7345-44a6-8e5a-3599679bfe9e")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Support Operations Lead" {
		t.Fatalf("posting not found via jobUrl: %+v", got)
	}
}
