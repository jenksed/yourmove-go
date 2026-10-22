package main

import (
	"database/sql"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func TestCandidatePronounsMigrationAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// Literal legacy schema: no pronouns column. All original values and approval survive.
	_, err = db.Exec(`CREATE TABLE candidate_profile(id INTEGER PRIMARY KEY,first_name TEXT,last_name TEXT,email TEXT,phone TEXT,city TEXT,region TEXT,country TEXT,linkedin TEXT,github TEXT,portfolio TEXT,approved INTEGER,revision INTEGER,updated_at TEXT);
 INSERT INTO candidate_profile VALUES(1,'Casey','Fixture','casey@example.test','555-0100','Detroit','Michigan','US','https://linkedin.example.test','https://github.example.test','https://portfolio.example.test',1,7,'legacy timestamp')`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	want := CandidateProfile{FirstName: "Casey", LastName: "Fixture", Email: "casey@example.test", Phone: "555-0100", City: "Detroit", Region: "Michigan", Country: "US", LinkedIn: "https://linkedin.example.test", GitHub: "https://github.example.test", Portfolio: "https://portfolio.example.test", Approved: true, Revision: 7, UpdatedAt: "legacy timestamp"}
	for i := 0; i < 2; i++ {
		db, err = openDB(path)
		if err != nil {
			t.Fatal(err)
		}
		a := &App{db: db}
		got, err := a.candidateProfile()
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("restart %d: %+v %v", i, got, err)
		}
		db.Close()
	}
	db, err = openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{db: db}
	// Approval is explicit for each new revision; a draft cannot reuse old approval.
	form := url.Values{"first_name": {"Casey"}, "last_name": {"Fixture"}, "email": {"casey@example.test"}, "pronouns": {"He/him"}}
	rec := postApp(newMux(a), "/candidate", form, "", "")
	if rec.Code != 303 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	draft, _ := a.candidateProfile()
	if draft.Approved || draft.Pronouns != "He/him" || draft.Revision != 8 {
		t.Fatalf("draft: %+v", draft)
	}
	form.Set("approve", "yes")
	rec = postApp(newMux(a), "/candidate", form, "", "")
	if rec.Code != 303 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	approved, _ := a.candidateProfile()
	db.Close()
	db, err = openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a.db = db
	got, err := a.candidateProfile()
	if err != nil || !reflect.DeepEqual(got, approved) || !got.Approved || got.Revision != 9 {
		t.Fatalf("approved restart: %+v %v", got, err)
	}
	form.Set("pronouns", "inferred")
	rec = postApp(newMux(a), "/candidate", form, "", "")
	if rec.Code != 400 {
		t.Fatal("invalid preference accepted")
	}
	page := httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/candidate", nil))
	if page.Code != 200 {
		t.Fatal(page.Code)
	}
}

func TestAdditionalApprovedCandidateFacts(t *testing.T) {
	p := CandidateProfile{Approved: true, City: "Detroit", Region: "Michigan", Country: "United States", LinkedIn: "https://linkedin.example.test", GitHub: "https://github.example.test", Portfolio: "https://portfolio.example.test", Pronouns: "He/him"}
	for _, tc := range []struct{ id, label, kind, value string }{
		{"city", "City", "text", p.City}, {"state", "State", "text", p.Region}, {"region", "State / region", "text", p.Region}, {"country", "Country", "text", p.Country},
		{"linkedin", "LinkedIn URL", "url", p.LinkedIn}, {"github_url", "GitHub profile URL", "text", p.GitHub}, {"portfolio", "Portfolio or work sample URL", "url", p.Portfolio},
		{"arbitrary-stable-id", "What are your pronouns?", "text", "He/him"}, {"pronouns", "Preferred pronouns", "textarea", "He/him"},
		{"city", "Preferred city for relocation", "text", ""}, {"country", "Country of citizenship", "text", ""}, {"linkedin", "Manager LinkedIn URL", "url", ""},
		{"portfolio", "Portfolio URL", "email", ""}, {"pronouns", "Pronouns", "checkbox", ""}, {"pronouns", "Pronouns", "combobox", ""}, {"question-1", "Pronouns", "text", ""},
	} {
		t.Run(tc.id+tc.label+tc.kind, func(t *testing.T) {
			f := schema.Field{ID: "browser:" + tc.id, Label: tc.label, Kind: tc.kind}
			got, _ := resolveApplicationAnswer(f, "", false, p, "", false)
			if got != tc.value {
				t.Fatalf("got %q want %q", got, tc.value)
			}
			f.Sensitive = true
			if got, _ := resolveApplicationAnswer(f, "", false, p, "", false); got != "" {
				t.Fatal("sensitive fact used")
			}
			f.Sensitive = false
			unapproved := p
			unapproved.Approved = false
			if got, _ := resolveApplicationAnswer(f, "", false, unapproved, "", false); got != "" {
				t.Fatal("draft used")
			}
			if got, _ := resolveApplicationAnswer(f, "override", true, p, "", false); got != "override" {
				t.Fatal("override lost")
			}
			if got, _ := resolveApplicationAnswer(f, "", true, p, "", false); got != "" {
				t.Fatal("explicit blank lost")
			}
		})
	}
	for _, kind := range []string{"select", "radio"} {
		for _, option := range []string{"He/him", "He/Him", "He/Him/His", " he / HIM / his "} {
			f := schema.Field{ID: "browser:pronouns", Label: "Pronouns", Kind: kind, Options: []string{"Choose", option, "She/Her"}}
			got, _ := resolveApplicationAnswer(f, "", false, p, "", false)
			if got != option {
				t.Fatalf("%s %q -> %q", kind, option, got)
			}
		}
	}
	for _, options := range [][]string{{"He/Him", "He/Him/His"}, {"He/Him", "He/Him"}, {"He/him or they/them"}, {"He"}, {"Prefer not to say"}, {"He/Him (male)"}} {
		f := schema.Field{ID: "browser:pronouns", Label: "Pronouns", Kind: "select", Options: options}
		if got, _ := resolveApplicationAnswer(f, "", false, p, "", false); got != "" {
			t.Fatalf("ambiguous/unreliable options: %v", options)
		}
	}
	for _, preference := range []string{"She/her", "They/them"} {
		q := p
		q.Pronouns = preference
		f := schema.Field{ID: "browser:pronouns", Label: "Pronouns", Kind: "select", Options: []string{"He/Him", preference}}
		if got, _ := resolveApplicationAnswer(f, "", false, q, "", false); got != preference {
			t.Fatal(preference, got)
		}
	}
	p.Pronouns = ""
	if got, _ := resolveApplicationAnswer(schema.Field{ID: "browser:pronouns", Label: "Pronouns", Kind: "text"}, "", false, p, "", false); got != "" {
		t.Fatal("empty pronouns inferred")
	}
}

