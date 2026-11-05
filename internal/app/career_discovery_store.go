package app

// Quarantined interview-session store for YM-003A career discovery.
//
// A session holds one applicant's persisted interview working state: an
// opaque id, a loose resume reference plus excerpt hash, the serialized
// engine state, and a nullable link to a promoted Career Evidence draft.
// It is deliberately isolated from career_evidence: no foreign keys in
// either direction, no shared writer. The only bridge is promotion, which
// goes through the existing createCareerEvidenceDraft/updateCareerEvidence
// owner. Sessions expire and are swept on access; expiry never touches
// career_evidence rows.

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	discovery "github.com/jenksed/yourmove-career-discovery"
)

// careerDiscoverySessionTTL keeps an interview resumable across quit and
// relaunch long enough for a real applicant workflow without keeping
// abandoned working state forever.
const careerDiscoverySessionTTL = 30 * 24 * time.Hour

// careerDiscoveryStatuses are the only session states. A proposal is a
// property of the persisted engine state, not a status: an interview with
// a proposal may still continue or promote.
const (
	careerDiscoveryLeads     = "leads"
	careerDiscoveryInterview = "interview"
	careerDiscoveryPromoted  = "promoted"
)

var (
	// ErrCareerDiscoveryStale reports a revision conflict between concurrent editors.
	ErrCareerDiscoveryStale = errors.New("career discovery session changed")
	// ErrCareerDiscoveryState reports persisted state the engine no longer accepts.
	ErrCareerDiscoveryState = errors.New("career discovery session state is invalid")
)

type careerDiscoverySession struct {
	ID                string
	ResumeID          int64
	ResumeSHA256      string
	ResumeExcerptHash string
	State             discovery.State
	Status            string
	Provider          string
	Model             string
	FailureCode       string
	FailureDetail     string
	PromotedEvidence  int64 // 0 means no promoted draft
	Revision          int64
	CreatedAt         string
	UpdatedAt         string
	ExpiresAt         string
}

// careerDiscoveryGuard serializes external inference and promotion per
// session so two tabs cannot double-run (and double-bill) one authorized
// step or create two drafts. It guards only the in-flight call; revision
// gates serialize the persisted writes.
type careerDiscoveryGuard struct {
	mu      sync.Mutex
	running map[string]bool
}

func (g *careerDiscoveryGuard) begin(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.running == nil {
		g.running = make(map[string]bool)
	}
	if g.running[id] {
		return false
	}
	g.running[id] = true
	return true
}

func (g *careerDiscoveryGuard) finish(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.running, id)
}

func careerDiscoverySessionID() (string, error) {
	var id [32]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}

func careerDiscoveryHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// careerDiscoverySweepExpired deletes expired sessions. Career evidence
// rows are never touched: a promoted draft outlives its session.
func (a *App) careerDiscoverySweepExpired(now time.Time) error {
	_, err := a.db.Exec(`DELETE FROM career_discovery_sessions WHERE expires_at != '' AND expires_at < ?`,
		now.UTC().Format(time.RFC3339Nano))
	return err
}

const careerDiscoverySelect = `SELECT s.id,s.resume_id,s.resume_sha256,s.resume_excerpt_hash,s.state_json,s.status,
	s.provider,s.model,s.failure_code,s.failure_detail,s.promoted_evidence_id,s.revision,
	s.created_at,s.updated_at,s.expires_at,(SELECT name FROM resumes r WHERE r.id=s.resume_id)
	FROM career_discovery_sessions s`

func scanCareerDiscoverySession(row *sql.Row) (careerDiscoverySession, string, error) {
	var session careerDiscoverySession
	var stateJSON string
	var promoted sql.NullInt64
	var resumeName sql.NullString
	err := row.Scan(&session.ID, &session.ResumeID, &session.ResumeSHA256, &session.ResumeExcerptHash,
		&stateJSON, &session.Status, &session.Provider, &session.Model,
		&session.FailureCode, &session.FailureDetail, &promoted, &session.Revision,
		&session.CreatedAt, &session.UpdatedAt, &session.ExpiresAt, &resumeName)
	if err != nil {
		return session, "", err
	}
	if promoted.Valid {
		session.PromotedEvidence = promoted.Int64
	}
	name := ""
	if resumeName.Valid {
		name = resumeName.String
	}
	if err := json.Unmarshal([]byte(stateJSON), &session.State); err != nil {
		return session, name, fmt.Errorf("%w: %v", ErrCareerDiscoveryState, err)
	}
	if err := careerDiscoveryCheckState(session.State); err != nil {
		return session, name, err
	}
	return session, name, nil
}

// careerDiscoveryCheckState revalidates deserialized engine state before
// the host acts on it: exact response/source identities and, when a
// proposal is present, a fresh rebind to the recorded responses.
func careerDiscoveryCheckState(state discovery.State) error {
	if state.Resume.ID != "resume" || state.Resume.Kind != "document_excerpt" {
		return ErrCareerDiscoveryState
	}
	for i, lead := range state.Leads {
		if lead.ID != fmt.Sprintf("lead-%d", i+1) {
			return ErrCareerDiscoveryState
		}
	}
	if state.SelectedLeadID != "" && state.SelectedLead() == nil {
		return ErrCareerDiscoveryState
	}
	for i, response := range state.Responses {
		if response.Source.ID != fmt.Sprintf("response-%d", i+1) || response.Source.Kind != "self_report" {
			return ErrCareerDiscoveryState
		}
	}
	if state.Proposal != nil {
		if err := state.ValidateProposal(); err != nil {
			return ErrCareerDiscoveryState
		}
	}
	return nil
}

func (a *App) careerDiscoveryLoad(id string) (careerDiscoverySession, string, error) {
	if err := a.careerDiscoverySweepExpired(time.Now()); err != nil {
		return careerDiscoverySession{}, "", err
	}
	return scanCareerDiscoverySession(a.db.QueryRow(careerDiscoverySelect+` WHERE s.id=?`, id))
}

// careerDiscoveryStore persists one state transition under a revision gate
// in a single short write. Callers hold no transaction across inference.
func (a *App) careerDiscoveryStore(session careerDiscoverySession, expectedRevision int64) error {
	stateJSON, err := json.Marshal(session.State)
	if err != nil {
		return err
	}
	var promoted any
	if session.PromotedEvidence > 0 {
		promoted = session.PromotedEvidence
	}
	result, err := a.db.Exec(`UPDATE career_discovery_sessions
		SET state_json=?,status=?,provider=?,model=?,failure_code=?,failure_detail=?,promoted_evidence_id=?,
		revision=revision+1,updated_at=? WHERE id=? AND revision=?`,
		string(stateJSON), session.Status, session.Provider, session.Model,
		session.FailureCode, session.FailureDetail, promoted,
		time.Now().UTC().Format(time.RFC3339Nano), session.ID, expectedRevision)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrCareerDiscoveryStale
	}
	return nil
}
