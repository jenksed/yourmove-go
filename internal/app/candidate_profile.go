package app

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type CandidateProfile struct {
	FirstName string
	LastName  string
	Email     string
	Phone     string
	City      string
	Region    string
	Country   string
	LinkedIn  string
	GitHub    string
	Portfolio string
	Pronouns  string
	Approved  bool
	Revision  int64
	UpdatedAt string
}

func (a *App) candidateProfile() (CandidateProfile, error) {
	var p CandidateProfile
	err := a.db.QueryRow(`SELECT first_name,last_name,email,phone,city,region,country,linkedin,github,portfolio,pronouns,approved,revision,updated_at FROM candidate_profile WHERE id=1`).Scan(&p.FirstName, &p.LastName, &p.Email, &p.Phone, &p.City, &p.Region, &p.Country, &p.LinkedIn, &p.GitHub, &p.Portfolio, &p.Pronouns, &p.Approved, &p.Revision, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}
	return p, err
}
func (a *App) saveCandidateProfile(p CandidateProfile) error {
	p.FirstName = strings.TrimSpace(p.FirstName)
	p.LastName = strings.TrimSpace(p.LastName)
	p.Email = strings.TrimSpace(p.Email)
	switch p.Pronouns {
	case "", "He/him", "She/her", "They/them":
	default:
		return fmt.Errorf("invalid pronouns preference")
	}
	for _, v := range []string{p.FirstName, p.LastName, p.Email, p.Phone, p.City, p.Region, p.Country, p.LinkedIn, p.GitHub, p.Portfolio} {
		if len(v) > 320 || strings.ContainsAny(v, "\x00\r\n") {
			return fmt.Errorf("invalid profile field (length or line breaks)")
		}
	}
	if p.Approved && (p.FirstName == "" || p.LastName == "" || p.Email == "") {
		return fmt.Errorf("first name, last name and email are required for profile approval")
	}
	if p.Email != "" && (!strings.Contains(p.Email, "@") || strings.ContainsAny(p.Email, " \t")) {
		return fmt.Errorf("email address is invalid")
	}
	_, err := a.db.Exec(`INSERT INTO candidate_profile(id,first_name,last_name,email,phone,city,region,country,linkedin,github,portfolio,pronouns,approved,revision,updated_at)
 VALUES(1,?,?,?,?,?,?,?,?,?,?,?,?,1,?) ON CONFLICT(id) DO UPDATE SET first_name=excluded.first_name,last_name=excluded.last_name,email=excluded.email,phone=excluded.phone,city=excluded.city,region=excluded.region,country=excluded.country,linkedin=excluded.linkedin,github=excluded.github,portfolio=excluded.portfolio,pronouns=excluded.pronouns,approved=excluded.approved,revision=candidate_profile.revision+1,updated_at=excluded.updated_at`, p.FirstName, p.LastName, p.Email, p.Phone, strings.TrimSpace(p.City), strings.TrimSpace(p.Region), strings.TrimSpace(p.Country), strings.TrimSpace(p.LinkedIn), strings.TrimSpace(p.GitHub), strings.TrimSpace(p.Portfolio), p.Pronouns, p.Approved, time.Now().UTC().Format(time.RFC3339))
	return err
}
