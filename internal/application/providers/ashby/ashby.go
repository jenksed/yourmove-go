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
	"github.com/jenksed/yourmove-go/internal/platforms"
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
	identity, err := platforms.AshbyPostingIdentity(raw)
	if err != nil {
		return "", "", err
	}
	return identity.Board, identity.PostingID, nil
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
	type postingRecord struct {
		ID       string          `json:"id"`
		JobURL   string          `json:"jobUrl"`
		ApplyURL json.RawMessage `json:"applyUrl"`
	}
	var doc struct {
		Jobs []postingRecord `json:"jobs"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return schema.Result{}, fmt.Errorf("decode Ashby board: %w", err)
	}
	selected := -1
	for index, candidate := range doc.Jobs {
		matched, err := postingRecordMatches(board, id, candidate.ID, candidate.JobURL)
		if err != nil {
			return schema.Result{}, err
		}
		if matched {
			if selected >= 0 {
				return schema.Result{}, fmt.Errorf("Ashby posting identity is ambiguous on board %q", board)
			}
			selected = index
		}
	}
	if selected < 0 {
		return schema.Result{}, errors.New("Ashby posting was not found on its public job board")
	}
	job := doc.Jobs[selected]
	apply := optionalURLString(job.ApplyURL)
	if !matchesApplicationDestination(board, id, apply) {
		apply = raw
	}
	return schema.Result{Platform: "ashby", Source: "public_api", Completeness: "partial", ApplicationURL: apply, Note: "Ashby's documented public posting API does not expose application questions. The application and possible later steps require rendered, read-only inspection; no employer fields were inferred.", Fields: []schema.Field{}, Steps: []schema.Step{{Key: "posting", Label: "Job posting", State: "observed", Action: "none"}, {Key: "application", Label: "Application form (uninspected)", State: "anticipated", Action: "inspect"}, {Key: "conditional", Label: "Conditional or subsequent steps (unknown)", State: "unknown", Action: "human"}}}, nil
}

func optionalURLString(raw json.RawMessage) string {
	var value string
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return value
}

func postingRecordMatches(requestedBoard, requestedID, returnedID, returnedURL string) (bool, error) {
	idMatches := returnedID != "" && returnedID == requestedID
	if returnedURL == "" {
		if idMatches {
			return false, fmt.Errorf("Ashby selected posting %s has an ID but no qualified jobUrl", requestedID)
		}
		return false, nil
	}
	returnedBoard, returnedPostingID, err := posting(returnedURL)
	if err != nil {
		if idMatches || ashbyURLHasPostingID(returnedURL, requestedID) {
			return false, fmt.Errorf("Ashby selected posting has an invalid jobUrl: %w", err)
		}
		return false, nil
	}
	urlHasRequestedID := returnedPostingID == requestedID
	urlMatches := returnedBoard == requestedBoard && urlHasRequestedID
	if (idMatches && !urlMatches) || (urlHasRequestedID && returnedBoard != requestedBoard) {
		return false, fmt.Errorf("Ashby selected posting ID/jobUrl conflicts with requested board %q and posting %q", requestedBoard, requestedID)
	}
	if urlMatches && returnedID != "" && returnedID != requestedID {
		return false, fmt.Errorf("Ashby selected posting id %q conflicts with matching jobUrl", returnedID)
	}
	return urlMatches, nil
}

func ashbyURLHasPostingID(rawURL, postingID string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	return len(parts) == 2 && parts[1] == postingID
}
