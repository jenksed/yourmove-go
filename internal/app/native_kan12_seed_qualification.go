//go:build qualification

package app

import "github.com/jenksed/yourmove-go/internal/application/schema"

// seedNativeKan12 prepares the two saved answers and two Answer Library
// examples that the KAN-12 native qualification journey exercises. It runs
// only for a fresh opt-in workspace; restarts must never silently repopulate
// answers the applicant deleted.
func seedNativeKan12(a *App) error {
	// The recognized Ashby posting identity keeps the seeded browser
	// inspection on the live, editable path. Early preview discovery is
	// neutralized by the caller so the desk cannot flip to read-only preview.
	const posting = NativeReactFixturePosting
	if err := a.saveCandidateProfile(CandidateProfile{
		FirstName: "Alex", LastName: "Fixture", Email: "alex@example.test", Approved: true,
	}); err != nil {
		return err
	}
	fields := []schema.Field{
		{ID: "browser:why", Label: "Why do you want to join Fixtureworks? This is a deliberately long employer question to exercise wrapping in the selector, the editor heading, and narrow layouts.", Kind: "textarea"},
		{ID: "browser:challenge", Label: "Describe a challenging technical problem you solved recently and what you learned from the experience in detail.", Kind: "textarea"},
	}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: posting + "/application", Fields: fields}); err != nil {
		return err
	}
	for i, answer := range []string{
		"I want to join Fixtureworks because the team owns reliability end to end, which matches my on-call experience.",
		"I debugged a midnight queue backlog by tracing one poison message, then added a dead-letter queue and alerting.",
	} {
		if _, err := a.db.Exec(`INSERT INTO application_answers(job_id,destination,provider,question_key,label,answer,revision) VALUES(1, ?, 'ashby', ?, ?, ?, 1)`,
			posting, questionKey(posting, "ashby", fields[i]), fields[i].Label, answer); err != nil {
			return err
		}
	}
	libraries := []struct {
		question, answer, employer, title string
	}{
		{
			"Why do you want to join a new engineering team?",
			"I look for teams that own reliability end to end, review incidents openly, and keep on-call sustainable for everyone involved.",
			"Acme Robotics", "Backend Engineer",
		},
		{
			"Describe a hard production incident you resolved",
			"A poison message stalled the queue at midnight; I quarantined it, replayed the backlog, and added dead-letter alerting afterward.",
			"", "",
		},
	}
	for i, entry := range libraries {
		if _, err := a.db.Exec(`INSERT INTO answer_library(original_question,answer,source_employer,source_job_title,source_host,
			source_job_id,source_destination,source_provider,source_question_key,source_revision,source_inspection_id,
			source_inspection_fingerprint,source_field_id,source_field_kind,source_kind,question_terms,answer_terms,
			search_index_version,created_at,updated_at,revision)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1)`,
			entry.question, entry.answer, entry.employer, entry.title, "jobs.ashbyhq.com",
			900+i, posting, "ashby",
			"kan12-question", 1, 900+i, "synthetic", "browser:library", "textarea", "live",
			answerLibraryIndexTerms(entry.question), answerLibraryIndexTerms(entry.answer), answerLibrarySearchIndexVersion,
			answerLibraryTimestamp(), answerLibraryTimestamp()); err != nil {
			return err
		}
	}
	return nil
}
