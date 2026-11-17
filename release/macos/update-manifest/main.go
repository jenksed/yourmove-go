// Command update-manifest creates and verifies Wails endpoint
// update manifests carrying a signed artifact digest for YourMove macOS releases.
//
// It is release tooling, not application code: it never ships in the app,
// never embeds key material, and handles the owner-controlled private
// signing key only through explicit file arguments. Private keys must live
// outside the repository with owner-only permissions.
//
// Usage:
//
//	update-manifest gen-key --out /private/path/update.key [--pub-out /private/path/update.pub]
//	update-manifest sign --zip YourMove-<v>-darwin-arm64-update.zip --version 0.2.1 \
//	  --url https://github.com/jenksed/yourmove-go/releases/download/v0.2.1/<zip> \
//	  --key /private/path/update.key --out stable.json \
//	  --name "YourMove 0.2.1" --notes-file notes.md
//
// The manifest matches the Wails v3.1.0 endpoint provider schema
// (schemaVersion 1, digestAlgo sha256, signatureAlgo ed25519 over the raw
// SHA-256 digest). Manifest metadata itself is not signed. sign re-verifies the written manifest against the derived
// public key and fails closed on any mismatch.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const usage = `update-manifest: signed Wails update manifests for YourMove macOS releases.

Subcommands:
  gen-key --out PRIVATE [--pub-out PUBLIC_PEM]
      Generate a fresh Ed25519 signing key. PRIVATE is PKCS#8 PEM written
      with owner-only permissions; PUBLIC_PEM is the PKIX PEM public key
      for YOURMOVE_UPDATE_PUBLIC_KEY_FILE at build time.
  sign --zip ZIP --version VERSION --url ARTIFACT_URL --key PRIVATE --out MANIFEST
       [--channel CHANNEL] [--name NAME] [--notes NOTES | --notes-file FILE]
      Sign ZIP and write the authenticated manifest. Re-verifies before exit.
`

type manifestArtifact struct {
	URL           string `json:"url"`
	Filename      string `json:"filename"`
	Filetype      string `json:"filetype"`
	Size          int64  `json:"size"`
	Platform      string `json:"platform"`
	Arch          string `json:"arch"`
	DigestAlgo    string `json:"digestAlgo"`
	Digest        string `json:"digest"`
	SignatureAlgo string `json:"signatureAlgo"`
	Signature     string `json:"signature"`
}

type manifest struct {
	SchemaVersion int                `json:"schemaVersion"`
	Version       string             `json:"version"`
	Channel       string             `json:"channel"`
	Name          string             `json:"name"`
	Notes         string             `json:"notes"`
	PublishedAt   string             `json:"publishedAt"`
	Artifacts     []manifestArtifact `json:"artifacts"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "update-manifest: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("missing subcommand\n" + usage)
	}
	switch args[0] {
	case "gen-key":
		return runGenKey(args[1:])
	case "sign":
		return runSign(args[1:])
	default:
		return fmt.Errorf("unknown subcommand %q\n%s", args[0], usage)
	}
}

func runGenKey(args []string) error {
	fs := flag.NewFlagSet("gen-key", flag.ContinueOnError)
	out := fs.String("out", "", "private key output path (PKCS#8 PEM, mode 0600)")
	pubOut := fs.String("pub-out", "", "public key output path (PKIX PEM)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return errors.New("gen-key: --out is required")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("gen-key: generate: %w", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("gen-key: marshal: %w", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(*out, privPEM, 0o600); err != nil {
		return fmt.Errorf("gen-key: write private: %w", err)
	}
	pubPEM := publicKeyPEM(pub)
	if *pubOut != "" {
		if err := os.WriteFile(*pubOut, pubPEM, 0o600); err != nil {
			return fmt.Errorf("gen-key: write public: %w", err)
		}
	}
	sum := sha256.Sum256([]byte(pub))
	fmt.Printf("private: %s\n", *out)
	if *pubOut != "" {
		fmt.Printf("public:  %s\n", *pubOut)
	}
	fmt.Printf("public key SHA-256 fingerprint: %x\n", sum)
	fmt.Printf("%s", pubPEM)
	return nil
}

func publicKeyPEM(pub ed25519.PublicKey) []byte {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		panic(fmt.Sprintf("marshal public key: %v", err))
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
}

func loadPrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key: %w", err)
	}
	if block, _ := pem.Decode(data); block != nil {
		anyKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS#8: %w", err)
		}
		priv, ok := anyKey.(ed25519.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is %T, not Ed25519", anyKey)
		}
		return priv, nil
	}
	switch len(data) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(data), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(bytes.Clone(data)), nil
	default:
		return nil, fmt.Errorf("unrecognised key format (%d bytes): want PKCS#8 PEM, 32-byte seed, or 64-byte private key", len(data))
	}
}

