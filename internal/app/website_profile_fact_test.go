package app

import (
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func TestWebsiteUsesEligibleCandidateProfileURLWithoutDuplicatingGitHub(t *testing.T) {
	website := schema.Field{ID: "browser:website", Label: "Website", Kind: "url", Required: true}
	github := schema.Field{ID: "browser:github_url", Label: "GitHub profile URL", Kind: "url"}
	linkedin := schema.Field{ID: "browser:linkedin", Label: "LinkedIn Profile Link", Kind: "text"}
	if got := additionalProfileAttribute(website); got != "website" {
		t.Fatalf("Website role=%q want profile website", got)
	}
	if got := additionalProfileAttribute(github); got != "github" {
		t.Fatalf("GitHub role=%q want github", got)
	}
	if got := additionalProfileAttribute(linkedin); got != "linkedin" {
		t.Fatalf("LinkedIn role=%q want linkedin", got)
	}

	for _, tc := range []struct {
		name, portfolio, github string
		separateGitHub          bool
		wantWebsite             string
	}{
		{name: "portfolio wins even with separate GitHub", portfolio: "https://portfolio.example.test", github: "https://github.example.test", separateGitHub: true, wantWebsite: "https://portfolio.example.test"},
		{name: "portfolio wins without separate GitHub", portfolio: "https://portfolio.example.test", github: "https://github.example.test", wantWebsite: "https://portfolio.example.test"},
		{name: "GitHub fallback without separate destination", github: "https://github.example.test", wantWebsite: "https://github.example.test"},
		{name: "no duplicate when separate GitHub destination exists", github: "https://github.example.test", separateGitHub: true, wantWebsite: ""},
		{name: "no eligible URL", wantWebsite: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profile := CandidateProfile{Approved: true, Portfolio: tc.portfolio, GitHub: tc.github, LinkedIn: "https://linkedin.example.test"}
			fields := []schema.Field{website}
			if tc.separateGitHub {
				fields = append(fields, github)
			}
			fields = append(fields, linkedin)
			hasSeparateGitHub := false
			for _, field := range fields {
				if classifyApplicationField(field, true).Role == fieldCandidateProfile && additionalProfileAttribute(field) == "github" {
					hasSeparateGitHub = true
				}
			}
			value, source := resolveApplicationAnswerForWebsite(website, "", false, profile, "", false, hasSeparateGitHub)
			if value != tc.wantWebsite {
				t.Fatalf("Website value=%q want %q", value, tc.wantWebsite)
			}
			if value == "" && source != "" {
				t.Fatalf("empty Website has source %q", source)
			}
			if value != "" && source != "Approved candidate information" {
				t.Fatalf("Website source=%q", source)
			}
			if tc.separateGitHub {
				githubValue, githubSource := resolveApplicationAnswer(github, "", false, profile, "", false)
				if githubValue != tc.github || githubSource != "Approved candidate information" {
					t.Fatalf("GitHub value/source=%q/%q", githubValue, githubSource)
				}
			}
			linkedinValue, linkedinSource := resolveApplicationAnswer(linkedin, "", false, profile, "", false)
			if linkedinValue != profile.LinkedIn || linkedinSource != "Approved candidate information" {
				t.Fatalf("LinkedIn value/source=%q/%q", linkedinValue, linkedinSource)
			}
		})
	}
}

