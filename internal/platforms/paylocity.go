package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type jobPosting struct {
	Type        string          `json:"@type"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	BaseSalary  json.RawMessage `json:"baseSalary"`
}

func fetchJobMetadata(ctx context.Context, jobURL string) (Metadata, error) {
	if !supportedMetadataURL(jobURL) {
		return Metadata{}, fmt.Errorf("metadata extraction not supported for URL")
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !supportedMetadataURL(req.URL.String()) {
				return fmt.Errorf("metadata redirect to unsupported host")
			}
			if len(via) >= 5 {
				return fmt.Errorf("too many metadata redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		jobURL,
		nil,
	)
	if err != nil {
		return Metadata{}, fmt.Errorf("create metadata request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return Metadata{}, fmt.Errorf("fetch job metadata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return Metadata{}, fmt.Errorf("%w: job page returned HTTP %d", ErrClosed, resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return Metadata{}, fmt.Errorf("job page returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil {
		return Metadata{}, fmt.Errorf("read job metadata: %w", err)
	}

	if len(body) > 1024*1024 {
		return Metadata{}, fmt.Errorf("job metadata exceeds 1 MiB")
	}

	return extractJobMetadata(bytes.NewReader(body))
}

func extractJobMetadata(r io.Reader) (Metadata, error) {
	tokenizer := html.NewTokenizer(r)

	for {
		tokenType := tokenizer.Next()

		if tokenType == html.ErrorToken {
			if tokenizer.Err() == io.EOF {
				break
			}
			return Metadata{}, fmt.Errorf("parse HTML: %w", tokenizer.Err())
		}

		if tokenType != html.StartTagToken {
			continue
		}

		token := tokenizer.Token()

		if token.Data != "script" {
			continue
		}

		isJSONLD := false

		for _, attr := range token.Attr {
			if attr.Key == "type" &&
				strings.EqualFold(attr.Val, "application/ld+json") {
				isJSONLD = true
				break
			}
		}

		if !isJSONLD {
			continue
		}

		if tokenizer.Next() != html.TextToken {
			continue
		}

		var posting jobPosting

		if err := json.Unmarshal(tokenizer.Text(), &posting); err != nil {
			continue
		}

		if posting.Type == "JobPosting" {
			return completeMetadata(Metadata{
				Title:       strings.TrimSpace(posting.Title),
				Description: descriptionText(posting.Description),
				SalaryText:  salaryFromJSONLD(posting.BaseSalary),
			}), nil
		}
	}

	return Metadata{}, fmt.Errorf("no JobPosting metadata found")
}

func supportedMetadataURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	return u.Scheme == "https" &&
		u.Hostname() == "2000recruiting.paylocity.com" &&
		u.User == nil &&
		u.Port() == ""
}
