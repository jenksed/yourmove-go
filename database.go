package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// openDB opens the database and makes sure the jobs schema is usable.
func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// SQLite has one writer. A single pooled connection serializes local writes,
	// including the bounded enrichment workers, without competing for file locks.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	if err := initJobsSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func initJobsSchema(db *sql.DB) error {
	if _, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS jobs (
            id INTEGER PRIMARY KEY,
            url TEXT NOT NULL,
            host TEXT NOT NULL,
            title TEXT NOT NULL DEFAULT '',
            description TEXT NOT NULL DEFAULT '',
            company TEXT NOT NULL DEFAULT '',
            stage TEXT NOT NULL DEFAULT 'review',
            priority TEXT NOT NULL DEFAULT 'normal',
            salary_text TEXT NOT NULL DEFAULT '',
            salary_origin TEXT NOT NULL DEFAULT '',
            title_manual INTEGER NOT NULL DEFAULT 0,
            description_manual INTEGER NOT NULL DEFAULT 0,
            company_manual INTEGER NOT NULL DEFAULT 0,
            salary_manual INTEGER NOT NULL DEFAULT 0,
            source_status TEXT NOT NULL DEFAULT 'not_checked',
            source_note TEXT NOT NULL DEFAULT '',
            source_checked_at TEXT NOT NULL DEFAULT '',
            selected_resume_id INTEGER REFERENCES resumes(id)
        )
    `); err != nil {
		return fmt.Errorf("create jobs table: %w", err)
	}

	rows, err := db.Query("PRAGMA table_info(jobs)")
	if err != nil {
		return fmt.Errorf("inspect jobs schema: %w", err)
	}

	columns := make(map[string]bool)
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return fmt.Errorf("scan jobs schema: %w", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("read jobs schema: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close jobs schema rows: %w", err)
	}

	if !columns["title"] {
		if _, err := db.Exec(`ALTER TABLE jobs ADD COLUMN title TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add job title column: %w", err)
		}
	}
	if !columns["description"] {
		if _, err := db.Exec(`ALTER TABLE jobs ADD COLUMN description TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add job description column: %w", err)
		}
	}
	// Additive migrations retain saved jobs from prior versions.
	for _, column := range []struct{ name, ddl string }{
		{"company", `ALTER TABLE jobs ADD COLUMN company TEXT NOT NULL DEFAULT ''`},
		{"stage", `ALTER TABLE jobs ADD COLUMN stage TEXT NOT NULL DEFAULT 'review'`},
		{"priority", `ALTER TABLE jobs ADD COLUMN priority TEXT NOT NULL DEFAULT 'normal'`},
		{"salary_text", `ALTER TABLE jobs ADD COLUMN salary_text TEXT NOT NULL DEFAULT ''`},
		{"salary_origin", `ALTER TABLE jobs ADD COLUMN salary_origin TEXT NOT NULL DEFAULT ''`},
		{"title_manual", `ALTER TABLE jobs ADD COLUMN title_manual INTEGER NOT NULL DEFAULT 1`},
		{"description_manual", `ALTER TABLE jobs ADD COLUMN description_manual INTEGER NOT NULL DEFAULT 1`},
		{"company_manual", `ALTER TABLE jobs ADD COLUMN company_manual INTEGER NOT NULL DEFAULT 1`},
		{"salary_manual", `ALTER TABLE jobs ADD COLUMN salary_manual INTEGER NOT NULL DEFAULT 0`},
		{"source_status", `ALTER TABLE jobs ADD COLUMN source_status TEXT NOT NULL DEFAULT 'not_checked'`},
		{"source_note", `ALTER TABLE jobs ADD COLUMN source_note TEXT NOT NULL DEFAULT ''`},
		{"source_checked_at", `ALTER TABLE jobs ADD COLUMN source_checked_at TEXT NOT NULL DEFAULT ''`},
		{"selected_resume_id", `ALTER TABLE jobs ADD COLUMN selected_resume_id INTEGER REFERENCES resumes(id)`},
	} {
		if !columns[column.name] {
			if _, err := db.Exec(column.ddl); err != nil {
				return fmt.Errorf("add jobs %s column: %w", column.name, err)
			}
		}
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS resumes (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        stored_name TEXT NOT NULL UNIQUE,
        sha256 TEXT NOT NULL UNIQUE,
        size INTEGER NOT NULL,
        kind TEXT NOT NULL,
        imported_at TEXT NOT NULL
    )`); err != nil {
		return fmt.Errorf("create resumes table: %w", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS submissions (
        id INTEGER PRIMARY KEY,
        job_id INTEGER NOT NULL REFERENCES jobs(id),
        resume_id INTEGER REFERENCES resumes(id),
        submitted_at TEXT NOT NULL,
        resume_name TEXT NOT NULL DEFAULT '',
        resume_sha256 TEXT NOT NULL DEFAULT '',
        token TEXT NOT NULL UNIQUE,
        notes TEXT NOT NULL DEFAULT ''
    )`); err != nil {
		return fmt.Errorf("create submissions table: %w", err)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS submissions_by_job ON submissions(job_id,id DESC)`); err != nil {
		return fmt.Errorf("index job submissions: %w", err)
	}
	// Role rules are user-owned configuration. Resume contents and job history remain intact.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS role_families (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL UNIQUE COLLATE NOCASE,
        title_terms TEXT NOT NULL,
        context_terms TEXT NOT NULL DEFAULT '',
        exclude_terms TEXT NOT NULL DEFAULT ''
    )`); err != nil {
		return fmt.Errorf("create role families: %w", err)
	}
	rows, err = db.Query(`PRAGMA table_info(resumes)`)
	if err != nil {
		return fmt.Errorf("inspect resume schema: %w", err)
	}
	resumeColumns := map[string]bool{}
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var def any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &def, &pk); err != nil {
			rows.Close()
			return err
		}
		resumeColumns[name] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !resumeColumns["role_family_id"] {
		if _, err := db.Exec(`ALTER TABLE resumes ADD COLUMN role_family_id INTEGER REFERENCES role_families(id)`); err != nil {
			return fmt.Errorf("add resume family: %w", err)
		}
	}
	if !resumeColumns["is_primary"] {
		if _, err := db.Exec(`ALTER TABLE resumes ADD COLUMN is_primary INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add primary resume flag: %w", err)
		}
	}
	// G5: read-only application requirements and a single locally approved profile.
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS candidate_profile (
		 id INTEGER PRIMARY KEY CHECK(id=1), first_name TEXT NOT NULL DEFAULT '', last_name TEXT NOT NULL DEFAULT '',
		 email TEXT NOT NULL DEFAULT '', phone TEXT NOT NULL DEFAULT '', city TEXT NOT NULL DEFAULT '',
		 region TEXT NOT NULL DEFAULT '', country TEXT NOT NULL DEFAULT '', linkedin TEXT NOT NULL DEFAULT '',
		 github TEXT NOT NULL DEFAULT '', portfolio TEXT NOT NULL DEFAULT '', approved INTEGER NOT NULL DEFAULT 0,
		 revision INTEGER NOT NULL DEFAULT 0, updated_at TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE IF NOT EXISTS application_inspections (
		 id INTEGER PRIMARY KEY, job_id INTEGER NOT NULL REFERENCES jobs(id), platform TEXT NOT NULL,
		 source TEXT NOT NULL, completeness TEXT NOT NULL, application_url TEXT NOT NULL DEFAULT '',
		 note TEXT NOT NULL DEFAULT '', fingerprint TEXT NOT NULL, checked_at TEXT NOT NULL,
		 UNIQUE(job_id,fingerprint))`,
		`CREATE INDEX IF NOT EXISTS inspections_by_job ON application_inspections(job_id,checked_at DESC,id DESC)`,
		`CREATE TABLE IF NOT EXISTS application_fields (
		 inspection_id INTEGER NOT NULL REFERENCES application_inspections(id), ordinal INTEGER NOT NULL,
		 field_id TEXT NOT NULL, label TEXT NOT NULL, kind TEXT NOT NULL, required INTEGER NOT NULL,
		 sensitive INTEGER NOT NULL, group_name TEXT NOT NULL, options_json TEXT NOT NULL,
		 PRIMARY KEY(inspection_id,ordinal))`,
		`CREATE TABLE IF NOT EXISTS application_steps (
		 inspection_id INTEGER NOT NULL REFERENCES application_inspections(id), ordinal INTEGER NOT NULL,
		 step_key TEXT NOT NULL, label TEXT NOT NULL, state TEXT NOT NULL, action TEXT NOT NULL,
		 PRIMARY KEY(inspection_id,ordinal))`,
	} {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("initialize application discovery: %w", err)
		}
	}
	return nil
}
