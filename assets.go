package main

import "embed"

//go:embed static
var staticFiles embed.FS

//go:embed templates
var templateFiles embed.FS
