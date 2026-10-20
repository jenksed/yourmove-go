package chromium

// This is the only employer-page mutation in the resume-handoff slice. A
// user-authorized, SHA-verified resume is selected into one identified file
// input. No form navigation, text filling, or submission occurs here.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// PreflightError is emitted only before DOM.setFileInputFiles has been sent.
// This makes bounded retries safe without risking duplicate uploads. All
// errors after that command remain ambiguous and require human inspection.
type PreflightError struct{ Cause error }

func (e *PreflightError) Error() string { return "resume form preflight: " + e.Cause.Error() }
func (e *PreflightError) Unwrap() error { return e.Cause }

var ErrNoResumeInput = errors.New("resume input not present on this application step")
var ErrAmbiguousResumeInput = errors.New("resume upload control cannot be identified safely; attach the file manually in Chromium")

type AttachmentResult struct {
	Selected bool
	Verified bool
	Detail   string
}
type fileInput struct {
	Label    string `json:"label"`
	Accept   string `json:"accept"`
	Existing bool   `json:"existing"`
	Parser   bool   `json:"parser"`
}

const fileInputSurvey = `(()=>[...document.querySelectorAll('input[type="file"]')].map(el=>{const group=el.closest('[data-field-entry-id]');const title=group?.querySelector('.ashby-application-form-question-title')?.innerText;const label=title||el.labels?.[0]?.innerText||el.getAttribute('aria-label')||el.getAttribute('data-testid')||el.name||el.id||el.parentElement?.innerText?.slice(0,100)||'';return {label:String(label).trim().slice(0,450),accept:String(el.accept||''),existing:el.files?.length>0,parser:!!el.closest('.ashby-application-form-autofill-input-root')}}))()`

var resumeWord = regexp.MustCompile(`(?i)(resume|résumé|curriculum[\s_-]*vitae|\bcv\b)`)
var coverWord = regexp.MustCompile(`(?i)(cover[\s_-]*letter|portfolio|transcript|photo)`)

// chooseResumeInput abstains on ambiguous destinations and never overwrites a
// user-supplied file already present in the employer form.
func chooseResumeInput(inputs []fileInput, extension string) (int, error) {
	if len(inputs) == 0 {
		return -1, ErrNoResumeInput
	}
	hits := []int{}
	for i, in := range inputs {
		if !in.Parser && resumeWord.MatchString(in.Label) && !coverWord.MatchString(in.Label) {
			hits = append(hits, i)
		}
	}
	if len(hits) != 1 {
		return -1, ErrAmbiguousResumeInput
	}
	if inputs[hits[0]].Existing {
		return -1, errors.New("a resume is already attached in the employer form; review it manually instead of overwriting")
	}
	accept := strings.ToLower(inputs[hits[0]].Accept)
	if accept != "" && accept != "*" && accept != "*/*" {
		extension = strings.ToLower(extension)
		compatible := false
		for _, part := range strings.Split(accept, ",") {
			part = strings.TrimSpace(part)
			if part == extension || (extension == ".pdf" && part == "application/pdf") || (extension == ".docx" && (part == "application/vnd.openxmlformats-officedocument.wordprocessingml.document" || part == "application/msword")) {
				compatible = true
			}
		}
		if !compatible {
			return -1, errors.New("employer resume input does not accept the selected document format")
		}
	}
	return hits[0], nil
}

func sameFileInputs(a, b []fileInput) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

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
	if err := cdpCall(ctx, ws, id, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true, "silent": true}, &result); err != nil {
		return err
	}
	if len(result.Exception) > 0 || len(result.Result.Value) == 0 {
		return errors.New("browser cannot inspect the current resume control")
	}
	return json.Unmarshal(result.Result.Value, out)
}

func (s *Session) AttachResume(ctx context.Context, path string) (AttachmentResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return AttachmentResult{}, errors.New("browser session closed")
	}
	if !filepath.IsAbs(path) {
		return AttachmentResult{}, errors.New("resume file path must be absolute")
	}
	c, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
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
	idx, err := chooseResumeInput(inputs, filepath.Ext(path))
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
	// Any error after this command may be an ambiguous employer upload.
	// No automatic replay unless a person confirms the field is empty.
	if err := cdpCall(c, ws, 6, "DOM.setFileInputFiles", map[string]any{"nodeId": node.NodeID, "files": []string{path}}, nil); err != nil {
		return AttachmentResult{}, fmt.Errorf("file selection outcome unverified: %w", err)
	}
	// A successful CDP call is only evidence of file selection, not server receipt.
	out := AttachmentResult{Selected: true, Detail: "File selected in employer browser; server acceptance not confirmed"}
	var confirmed struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}
	verifyExpr := fmt.Sprintf(`(()=>{const f=document.querySelectorAll('input[type="file"]')[%d]?.files?.[0];return {name:f?.name||"",size:f?.size||0}})()`, idx)
	if err := evaluateRead(c, ws, 7, verifyExpr, &confirmed); err == nil && confirmed.Name == filepath.Base(path) && confirmed.Size > 0 {
		out.Verified = true
		out.Detail = "Exact named file present in employer input; employer upload acceptance still unverified"
	}
	return out, nil
}
