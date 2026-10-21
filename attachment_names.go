package main

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// attachmentFilename uses public-facing facts, never internal document labels,
// version numbers, model names, or storage IDs. The original extension stays intact.
func attachmentFilename(profile CandidateProfile, job Job, document, extension string) string {
	var parts []string
	if profile.Approved {
		if name := attachmentNamePart(profile.FirstName+" "+profile.LastName, 60); name != "" {
			parts = append(parts, name)
		}
	}
	parts = append(parts, document)
	for _, value := range []string{job.Title, job.Company} {
		if part := attachmentNamePart(value, 60); part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "_") + extension
}

func attachmentNamePart(value string, limit int) string {
	var out []rune
	size := 0
	separator := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			nextSize := size + utf8.RuneLen(r)
			if separator && len(out) > 0 {
				nextSize++
			}
			if nextSize > limit {
				break
			}
			if separator && len(out) > 0 {
				out = append(out, '_')
			}
			out = append(out, r)
			size = nextSize
			separator = false
		} else {
			separator = true
		}
	}
	return string(out)
}
