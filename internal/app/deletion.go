package app

import (
	"errors"
	"fmt"
)

const maxDeleteBatch = 50

var ErrJobHasHistory = errors.New("job has confirmed submission history")

// deleteJobs is atomic. Resume files are shared assets and are never deleted.
// The caller must explicitly authorize removal of historical applications.
func (a *App) deleteJobs(ids []int64, allowHistory bool) (int64, error) {
	if len(ids) == 0 || len(ids) > maxDeleteBatch {
		return 0, fmt.Errorf("select 1–%d jobs", maxDeleteBatch)
	}
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return 0, fmt.Errorf("invalid or duplicate job ID")
		}
		seen[id] = true
	}
	// Serialize against single/batch intake: deleted canonical URLs can be re-added.
	a.intakeMu.Lock()
	defer a.intakeMu.Unlock()
	tx, err := a.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	for _, id := range ids {
		var count int64
		err := tx.QueryRow(`SELECT (SELECT COUNT(*) FROM submissions WHERE job_id=?) FROM jobs WHERE id=?`, id, id).Scan(&count)
		if err != nil {
			return 0, err
		}
		if count > 0 && !allowHistory {
			return 0, ErrJobHasHistory
		}
	}
	for _, id := range ids {
		// Answers and inspections are job-local; remove dependents before the job.
		if _, err := tx.Exec(`DELETE FROM application_answers WHERE job_id=?`, id); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(`DELETE FROM application_fields WHERE inspection_id IN (SELECT id FROM application_inspections WHERE job_id=?)`, id); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(`DELETE FROM application_steps WHERE inspection_id IN (SELECT id FROM application_inspections WHERE job_id=?)`, id); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(`DELETE FROM application_inspections WHERE job_id=?`, id); err != nil {
			return 0, err
		}
		if allowHistory {
			if _, err := tx.Exec(`DELETE FROM submissions WHERE job_id=?`, id); err != nil {
				return 0, err
			}
		}
		if _, err := tx.Exec(`DELETE FROM cover_letters WHERE job_id=?`, id); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(`DELETE FROM jobs WHERE id=?`, id); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	for _, id := range ids {
		_ = a.browsers.close(id)
	}
	return int64(len(ids)), nil
}
