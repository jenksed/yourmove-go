package greenhouse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"example.com/yourmove-lite/internal/application/schema"
)

const maxResponse = 4 << 20

type Inspector struct{ client *http.Client }

func New(client *http.Client) *Inspector {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	return &Inspector{client: client}
}
func posting(raw string) (string, string, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || (u.Hostname() != "job-boards.greenhouse.io" && u.Hostname() != "boards.greenhouse.io") {
		return "", "", errors.New("unsupported Greenhouse URL")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 3 || parts[1] != "jobs" || !token(parts[0]) {
		return "", "", errors.New("invalid Greenhouse path")
	}
	id, e := strconv.ParseUint(parts[2], 10, 64)
	if e != nil || id == 0 {
		return "", "", errors.New("invalid Greenhouse job ID")
	}
	return parts[0], parts[2], nil
}
func token(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

type answerField struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Values []struct {
		Label string `json:"label"`
		Name  string `json:"name"`
	} `json:"values"`
}
type question struct {
	Label    string        `json:"label"`
	Required bool          `json:"required"`
	Fields   []answerField `json:"fields"`
}
type payload struct {
	Questions            []question      `json:"questions"`
	LocationQuestions    []question      `json:"location_questions"`
	Compliance           []question      `json:"compliance"`
	DemographicQuestions json.RawMessage `json:"demographic_questions"`
}

func (i *Inspector) Inspect(ctx context.Context, raw string) (schema.Result, error) {
	board, id, err := posting(raw)
	if err != nil {
		return schema.Result{}, err
	}
	endpoint := "https://boards-api.greenhouse.io/v1/boards/" + url.PathEscape(board) + "/jobs/" + url.PathEscape(id) + "?questions=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return schema.Result{}, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := i.client.Do(req)
	if err != nil {
		return schema.Result{}, fmt.Errorf("Greenhouse API request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return schema.Result{}, fmt.Errorf("Greenhouse API HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return schema.Result{}, err
	}
	if len(data) > maxResponse {
		return schema.Result{}, errors.New("Greenhouse schema response too large")
	}
	var p payload
	if err := json.Unmarshal(data, &p); err != nil {
		return schema.Result{}, fmt.Errorf("decode Greenhouse questions: %w", err)
	}
	result := schema.Result{Platform: "greenhouse", Source: "public_api", Completeness: "public_schema", ApplicationURL: raw, Note: "Public question definitions only. Conditional fields, custom controls and later screens are not verified.", Fields: []schema.Field{}, Steps: []schema.Step{{Key: "public-application", Label: "Public application requirements", State: "anticipated", Action: "inspect"}}}
	seen := map[string]bool{}
	appendQuestions := func(group string, questions []question) {
		for _, q := range questions {
			for _, field := range q.Fields {
				if strings.TrimSpace(field.Name) == "" {
					continue
				}
				key := group + ":" + field.Name
				if seen[key] {
					continue
				}
				seen[key] = true
				options := make([]string, 0, len(field.Values))
				for _, v := range field.Values {
					if v.Label != "" {
						options = append(options, v.Label)
					} else if v.Name != "" {
						options = append(options, v.Name)
					}
				}
				sensitive := group == "compliance" || group == "demographic" || strings.Contains(strings.ToLower(q.Label), "sponsorship") || strings.Contains(strings.ToLower(q.Label), "authorization") || strings.Contains(strings.ToLower(q.Label), "consent")
				result.Fields = append(result.Fields, schema.Field{ID: key, Label: strings.TrimSpace(q.Label), Kind: field.Type, Required: q.Required, Sensitive: sensitive, Options: options, Group: group})
			}
		}
	}
	appendQuestions("application", p.Questions)
	appendQuestions("location", p.LocationQuestions)
	appendQuestions("compliance", p.Compliance)
	// Demographic questions have a distinct shape; do not pretend they were decoded.
	if len(p.DemographicQuestions) > 0 && string(p.DemographicQuestions) != "null" {
		result.Note += " Demographic question definitions require separate human review."
	}
	if len(result.Fields) == 0 {
		result.Completeness = "partial"
		result.Note = "API returned no recognizable public application fields; actual form inspection is still required."
	}
	return result, nil
}
