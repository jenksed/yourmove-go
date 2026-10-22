package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// Reuse is intentionally employer-scoped and exact: provider identity, field
// identity, wording, kind, options and sensitivity must all still agree.
// No label-only matching or AI inference participates in this decision.
func answerEmployerScope(destination string) string {
	u, err := url.Parse(destination)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if browser.PostingPlatform(destination) == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host + "/" + parts[0]
}
func reusableAnswerKey(destination string, f schema.Field) string {
	scope := answerEmployerScope(destination)
	if scope == "" || !editableQuestion(f, true) {
		return ""
	}
	data, _ := json.Marshal(struct {
		Provider, Employer string
		Field              schema.Field
	}{browser.PostingPlatform(destination), scope, f})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

type reusableAnswer struct {
	Value    string
	Revision int64
}

func (a *App) reusableAnswers(destination string) (map[string]reusableAnswer, error) {
	out := map[string]reusableAnswer{}
	rows, err := a.db.Query(`SELECT question_key,answer,revision FROM reusable_answers WHERE employer_scope=?`, answerEmployerScope(destination))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var v reusableAnswer
		if err := rows.Scan(&key, &v.Value, &v.Revision); err != nil {
			return nil, err
		}
		out[key] = v
	}
	return out, rows.Err()
}
