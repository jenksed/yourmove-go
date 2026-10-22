package app

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
)

func (d CoverLetter) ContentHash() string { return fmt.Sprintf("%x", sha256.Sum256([]byte(d.Body))) }

// coverLetterPDF is a deliberately small deterministic text export. Standard
// PDF fonts cover Windows-1252. Unsupported glyphs fail closed rather than
// silently altering the user's letter; exact Unicode text transfer remains available.
func coverLetterPDF(body string) ([]byte, error) {
	const special = "€\x00‚ƒ„…†‡ˆ‰Š‹Œ\x00Ž\x00\x00‘’“”•–—˜™š›œ\x00žŸ"
	body = strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\r", "\n")
	var encoded []byte
	for _, r := range body {
		switch {
		case r == '\r':
			continue
		case r == '\n':
			encoded = append(encoded, '\n')
		case r == '\t':
			encoded = append(encoded, ' ', ' ', ' ', ' ')
		case r >= 32 && r <= 126 || r >= 160 && r <= 255:
			encoded = append(encoded, byte(r))
		default:
			found := false
			for i, candidate := range []rune(special) {
				if r == candidate && r != 0 {
					encoded = append(encoded, byte(128+i))
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("PDF export cannot represent a character in this letter; use the exact text option or attach your own document")
			}
		}
	}
	if len(encoded) == 0 {
		return nil, fmt.Errorf("cover letter is empty")
	}
	// Fixed-pitch layout makes width deterministic without a font dependency.
	const columns, linesPerPage = 74, 44
	var lines []string
	for _, paragraph := range strings.Split(string(encoded), "\n") {
		for len(paragraph) > columns {
			cut := strings.LastIndexByte(paragraph[:columns+1], ' ')
			if cut < 1 {
				cut = columns
			}
			lines = append(lines, paragraph[:cut])
			paragraph = paragraph[cut:]
			if strings.HasPrefix(paragraph, " ") {
				paragraph = paragraph[1:]
			}
		}
		lines = append(lines, paragraph)
	}
	pages := (len(lines) + linesPerPage - 1) / linesPerPage
	objects := []string{"", "", "<< /Type /Font /Subtype /Type1 /BaseFont /Courier /Encoding /WinAnsiEncoding >>"}
	var kids []string
	for page := 0; page < pages; page++ {
		pageID := len(objects) + 1
		kids = append(kids, fmt.Sprintf("%d 0 R", pageID))
		var stream strings.Builder
		stream.WriteString("BT /F1 10.5 Tf 15 TL 72 720 Td\n")
		end := min((page+1)*linesPerPage, len(lines))
		for _, line := range lines[page*linesPerPage : end] {
			line = strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)").Replace(line)
			fmt.Fprintf(&stream, "(%s) Tj T*\n", line)
		}
		stream.WriteString("ET\n")
		objects = append(objects, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", pageID+1), fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", stream.Len(), stream.String()))
	}
	objects[0] = "<< /Type /Catalog /Pages 2 0 R >>"
	objects[1] = fmt.Sprintf("<< /Type /Pages /Count %d /Kids [%s] >>", pages, strings.Join(kids, " "))
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := []int{0}
	for i, obj := range objects {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	start := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), start)
	return out.Bytes(), nil
}
