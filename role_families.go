package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// RoleFamily is an editable, deterministic routing rule. Terms are literal phrases,
// one per line (commas also accepted), never regular expressions or AI prompts.
type RoleFamily struct {
	ID           int64
	Name         string
	TitleTerms   string
	ContextTerms string
	ExcludeTerms string
}

func normalizeMatchText(s string) string {
	var b strings.Builder
	lastSpace := true
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			lastSpace = false
		} else if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func parsedTerms(raw string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == ',' || r == '\r' }) {
		term := normalizeMatchText(part)
		if term != "" && !seen[term] {
			result = append(result, term)
			seen[term] = true
		}
	}
	return result
}

func containsPhrase(normalizedText, normalizedTerm string) bool {
	return normalizedText != "" && normalizedTerm != "" && strings.Contains(" "+normalizedText+" ", " "+normalizedTerm+" ")
}

func validateFamily(f RoleFamily) (RoleFamily, error) {
	f.Name = strings.TrimSpace(f.Name)
	if len(f.Name) < 2 || len(f.Name) > 80 || strings.ContainsAny(f.Name, "\r\n\x00") {
		return f, errors.New("family name must be 2–80 characters on one line")
	}
	for _, field := range []struct {
		label    string
		raw      *string
		required bool
	}{
		{"job-title phrases", &f.TitleTerms, true},
		{"supporting phrases", &f.ContextTerms, false},
		{"excluded job-title phrases", &f.ExcludeTerms, false},
	} {
		if len(*field.raw) > 1600 {
			return f, fmt.Errorf("%s are too long", field.label)
		}
		terms := parsedTerms(*field.raw)
		if field.required && len(terms) == 0 {
			return f, errors.New("at least one job-title phrase is required")
		}
		if len(terms) > 30 {
			return f, fmt.Errorf("%s are limited to 30", field.label)
		}
		for _, term := range terms {
			if len([]rune(term)) < 2 || len(term) > 90 {
				return f, fmt.Errorf("%s must contain 2–90 characters per phrase", field.label)
			}
		}
		// Canonical rules have stable representation across edit forms and matching.
		*field.raw = strings.Join(terms, "\n")
	}
	return f, nil
}

