package browser

import (
	"errors"
	"regexp"
	"strings"
)

// PreflightError is emitted only before DOM.setFileInputFiles has been sent.
// This makes bounded retries safe without risking duplicate uploads. All
// errors after that command remain ambiguous and require human inspection.
type PreflightError struct{ Cause error }

func (e *PreflightError) Error() string { return "document form preflight: " + e.Cause.Error() }
func (e *PreflightError) Unwrap() error { return e.Cause }

var ErrNoResumeInput = errors.New("resume input not present on this application step")
var ErrAmbiguousResumeInput = errors.New("resume upload control cannot be identified safely; attach the file manually in Chromium")

type AttachmentResult struct {
	Selected bool
	Verified bool
	Detail   string
}
type FileInput struct {
	Label    string `json:"label"`
	Accept   string `json:"accept"`
	Existing bool   `json:"existing"`
	Parser   bool   `json:"parser"`
	Unsafe   bool   `json:"unsafe"`
	Identity string `json:"identity"`
}

const FileInputSurvey = `(()=>[...document.querySelectorAll('input[type="file"]')].map(el=>{
 const group=el.closest('[data-field-entry-id]');
 const title=group?.querySelector('.ashby-application-form-question-title')?.innerText||el.closest('.application-question')?.querySelector('.application-label')?.innerText;
 const semantic=title||el.labels?.[0]?.innerText||el.getAttribute('aria-label');
 const label=semantic||el.getAttribute('data-testid')||el.name||el.id||el.parentElement?.innerText?.slice(0,100)||'';
 // Hosted Greenhouse and Lever file inputs can be visually hidden behind an
 // Attach button. Their explicit machine identifiers still name the purpose.
 const purposes={resume:'Resume',resumeupload:'Resume',resumeuploadinput:'Resume',coverletter:'Cover Letter',coverletterupload:'Cover Letter',coverletteruploadinput:'Cover Letter'};
 const purpose=!semantic&&purposes[String(el.name||el.id).toLowerCase().replace(/[\s_-]/g,'')];
 const clean=String(label).trim().replace(/[\s*✱]+$/g,'');
 return {label:purpose||clean.slice(0,450),accept:String(el.accept||''),existing:el.files?.length>0,identity:[el.id,el.name,group?.getAttribute('data-field-entry-id')].join('|'),unsafe:el.matches(':disabled')||!el.closest('form,.ashby-application-form-container')||!!el.closest('[inert],[aria-hidden=true]'),parser:!!el.closest('.ashby-application-form-autofill-input-root,.awli-application-row,.awli-button-container')};
}))()`

var resumeWord = regexp.MustCompile(`(?i)(resume|résumé|curriculum[\s_-]*vitae|\bcv\b)`)
var coverWord = regexp.MustCompile(`(?i)(cover[\s_-]*letter|portfolio|transcript|photo)`)

// ChooseResumeInput abstains on ambiguous destinations and never overwrites a
// user-supplied file already present in the employer form.
func ChooseResumeInput(inputs []FileInput, extension string) (int, error) {
	return ChooseDocumentInput(inputs, extension, false)
}

var ErrDocumentFormat = errors.New("employer document input does not accept the selected document format")
var ErrExistingDocument = errors.New("a document is already attached in the employer form; review it manually instead of overwriting")
var ErrAmbiguousCoverInput = errors.New("cover-letter upload control cannot be identified safely")
var ErrNoCoverInput = errors.New("cover-letter file input not present on this application step")
var letterWord = regexp.MustCompile(`(?i)^cover[\s_-]*letter\s*(?:\((?:optional|required)\))?\s*\*?$`)

func ChooseDocumentInput(inputs []FileInput, extension string, cover bool) (int, error) {
	if len(inputs) == 0 {
		if cover {
			return -1, ErrNoCoverInput
		}
		return -1, ErrNoResumeInput
	}
	hits := []int{}
	for i, in := range inputs {
		match := resumeWord.MatchString(in.Label) && !coverWord.MatchString(in.Label)
		if cover {
			match = letterWord.MatchString(strings.TrimSpace(in.Label))
		}
		if !in.Parser && match {
			hits = append(hits, i)
		}
	}
	if cover && len(hits) == 0 {
		return -1, ErrNoCoverInput
	}
	if len(hits) != 1 {
		if cover {
			return -1, ErrAmbiguousCoverInput
		}
		return -1, ErrAmbiguousResumeInput
	}
	if inputs[hits[0]].Unsafe {
		return -1, errors.New("document control is unavailable; attach manually in Chromium")
	}
	if inputs[hits[0]].Existing {
		return -1, ErrExistingDocument
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
			return -1, ErrDocumentFormat
		}
	}
	return hits[0], nil
}

func SameFileInputs(a, b []FileInput) bool {
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
