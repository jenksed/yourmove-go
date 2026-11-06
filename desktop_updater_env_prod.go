//go:build desktop && darwin && production

package main

// Production builds: environment variables never supply the update channel,
// trust root, or automatic-install trigger. Only the ldflags-pinned embedded
// manifest URL and Ed25519 public key are honoured; when they are absent the
// updater stays disabled (fail closed) and the manual DMG path remains.

func updaterEnvManifestURL() string { return "" }
func updaterEnvPublicKey() []byte   { return nil }
func updateTestAutoEnabled() bool   { return false }
func updateBGFastEnabled() bool     { return false }
