package app

import (
	"github.com/jenksed/yourmove-go/internal/application/schema"
	"strings"
)

// Field role depends only on the observed control, never on a value, saved
// answer, provenance, profile approval, or previous Fill. Role grants no
// transfer authority: each operation still validates its explicitly authorized
// snapshot, revision, destination and live DOM target. Unsupported profile
// controls remain profile data and require manual completion.
type applicationFieldRole uint8

const (
	fieldUnsupported applicationFieldRole = iota
	fieldCandidateProfile
	fieldApplicationQuestion
	fieldDocument
	fieldEmployerOnly
)

type applicationFieldClassification struct {
	Role             applicationFieldRole
	ProfileAttribute string
}

func classifyApplicationField(f schema.Field, unique bool) applicationFieldClassification {
	if displayDocumentControl(f) {
		return applicationFieldClassification{Role: fieldDocument}
	}
	// Extended Lever-style pronouns collections cannot be represented as a
	// single profile fact or Yes/No answer. Keep the real question label and
	// observed options as manual review; never a writing task or Fill target.
	if pronounsRequiresManual(f) {
		return applicationFieldClassification{Role: fieldUnsupported}
	}
	if employerOnlyQuestion(f) {
		return applicationFieldClassification{Role: fieldEmployerOnly}
	}
	if key := profileContactKey(f); key != "" {
		return applicationFieldClassification{fieldCandidateProfile, key}
	}
	if key := additionalProfileAttribute(f); key != "" {
		return applicationFieldClassification{fieldCandidateProfile, key}
	}
	if !stableQuestionIdentity(f, unique) {
		return applicationFieldClassification{Role: fieldUnsupported}
	}
	switch f.Kind {
	case "text", "input_text", "email", "tel", "url", "textarea", "select", "checkbox", "radio", "multi_value_single_select":
		return applicationFieldClassification{Role: fieldApplicationQuestion}
	}
	return applicationFieldClassification{Role: fieldUnsupported}
}
func isProfileField(f schema.Field) bool {
	return classifyApplicationField(f, true).Role == fieldCandidateProfile
}
func additionalProfileAttribute(f schema.Field) string {
	id := profileQuestionIdentity(f.ID)
	if f.Sensitive || id == "" || positionalQuestionID.MatchString(id) || strings.HasPrefix(id, "unidentified-") {
		return ""
	}
	// Ordinary profile fields may have employer-generated opaque identities.
	label := candidateProfileLabel(f.Label)
	linkedin := label == "linkedin" || label == "linkedin url" || label == "linkedin profile" || label == "linkedin profile url" || label == "linkedin profile link" || label == "linkedin link" || label == "your linkedin profile" || label == "your linkedin url"
	if linkedin {
		id = "linkedin"
	}
	location := label == "location" || label == "current location" || label == "your location"
	if location && (f.Kind == "text" || f.Kind == "combobox") {
		return "location"
	}

	pronouns := label == "pronouns" || label == "preferred pronouns" || label == "what are your pronouns?" || label == "what are your pronouns"
	if pronouns && pronounsRequiresManual(f) {
		return ""
	}
	var value string
	urlField := false
	if pronouns {
		value = "pronouns"
	} else {
		switch id {
		case "city":
			if label == "city" {
				value = "city"
			}
		case "region", "state", "state_province":
			if label == "state" || label == "region" || label == "state / region" || label == "state/region" || label == "state/province" {
				value = "region"
			}
		case "country":
			if label == "country" {
				value = "country"
			}
		case "linkedin", "linkedin_url":
			urlField = true
			if linkedin {
				value = "linkedin"
			}
		case "github", "github_url":
			urlField = true
			if label == "github" || label == "github url" || label == "github profile" || label == "github profile url" {
				value = "github"
			}
		case "portfolio", "portfolio_url":
			urlField = true
			if label == "portfolio" || label == "portfolio url" || label == "portfolio website" || label == "portfolio or work sample url" {
				value = "portfolio"
			}
		}
	}
	if f.Kind == "text" || f.Kind == "textarea" || (urlField && f.Kind == "url") || (!urlField && (f.Kind == "select" || (pronouns && f.Kind == "radio"))) {
		return value
	}
	return ""
}

// Match the same explicit contact identities and compatible labels as the
// browser contact filler. A name belonging to a manager or reference is excluded.
func profileContactKey(f schema.Field) string {
	if f.Sensitive {
		return ""
	}
	id := profileQuestionIdentity(f.ID)
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

// writingCategory is a transport/presentation projection of role. Editability
// may restrict an employer question, but cannot change a profile field's role.
func writingCategory(f schema.Field, unique, editable bool) string {
	switch classifyApplicationField(f, unique).Role {
	case fieldCandidateProfile:
		return "profile"
	case fieldEmployerOnly:
		return "employer"
	case fieldApplicationQuestion:
		if editable {
			return "answer"
		}
	}
	return "review"
}
