package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/jenksed/yourmove-go/internal/jobdocument"
)

// jobReadingView is the single prepared description and fact projection used
// by the Overview. Canonical job fields remain the inputs to existing writers.
type jobReadingView struct {
	PrimaryDocument   jobdocument.Document
	Sections          []jobReadingSection
	SectionJumps      []jobReadingSectionJump
	HasPrimary        bool
	PrimaryLabel      string
	EmployerDocument  jobdocument.Document
	HasEmployer       bool
	CanonicalText     string
	HasCanonical      bool
	ContentNote       string
	FreshnessLabel    string
	FreshnessDetail   string
	Location          string
	Workplace         string
	EmploymentType    string
	SalaryNote        string
	EmployerSalary    string
	HasEmployerSalary bool
}

type jobReadingSection struct {
	Kind     jobdocument.SectionKind
	Title    string
	AnchorID string
	Blocks   []jobdocument.Block
}

type jobReadingSectionJump struct {
	Label    string
	AnchorID string
}

func buildJobReadingView(job Job, source jobPostingSourceState, sourceFound bool) jobReadingView {
	view := jobReadingView{FreshnessLabel: postingSourceFreshness(job, source, sourceFound)}
	retained := postingSourceWasRetained(job, source, sourceFound)
	view.FreshnessDetail = postingSourceFreshnessDetail(job, source, sourceFound, retained)

	canonical := job.DisplayDescription()
	canonicalDoc, hasCanonical := jobDescriptionText(canonical)
	view.CanonicalText = canonical
	view.HasCanonical = hasCanonical

	var employerDoc jobdocument.Document
	hasEmployerDoc := false
	if sourceFound && !source.Invalid && source.Source != nil &&
		(strings.TrimSpace(source.Source.DescriptionHTML) != "" || strings.TrimSpace(source.Source.DescriptionPlain) != "") {
		parsed, err := jobdocument.ParseDescription(source.Source.DescriptionHTML, source.Source.DescriptionPlain, job.URL)
		if err == nil {
			employerDoc = parsed
			hasEmployerDoc = true
		}
	}

	if job.DescriptionManual {
		view.PrimaryDocument, view.HasPrimary = canonicalDoc, hasCanonical
		view.PrimaryLabel = "Your saved description"
		view.EmployerDocument, view.HasEmployer = employerDoc, hasEmployerDoc
		if hasEmployerDoc {
			view.ContentNote = "Your saved edit is primary. The employer-provided version is retained below for comparison."
		}
	} else if hasEmployerDoc {
		view.PrimaryDocument = employerDoc
		view.HasPrimary = true
		view.PrimaryLabel = "Employer-provided description"
	} else if hasCanonical {
		view.PrimaryDocument, view.HasPrimary = canonicalDoc, true
		view.PrimaryLabel = "Saved description"
		if sourceFound && !source.Invalid && source.Source != nil {
			view.ContentNote = "The employer source contains role facts but no readable description. Showing the saved canonical text."
		} else if sourceFound && source.Invalid {
			view.ContentNote = "The retained employer source could not be validated. Showing the saved canonical text."
		} else if sourceFound && source.Availability == "unavailable" {
			view.ContentNote = "The latest successful retrieval did not provide a rich description. Showing the saved canonical text."
		}
	} else if sourceFound && source.Invalid {
		view.ContentNote = "The retained employer source could not be validated, and no readable canonical description is saved."
	} else {
		view.ContentNote = "No description is saved yet. Paste the job description below to read it and prepare your application."
	}
	if view.HasPrimary {
		view.Sections, view.SectionJumps = jobReadingSections(view.PrimaryDocument)
	}

	if sourceFound && !source.Invalid && source.Source != nil {
		view.Location = source.Source.Location
		view.Workplace = workplaceArrangement(source.Source.WorkplaceType)
		view.EmploymentType = employmentTypeLabel(source.Source.EmploymentType)
		if source.Source.SalarySummary != "" && source.Source.SalarySummary != job.SalaryText {
			view.EmployerSalary = source.Source.SalarySummary
			view.HasEmployerSalary = true
		}
	}
	view.SalaryNote = jobSalaryProvenance(job, source, sourceFound, retained)
	return view
}

