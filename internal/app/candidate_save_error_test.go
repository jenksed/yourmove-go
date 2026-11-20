package app

import (
	"net/url"
	"strings"
	"testing"
)

// A rejected profile save must never display approval: the error-page
// heading badge reflects the persisted approval state, while the rejected
// entry stays editable. Regression coverage for a native KAN-13 observation
// where an empty approved POST rendered "Approved by you · Revision 0".
func TestRejectedProfileSaveShowsPersistedApproval(t *testing.T) {
	a, _ := testWorkspace(t)

	// Nothing persisted yet: the rejected approval shows Not approved.
	rec := postApp(newMux(a), "/candidate", url.Values{"approve": {"yes"}}, "", "")
	if rec.Code != 400 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Profile was not saved") {
		t.Fatalf("missing not-saved alert: %s", body)
	}
	if strings.Contains(body, "Approved by you") {
		t.Fatalf("unsaved approval displayed: %s", body)
	}
	if !strings.Contains(body, "Not approved") {
		t.Fatalf("missing not-approved badge: %s", body)
	}
	if stored, _ := a.candidateProfile(); stored.Approved {
		t.Fatalf("rejected save persisted approval: %+v", stored)
	}

	// Persist an approved profile, then fail an edit: the badge shows the
	// persisted revision, and the rejected entry stays editable.
	ok := url.Values{"first_name": {"Alex"}, "last_name": {"Fixture"}, "email": {"alex@example.test"}, "approve": {"yes"}}
	if rec := postApp(newMux(a), "/candidate", ok, "", ""); rec.Code != 303 {
		t.Fatalf("approved save status=%d body=%s", rec.Code, rec.Body.String())
	}
	bad := url.Values{"first_name": {""}, "last_name": {"Fixture"}, "email": {"alex@example.test"}, "city": {"Cadillac"}, "approve": {"yes"}}
	rec = postApp(newMux(a), "/candidate", bad, "", "")
	if rec.Code != 400 {
		t.Fatalf("edit status=%d body=%s", rec.Code, rec.Body.String())
	}
	body = rec.Body.String()
	if !strings.Contains(body, "Approved by you · Revision 1") {
		t.Fatalf("persisted approval badge missing: %s", body)
	}
	if !strings.Contains(body, `value="Cadillac"`) {
		t.Fatalf("rejected entry not preserved: %s", body)
	}
	if stored, _ := a.candidateProfile(); !stored.Approved || stored.FirstName != "Alex" || stored.Revision != 1 {
		t.Fatalf("persisted profile changed by rejected edit: %+v", stored)
	}
}
