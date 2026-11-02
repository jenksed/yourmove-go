package platforms

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type feedRoundTrip func(*http.Request) (*http.Response, error)

func (f feedRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

const personioRecoveryFixture = `<workzag-jobs><position><id>12345</id><subcompany>Example Robotics GmbH</subcompany><office>Berlin</office><name>Platform Engineer</name><jobDescriptions><jobDescription><name>Your responsibilities</name><value><![CDATA[<p>Build reliable applicant-controlled systems.</p><ul><li>Design bounded public services.</li><li>Review changes with the team.</li></ul>]]></value></jobDescription><jobDescription><name>Qualifications</name><value><![CDATA[<p>Experience with Go and SQLite.</p>]]></value></jobDescription></jobDescriptions><schedule>full-time</schedule></position></workzag-jobs>`
const teamtailorRecoveryFixture = `<rss version="2.0" xmlns:tt="https://teamtailor.com/locations"><channel><title>Example Robotics</title><item><title>Platform Engineer</title><description>&lt;h2&gt;Responsibilities&lt;/h2&gt;&lt;p&gt;Build safe services.&lt;/p&gt;&lt;ul&gt;&lt;li&gt;Review changes.&lt;/li&gt;&lt;/ul&gt;</description><link>https://example.teamtailor.com/jobs/12345-platform-engineer</link><guid>public-global-id</guid><remoteStatus>hybrid</remoteStatus><tt:locations><tt:location><tt:city>Berlin</tt:city><tt:country>Germany</tt:country></tt:location></tt:locations></item></channel></rss>`
const recruiteeRecoveryFixture = `<offers><offer><id>54321</id><slug>platform-engineer</slug><title>Platform Engineer</title><description><![CDATA[<h2>Responsibilities</h2><p>Build safe services.</p>]]></description><requirements><![CDATA[<ul><li>Experience with Go.</li></ul>]]></requirements><company_name>Example Robotics</company_name><careers_url>https://example.recruitee.com/o/platform-engineer</careers_url><city>Berlin</city><country>Germany</country><remote>false</remote><hybrid>true</hybrid><on_site>false</on_site><employment_type_code>fulltime_permanent</employment_type_code></offer></offers>`

// Fixtures are synthetic content in structures observed on two public employers
// per provider; they do not reproduce employer prose or claim live acceptance.
func TestPublicFeedProvidersRecoverExactPosting(t *testing.T) {
	for _, tt := range []struct {
		name, url, endpoint, body string
		fetch                     func(context.Context, *http.Client, string) (Metadata, error)
	}{
		{"personio", "https://example.jobs.personio.de/job/12345?language=en", "https://example.jobs.personio.de/xml?language=en", personioRecoveryFixture, fetchPersonio},
		{"teamtailor", "https://example.teamtailor.com/jobs/12345-platform-engineer", "https://example.teamtailor.com/jobs.rss?offset=0&per_page=100", teamtailorRecoveryFixture, fetchTeamtailor},
		{"recruitee", "https://example.recruitee.com/o/platform-engineer", "https://example.recruitee.com/api/feeds/offers.xml", recruiteeRecoveryFixture, fetchRecruitee},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: feedRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != "GET" || r.URL.String() != tt.endpoint {
					t.Fatalf("unexpected request %s %s", r.Method, r.URL)
				}
				if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
					t.Fatal("credentials sent")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tt.body)), Header: make(http.Header)}, nil
			})}
			m, err := tt.fetch(context.Background(), client, tt.url)
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || m.Title != "Platform Engineer" || m.Company == "" || !strings.Contains(m.Description, "services") && tt.name != "personio" {
				t.Fatalf("bad canonical result: %+v", m)
			}
			if m.Source.Source == nil {
				t.Fatal("missing rich source")
			}
			if err := m.Source.Validate(tt.url, m.Description); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(m.Source.Source.DescriptionHTML, "<li>") {
				t.Fatal("list structure lost")
			}
			if m.Source.Source.Location == "" {
				t.Fatal("direct location missing")
			}
		})
	}
}
func TestPublicFeedIdentityRejectsUnsafeAndUnqualifiedVariants(t *testing.T) {
	for _, parse := range []func(string) (PostingIdentity, error){PersonioPostingIdentity, TeamtailorPostingIdentity, RecruiteePostingIdentity} {
		for _, raw := range []string{"http://example.jobs.personio.de/job/12345", "https://user:pass@example.jobs.personio.de/job/12345", "https://example.jobs.personio.de:443/job/12345", "https://example.jobs.personio.de.evil.test/job/12345", "https://127.0.0.1/job/12345", "https://example.jobs.personio.de/job/%31%32%33", "https://jobs.example.com/jobs/12345"} {
			if _, err := parse(raw); err == nil {
				t.Fatalf("accepted unsafe/unqualified %s", raw)
			}
		}
	}
	if _, err := PersonioPostingIdentity("https://example.jobs.personio.de/job/12345?language=en&language=de"); err == nil {
		t.Fatal("accepted ambiguous language")
	}
}
func TestPublicFeedProvidersRejectIdentityAmbiguityAndMalformed(t *testing.T) {
	p, _ := PersonioPostingIdentity("https://example.jobs.personio.de/job/12345")
	duplicate := strings.Replace(personioRecoveryFixture, "</workzag-jobs>", strings.TrimSuffix(strings.TrimPrefix(personioRecoveryFixture, "<workzag-jobs>"), "</workzag-jobs>")+"</workzag-jobs>", 1)
	if _, err := extractPersonioMetadata([]byte(duplicate), p); err == nil {
		t.Fatal("Personio duplicate accepted")
	}
	r, _ := RecruiteePostingIdentity("https://example.recruitee.com/o/platform-engineer")
	if _, err := extractRecruiteeMetadata([]byte(strings.ReplaceAll(recruiteeRecoveryFixture, "example.recruitee.com", "other.recruitee.com")), r); err == nil {
		t.Fatal("Recruitee wrong employer accepted")
	}
	ttid, _ := TeamtailorPostingIdentity("https://example.teamtailor.com/jobs/12345-platform-engineer")
	if _, _, found, err := extractTeamtailorMetadata([]byte(strings.ReplaceAll(teamtailorRecoveryFixture, "example.teamtailor.com", "other.teamtailor.com")), ttid); err != nil || found {
		t.Fatal("Teamtailor wrong employer matched")
	}
	for _, bad := range []string{"<offers>", `<!DOCTYPE offers [<!ENTITY x SYSTEM "file:///etc/passwd">]><offers><offer>&x;</offer></offers>`} {
		if _, err := extractRecruiteeMetadata([]byte(bad), r); err == nil {
			t.Fatal("invalid XML accepted")
		}
	}
}
func TestPublicJobFeedRejectsRedirectAndOversize(t *testing.T) {
	for _, status := range []int{301, 401, 403, 404, 410, 503} {
		calls := 0
		client := &http.Client{Transport: feedRoundTrip(func(r *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: status, Header: http.Header{"Location": []string{"https://private.invalid/"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		})}
		_, err := readPublicJobFeed(context.Background(), client, "https://example.recruitee.com/api/feeds/offers.xml")
		if err == nil || calls != 1 {
			t.Fatalf("status %d err=%v calls=%d", status, err, calls)
		}
	}
	client := &http.Client{Transport: feedRoundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(strings.Repeat("x", maxPostingResponse+1)))}, nil
	})}
	if _, err := readPublicJobFeed(context.Background(), client, "https://example.recruitee.com/api/feeds/offers.xml"); err == nil {
		t.Fatal("oversize accepted")
	}
}

