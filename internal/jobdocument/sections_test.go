package jobdocument

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestClassifyHeadingUsesOnlyExplicitVocabulary(t *testing.T) {
	tests := []struct {
		name    string
		level   int
		heading string
		want    SectionKind
	}{
		{"responsibilities", 2, "Responsibilities", SectionResponsibilities},
		{"responsibilities case and colon", 2, "WHAT YOU WILL DO:", SectionResponsibilities},
		{"responsibilities contraction", 3, "What you'll do", SectionResponsibilities},
		{"responsibilities typographic contraction", 2, "What you’ll do", SectionResponsibilities},
		{"qualifications", 2, "Qualifications", SectionQualifications},
		{"qualifications contraction", 2, "What we're looking for", SectionQualifications},
		{"qualifications bring", 2, "What you will bring", SectionQualifications},
		{"qualifications bring contraction", 2, "What you'll bring", SectionQualifications},
		{"compensation", 2, "Compensation", SectionCompensation},
		{"salary", 2, "Salary", SectionCompensation},
		{"pay", 2, "Pay", SectionCompensation},
		{"application instructions", 2, "How to apply", SectionHiring},
		{"application instructions heading", 2, "Application instructions", SectionHiring},
		{"application process", 2, "Application process", SectionHiring},
		{"hiring instructions heading", 2, "Hiring instructions", SectionHiring},
		{"hiring process", 2, "Hiring process", SectionHiring},
		{"interview process", 3, "Interview process", SectionHiring},
		{"compound heading", 2, "Qualifications and compensation", SectionUnknown},
		{"near match", 2, "What you'll do next", SectionUnknown},
		{"disclaimer", 2, "Equal employment opportunity", SectionUnknown},
		{"legal disclaimer", 2, "Employment disclaimer", SectionUnknown},
		{"benefits are secondary", 2, "Benefits", SectionUnknown},
		{"non-English", 2, "Cómo solicitar", SectionUnknown},
		{"paragraph-like h1", 1, "Responsibilities", SectionUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyHeading(tt.level, tt.heading); got != tt.want {
				t.Fatalf("classifyHeading(%d, %q) = %q, want %q", tt.level, tt.heading, got, tt.want)
			}
		})
	}
}

func TestSplitSectionsPreservesEveryBlockAndSourceOrder(t *testing.T) {
	doc, err := ParseDescription(`<p>Role overview before any section.</p>`+
		`<h1>Senior Engineer</h1><p>Opening detail.</p>`+
		`<h2>What you will do</h2><p>First responsibilities.</p>`+
		`<h2>Responsibilities:</h2><ul><li>Repeated responsibility section.</li></ul>`+
		`<h2>Qualifications</h2><p>Experience with production systems.</p>`+
		`<h2>Equal employment opportunity</h2><p>All applicants are considered under the same policy.</p>`+
		`<h2>How to apply</h2><ol><li>Send the requested materials.</li></ol>`+
		`<p>Contact the hiring team about accommodations.</p>`, "", testSavedPostingURL)
	if err != nil {
		t.Fatal(err)
	}

	sections := SplitSections(doc)
	var flattened []Block
	for _, section := range sections {
		flattened = append(flattened, section.Blocks...)
	}
	if !reflect.DeepEqual(flattened, doc.Blocks) {
		t.Fatalf("split/flatten changed document blocks\n got: %#v\nwant: %#v", flattened, doc.Blocks)
	}

	wantKinds := []SectionKind{
		SectionUnknown, // preamble
		SectionUnknown, // H1 role title and opening detail
		SectionResponsibilities,
		SectionResponsibilities, // repeated heading remains separate
		SectionQualifications,
		SectionUnknown, // consequential but unrecognized policy content
		SectionHiring,
	}
	if len(sections) != len(wantKinds) {
		t.Fatalf("section count = %d, want %d: %+v", len(sections), len(wantKinds), sections)
	}
	for i, want := range wantKinds {
		if sections[i].Kind != want {
			t.Errorf("section %d (%q) kind = %q, want %q", i, sections[i].Title, sections[i].Kind, want)
		}
	}
	if sections[0].Title != "" || sections[0].Blocks[0].Kind != BlockParagraph {
		t.Fatalf("preamble was not retained as unknown content: %+v", sections[0])
	}
	if sections[2].Title != "What you will do" || sections[3].Title != "Responsibilities:" {
		t.Fatalf("repeated headings were merged, normalized in-place, or reordered: %+v", sections[2:4])
	}
	if got := inlinePlainText(sections[5].Blocks[0].Content); got != "Equal employment opportunity" {
		t.Fatalf("unknown disclaimer heading was not retained: %q", got)
	}
	last := sections[len(sections)-1]
	if len(last.Blocks) != 3 || last.Blocks[len(last.Blocks)-1].Kind != BlockParagraph ||
		inlinePlainText(last.Blocks[len(last.Blocks)-1].Content) != "Contact the hiring team about accommodations." {
		t.Fatalf("final hiring instructions/trailing content were lost: %+v", last)
	}
}

func TestSplitSectionsAbstainsOnUnlistedOrAmbiguousHeadings(t *testing.T) {
	doc, err := ParseDescription(`<p>What you'll do appears here as prose, not a heading.</p>`+
		`<h2>Requirements and limitations</h2><p>Important legal terms remain available.</p>`+
		`<h2>What we expect and what we offer</h2><p>Combined, ambiguous content.</p>`+
		`<h2>Información sobre la solicitud</h2><p>Untranslated source content.</p>`, "", testSavedPostingURL)
	if err != nil {
		t.Fatal(err)
	}

	sections := SplitSections(doc)
	if len(sections) != 4 {
		t.Fatalf("section count = %d, want 4: %+v", len(sections), sections)
	}
	for i, section := range sections {
		if section.Kind != SectionUnknown {
			t.Errorf("section %d (%q) unexpectedly classified as %q", i, section.Title, section.Kind)
		}
	}
	if len(sections[0].Blocks) != 1 || sections[0].Blocks[0].Kind != BlockParagraph {
		t.Fatalf("ordinary prose was reinterpreted as a heading: %+v", sections[0])
	}
	if got := inlinePlainText(sections[1].Blocks[0].Content); got != "Requirements and limitations" {
		t.Fatalf("ambiguous requirements heading was not preserved: %q", got)
	}
}

func TestSplitSectionsClassifiesSyntheticM0AshbyPosting(t *testing.T) {
	data, err := os.ReadFile("../platforms/testdata/job-intelligence/ashby-long-synthetic.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Jobs []struct {
			DescriptionHTML string `json:"descriptionHtml"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Jobs) != 1 {
		t.Fatalf("fixture jobs = %d, want one", len(fixture.Jobs))
	}
	doc, err := ParseDescription(fixture.Jobs[0].DescriptionHTML, "", testSavedPostingURL)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[SectionKind]bool{}
	for _, section := range SplitSections(doc) {
		seen[section.Kind] = true
	}
	for _, want := range []SectionKind{
		SectionResponsibilities,
		SectionQualifications,
		SectionCompensation,
		SectionHiring,
	} {
		if !seen[want] {
			t.Errorf("M0 Ashby posting did not recognize %q", want)
		}
	}
}

func inlinePlainText(inlines []Inline) string {
	var text string
	for _, inline := range inlines {
		text += inline.Text
	}
	return text
}