func jobReadingSections(document jobdocument.Document) ([]jobReadingSection, []jobReadingSectionJump) {
	classified := jobdocument.SplitSections(document)
	sections := make([]jobReadingSection, 0, len(classified))
	firstAnchors := make(map[jobdocument.SectionKind]string, 4)
	occurrences := make(map[jobdocument.SectionKind]int, 4)
	for _, section := range classified {
		view := jobReadingSection{Kind: section.Kind, Title: section.Title, Blocks: section.Blocks}
		if section.Kind != jobdocument.SectionUnknown {
			occurrences[section.Kind]++
			view.AnchorID = fmt.Sprintf("job-section-%s-%d", section.Kind, occurrences[section.Kind])
			if _, found := firstAnchors[section.Kind]; !found {
				firstAnchors[section.Kind] = view.AnchorID
			}
		}
		sections = append(sections, view)
	}

	// Show a stable priority order in the navigation while leaving the full
	// document itself untouched in its employer-provided order.
	jumps := make([]jobReadingSectionJump, 0, len(firstAnchors))
	for _, item := range []jobReadingSectionJump{
		{Label: "Responsibilities", AnchorID: firstAnchors[jobdocument.SectionResponsibilities]},
		{Label: "Qualifications", AnchorID: firstAnchors[jobdocument.SectionQualifications]},
		{Label: "Compensation", AnchorID: firstAnchors[jobdocument.SectionCompensation]},
		{Label: "Hiring process", AnchorID: firstAnchors[jobdocument.SectionHiring]},
	} {
		if item.AnchorID != "" {
			jumps = append(jumps, item)
		}
	}
	return sections, jumps
}

func jobDescriptionText(value string) (jobdocument.Document, bool) {
	if strings.TrimSpace(value) == "" {
		return jobdocument.Document{}, false
	}
	doc, err := jobdocument.ParsePlainText(value)
	if err != nil {
		return jobdocument.Document{}, false
	}
	return doc, true
}

func postingSourceWasRetained(job Job, source jobPostingSourceState, found bool) bool {
	if !found || source.Invalid {
		return false
	}
	if job.SourceStatus != "retrieved" || job.SourceCheckedAt == "" {
		return true
	}
	checked, checkedErr := time.Parse(time.RFC3339Nano, job.SourceCheckedAt)
	success, successErr := time.Parse(time.RFC3339Nano, source.LatestSuccessAt)
	return checkedErr != nil || successErr != nil || checked.After(success)
}

func postingSourceFreshness(job Job, source jobPostingSourceState, found bool) string {
	switch {
	case found && source.Invalid:
		return "Saved employer source could not be validated"
	case found && postingSourceWasRetained(job, source, found):
		return "Earlier successful employer source retained"
	case found && source.Availability == "unavailable":
		return "Latest retrieval succeeded without usable source content"
	case found:
		return "Employer source from the latest successful retrieval"
	case job.SourceStatus == "retrieved":
		return "Last checked; no structured source evidence is retained"
	case job.SourceCheckedAt != "":
		return "No successful source evidence is retained"
	default:
		return "No successful source retrieval is recorded"
	}
}

func postingSourceFreshnessDetail(job Job, source jobPostingSourceState, found, retained bool) string {
	if found && source.Invalid {
		return "Showing saved canonical text. Invalid source bytes and facts were ignored."
	}
	if found && retained {
		detail := "Last successful retrieval: " + readableSourceTime(source.LatestSuccessAt) + "."
		if job.SourceCheckedAt != "" {
			detail += " Latest check: " + job.SourceStatusLabel() + " on " + readableSourceTime(job.SourceCheckedAt) + "."
		}
		return detail
	}
	if found {
		return "Last successful retrieval: " + readableSourceTime(source.LatestSuccessAt) + "."
	}
	if job.SourceCheckedAt != "" {
		return "Last check: " + job.SourceStatusLabel() + " on " + readableSourceTime(job.SourceCheckedAt) + "."
	}
	return ""
}

func readableSourceTime(value string) string {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	return parsed.Local().Format("Jan 2, 2006 at 3:04 PM MST")
}

func workplaceArrangement(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "remote":
		return "Remote"
	case "hybrid":
		return "Hybrid"
	case "onsite", "on_site", "on-site":
		return "On-site"
	case "":
		return ""
	default:
		return "Unrecognized provider value: " + strings.TrimSpace(value)
	}
}

func employmentTypeLabel(value string) string {
	compact := strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.TrimSpace(value)))
	switch compact {
	case "fulltime":
		return "Full time"
	case "parttime":
		return "Part time"
	case "contract":
		return "Contract"
	case "temporary":
		return "Temporary"
	case "internship":
		return "Internship"
	case "volunteer":
		return "Volunteer"
	case "other":
		return "Other"
	case "":
		return ""
	default:
		return "Unrecognized provider value: " + strings.TrimSpace(value)
	}
}

func jobSalaryProvenance(job Job, source jobPostingSourceState, found, retained bool) string {
	if job.SalaryManual {
		return "Manually edited by you."
	}
	if !found || source.Invalid {
		return job.SalaryOriginLabel()
	}
	if source.SalaryConfirmed {
		if retained {
			return "Employer value retained from an earlier successful retrieval."
		}
		return "Confirmed by the latest successful retrieval."
	}
	if job.SalaryText != "" {
		return "Saved value retained; the latest successful retrieval did not confirm it."
	}
	return "No salary was supplied by the latest successful retrieval."
}
