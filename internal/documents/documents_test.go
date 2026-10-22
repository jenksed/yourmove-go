package documents

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{`C:\private\resume.pdf`, "resume.pdf"},
		{" /private/resume.docx ", "resume.docx"},
		{"", ""},
		{".", ""},
		{"/", ""},
		{"resume\n.pdf", ""},
		{"resume\x7f.pdf", ""},
		{strings.Repeat("a", 181), ""},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%q", tc.name), func(t *testing.T) {
			if got := SanitizeFilename(tc.name); got != tc.want {
				t.Fatalf("SanitizeFilename(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestValidateResumeKeepsSizeAndFormatErrors(t *testing.T) {
	if kind, err := ValidateResume("resume.PDF", []byte("%PDF-1.4")); err != nil || kind != ".pdf" {
		t.Fatalf("valid PDF kind=%q error=%v", kind, err)
	}
	if _, err := ValidateResume("empty.pdf", nil); err == nil || err.Error() != "resume must be between 1 byte and 10 MiB" {
		t.Fatalf("empty file error = %v", err)
	}
	if _, err := ValidateResume("large.pdf", bytes.Repeat([]byte("x"), MaxResumeSize+1)); err == nil || err.Error() != "resume must be between 1 byte and 10 MiB" {
		t.Fatalf("oversized file error = %v", err)
	}
	if _, err := ValidateResume("fake.pdf", []byte("not a PDF")); err == nil || err.Error() != "file is not a PDF" {
		t.Fatalf("invalid PDF error = %v", err)
	}
	if _, err := ValidateResume("resume.txt", []byte("%PDF-1.4")); err == nil || err.Error() != "only .pdf and .docx resumes are supported" {
		t.Fatalf("unsupported extension error = %v", err)
	}
}

func TestExtractDOCXText(t *testing.T) {
	const xml = `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Support</w:t></w:r><w:r><w:t xml:space="preserve"> Engineer</w:t></w:r></w:p><w:p><w:r><w:t>Linux &amp; MySQL troubleshooting</w:t></w:r></w:p></w:body></w:document>`
	got, err := ExtractText(".docx", makeDOCX(t, xml))
	if err != nil {
		t.Fatal(err)
	}
	if want := "Support Engineer\nLinux & MySQL troubleshooting"; got != want {
		t.Fatalf("extracted text = %q, want %q", got, want)
	}
}

func TestExtractDOCXTextLimits(t *testing.T) {
	t.Run("XML size", func(t *testing.T) {
		xml := `<w:document>` + strings.Repeat("x", (8<<20)+1) + `</w:document>`
		_, err := ExtractText(".docx", makeDOCX(t, xml))
		if err == nil || err.Error() != "DOCX document XML exceeds extraction limit" {
			t.Fatalf("XML size error = %v", err)
		}
	})

	t.Run("extracted text size", func(t *testing.T) {
		const ns = `http://schemas.openxmlformats.org/wordprocessingml/2006/main`
		xml := `<w:document xmlns:w="` + ns + `"><w:body><w:p><w:r><w:t>` + strings.Repeat("x", (256<<10)+1) + `</w:t></w:r></w:p></w:body></w:document>`
		_, err := ExtractText(".docx", makeDOCX(t, xml))
		if err == nil || err.Error() != "extracted resume text exceeds limit" {
			t.Fatalf("extracted text size error = %v", err)
		}
	})
}

func TestExtractPDFTextForCoverLetter(t *testing.T) {
	text, err := ExtractText(".pdf", makePDF(t, 1, "Linux support production troubleshooting and incident response"))
	if err != nil || !strings.Contains(text, "Linux") || !strings.Contains(text, "incident") {
		t.Fatalf("PDF text %q error %v", text, err)
	}
}

func TestExtractPDFTextLimits(t *testing.T) {
	t.Run("page count", func(t *testing.T) {
		_, err := ExtractText(".pdf", makePDF(t, 61, "short text"))
		if err == nil || err.Error() != "PDF has too many pages" {
			t.Fatalf("page count error = %v", err)
		}
	})

	t.Run("extracted text size", func(t *testing.T) {
		_, err := ExtractText(".pdf", makePDF(t, 1, strings.Repeat("x", (256<<10)+1)))
		if err == nil || err.Error() != "PDF extracted text exceeds limit" {
			t.Fatalf("extracted text size error = %v", err)
		}
	})
}

func makeDOCX(t *testing.T, documentXML string) []byte {
	t.Helper()
	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	types, err := archive.Create("[Content_Types].xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(types, `<Types/>`); err != nil {
		t.Fatal(err)
	}
	document, err := archive.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(document, documentXML); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func makePDF(t *testing.T, pageCount int, text string) []byte {
	t.Helper()
	fontID := 3 + pageCount
	objectCount := 3 + 2*pageCount
	objects := make([]string, objectCount+1)
	objects[1] = "<< /Type /Catalog /Pages 2 0 R >>"
	var kids strings.Builder
	kids.WriteString("<< /Type /Pages /Kids [")
	for page := 0; page < pageCount; page++ {
		fmt.Fprintf(&kids, "%d 0 R ", 3+page)
	}
	fmt.Fprintf(&kids, "] /Count %d >>", pageCount)
	objects[2] = kids.String()
	for page := 0; page < pageCount; page++ {
		pageID := 3 + page
		contentID := 4 + pageCount + page
		objects[pageID] = fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 %d 0 R >> >> /Contents %d 0 R >>", fontID, contentID)
		stream := fmt.Sprintf("BT /F1 12 Tf 72 700 Td (%s) Tj ET", text)
		objects[contentID] = fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream)
	}
	objects[fontID] = "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, objectCount+1)
	for id := 1; id <= objectCount; id++ {
		offsets[id] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", id, objects[id])
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", objectCount+1)
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", objectCount+1, xref)
	return buf.Bytes()
}
