//go:build qualification

package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/storage"
)

// TestNativeReactFixtureKan16OptIn covers the opt-in KAN-16 closure seed:
// four initially active fixtures (open/closable/unknown/applied) with saved
// résumé, answer, and letter material plus a recorded Applied submission.
// The deterministic qualification enrichment seam (present in the desktop
// qualification build, not a unit-test mock) drives the real Go availability
// verdicts with no network; restarts never reseed or overwrite edits.
func TestNativeReactFixtureKan16OptIn(t *testing.T) {
	root, err := os.MkdirTemp(t.TempDir(), "yourmove-react-native-")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("YOURMOVE_DATA_DIR", root)
	t.Setenv("YOURMOVE_QUALIFY_KAN16", "1")
	if err := os.WriteFile(filepath.Join(root, ".yourmove-react-fixture"), []byte("yourmove-react-fixture-v1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "synthetic-resume.docx"), sampleDOCXForPreview(t, "Synthetic Applicant"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "synthetic-letter-source.docx"), sampleDOCXForPreview(t, "Synthetic letter source"), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := storage.OpenDB(filepath.Join(root, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	docs := filepath.Join(root, "resumes")
	if err := os.Mkdir(docs, 0700); err != nil {
		t.Fatal(err)
	}
	a := newTestApp(t, db, docs)
	if err := ConfigureNativeReactFixture(a, root, "http://127.0.0.1:1234/synthetic"); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := db.QueryRow("SELECT count(*) FROM jobs").Scan(&count); err != nil || count != 4 {
		t.Fatalf("kan16 jobs: %d %v", count, err)
	}
	jobByURL := func(raw string) Job {
		t.Helper()
		var id int64
		if err := db.QueryRow("SELECT id FROM jobs WHERE url=?", raw).Scan(&id); err != nil {
			t.Fatal(err)
		}
		job, err := a.jobByID(id)
		if err != nil {
			t.Fatal(err)
		}
		return job
	}
	openJob := jobByURL(kan16QualOpenPosting)
	closedJob := jobByURL(kan16QualClosedPosting)
	unknownJob := jobByURL(kan16QualUnknownPosting)
	appliedJob := jobByURL(kan16QualAppliedPosting)
	for _, job := range []Job{openJob, closedJob, unknownJob} {
		if job.Stage == StageClosed || job.Stage == StageApplied || job.SubmissionCount != 0 {
			t.Fatalf("fixture not initially active: %+v", job)
		}
	}
	if appliedJob.Stage != StageApplied || appliedJob.SubmissionCount != 1 {
		t.Fatalf("applied fixture not recorded: %+v", appliedJob)
	}
	if openJob.SelectedResumeID == 0 || closedJob.SelectedResumeID == 0 || appliedJob.SelectedResumeID == 0 {
		t.Fatalf("resume not assigned: open=%d closed=%d applied=%d", openJob.SelectedResumeID, closedJob.SelectedResumeID, appliedJob.SelectedResumeID)
	}
	var answers int
	if err := db.QueryRow("SELECT count(*) FROM application_answers WHERE job_id=?", openJob.ID).Scan(&answers); err != nil || answers != 1 {
		t.Fatalf("open saved answers: %d %v", answers, err)
	}
	var letters int
	if err := db.QueryRow("SELECT count(*) FROM cover_letters WHERE job_id=?", openJob.ID).Scan(&letters); err != nil || letters != 1 {
		t.Fatalf("open saved letter: %d %v", letters, err)
	}

	// Deterministic verdicts through the real Go checker with no network.
	ctx := context.Background()
	if state, _ := a.checkPostingAvailability(ctx, openJob); state != postingAvailabilityOpen {
		t.Fatalf("open fixture: %q", state)
	}
	if state, _ := a.checkPostingAvailability(ctx, closedJob); state != postingAvailabilityClosed {
		t.Fatalf("closed fixture: %q", state)
	}
	if state, detail := a.checkPostingAvailability(ctx, unknownJob); state != postingAvailabilityUnknown || strings.TrimSpace(detail) == "" {
		t.Fatalf("unknown fixture: %q %q", state, detail)
	}
	if state, _ := a.checkPostingAvailability(ctx, appliedJob); state != postingAvailabilityUnknown {
		t.Fatalf("applied fixture must stay immune: %q", state)
	}

	// The availability endpoint persists the closed retirement (prep
	// preserved) while the Applied submission stays recorded history.
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/"+strconv.FormatInt(closedJob.ID, 10)+"/availability", nil))
	var availability struct {
		State  string `json:"state"`
		Notice string `json:"notice"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &availability); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || availability.State != "closed" || availability.Notice != postingClosedNotice {
		t.Fatalf("closed availability: %d %s", rec.Code, rec.Body.String())
	}
	if job, err := a.jobByID(closedJob.ID); err != nil || job.Stage != StageClosed || job.Attention() != "history" {
		t.Fatalf("closed fixture not retired: %+v %v", job, err)
	}
	if _, block := a.ensurePostingOpen(ctx, appliedJob.ID); block == "" {
		t.Fatal("applied fixture lost its transfer guard")
	}
	if job, err := a.jobByID(appliedJob.ID); err != nil || job.Stage != StageApplied || job.SubmissionCount != 1 {
		t.Fatalf("closure probe overwrote Applied: %+v %v", job, err)
	}

	// Restart: same workspace rebinds without reseeding or overwriting.
	if err := ConfigureNativeReactFixture(a, root, "http://127.0.0.1:4321/synthetic"); err != nil {
		t.Fatalf("rebind same workspace: %v", err)
	}
	if err := db.QueryRow("SELECT count(*) FROM jobs").Scan(&count); err != nil || count != 4 {
		t.Fatalf("restart changed fixtures: %d %v", count, err)
	}
	if _, err := a.jobByID(closedJob.ID); err != nil {
		t.Fatal(err)
	}
}
