package storage

import (
	"database/sql"
	"fmt"
)

// A manually written letter has no generation resume sources. Rebuild only the
// legacy NOT NULL constraint, preserving every draft ID, body and evidence field.
// SQLite cannot remove a column constraint through ALTER COLUMN.
func allowManualCoverLetterSources(db *sql.DB) error {
	var required bool
	if err := db.QueryRow(`SELECT "notnull" FROM pragma_table_info('cover_letters') WHERE name='master_resume_id'`).Scan(&required); err != nil {
		return err
	}
	if !required {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{
		`CREATE TABLE cover_letters_nullable (
 id INTEGER PRIMARY KEY,job_id INTEGER NOT NULL REFERENCES jobs(id),body TEXT NOT NULL,model TEXT NOT NULL,
 provider TEXT NOT NULL DEFAULT 'ollama',master_resume_id INTEGER REFERENCES resumes(id),master_sha256 TEXT NOT NULL,
 role_resume_id INTEGER REFERENCES resumes(id),role_sha256 TEXT NOT NULL,job_description_sha256 TEXT NOT NULL,
 created_at TEXT NOT NULL,edited_at TEXT NOT NULL DEFAULT '')`,
		`INSERT INTO cover_letters_nullable(id,job_id,body,model,provider,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at,edited_at)
 SELECT id,job_id,body,model,provider,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at,edited_at FROM cover_letters`,
		`DROP TABLE cover_letters`,
		`ALTER TABLE cover_letters_nullable RENAME TO cover_letters`,
	} {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("allow manual cover-letter sources: %w", err)
		}
	}
	return tx.Commit()
}
