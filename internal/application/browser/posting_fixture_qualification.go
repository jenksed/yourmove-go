//go:build desktop && darwin && qualification

package browser

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

const nativeFillFixtureAddress = "127.0.0.1:47417"
const nativeFillFixturePath = "/kan17-synthetic-application"
const NativeFillFixtureURL = "http://" + nativeFillFixtureAddress + nativeFillFixturePath

var nativeFillFixtureActive atomic.Bool

func isNativeFillFixture(raw string) bool {
	return raw == NativeFillFixtureURL && nativeFillFixtureActive.Load()
}

// StartNativeFillFixture owns the exact origin for the lifetime of the
// qualification process. A port collision fails before any posting is eligible.
func StartNativeFillFixture() (func(), error) {
	root := os.Getenv("YOURMOVE_DATA_DIR")
	if !filepath.IsAbs(root) || filepath.Dir(root) != "/private/tmp" || !strings.HasPrefix(filepath.Base(root), "yourmove-kan17-") {
		return nil, fmt.Errorf("KAN-17 requires a dedicated /private/tmp workspace")
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return nil, fmt.Errorf("KAN-17 requires a private workspace directory")
	}
	marker, err := os.ReadFile(filepath.Join(root, ".yourmove-kan17"))
	if err != nil || string(marker) != "kan17-native-fill-v1\n" {
		return nil, fmt.Errorf("KAN-17 synthetic workspace marker required")
	}
	if err := os.Chdir(root); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", nativeFillFixtureAddress)
	if err != nil {
		return nil, err
	}
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Host != nativeFillFixtureAddress || r.URL.RequestURI() != nativeFillFixturePath {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; form-action 'none'; frame-ancestors 'none'; base-uri 'none'")
		_, _ = io.WriteString(w, nativeFillFixturePage)
	})}
	nativeFillFixtureActive.Store(true)
	go func() {
		_ = server.Serve(listener)
		nativeFillFixtureActive.Store(false)
	}()
	return func() {
		nativeFillFixtureActive.Store(false)
		_ = server.Close()
	}, nil
}

const nativeFillFixturePage = `<!doctype html>
<meta charset="utf-8">
<title>KAN-17 Synthetic Form</title>
<style>body{font:18px system-ui;margin:3rem;max-width:42rem}label{display:block;margin:1rem 0}input{display:block;width:90%;padding:.5rem;font:inherit}</style>
<h1>KAN-17 Synthetic Form</h1>
<form>
  <label>What project would you like to discuss?<input id="project" name="project" type="text"></label>
  <label>What topic would you like to explore?<input id="topic" name="topic" type="text"></label>
  <label>What else should the team know?<input id="extra" name="extra" type="text"></label>
  <label>Résumé<input id="resume" name="resume" type="file" accept=".pdf"></label>
  <button type="submit">Synthetic submit</button>
</form>
<p>Submission attempts: <output id="attempts">0</output></p>
<script>
  const attempts = document.getElementById('attempts');
  document.querySelector('form').addEventListener('submit', event => {
    event.preventDefault();
    attempts.value = Number(attempts.value) + 1;
  });
</script>`
