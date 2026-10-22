package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

type Inspection struct {
	ID             int64
	JobID          int64
	Platform       string
	Source         string
	Completeness   string
	ApplicationURL string
	Note           string
	Fingerprint    string
	CheckedAt      string
	Fields         []InspectedField
	Steps          []schema.Step
}
type InspectedField struct {
	schema.Field
	Mapping string // exact-profile-field, manual, sensitive
}

func (a *App) saveInspection(jobID int64, r schema.Result) (int64, error) {
	if r.Platform == "" || r.Source == "" || len(r.Fields) > 250 || len(r.Steps) > 30 {
		return 0, fmt.Errorf("invalid inspection result")
	}
	tx, err := a.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var exists bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM jobs WHERE id=?)`, jobID).Scan(&exists); err != nil {
		return 0, err
	}
	if !exists {
		return 0, sql.ErrNoRows
	}
	// Fixed-width nanoseconds keep lexical ordering correct even during rapid reinspection.
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z07:00")
	_, err = tx.Exec(`INSERT INTO application_inspections(job_id,platform,source,completeness,application_url,note,fingerprint,checked_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(job_id,fingerprint) DO UPDATE SET checked_at=excluded.checked_at`, jobID, r.Platform, r.Source, r.Completeness, r.ApplicationURL, r.Note, r.Fingerprint(), now)
	if err != nil {
		return 0, err
	}
	// Query the authoritative key after INSERT or UPSERT UPDATE.
	var id int64
	if err := tx.QueryRow(`SELECT id FROM application_inspections WHERE job_id=? AND fingerprint=?`, jobID, r.Fingerprint()).Scan(&id); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`DELETE FROM application_fields WHERE inspection_id=?`, id); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`DELETE FROM application_steps WHERE inspection_id=?`, id); err != nil {
		return 0, err
	}
	for n, f := range r.Fields {
		choices, _ := json.Marshal(f.Options)
		if len(f.ID) > 250 || len(f.Label) > 500 || len(f.Kind) > 100 || len(f.Group) > 100 || len(choices) > 20000 {
			return 0, fmt.Errorf("inspection field exceeds bound")
		}
		if _, err := tx.Exec(`INSERT INTO application_fields(inspection_id,ordinal,field_id,label,kind,required,sensitive,group_name,options_json) VALUES(?,?,?,?,?,?,?,?,?)`, id, n, f.ID, f.Label, f.Kind, f.Required, f.Sensitive, f.Group, string(choices)); err != nil {
			return 0, err
		}
	}
	for n, s := range r.Steps {
		if len(s.Label) > 400 || len(s.Key) > 150 || len(s.State) > 50 || len(s.Action) > 50 {
			return 0, fmt.Errorf("inspection step exceeds bound")
		}
		if _, err := tx.Exec(`INSERT INTO application_steps(inspection_id,ordinal,step_key,label,state,action) VALUES(?,?,?,?,?,?)`, id, n, s.Key, s.Label, s.State, s.Action); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

func (a *App) latestInspection(jobID int64) (Inspection, error) {
	var id int64
	err := a.db.QueryRow(`SELECT id FROM application_inspections WHERE job_id=? ORDER BY checked_at DESC,id DESC LIMIT 1`, jobID).Scan(&id)
	if err != nil {
		return Inspection{}, err
	}
	return a.inspectionByID(id)
}

// InspectionSummary is history of distinct observed form states, not a claim
// that the live browser survived an application restart.
type InspectionSummary struct {
	ID             int64
	CheckedAt      string
	ApplicationURL string
	FieldCount     int
}

func (a *App) inspectionSummaries(jobID int64, source string, limit int) ([]InspectionSummary, error) {
	if limit < 1 || limit > 30 {
		limit = 12
	}
	rows, err := a.db.Query(`SELECT i.id,i.checked_at,i.application_url,(SELECT COUNT(*) FROM application_fields f WHERE f.inspection_id=i.id) FROM application_inspections i WHERE i.job_id=? AND i.source=? ORDER BY i.checked_at DESC,i.id DESC LIMIT ?`, jobID, source, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var found []InspectionSummary
	for rows.Next() {
		var s InspectionSummary
		if err := rows.Scan(&s.ID, &s.CheckedAt, &s.ApplicationURL, &s.FieldCount); err != nil {
			return nil, err
		}
		found = append(found, s)
	}
	return found, rows.Err()
}
func (a *App) latestInspectionBySource(jobID int64, source string) (Inspection, error) {
	var id int64
	err := a.db.QueryRow(`SELECT id FROM application_inspections WHERE job_id=? AND source=? ORDER BY checked_at DESC,id DESC LIMIT 1`, jobID, source).Scan(&id)
	if err != nil {
		return Inspection{}, err
	}
	return a.inspectionByID(id)
}
func (a *App) inspectionByID(id int64) (Inspection, error) {
	var item Inspection
	err := a.db.QueryRow(`SELECT id,job_id,platform,source,completeness,application_url,note,fingerprint,checked_at FROM application_inspections WHERE id=?`, id).Scan(&item.ID, &item.JobID, &item.Platform, &item.Source, &item.Completeness, &item.ApplicationURL, &item.Note, &item.Fingerprint, &item.CheckedAt)
	if err != nil {
		return item, err
	}
	rows, err := a.db.Query(`SELECT field_id,label,kind,required,sensitive,group_name,options_json FROM application_fields WHERE inspection_id=? ORDER BY ordinal`, id)
	if err != nil {
		return item, err
	}
	for rows.Next() {
		var f InspectedField
		var opts string
		if err := rows.Scan(&f.ID, &f.Label, &f.Kind, &f.Required, &f.Sensitive, &f.Group, &opts); err != nil {
			rows.Close()
			return item, err
		}
		if err := json.Unmarshal([]byte(opts), &f.Options); err != nil {
			rows.Close()
			return item, err
		}
		item.Fields = append(item.Fields, f)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return item, err
	}
	if err := rows.Close(); err != nil {
		return item, err
	}
	steps, err := a.db.Query(`SELECT step_key,label,state,action FROM application_steps WHERE inspection_id=? ORDER BY ordinal`, id)
	if err != nil {
		return item, err
	}
	defer steps.Close()
	for steps.Next() {
		var s schema.Step
		if err := steps.Scan(&s.Key, &s.Label, &s.State, &s.Action); err != nil {
			return item, err
		}
		item.Steps = append(item.Steps, s)
	}
	return item, steps.Err()
}

// Never infer employer-specific answers from labels. Map only stable ATS field IDs
// and only to user-approved, nonempty profile facts. No values are transmitted.
func mapProfileField(f schema.Field, p CandidateProfile) string {
	if !p.Approved || f.Sensitive {
		return "manual"
	}
	id := strings.TrimPrefix(f.ID, "application:")
	values := map[string]string{"first_name": p.FirstName, "last_name": p.LastName, "email": p.Email, "phone": p.Phone}
	if strings.TrimSpace(values[id]) != "" && (f.Kind == "input_text" || f.Kind == "textarea") {
		return "approved profile"
	}
	return "manual"
}
