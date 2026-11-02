package app

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/platforms"
	"github.com/jenksed/yourmove-go/internal/storage"
)

const testAshbyPostingURL = "https://jobs.ashbyhq.com/acme/11111111-1111-4111-8111-111111111111"
const testPostingSuccessAt = "2026-10-24T12:30:00Z"

func sealedTestPostingSource(t *testing.T, canonical string) platforms.PostingSource {
	t.Helper()
	identity, err := platforms.AshbyPostingIdentity(testAshbyPostingURL)
	if err != nil {
		t.Fatal(err)
	}
	source, err := platforms.SealPostingSource(platforms.PostingSource{
		Identity: identity, DescriptionHTML: "<h2>Work</h2><p>Build systems.</p>",
		DescriptionPlain: "Work\nBuild systems.", Location: "Remote",
		WorkplaceType: "remote", EmploymentType: "full_time", SalarySummary: "€120K - €150K",
	}, canonical)
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func saveTestJob(t *testing.T, a *App, description string) int64 {
	t.Helper()
	if err := a.saveJob(Job{
		URL: testAshbyPostingURL, Host: "jobs.ashbyhq.com", Title: "Platform Engineer",
		Description: description, SalaryText: "€120K - €150K", SalaryOrigin: "employer",
	}); err != nil {
		t.Fatal(err)
	}
	return 1
}

func TestJobPostingSourceLatestSuccessReopensWithoutChangingApplicantData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.db")
	db, err := storage.OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	a := newTestApp(t, db, "")
	canonical := "Platform role\nBuild reliable services."
	jobID := saveTestJob(t, a, canonical)
	if _, err := db.Exec(`INSERT INTO application_answers(job_id,destination,provider,question_key,label,answer,revision)
		VALUES(1, ?, 'ashby', 'question-1', 'Experience', 'Synthetic saved answer', 4)`, testAshbyPostingURL); err != nil {
		t.Fatal(err)
	}

	source := sealedTestPostingSource(t, canonical)
	if err := saveJobPostingSource(db, jobID, testAshbyPostingURL, canonical, testPostingSuccessAt,
		platforms.AvailablePostingSource(source), true, true); err != nil {
		t.Fatal(err)
	}

	var sourceJSON []byte
	var jobDescription, answer string
	var checkedAt string
	if err := db.QueryRow(`SELECT source_json FROM job_posting_sources WHERE job_id=1`).Scan(&sourceJSON); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sourceJSON, []byte("Synthetic saved answer")) || bytes.Contains(sourceJSON, []byte("applicant")) {
		t.Fatalf("employer source envelope includes applicant data: %s", sourceJSON)
	}
	if err := db.QueryRow(`SELECT description,source_checked_at FROM jobs WHERE id=1`).Scan(&jobDescription, &checkedAt); err != nil {
		t.Fatal(err)
	}
	if jobDescription != canonical || checkedAt != "" {
		t.Fatalf("source persistence changed canonical job or attempt time: description=%q checked=%q", jobDescription, checkedAt)
	}
	if err := db.QueryRow(`SELECT answer FROM application_answers WHERE job_id=1 AND question_key='question-1'`).Scan(&answer); err != nil {
		t.Fatal(err)
	}
	if answer != "Synthetic saved answer" {
		t.Fatalf("applicant answer changed: %q", answer)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := storage.OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reopenedApp := newTestApp(t, reopened, "")
	job, err := reopenedApp.jobByID(jobID)
	if err != nil {
		t.Fatal(err)
	}
	state, found, err := loadJobPostingSource(reopened, job)
	if err != nil || !found {
		t.Fatalf("load after restart: found=%v err=%v", found, err)
	}
	if state.Invalid || state.Source == nil || state.Source.DescriptionHTML != source.DescriptionHTML ||
		state.LatestSuccessAt != testPostingSuccessAt || !state.DescriptionConfirmed || !state.SalaryConfirmed {
		t.Fatalf("reopened source state = %+v", state)
	}
	if err := reopened.QueryRow(`SELECT answer FROM application_answers WHERE job_id=1 AND question_key='question-1'`).Scan(&answer); err != nil {
		t.Fatal(err)
	}
	if answer != "Synthetic saved answer" {
		t.Fatalf("applicant answer changed after reopen: %q", answer)
	}
}

