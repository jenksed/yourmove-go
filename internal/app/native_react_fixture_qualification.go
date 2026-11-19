//go:build qualification

package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/schema"
	"github.com/jenksed/yourmove-go/internal/platforms"
)

const NativeReactFixturePosting = "https://jobs.ashbyhq.com/yourmove-native-fixture/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

type nativeReactFixtureSession struct {
	*chromium.Session
	fixture string
	// expected is the synthetic posting the observation URL is rewritten
	// to. The single-job fixture always expects NativeReactFixturePosting;
	// KAN-16 fixtures expect their own seeded posting.
	expected string
}

func (s *nativeReactFixtureSession) Observe(ctx context.Context) (browser.Observation, error) {
	o, err := s.Session.Observe(ctx)
	if err != nil {
		return o, err
	}
	if o.URL != s.fixture && !strings.HasPrefix(o.URL, s.fixture+"/") {
		return browser.Observation{}, errors.New("fixture observation outside the bound route")
	}
	o.URL = s.expected + strings.TrimPrefix(o.URL, s.fixture)
	return o, nil
}

// KAN-16 deterministic closure-qualification fixtures. All are initially
// active synthetic Greenhouse postings; no live employer traffic is ever
// issued because the qualification enrichment seam below answers from a
// fixed map. The closed fixture retires on its first availability check,
// the unknown fixture stays active with an inconclusive lookup, and the
// applied fixture stays immune as recorded history.
const (
	kan16QualOpenPosting    = "https://job-boards.greenhouse.io/kan16qual-open/jobs/11111111"
	kan16QualClosedPosting  = "https://job-boards.greenhouse.io/kan16qual-closed/jobs/22222222"
	kan16QualUnknownPosting = "https://job-boards.greenhouse.io/kan16qual-unknown/jobs/33333333"
	kan16QualAppliedPosting = "https://job-boards.greenhouse.io/kan16qual-applied/jobs/44444444"
)

func kan16QualFixturePosting(raw string) bool {
	switch raw {
	case kan16QualOpenPosting, kan16QualClosedPosting, kan16QualUnknownPosting, kan16QualAppliedPosting:
		return true
	}
	return false
}

