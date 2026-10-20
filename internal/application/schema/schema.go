// Package schema defines read-only application requirements, independent of ATS clients.
package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Field describes an employer question, not an authorized answer or DOM selector.
type Field struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	Kind      string   `json:"kind"`
	Required  bool     `json:"required"`
	Sensitive bool     `json:"sensitive,omitempty"`
	Options   []string `json:"options,omitempty"`
	Group     string   `json:"group,omitempty"`
}

// Step is an observed or anticipated state of the application journey.
// An anticipated step is never evidence that the form was actually rendered.
type Step struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	State  string `json:"state"`  // observed, anticipated, unknown
	Action string `json:"action"` // inspect, human, none
}

type Result struct {
	Platform       string  `json:"platform"`
	Source         string  `json:"source"`       // public_api, public_page, browser
	Completeness   string  `json:"completeness"` // public_schema, partial, unavailable
	ApplicationURL string  `json:"application_url,omitempty"`
	Note           string  `json:"note"`
	Fields         []Field `json:"fields"`
	Steps          []Step  `json:"steps"`
}

func (r Result) Fingerprint() string {
	b, _ := json.Marshal(struct {
		Platform       string
		Source         string
		Completeness   string
		ApplicationURL string
		Fields         []Field
		Steps          []Step
	}{r.Platform, r.Source, r.Completeness, r.ApplicationURL, r.Fields, r.Steps})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
