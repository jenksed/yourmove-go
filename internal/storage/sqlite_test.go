package storage

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnswerLibrarySchemaMigratesLegacyAnswersWithoutPromotion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-answers.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE jobs (id INTEGER PRIMARY KEY,url TEXT NOT NULL,host TEXT NOT NULL)`,
		`CREATE TABLE application_answers (job_id INTEGER NOT NULL,destination TEXT NOT NULL,provider TEXT NOT NULL,question_key TEXT NOT NULL,label TEXT NOT NULL,answer TEXT NOT NULL,revision INTEGER NOT NULL,PRIMARY KEY(job_id,destination,provider,question_key))`,
		`INSERT INTO jobs(id,url,host) VALUES(1,'https://jobs.example.test/role','jobs.example.test')`,
		`INSERT INTO application_answers(job_id,destination,provider,question_key,label,answer,revision) VALUES(1,'https://jobs.example.test/role','fixture','stable-question-key','Describe a technical issue','Synthetic legacy answer',3)`,
	} {
		if _, err := legacy.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		db, err := OpenDB(path)
		if err != nil {
			t.Fatalf("open migrated legacy database (attempt %d): %v", attempt, err)
		}
		var answer string
		var revision int64
		if err := db.QueryRow(`SELECT answer,revision FROM application_answers WHERE job_id=1`).Scan(&answer, &revision); err != nil {
			t.Fatal(err)
		}
		if answer != "Synthetic legacy answer" || revision != 3 {
			t.Fatalf("legacy application answer changed: %q revision=%d", answer, revision)
		}
		var libraryCount, foreignKeys int
		if err := db.QueryRow(`SELECT COUNT(*) FROM answer_library`).Scan(&libraryCount); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_foreign_key_list('answer_library')`).Scan(&foreignKeys); err != nil {
			t.Fatal(err)
		}
		if libraryCount != 0 || foreignKeys != 0 {
			t.Fatalf("startup auto-promoted content or tied snapshots to jobs: entries=%d foreign_keys=%d", libraryCount, foreignKeys)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestApplicationStorageMigratesLegacyDatabase(t *testing.T) {
	root := t.TempDir()
	workingDir := filepath.Join(root, "working")
	workspace := filepath.Join(root, "workspace")
	if err := os.Mkdir(workingDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(workspace, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(workspace, 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workingDir)
	t.Setenv("YOURMOVE_DATA_DIR", workspace)

	legacy := filepath.Join(workingDir, "jobs.db")
	db, err := OpenDB(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO jobs (url,host,title) VALUES ('https://example.com/legacy','example.com','Old job')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	dbPath, resumeDir, err := ApplicationStorage()
	if err != nil {
		t.Fatal(err)
	}
	if dbPath != filepath.Join(workspace, "jobs.db") || resumeDir != filepath.Join(workspace, "resumes") {
		t.Fatalf("application paths = %q, %q", dbPath, resumeDir)
	}
	for _, path := range []string{workspace, resumeDir} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0700 {
			t.Fatalf("permissions for %s = %04o, want 0700", path, got)
		}
	}

	migrated, err := OpenDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	var title string
	if err := migrated.QueryRow("SELECT title FROM jobs WHERE id=1").Scan(&title); err != nil {
		t.Fatal(err)
	}
	if err := migrated.Close(); err != nil {
		t.Fatal(err)
	}
	if title != "Old job" {
		t.Fatalf("migrated job title = %q", title)
	}

	// Repeating path selection leaves the installed destination and source intact.
	if _, _, err := ApplicationStorage(); err != nil {
		t.Fatal(err)
	}
	original, err := OpenDB(legacy)
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	if err := original.QueryRow("SELECT title FROM jobs WHERE id=1").Scan(&title); err != nil {
		t.Fatalf("original disappeared: %v", err)
	}
	if title != "Old job" {
		t.Fatalf("original job title = %q", title)
	}
}

func TestApplicationStorageRequiresAbsoluteOverride(t *testing.T) {
	t.Setenv("YOURMOVE_DATA_DIR", "relative-workspace")
	if _, _, err := ApplicationStorage(); err == nil || err.Error() != "YOURMOVE_DATA_DIR must be absolute" {
		t.Fatalf("relative override error = %v", err)
	}
}

func TestMigrationRefusesDocumentWorkspaceWithoutInstallingDatabase(t *testing.T) {
	root := t.TempDir()
	workingDir := filepath.Join(root, "working")
	workspace := filepath.Join(root, "destination")
	legacyResumeDir := filepath.Join(workingDir, "resumes")
	if err := os.MkdirAll(legacyResumeDir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workingDir)
	t.Setenv("YOURMOVE_DATA_DIR", workspace)

	legacy := filepath.Join(workingDir, "jobs.db")
	db, err := OpenDB(legacy)
	if err != nil {
		t.Fatal(err)
	}
	original := []byte("%PDF-1.4\nsynthetic original")
	if _, err := db.Exec(`INSERT INTO resumes (name,stored_name,sha256,size,kind,imported_at) VALUES ('original.pdf','resume-fixture.pdf','synthetic-sha256',?,'pdf','fixture')`, len(original)); err != nil {
		t.Fatal(err)
	}
	resumePath := filepath.Join(legacyResumeDir, "resume-fixture.pdf")
	if err := os.WriteFile(resumePath, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(workspace, "jobs.db")
	_, _, err = ApplicationStorage()
	if err == nil || !strings.Contains(err.Error(), "entire workspace") {
		t.Fatalf("migration should refuse with recovery instructions: %v", err)
	}
	for _, path := range []string{destination, destination + ".migration-tmp"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("migration left %s: %v", path, err)
		}
	}
	after, err := os.ReadFile(legacy)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("source database changed: %v", err)
	}
	stored, err := os.ReadFile(resumePath)
	if err != nil || !bytes.Equal(original, stored) {
		t.Fatalf("source document changed: %v", err)
	}
}

func TestMigrationDoesNotInstallOverAnInterruptedStage(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "legacy.db")
	db, err := OpenDB(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO jobs (url,host,title) VALUES ('https://example.com/legacy','example.com','Old job')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "destination.db")
	stage := destination + ".migration-tmp"
	partial := []byte("synthetic interrupted migration")
	if err := os.WriteFile(stage, partial, 0600); err != nil {
		t.Fatal(err)
	}

	if err := migrateLegacyJobs(destination, legacy); err == nil || !strings.Contains(err.Error(), "old partial migration") {
		t.Fatalf("partial migration error = %v", err)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("migration installed destination: %v", err)
	}
	got, err := os.ReadFile(stage)
	if err != nil || !bytes.Equal(partial, got) {
		t.Fatalf("partial stage changed: %v", err)
	}
}