func TestProfileFactsRemainPendingAfterContacts(t *testing.T) {
	p := CandidateProfile{Approved: true, Email: "casey@example.test", Pronouns: "He/him", LinkedIn: "https://linkedin.example.test"}
	var ready []preparedQuestion
	for _, f := range []schema.Field{{ID: "browser:email", Label: "Email", Kind: "email"}, {ID: "browser:pronouns", Label: "Pronouns", Kind: "text"}, {ID: "browser:linkedin", Label: "LinkedIn URL", Kind: "url"}} {
		value, source := resolveApplicationAnswer(f, "", false, p, "", false)
		ready = append(ready, preparedQuestion{Field: f, Value: value, Source: source, Key: f.ID, Editable: true})
	}
	pending := pendingAnswers(ready, nil, true)
	if len(pending) != 2 || pending[0].Field.ID != "browser:pronouns" || pending[1].Field.ID != "browser:linkedin" {
		t.Fatalf("wrong pending facts: %+v", pending)
	}
}

func TestApprovedContactAliasesMatchBrowserFiller(t *testing.T) {
	p := CandidateProfile{Approved: true, FirstName: "Casey", LastName: "Fixture", Email: "casey@example.test", Phone: "202-555-0104"}
	for _, tc := range []struct{ id, label, kind, want string }{
		{"name", "Name", "text", "Casey Fixture"}, {"full_name", "Full name *", "text", "Casey Fixture"}, {"_systemfield_name", "Your name (required)", "text", "Casey Fixture"},
		{"_systemfield_first_name", "Given name", "text", "Casey"}, {"surname", "Surname", "text", "Fixture"}, {"email_address", "Email address", "email", p.Email}, {"_systemfield_phone", "Mobile number", "tel", p.Phone},
		{"name", "Manager name", "text", ""}, {"name", "Name", "email", ""}, {"manager_name", "Name", "text", ""}, {"question-1", "Name", "text", ""},
	} {
		t.Run(tc.id+tc.label+tc.kind, func(t *testing.T) {
			f := schema.Field{ID: "browser:" + tc.id, Label: tc.label, Kind: tc.kind}
			value, source := resolveApplicationAnswer(f, "", false, p, "", false)
			if value != tc.want {
				t.Fatalf("got %q want %q", value, tc.want)
			}
			if tc.want != "" {
				if source != "Approved candidate information" || !(preparedQuestion{Field: f, Value: value, Source: source}).profileContact() {
					t.Fatal("contact not classified consistently")
				}
				q := preparedQuestion{Field: f, Value: value, Source: source, Editable: true}
				if len(pendingAnswers([]preparedQuestion{q}, nil, true)) != 0 {
					t.Fatal("contact already handled by handoff offered for replay")
				}
			}
			unapproved := p
			unapproved.Approved = false
			if value, _ := resolveApplicationAnswer(f, "", false, unapproved, "", false); value != "" {
				t.Fatal("unapproved profile reused")
			}
			f.Sensitive = true
			if value, _ := resolveApplicationAnswer(f, "", false, p, "", false); value != "" {
				t.Fatal("sensitive control reused")
			}
			if value, _ := resolveApplicationAnswer(f, "override", true, p, "", false); value != "override" {
				t.Fatal("application override lost")
			}
		})
	}
}
