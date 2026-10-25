package platforms

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const (
	testAshbyPostingURL = "https://jobs.ashbyhq.com/fable/11111111-1111-4111-8111-111111111111"
	testCanonicalText   = "Role\n\nA plain canonical description."
)

func testPostingSource(t *testing.T) PostingSource {
	t.Helper()
	identity, err := AshbyPostingIdentity(testAshbyPostingURL)
	if err != nil {
		t.Fatal(err)
	}
	source, err := SealPostingSource(PostingSource{
		Version:          PostingSourceFormatVersion,
		Identity:         identity,
		DescriptionHTML:  `<h2>Role</h2><p>A <strong>plain</strong> canonical description.</p>`,
		DescriptionPlain: testCanonicalText,
		Location:         "Remote - European Union",
		WorkplaceType:    "Remote",
		EmploymentType:   "FullTime",
		SalarySummary:    "€120K - €150K",
	}, testCanonicalText)
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func testPostingSourceForIdentity(t *testing.T, identity PostingIdentity) PostingSource {
	t.Helper()
	source, err := SealPostingSource(PostingSource{
		Version:          PostingSourceFormatVersion,
		Identity:         identity,
		DescriptionHTML:  `<h2>Role</h2><p>A provider-qualified source.</p>`,
		DescriptionPlain: testCanonicalText,
	}, testCanonicalText)
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func TestPostingSourceIdentityBindsProviderBoardAndPosting(t *testing.T) {
	source := testPostingSource(t)
	if err := source.Validate(testAshbyPostingURL+"?source=fixture#overview", testCanonicalText); err != nil {
		t.Fatalf("query/fragment changed posting identity: %v", err)
	}
	for _, rawURL := range []string{
		"https://jobs.ashbyhq.com/other/11111111-1111-4111-8111-111111111111",
		"https://jobs.ashbyhq.com/fable/22222222-2222-4222-8222-222222222222",
		"http://jobs.ashbyhq.com/fable/11111111-1111-4111-8111-111111111111",
		"https://jobs.ashbyhq.com.evil.test/fable/11111111-1111-4111-8111-111111111111",
		"https://user@jobs.ashbyhq.com/fable/11111111-1111-4111-8111-111111111111",
		"https://jobs.ashbyhq.com:443/fable/11111111-1111-4111-8111-111111111111",
	} {
		t.Run(rawURL, func(t *testing.T) {
			if err := source.Validate(rawURL, testCanonicalText); err == nil {
				t.Fatal("invalid or mismatched URL was accepted")
			}
		})
	}
	if err := source.Validate(testAshbyPostingURL, testCanonicalText+" changed"); !errors.Is(err, ErrPostingSourceDigest) {
		t.Fatalf("canonical mismatch error = %v", err)
	}
}

func TestPostingSourceIdentitySupportsGreenhouseAliasesAndCodec(t *testing.T) {
	identity := PostingIdentity{Provider: "greenhouse", Board: "acme", PostingID: "1234567"}
	source := testPostingSourceForIdentity(t, identity)
	for _, rawURL := range []string{
		"https://job-boards.greenhouse.io/acme/jobs/1234567?gh_src=email#description",
		"https://boards.greenhouse.io/acme/jobs/1234567?gh_src=email",
	} {
		if err := source.Validate(rawURL, testCanonicalText); err != nil {
			t.Errorf("qualified Greenhouse URL %q failed source validation: %v", rawURL, err)
		}
	}
	encoded, err := EncodePostingSource(source, "https://job-boards.greenhouse.io/acme/jobs/1234567", testCanonicalText)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodePostingSource(encoded, "https://boards.greenhouse.io/acme/jobs/1234567", testCanonicalText)
	if err != nil || decoded.Identity != identity {
		t.Fatalf("Greenhouse source codec identity = %+v, err=%v", decoded.Identity, err)
	}

	for _, rawURL := range []string{
		"https://job-boards.greenhouse.io/other/jobs/1234567",
		"https://job-boards.greenhouse.io/acme/jobs/1234568",
		"http://job-boards.greenhouse.io/acme/jobs/1234567",
		"https://job-boards.greenhouse.io.evil.test/acme/jobs/1234567",
		"https://user@job-boards.greenhouse.io/acme/jobs/1234567",
		"https://job-boards.greenhouse.io:443/acme/jobs/1234567",
		"https://job-boards.greenhouse.io/acme/jobs/1234567%2Fother",
	} {
		if err := source.Validate(rawURL, testCanonicalText); err == nil {
			t.Errorf("mismatched or untrusted Greenhouse URL accepted: %q", rawURL)
		}
	}
}

func TestPostingSourceIdentityBindsLeverSiteRegionAndPosting(t *testing.T) {
	tests := []struct {
		name     string
		region   string
		posting  string
		mismatch string
	}{
		{
			name:     "global",
			region:   "global",
			posting:  "https://jobs.lever.co/acme/abc-123",
			mismatch: "https://jobs.eu.lever.co/acme/abc-123",
		},
		{
			name:     "eu",
			region:   "eu",
			posting:  "https://jobs.eu.lever.co/acme/abc-123",
			mismatch: "https://jobs.lever.co/acme/abc-123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identity := PostingIdentity{Provider: "lever", Board: "acme", Region: tt.region, PostingID: "abc-123"}
			source := testPostingSourceForIdentity(t, identity)
			if err := source.Validate(tt.posting+"?lever-source=fixture#description", testCanonicalText); err != nil {
				t.Fatalf("valid Lever %s source: %v", tt.name, err)
			}
			for _, rawURL := range []string{
				tt.mismatch,
				"https://jobs.lever.co/other/abc-123",
				"https://jobs.lever.co/acme/changed-id",
				"http://jobs.lever.co/acme/abc-123",
				"https://jobs.lever.co.evil.test/acme/abc-123",
				"https://user@jobs.lever.co/acme/abc-123",
				"https://jobs.lever.co:443/acme/abc-123",
				"https://jobs.lever.co/acme/abc-123%2Fother",
			} {
				if err := source.Validate(rawURL, testCanonicalText); err == nil {
					t.Errorf("mismatched or untrusted Lever URL accepted: %q", rawURL)
				}
			}
		})
	}
}

func TestPostingSourceIdentityRejectsUnsupportedProviderAndRegion(t *testing.T) {
	for _, identity := range []PostingIdentity{
		{Provider: "paylocity", Board: "acme", PostingID: "abc"},
		{Provider: "greenhouse", Board: "acme", Region: "eu", PostingID: "1234567"},
		{Provider: "lever", Board: "acme", PostingID: "abc-123"},
		{Provider: "lever", Board: "acme", Region: "uk", PostingID: "abc-123"},
	} {
		_, err := SealPostingSource(PostingSource{
			Version:          PostingSourceFormatVersion,
			Identity:         identity,
			DescriptionPlain: testCanonicalText,
		}, testCanonicalText)
		if !errors.Is(err, ErrPostingSourceInvalid) {
			t.Errorf("unsupported identity %+v error = %v, want ErrPostingSourceInvalid", identity, err)
		}
	}

	serialized, err := json.Marshal(PostingIdentity{Provider: "ashby", Board: "fable", PostingID: "11111111-1111-4111-8111-111111111111"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serialized), `"region"`) {
		t.Fatalf("empty optional region altered legacy Ashby identity JSON: %s", serialized)
	}
}

func TestAshbyIdentitySupportsMissingOptionalJSONIDWithMatchingJobURL(t *testing.T) {
	const response = `{"jobs":[{"jobUrl":"https://jobs.ashbyhq.com/fable/11111111-1111-4111-8111-111111111111"}]}`
	var board struct {
		Jobs []struct {
			ID     string `json:"id"`
			JobURL string `json:"jobUrl"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(response), &board); err != nil {
		t.Fatal(err)
	}
	if len(board.Jobs) != 1 || board.Jobs[0].ID != "" {
		t.Fatalf("fixture no longer represents the missing-ID case: %+v", board.Jobs)
	}
	identity, err := AshbyPostingIdentity(board.Jobs[0].JobURL)
	if err != nil {
		t.Fatalf("qualified matching jobUrl identity: %v", err)
	}
	if err := identity.ValidateURL(testAshbyPostingURL); err != nil {
		t.Fatalf("jobUrl did not bind to the saved posting: %v", err)
	}
}

func TestPostingSourceDigestIsStableAndTracksSourceContent(t *testing.T) {
	source := testPostingSource(t)
	first := source.SourceContentSHA256
	resealed, err := SealPostingSource(source, testCanonicalText)
	if err != nil {
		t.Fatal(err)
	}
	if resealed.SourceContentSHA256 != first {
		t.Fatalf("digest changed on identical source: %s != %s", resealed.SourceContentSHA256, first)
	}

	// Observation timestamps are owned by app/SQLite, not by this source type;
	// any two fetch observations of unchanged content therefore share a digest.
	type observation struct {
		Source    PostingSource
		FetchedAt string
	}
	firstFetch := observation{Source: source, FetchedAt: "2026-10-24T10:00:00Z"}
	secondFetch := observation{Source: resealed, FetchedAt: "2026-10-24T11:00:00Z"}
	if firstFetch.FetchedAt == secondFetch.FetchedAt || firstFetch.Source.SourceContentSHA256 != secondFetch.Source.SourceContentSHA256 {
		t.Fatal("fetch/check time affected the source-content digest")
	}

	changed := source
	changed.DescriptionHTML += "<p>New employer content.</p>"
	changed, err = SealPostingSource(changed, testCanonicalText)
	if err != nil {
		t.Fatal(err)
	}
	if changed.SourceContentSHA256 == first {
		t.Fatal("HTML-only source change did not change source-content digest")
	}
	if changed.CanonicalDescriptionSHA256 != source.CanonicalDescriptionSHA256 {
		t.Fatal("HTML-only source change altered canonical-description digest")
	}
}

func TestPostingSourceCanonicalOnlyFallbackIsTyped(t *testing.T) {
	var metadata Metadata // Legacy callers populate only canonical strings.
	result := NormalizePostingSourceResult(metadata.Source)
	if result.Availability != PostingSourceUnavailable || result.Reason != PostingSourceAbsent || result.Source != nil {
		t.Fatalf("legacy metadata source result = %+v", result)
	}
	if err := result.Validate(testAshbyPostingURL, testCanonicalText); err != nil {
		t.Fatalf("canonical-only success: %v", err)
	}
	for _, reason := range []PostingSourceUnavailableReason{
		PostingSourceAbsent, PostingSourceTooLarge, PostingSourceInvalid, PostingSourceUnsupported,
	} {
		if err := UnavailablePostingSource(reason).Validate(testAshbyPostingURL, testCanonicalText); err != nil {
			t.Errorf("unavailable reason %q: %v", reason, err)
		}
	}
	if err := UnavailablePostingSource("provider said arbitrary details").Validate(testAshbyPostingURL, testCanonicalText); !errors.Is(err, ErrPostingSourceInvalid) {
		t.Fatalf("unbounded unavailable reason accepted: %v", err)
	}
}

func TestPostingSourceRejectsUnknownVersionWrongAssociationAndBadDigests(t *testing.T) {
	source := testPostingSource(t)
	unknownVersion := source
	unknownVersion.Version++
	if err := unknownVersion.Validate(testAshbyPostingURL, testCanonicalText); !errors.Is(err, ErrPostingSourceVersion) {
		t.Fatalf("unknown version error = %v", err)
	}
	wrongAssociation := source
	wrongAssociation.Identity.Board = "elsewhere"
	if err := wrongAssociation.Validate(testAshbyPostingURL, testCanonicalText); !errors.Is(err, ErrPostingSourceAssociation) {
		t.Fatalf("wrong association error = %v", err)
	}
	badDigest := source
	badDigest.DescriptionPlain += "changed"
	if err := badDigest.Validate(testAshbyPostingURL, testCanonicalText); !errors.Is(err, ErrPostingSourceDigest) {
		t.Fatalf("source digest error = %v", err)
	}
}

func TestPostingSourceRejectsOversizedInvalidUTF8AndNUL(t *testing.T) {
	source := testPostingSource(t)
	oversized := source
	oversized.DescriptionHTML = strings.Repeat("x", MaxPostingSourceInputBytes+1)
	if _, err := SealPostingSource(oversized, testCanonicalText); !errors.Is(err, ErrPostingSourceTooLarge) {
		t.Fatalf("oversized source error = %v", err)
	}
	invalidUTF8 := source
	invalidUTF8.DescriptionPlain = string([]byte{0xff})
	if _, err := SealPostingSource(invalidUTF8, testCanonicalText); !errors.Is(err, ErrPostingSourceInvalid) {
		t.Fatalf("invalid UTF-8 error = %v", err)
	}
	withNUL := source
	withNUL.DescriptionHTML += "\x00"
	if _, err := SealPostingSource(withNUL, testCanonicalText); !errors.Is(err, ErrPostingSourceInvalid) {
		t.Fatalf("NUL error = %v", err)
	}
}

func TestPostingSourceJSONLimitAcceptsExactBoundaryAndRejectsPlusOne(t *testing.T) {
	source := testPostingSource(t)
	base, err := EncodePostingSource(source, testAshbyPostingURL, testCanonicalText)
	if err != nil {
		t.Fatal(err)
	}
	padding := MaxPostingSourceJSONBytes - len(base)
	source.DescriptionHTML += strings.Repeat("x", padding)
	source, err = SealPostingSource(source, testCanonicalText)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodePostingSource(source, testAshbyPostingURL, testCanonicalText)
	if err != nil {
		t.Fatalf("exact boundary encode: %v", err)
	}
	if len(encoded) != MaxPostingSourceJSONBytes {
		t.Fatalf("encoded size = %d, want %d", len(encoded), MaxPostingSourceJSONBytes)
	}
	if _, err := DecodePostingSource(encoded, testAshbyPostingURL, testCanonicalText); err != nil {
		t.Fatalf("exact boundary decode: %v", err)
	}

	tooLarge := source
	tooLarge.DescriptionHTML += "x"
	tooLarge, err = SealPostingSource(tooLarge, testCanonicalText)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EncodePostingSource(tooLarge, testAshbyPostingURL, testCanonicalText); !errors.Is(err, ErrPostingSourceTooLarge) {
		t.Fatalf("over-limit encode error = %v", err)
	}
}

func TestPostingSourceCodecRejectsFutureCorruptAndUnassociatedEnvelopes(t *testing.T) {
	source := testPostingSource(t)
	encoded, err := EncodePostingSource(source, testAshbyPostingURL, testCanonicalText)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	object["version"] = float64(99)
	future, _ := json.Marshal(object)
	if _, err := DecodePostingSource(future, testAshbyPostingURL, testCanonicalText); !errors.Is(err, ErrPostingSourceVersion) {
		t.Fatalf("future version error = %v", err)
	}
	if _, err := DecodePostingSource([]byte(`{"version":1`), testAshbyPostingURL, testCanonicalText); !errors.Is(err, ErrPostingSourceInvalid) {
		t.Fatalf("corrupt JSON error = %v", err)
	}
	unknownField := bytes.Replace(encoded, []byte(`"version":1`), []byte(`"version":1,"future":true`), 1)
	if _, err := DecodePostingSource(unknownField, testAshbyPostingURL, testCanonicalText); !errors.Is(err, ErrPostingSourceInvalid) {
		t.Fatalf("unknown field error = %v", err)
	}
	if _, err := DecodePostingSource(encoded, "https://jobs.ashbyhq.com/other/11111111-1111-4111-8111-111111111111", testCanonicalText); !errors.Is(err, ErrPostingSourceAssociation) {
		t.Fatalf("wrong saved posting error = %v", err)
	}
	if _, err := DecodePostingSource(bytes.Repeat([]byte{'x'}, MaxPostingSourceJSONBytes+1), testAshbyPostingURL, testCanonicalText); !errors.Is(err, ErrPostingSourceTooLarge) {
		t.Fatalf("oversized envelope error = %v", err)
	}
	if _, err := DecodePostingSource([]byte{'{', 0xff, '}'}, testAshbyPostingURL, testCanonicalText); !errors.Is(err, ErrPostingSourceInvalid) {
		t.Fatalf("invalid UTF-8 envelope error = %v", err)
	}
}

func TestPostingSourceIntegrityDecodeRetainsEmployerSourceBesideManualCorrection(t *testing.T) {
	source := testPostingSource(t)
	encoded, err := EncodePostingSource(source, testAshbyPostingURL, testCanonicalText)
	if err != nil {
		t.Fatal(err)
	}
	manualDescription := "Applicant's manually corrected description."
	if _, err := DecodePostingSource(encoded, testAshbyPostingURL, manualDescription); !errors.Is(err, ErrPostingSourceDigest) {
		t.Fatalf("strict canonical decode error = %v", err)
	}
	retained, err := DecodePostingSourceIntegrity(encoded, testAshbyPostingURL)
	if err != nil {
		t.Fatalf("integrity-only retained source decode: %v", err)
	}
	if retained.SourceContentSHA256 != source.SourceContentSHA256 || retained.CanonicalDescriptionSHA256 != source.CanonicalDescriptionSHA256 {
		t.Fatalf("retained source identity/digests changed: %+v", retained)
	}
}
