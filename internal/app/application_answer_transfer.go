package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"

	"github.com/jenksed/yourmove-go/internal/application/browser"
)

func (p questionPreparation) ReadyAnswers() []preparedQuestion {
	var ready []preparedQuestion
	for _, q := range p.Questions {
		if q.Editable && q.Value != "" {
			ready = append(ready, q)
		}
	}
	sort.Slice(ready, func(i, j int) bool { return ready[i].Key < ready[j].Key })
	return ready
}
func (p questionPreparation) ReviewHash() string {
	type reviewedAnswer struct{ Key, Value string }
	var reviewed []reviewedAnswer
	for _, q := range p.ReadyAnswers() {
		reviewed = append(reviewed, reviewedAnswer{q.Key, q.Value})
	}
	data, _ := json.Marshal(reviewed)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

type applicationAnswerTransfer struct {
	hash, status string
	approved     []preparedQuestion
	attempted    []preparedQuestion // Session-only snapshots prevent replay, including uncertain outcomes.
	outcomes     []browser.AnswerOutcome
	current      func(preparedQuestion) bool
	resolve      map[string]func(context.Context, readOnlySession, preparedQuestion) (preparedQuestion, string)
}

func reactAnswerScopeHash(scope []preparedQuestion) string {
	type answer struct {
		Key, Value, Destination, Source, Fingerprint, QuestionID string
		Revision, ID                                             int64
		Field                                                    any
		Early                                                    bool
	}
	items := make([]answer, 0, len(scope))
	for _, q := range scope {
		items = append(items, answer{q.Key, q.Value, q.Destination, q.DiscoverySource, q.DiscoveryFingerprint, q.SourceQuestionID, q.Revision, q.EarlyAnswerID, q.Field, q.IsEarly})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	data, _ := json.Marshal(items)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func (a *App) reactAnswerTransfer(job Job, scope []preparedQuestion, profileRevision string) *applicationAnswerTransfer {
	up := &applicationAnswerTransfer{hash: reactAnswerScopeHash(scope), status: "waiting", approved: scope, resolve: map[string]func(context.Context, readOnlySession, preparedQuestion) (preparedQuestion, string){}}
	for _, item := range scope {
		q := item
		up.resolve[q.Key] = func(ctx context.Context, session readOnlySession, q preparedQuestion) (preparedQuestion, string) {
			latest, err := a.jobByID(job.ID)
			if err != nil || latest.URL != job.URL || latest.Stage == StageApplied || latest.Stage == StageClosed || latest.SubmissionCount > 0 {
				return q, "manual"
			}
			profile, err := a.candidateProfile()
			if err != nil || (profileRevision != "" && fmt.Sprint(profile.Revision) != profileRevision) {
				return q, "manual"
			}
			if !q.IsEarly {
				if q.Revision > 0 {
					saved, e := a.savedLiveAnswer(latest, q.Key)
					if e != nil || saved.Revision != q.Revision || saved.Value != q.Value {
						return q, "manual"
					}
				} else {
					profile.Approved = true
					prep, e := a.prepareQuestions(latest, profile)
					valid := false
					if e == nil {
						for _, candidate := range prep.ReadyAnswers() {
							if candidate.Key == q.Key && candidate.Value == q.Value {
								valid = true
								break
							}
						}
					}
					if !valid {
						return q, "manual"
					}
				}
				return q, "ready"
			}
			answer, err := earlyApplicationAnswerByID(a, job.ID, q.EarlyAnswerID)
			if err != nil || answer.Revision != q.Revision || answer.Answer != q.Value || answer.Destination != q.Destination || answer.DiscoverySource != q.DiscoverySource || answer.DiscoveryFingerprint != q.DiscoveryFingerprint || answer.SourceQuestionID != q.SourceQuestionID || earlyFieldSnapshot(answer.Field) != earlyFieldSnapshot(q.Field) {
				return q, "manual"
			}
			public, err := a.latestValidInspectionForPosting(job.ID, q.DiscoverySource, job.URL)
			if err != nil || public.Fingerprint != q.DiscoveryFingerprint {
				return q, "manual"
			}
			observation, err := session.Observe(ctx)
			if err != nil {
				return q, "waiting"
			}
			result, err := browser.AsResult(job.URL, observation)
			if err != nil {
				return q, "waiting"
			}
			live := Inspection{Platform: result.Platform, Source: result.Source, ApplicationURL: result.ApplicationURL, Completeness: "rendered_partial"}
			for _, f := range result.Fields {
				live.Fields = append(live.Fields, InspectedField{Field: f})
			}
			match := reconcileEarlyAnswer(latest, answer, &public, live, true)
			if match.Status == "Not observed yet" || match.Status == "No longer available" {
				return q, "waiting"
			}
			if !match.CanAccept {
				return q, "manual"
			}
			_, err = a.savedLiveAnswer(latest, match.LiveQuestionKey)
			if !errors.Is(err, sql.ErrNoRows) {
				return q, "manual"
			}
			qualified := q
			qualified.Field = match.LiveField
			qualified.Key = match.LiveQuestionKey
			if validateApplicationAnswerValue(qualified, q.Value) != "" {
				return q, "manual"
			}
			return qualified, "ready"
		}
	}
	return up
}

type answerFillSession interface {
	FillAnswer(context.Context, browser.AnswerValue) (browser.AnswerOutcome, error)
}

func (a *App) transferApplicationAnswers(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	status, detail := a.answerTransferCommand(r, id)
	writeTransferResult(w, r, status, detail)
}

// answerTransferCommand validates the exact submitted material before arming its existing operation.
func (a *App) answerTransferCommand(r *http.Request, id int64) (int, string) {
	return a.answerTransferForAction(r, id, false)
}

func (a *App) answerTransferForAction(r *http.Request, id int64, profileAuthorized bool) (int, string) {
	job, err := a.jobByID(id)
	if err != nil {
		return http.StatusNotFound, "Opportunity or material unavailable"
	}
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 {
		return 409, "This application is no longer active"
	}
	profile, err := a.candidateProfile()
	if err != nil {
		return 500, "Unable to read candidate information"
	}
	profileRevision := profile.Revision
	if profileAuthorized {
		if r.PostForm.Get("profile_revision") != fmt.Sprint(profile.Revision) {
			return http.StatusConflict, "Candidate Profile changed. Review it before filling."
		}
		profile.Approved = true
	}
	prepared, err := a.prepareQuestions(job, profile)
	if err != nil {
		return 500, "Unable to read application answers"
	}
	state := a.browsers.state(id)
	prepared.Questions = pendingAnswers(prepared.ReadyAnswers(), state.AnswerAttempted, state.ContactStatus != "")
	if r.PostForm.Get("review_hash") != prepared.ReviewHash() || r.PostForm.Get("destination") != job.URL || len(prepared.ReadyAnswers()) == 0 {
		return 409, "Review the current saved answers and destination before transfer"
	}
	up := &applicationAnswerTransfer{hash: prepared.ReviewHash(), status: "waiting", approved: prepared.ReadyAnswers()}
	up.current = func(q preparedQuestion) bool {
		latest, e := a.jobByID(id)
		if e != nil || latest.URL != job.URL || latest.Stage == StageClosed || latest.Stage == StageApplied || latest.SubmissionCount > 0 {
			return false
		}
		p, e := a.candidateProfile()
		if e != nil {
			return false
		}
		if profileAuthorized {
			if p.Revision != profileRevision {
				return false
			}
			p.Approved = true
		}
		if q.Revision > 0 {
			saved, e := a.savedLiveAnswer(latest, q.Key)
			return e == nil && saved.Revision == q.Revision && saved.Value == q.Value
		}
		current, e := a.prepareQuestions(latest, p)
		if e != nil {
			return false
		}
		for _, candidate := range current.ReadyAnswers() {
			if candidate.Key == q.Key && candidate.Value == q.Value {
				return true
			}
		}
		return false
	}
	if err := a.browsers.armAnswers(id, job.URL, up); err != nil {
		return 409, err.Error()
	}
	return http.StatusSeeOther, fmt.Sprintf("/jobs/%d#prepare", id)
}
func (h *sessionHost) armAnswers(id int64, destination string, up *applicationAnswerTransfer) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sessions[id] == nil || h.cancels[id] == nil || h.states[id].Destination != destination {
		return errors.New("Open the correct managed application browser before transferring answers")
	}
	if h.answers == nil {
		h.answers = map[int64]*applicationAnswerTransfer{}
	}
	if old := h.answers[id]; old != nil {
		newAnswers := pendingAnswers(up.approved, old.attempted, false)
		newAnswers = excludePendingAuthorizations(newAnswers, old.approved)
		if handoff := h.handoffs[id]; handoff != nil && handoff.contactStatus != "" {
			newAnswers = pendingAnswers(newAnswers, nil, true)
		}
		if len(newAnswers) == 0 {
			return errors.New("No new answers are ready; review the existing outcomes in Chromium")
		}
		if old.resolve == nil {
			old.resolve = map[string]func(context.Context, readOnlySession, preparedQuestion) (preparedQuestion, string){}
		}
		for _, q := range newAnswers {
			if resolver := up.resolve[q.Key]; resolver != nil {
				old.resolve[q.Key] = resolver
			} else if up.current != nil {
				current := up.current
				old.resolve[q.Key] = func(_ context.Context, _ readOnlySession, q preparedQuestion) (preparedQuestion, string) {
					if current(q) {
						return q, "ready"
					}
					return q, "manual"
				}
			}
		}
		old.approved = append(old.approved, newAnswers...)
		old.hash = up.hash
		if old.status != "checking" {
			old.status = "waiting"
		}
		return nil
	}
	if handoff := h.handoffs[id]; handoff != nil && handoff.contactStatus != "" {
		up.approved = pendingAnswers(up.approved, nil, true)
	}
	if len(up.approved) == 0 {
		return errors.New("No new answers are ready; review the existing outcomes in Chromium")
	}
	h.answers[id] = up
	return nil
}
func (h *sessionHost) tryAnswers(ctx context.Context, id int64) {
	if ctx.Err() != nil {
		return
	}
	h.mu.Lock()
	up, s := h.answers[id], h.sessions[id]
	if up == nil || s == nil || up.status != "waiting" {
		h.mu.Unlock()
		return
	}
	up.status = "checking"
	batch := append([]preparedQuestion(nil), up.approved...)
	resolvers := make(map[string]func(context.Context, readOnlySession, preparedQuestion) (preparedQuestion, string), len(up.resolve))
	for key, resolver := range up.resolve {
		resolvers[key] = resolver
	}
	current := up.current
	h.mu.Unlock()
	var outcomes []browser.AnswerOutcome
	var waiting []preparedQuestion
	filler, ok := s.(answerFillSession)
	for _, q := range batch {
		out := browser.AnswerOutcome{Label: q.Field.Label, Status: "manual", Detail: "The approved answer changed or transfer is unavailable. Review it in Prepare."}
		if ctx.Err() != nil {
			waiting = append(waiting, q)
			continue
		} else if ok {
			fill := q
			if resolve := resolvers[q.Key]; resolve != nil {
				var status string
				fill, status = resolve(ctx, s, q)
				if status == "waiting" {
					waiting = append(waiting, q)
					continue
				}
				if status != "ready" {
					outcomes = append(outcomes, out)
					continue
				}
			} else if current == nil || !current(q) {
				outcomes = append(outcomes, out)
				continue
			}

			if q.Revision > 0 && !q.IsEarly {
				observation, err := s.Observe(ctx)
				if err != nil {
					waiting = append(waiting, q)
					continue
				}
				result, err := browser.AsResult(q.Destination, observation)
				if err != nil {
					waiting = append(waiting, q)
					continue
				}
				found, compatible := false, false
				for _, field := range result.Fields {
					if field.ID == q.Field.ID {
						found = true
						field.Group = q.Field.Group
						compatible = earlyFieldSnapshot(field) == earlyFieldSnapshot(q.Field)
						break
					}
				}
				if !found {
					waiting = append(waiting, q)
					continue
				}
				if !compatible {
					outcomes = append(outcomes, out)
					continue
				}
			}
			// Record the attempt before the external effect. A concurrent reader
			// must never interpret an in-flight write as safe to replay.
			h.mu.Lock()
			if h.answers[id] != up {
				h.mu.Unlock()
				return
			}
			if ctx.Err() != nil {
				h.mu.Unlock()
				waiting = append(waiting, q)
				continue
			}
			// A prior dispatch may have happened after this snapshot was authorized.
			if len(pendingAnswers([]preparedQuestion{q}, up.attempted, false)) == 0 {
				h.mu.Unlock()
				outcomes = append(outcomes, out)
				continue
			}
			up.attempted = append(up.attempted, q)
			h.mu.Unlock()
			var err error
			out, err = filler.FillAnswer(ctx, browser.AnswerValue{Field: fill.Field, Value: q.Value})
			if err != nil {
				out = browser.AnswerOutcome{Label: q.Field.Label, Status: "uncertain", Detail: "This answer could not be verified. Review the live form; other successful fields remain completed."}
			}
		}
		outcomes = append(outcomes, out)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.answers[id] != up {
		return
	}
	up.outcomes = append(up.outcomes, outcomes...)
	up.approved = append(waiting, up.approved[len(batch):]...)
	if len(up.approved) > 0 {
		up.status = "waiting"
	} else {
		up.status = "complete"
	}
}
