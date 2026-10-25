package app

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jenksed/yourmove-go/internal/platforms"
)

const maxSavedPostingURLBytes = 2048

type postingSourceExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

type postingSourceQueryer interface {
	QueryRow(query string, args ...any) *sql.Row
}

// jobPostingSourceState is the latest successful employer-source observation.
// It intentionally contains no applicant-owned fields or application authority.
type jobPostingSourceState struct {
	LatestSuccessAt      string
	Availability         platforms.PostingSourceAvailability
	UnavailableReason    platforms.PostingSourceUnavailableReason
	Source               *platforms.PostingSource
	SourceContentSHA256  string
	DescriptionConfirmed bool
	SalaryConfirmed      bool
	Invalid              bool
}

// saveJobPostingSource writes only a successful retrieval. Call it with the
// caller's existing transaction so the job projection and its evidence commit
// or roll back together. Failed retrievals must leave this row untouched.
func saveJobPostingSource(
	execer postingSourceExecer,
	jobID int64,
	savedPostingURL string,
	canonicalDescription string,
	latestSuccessAt string,
	result platforms.PostingSourceResult,
	descriptionConfirmed bool,
	salaryConfirmed bool,
) error {
	if execer == nil {
		return errors.New("save job posting source: nil database writer")
	}
	if jobID <= 0 || len(savedPostingURL) == 0 || len(savedPostingURL) > maxSavedPostingURLBytes {
		return errors.New("save job posting source: invalid job identity")
	}
	if len(latestSuccessAt) == 0 || len(latestSuccessAt) > 80 {
		return errors.New("save job posting source: invalid success time")
	}
	if _, err := time.Parse(time.RFC3339Nano, latestSuccessAt); err != nil {
		return fmt.Errorf("save job posting source: invalid success time: %w", err)
	}

	result = platforms.NormalizePostingSourceResult(result)
	if err := result.Validate(savedPostingURL, canonicalDescription); err != nil {
		return fmt.Errorf("save job posting source: validate result: %w", err)
	}

	var encoded any
	var digest any
	availability := string(result.Availability)
	reason := string(result.Reason)
	if result.Availability == platforms.PostingSourceAvailable {
		data, err := platforms.EncodePostingSource(*result.Source, savedPostingURL, canonicalDescription)
		if err != nil {
			return fmt.Errorf("save job posting source: encode source: %w", err)
		}
		encoded = data
		digest = result.Source.SourceContentSHA256
	}

	_, err := execer.Exec(`INSERT INTO job_posting_sources (
		job_id, latest_success_at, availability, unavailable_reason,
		source_json, source_content_sha256, description_confirmed, salary_confirmed
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(job_id) DO UPDATE SET
		latest_success_at=excluded.latest_success_at,
		availability=excluded.availability,
		unavailable_reason=excluded.unavailable_reason,
		source_json=excluded.source_json,
		source_content_sha256=excluded.source_content_sha256,
		description_confirmed=excluded.description_confirmed,
		salary_confirmed=excluded.salary_confirmed`,
		jobID, latestSuccessAt, availability, reason, encoded, digest,
		boolToInt(descriptionConfirmed), boolToInt(salaryConfirmed))
	if err != nil {
		return fmt.Errorf("save job posting source for job %d: %w", jobID, err)
	}
	return nil
}

// usablePostingSource keeps a successful canonical retrieval usable when its
// optional rich envelope is malformed, oversized, or associated with another
// posting. The canonical job fields remain the provider's primary result.
func usablePostingSource(result platforms.PostingSourceResult, savedPostingURL, canonicalDescription string) platforms.PostingSourceResult {
	result = platforms.NormalizePostingSourceResult(result)
	if err := result.Validate(savedPostingURL, canonicalDescription); err == nil {
		return result
	} else if errors.Is(err, platforms.ErrPostingSourceTooLarge) {
		return platforms.UnavailablePostingSource(platforms.PostingSourceTooLarge)
	}
	return platforms.UnavailablePostingSource(platforms.PostingSourceInvalid)
}