// ConfigureNativeReactFixture binds a single known synthetic job to a single
// loopback route. This code is absent from production builds. No hostname or
// employer scope checks are altered for normal application sessions.
func ConfigureNativeReactFixture(a *App, root, fixture string) error {
	if a == nil || !filepath.IsAbs(root) || filepath.Clean(root) != root || !strings.HasPrefix(filepath.Base(root), "yourmove-react-native-") || os.Getenv("YOURMOVE_DATA_DIR") != root {
		return errors.New("qualification requires its dedicated synthetic workspace")
	}
	marker, err := os.ReadFile(filepath.Join(root, ".yourmove-react-fixture"))
	if err != nil || string(marker) != "yourmove-react-fixture-v1\n" {
		return errors.New("qualification marker missing")
	}
	u, err := url.Parse(fixture)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.Path != "/synthetic" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("qualification requires an exact loopback route")
	}
	var seq int
	var name, dbPath string
	if err := a.db.QueryRow("PRAGMA database_list").Scan(&seq, &name, &dbPath); err != nil || name != "main" {
		return errors.New("cannot establish fixture database")
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	for _, path := range []string{dbPath, a.resumeDir} {
		canonical, err := filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(canonicalRoot, canonical)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return errors.New("fixture data escapes workspace")
		}
	}
	var count int
	if err := a.db.QueryRow("SELECT count(*) FROM jobs").Scan(&count); err != nil {
		return err
	}
	// KAN-12 native qualification (saved answers plus Answer Library
	// examples) is opt-in via YOURMOVE_QUALIFY_KAN12=1 and seeds only a
	// freshly created workspace. A restart must never silently repopulate
	// answers the applicant deleted, so an existing workspace is left alone.
	kan12Fixture := strings.EqualFold(strings.TrimSpace(os.Getenv("YOURMOVE_QUALIFY_KAN12")), "1")
	// KAN-16 closure qualification is opt-in via YOURMOVE_QUALIFY_KAN16=1
	// and seeds four initially active synthetic postings (open, closable,
	// unknown, applied) with saved résumé, answer, and letter material. The
	// two flags address different workspaces and must not be combined.
	kan16Fixture := strings.EqualFold(strings.TrimSpace(os.Getenv("YOURMOVE_QUALIFY_KAN16")), "1")
	if kan16Fixture && kan12Fixture {
		return errors.New("KAN-12 and KAN-16 qualification workspaces are separate")
	}
	freshWorkspace := count == 0
	if kan16Fixture {
		if err := configureNativeKan16Fixture(a, root, count, freshWorkspace); err != nil {
			return err
		}
	} else if count == 0 {
		const description = "A synthetic opportunity for native qualification."
		if err := a.saveJob(Job{URL: NativeReactFixturePosting, Host: "jobs.ashbyhq.com", Title: "Synthetic Product Engineer", Company: "Fixture Employer", Description: description}); err != nil {
			return err
		}
		// The letter references a real managed synthetic document. It is seeded
		// only with a new job; a restart must never overwrite applicant edits.
		data, err := os.ReadFile(filepath.Join(root, "synthetic-letter-source.docx"))
		if err != nil {
			return err
		}
		source, err := a.importResume("synthetic-letter-source.docx", bytes.NewReader(data))
		if err != nil {
			return err
		}
		var jobID int64
		if err := a.db.QueryRow("SELECT id FROM jobs WHERE url=?", NativeReactFixturePosting).Scan(&jobID); err != nil {
			return err
		}
		digest := sha256.Sum256([]byte(description))
		_, err = a.db.Exec(`INSERT INTO cover_letters(job_id,body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
			jobID, "Dear hiring team,\n\nThis is a synthetic cover letter for native qualification. I enjoy working with thoughtful teams.\n\nSynthetic Applicant", "fixture", "none", source.ID, source.SHA256, source.ID, source.SHA256, fmt.Sprintf("%x", digest), time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			return err
		}
	} else {
		var matching int
		if err := a.db.QueryRow("SELECT count(*) FROM jobs WHERE url=?", NativeReactFixturePosting).Scan(&matching); err != nil || count != 1 || matching != 1 {
			return errors.New("unexpected qualification workspace")
		}
	}
	if kan12Fixture && freshWorkspace {
		if err := seedNativeKan12(a); err != nil {
			return err
		}
	}
	if kan12Fixture {
		a.previewApplication = func(context.Context, string) (schema.Result, error) {
			return schema.Result{}, errors.New("synthetic preview disabled")
		}
	}
	a.browsers.factory = func(ctx context.Context, posting string) (readOnlySession, error) {
		expected := ""
		switch {
		case posting == NativeReactFixturePosting:
			expected = NativeReactFixturePosting
		case kan16Fixture && kan16QualFixturePosting(posting):
			expected = posting
		default:
			return nil, errors.New("fixture posting not authorized")
		}
		s, err := chromium.LaunchVisibleFixture(ctx, fixture)
		if err != nil {
			return nil, err
		}
		return &nativeReactFixtureSession{Session: s, fixture: fixture, expected: expected}, nil
	}
	a.inspectApplication = func(context.Context, string) (schema.Result, error) {
		return schema.Result{}, errors.New("public inspection disabled in qualification")
	}
	if kan16Fixture {
		// Deterministic provider responses for the four KAN-16 fixtures.
		// No network is issued: open retrieves, closed is provider-confirmed,
		// unknown is an inconclusive lookup, and the applied posting reports
		// a late closure that Applied immunity must preserve as history.
		kan16Responses := map[string][2]string{
			kan16QualOpenPosting:    {"retrieved", "Synthetic open posting."},
			kan16QualClosedPosting:  {"closed", "Synthetic provider-confirmed closure."},
			kan16QualUnknownPosting: {"failed", "Synthetic inconclusive lookup."},
			kan16QualAppliedPosting: {"closed", "Synthetic late closure."},
		}
		a.enrichmentFetch = func(_ context.Context, raw string) (platforms.Metadata, string, string) {
			if outcome, ok := kan16Responses[raw]; ok {
				if outcome[0] == "retrieved" {
					return platforms.Metadata{Title: "Synthetic Open Role", Company: "Fixture Employer", Description: "Synthetic description."}, outcome[0], outcome[1]
				}
				return platforms.Metadata{}, outcome[0], outcome[1]
			}
			return platforms.Metadata{}, "unavailable", "Posting refresh is disabled in qualification."
		}
	} else {
		a.enrichmentFetch = func(context.Context, string) (platforms.Metadata, string, string) {
			return platforms.Metadata{}, "unavailable", "Posting refresh is disabled in qualification."
		}
	}
	if !browser.OwnsPostingURL(NativeReactFixturePosting, NativeReactFixturePosting) {
		return fmt.Errorf("synthetic posting identity rejected")
	}
	return nil
}

// configureNativeKan16Fixture seeds (fresh workspace only) or validates the
// four initially active KAN-16 closure-qualification postings with saved
// résumé, answer, and letter material plus a recorded Applied submission. A
// restart never reseeds and never overwrites applicant edits; it only
// verifies the workspace still holds exactly the KAN-16 set. No employer is
// contacted and no submission is recorded beyond the pre-seeded Applied row.
func configureNativeKan16Fixture(a *App, root string, count int, fresh bool) error {
	postings := []struct {
		url   string
		title string
	}{
		{kan16QualOpenPosting, "Synthetic Open Role (KAN-16)"},
		{kan16QualClosedPosting, "Synthetic Closable Role (KAN-16)"},
		{kan16QualUnknownPosting, "Synthetic Unknown Role (KAN-16)"},
		{kan16QualAppliedPosting, "Synthetic Applied Role (KAN-16)"},
	}
	if !fresh {
		if count != len(postings) {
			return errors.New("unexpected qualification workspace")
		}
		for _, p := range postings {
			var matching int
			if err := a.db.QueryRow("SELECT count(*) FROM jobs WHERE url=?", p.url).Scan(&matching); err != nil || matching != 1 {
				return errors.New("unexpected qualification workspace")
			}
		}
		return nil
	}
	if count != 0 {
		return errors.New("unexpected qualification workspace")
	}
	const description = "Applicant-saved synthetic description."
	for _, p := range postings {
		if err := a.saveJob(Job{
			URL: p.url, Host: "job-boards.greenhouse.io",
			Title: p.title, TitleManual: true,
			Company: "Fixture Employer", CompanyManual: true,
			Description: description, DescriptionManual: true,
			SourceStatus: "not_checked",
		}); err != nil {
			return err
		}
	}
	jobID := func(raw string) (int64, error) {
		var id int64
		if err := a.db.QueryRow("SELECT id FROM jobs WHERE url=?", raw).Scan(&id); err != nil {
			return 0, err
		}
		return id, nil
	}
	openID, err := jobID(kan16QualOpenPosting)
	if err != nil {
		return err
	}
	closedID, err := jobID(kan16QualClosedPosting)
	if err != nil {
		return err
	}
	appliedID, err := jobID(kan16QualAppliedPosting)
	if err != nil {
		return err
	}
	if err := a.saveCandidateProfile(CandidateProfile{
		FirstName: "Alex", LastName: "Fixture", Email: "alex@example.test", Approved: true,
	}); err != nil {
		return err
	}
	resumeData, err := os.ReadFile(filepath.Join(root, "synthetic-resume.docx"))
	if err != nil {
		return err
	}
	resume, err := a.importResume("synthetic-resume.docx", bytes.NewReader(resumeData))
	if err != nil {
		return err
	}
	for _, id := range []int64{openID, closedID, appliedID} {
		if err := a.assignResume(id, resume.ID); err != nil {
			return err
		}
	}
	// One saved answer plus a selected cover letter on the open fixture so
	// the continuation scope is non-empty during qualification.
	field := schema.Field{ID: "browser:motivation", Label: "Why do you want to join this team?", Kind: "textarea"}
	if _, err := a.saveInspection(openID, schema.Result{
		Platform: "greenhouse", Source: "browser", Completeness: "rendered_partial",
		ApplicationURL: kan16QualOpenPosting, Fields: []schema.Field{field},
	}); err != nil {
		return err
	}
	if _, err := a.db.Exec(`INSERT INTO application_answers(job_id,destination,provider,question_key,label,answer,revision) VALUES(?,?, 'greenhouse',?,?,?,1)`,
		openID, kan16QualOpenPosting, questionKey(kan16QualOpenPosting, "greenhouse", field), field.Label,
		"I want to join this team because the on-call work matches my reliability experience."); err != nil {
		return err
	}
	descDigest := sha256.Sum256([]byte(description))
	if _, err := a.db.Exec(`INSERT INTO cover_letters(job_id,body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		openID, "Dear hiring team,\n\nThis is a synthetic cover letter for KAN-16 closure qualification only.\n\nSynthetic Applicant",
		"fixture", "none", resume.ID, resume.SHA256, resume.ID, resume.SHA256,
		fmt.Sprintf("%x", descDigest), time.Now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	// The applied fixture records the applicant's manual submission locally
	// before any closure probe, so availability and transfer guards must
	// preserve it as history. Nothing is sent to an employer.
	token, err := newSubmissionToken()
	if err != nil {
		return err
	}
	if err := a.recordSubmissionReview(appliedID, token, resume.ID, resume.SHA256); err != nil {
		return err
	}
	if err := a.recordSubmission(appliedID, "Submitted personally on the employer site (synthetic).", token); err != nil {
		return err
	}
	for _, raw := range []string{kan16QualOpenPosting, kan16QualClosedPosting, kan16QualUnknownPosting, kan16QualAppliedPosting} {
		if !browser.OwnsPostingURL(raw, raw) {
			return fmt.Errorf("synthetic posting identity rejected: %s", raw)
		}
	}
	return nil
}
