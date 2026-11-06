package browser

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// IsExecutableFile reports whether path names a file the host OS can launch
// as a browser-related executable (browser binary, Node runtime, or an
// explicit operator override such as YOURMOVE_CHROMIUM).
//
// Unix keeps the historical execute-bit requirement. Windows files carry no
// execute bits — regular files report 0666/0444 — so a bit check rejects
// every Windows executable by construction, including an explicit override
// pointing at chrome.exe. On Windows a regular file with an executable
// extension counts instead. Directories never count on either OS.
func IsExecutableFile(path string) bool {
	return isExecutableFileForGOOS(path, runtime.GOOS)
}

// isExecutableFileForGOOS is the OS-parameterized core so the Windows branch
// stays unit-testable on Unix hosts. Production callers use IsExecutableFile.
func isExecutableFileForGOOS(path, goos string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || !info.Mode().IsRegular() {
		return false
	}
	if goos == "windows" {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".exe", ".cmd", ".bat", ".com":
			return true
		}
		return false
	}
	return info.Mode().Perm()&0111 != 0
}
