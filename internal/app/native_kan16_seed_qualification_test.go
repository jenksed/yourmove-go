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
	if err := db.QueryRow("SELECT count(*) FROM application_answers WHERE job_id=?", closedJob.ID).Scan(&answers); err != nil || answers != 1 {
		t.Fatalf("closed saved answers: %d %v", answers, err)
	}
	if err := db.QueryRow("SELECT count(*) FROM cover_letters WHERE job_id=?", closedJob.ID).Scan(&letters); err != nil || letters != 1 {
		t.Fatalf("closed saved letter: %d %v", letters, err)
	}
	const wantAnswer = "I want to join this team because the on-call work matches my reliability experience."
	const wantLabel = "Why do you want to join this team?"
	const wantLetter = "Dear hiring team,\n\nThis is a synthetic cover letter for KAN-16 closure qualification only.\n\nSynthetic Applicant"
	var openDest, openProvider, openKey, openLabel, openAnswer string
	var openRevision int64
	if err := db.QueryRow(`SELECT destination,provider,question_key,label,answer,revision FROM application_answers WHERE job_id=?`, openJob.ID).Scan(&openDest, &openProvider, &openKey, &openLabel, &openAnswer, &openRevision); err != nil {
		t.Fatal(err)
	}
	var closedDest, closedProvider, closedKey, closedLabel, closedAnswer string
	var closedRevision int64
	if err := db.QueryRow(`SELECT destination,provider,question_key,label,answer,revision FROM application_answers WHERE job_id=?`, closedJob.ID).Scan(&closedDest, &closedProvider, &closedKey, &closedLabel, &closedAnswer, &closedRevision); err != nil {
		t.Fatal(err)
	}
	if openDest != kan16QualOpenPosting || openProvider != "greenhouse" || openLabel != wantLabel || openAnswer != wantAnswer || openRevision != 1 {
		t.Fatalf("open answer identity: %+v", openAnswer)
	}
	if closedDest != kan16QualClosedPosting || closedProvider != "greenhouse" || closedLabel != wantLabel || closedAnswer != wantAnswer || closedRevision != 1 {
		t.Fatalf("closed answer identity: dest=%q provider=%q label=%q answer=%q rev=%d", closedDest, closedProvider, closedLabel, closedAnswer, closedRevision)
	}
	if openKey == closedKey {
		t.Fatal("open and closed answers must keep per-job question identity")
	}
	var openLetterBody, openLetterProvider, openLetterModel string
	var openMasterID, openRoleID int64
	var openMasterSHA, openRoleSHA, openJobSHA string
	if err := db.QueryRow(`SELECT body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256 FROM cover_letters WHERE job_id=?`, openJob.ID).Scan(&openLetterBody, &openLetterProvider, &openLetterModel, &openMasterID, &openMasterSHA, &openRoleID, &openRoleSHA, &openJobSHA); err != nil {
		t.Fatal(err)
	}
	var closedLetterBody, closedLetterProvider, closedLetterModel string
	var closedMasterID, closedRoleID int64
	var closedMasterSHA, closedRoleSHA, closedJobSHA string
	if err := db.QueryRow(`SELECT body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256 FROM cover_letters WHERE job_id=?`, closedJob.ID).Scan(&closedLetterBody, &closedLetterProvider, &closedLetterModel, &closedMasterID, &closedMasterSHA, &closedRoleID, &closedRoleSHA, &closedJobSHA); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{openLetterBody, closedLetterBody} {
		if body != wantLetter {
			t.Fatalf("letter body mismatch: %q", body)
		}
	}
	if openMasterID != openJob.SelectedResumeID || openRoleID != openJob.SelectedResumeID || closedMasterID != closedJob.SelectedResumeID || closedRoleID != closedJob.SelectedResumeID {
		t.Fatalf("letter resume association: open master=%d role=%d sel=%d closed master=%d role=%d sel=%d", openMasterID, openRoleID, openJob.SelectedResumeID, closedMasterID, closedRoleID, closedJob.SelectedResumeID)
	}
	var openInspectionURL, closedInspectionURL string
	if err := db.QueryRow(`SELECT application_url FROM application_inspections WHERE job_id=? ORDER BY id DESC LIMIT 1`, openJob.ID).Scan(&openInspectionURL); err != nil || openInspectionURL != kan16QualOpenPosting {
		t.Fatalf("open inspection destination: %q %v", openInspectionURL, err)
	}
	if err := db.QueryRow(`SELECT application_url FROM application_inspections WHERE job_id=? ORDER BY id DESC LIMIT 1`, closedJob.ID).Scan(&closedInspectionURL); err != nil || closedInspectionURL != kan16QualClosedPosting {
		t.Fatalf("closed inspection destination: %q %v", closedInspectionURL, err)
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
	retired, err := a.jobByID(closedJob.ID)
	if err != nil || retired.Stage != StageClosed || retired.Attention() != "history" {
		t.Fatalf("closed fixture not retired: %+v %v", retired, err)
	}
	// Retirement preserves exact saved writing and associations.
	if retired.SelectedResumeID != closedJob.SelectedResumeID {
		t.Fatalf("closed resume changed on retirement: %d vs %d", retired.SelectedResumeID, closedJob.SelectedResumeID)
	}
	var afterDest, afterProvider, afterKey, afterLabel, afterAnswer string
	var afterRevision int64
	if err := db.QueryRow(`SELECT destination,provider,question_key,label,answer,revision FROM application_answers WHERE job_id=?`, closedJob.ID).Scan(&afterDest, &afterProvider, &afterKey, &afterLabel, &afterAnswer, &afterRevision); err != nil {
		t.Fatal(err)
	}
	if afterDest != closedDest || afterProvider != closedProvider || afterKey != closedKey || afterLabel != closedLabel || afterAnswer != closedAnswer || afterRevision != closedRevision {
		t.Fatalf("closed answer changed on retirement: %+v", afterAnswer)
	}
	var afterBody, afterLetterProvider, afterModel string
	var afterMasterID, afterRoleID int64
	var afterMasterSHA, afterRoleSHA, afterJobSHA string
	if err := db.QueryRow(`SELECT body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256 FROM cover_letters WHERE job_id=?`, closedJob.ID).Scan(&afterBody, &afterLetterProvider, &afterModel, &afterMasterID, &afterMasterSHA, &afterRoleID, &afterRoleSHA, &afterJobSHA); err != nil {
		t.Fatal(err)
	}
	if afterBody != closedLetterBody || afterLetterProvider != closedLetterProvider || afterModel != closedLetterModel || afterMasterID != closedMasterID || afterMasterSHA != closedMasterSHA || afterRoleID != closedRoleID || afterRoleSHA != closedRoleSHA || afterJobSHA != closedJobSHA {
		t.Fatalf("closed letter changed on retirement")
	}
	var afterInspectionURL string
	if err := db.QueryRow(`SELECT application_url FROM application_inspections WHERE job_id=? ORDER BY id DESC LIMIT 1`, closedJob.ID).Scan(&afterInspectionURL); err != nil || afterInspectionURL != kan16QualClosedPosting {
		t.Fatalf("closed inspection changed on retirement: %q %v", afterInspectionURL, err)
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
	rebound, err := a.jobByID(closedJob.ID)
	if err != nil || rebound.Stage != StageClosed || rebound.SelectedResumeID != closedJob.SelectedResumeID {
		t.Fatalf("rebind changed closed retirement: %+v %v", rebound, err)
	}
	var rebindDest, rebindProvider, rebindKey, rebindLabel, rebindAnswer string
	var rebindRevision int64
	if err := db.QueryRow(`SELECT destination,provider,question_key,label,answer,revision FROM application_answers WHERE job_id=?`, closedJob.ID).Scan(&rebindDest, &rebindProvider, &rebindKey, &rebindLabel, &rebindAnswer, &rebindRevision); err != nil {
		t.Fatal(err)
	}
	if rebindDest != closedDest || rebindProvider != closedProvider || rebindKey != closedKey || rebindLabel != closedLabel || rebindAnswer != closedAnswer || rebindRevision != closedRevision {
		t.Fatalf("rebind changed closed answer: %+v", rebindAnswer)
	}
	var rebindBody string
	var rebindMasterID, rebindRoleID int64
	if err := db.QueryRow(`SELECT body,master_resume_id,role_resume_id FROM cover_letters WHERE job_id=?`, closedJob.ID).Scan(&rebindBody, &rebindMasterID, &rebindRoleID); err != nil || rebindBody != closedLetterBody || rebindMasterID != closedMasterID || rebindRoleID != closedRoleID {
		t.Fatalf("rebind changed closed letter: %v", err)
	}
	if err := db.QueryRow("SELECT count(*) FROM application_answers WHERE job_id=?", closedJob.ID).Scan(&answers); err != nil || answers != 1 {
		t.Fatalf("rebind changed closed answers: %d %v", answers, err)
	}
	if err := db.QueryRow("SELECT count(*) FROM cover_letters WHERE job_id=?", closedJob.ID).Scan(&letters); err != nil || letters != 1 {
		t.Fatalf("rebind changed closed letters: %d %v", letters, err)
	}
}
