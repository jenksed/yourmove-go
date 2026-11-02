package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	errPostingInput  = errors.New("invalid posting input")
	errPostingLookup = errors.New("check saved postings")
)

// savePostingURL owns single-URL intake for both HTML and JSON transports.
// Existing URLs bypass metadata retrieval and retain all saved work. The
// duplicate result describes this pre-fetch check; storeJobIfNew still guards
// against an intervening intake, preserving the existing response semantics.
func (a *App) savePostingURL(ctx context.Context, rawURL, rawTitle string) (id int64, duplicate bool, err error) {
	jobURL, host, err := normalizePostingURL(rawURL)
	title := strings.TrimSpace(rawTitle)
	if err != nil || len(title) > 200 {
		return 0, false, errPostingInput
	}
	index, err := loadURLIndex(a.db)
	if err != nil {
		return 0, false, fmt.Errorf("%w: %w", errPostingLookup, err)
	}
	if existing, ok := index[jobURL]; ok {
		return existing, true, nil
	}
	fetch := a.enrichmentFetch
	if fetch == nil {
		fetch = fetchPostingDetails
	}
	metadata, status, note := fetch(ctx, jobURL)
	manualTitle := title != ""
	if title == "" {
		title = metadata.Title
	}
	salaryOrigin := metadata.SalaryOrigin
	if metadata.SalaryText != "" && salaryOrigin == "" {
		salaryOrigin = "employer"
	}
	job := Job{
		URL: jobURL, Host: host, Title: title, Company: metadata.Company,
		Description: metadata.Description, SalaryText: metadata.SalaryText,
		SalaryOrigin: salaryOrigin, TitleManual: manualTitle,
		SourceStatus: status, SourceNote: note,
		SourceCheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if status != "retrieved" {
		id, _, err = a.storeJobIfNew(job)
		return id, false, err
	}
	checkedAt := time.Now().UTC().Format(time.RFC3339Nano)
	id, _, err = a.storeJobIfNewWithPostingSource(job, metadata.Source, checkedAt, metadata.Description != "", metadata.SalaryText != "")
	return id, false, err
}
