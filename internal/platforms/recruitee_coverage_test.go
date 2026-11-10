package platforms

import (
	"strings"
	"testing"
)

// Recruitee public metadata coverage: the XML feed is the only public source
// (no credentials, no /api/offers JSON, no browser Fill). A representative
// posting must import with the useful public information actually available:
// identity, title, employer, description plus requirements, location,
// workplace mode, and employment type. Salary is honestly unavailable: the
// public feed carries no salary fields, so none is invented. Missing,
// malformed, or unavailable feeds fail without touching stored data.
func TestRecruiteeMetadataCoverage(t *testing.T) {
	identity, err := RecruiteePostingIdentity("https://example.recruitee.com/o/platform-engineer")
	if err != nil {
		t.Fatal(err)
	}
	offer := func(remote, hybrid, onSite, employment string) string {
		return `<offers><offer><id>54321</id><slug>platform-engineer</slug><title>Platform Engineer</title><description><![CDATA[<h2>Responsibilities</h2><p>Build safe services.</p>]]></description><requirements><![CDATA[<ul><li>Experience with Go.</li></ul>]]></requirements><company_name>Example Robotics</company_name><careers_url>https://example.recruitee.com/o/platform-engineer</careers_url><city>Berlin</city><country>Germany</country><remote>` + remote + `</remote><hybrid>` + hybrid + `</hybrid><on_site>` + onSite + `</on_site><employment_type_code>` + employment + `</employment_type_code></offer></offers>`
	}
	for _, tc := range []struct{ remote, hybrid, onSite, want string }{
		{"true", "false", "false", "Remote"},
		{"false", "true", "false", "Hybrid"},
		{"false", "false", "true", "OnSite"},
		// Multiple advertised modes must not collapse into a stronger claim.
		{"true", "true", "false", ""},
		{"false", "false", "false", ""},
	} {
		m, err := extractRecruiteeMetadata([]byte(offer(tc.remote, tc.hybrid, tc.onSite, "fulltime_permanent")), identity)
		if err != nil {
			t.Fatalf("remote=%s hybrid=%s on_site=%s: %v", tc.remote, tc.hybrid, tc.onSite, err)
		}
		if m.Title != "Platform Engineer" || m.Company != "Example Robotics" {
			t.Fatalf("identity fields lost: %+v", m)
		}
		if !strings.Contains(m.Description, "Build safe services.") || !strings.Contains(m.Description, "Experience with Go.") {
			t.Fatalf("description or requirements lost: %q", m.Description)
		}
		if m.Source.Source == nil || m.Source.Source.Location != "Berlin, Germany" {
			t.Fatalf("location lost: %+v", m.Source.Source)
		}
		if m.Source.Source.WorkplaceType != tc.want {
			t.Fatalf("remote=%s hybrid=%s on_site=%s workplace=%q want %q", tc.remote, tc.hybrid, tc.onSite, m.Source.Source.WorkplaceType, tc.want)
		}
		if m.Source.Source.EmploymentType != "fulltime_permanent" {
			t.Fatalf("employment type lost: %q", m.Source.Source.EmploymentType)
		}
		// Salary is honestly unavailable from the public feed.
		if m.SalaryText != "" || m.SalaryOrigin != "" {
			t.Fatalf("invented salary: %q origin %q", m.SalaryText, m.SalaryOrigin)
		}
		if err := m.Source.Validate("https://example.recruitee.com/o/platform-engineer", m.Description); err != nil {
			t.Fatalf("source validation: %v", err)
		}
	}
	// Requirements are appended when present; a posting without them still
	// imports with its description.
	withoutRequirements := strings.Replace(offer("false", "true", "false", "fulltime_permanent"), "<requirements><![CDATA[<ul><li>Experience with Go.</li></ul>]]></requirements>", "", 1)
	m, err := extractRecruiteeMetadata([]byte(withoutRequirements), identity)
	if err != nil {
		t.Fatalf("requirements-absent feed: %v", err)
	}
	if !strings.Contains(m.Description, "Build safe services.") {
		t.Fatalf("description lost without requirements: %q", m.Description)
	}
	// A posting without readable title or description is rejected honestly.
	for _, bad := range []string{
		strings.Replace(offer("false", "true", "false", "fulltime_permanent"), "<title>Platform Engineer</title>", "<title></title>", 1),
		strings.Replace(strings.Replace(offer("false", "true", "false", "fulltime_permanent"), "<description><![CDATA[<h2>Responsibilities</h2><p>Build safe services.</p>]]></description>", "<description></description>", 1), "<requirements><![CDATA[<ul><li>Experience with Go.</li></ul>]]></requirements>", "", 1),
	} {
		if _, err := extractRecruiteeMetadata([]byte(bad), identity); err == nil {
			t.Fatal("unreadable Recruitee posting accepted")
		}
	}
}
