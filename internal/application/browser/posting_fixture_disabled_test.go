//go:build !qualification

package browser

import "testing"

func TestNormalBuildRejectsNativeFillFixture(t *testing.T) {
	const fixture = "http://127.0.0.1:47417/kan17-synthetic-application"
	t.Setenv("YOURMOVE_QUALIFY_KAN17", "1")
	if PostingPlatform(fixture) != "" || OwnsPostingURL(fixture, fixture) {
		t.Fatal("normal build accepted the synthetic fixture")
	}
	if _, err := AsResult(fixture, Observation{URL: fixture}); err == nil {
		t.Fatal("normal build accepted capture from the synthetic fixture")
	}
}
