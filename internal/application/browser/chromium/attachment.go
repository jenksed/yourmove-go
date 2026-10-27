package chromium

// This is the shared purpose-qualified attachment primitive. A
// user-authorized, integrity-checked document is selected into one identified file
// input. No form navigation or submission occurs here.
import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/coder/websocket"
	"github.com/jenksed/yourmove-go/internal/application/browser"
)

// PreflightError, AttachmentResult, and the document-selection sentinels are
// owned by the browser package so every engine honors the same rules.
type PreflightError = browser.PreflightError

type AttachmentResult = browser.AttachmentResult

type fileInput = browser.FileInput

const fileInputSurvey = browser.FileInputSurvey

var ErrNoResumeInput = browser.ErrNoResumeInput
var ErrAmbiguousResumeInput = browser.ErrAmbiguousResumeInput

var ErrDocumentFormat = browser.ErrDocumentFormat
var ErrExistingDocument = browser.ErrExistingDocument
var ErrAmbiguousCoverInput = browser.ErrAmbiguousCoverInput
var ErrNoCoverInput = browser.ErrNoCoverInput

// chooseResumeInput abstains on ambiguous destinations and never overwrites a
// user-supplied file already present in the employer form.
func chooseResumeInput(inputs []fileInput, extension string) (int, error) {
	return browser.ChooseResumeInput(inputs, extension)
}

func chooseDocumentInput(inputs []fileInput, extension string, cover bool) (int, error) {
	return browser.ChooseDocumentInput(inputs, extension, cover)
}

func sameFileInputs(a, b []fileInput) bool { return browser.SameFileInputs(a, b) }

