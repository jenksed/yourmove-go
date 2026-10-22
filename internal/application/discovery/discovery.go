// Package discovery coordinates public, read-only provider discovery.
package discovery

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/providers/ashby"
	"github.com/jenksed/yourmove-go/internal/application/providers/greenhouse"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

var ErrUnsupported = errors.New("application discovery is unsupported for this posting")

type Inspector interface {
	Inspect(context.Context, string) (schema.Result, error)
}

// SupportsPublicQuestions reports providers whose documented public API can
// return question definitions. Other supported posting lookups may resolve a
// destination without exposing the employer's application form.
func SupportsPublicQuestions(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "job-boards.greenhouse.io", "boards.greenhouse.io":
		return true
	default:
		return false
	}
}

type Registry struct {
	Greenhouse Inspector
	Ashby      Inspector
}

func New() Registry { return Registry{Greenhouse: greenhouse.New(nil), Ashby: ashby.New(nil)} }
func (r Registry) Inspect(ctx context.Context, raw string) (schema.Result, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Hostname() == "" {
		return schema.Result{}, fmt.Errorf("invalid application posting URL")
	}
	var provider Inspector
	switch strings.ToLower(u.Hostname()) {
	case "job-boards.greenhouse.io", "boards.greenhouse.io":
		provider = r.Greenhouse
	case "jobs.ashbyhq.com":
		provider = r.Ashby
	default:
		return schema.Result{}, ErrUnsupported
	}
	if provider == nil {
		return schema.Result{}, ErrUnsupported
	}
	c, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	return provider.Inspect(c, raw)
}
