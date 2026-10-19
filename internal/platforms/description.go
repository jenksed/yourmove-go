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
