package browser

import "testing"

func TestAshbyPostingOwnership(t *testing.T) {
	saved := "https://jobs.ashbyhq.com/render/e1f2d6a2-b9e2-4f16-b863-3ca228c83e75"
	for _, v := range []string{saved, saved + "/application", saved + "/application?stage=2"} {
		if !OwnsAshbyURL(saved, v) {
			t.Fatalf("rejected owned URL %s", v)
		}
	}
	for _, v := range []string{"https://example.com/application", "https://jobs.ashbyhq.com.evil.test/render/e1f2d6a2-b9e2-4f16-b863-3ca228c83e75", "https://jobs.ashbyhq.com/render/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/application", "http://jobs.ashbyhq.com/render/e1f2d6a2-b9e2-4f16-b863-3ca228c83e75"} {
		if OwnsAshbyURL(saved, v) {
			t.Fatalf("accepted foreign URL %s", v)
		}
	}
}
func TestBrowserResultKeepsStructuralEvidenceOnly(t *testing.T) {
	s := "https://jobs.ashbyhq.com/render/e1f2d6a2-b9e2-4f16-b863-3ca228c83e75"
	o := Observation{URL: s + "/application", Heading: "Apply", Controls: []Control{{ID: "candidate_email", Label: "Email address", Kind: "email", Required: true}, {ID: "sponsorship", Label: "Sponsorship", Sensitive: true}}, Actions: []Action{{Label: "Next", Method: "POST"}}}
	r, err := AsResult(s, o)
	if err != nil {
		t.Fatal(err)
	}
	if r.Source != "browser" || r.Completeness != "rendered_partial" || len(r.Fields) != 2 || !r.Fields[1].Sensitive || len(r.Steps) != 2 || r.Steps[1].Action != "human · submission" {
		t.Fatalf("bad observation: %+v", r)
	}
}