// loadJobPostingSource distinguishes an absent observation from invalid
// persisted evidence. SQL errors propagate; malformed rows return found=true
// with Invalid set, allowing callers to keep the canonical Job readable.
func loadJobPostingSource(queryer postingSourceQueryer, job Job) (jobPostingSourceState, bool, error) {
	if queryer == nil {
		return jobPostingSourceState{}, false, errors.New("load job posting source: nil database reader")
	}
	var (
		latestRaw, availabilityRaw, reasonRaw       any
		sourceRaw, digestRaw                        any
		sourceSize                                  sql.NullInt64
		descriptionConfirmedRaw, salaryConfirmedRaw any
	)
	err := queryer.QueryRow(`SELECT latest_success_at, availability, unavailable_reason,
		CASE WHEN length(source_json) BETWEEN 1 AND ? THEN source_json END,
		length(source_json), source_content_sha256, description_confirmed, salary_confirmed
		FROM job_posting_sources WHERE job_id=?`, platforms.MaxPostingSourceJSONBytes, job.ID).Scan(
		&latestRaw, &availabilityRaw, &reasonRaw, &sourceRaw, &sourceSize, &digestRaw,
		&descriptionConfirmedRaw, &salaryConfirmedRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return jobPostingSourceState{}, false, nil
	}
	if err != nil {
		return jobPostingSourceState{}, false, fmt.Errorf("load job posting source for job %d: %w", job.ID, err)
	}

	state := jobPostingSourceState{Invalid: true}
	latest, ok := storedText(latestRaw)
	if !ok || len(latest) == 0 || len(latest) > 80 {
		return state, true, nil
	}
	if _, err := time.Parse(time.RFC3339Nano, latest); err != nil {
		return state, true, nil
	}
	state.LatestSuccessAt = latest
	availabilityText, ok := storedText(availabilityRaw)
	if !ok {
		return state, true, nil
	}
	reasonText, ok := storedText(reasonRaw)
	if !ok {
		return state, true, nil
	}
	descriptionConfirmed, ok := storedBool(descriptionConfirmedRaw)
	if !ok {
		return state, true, nil
	}
	salaryConfirmed, ok := storedBool(salaryConfirmedRaw)
	if !ok {
		return state, true, nil
	}
	switch platforms.PostingSourceAvailability(availabilityText) {
	case platforms.PostingSourceUnavailable:
		reason := platforms.PostingSourceUnavailableReason(reasonText)
		result := platforms.UnavailablePostingSource(reason)
		if sourceRaw != nil || sourceSize.Valid || digestRaw != nil || result.Validate(job.URL, job.Description) != nil {
			return state, true, nil
		}
		state.Availability = platforms.PostingSourceUnavailable
		state.UnavailableReason = reason
		state.DescriptionConfirmed = descriptionConfirmed && !job.DescriptionManual
		state.SalaryConfirmed = salaryConfirmed && !job.SalaryManual
		state.Invalid = false
		return state, true, nil
	case platforms.PostingSourceAvailable:
		if reasonText != "" {
			return state, true, nil
		}
		if !sourceSize.Valid || sourceSize.Int64 < 1 || sourceSize.Int64 > platforms.MaxPostingSourceJSONBytes {
			return state, true, nil
		}
		data, ok := storedBytes(sourceRaw)
		if !ok || int64(len(data)) != sourceSize.Int64 {
			return state, true, nil
		}
		storedDigest, ok := storedText(digestRaw)
		if !ok || len(storedDigest) != 64 {
			return state, true, nil
		}
		var source platforms.PostingSource
		var err error
		if job.DescriptionManual || !descriptionConfirmed {
			source, err = platforms.DecodePostingSourceIntegrity(data, job.URL)
		} else {
			source, err = platforms.DecodePostingSource(data, job.URL, job.Description)
		}
		if err != nil || source.SourceContentSHA256 != storedDigest {
			return state, true, nil
		}
		state.Availability = platforms.PostingSourceAvailable
		state.Source = &source
		state.SourceContentSHA256 = storedDigest
		state.DescriptionConfirmed = descriptionConfirmed && !job.DescriptionManual
		state.SalaryConfirmed = salaryConfirmed && !job.SalaryManual
		state.Invalid = false
		return state, true, nil
	default:
		return state, true, nil
	}
}

func storedText(value any) (string, bool) {
	switch value := value.(type) {
	case string:
		return value, true
	case []byte:
		return string(value), true
	default:
		return "", false
	}
}

func storedBytes(value any) ([]byte, bool) {
	switch value := value.(type) {
	case []byte:
		return value, true
	case string:
		return []byte(value), true
	default:
		return nil, false
	}
}

func storedBool(value any) (bool, bool) {
	number, ok := value.(int64)
	if !ok || (number != 0 && number != 1) {
		return false, false
	}
	return number == 1, true
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
