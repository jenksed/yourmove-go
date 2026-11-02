package browser

import (
	"errors"
	"regexp"
	"strings"
)

// FileInput is one employer file control observed by the read-only survey.
// It carries no file bytes or user values.
type FileInput struct {
	Label    string `json:"label"`
	Accept   string `json:"accept"`
	Existing bool   `json:"existing"`
	Parser   bool   `json:"parser"`
	Unsafe   bool   `json:"unsafe"`
	Identity string `json:"identity"`
}

// FileInputSurvey is engine-agnostic DOM JavaScript: it lists visible file
// inputs with their purpose labels. Every driver evaluates the same source.
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
var letterWord = regexp.MustCompile(`(?i)^cover[\s_-]*letter\s*(?:\((?:optional|required)\))?\s*\*?$`)

// ChooseResumeInput abstains on ambiguous destinations and never overwrites a
// user-supplied file already present in the employer form.
func ChooseResumeInput(inputs []FileInput, extension string) (int, error) {
	return ChooseDocumentInput(inputs, extension, false)
}

// ChooseDocumentInput selects one purpose-qualified file input. Errors are the
// shared document sentinels so every engine honors the same authorization
// and retry rules.
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
		return -1, errors.New("document control is unavailable; attach manually in the employer browser")
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

// SameFileInputs reports whether a rerender changed the surveyed controls
// between preflight reads. Any change aborts selection before mutation.
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
