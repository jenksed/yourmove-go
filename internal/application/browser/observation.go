// Package browser holds the read-only observation contract. No applicant values
// or executable selectors from an employer are stored in this representation.
package browser

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jenksed/yourmove-go/internal/application/flow"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

type Control struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	Kind      string   `json:"kind"`
	Required  bool     `json:"required"`
	Options   []string `json:"options"`
	Sensitive bool     `json:"sensitive"`
}
type Action struct {
	Label       string `json:"label"`
	Method      string `json:"method"`
	Destination string `json:"destination"`
}
type Frame struct {
	Host  string `json:"host"`
	Title string `json:"title"`
}
type Observation struct {
	URL        string    `json:"url"`
	Title      string    `json:"title"`
	Heading    string    `json:"heading"`
	ReadyState string    `json:"readyState"`
	Forms      int       `json:"forms"`
	Controls   []Control `json:"controls"`
	Actions    []Action  `json:"actions"`
	Frames     []Frame   `json:"frames"`
	Truncated  bool      `json:"truncated"`
}

// OwnsAshbyURL enforces an exact saved posting identity, even after the user
// navigates through application tabs and new same-posting windows.
func OwnsAshbyURL(saved, observed string) bool {
	s, e1 := url.Parse(saved)
	o, e2 := url.Parse(observed)
	if e1 != nil || e2 != nil || s.Scheme != "https" || o.Scheme != "https" || s.Host != "jobs.ashbyhq.com" || o.Host != s.Host || s.User != nil || o.User != nil || s.Port() != "" || o.Port() != "" {
		return false
	}
	part := strings.Split(strings.Trim(s.Path, "/"), "/")
	if len(part) != 2 || part[0] == "" || len(part[1]) != 36 {
		return false
	}
	base := "/" + part[0] + "/" + part[1]
	return o.Path == base || strings.HasPrefix(o.Path, base+"/")
}

// AsResult records only structural evidence from the currently rendered page.
// Never persist control values, cookies, request bodies, HTML, or resume bytes.
func AsResult(jobURL string, o Observation) (schema.Result, error) {
	if !OwnsPostingURL(jobURL, o.URL) {
		return schema.Result{}, errors.New("browser moved outside the saved job; no fields were collected")
	}
	if len(o.Controls) > 200 || len(o.Actions) > 60 || len(o.Frames) > 20 {
		return schema.Result{}, fmt.Errorf("browser inspection exceeds limits")
	}
	fields := make([]schema.Field, 0, len(o.Controls))
	for _, c := range o.Controls {
		if c.Label == "" {
			continue
		}
		if len(c.ID) > 250 || len(c.Label) > 500 || len(c.Kind) > 100 || len(c.Options) > 60 {
			return schema.Result{}, fmt.Errorf("invalid browser field")
		}
		fields = append(fields, schema.Field{ID: "browser:" + c.ID, Label: c.Label, Kind: c.Kind, Required: c.Required, Sensitive: c.Sensitive, Options: c.Options, Group: "rendered form"})
	}
	label := strings.TrimSpace(o.Heading)
	if label == "" {
		label = "Employer application"
	}
	if len(label) > 120 {
		label = label[:120]
	}
	steps := []schema.Step{{Key: "rendered:" + o.URL, Label: label, State: "observed", Action: "none"}}
	for i, a := range o.Actions {
		if len(a.Label) > 200 || len(a.Destination) > 2048 {
			continue
		}
		decision := flow.Classify(a.Label, a.Method, a.Destination, false)
		steps = append(steps, schema.Step{Key: fmt.Sprintf("action:%d", i), Label: a.Label, State: "unknown", Action: "human · " + string(decision.Kind)})
		if len(steps) == 30 {
			break
		}
	}
	note := fmt.Sprintf("Rendered read-only snapshot: %d visible controls, %d action candidates, %d embedded frames. Other pages and conditional states remain uninspected. No values were captured.", len(o.Controls), len(o.Actions), len(o.Frames))
	if len(o.Frames) > 0 {
		note += " Embedded frames require separate inspection; their controls are not verified."
	}
	if o.Truncated {
		note += " Observation exceeded a limit; additional controls may exist."
	}
	return schema.Result{Platform: PostingPlatform(jobURL), Source: "browser", Completeness: "rendered_partial", ApplicationURL: o.URL, Note: note, Fields: fields, Steps: steps}, nil
}

// AsPreviewResult records only the same structural snapshot used by ordinary
// browser inspection, under a separate source and identity namespace. A later
// live observation can reconcile only the exact DOM identity captured here.
func AsPreviewResult(jobURL string, o Observation) (schema.Result, error) {
	result, err := AsResult(jobURL, o)
	if err != nil {
		return schema.Result{}, err
	}
	result.Source = "preview"
	result.Completeness = "rendered_preview_partial"
	result.Note = "Read-only rendered preview. No controls were filled, files attached, consent clicked, or application submitted. This captures only the currently visible page; later and conditional steps remain uninspected. Opening the public employer page can still generate ordinary website network traffic and analytics. " + result.Note
	blockedText := strings.ToLower(o.Title + " " + o.Heading)
	for _, marker := range []string{"captcha", "verify you are human", "checking your browser", "sign in", "log in", "login", "authentication"} {
		if strings.Contains(blockedText, marker) {
			result.Note = "Read-only preview stopped at a possible authentication or anti-bot boundary. No interaction was attempted. Captured controls, if any, are preserved; continue with the normal rendered-form inspection path. " + result.Note
			break
		}
	}
	for i := range result.Fields {
		result.Fields[i].ID = "preview:" + strings.TrimPrefix(result.Fields[i].ID, "browser:")
	}
	return result, nil
}
