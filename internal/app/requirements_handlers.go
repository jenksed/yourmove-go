package app

import (
	"log"
	"net/http"
)

type requirementSummary struct {
	Platform      string
	Source        string
	Label         string
	Kind          string
	Group         string
	JobCount      int
	RequiredCount int
}
type requirementsPageData struct {
	InspectedJobs int
	Questions     []requirementSummary
}

// The inventory groups only exact public wording/type/provider combinations.
// It is observation frequency, never evidence of answer equivalence.
func (a *App) requirementsPage(w http.ResponseWriter, r *http.Request) {
	view := requirementsPageData{}
	if err := a.db.QueryRow(`SELECT COUNT(DISTINCT job_id) FROM application_inspections`).Scan(&view.InspectedJobs); err != nil {
		http.Error(w, "Unable to count inspections", 500)
		return
	}
	rows, err := a.db.Query(`SELECT i.platform,i.source,f.label,f.kind,f.group_name,COUNT(DISTINCT i.job_id),SUM(f.required)
 FROM application_fields f JOIN application_inspections i ON i.id=f.inspection_id
 WHERE i.id=(SELECT ii.id FROM application_inspections ii WHERE ii.job_id=i.job_id AND ii.source=i.source ORDER BY ii.checked_at DESC,ii.id DESC LIMIT 1)
 GROUP BY i.platform,i.source,f.label COLLATE NOCASE,f.kind,f.group_name
 ORDER BY COUNT(DISTINCT i.job_id) DESC, f.label COLLATE NOCASE LIMIT 250`)
	if err != nil {
		http.Error(w, "Unable to load requirements", 500)
		return
	}
	for rows.Next() {
		var q requirementSummary
		if err := rows.Scan(&q.Platform, &q.Source, &q.Label, &q.Kind, &q.Group, &q.JobCount, &q.RequiredCount); err != nil {
			rows.Close()
			http.Error(w, "Unable to read requirements", 500)
			return
		}
		view.Questions = append(view.Questions, q)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		http.Error(w, "Unable to load requirements", 500)
		return
	}
	rows.Close()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := a.resources.requirementsTemplate.Execute(w, view); err != nil {
		log.Printf("render requirements library: %v", err)
	}
}
