package app

import (
	"regexp"
	"strings"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

var positionalQuestionID = regexp.MustCompile(`(?i)^(?:[0-9]+|(?:field|question|input)[_-]?[0-9]+)$`)

// Ashby renders stable field identities under a per-form UUID namespace.
const formUUID = `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`

var namespacedSystemField = regexp.MustCompile(`(?i)^` + formUUID + `__(systemfield_[a-z_]+)$`)
var namespacedCustomField = regexp.MustCompile(`(?i)^` + formUUID + `_` + formUUID + `$`)

func editableQuestion(f schema.Field, unique bool) bool {
	if !stableQuestionIdentity(f, unique) {
		return false
	}
	switch f.Kind {
	case "text", "email", "tel", "url", "textarea", "select", "checkbox", "radio":
		return true
	}
	return false
}

// editableQuestionForPlatform permits only the observed Ashby location
// autocomplete, which has a dedicated guarded path in the Chromium filler.
func editableQuestionForPlatform(f schema.Field, unique bool, platform string, p CandidateProfile) bool {
	if editableQuestion(f, unique) {
		return true
	}
	return platform == "ashby" && stableQuestionIdentity(f, unique) && f.Kind == "combobox" && supportedLocationLabel(f.Label) && candidateLocation(p) != ""
}

func profileQuestionIdentity(id string) string {
	for _, prefix := range []string{"browser:", "application:", "preview:"} {
		id = strings.TrimPrefix(id, prefix)
	}
	return id
}

func stableQuestionIdentity(f schema.Field, unique bool) bool {
	if !unique || f.Sensitive {
		return false
	}
	// Observer fallback wording describes the observation failure, never the
	// employer's question, so it must not become a question identity,
	// reusable entry, or transfer target.
	if inspectorFallbackLabel(f.Label) {
		return false
	}
	id := profileQuestionIdentity(f.ID)
	if id == "" || strings.HasPrefix(id, "unidentified-") || strings.Contains(strings.ToLower(f.Label), "cover letter") {
		return false
	}
	// Positional machine identities ("question-1", "field_3") are DOM
	// observation junk, never employer questions. Greenhouse public API
	// definitions are the exception: the provider itself names custom
	// questions "question_N", so those identities are authoritative.
	if positionalQuestionID.MatchString(id) && !providerDefinitionIdentity(f.ID) {
		return false
	}
	return true
}

// providerDefinitionIdentity reports IDs minted by a documented provider
// definition (Greenhouse public API groups), as opposed to live DOM
// observation. Only the Greenhouse inspector uses these group prefixes.
func providerDefinitionIdentity(id string) bool {
	for _, prefix := range []string{"application:", "location:", "compliance:"} {
		if strings.HasPrefix(id, prefix) {
			return true
		}
	}
	return false
}

// inspectorFallbackLabel reports labels the read-only observer emits when it
// could not establish employer wording. They describe the observation
// failure, never the employer's question.
func inspectorFallbackLabel(label string) bool {
	trimmed := strings.TrimSpace(label)
	return trimmed == "Unlabeled control" || strings.HasPrefix(trimmed, "Radio group: ")
}

// employerOnlyQuestion labels identify questions the existing preparation
// policy excludes for privacy or legal attestation reasons. Ordinary contact
// labels remain actionable when an approved profile fact is missing.
func employerOnlyQuestion(f schema.Field) bool {
	if f.Sensitive {
		return true
	}
	for _, phrase := range []string{
		"authorization", "authorized to work", "eligible to work", "work eligibility", "visa", "sponsor", "immigration", "citizenship", "citizen",
		"race", "ethnicity", "gender", "sexual orientation", "sponsorship", "work permit", "disability", "accommodation", "veteran",
		"date of birth", "birth date", "birth year", "birthday", "how old", "age", "religion", "marital status", "medical", "health condition", "demographic", "pronoun",
		"salary", "salaries", "compensation", "pay expectation", "desired pay", "hourly rate", "wage", "reference", "referee", "street address", "mailing address",
		"password", "credential", "secret", "api key", "access token", "social security", "ssn", "bank account",
		"agree to", "i agree", "consent", "acknowledg", "attest", "certify", "legally bound", "background check",
	} {
		if answerLibraryContainsPhrase(f.Label, phrase) {
			return true
		}
	}
	return false
}

func supportedLocationLabel(label string) bool {
	switch candidateProfileLabel(label) {
	case "location", "current location", "your location":
		return true
	}
	return false
}

func candidateProfileLabel(label string) string {
	label = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(label), "*")))
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(label, " (optional)"), " (required)"))
}

func candidateLocation(p CandidateProfile) string {
	if !p.Approved {
		return ""
	}
	parts := []string{}
	for _, part := range []string{p.City, p.Region, p.Country} {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, ", ")
}

// resolveApplicationAnswer is pure and uses explicit identity plus compatible
// labels. Label similarity alone never supplies an answer.
func resolveApplicationAnswer(f schema.Field, explicit string, hasExplicit bool, p CandidateProfile, memory string, hasMemory bool) (string, string) {
	if hasExplicit {
		return explicit, "Saved for this application"
	}
	if f.Sensitive {
		return "", ""
	}

	if p.Approved {
		if key := profileContactKey(f); key != "" {
			value := strings.TrimSpace((browser.ContactValues{FirstName: p.FirstName, LastName: p.LastName, Email: p.Email, Phone: p.Phone}).Value(key))
			if value != "" {
				return value, "Approved candidate information"
			}
		}
	}
	if value := resolveAdditionalProfileFact(f, p); value != "" {
		return value, "Approved candidate information"
	}
	if hasMemory && !f.Sensitive {
		return memory, "Reusable answer approved for this employer"
	}
	return "", ""
}

// Resolve values separately from intrinsic field identity and transfer authority.
func resolveAdditionalProfileFact(f schema.Field, p CandidateProfile) string {
	if !p.Approved {
		return ""
	}
	key := additionalProfileAttribute(f)
	if key == "location" {
		return candidateLocation(p)
	}
	value := map[string]string{"pronouns": p.Pronouns, "city": p.City, "region": p.Region, "country": p.Country, "linkedin": p.LinkedIn, "github": p.GitHub, "portfolio": p.Portfolio}[key]
	if value == "" {
		return ""
	}
	if f.Kind != "select" && f.Kind != "radio" {
		return value
	}
	match := ""
	for _, option := range f.Options {
		if option == value || (key == "pronouns" && pronounOption(option, value)) {
			if match != "" {
				return ""
			}
			match = option
		}
	}
	return match
}

func pronounOption(option, preference string) bool {
	// Only case and spacing around slashes vary; no fuzzy or substring matching.
	parts := strings.Split(strings.ToLower(strings.TrimSpace(option)), "/")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	normalized := strings.Join(parts, "/")
	switch preference {
	case "He/him":
		return normalized == "he/him" || normalized == "he/him/his"
	case "She/her":
		return normalized == "she/her" || normalized == "she/her/hers"
	case "They/them":
		return normalized == "they/them" || normalized == "they/them/theirs"
	}
	return false
}
