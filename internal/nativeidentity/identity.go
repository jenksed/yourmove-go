// Package nativeidentity provides opt-in, content-free native qualification identity.
package nativeidentity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var runPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

// Validate runs before storage selection. A qualification launch must never
// fall back to the user's normal workspace, even if the override was dropped.
func Validate() (string, error) {
	run, enabled := os.LookupEnv("YOURMOVE_NATIVE_RUN_ID")
	if !enabled {
		return "", nil
	}
	if !runPattern.MatchString(run) {
		return "", fmt.Errorf("native run ID must be 32 lowercase hex characters")
	}
	root := os.Getenv("YOURMOVE_DATA_DIR")
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("native qualification requires an explicit absolute YOURMOVE_DATA_DIR")
	}
	marker := filepath.Join(root, ".yourmove-synthetic-marker")
	info, err := os.Lstat(marker)
	if err != nil || !info.Mode().IsRegular() || info.Size() != 33 {
		return "", fmt.Errorf("native qualification requires a synthetic workspace marker")
	}
	data, err := os.ReadFile(marker)
	if err != nil || !runPattern.MatchString(strings.TrimSuffix(string(data), "\n")) {
		return "", fmt.Errorf("invalid synthetic workspace marker")
	}
	return run, nil
}

type Record struct {
	RunID            string `json:"run_id"`
	Marker           string `json:"workspace_marker"`
	PID              int    `json:"pid"`
	Started          string `json:"started"`
	Executable       string `json:"executable"`
	SHA256           string `json:"sha256"`
	WorkingDirectory string `json:"working_directory"`
	DataRoot         string `json:"data_root"`
	Database         string `json:"database"`
	Resumes          string `json:"resumes"`
	WindowTitle      string `json:"window_title"`
}

// Write records paths and process identity only, never application contents.
// Each launch gets a new exclusive record; stale evidence cannot be overwritten.
func Write(run, db, resumes string) (string, error) {
	if run == "" {
		return "YourMove", nil
	}
	if !runPattern.MatchString(run) {
		return "", fmt.Errorf("invalid native run ID")
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	marker, err := os.ReadFile(filepath.Join(filepath.Dir(db), ".yourmove-synthetic-marker"))
	if err != nil {
		return "", err
	}
	title := fmt.Sprintf("YourMove · %s · PID %d", run, os.Getpid())
	record := Record{run, strings.TrimSpace(string(marker)), os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano), exe, hex.EncodeToString(sum[:]), cwd, filepath.Dir(db), db, resumes, title}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", err
	}
	f, err := os.OpenFile(filepath.Join(filepath.Dir(db), ".yourmove-native-"+run+".json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	_, writeErr := f.Write(encoded)
	closeErr := f.Close()
	if writeErr != nil {
		return "", writeErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return title, nil
}
