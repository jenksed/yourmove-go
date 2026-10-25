package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ApplicationStorage keeps both desktop and browser mode on the same database.
// The legacy working-directory database is left untouched after a consistent copy.
func ApplicationStorage() (dbPath, resumeDir string, err error) {
	root, explicit := os.LookupEnv("YOURMOVE_DATA_DIR")
	if explicit && root == "" {
		return "", "", fmt.Errorf("YOURMOVE_DATA_DIR is set but empty")
	}
	if !explicit {
		if runtime.GOOS == "darwin" {
			home, e := os.UserHomeDir()
			if e != nil {
				return "", "", e
			}
			root = filepath.Join(home, "Library", "Application Support", "YourMove")
		} else {
			config, e := os.UserConfigDir()
			if e != nil {
				return "", "", e
			}
			root = filepath.Join(config, "YourMove")
		}
	}
	if !filepath.IsAbs(root) {
		return "", "", fmt.Errorf("YOURMOVE_DATA_DIR must be absolute")
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return "", "", fmt.Errorf("create app directory: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", fmt.Errorf("resolve app directory: %w", err)
	}
	if root == string(filepath.Separator) {
		return "", "", fmt.Errorf("app directory cannot be the filesystem root")
	}
	// A root alias is canonicalized once. Workspace members must not redirect
	// SQLite or document operations outside that selected directory.
	for _, name := range []string{"jobs.db", "resumes"} {
		info, e := os.Lstat(filepath.Join(root, name))
		if e != nil && !errors.Is(e, os.ErrNotExist) {
			return "", "", e
		}
		if e == nil && (info.Mode()&os.ModeSymlink != 0 || (name == "jobs.db" && !info.Mode().IsRegular()) || (name == "resumes" && !info.IsDir())) {
			return "", "", fmt.Errorf("workspace %s must be a direct %s", name, map[string]string{"jobs.db": "regular file", "resumes": "directory"}[name])
		}
	}
	if err = os.Chmod(root, 0700); err != nil {
		return "", "", err
	}
	resumeDir = filepath.Join(root, "resumes")
	if err = os.MkdirAll(resumeDir, 0700); err != nil {
		return "", "", fmt.Errorf("create resume directory: %w", err)
	}
	if err = os.Chmod(resumeDir, 0700); err != nil {
		return "", "", err
	}
	dbPath = filepath.Join(root, "jobs.db")
	// An explicit workspace is authoritative, including an empty synthetic one.
	// Only normal application mode may discover a working-directory legacy DB.
	if explicit {
		return dbPath, resumeDir, nil
	}
	legacy, err := filepath.Abs("jobs.db")
	if err != nil {
		return "", "", err
	}
	if legacy != dbPath {
		if err := migrateLegacyJobs(dbPath, legacy); err != nil {
			return "", "", err
		}
	}
	return dbPath, resumeDir, nil
}

// migrateLegacyJobs uses SQLite's online-consistent VACUUM INTO snapshot.
// A staged copy ensures interrupted migrations cannot leave a half-written jobs.db.
func migrateLegacyJobs(destination, legacy string) error {
	if _, err := os.Stat(destination); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := os.Stat(legacy); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	db, err := sql.Open("sqlite", legacy)
	if err != nil {
		return err
	}
	defer db.Close()
	stage := destination + ".migration-tmp"
	if _, err := os.Stat(stage); err == nil {
		return fmt.Errorf("old partial migration at %s; inspect before retrying", stage)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := db.Exec("VACUUM INTO ?", stage); err != nil {
		return fmt.Errorf("copy existing jobs with SQLite snapshot: %w", err)
	}
	// Inspect the consistent snapshot, not the live source. A database-only
	// migration cannot carry the document files referenced by a modern workspace.
	defer os.Remove(stage)
	snapshot, err := sql.Open("sqlite", stage)
	if err != nil {
		return err
	}
	var tables, documents int
	err = snapshot.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='resumes'").Scan(&tables)
	if err == nil && tables > 0 {
		err = snapshot.QueryRow("SELECT count(*) FROM resumes").Scan(&documents)
	}
	closeErr := snapshot.Close()
	if err != nil {
		return fmt.Errorf("inspect migration snapshot: %w", err)
	}
	if closeErr != nil {
		return closeErr
	}
	if documents > 0 {
		return fmt.Errorf("legacy workspace at %s contains resume documents; automatic database-only migration refused. Quit YourMove and back up and restore the entire workspace (jobs.db and resumes directory) together; the original has been preserved", legacy)
	}
	if err := os.Chmod(stage, 0600); err != nil {
		return err
	}
	if err := os.Rename(stage, destination); err != nil {
		return fmt.Errorf("install migrated jobs db: %w", err)
	}
	return nil
}
