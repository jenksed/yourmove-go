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

// FillCoverLetter handles only an explicitly labeled native cover-letter text
// control. Existing entries are authoritative; no upload fallback is attempted.
func (s *Session) FillCoverLetter(ctx context.Context, body string) (AttachmentResult, error) {
	c, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	if err := s.lockContext(c); err != nil {
		return AttachmentResult{}, err
	}
	defer s.mu.Unlock()
	if s.closed {
		return AttachmentResult{}, errors.New("browser session closed")
	}
	target, err := s.target(c)
	if err != nil {
		return AttachmentResult{}, err
	}
	ws, err := dialCDP(c, target.WebSocketURL)
	if err != nil {
		return AttachmentResult{}, err
	}
	defer ws.Close(websocket.StatusNormalClosure, "")
	saved, err := url.Parse(s.postingURL)
	if err != nil {
		return AttachmentResult{}, err
	}
	args, _ := json.Marshal([]string{saved.Scheme + "://" + saved.Host, strings.TrimRight(saved.Path, "/"), body})
	// Re-resolve after events, preserving React rerenders and conditional changes.
	const operation = browser.CoverLetterOperation
	var remote struct {
		Result struct {
			Value AttachmentResult `json:"value"`
		} `json:"result"`
		Exception json.RawMessage `json:"exceptionDetails"`
	}
	err = cdpCall(c, ws, 1, "Runtime.evaluate", map[string]any{"expression": operation + "(..." + string(args) + ")", "returnByValue": true, "awaitPromise": true, "silent": true}, &remote)
	if err != nil || len(remote.Exception) > 0 {
		return AttachmentResult{}, errors.New("cover-letter transfer could not be verified; inspect the employer form before continuing")
	}
	return remote.Result.Value, nil
}
