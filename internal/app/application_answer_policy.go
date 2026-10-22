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
	id := strings.TrimPrefix(f.ID, "browser:")
	if !unique || f.Sensitive || id == "" || positionalQuestionID.MatchString(id) || strings.HasPrefix(id, "unidentified-") || strings.Contains(strings.ToLower(f.Label), "cover letter") {
		return false
	}
	switch f.Kind {
	case "text", "email", "tel", "url", "textarea", "select", "checkbox", "radio":
		return true
	}
	return false
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

// These exact labels and identities intentionally exclude eligibility, consent,
// demographic questions and general narrative prompts. No value is inferred.
func resolveAdditionalProfileFact(f schema.Field, p CandidateProfile) string {
	id := strings.TrimPrefix(strings.TrimPrefix(f.ID, "browser:"), "application:")
	if !p.Approved || f.Sensitive || id == "" || positionalQuestionID.MatchString(id) || strings.HasPrefix(id, "unidentified-") {
		return ""
	}
	label := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(f.Label), "*")))
	// Ordinary profile fields may have employer-generated opaque identities.
	label = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(label, " (optional)"), " (required)"))
	linkedin := label == "linkedin" || label == "linkedin url" || label == "linkedin profile" || label == "linkedin profile url" || label == "linkedin profile link" || label == "linkedin link" || label == "your linkedin profile" || label == "your linkedin url"
	if linkedin {
		id = "linkedin"
	}
	location := label == "location" || label == "current location" || label == "your location"
	if location && (f.Kind == "text" || f.Kind == "combobox") {
		parts := []string{}
		for _, part := range []string{p.City, p.Region, p.Country} {
			if part = strings.TrimSpace(part); part != "" {
				parts = append(parts, part)
			}
		}
		return strings.Join(parts, ", ")
	}

	pronouns := label == "pronouns" || label == "preferred pronouns" || label == "what are your pronouns?" || label == "what are your pronouns"
	var value string
	urlField := false
	if pronouns {
		value = p.Pronouns
	} else {
		switch id {
		case "city":
			if label == "city" {
				value = p.City
			}
		case "region", "state", "state_province":
			if label == "state" || label == "region" || label == "state / region" || label == "state/region" || label == "state/province" {
				value = p.Region
			}
		case "country":
			if label == "country" {
				value = p.Country
			}
		case "linkedin", "linkedin_url":
			urlField = true
			if linkedin {
				value = p.LinkedIn
			}
		case "github", "github_url":
			urlField = true
			if label == "github" || label == "github url" || label == "github profile" || label == "github profile url" {
				value = p.GitHub
			}
		case "portfolio", "portfolio_url":
			urlField = true
			if label == "portfolio" || label == "portfolio url" || label == "portfolio website" || label == "portfolio or work sample url" {
				value = p.Portfolio
			}
		}
	}
	if value == "" {
		return ""
	}
	if f.Kind == "text" || f.Kind == "textarea" || (urlField && f.Kind == "url") {
		return value
	}
	if urlField || (f.Kind != "select" && !(pronouns && f.Kind == "radio")) {
		return ""
	}
	// Preserve exact employer text. Even two equivalent options are ambiguous.
	match := ""
	for _, option := range f.Options {
		equal := option == value
		if pronouns {
			equal = pronounOption(option, value)
		}
		if equal {
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

// Match the same explicit contact identities and compatible labels as the
// browser contact filler. A name belonging to a manager or reference is excluded.
func profileContactKey(f schema.Field) string {
	if f.Sensitive {
		return ""
	}
	id := strings.TrimPrefix(strings.TrimPrefix(f.ID, "browser:"), "application:")
	if match := namespacedSystemField.FindStringSubmatch(id); match != nil {
		id = match[1]
	}
	// A telephone input supplies machine semantics even with an opaque ID.
	// The exact compatible label below still excludes references and managers.
	if f.Kind == "tel" && namespacedCustomField.MatchString(id) {
		id = "phone"
	}
	id = strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(id))
	label := strings.ToLower(f.Label)
	label = strings.NewReplacer("(required)", "", "(optional)", "", "*", "", "✱", "").Replace(label)
	label = strings.Join(strings.Fields(label), " ")
	key := ""
	switch id {
	case "name", "fullname", "systemfieldname":
		if label == "name" || label == "full name" || label == "full legal name" || label == "your name" {
			key = "name"
		}
	case "firstname", "givenname", "systemfieldfirstname":
		if label == "first name" || label == "given name" {
			key = "first_name"
		}
	case "lastname", "familyname", "surname", "systemfieldlastname":
		if label == "last name" || label == "family name" || label == "surname" {
			key = "last_name"
		}
	case "email", "emailaddress", "systemfieldemail":
		if label == "email" || label == "email address" {
			key = "email"
		}
	case "phone", "phonenumber", "mobilephone", "systemfieldphone":
		if label == "phone" || label == "phone number" || label == "mobile phone" || label == "mobile number" {
			key = "phone"
		}
	}
	switch f.Kind {
	case "text", "input_text", "textarea":
		return key
	case "email":
		if key == "email" {
			return key
		}
	case "tel":
		if key == "phone" {
			return key
		}
	}
	return ""
}
