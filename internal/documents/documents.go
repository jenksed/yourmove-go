// Package documents validates resume uploads and extracts text from supported
// resume formats. It operates only on bytes supplied by its caller.
package documents

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	pdf "github.com/ledongthuc/pdf"
)

// MaxResumeSize is the maximum size accepted for a resume file.
const MaxResumeSize = 10 << 20

// ValidateResume checks the size and the basic package structure of a PDF or
// DOCX resume. It returns the normalized extension used as the document kind.
func ValidateResume(name string, data []byte) (string, error) {
	if len(data) == 0 || len(data) > MaxResumeSize {
		return "", fmt.Errorf("resume must be between 1 byte and 10 MiB")
	}
	kind := strings.ToLower(filepath.Ext(name))
	switch kind {
	case ".pdf":
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			return "", fmt.Errorf("file is not a PDF")
		}
	case ".docx":
		z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return "", fmt.Errorf("file is not a DOCX: %w", err)
		}
		document, contentTypes := false, false
		for _, f := range z.File {
			if f.Name == "word/document.xml" {
				document = true
			}
			if f.Name == "[Content_Types].xml" {
				contentTypes = true
			}
		}
		if !document || !contentTypes {
			return "", fmt.Errorf("DOCX missing required package entries")
		}
	default:
		return "", fmt.Errorf("only .pdf and .docx resumes are supported")
	}
	return kind, nil
}

// SanitizeFilename returns a safe display filename for an uploaded resume.
// The result must not be used as a filesystem path.
func SanitizeFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.TrimSpace(name)
	if len(name) > 180 {
		return ""
	}
	if name == "." || name == "/" || strings.ContainsAny(name, "\x00\r\n") {
		return ""
	}
	for _, char := range name {
		if char < 0x20 || char == 0x7f {
			return ""
		}
	}
	return name
}

// ExtractText extracts readable text from PDF and DOCX resume bytes.
func ExtractText(kind string, data []byte) (string, error) {
	switch kind {
	case ".docx":
		return extractDOCXText(data)
	case ".pdf":
		return extractPDFText(data)
	default:
		return "", fmt.Errorf("text extraction for %s is not supported", kind)
	}
}

// A read-only, bounded PDF extraction. No external process, upload or shell command.
// Image-only or encrypted PDFs may not contain usable text: those need DOCX.
func extractPDFText(data []byte) (result string, err error) {
	defer func() {
		if recover() != nil {
			result = ""
			err = errors.New("unable to parse PDF text")
		}
	}()
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	if reader.NumPage() > 60 {
		return "", errors.New("PDF has too many pages")
	}
	textReader, err := reader.GetPlainText()
	if err != nil {
		return "", err
	}
	const limit = 256 << 10
	raw, err := io.ReadAll(io.LimitReader(textReader, limit+1))
	if err != nil {
		return "", err
	}
	if len(raw) > limit {
		return "", errors.New("PDF extracted text exceeds limit")
	}
	result = strings.TrimSpace(string(raw))
	if len([]rune(result)) < 30 {
		return "", errors.New("PDF contains insufficient selectable text; use a DOCX revision")
	}
	return result, nil
}

func extractDOCXText(data []byte) (string, error) {
	const maxXML = 8 << 20
	const maxText = 256 << 10
	const wordNS = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"

	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}

	for _, file := range archive.File {
		if file.Name != "word/document.xml" {
			continue
		}
		if file.UncompressedSize64 > maxXML {
			return "", errors.New("DOCX document XML exceeds extraction limit")
		}

		reader, err := file.Open()
		if err != nil {
			return "", err
		}
		content, readErr := io.ReadAll(io.LimitReader(reader, maxXML+1))
		closeErr := reader.Close()
		if readErr != nil {
			return "", readErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if len(content) > maxXML {
			return "", errors.New("DOCX document XML exceeds extraction limit")
		}

		decoder := xml.NewDecoder(bytes.NewReader(content))
		var result, paragraph strings.Builder
		inParagraph, inText := false, false
		hidden := 0

		for {
			token, err := decoder.Token()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return "", fmt.Errorf("parse DOCX text: %w", err)
			}

			switch node := token.(type) {
			case xml.StartElement:
				if node.Name.Space != wordNS {
					continue
				}
				switch node.Name.Local {
				case "del", "moveFrom":
					hidden++
				case "p":
					if hidden == 0 {
						paragraph.Reset()
						inParagraph = true
					}
				case "t":
					inText = inParagraph && hidden == 0
				case "tab":
					if inParagraph && hidden == 0 {
						paragraph.WriteByte('\t')
					}
				case "br", "cr":
					if inParagraph && hidden == 0 {
						paragraph.WriteByte('\n')
					}
				}

			case xml.CharData:
				if inText && hidden == 0 {
					paragraph.Write(node)
				}

			case xml.EndElement:
				if node.Name.Space != wordNS {
					continue
				}
				switch node.Name.Local {
				case "t":
					inText = false
				case "p":
					if inParagraph && hidden == 0 {
						line := strings.TrimSpace(paragraph.String())
						if line != "" {
							if result.Len() > 0 {
								result.WriteByte('\n')
							}
							result.WriteString(line)
						}
						inParagraph = false
					}
				case "del", "moveFrom":
					if hidden > 0 {
						hidden--
					}
				}
			}

			if result.Len()+paragraph.Len() > maxText {
				return "", errors.New("extracted resume text exceeds limit")
			}
		}

		text := strings.TrimSpace(result.String())
		if text == "" {
			return "", errors.New("DOCX contains no readable document text")
		}
		return text, nil
	}

	return "", errors.New("DOCX missing word/document.xml")
}
