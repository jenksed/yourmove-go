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
				"descriptionPlain": "Build developer tooling and automation."
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

	if metadata.Description != "Build developer tooling and automation." {
		t.Errorf("unexpected description: %q", metadata.Description)
	}
}
