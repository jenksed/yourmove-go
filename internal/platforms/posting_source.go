package platforms

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	PostingSourceFormatVersion = 1
	MaxPostingSourceInputBytes = 1 << 20
	MaxPostingSourceJSONBytes  = 1 << 20
)

var (
	ErrPostingSourceTooLarge    = errors.New("posting source exceeds size limit")
	ErrPostingSourceInvalid     = errors.New("posting source is invalid")
	ErrPostingSourceVersion     = errors.New("unsupported posting source version")
	ErrPostingSourceAssociation = errors.New("posting source does not match saved posting")
	ErrPostingSourceDigest      = errors.New("posting source digest mismatch")
)

// PostingIdentity is the validated provider, board/site, optional provider
// region, and posting key. It is association evidence only; it does not
// authorize an application action.
type PostingIdentity struct {
	Provider  string `json:"provider"`
	Board     string `json:"board"`
	Region    string `json:"region,omitempty"`
	PostingID string `json:"posting_id"`
}

// PostingSource contains only employer-owned posting content and direct facts.
// Retrieval times, applicant data, and application destinations belong elsewhere.
type PostingSource struct {
	Version                    int             `json:"version"`
	Identity                   PostingIdentity `json:"identity"`
	DescriptionHTML            string          `json:"description_html"`
	DescriptionPlain           string          `json:"description_plain"`
	CanonicalDescriptionSHA256 string          `json:"canonical_description_sha256"`
	Location                   string          `json:"location,omitempty"`
	WorkplaceType              string          `json:"workplace_type,omitempty"`
	EmploymentType             string          `json:"employment_type,omitempty"`
	SalarySummary              string          `json:"salary_summary,omitempty"`
	SourceContentSHA256        string          `json:"source_content_sha256"`
}

type PostingSourceAvailability string

const (
	PostingSourceAvailable   PostingSourceAvailability = "available"
	PostingSourceUnavailable PostingSourceAvailability = "unavailable"
)

type PostingSourceUnavailableReason string

const (
	PostingSourceAbsent      PostingSourceUnavailableReason = "absent"
	PostingSourceTooLarge    PostingSourceUnavailableReason = "too_large"
	PostingSourceInvalid     PostingSourceUnavailableReason = "invalid"
	PostingSourceUnsupported PostingSourceUnavailableReason = "unsupported"
)

// PostingSourceResult describes rich-source availability after a successful
// retrieval. A failed retrieval is represented by FetchJob's error return.
type PostingSourceResult struct {
	Availability PostingSourceAvailability
	Reason       PostingSourceUnavailableReason
	Source       *PostingSource
}

// UnavailablePostingSource records a successful retrieval without usable
// selected source content. Reasons are bounded so persistence never stores an
// arbitrary provider error as lifecycle state.
func UnavailablePostingSource(reason PostingSourceUnavailableReason) PostingSourceResult {
	return PostingSourceResult{Availability: PostingSourceUnavailable, Reason: reason}
}

// AvailablePostingSource wraps a validated source candidate. Call Validate
// with the saved URL and canonical description before persisting or rendering.
func AvailablePostingSource(source PostingSource) PostingSourceResult {
	return PostingSourceResult{Availability: PostingSourceAvailable, Source: &source}
}

// NormalizePostingSourceResult makes legacy metadata's zero value explicitly
// mean successful canonical-only retrieval with no rich source supplied.
func NormalizePostingSourceResult(result PostingSourceResult) PostingSourceResult {
	if result.Availability == "" && result.Reason == "" && result.Source == nil {
		return UnavailablePostingSource(PostingSourceAbsent)
	}
	return result
}

// AshbyPostingIdentity validates a saved posting URL and returns its exact
// board/posting association. Query and fragment data do not identify a posting.
func AshbyPostingIdentity(rawURL string) (PostingIdentity, error) {
	board, postingID, err := parseAshbyURL(rawURL)
	if err != nil {
		return PostingIdentity{}, fmt.Errorf("Ashby posting identity: %w", err)
	}
	return PostingIdentity{Provider: "ashby", Board: board, PostingID: postingID}, nil
}

