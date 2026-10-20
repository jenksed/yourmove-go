package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const maxBatchJobs = 50

var errBatchInput = errors.New("invalid batch input")

// normalizePostingURL preserves query parameters because some ATS use them as
// posting IDs. Remove fragments and harmless case differences. Avoid rewriting
// RawPath, which can contain meaningful percent-encoded path characters.
func normalizePostingURL(input string) (string, string, error) {
	input = strings.TrimSpace(input)
	if input == "" || len(input) > 2048 || strings.ContainsAny(input, "\r\n\t <>\x00") {
		return "", "", errors.New("a valid posting URL is required (max 2048 characters)")
	}
	parsed, err := url.Parse(input)
	if err != nil || parsed == nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" || parsed.User != nil || parsed.Opaque != "" {
		return "", "", errors.New("use an absolute http:// or https:// posting URL without credentials")
	}
	parsed.Host = strings.ToLower(parsed.Host)
	if parsed.RawPath == "" && strings.HasSuffix(parsed.Path, "/") && len(parsed.Path) > 1 {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	}
	parsed.Fragment = ""
	parsed.RawFragment = ""
	return parsed.String(), strings.ToLower(parsed.Hostname()), nil
}

type jobURLQuerier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

// loadURLIndex tolerates pre-existing duplicate rows. No destructive uniqueness
// migration is needed, and legacy posting URLs are not silently rewritten.
func loadURLIndex(q jobURLQuerier) (map[string]int64, error) {
	rows, err := q.Query(`SELECT id,url FROM jobs ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	index := make(map[string]int64)
	for rows.Next() {
		var id int64
		var stored string
		if err := rows.Scan(&id, &stored); err != nil {
			return nil, err
		}
		key, _, err := normalizePostingURL(stored)
		if err != nil {
			continue
		} // Old malformed records remain untouched.
		if _, exists := index[key]; !exists {
			index[key] = id
		}
	}
	return index, rows.Err()
}

func (a *App) hasJobURL(rawURL string) (bool, error) {
	index, err := loadURLIndex(a.db)
	if err != nil {
		return false, err
	}
	_, exists := index[rawURL]
	return exists, nil
}

// storeJobIfNew serializes writes from both intake paths. A transaction keeps
// duplicate detection and the insert together without mutating old records.
func (a *App) storeJobIfNew(job Job) (id int64, added bool, err error) {
	a.intakeMu.Lock()
	defer a.intakeMu.Unlock()
	tx, err := a.db.Begin()
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()
	index, err := loadURLIndex(tx)
	if err != nil {
		return 0, false, err
	}
	if existing, ok := index[job.URL]; ok {
		return existing, false, nil
	}
	result, err := insertJob(tx, job)
	if err != nil {
		return 0, false, err
	}
	id, err = result.LastInsertId()
	if err != nil {
		return 0, false, err
	}
	if err := tx.Commit(); err != nil {
		return 0, false, fmt.Errorf("commit job: %w", err)
	}
	return id, true, nil
}

type BatchItem struct {
	Line   int
	URL    string
	Status string // added, duplicate, invalid
	Detail string
	JobID  int64
}

type BatchResult struct {
	Items      []BatchItem
	Added      int
	Duplicates int
	Invalid    int
}

// importJobBatch persists all valid new URLs in a single transaction. Invalid
// lines are reported individually; a database error rolls the entire batch back.
// No external website is visited. Per-job source refresh remains explicit.
func (a *App) importJobBatch(raw string) (BatchResult, error) {
	var result BatchResult
	type sourceLine struct {
		number int
		value  string
	}
	var lines []sourceLine
	for index, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, sourceLine{number: index + 1, value: line})
		if len(lines) > maxBatchJobs {
			return result, fmt.Errorf("%w: import up to %d URLs at a time", errBatchInput, maxBatchJobs)
		}
	}
	if len(lines) == 0 {
		return result, fmt.Errorf("%w: paste at least one job URL", errBatchInput)
	}

	a.intakeMu.Lock()
	defer a.intakeMu.Unlock()
	tx, err := a.db.Begin()
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	index, err := loadURLIndex(tx)
	if err != nil {
		return result, err
	}
	for _, line := range lines {
		item := BatchItem{Line: line.number, URL: line.value}
		cleaned, host, err := normalizePostingURL(line.value)
		switch {
		case err != nil:
			item.Status, item.Detail = "invalid", err.Error()
			result.Invalid++
		default:
			item.URL = cleaned
			if existing, ok := index[cleaned]; ok {
				item.JobID = existing
				item.Status, item.Detail = "duplicate", "Already in your workspace; existing details retained."
				result.Duplicates++
			} else {
				stored, err := insertJob(tx, Job{URL: cleaned, Host: host, SourceStatus: "not_checked"})
				if err != nil {
					return BatchResult{}, fmt.Errorf("save line %d: %w", line.number, err)
				}
				id, err := stored.LastInsertId()
				if err != nil {
					return BatchResult{}, err
				}
				item.JobID = id
				index[cleaned] = id
				item.Status, item.Detail = "added", "Saved. Review and refresh when ready."
				result.Added++
			}
		}
		result.Items = append(result.Items, item)
	}
	if err := tx.Commit(); err != nil {
		return BatchResult{}, fmt.Errorf("commit imported jobs: %w", err)
	}
	return result, nil
}

var _ jobURLQuerier = (*sql.DB)(nil)
var _ jobURLQuerier = (*sql.Tx)(nil)
