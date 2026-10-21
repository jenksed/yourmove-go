package browser

import (
	"example.com/yourmove-lite/internal/application/schema"
	"fmt"
)

// ContactValues is the displayed, approved snapshot carried by a handoff.
// Inspection IDs and employer-provided labels are never transfer authority.
type ContactValues struct {
	Revision  int64
	FirstName string
	LastName  string
	Email     string
	Phone     string
}

func (v ContactValues) Value(key string) string {
	switch key {
	case "name":
		return v.FirstName + " " + v.LastName
	case "first_name":
		return v.FirstName
	case "last_name":
		return v.LastName
	case "email":
		return v.Email
	case "phone":
		return v.Phone
	}
	return ""
}

// ContactOutcome contains no candidate values; it can safely be shown in status.
type ContactOutcome struct {
	Key    string `json:"key"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

func (o ContactOutcome) Label() string {
	switch o.Key {
	case "name":
		return "Name"
	case "first_name":
		return "First name"
	case "last_name":
		return "Last name"
	case "email":
		return "Email"
	case "phone":
		return "Phone"
	}
	return "Contact field"
}

type ContactResult struct {
	Ready  bool             `json:"ready"`
	Fields []ContactOutcome `json:"fields"`
}

func (r ContactResult) Summary() string {
	filled, correct, attention := 0, 0, 0
	for _, field := range r.Fields {
		switch field.Status {
		case "filled":
			filled++
		case "correct":
			correct++
		case "skipped":
		default:
			attention++
		}
	}
	summary := fmt.Sprintf("%d contact fields filled.", filled)
	if correct > 0 {
		summary += fmt.Sprintf(" %d already correct.", correct)
	}
	if attention > 0 {
		summary += fmt.Sprintf(" %d need your attention.", attention)
	}
	return summary
}

// AnswerValue contains only an explicitly prepared application answer. Field
// metadata is revalidated against the live DOM before any interaction.
type AnswerValue struct {
	Field schema.Field `json:"field"`
	Value string       `json:"value"`
}

// AnswerOutcome reports reconciliation without retaining the approved answer.
type AnswerOutcome struct {
	Label  string `json:"label"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}