func (a *App) allFamilies() ([]RoleFamily, error) {
	rows, err := a.db.Query(`SELECT id,name,title_terms,context_terms,exclude_terms FROM role_families ORDER BY name COLLATE NOCASE,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	families := []RoleFamily{}
	for rows.Next() {
		var f RoleFamily
		if err := rows.Scan(&f.ID, &f.Name, &f.TitleTerms, &f.ContextTerms, &f.ExcludeTerms); err != nil {
			return nil, err
		}
		families = append(families, f)
	}
	return families, rows.Err()
}

func (a *App) saveFamily(f RoleFamily) error {
	f, err := validateFamily(f)
	if err != nil {
		return err
	}
	if f.ID == 0 {
		_, err = a.db.Exec(`INSERT INTO role_families(name,title_terms,context_terms,exclude_terms) VALUES (?,?,?,?)`, f.Name, f.TitleTerms, f.ContextTerms, f.ExcludeTerms)
	} else {
		var result sql.Result
		result, err = a.db.Exec(`UPDATE role_families SET name=?,title_terms=?,context_terms=?,exclude_terms=? WHERE id=?`, f.Name, f.TitleTerms, f.ContextTerms, f.ExcludeTerms, f.ID)
		if err == nil {
			var n int64
			n, err = result.RowsAffected()
			if err == nil && n == 0 {
				err = sql.ErrNoRows
			}
		}
	}
	return err
}

func (a *App) deleteFamily(id int64) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE resumes SET role_family_id=NULL,is_primary=0 WHERE role_family_id=?`, id); err != nil {
		return err
	}
	result, err := tx.Exec(`DELETE FROM role_families WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

// assignResumeFamily updates routing only: explicit selections on jobs and
// immutable submitted-document snapshots never change.
func (a *App) assignResumeFamily(resumeID, familyID int64, primary bool) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if familyID != 0 {
		var exists bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM role_families WHERE id=?)`, familyID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return sql.ErrNoRows
		}
	}
	var result sql.Result
	result, err = tx.Exec(`UPDATE resumes SET role_family_id=NULLIF(?,0),is_primary=? WHERE id=?`, familyID, primary && familyID != 0, resumeID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	if primary && familyID != 0 {
		if _, err := tx.Exec(`UPDATE resumes SET is_primary=0 WHERE role_family_id=? AND id<>?`, familyID, resumeID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// The examples are intentionally opt-in. They do not change existing rules and
// install no resumes. Users may edit every phrase to fit their own search.
var exampleFamilies = []RoleFamily{
	{Name: "Technical Support Engineering", TitleTerms: "technical support engineer\nsupport engineer\ncustomer support engineer", ContextTerms: "api\ndebugging\nlogs\npython\nsql"},
	{Name: "Linux & Hosting Support", TitleTerms: "linux administrator\nlinux systems administrator\nlinux support engineer\nlinux technical support engineer\nhosting support engineer\nlinux support\nhosting support\nsystems administrator", ContextTerms: "linux\nbash\nnginx\nmysql\ndns"},
	{Name: "Implementation", TitleTerms: "implementation specialist\nimplementation engineer\nimplementation consultant\nonboarding specialist\ntechnical implementation", ContextTerms: "onboarding\nmigration\nconfiguration\ndeployment\nintegration"},
	{Name: "Technical Account Management", TitleTerms: "technical account manager\ntechnical account management\ntam", ContextTerms: "stakeholder\nrenewal\nescalation\naccount management"},
	{Name: "Customer Success", TitleTerms: "customer success manager\ncustomer success engineer\ncustomer success specialist", ContextTerms: "adoption\nrenewal\nonboarding\ncustomer success"},
	{Name: "Application Support", TitleTerms: "application support engineer\napplication support analyst\napplication support specialist", ContextTerms: "incident\nsql\napplication logs\ntroubleshooting"},
	{Name: "Web & Hosting Operations", TitleTerms: "web operations\nweb administrator\nwordpress support\nwebsite operations\nhosting operations", ContextTerms: "wordpress\nphp\nweb server\ndns"},
	{Name: "General Customer Service", TitleTerms: "customer service representative\ncustomer support representative\ncustomer care representative\ncustomer service specialist", ContextTerms: "phone\nemail\nticketing", ExcludeTerms: "engineer\nmanager"},
}

func (a *App) installExampleFamilies() (int, error) {
	tx, err := a.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	count := 0
	for _, f := range exampleFamilies {
		f, err = validateFamily(f)
		if err != nil {
			return 0, err
		}
		r, err := tx.Exec(`INSERT INTO role_families(name,title_terms,context_terms,exclude_terms) VALUES (?,?,?,?) ON CONFLICT(name) DO NOTHING`, f.Name, f.TitleTerms, f.ContextTerms, f.ExcludeTerms)
		if err != nil {
			return 0, err
		}
		n, err := r.RowsAffected()
		if err != nil {
			return 0, err
		}
		count += int(n)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

// MatchRecommendation is advice, not an assigned or employer-sent document.
// Matches require a literal phrase in the saved job TITLE, supporting text only
// breaks ties; an ambiguous match abstains instead of inventing certainty.
type MatchRecommendation struct {
	Status          string // matched, ambiguous, no-title, no-family, no-resume, choose-primary
	FamilyName      string
	ResumeID        int64
	ResumeName      string
	TitleEvidence   string
	ContextEvidence string
}

func matchResume(job Job, families []RoleFamily, resumes []Resume) MatchRecommendation {
	title := normalizeMatchText(job.Title)
	if title == "" {
		return MatchRecommendation{Status: "no-title"}
	}
	context := normalizeMatchText(job.Description)
	type familyScore struct {
		family        RoleFamily
		score         int
		term, context string
	}
	ranked := []familyScore{}
	for _, f := range families {
		excluded := false
		for _, term := range parsedTerms(f.ExcludeTerms) {
			if containsPhrase(title, term) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}
		score, titleTerm, contextTerm := 0, "", ""
		for _, term := range parsedTerms(f.TitleTerms) {
			if containsPhrase(title, term) {
				// Longer title phrases are more specific, not just more frequent.
				candidate := 100 + len(strings.Fields(term))*5 + len(term)
				if candidate > score {
					score, titleTerm = candidate, term
				}
			}
		}
		if score == 0 {
			continue
		}
		contextHits := 0
		for _, term := range parsedTerms(f.ContextTerms) {
			if containsPhrase(context, term) {
				if contextHits < 3 {
					score += 2
				} // Context cannot outweigh title specificity.
				contextHits++
				if contextTerm == "" {
					contextTerm = term
				}
			}
		}
		ranked = append(ranked, familyScore{f, score, titleTerm, contextTerm})
	}
	if len(ranked) == 0 {
		return MatchRecommendation{Status: "no-family"}
	}
	best := ranked[0]
	tied := false
	for _, candidate := range ranked[1:] {
		if candidate.score > best.score {
			best = candidate
			tied = false
		} else if candidate.score == best.score {
			tied = true
		}
	}
	if tied {
		return MatchRecommendation{Status: "ambiguous"}
	}
	result := MatchRecommendation{Status: "no-resume", FamilyName: best.family.Name, TitleEvidence: best.term, ContextEvidence: best.context}
	var candidates []Resume
	for _, r := range resumes {
		if r.RoleFamilyID == best.family.ID {
			candidates = append(candidates, r)
		}
	}
	if len(candidates) == 0 {
		return result
	}
	var chosen *Resume
	for i := range candidates {
		if candidates[i].IsPrimary {
			chosen = &candidates[i]
			break
		}
	}
	if chosen == nil && len(candidates) == 1 {
		chosen = &candidates[0]
	}
	if chosen == nil {
		result.Status = "choose-primary"
		return result
	}
	result.Status = "matched"
	result.ResumeID = chosen.ID
	result.ResumeName = chosen.Name
	return result
}
