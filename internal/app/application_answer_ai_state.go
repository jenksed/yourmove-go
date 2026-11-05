package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	discovery "github.com/jenksed/yourmove-career-discovery"
)

// All mutable ticket fields belong to the registry mutex. Snapshots never change.
type answerAITicket struct {
	active              bool
	snapshot            answerGenerationSnapshot
	state, answer, code string
	// failStage and failDetail are privacy-safe output-failure diagnostics:
	// a short stage code plus byte/item counts only. Rejected model output
	// and candidate information are never retained here.
	failStage, failDetail string
	trace                 *discovery.AnswerTrace
	expires               time.Time
	cancel                context.CancelFunc
}
type answerAIRegistry struct {
	mu      sync.Mutex
	tickets map[string]*answerAITicket
	closed  bool
}

func (r *answerAIRegistry) cleanup(now time.Time) {
	for token, t := range r.tickets {
		if !t.active && !now.Before(t.expires) {
			delete(r.tickets, token)
		}
	}
}
func (r *answerAIRegistry) add(s answerGenerationSnapshot) (string, time.Time, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cleanup(time.Now())
	if r.closed || len(r.tickets) >= 16 {
		return "", time.Time{}, false
	}
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", time.Time{}, false
	}
	token := hex.EncodeToString(b[:])
	expires := time.Now().Add(10 * time.Minute)
	if r.tickets == nil {
		r.tickets = make(map[string]*answerAITicket)
	}
	r.tickets[token] = &answerAITicket{snapshot: s, state: "reviewed", expires: expires}
	return token, expires, true
}
func (r *answerAIRegistry) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	for _, t := range r.tickets {
		if t.cancel != nil {
			t.cancel()
		}
		t.state = "canceled"
		t.answer = ""
		t.trace = nil
	}
}
