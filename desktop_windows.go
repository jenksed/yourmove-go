//go:build desktop && windows && !qualification

package main

// Windows native entry point (WebView2). The composition is shared with
// macOS in desktop_shared.go so Windows launches the same Go application,
// routes, and SQLite workspace behavior.
func main() {
	runDesktop()
}
