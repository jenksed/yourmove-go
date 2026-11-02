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
}

func (s *nativeReactFixtureSession) Observe(ctx context.Context) (browser.Observation, error) {
	o, err := s.Session.Observe(ctx)
	if err != nil {
		return o, err
	}
	if o.URL != s.fixture && !strings.HasPrefix(o.URL, s.fixture+"/") {
		return browser.Observation{}, errors.New("fixture observation outside the bound route")
	}
	o.URL = NativeReactFixturePosting + strings.TrimPrefix(o.URL, s.fixture)
	return o, nil
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
	if count == 0 {
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
	a.browsers.factory = func(ctx context.Context, posting string) (readOnlySession, error) {
		if posting != NativeReactFixturePosting {
			return nil, errors.New("fixture posting not authorized")
		}
		s, err := chromium.LaunchVisibleFixture(ctx, fixture)
		if err != nil {
			return nil, err
		}
		return &nativeReactFixtureSession{Session: s, fixture: fixture}, nil
	}
	a.inspectApplication = func(context.Context, string) (schema.Result, error) {
		return schema.Result{}, errors.New("public inspection disabled in qualification")
	}
	a.enrichmentFetch = func(context.Context, string) (platforms.Metadata, string, string) {
		return platforms.Metadata{}, "unavailable", "Posting refresh is disabled in qualification."
	}
	if !browser.OwnsPostingURL(NativeReactFixturePosting, NativeReactFixturePosting) {
		return fmt.Errorf("synthetic posting identity rejected")
	}
	return nil
}
