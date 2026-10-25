package jobdocument

import (
	"strings"
)

// SectionKind is a narrow reading category assigned only to an explicit,
// recognized section heading. Unknown is the safe default.
type SectionKind string

const (
	SectionUnknown          SectionKind = "unknown"
	SectionResponsibilities SectionKind = "responsibilities"
	SectionQualifications   SectionKind = "qualifications"
	SectionCompensation     SectionKind = "compensation"
	SectionHiring           SectionKind = "hiring"
)

// Section is a contiguous portion of a Document. Blocks includes the heading
// block, when present, so concatenating Sections in order reproduces the input
// document without dropping or duplicating content.
type Section struct {
	Kind   SectionKind
	Title  string
	Blocks []Block
}

// SplitSections partitions the original block sequence at headings. A
// preamble before the first heading is an unknown section. Every heading,
// including unknown headings and repeated headings, starts a new section.
// Non-heading blocks remain with the preceding heading, and all blocks retain
// their original order.
func SplitSections(document Document) []Section {
	if len(document.Blocks) == 0 {
		return nil
	}

	sections := make([]Section, 0, len(document.Blocks))
	current := Section{Kind: SectionUnknown}
	for _, block := range document.Blocks {
		if block.Kind == BlockHeading {
			if len(current.Blocks) > 0 {
				sections = append(sections, current)
			}
			current = Section{
				Kind:  classifyHeading(block.Level, blockText(block.Content)),
				Title: strings.Join(strings.Fields(blockText(block.Content)), " "),
			}
		}
		current.Blocks = append(current.Blocks, block)
	}
	if len(current.Blocks) > 0 {
		sections = append(sections, current)
	}
	return sections
}

func classifyHeading(level int, heading string) SectionKind {
	// H1 commonly names the role rather than a reading section. Keep it
	// unclassified even when its text happens to match a category heading.
	if level < 2 {
		return SectionUnknown
	}

	heading = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(heading), ":"))
	for _, candidate := range responsibilitiesHeadings {
		if strings.EqualFold(heading, candidate) {
			return SectionResponsibilities
		}
	}
	for _, candidate := range qualificationHeadings {
		if strings.EqualFold(heading, candidate) {
			return SectionQualifications
		}
	}
	for _, candidate := range compensationHeadings {
		if strings.EqualFold(heading, candidate) {
			return SectionCompensation
		}
	}
	for _, candidate := range hiringHeadings {
		if strings.EqualFold(heading, candidate) {
			return SectionHiring
		}
	}
	return SectionUnknown
}

var (
	responsibilitiesHeadings = []string{
		"Responsibilities",
		"What you will do",
		"What you'll do",
		"What you’ll do",
	}
	qualificationHeadings = []string{
		"Qualifications",
		"What we're looking for",
		"What you will bring",
		"What you'll bring",
	}
	compensationHeadings = []string{
		"Compensation",
		"Salary",
		"Pay",
	}
	hiringHeadings = []string{
		"How to apply",
		"Application instructions",
		"Application process",
		"Hiring instructions",
		"Hiring process",
		"Interview process",
	}
)

func blockText(inlines []Inline) string {
	var text strings.Builder
	for _, inline := range inlines {
		text.WriteString(inline.Text)
	}
	return text.String()
}