// cdpCall correlates each response by ID. Chrome may emit unrelated events.
func cdpCall(ctx context.Context, ws *websocket.Conn, id int, method string, params any, dst any) error {
	payload, _ := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
	if err := ws.Write(ctx, websocket.MessageText, payload); err != nil {
		return err
	}
	for {
		_, body, err := ws.Read(ctx)
		if err != nil {
			return err
		}
		if int64(len(body)) > maxCDPMessageBytes {
			return errors.New("browser response too large")
		}
		var v struct {
			ID    int `json:"id"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
			Result json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(body, &v); err != nil || v.ID != id {
			continue
		}
		if v.Error != nil {
			return fmt.Errorf("browser %s: %s", method, v.Error.Message)
		}
		if dst != nil {
			return json.Unmarshal(v.Result, dst)
		}
		return nil
	}
}
func evaluateRead(ctx context.Context, ws *websocket.Conn, id int, expression string, out any) error {
	var result struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		Exception json.RawMessage `json:"exceptionDetails"`
	}
	if err := cdpCall(ctx, ws, id, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true, "awaitPromise": true, "silent": true}, &result); err != nil {
		return err
	}
	if len(result.Exception) > 0 || len(result.Result.Value) == 0 {
		return errors.New("browser cannot inspect the current control")
	}
	return json.Unmarshal(result.Result.Value, out)
}

func (s *Session) AttachResume(ctx context.Context, path string) (AttachmentResult, error) {
	return s.attachDocument(ctx, path, false, "")
}

// AttachCoverLetter uses the same serialized, purpose-qualified file operation.
// The expected digest pins the prepared bytes through the mutation boundary.
func (s *Session) AttachCoverLetter(ctx context.Context, path, digest string) (AttachmentResult, error) {
	return s.attachDocument(ctx, path, true, digest)
}

func (s *Session) attachDocument(ctx context.Context, path string, cover bool, digest string) (AttachmentResult, error) {
	c, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	if err := s.lockContext(c); err != nil {
		return AttachmentResult{}, err
	}
	defer s.mu.Unlock()
	if s.closed {
		return AttachmentResult{}, errors.New("browser session closed")
	}
	if !filepath.IsAbs(path) {
		return AttachmentResult{}, errors.New("resume file path must be absolute")
	}
	target, err := s.target(c)
	if err != nil {
		return AttachmentResult{}, err
	}
	ws, err := dialCDP(c, target.WebSocketURL)
	if err != nil {
		return AttachmentResult{}, &PreflightError{Cause: err}
	}
	defer ws.Close(websocket.StatusNormalClosure, "")
	var inputs []fileInput
	if err := evaluateRead(c, ws, 1, fileInputSurvey, &inputs); err != nil {
		return AttachmentResult{}, &PreflightError{Cause: err}
	}
	idx, err := chooseDocumentInput(inputs, filepath.Ext(path), cover)
	if err != nil {
		return AttachmentResult{}, err
	}
	// Inspect CDP DOM nodes, rather than generating JavaScript to set files.
	var doc struct {
		Root struct {
			NodeID int `json:"nodeId"`
		} `json:"root"`
	}
	if err := cdpCall(c, ws, 2, "DOM.getDocument", map[string]any{"depth": 1, "pierce": false}, &doc); err != nil {
		return AttachmentResult{}, &PreflightError{Cause: err}
	}
	// Reinspect immediately before mutation: rerenders can replace or reorder
	// Ashby's file inputs between the initial survey and selection.
	var current []fileInput
	if err := evaluateRead(c, ws, 3, fileInputSurvey, &current); err != nil {
		return AttachmentResult{}, &PreflightError{Cause: err}
	}
	if !sameFileInputs(inputs, current) {
		return AttachmentResult{}, &PreflightError{Cause: errors.New("resume controls changed before file selection")}
	}
	// Resolve just the chosen DOM node. DOM.querySelectorAll only searches
	// nodes pushed into the CDP DOM tree; using it with shallow getDocument
	// returns nothing. Runtime objectId + DOM.requestNode avoids transferring
	// the entire subtree and was validated with a real Chromium process.
	var selected struct {
		Result struct {
			Type     string `json:"type"`
			Subtype  string `json:"subtype"`
			ObjectID string `json:"objectId"`
		} `json:"result"`
	}
	expr := fmt.Sprintf("document.querySelectorAll('input[type=file]')[%d]", idx)
	if err := cdpCall(c, ws, 4, "Runtime.evaluate", map[string]any{"expression": expr, "returnByValue": false, "silent": true}, &selected); err != nil {
		return AttachmentResult{}, &PreflightError{Cause: err}
	}
	if selected.Result.Type != "object" || selected.Result.Subtype != "node" || selected.Result.ObjectID == "" {
		return AttachmentResult{}, &PreflightError{Cause: errors.New("resume input disappeared before selection")}
	}
	var node struct {
		NodeID int `json:"nodeId"`
	}
	if err := cdpCall(c, ws, 5, "DOM.requestNode", map[string]any{"objectId": selected.Result.ObjectID}, &node); err != nil {
		return AttachmentResult{}, &PreflightError{Cause: err}
	}
	if node.NodeID <= 0 {
		return AttachmentResult{}, &PreflightError{Cause: errors.New("unable to resolve the current resume input")}
	}
	owns := func(candidate string) bool { return browser.OwnsPostingURL(s.postingURL, candidate) }
	if s.fixtureOwner != nil {
		owns = s.fixtureOwner
	}
	var currentURL string
	if err := evaluateRead(c, ws, 8, "location.href", &currentURL); err != nil || !owns(currentURL) {
		return AttachmentResult{}, errors.New("application changed before document selection; review the destination")
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return AttachmentResult{}, errors.New("prepared document is unavailable")
	}
	if cover && fmt.Sprintf("%x", sha256.Sum256(data)) != digest {
		return AttachmentResult{}, errors.New("prepared cover letter changed; review it again")
	}
	// Any error after this command may be an ambiguous employer upload.
	// No automatic replay unless a person confirms the field is empty.
	if err := cdpCall(c, ws, 6, "DOM.setFileInputFiles", map[string]any{"nodeId": node.NodeID, "files": []string{path}}, nil); err != nil {
		return AttachmentResult{}, fmt.Errorf("file selection outcome unverified: %w", err)
	}
	// A successful CDP call is only evidence of file selection, not server receipt.
	out := AttachmentResult{Selected: true, Detail: "File selected in employer browser; server acceptance not confirmed"}
	var confirmed struct {
		Name   string `json:"name"`
		Size   int64  `json:"size"`
		Digest string `json:"digest"`
		URL    string `json:"url"`
	}
	verifyExpr := fmt.Sprintf(`(()=>{const f=document.querySelectorAll('input[type="file"]')[%d]?.files?.[0];return {url:location.href,name:f?.name||"",size:f?.size||0}})()`, idx)
	if cover {
		verifyExpr = fmt.Sprintf(`(async()=>{const e=document.querySelectorAll('input[type="file"]')[%d];const f=e?.files?.[0];if(!f)return {};const hash=await crypto.subtle.digest('SHA-256',await f.arrayBuffer());return {url:location.href,name:f.name,size:f.size,digest:[...new Uint8Array(hash)].map(x=>x.toString(16).padStart(2,'0')).join('')}})()`, idx)
	}
	if err := evaluateRead(c, ws, 7, verifyExpr, &confirmed); err == nil && confirmed.Name == filepath.Base(path) && confirmed.Size == int64(len(data)) && owns(confirmed.URL) && (!cover || confirmed.Digest == digest) {
		out.Verified = true
		out.Detail = "Exact named file present in employer input; employer upload acceptance still unverified"
	}
	return out, nil
}