func TestJobPostingSourceManualCorrectionsRemainPrimary(t *testing.T) {
	a, _ := testWorkspace(t)
	canonical := "Employer canonical description"
	jobID := saveTestJob(t, a, canonical)
	if err := saveJobPostingSource(a.db, jobID, testAshbyPostingURL, canonical, testPostingSuccessAt,
		platforms.AvailablePostingSource(sealedTestPostingSource(t, canonical)), true, true); err != nil {
		t.Fatal(err)
	}

	job, err := a.jobByID(jobID)
	if err != nil {
		t.Fatal(err)
	}
	job.Description = "Applicant's corrected description"
	job.SalaryText = "Salary confirmed by applicant"
	if err := a.updateJob(job); err != nil {
		t.Fatal(err)
	}
	job, err = a.jobByID(jobID)
	if err != nil {
		t.Fatal(err)
	}
	state, found, err := loadJobPostingSource(a.db, job)
	if err != nil || !found {
		t.Fatalf("load manual source: found=%v err=%v", found, err)
	}
	if state.Invalid || state.Source == nil || state.Source.CanonicalDescriptionSHA256 == platforms.CanonicalDescriptionDigest(job.Description) {
		t.Fatalf("manual correction should retain validated employer source secondarily: %+v", state)
	}
	if state.DescriptionConfirmed || state.SalaryConfirmed || job.Description != "Applicant's corrected description" || job.SalaryText != "Salary confirmed by applicant" {
		t.Fatalf("manual values were not kept primary: state=%+v job=%+v", state, job)
	}
}

func TestJobPostingSourceRequiresCanonicalBindingWithoutManualCorrection(t *testing.T) {
	a, _ := testWorkspace(t)
	canonical := "Employer canonical description"
	jobID := saveTestJob(t, a, canonical)
	if err := saveJobPostingSource(a.db, jobID, testAshbyPostingURL, canonical, testPostingSuccessAt,
		platforms.AvailablePostingSource(sealedTestPostingSource(t, canonical)), true, true); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`UPDATE jobs SET description='unexpected nonmanual change' WHERE id=?`, jobID); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(jobID)
	if err != nil {
		t.Fatal(err)
	}
	state, found, err := loadJobPostingSource(a.db, job)
	if err != nil || !found {
		t.Fatalf("load mismatched source: found=%v err=%v", found, err)
	}
	if !state.Invalid || state.Source != nil || state.DescriptionConfirmed || state.SalaryConfirmed {
		t.Fatalf("source with wrong canonical association was trusted: %+v", state)
	}
}

func TestJobPostingSourceCanonicalOnlySuccess(t *testing.T) {
	a, _ := testWorkspace(t)
	jobID := saveTestJob(t, a, "Canonical text from metadata")
	// The zero result is the compatibility representation of successful
	// canonical-only retrieval, not a request failure.
	if err := saveJobPostingSource(a.db, jobID, testAshbyPostingURL, "Canonical text from metadata", testPostingSuccessAt,
		platforms.PostingSourceResult{}, true, false); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(jobID)
	if err != nil {
		t.Fatal(err)
	}
	state, found, err := loadJobPostingSource(a.db, job)
	if err != nil || !found {
		t.Fatalf("load canonical-only state: found=%v err=%v", found, err)
	}
	if state.Invalid || state.Availability != platforms.PostingSourceUnavailable ||
		state.UnavailableReason != platforms.PostingSourceAbsent || state.Source != nil ||
		!state.DescriptionConfirmed || state.SalaryConfirmed || state.LatestSuccessAt != testPostingSuccessAt {
		t.Fatalf("canonical-only state = %+v", state)
	}
}

