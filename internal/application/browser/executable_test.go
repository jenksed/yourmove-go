package browser

import (
	"os"
	"path/filepath"
	"testing"
)

func writeProbeFile(t *testing.T, name string, perm os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("probe"), perm); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestIsExecutableFileUnixKeepsExecuteBitRequirement(t *testing.T) {
	if got := isExecutableFileForGOOS(writeProbeFile(t, "chrome", 0700), "linux"); !got {
		t.Fatal("executable bit file rejected on linux")
	}
	if got := isExecutableFileForGOOS(writeProbeFile(t, "chrome", 0600), "linux"); got {
		t.Fatal("non-executable regular file accepted on linux")
	}
	if got := isExecutableFileForGOOS(writeProbeFile(t, "chrome.exe", 0600), "linux"); got {
		t.Fatal("unadorned .exe accepted on linux without execute bits")
	}
	if got := isExecutableFileForGOOS(t.TempDir(), "linux"); got {
		t.Fatal("directory accepted on linux")
	}
	if got := isExecutableFileForGOOS("", "linux"); got {
		t.Fatal("empty path accepted on linux")
	}
	if got := isExecutableFileForGOOS(filepath.Join(t.TempDir(), "missing"), "linux"); got {
		t.Fatal("missing path accepted on linux")
	}
}

func TestIsExecutableFileWindowsAcceptsRegularExe(t *testing.T) {
	// Windows files carry no execute bits: a freshly written .exe reports
	// 0666/0444. The bit check every Windows binary fails must not apply.
	if got := isExecutableFileForGOOS(writeProbeFile(t, "chrome.exe", 0600), "windows"); !got {
		t.Fatal("regular .exe rejected on windows")
	}
	if got := isExecutableFileForGOOS(writeProbeFile(t, "CHROME.EXE", 0444), "windows"); !got {
		t.Fatal("read-only uppercase .EXE rejected on windows")
	}
	if got := isExecutableFileForGOOS(writeProbeFile(t, "node.cmd", 0600), "windows"); !got {
		t.Fatal("regular .cmd rejected on windows")
	}
	if got := isExecutableFileForGOOS(writeProbeFile(t, "chrome", 0600), "windows"); got {
		t.Fatal("extensionless file accepted on windows")
	}
	if got := isExecutableFileForGOOS(writeProbeFile(t, "notes.txt", 0600), "windows"); got {
		t.Fatal("non-executable extension accepted on windows")
	}
	if got := isExecutableFileForGOOS(t.TempDir(), "windows"); got {
		t.Fatal("directory accepted on windows")
	}
	if got := isExecutableFileForGOOS("", "windows"); got {
		t.Fatal("empty path accepted on windows")
	}
}
