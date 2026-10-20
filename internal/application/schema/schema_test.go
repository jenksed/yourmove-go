package schema

import "testing"

func TestFingerprintTracksMaterialSourceChanges(t *testing.T) {
	a := Result{Platform: "ashby", Source: "public_api", Completeness: "partial", ApplicationURL: "https://jobs.ashbyhq.com/a/job", Fields: []Field{}, Steps: []Step{{Key: "application", State: "anticipated"}}}
	b := a
	if a.Fingerprint() != b.Fingerprint() {
		t.Fatal("same schema has different fingerprint")
	}
	b.ApplicationURL = "https://jobs.ashbyhq.com/a/job/application"
	if a.Fingerprint() == b.Fingerprint() {
		t.Fatal("changed employer destination reused evidence")
	}
}