func TestJobPostingSourceLatestCanonicalOnlySuccessSupersedesRichSource(t *testing.T) {
	a, _ := testWorkspace(t)
	canonical := "Canonical text from metadata"
	jobID := saveTestJob(t, a, canonical)
	if err := saveJobPostingSource(a.db, jobID, testAshbyPostingURL, canonical, testPostingSuccessAt,
		platforms.AvailablePostingSource(sealedTestPostingSource(t, canonical)), true, true); err != nil {
		t.Fatal(err)
	}
	newerSuccessAt := "2026-10-24T13:00:00Z"
	if err := saveJobPostingSource(a.db, jobID, testAshbyPostingURL, canonical, newerSuccessAt,
		platforms.UnavailablePostingSource(platforms.PostingSourceAbsent), true, false); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(jobID)
	if err != nil {
		t.Fatal(err)
	}
	state, found, err := loadJobPostingSource(a.db, job)
	if err != nil || !found {
		t.Fatalf("load superseding observation: found=%v err=%v", found, err)
	}
	if state.Invalid || state.Availability != platforms.PostingSourceUnavailable ||
		state.UnavailableReason != platforms.PostingSourceAbsent || state.Source != nil ||
		state.LatestSuccessAt != newerSuccessAt || !state.DescriptionConfirmed || state.SalaryConfirmed {
		t.Fatalf("newest successful canonical-only observation did not supersede rich source: %+v", state)
	}
	var sourceJSON, digest any
	if err := a.db.QueryRow(`SELECT source_json,source_content_sha256 FROM job_posting_sources WHERE job_id=?`, jobID).Scan(&sourceJSON, &digest); err != nil {
		t.Fatal(err)
	}
	if sourceJSON != nil || digest != nil {
		t.Fatalf("canonical-only observation retained old source bytes: json=%T digest=%T", sourceJSON, digest)
	}
}

func TestJobPostingSourceInvalidEnvelopeFallsBack(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(t *testing.T, db *sql.DB, encoded []byte, source platforms.PostingSource)
	}{
		{
			name: "future version",
			mutate: func(t *testing.T, db *sql.DB, encoded []byte, _ platforms.PostingSource) {
				var envelope map[string]any
				if err := json.Unmarshal(encoded, &envelope); err != nil {
					t.Fatal(err)
				}
				envelope["version"] = float64(platforms.PostingSourceFormatVersion + 1)
				bad, err := json.Marshal(envelope)
				if err != nil {
					t.Fatal(err)
				}
				forceCorruptSource(t, db, bad)
			},
		},
		{
			name: "corrupt JSON",
			mutate: func(t *testing.T, db *sql.DB, _ []byte, _ platforms.PostingSource) {
				forceCorruptSource(t, db, []byte("{"))
			},
		},
		{
			name: "wrong posting identity",
			mutate: func(t *testing.T, db *sql.DB, encoded []byte, _ platforms.PostingSource) {
				var envelope map[string]any
				if err := json.Unmarshal(encoded, &envelope); err != nil {
					t.Fatal(err)
				}
				identity := envelope["identity"].(map[string]any)
				identity["board"] = "another-board"
				bad, err := json.Marshal(envelope)
				if err != nil {
					t.Fatal(err)
				}
				forceCorruptSource(t, db, bad)
			},
		},
		{
			name: "content digest mismatch",
			mutate: func(t *testing.T, db *sql.DB, encoded []byte, _ platforms.PostingSource) {
				var envelope map[string]any
				if err := json.Unmarshal(encoded, &envelope); err != nil {
					t.Fatal(err)
				}
				envelope["description_html"] = "<p>changed content</p>"
				bad, err := json.Marshal(envelope)
				if err != nil {
					t.Fatal(err)
				}
				forceCorruptSource(t, db, bad)
			},
		},
		{
			name: "database digest mismatch",
			mutate: func(t *testing.T, db *sql.DB, _ []byte, _ platforms.PostingSource) {
				if _, err := db.Exec(`PRAGMA ignore_check_constraints=ON`); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`UPDATE job_posting_sources SET source_content_sha256=? WHERE job_id=1`, strings.Repeat("0", 64)); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "oversized source",
			mutate: func(t *testing.T, db *sql.DB, _ []byte, _ platforms.PostingSource) {
				forceCorruptSource(t, db, bytes.Repeat([]byte("x"), platforms.MaxPostingSourceJSONBytes+1))
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			a, _ := testWorkspace(t)
			canonical := "Canonical description"
			saveTestJob(t, a, canonical)
			source := sealedTestPostingSource(t, canonical)
			if err := saveJobPostingSource(a.db, 1, testAshbyPostingURL, canonical, testPostingSuccessAt,
				platforms.AvailablePostingSource(source), true, true); err != nil {
				t.Fatal(err)
			}
			encoded, err := platforms.EncodePostingSource(source, testAshbyPostingURL, canonical)
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(t, a.db, encoded, source)
			job, err := a.jobByID(1)
			if err != nil {
				t.Fatal(err)
			}
			state, found, err := loadJobPostingSource(a.db, job)
			if err != nil || !found {
				t.Fatalf("load invalid source: found=%v err=%v", found, err)
			}
			if !state.Invalid || state.Source != nil || state.DescriptionConfirmed || state.SalaryConfirmed {
				t.Fatalf("invalid evidence did not safely fall back: %+v", state)
			}
		})
	}
}

