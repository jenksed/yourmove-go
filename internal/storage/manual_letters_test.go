package storage

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestManualLetterMigrationRetainsLegacyEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE cover_letters (
 id INTEGER PRIMARY KEY,job_id INTEGER NOT NULL,body TEXT NOT NULL,model TEXT NOT NULL,
 provider TEXT NOT NULL DEFAULT 'ollama',master_resume_id INTEGER NOT NULL,master_sha256 TEXT NOT NULL,
 role_resume_id INTEGER NOT NULL,role_sha256 TEXT NOT NULL,job_description_sha256 TEXT NOT NULL,
 created_at TEXT NOT NULL,edited_at TEXT NOT NULL DEFAULT '');
 INSERT INTO cover_letters VALUES(42,3,'exact existing body','model','ollama',7,'master-hash',8,'role-hash','description-hash','created','edited')`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err = OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	var body, master, role, description, created, edited string
	var jobID, masterID, roleID int
	err = db.QueryRow(`SELECT job_id,body,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at,edited_at FROM cover_letters WHERE id=42`).Scan(&jobID, &body, &masterID, &master, &roleID, &role, &description, &created, &edited)
	if err != nil || jobID != 3 || body != "exact existing body" || masterID != 7 || master != "master-hash" || roleID != 8 || role != "role-hash" || description != "description-hash" || created != "created" || edited != "edited" {
		t.Fatalf("legacy data changed: %v %s %s %s", err, body, master, role)
	}
	db.Close()
	db, err = OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM cover_letters WHERE id=42`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("reopen count %d: %v", count, err)
	}
	var integrity string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity %s: %v", integrity, err)
	}
}
