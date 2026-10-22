package main

import (
	"context"
	"crypto/sha256"
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
	if old := h.answers[id]; old != nil && (old.status == "waiting" || old.status == "checking" || old.hash == up.hash) {
		return errors.New("These answers were already authorized. Review the transfer result or save a changed answer before continuing")
	}
	if h.answers == nil {
		h.answers = map[int64]*applicationAnswerTransfer{}
	}
	if old := h.answers[id]; old != nil {
		up.approved = pendingAnswers(up.approved, old.attempted, false)
		up.attempted = append(up.attempted, old.attempted...)
		up.outcomes = append(up.outcomes, old.outcomes...)
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
	h.mu.Unlock()
	var outcomes []browser.AnswerOutcome
	filler, ok := s.(answerFillSession)
	for _, q := range up.approved {
		out := browser.AnswerOutcome{Label: q.Field.Label, Status: "manual", Detail: "The approved answer changed or transfer is unavailable. Review it in Prepare."}
		if ctx.Err() != nil {
			out.Status = "uncertain"
			out.Detail = "Transfer interrupted. Review this field in Chromium."
		} else if ok && up.current(q) {
			var err error
			out, err = filler.FillAnswer(ctx, browser.AnswerValue{Field: q.Field, Value: q.Value})
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
	up.attempted = append(up.attempted, up.approved...)
	up.status = "complete"
	up.approved = nil
}
