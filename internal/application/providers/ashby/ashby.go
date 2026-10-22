package ashby

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

const maxResponse = 32 << 20

type Inspector struct{ client *http.Client }

func New(client *http.Client) *Inspector {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	return &Inspector{client: client}
}
func posting(raw string) (string, string, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Hostname() != "jobs.ashbyhq.com" {
		return "", "", errors.New("unsupported Ashby URL")
	}
	p := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(p) != 2 || !token(p[0]) || !uuid(p[1]) {
		return "", "", errors.New("invalid Ashby posting path")
	}
	return p[0], p[1], nil
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
func uuid(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !((c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}
func (i *Inspector) Inspect(ctx context.Context, raw string) (schema.Result, error) {
	board, id, err := posting(raw)
	if err != nil {
		return schema.Result{}, err
	}
	// The publicly documented posting API has no applicationFormDefinition.
	// Do not use undocumented Ashby GraphQL or claim that question discovery succeeded.
	endpoint := "https://api.ashbyhq.com/posting-api/job-board/" + url.PathEscape(board)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return schema.Result{}, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := i.client.Do(req)
	if err != nil {
		return schema.Result{}, fmt.Errorf("Ashby public board: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return schema.Result{}, fmt.Errorf("Ashby board API HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return schema.Result{}, err
	}
	if len(data) > maxResponse {
		return schema.Result{}, errors.New("Ashby board exceeds bounded limit")
	}
	var doc struct {
		Jobs []struct {
			ID       string `json:"id"`
			JobURL   string `json:"jobUrl"`
			ApplyURL string `json:"applyUrl"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return schema.Result{}, fmt.Errorf("decode Ashby board: %w", err)
	}
	for _, j := range doc.Jobs {
		found := j.ID == id
		if !found && j.JobURL != "" {
			if u, err := url.Parse(j.JobURL); err == nil && u.Hostname() == "jobs.ashbyhq.com" {
				parts := strings.Split(strings.Trim(u.Path, "/"), "/")
				found = len(parts) == 2 && parts[1] == id
			}
		}
		if !found {
			continue
		}
		apply := j.ApplyURL
		if apply != "" {
			if u, err := url.Parse(apply); err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" {
				apply = ""
			}
		}
		if apply == "" {
			apply = raw
		}
		return schema.Result{Platform: "ashby", Source: "public_api", Completeness: "partial", ApplicationURL: apply, Note: "Ashby's documented public posting API does not expose application questions. The application and possible later steps require rendered, read-only inspection; no employer fields were inferred.", Fields: []schema.Field{}, Steps: []schema.Step{{Key: "posting", Label: "Job posting", State: "observed", Action: "none"}, {Key: "application", Label: "Application form (uninspected)", State: "anticipated", Action: "inspect"}, {Key: "conditional", Label: "Conditional or subsequent steps (unknown)", State: "unknown", Action: "human"}}}, nil
	}
	return schema.Result{}, errors.New("Ashby posting was not found on its public job board")
}
