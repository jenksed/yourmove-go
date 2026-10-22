package app

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// A prior implementation nested a whole section and multiple forms inside
// a button. WebKit silently repaired the DOM and controls behaved unpredictably.
func TestInspectionTemplateHasIndependentFormsAndButtons(t *testing.T) {
	data, err := os.ReadFile("../../templates/inspection.html")
	if err != nil {
		t.Fatal(err)
	}
	tokens := regexp.MustCompile(`(?i)</?(?:button|form|section|header|div)\b[^>]*>`).FindAllString(string(data), -1)
	forms, buttons := 0, 0
	for _, token := range tokens {
		closing := strings.HasPrefix(token, "</")
		typ := strings.ToLower(strings.Fields(strings.Trim(token, "<>/"))[0])
		if !closing {
			if buttons > 0 && (typ == "section" || typ == "form" || typ == "header" || typ == "div") {
				t.Fatalf("%s opened inside a button", typ)
			}
			if typ == "form" {
				forms++
				if forms > 1 {
					t.Fatal("nested form found")
				}
			}
			if typ == "button" {
				buttons++
			}
		} else {
			if typ == "form" {
				forms--
				if forms < 0 {
					t.Fatal("unmatched </form>")
				}
			}
			if typ == "button" {
				buttons--
				if buttons < 0 {
					t.Fatal("unmatched </button>")
				}
			}
		}
	}
	if forms != 0 || buttons != 0 {
		t.Fatalf("unclosed form or button: %d, %d", forms, buttons)
	}
}