const icimsRecoveryFixture = `<!doctype html><script type="application/ld+json">{"@type":"JobPosting","url":"https://example.icims.com/jobs/12345/platform-engineer/job","title":"Platform Engineer","description":"<h2>Responsibilities</h2><p>Build safe services.</p><ul><li>Review changes.</li></ul>","hiringOrganization":{"name":"Example Robotics"},"employmentType":"FULL_TIME","jobLocation":[{"address":{"addressLocality":"Berlin","addressCountry":"Germany"}}]}</script>`

func TestICIMSClassicPostingSource(t *testing.T) {
	identity, err := ICIMSPostingIdentity("https://example.icims.com/jobs/12345/platform%2C-engineer/job")
	if err != nil {
		t.Fatal(err)
	}
	m, err := extractICIMSMetadata([]byte(icimsRecoveryFixture), identity)
	if err != nil {
		t.Fatal(err)
	}
	if m.Company != "Example Robotics" || m.Title != "Platform Engineer" || !strings.Contains(m.Description, "services") {
		t.Fatalf("unexpected %+v", m)
	}
	if err := m.Source.Validate("https://example.icims.com/jobs/12345/job", m.Description); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{strings.ReplaceAll(icimsRecoveryFixture, "example.icims.com", "other.icims.com"), strings.ReplaceAll(icimsRecoveryFixture, "12345", "12346"), icimsRecoveryFixture + icimsRecoveryFixture} {
		if _, err := extractICIMSMetadata([]byte(bad), identity); err == nil {
			t.Fatal("ambiguous or wrong identity accepted")
		}
	}
	for _, bad := range []string{"https://example.icims.com/jobs/12345/foo%2Fbar/job", "https://user@example.icims.com/jobs/12345/job", "https://example.icims.com:443/jobs/12345/job", "https://example.icims.com/jobs/12345/../job", "https://api.icims.com/customers/12345/portalposts/job/12345"} {
		if _, err := ICIMSPostingIdentity(bad); err == nil {
			t.Fatal("unsafe or API URL accepted", bad)
		}
	}
}

func TestTeamtailorBoundedPagination(t *testing.T) {
	raw := "https://example.teamtailor.com/jobs/12345-platform-engineer"
	filler := strings.Repeat(`<item><title>Another role</title><link>https://example.teamtailor.com/jobs/999-other</link></item>`, 100)
	page1 := `<rss><channel><title>Example Robotics</title>` + filler + `</channel></rss>`
	calls := 0
	client := &http.Client{Transport: feedRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		body := page1
		if r.URL.Query().Get("offset") == "100" {
			body = teamtailorRecoveryFixture
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	if _, err := fetchTeamtailor(context.Background(), client, raw); err != nil || calls != 2 {
		t.Fatalf("page2 err=%v calls=%d", err, calls)
	}
	calls = 0
	client.Transport = feedRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(page1))}, nil
	})
	if _, err := fetchTeamtailor(context.Background(), client, raw); err == nil || calls != 2 {
		t.Fatalf("unbounded pages or false success: err=%v calls=%d", err, calls)
	}
}
func TestPublicFeedTitleBounds(t *testing.T) {
	identity, _ := PersonioPostingIdentity("https://example.jobs.personio.de/job/12345")
	bad := strings.Replace(personioRecoveryFixture, "<name>Platform Engineer</name>", "<name>"+strings.Repeat("x", 201)+"</name>", 1)
	if _, err := extractPersonioMetadata([]byte(bad), identity); err == nil {
		t.Fatal("oversize title accepted")
	}
}
