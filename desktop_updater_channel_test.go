//go:build desktop && darwin && !qualification

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateChannelMissingIsAnError(t *testing.T) {
	s := httptest.NewServer(http.NotFoundHandler())
	defer s.Close()
	c := &http.Client{Transport: updateChannelTransport{http.DefaultTransport}}
	resp, err := c.Get(s.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("missing channel was treated as a successful check")
	}
}
