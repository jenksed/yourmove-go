package main

import "embed"

//go:embed static
var staticFiles embed.FS

//go:embed templates
var templateFiles embed.FS

// All providers receive this one version-controlled source through app.Config.
//
//go:embed prompts/cover_letter.md
var coverLetterInstructions string
