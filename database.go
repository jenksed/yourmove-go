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
            description TEXT NOT NULL DEFAULT ''
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
	return nil
}
