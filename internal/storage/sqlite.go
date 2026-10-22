package storage

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// OpenDB opens the database and makes sure the jobs schema is usable.
func OpenDB(path string) (*sql.DB, error) {
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
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS submission_reviews (
		token TEXT PRIMARY KEY,
		job_id INTEGER NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
		resume_id INTEGER,
		resume_sha256 TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		return fmt.Errorf("create submission reviews table: %w", err)
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
	if !resumeColumns["is_master"] {
		if _, err := db.Exec(`ALTER TABLE resumes ADD COLUMN is_master INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add master resume flag: %w", err)
		}
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS one_master_resume ON resumes(is_master) WHERE is_master=1`); err != nil {
		return fmt.Errorf("index master resume: %w", err)
	}
	for _, column := range []struct{ name, ddl string }{
		{"archived_at", `ALTER TABLE resumes ADD COLUMN archived_at TEXT NOT NULL DEFAULT ''`},
		{"series_id", `ALTER TABLE resumes ADD COLUMN series_id INTEGER NOT NULL DEFAULT 0`},
		{"version", `ALTER TABLE resumes ADD COLUMN version INTEGER NOT NULL DEFAULT 1`},
	} {
		if !resumeColumns[column.name] {
			if _, err := db.Exec(column.ddl); err != nil {
				return fmt.Errorf("add resume %s: %w", column.name, err)
			}
		}
	}

	// Cover letters are saved text drafts tied to immutable resume file hashes.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS cover_letters (
        id INTEGER PRIMARY KEY,
        job_id INTEGER NOT NULL REFERENCES jobs(id),
        body TEXT NOT NULL,
        model TEXT NOT NULL,
        provider TEXT NOT NULL DEFAULT 'ollama',
        master_resume_id INTEGER NOT NULL REFERENCES resumes(id),
        master_sha256 TEXT NOT NULL,
        role_resume_id INTEGER NOT NULL REFERENCES resumes(id),
        role_sha256 TEXT NOT NULL,
        job_description_sha256 TEXT NOT NULL,
        created_at TEXT NOT NULL,
        edited_at TEXT NOT NULL DEFAULT ''
    )`); err != nil {
		return fmt.Errorf("create cover letters table: %w", err)
	}
	var hasProvider bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pragma_table_info('cover_letters') WHERE name='provider')`).Scan(&hasProvider); err != nil {
		return fmt.Errorf("inspect cover letter provider schema: %w", err)
	}
	if !hasProvider {
		if _, err := db.Exec(`ALTER TABLE cover_letters ADD COLUMN provider TEXT NOT NULL DEFAULT 'ollama'`); err != nil {
			return fmt.Errorf("migrate cover letter provider schema: %w", err)
		}
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS cover_letters_by_job ON cover_letters(job_id,id DESC)`); err != nil {
		return fmt.Errorf("index cover letters: %w", err)
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
		`CREATE TABLE IF NOT EXISTS reusable_answers (
 question_key TEXT PRIMARY KEY, employer_scope TEXT NOT NULL, label TEXT NOT NULL,
 answer TEXT NOT NULL, revision INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS application_answers (
 job_id INTEGER NOT NULL REFERENCES jobs(id), destination TEXT NOT NULL, provider TEXT NOT NULL,
 question_key TEXT NOT NULL, label TEXT NOT NULL, answer TEXT NOT NULL, revision INTEGER NOT NULL,
 PRIMARY KEY(job_id,destination,provider,question_key))`,
		// Early drafts are intentionally separate from employer answers. Their
		// question and schema snapshots remain understandable without a mutable
		// inspection row and never enter the live transfer query.
		`CREATE TABLE IF NOT EXISTS early_application_answers (
		 id INTEGER PRIMARY KEY,
		 job_id INTEGER NOT NULL REFERENCES jobs(id),
		 destination TEXT NOT NULL,
		 discovery_source TEXT NOT NULL,
		 source_question_id TEXT NOT NULL,
		 question_label TEXT NOT NULL,
		 field_snapshot_json TEXT NOT NULL,
		 discovery_fingerprint TEXT NOT NULL,
		 answer TEXT NOT NULL,
		 revision INTEGER NOT NULL,
		 reconciliation_status TEXT NOT NULL DEFAULT 'unreconciled',
		 live_inspection_fingerprint TEXT NOT NULL DEFAULT '',
		 live_question_key TEXT NOT NULL DEFAULT '',
		 reconciled_at TEXT NOT NULL DEFAULT '',
		 created_at TEXT NOT NULL,
		 updated_at TEXT NOT NULL,
		 UNIQUE(job_id,destination,discovery_source,discovery_fingerprint,source_question_id))`,
		`CREATE INDEX IF NOT EXISTS early_answers_by_job ON early_application_answers(job_id,destination,updated_at DESC,id DESC)`,
		// Answer Library entries are independent snapshots. Source IDs and URLs are
		// audit/deduplication metadata, deliberately without a jobs foreign key.
		`CREATE TABLE IF NOT EXISTS answer_library (
		 id INTEGER PRIMARY KEY,
		 original_question TEXT NOT NULL,
		 answer TEXT NOT NULL,
		 source_employer TEXT NOT NULL DEFAULT '',
		 source_job_title TEXT NOT NULL DEFAULT '',
		 source_host TEXT NOT NULL DEFAULT '',
		 source_job_id INTEGER NOT NULL,
		 source_destination TEXT NOT NULL,
		 source_provider TEXT NOT NULL,
		 source_question_key TEXT NOT NULL,
		 source_revision INTEGER NOT NULL,
		 source_inspection_id INTEGER NOT NULL,
		 source_inspection_fingerprint TEXT NOT NULL,
		 source_field_id TEXT NOT NULL,
		 source_field_kind TEXT NOT NULL,
		 question_terms TEXT NOT NULL DEFAULT '',
		 answer_terms TEXT NOT NULL DEFAULT '',
		 search_index_version INTEGER NOT NULL DEFAULT 0,
		 created_at TEXT NOT NULL,
		 updated_at TEXT NOT NULL,
		 revision INTEGER NOT NULL DEFAULT 1,
		 UNIQUE(source_job_id,source_destination,source_provider,source_question_key,source_revision))`,
		`CREATE INDEX IF NOT EXISTS answer_library_updated ON answer_library(updated_at DESC,id DESC)`,
		// Career Evidence belongs to the applicant independently of jobs and saved answers.
		// Answer Library source fields are snapshots, not foreign keys.
		`CREATE TABLE IF NOT EXISTS career_evidence (
		 id INTEGER PRIMARY KEY,
		 title TEXT NOT NULL DEFAULT '',
		 situation TEXT NOT NULL DEFAULT '',
		 actions TEXT NOT NULL DEFAULT '',
		 employer_context TEXT NOT NULL DEFAULT '',
		 role_context TEXT NOT NULL DEFAULT '',
		 technologies TEXT NOT NULL DEFAULT '',
		 outcome TEXT NOT NULL DEFAULT '',
		 outcome_state TEXT NOT NULL DEFAULT 'not_recorded' CHECK(outcome_state IN ('not_recorded','unknown','recorded')),
		 period TEXT NOT NULL DEFAULT '',
		 source_notes TEXT NOT NULL DEFAULT '',
		 source_library_entry_id INTEGER NOT NULL DEFAULT 0,
		 source_library_created_at TEXT NOT NULL DEFAULT '',
		 source_question TEXT NOT NULL DEFAULT '',
		 source_writing TEXT NOT NULL DEFAULT '',
		 source_employer TEXT NOT NULL DEFAULT '',
		 source_job_title TEXT NOT NULL DEFAULT '',
		 source_host TEXT NOT NULL DEFAULT '',
		 status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','reviewed')),
		 revision INTEGER NOT NULL DEFAULT 1,
		 created_at TEXT NOT NULL,
		 updated_at TEXT NOT NULL)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS career_evidence_library_source_once
		 ON career_evidence(source_library_entry_id,source_library_created_at) WHERE source_library_entry_id>0`,
		`CREATE INDEX IF NOT EXISTS career_evidence_updated ON career_evidence(updated_at DESC,id DESC)`,
		`CREATE TABLE IF NOT EXISTS application_steps (
		 inspection_id INTEGER NOT NULL REFERENCES application_inspections(id), ordinal INTEGER NOT NULL,
		 step_key TEXT NOT NULL, label TEXT NOT NULL, state TEXT NOT NULL, action TEXT NOT NULL,
		 PRIMARY KEY(inspection_id,ordinal))`,
	} {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("initialize application discovery: %w", err)
		}
	}
	for _, column := range []struct{ name, ddl string }{
		{"question_terms", `ALTER TABLE answer_library ADD COLUMN question_terms TEXT NOT NULL DEFAULT ''`},
		{"answer_terms", `ALTER TABLE answer_library ADD COLUMN answer_terms TEXT NOT NULL DEFAULT ''`},
		{"search_index_version", `ALTER TABLE answer_library ADD COLUMN search_index_version INTEGER NOT NULL DEFAULT 0`},
	} {
		var exists bool
		if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pragma_table_info('answer_library') WHERE name=?)`, column.name).Scan(&exists); err != nil {
			return fmt.Errorf("inspect Answer Library %s: %w", column.name, err)
		}
		if !exists {
			if _, err := db.Exec(column.ddl); err != nil {
				return fmt.Errorf("add Answer Library %s: %w", column.name, err)
			}
		}
	}
	// Add an empty explicit preference without revising or approving existing data.
	var hasPronouns bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pragma_table_info('candidate_profile') WHERE name='pronouns')`).Scan(&hasPronouns); err != nil {
		return fmt.Errorf("inspect candidate pronouns schema: %w", err)
	}
	if !hasPronouns {
		if _, err := db.Exec(`ALTER TABLE candidate_profile ADD COLUMN pronouns TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migrate candidate pronouns: %w", err)
		}
	}
	return nil
}
