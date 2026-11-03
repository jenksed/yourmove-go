package chromium

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/jenksed/yourmove-go/internal/application/browser"
)

// FillAnswer reuses the read-only observation as the live identity contract.
// Each interaction resolves afresh; a changed field fails locally, leaving all
// other fields and documents alone.
func (s *Session) FillAnswer(ctx context.Context, answer browser.AnswerValue) (browser.AnswerOutcome, error) {
	out := browser.AnswerOutcome{Label: answer.Field.Label}
	c, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	if err := s.lockContext(c); err != nil {
		return out, err
	}
	defer s.mu.Unlock()
	if s.closed {
		return out, errors.New("browser session closed")
	}
	if answer.Field.Sensitive || answer.Value == "" || !strings.HasPrefix(answer.Field.ID, "browser:") {
		out.Status = "manual"
		out.Detail = "This question needs manual entry."
		return out, nil
	}
	target, err := s.target(c)
	if err != nil {
		return out, err
	}
	ws, err := dialCDP(c, target.WebSocketURL)
	if err != nil {
		return out, err
	}
	defer ws.Close(websocket.StatusNormalClosure, "")
	saved, err := url.Parse(s.postingURL)
	if err != nil {
		return out, err
	}
	args, _ := json.Marshal([]any{saved.Scheme + "://" + saved.Host, strings.TrimRight(saved.Path, "/"), answer})
	const operation = browser.AnswerOperation
	script := strings.Replace(operation, "OBSERVATION", browser.ReadOnlyScript, 1)
	var response struct {
		Result struct {
			Value browser.AnswerOutcome `json:"value"`
		} `json:"result"`
		Exception json.RawMessage `json:"exceptionDetails"`
	}
	err = cdpCall(c, ws, 1, "Runtime.evaluate", map[string]any{"expression": script + "(..." + string(args) + ")", "awaitPromise": true, "returnByValue": true, "silent": true}, &response)
	if err != nil || len(response.Exception) > 0 {
		return out, errors.New("answer transfer could not be verified; review this field in Chromium")
	}
	out.Status = response.Result.Value.Status
	out.Detail = response.Result.Value.Detail
	return out, nil
}
