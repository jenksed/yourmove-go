//go:build desktop && darwin && !production

package main

import (
	"log"
	"os"
	"strings"
)

// Development builds only: controlled fixture channel via environment.
// Production builds (see desktop_updater_env_prod.go) ignore these entirely,
// so an environment variable can never replace the pinned trust root or
// silently enable automatic installation in a release candidate.

func updaterEnvManifestURL() string {
	return strings.TrimSpace(os.Getenv("YOURMOVE_UPDATE_MANIFEST_URL"))
}

func updaterEnvPublicKey() []byte {
	if path := strings.TrimSpace(os.Getenv("YOURMOVE_UPDATE_PUBLIC_KEY_FILE")); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("update public key file unreadable: %v", err)
			return nil
		}
		if len(data) == 0 {
			return nil
		}
		return data
	}
	if raw := strings.TrimSpace(os.Getenv("YOURMOVE_UPDATE_PUBLIC_KEY")); raw != "" {
		return []byte(strings.ReplaceAll(raw, `\n`, "\n"))
	}
	return nil
}

func updateTestAutoEnabled() bool { return os.Getenv("YOURMOVE_UPDATE_TEST_AUTO") == "1" }
func updateBGFastEnabled() bool   { return os.Getenv("YOURMOVE_UPDATE_BG_FAST") == "1" }
