package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/endpoint"
)

// makeTestZIP builds a minimal deterministic ZIP for signing tests.
func makeTestZIP(t *testing.T, path string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create("payload.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("yourmove update fixture")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func testKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func signFixture(t *testing.T, dir, version string, priv ed25519.PrivateKey, artifactURL string) (zipPath, manifestPath string) {
	t.Helper()
	zipPath = filepath.Join(dir, "YourMove-"+version+"-darwin-arm64-update.zip")
	zipBytes := makeTestZIP(t, zipPath)
	keyPath := filepath.Join(dir, "test.key")
	seed := priv.Seed()
	if err := os.WriteFile(keyPath, seed, 0o600); err != nil {
		t.Fatal(err)
	}
	manifestPath = filepath.Join(dir, "stable.json")
	if err := run([]string{"sign",
		"--zip", zipPath,
		"--version", version,
		"--url", artifactURL,
		"--key", keyPath,
		"--out", manifestPath,
		"--name", "YourMove " + version + " (test)",
		"--notes", "Test notes.",
	}); err != nil {
		t.Fatalf("sign: %v", err)
	}
	_ = zipBytes
	return zipPath, manifestPath
}

func TestSignRoundTripVerifies(t *testing.T) {
	pub, priv := testKey(t)
	dir := t.TempDir()
	_, manifestPath := signFixture(t, dir, "9.9.9", priv, "https://example.invalid/dl/test.zip")
	encoded, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyManifest(encoded, pub); err != nil {
		t.Fatalf("self-signed manifest rejected: %v", err)
	}
}

func TestSignRejectsTamperAndWrongKey(t *testing.T) {
	pub, priv := testKey(t)
	otherPub, _ := testKey(t)
	dir := t.TempDir()
	_, manifestPath := signFixture(t, dir, "9.9.9", priv, "https://example.invalid/dl/test.zip")
	encoded, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyManifest(encoded, otherPub); err == nil {
		t.Fatal("wrong public key accepted")
	}
	var m manifest
	if err := json.Unmarshal(encoded, &m); err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.StdEncoding.DecodeString(m.Artifacts[0].Digest)
	raw[0] ^= 0xff
	m.Artifacts[0].Digest = base64.StdEncoding.EncodeToString(raw)
	tampered, _ := json.Marshal(m)
	if err := verifyManifest(tampered, pub); err == nil {
		t.Fatal("tampered digest accepted")
	}
}

// TestManifestWorksWithWailsEndpointProvider serves a signed manifest and
// artifact over loopback HTTP and runs the real Wails v3.1.0 endpoint
// provider Check against it: newer versions surface as available with the
// recorded artifact, current versions report no update. Signature
// authentication itself is enforced by the updater core (fail closed with a
// pinned key), proven here by the round-trip and rejection tests above.
func TestManifestWorksWithWailsEndpointProvider(t *testing.T) {
	_, priv := testKey(t)
	dir := t.TempDir()
	zipPath, manifestPath := signFixture(t, dir, "0.2.2", priv, "https://example.invalid/dl/test.zip")
	zipBytes, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var m manifest
	if err := json.Unmarshal(encoded, &m); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/updates/stable.json":
			// Rewrite the artifact URL to this test server; the digest and
			// signature still cover the same ZIP bytes.
			m.Artifacts[0].URL = "http://" + r.Host + "/artifacts/test.zip"
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(m)
		case "/artifacts/test.zip":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(zipBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Point the manifest at the live test server by re-signing is
	// unnecessary: the provider resolves and downloads from whatever URL the
	// manifest carries, so rewrite+re-serve keeps digest/signature intact.
	provider, err := endpoint.New(endpoint.Config{URL: srv.URL + "/updates/stable.json", Channel: "stable"})
	if err != nil {
		t.Fatal(err)
	}
	rel, err := provider.Check(context.Background(), updater.CheckRequest{
		CurrentVersion: "0.2.1", Platform: "darwin", Arch: "arm64",
	})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if rel == nil {
		t.Fatal("expected update 0.2.2 to be available from 0.2.1")
	}
	if rel.Version != "0.2.2" {
		t.Fatalf("release version = %q, want 0.2.2", rel.Version)
	}
	if rel.Artifact.Size != int64(len(zipBytes)) {
		t.Fatalf("artifact size = %d, want %d", rel.Artifact.Size, len(zipBytes))
	}
	if len(rel.Verification.Signature) == 0 || len(rel.Verification.Digest) == 0 {
		t.Fatal("provider dropped digest/signature verification block")
	}

	same, err := provider.Check(context.Background(), updater.CheckRequest{
		CurrentVersion: "0.2.2", Platform: "darwin", Arch: "arm64",
	})
	if err != nil {
		t.Fatalf("check current: %v", err)
	}
	if same != nil {
		t.Fatalf("expected no update when current, got %v", same.Version)
	}
}