func TestRequiredWebsiteWithoutEligibleProfileURLStaysManual(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveCandidateProfile(CandidateProfile{FirstName: "Alex", LastName: "Fixture", Email: "alex@example.test", Approved: true}); err != nil {
		t.Fatal(err)
	}
	website := schema.Field{ID: "browser:website", Label: "Website", Kind: "url", Required: true}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: testAshbyPosting + "/application", Fields: []schema.Field{website}}); err != nil {
		t.Fatal(err)
	}
	job, _ := a.jobByID(1)
	profile, _ := a.candidateProfile()
	prepared, err := a.prepareQuestions(job, profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.ProfileQuestions()) != 1 || prepared.ProfileQuestions()[0].Value != "" || prepared.ProfileQuestions()[0].Field.Required != true {
		t.Fatalf("required Website not preserved for review: %+v", prepared.ProfileQuestions())
	}

	projection, err := a.buildContinuation(1)
	if err != nil {
		t.Fatal(err)
	}
	if got := projection["pendingAnswers"].([]continuationAnswerJSON); len(got) != 0 {
		t.Fatalf("missing Website entered Fill scope: %+v", got)
	}
	manual := projection["manualProfileFacts"].([]continuationManualProfileFactJSON)
	if len(manual) != 1 || manual[0].Label != "Website" {
		t.Fatalf("required Website not surfaced for manual review: %+v", manual)
	}
}

func TestPrepareQuestionsResolvesWebsiteFromWholeObservedForm(t *testing.T) {
	for _, tc := range []struct {
		name, portfolio, github string
		separateGitHub          bool
		wantWebsite             string
	}{
		{name: "GitHub fallback only when there is no separate destination", github: "https://github.example.test", wantWebsite: "https://github.example.test"},
		{name: "separate GitHub leaves Website blank", github: "https://github.example.test", separateGitHub: true},
		{name: "Portfolio wins when both destinations exist", portfolio: "https://portfolio.example.test", github: "https://github.example.test", separateGitHub: true, wantWebsite: "https://portfolio.example.test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := testWorkspace(t)
			if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
				t.Fatal(err)
			}
			profile := CandidateProfile{FirstName: "Alex", LastName: "Fixture", Email: "alex@example.test", Portfolio: tc.portfolio, GitHub: tc.github, Approved: true}
			if err := a.saveCandidateProfile(profile); err != nil {
				t.Fatal(err)
			}
			fields := []schema.Field{{ID: "browser:website", Label: "Website", Kind: "url", Required: true}}
			if tc.separateGitHub {
				fields = append(fields, schema.Field{ID: "browser:github_url", Label: "GitHub profile URL", Kind: "url"})
			}
			if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: testAshbyPosting + "/application", Fields: fields}); err != nil {
				t.Fatal(err)
			}
			job, _ := a.jobByID(1)
			prepared, err := a.prepareQuestions(job, profile)
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]string{}
			for _, q := range prepared.Questions {
				got[q.Field.Label] = q.Value
			}
			if got["Website"] != tc.wantWebsite {
				t.Fatalf("prepared Website=%q want %q", got["Website"], tc.wantWebsite)
			}
			if tc.separateGitHub && got["GitHub profile URL"] != tc.github {
				t.Fatalf("prepared GitHub=%q want %q", got["GitHub profile URL"], tc.github)
			}
		})
	}
}

func TestEarlyPreviewWebsiteAvoidsDuplicatingSeparateGitHub(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
		t.Fatal(err)
	}
	profile := CandidateProfile{FirstName: "Alex", LastName: "Fixture", Email: "alex@example.test", GitHub: "https://github.example.test", Approved: true}
	if err := a.saveCandidateProfile(profile); err != nil {
		t.Fatal(err)
	}
	fields := []schema.Field{
		{ID: "preview:website", Label: "Website", Kind: "url"},
		{ID: "preview:github", Label: "GitHub profile URL", Kind: "url"},
	}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "preview", ApplicationURL: testAshbyPosting + "/application", Fields: fields}); err != nil {
		t.Fatal(err)
	}
	job, _ := a.jobByID(1)
	prepared, err := a.prepareEarlyQuestions(job, nil)
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, q := range prepared.Questions {
		values[q.Field.Label] = q.ProfileValue
	}
	if values["Website"] != "" || values["GitHub profile URL"] != profile.GitHub {
		t.Fatalf("preview profile values duplicated GitHub: %+v", values)
	}
}