func forceCorruptSource(t *testing.T, db *sql.DB, data []byte) {
	t.Helper()
	if _, err := db.Exec(`PRAGMA ignore_check_constraints=ON`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE job_posting_sources SET source_json=? WHERE job_id=1`, data); err != nil {
		t.Fatal(err)
	}
}

func TestJobPostingSourceWriteAndReadSQLFailuresPropagate(t *testing.T) {
	a, _ := testWorkspace(t)
	canonical := "Canonical description"
	jobID := saveTestJob(t, a, canonical)
	if _, err := a.db.Exec(`CREATE TRIGGER reject_posting_source BEFORE INSERT ON job_posting_sources
		BEGIN SELECT RAISE(ABORT, 'injected source failure'); END`); err != nil {
		t.Fatal(err)
	}
	err := saveJobPostingSource(a.db, jobID, testAshbyPostingURL, canonical, testPostingSuccessAt,
		platforms.AvailablePostingSource(sealedTestPostingSource(t, canonical)), true, true)
	if err == nil || !strings.Contains(err.Error(), "injected source failure") {
		t.Fatalf("source write error = %v", err)
	}
	if _, err := a.db.Exec(`DROP TABLE job_posting_sources`); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(jobID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadJobPostingSource(a.db, job); err == nil || errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("source read SQL error = %v, want propagated error", err)
	}
}

func TestJobPostingSourceSaveRequiresValidatedIdentityAndTimestamp(t *testing.T) {
	a, _ := testWorkspace(t)
	canonical := "Canonical description"
	jobID := saveTestJob(t, a, canonical)
	source := sealedTestPostingSource(t, canonical)
	for _, test := range []struct {
		name, url, at string
	}{
		{name: "wrong URL", url: "https://jobs.ashbyhq.com/other-board/11111111-1111-4111-8111-111111111111", at: testPostingSuccessAt},
		{name: "invalid time", url: testAshbyPostingURL, at: "yesterday"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := saveJobPostingSource(a.db, jobID, test.url, canonical, test.at,
				platforms.AvailablePostingSource(source), true, true); err == nil {
				t.Fatal("invalid source observation was accepted")
			}
		})
	}
}