func runSign(args []string) error {
	fs := flag.NewFlagSet("sign", flag.ContinueOnError)
	zipPath := fs.String("zip", "", "packaged update ZIP (from package-update.sh)")
	version := fs.String("version", "", "release version, e.g. 0.2.1")
	channel := fs.String("channel", "stable", "release channel")
	name := fs.String("name", "", "release display name")
	notes := fs.String("notes", "", "release notes text")
	notesFile := fs.String("notes-file", "", "file with release notes text")
	artifactURL := fs.String("url", "", "absolute artifact URL recorded in the manifest")
	keyPath := fs.String("key", "", "private signing key file (never in the repo)")
	out := fs.String("out", "", "manifest output path (stable.json)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	for _, req := range []struct {
		label, value string
	}{
		{"--zip", *zipPath}, {"--version", *version},
		{"--url", *artifactURL}, {"--key", *keyPath}, {"--out", *out},
	} {
		if req.value == "" {
			return fmt.Errorf("sign: %s is required", req.label)
		}
	}
	if *notes != "" && *notesFile != "" {
		return errors.New("sign: --notes and --notes-file are mutually exclusive")
	}
	noteText := *notes
	if *notesFile != "" {
		data, err := os.ReadFile(*notesFile)
		if err != nil {
			return fmt.Errorf("sign: read notes: %w", err)
		}
		noteText = string(data)
	}
	zipBytes, err := os.ReadFile(*zipPath)
	if err != nil {
		return fmt.Errorf("sign: read zip: %w", err)
	}
	if len(zipBytes) == 0 {
		return errors.New("sign: zip is empty")
	}
	priv, err := loadPrivateKey(*keyPath)
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}
	pub := priv.Public().(ed25519.PublicKey)

	digest := sha256.Sum256(zipBytes)
	sig := ed25519.Sign(priv, digest[:])

	m := manifest{
		SchemaVersion: 1,
		Version:       *version,
		Channel:       *channel,
		Name:          *name,
		Notes:         noteText,
		PublishedAt:   time.Now().UTC().Format(time.RFC3339),
		Artifacts: []manifestArtifact{{
			URL:           *artifactURL,
			Filename:      filepath.Base(*zipPath),
			Filetype:      "zip",
			Size:          int64(len(zipBytes)),
			Platform:      "darwin",
			Arch:          "arm64",
			DigestAlgo:    "sha256",
			Digest:        base64.StdEncoding.EncodeToString(digest[:]),
			SignatureAlgo: "ed25519",
			Signature:     base64.StdEncoding.EncodeToString(sig),
		}},
	}
	encoded, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("sign: encode: %w", err)
	}
	// Fail closed: re-verify the exact bytes about to be published.
	if err := verifyManifest(encoded, pub); err != nil {
		return fmt.Errorf("sign: self-verification failed: %w", err)
	}
	if err := os.WriteFile(*out, append(encoded, '\n'), 0o644); err != nil {
		return fmt.Errorf("sign: write manifest: %w", err)
	}
	sum := sha256.Sum256([]byte(pub))
	fmt.Printf("manifest: %s\nversion:  %s\nchannel:  %s\nartifact: %s (%d bytes)\npublic key SHA-256 fingerprint: %x\n",
		*out, *version, *channel, *artifactURL, len(zipBytes), sum)
	return nil
}

// verifyManifest decodes a manifest exactly as the Wails endpoint provider
// would (base64 digest/signature, sha256/ed25519) and verifies the artifact
// signature under pub. The caller supplies the expected artifact bytes
// separately only through digest comparison; here the digest itself is the
// signed message, matching updater/ed25519Verifier semantics.
func verifyManifest(encoded []byte, pub ed25519.PublicKey) error {
	var m manifest
	if err := json.Unmarshal(encoded, &m); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	if m.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schemaVersion %d", m.SchemaVersion)
	}
	if len(m.Artifacts) != 1 {
		return fmt.Errorf("expected exactly 1 artifact, got %d", len(m.Artifacts))
	}
	a := m.Artifacts[0]
	if a.DigestAlgo != "sha256" || a.SignatureAlgo != "ed25519" {
		return fmt.Errorf("unexpected algorithms %q/%q", a.DigestAlgo, a.SignatureAlgo)
	}
	digest, err := base64.StdEncoding.DecodeString(a.Digest)
	if err != nil {
		return fmt.Errorf("bad digest encoding: %w", err)
	}
	sig, err := base64.StdEncoding.DecodeString(a.Signature)
	if err != nil {
		return fmt.Errorf("bad signature encoding: %w", err)
	}
	if len(digest) != sha256.Size {
		return fmt.Errorf("bad digest length %d", len(digest))
	}
	if !ed25519.Verify(pub, digest, sig) {
		return errors.New("ed25519 signature did not verify")
	}
	return nil
}