// ValidateURL checks that a supported provider URL identifies this exact
// board/site, region where applicable, and posting. Returned application URLs
// are deliberately not accepted here.
func (identity PostingIdentity) ValidateURL(rawURL string) error {
	var got PostingIdentity
	switch identity.Provider {
	case "ashby":
		parsed, err := AshbyPostingIdentity(rawURL)
		if err != nil {
			return err
		}
		got = parsed
	case "personio", "teamtailor", "recruitee", "icims":
		var err error
		switch identity.Provider {
		case "icims":
			got, err = ICIMSPostingIdentity(rawURL)
		case "personio":
			got, err = PersonioPostingIdentity(rawURL)
		case "teamtailor":
			got, err = TeamtailorPostingIdentity(rawURL)
		case "recruitee":
			got, err = RecruiteePostingIdentity(rawURL)
		}
		if err != nil {
			return err
		}
	case "workday":
		parsed, err := WorkdayPostingIdentity(rawURL)
		if err != nil {
			return fmt.Errorf("Workday posting identity: %w", err)
		}
		got = parsed
	case "smartrecruiters":
		company, postingID, err := parseSmartRecruitersURL(rawURL)
		if err != nil {
			return fmt.Errorf("SmartRecruiters posting identity: %w", err)
		}
		got = PostingIdentity{Provider: "smartrecruiters", Board: company, PostingID: postingID}
	case "workable":
		account, shortcode, err := parseWorkableURL(rawURL)
		if err != nil {
			return fmt.Errorf("Workable posting identity: %w", err)
		}
		got = PostingIdentity{Provider: "workable", Board: account, PostingID: shortcode}
	case "jobvite":
		site, postingID, err := parseJobviteURL(rawURL)
		if err != nil {
			return fmt.Errorf("Jobvite posting identity: %w", err)
		}
		got = PostingIdentity{Provider: "jobvite", Board: site, PostingID: postingID}
	case "greenhouse":
		board, postingID, err := parseGreenhouseURL(rawURL)
		if err != nil {
			return fmt.Errorf("Greenhouse posting identity: %w", err)
		}
		got = PostingIdentity{Provider: "greenhouse", Board: board, PostingID: postingID}
	case "lever":
		site, postingID, apiHost, err := parseLeverURL(rawURL)
		if err != nil {
			return fmt.Errorf("Lever posting identity: %w", err)
		}
		region := "global"
		if apiHost == "api.eu.lever.co" {
			region = "eu"
		}
		got = PostingIdentity{Provider: "lever", Board: site, Region: region, PostingID: postingID}
	default:
		return fmt.Errorf("%w: unsupported identity provider %q", ErrPostingSourceInvalid, identity.Provider)
	}
	if identity.Provider != got.Provider || identity.Board != got.Board || identity.Region != got.Region || identity.PostingID != got.PostingID {
		return fmt.Errorf("%w: got %q/%q/%q, want %q/%q/%q", ErrPostingSourceAssociation,
			got.Provider, got.Board, got.Region+"/"+got.PostingID,
			identity.Provider, identity.Board, identity.Region+"/"+identity.PostingID)
	}
	return nil
}

// CanonicalDescriptionDigest hashes the exact stored description bytes. It is
// independent of the rich-source digest and parser output.
func CanonicalDescriptionDigest(description string) string {
	digest := sha256.Sum256([]byte(description))
	return hex.EncodeToString(digest[:])
}

type postingSourceDigestProjection struct {
	Version                    int             `json:"version"`
	Identity                   PostingIdentity `json:"identity"`
	DescriptionHTML            string          `json:"description_html"`
	DescriptionPlain           string          `json:"description_plain"`
	CanonicalDescriptionSHA256 string          `json:"canonical_description_sha256"`
	Location                   string          `json:"location"`
	WorkplaceType              string          `json:"workplace_type"`
	EmploymentType             string          `json:"employment_type"`
	SalarySummary              string          `json:"salary_summary"`
}

// SealPostingSource binds a selected source to the exact canonical description
// and computes its deterministic content digest. The projection contains no
// fetch/check time, application URL, or applicant-owned value.
func SealPostingSource(source PostingSource, canonicalDescription string) (PostingSource, error) {
	if source.Version == 0 {
		source.Version = PostingSourceFormatVersion
	}
	if err := source.validateFields(); err != nil {
		return PostingSource{}, err
	}
	source.CanonicalDescriptionSHA256 = CanonicalDescriptionDigest(canonicalDescription)
	digest, err := source.contentDigest()
	if err != nil {
		return PostingSource{}, err
	}
	source.SourceContentSHA256 = digest
	return source, nil
}

// ContentDigest returns the stable digest of selected source content and facts.
// It deliberately excludes SourceContentSHA256 itself and all retrieval times.
func (source PostingSource) ContentDigest() (string, error) {
	if err := source.validateFields(); err != nil {
		return "", err
	}
	return source.contentDigest()
}

