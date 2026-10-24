package platforms

import (
	stdhtml "html"
	"io"
	"strings"

	"golang.org/x/net/html"
)

// descriptionText converts vendor-supplied HTML (sometimes entity-encoded)
// into readable plain text. Templates must still escape its result.
func descriptionText(raw string) string {
	// Greenhouse may send both HTML and entity-encoded HTML.
	for i := 0; i < 2; i++ {
		unescaped := stdhtml.UnescapeString(raw)
		if unescaped == raw {
			break
		}
		raw = unescaped
	}

	z := html.NewTokenizer(strings.NewReader(raw))
	var b strings.Builder
	ignored := ""
	newline := func() {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
	}
	for {
		typ := z.Next()
		if typ == html.ErrorToken {
			if z.Err() != io.EOF {
				return strings.TrimSpace(b.String())
			}
			break
		}
		tok := z.Token()
		switch typ {
		case html.StartTagToken:
			if tok.Data == "script" || tok.Data == "style" || tok.Data == "noscript" {
				ignored = tok.Data
			}
			if ignored != "" {
				continue
			}
			switch tok.Data {
			case "p", "div", "section", "h1", "h2", "h3", "h4", "h5", "h6", "li", "ul", "ol", "br":
				newline()
			}
			if tok.Data == "li" {
				b.WriteString("• ")
			}
		case html.EndTagToken:
			if tok.Data == ignored {
				ignored = ""
				continue
			}
			if ignored != "" {
				continue
			}
			switch tok.Data {
			case "p", "div", "section", "h1", "h2", "h3", "h4", "h5", "h6", "li", "ul", "ol":
				newline()
			}
		case html.TextToken:
			if ignored == "" {
				b.WriteString(tok.Data)
			}
		}
	}

	var lines []string
	for _, line := range strings.Split(b.String(), "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// NormalizeDescription compacts excessive blank lines without flattening lists,
// indentation, or paragraph boundaries. Existing source text can be displayed
// through this function without rewriting its persisted evidence or hashes.
func NormalizeDescription(text string) string {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(text, "\n")
	var out []string
	blanks := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blanks++
			continue
		}
		if len(out) > 0 && blanks > 0 {
			count := 1
			if blanks >= 3 {
				count = 2
			}
			for i := 0; i < count; i++ {
				out = append(out, "")
			}
		}
		out = append(out, strings.TrimRight(line, " \t"))
		blanks = 0
	}
	return strings.Join(out, "\n")
}
