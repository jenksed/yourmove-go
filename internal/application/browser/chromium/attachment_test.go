package chromium

import (
	"errors"
	"testing"
)

func TestChooseResumeInput(t *testing.T) {
	cases := []struct {
		name   string
		inputs []fileInput
		want   int
		err    error
	}{
		{"no control", nil, -1, ErrNoResumeInput},
		{"resume and cover letter", []fileInput{{Label: "Cover letter (optional)"}, {Label: "Resume upload"}}, 1, nil},
		{"only cover letter", []fileInput{{Label: "Cover letter"}}, -1, ErrAmbiguousResumeInput},
		{"multiple resumes", []fileInput{{Label: "Resume"}, {Label: "CV"}}, -1, ErrAmbiguousResumeInput},
		{"native resume plus parser", []fileInput{{Label: "Resume auto-fill", Parser: true}, {Label: "Upload resume", Parser: false}}, 1, nil},
		{"parser is not attachment", []fileInput{{Label: "Resume auto-fill", Parser: true}}, -1, ErrAmbiguousResumeInput},
		{"anonymous file", []fileInput{{Label: "file-upload"}}, -1, ErrAmbiguousResumeInput},
		{"existing user file", []fileInput{{Label: "Resume", Existing: true}}, -1, nil},
		{"pdf rejected by DOCX-only control", []fileInput{{Label: "Resume", Accept: ".docx"}}, -1, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := chooseResumeInput(tc.inputs, ".pdf")
			if got != tc.want {
				t.Fatalf("idx %d want %d", got, tc.want)
			}
			if tc.err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("err %v want %v", err, tc.err)
			}
			if tc.err == nil && tc.want >= 0 && err != nil {
				t.Fatal(err)
			}
			if tc.want < 0 && err == nil {
				t.Fatal("unsafe input accepted")
			}
		})
	}
}

func TestChooseCoverLetterInput(t *testing.T) {
	cases := []struct {
		name   string
		inputs []fileInput
		want   int
	}{
		{"cover beside resume", []fileInput{{Label: "Resume"}, {Label: "Cover Letter (optional)", Accept: "application/pdf"}}, 1},
		{"no letter", []fileInput{{Label: "Resume"}}, -1},
		{"ambiguous", []fileInput{{Label: "Cover Letter"}, {Label: "Cover Letter"}}, -1},
		{"combined purpose", []fileInput{{Label: "Resume or cover letter"}}, -1},
		{"unsupported format", []fileInput{{Label: "Cover Letter", Accept: ".docx"}}, -1},
		{"existing", []fileInput{{Label: "Cover Letter", Existing: true}}, -1},
		{"disabled", []fileInput{{Label: "Cover Letter", Unsafe: true}}, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := chooseDocumentInput(tc.inputs, ".pdf", true)
			if got != tc.want || (err == nil) != (tc.want >= 0) {
				t.Fatalf("got %d %v", got, err)
			}
		})
	}
}