func (source PostingSource) contentDigest() (string, error) {
	projection := postingSourceDigestProjection{
		Version:                    source.Version,
		Identity:                   source.Identity,
		DescriptionHTML:            source.DescriptionHTML,
		DescriptionPlain:           source.DescriptionPlain,
		CanonicalDescriptionSHA256: source.CanonicalDescriptionSHA256,
		Location:                   source.Location,
		WorkplaceType:              source.WorkplaceType,
		EmploymentType:             source.EmploymentType,
		SalarySummary:              source.SalarySummary,
	}
	encoded, err := json.Marshal(projection)
	if err != nil {
		return "", fmt.Errorf("encode posting source digest projection: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

// Validate checks the source version, bounds, UTF-8/NUL safety, identity,
// canonical description association, and both digests before use.
func (source PostingSource) Validate(savedPostingURL, canonicalDescription string) error {
	if err := source.ValidateIntegrity(savedPostingURL); err != nil {
		return err
	}
	if source.CanonicalDescriptionSHA256 != CanonicalDescriptionDigest(canonicalDescription) {
		return fmt.Errorf("%w: canonical description", ErrPostingSourceDigest)
	}
	return nil
}

// ValidateIntegrity checks the source envelope's own version, bounds, UTF-8/
// NUL safety, identity, and content digest. It deliberately does not compare
// the canonical digest to the current job description: app uses this when a
// manual correction should remain primary while the employer source remains
// available as secondary evidence. The original canonical association must
// still be checked with Validate before a successful retrieval is persisted.
func (source PostingSource) ValidateIntegrity(savedPostingURL string) error {
	if err := source.validateFields(); err != nil {
		return err
	}
	if err := source.Identity.ValidateURL(savedPostingURL); err != nil {
		return err
	}
	if !validSHA256Digest(source.CanonicalDescriptionSHA256) || !validSHA256Digest(source.SourceContentSHA256) {
		return fmt.Errorf("%w: malformed SHA-256 digest", ErrPostingSourceInvalid)
	}
	got, err := source.contentDigest()
	if err != nil {
		return err
	}
	if source.SourceContentSHA256 != got {
		return fmt.Errorf("%w: source content", ErrPostingSourceDigest)
	}
	encoded, err := json.Marshal(source)
	if err != nil {
		return fmt.Errorf("encode posting source: %w", err)
	}
	if len(encoded) > MaxPostingSourceJSONBytes {
		return ErrPostingSourceTooLarge
	}
	return nil
}

func validSHA256Digest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

func (source PostingSource) validateFields() error {
	if source.Version != PostingSourceFormatVersion {
		return fmt.Errorf("%w: %d", ErrPostingSourceVersion, source.Version)
	}
	textFields := []struct {
		name  string
		value string
	}{
		{"provider", source.Identity.Provider},
		{"board", source.Identity.Board},
		{"region", source.Identity.Region},
		{"posting_id", source.Identity.PostingID},
		{"description_html", source.DescriptionHTML},
		{"description_plain", source.DescriptionPlain},
		{"canonical_description_sha256", source.CanonicalDescriptionSHA256},
		{"location", source.Location},
		{"workplace_type", source.WorkplaceType},
		{"employment_type", source.EmploymentType},
		{"salary_summary", source.SalarySummary},
		{"source_content_sha256", source.SourceContentSHA256},
	}
	total := 0
	for _, field := range textFields {
		if !utf8.ValidString(field.value) {
			return fmt.Errorf("%w: %s is not valid UTF-8", ErrPostingSourceInvalid, field.name)
		}
		if strings.IndexByte(field.value, 0) >= 0 {
			return fmt.Errorf("%w: %s contains NUL", ErrPostingSourceInvalid, field.name)
		}
		total += len(field.value)
		if total > MaxPostingSourceInputBytes {
			return ErrPostingSourceTooLarge
		}
	}
	if source.Identity.Board == "" || source.Identity.PostingID == "" {
		return fmt.Errorf("%w: missing or unsupported identity", ErrPostingSourceInvalid)
	}
	switch source.Identity.Provider {
	case "ashby", "greenhouse", "smartrecruiters", "workable", "jobvite", "teamtailor", "recruitee", "icims":
		if source.Identity.Region != "" {
			return fmt.Errorf("%w: unsupported %s region", ErrPostingSourceInvalid, source.Identity.Provider)
		}
	case "personio":
		if source.Identity.Region != "" && !strings.Contains("|de|en|fr|es|nl|it|pt|", "|"+source.Identity.Region+"|") {
			return ErrPostingSourceInvalid
		}
	case "workday":
		if source.Identity.Region != "" && !safeToken(source.Identity.Region) {
			return fmt.Errorf("%w: unsupported Workday locale", ErrPostingSourceInvalid)
		}
	case "lever":
		if source.Identity.Region != "global" && source.Identity.Region != "eu" {
			return fmt.Errorf("%w: missing or unsupported Lever region", ErrPostingSourceInvalid)
		}
	default:
		return fmt.Errorf("%w: missing or unsupported identity provider", ErrPostingSourceInvalid)
	}
	if len(source.DescriptionHTML)+len(source.DescriptionPlain) > MaxPostingSourceInputBytes {
		return ErrPostingSourceTooLarge
	}
	return nil
}

// Validate checks an availability result without conflating it with request
// success or failure. Saved manual values and confirmation state live in app.
func (result PostingSourceResult) Validate(savedPostingURL, canonicalDescription string) error {
	result = NormalizePostingSourceResult(result)
	switch result.Availability {
	case PostingSourceAvailable:
		if result.Reason != "" || result.Source == nil {
			return fmt.Errorf("%w: available source requires content and no unavailable reason", ErrPostingSourceInvalid)
		}
		return result.Source.Validate(savedPostingURL, canonicalDescription)
	case PostingSourceUnavailable:
		if result.Source != nil || !validUnavailableReason(result.Reason) {
			return fmt.Errorf("%w: invalid unavailable source result", ErrPostingSourceInvalid)
		}
		return nil
	default:
		return fmt.Errorf("%w: unknown availability %q", ErrPostingSourceInvalid, result.Availability)
	}
}

func validUnavailableReason(reason PostingSourceUnavailableReason) bool {
	switch reason {
	case PostingSourceAbsent, PostingSourceTooLarge, PostingSourceInvalid, PostingSourceUnsupported:
		return true
	default:
		return false
	}
}

// EncodePostingSource writes the bounded versioned envelope after validating
// its saved-posting and canonical-description association.
func EncodePostingSource(source PostingSource, savedPostingURL, canonicalDescription string) ([]byte, error) {
	if err := source.Validate(savedPostingURL, canonicalDescription); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(source)
	if err != nil {
		return nil, fmt.Errorf("encode posting source: %w", err)
	}
	if len(encoded) > MaxPostingSourceJSONBytes {
		return nil, ErrPostingSourceTooLarge
	}
	return encoded, nil
}

// DecodePostingSource rejects oversized, invalid, unknown-field, and future
// version envelopes and requires the exact current canonical-description
// association. Use DecodePostingSourceIntegrity only for a retained employer
// source shown secondarily beside an applicant-owned manual correction.
func DecodePostingSource(data []byte, savedPostingURL, canonicalDescription string) (PostingSource, error) {
	source, err := decodePostingSourceEnvelope(data)
	if err != nil {
		return PostingSource{}, err
	}
	if err := source.Validate(savedPostingURL, canonicalDescription); err != nil {
		return PostingSource{}, err
	}
	return source, nil
}

// DecodePostingSourceIntegrity validates the envelope, exact saved posting,
// version, and source-content digest without requiring the saved canonical
// description to equal the employer's older projection. Only use this after
// a manual-description precedence check; it never upgrades freshness or
// confirmation for the manually edited value.
func DecodePostingSourceIntegrity(data []byte, savedPostingURL string) (PostingSource, error) {
	source, err := decodePostingSourceEnvelope(data)
	if err != nil {
		return PostingSource{}, err
	}
	if err := source.ValidateIntegrity(savedPostingURL); err != nil {
		return PostingSource{}, err
	}
	return source, nil
}

func decodePostingSourceEnvelope(data []byte) (PostingSource, error) {
	if len(data) > MaxPostingSourceJSONBytes {
		return PostingSource{}, ErrPostingSourceTooLarge
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return PostingSource{}, fmt.Errorf("%w: envelope is not valid UTF-8 text", ErrPostingSourceInvalid)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var source PostingSource
	if err := decoder.Decode(&source); err != nil {
		return PostingSource{}, fmt.Errorf("%w: decode envelope: %v", ErrPostingSourceInvalid, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return PostingSource{}, fmt.Errorf("%w: trailing JSON value", ErrPostingSourceInvalid)
		}
		return PostingSource{}, fmt.Errorf("%w: trailing envelope data: %v", ErrPostingSourceInvalid, err)
	}
	return source, nil
}
