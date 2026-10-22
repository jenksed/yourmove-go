package app

import "fmt"

type answerAIChoice struct{ Identity, Label string }
type answerAIChoices struct {
	Evidence, Writing               []answerAIChoice
	ApplicationResume, MasterResume *answerAIChoice
}

// Bound additional local retrieval; suggestions are listed first and never
// selected automatically. All authoritative reads happen again at review.
func (a *App) answerAIChoices(job Job, q preparedQuestion) *answerAIChoices {
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 || !narrativeAnswerEligible(q) {
		return nil
	}
	out := &answerAIChoices{}
	evidenceIDs := []int64{}
	libraryIDs := []int64{}
	for _, s := range q.CareerEvidenceSuggestions {
		evidenceIDs = append(evidenceIDs, s.Evidence.ID)
	}
	for _, s := range q.LibrarySuggestions {
		libraryIDs = append(libraryIDs, s.EntryID)
	}
	for _, query := range []struct {
		sql    string
		target *[]int64
	}{
		{`SELECT id FROM career_evidence WHERE status='reviewed' ORDER BY updated_at DESC,id DESC LIMIT 30`, &evidenceIDs},
		{`SELECT id FROM answer_library ORDER BY updated_at DESC,id DESC LIMIT 30`, &libraryIDs},
	} {
		rows, err := a.db.Query(query.sql)
		if err != nil {
			continue
		}
		for rows.Next() {
			var id int64
			if rows.Scan(&id) == nil {
				*query.target = append(*query.target, id)
			}
		}
		rows.Close()
	}
	seen := map[int64]bool{}
	for _, id := range evidenceIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		e, err := careerEvidenceByID(a.db, id)
		if err != nil || e.Status != "reviewed" {
			continue
		}
		out.Evidence = append(out.Evidence, answerAIChoice{fmt.Sprintf("%d:%d:%s", e.ID, e.Revision, e.CreatedAt), fmt.Sprintf("%s · revision %d", e.Title, e.Revision)})
	}
	seen = map[int64]bool{}
	for _, id := range libraryIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		e, err := a.answerLibraryBySourceID(a.db, id)
		if err != nil {
			continue
		}
		out.Writing = append(out.Writing, answerAIChoice{fmt.Sprintf("%d:%d:%s", e.ID, e.Revision, e.CreatedAt), fmt.Sprintf("%s · %s · %s · revision %d", e.OriginalQuestion, e.SourceEmployer, e.SourceJobTitle, e.Revision)})
	}
	choice := func(r Resume) *answerAIChoice {
		return &answerAIChoice{fmt.Sprintf("%d:%d:%s:%s", r.ID, r.Version, r.SHA256, r.ImportedAt), fmt.Sprintf("%s · version %d", r.Name, r.Version)}
	}
	if r, err := a.resumeByID(job.SelectedResumeID); err == nil && r.ArchivedAt == "" {
		out.ApplicationResume = choice(r)
	}
	if r, err := a.masterResume(); err == nil && r.ArchivedAt == "" {
		out.MasterResume = choice(r)
	}
	return out
}
