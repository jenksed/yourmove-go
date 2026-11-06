//go:build desktop && darwin && !qualification

package main

// macOS native entry point. The composition is shared with Windows in
// desktop_shared.go so both targets launch the same Go application,
// routes, and SQLite workspace behavior.
func main() {
	runDesktop()
}
