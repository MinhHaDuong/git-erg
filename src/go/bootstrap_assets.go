package main

import "embed"

//go:embed assets/AGENTS.md assets/spec-erg-v1.md assets/integration.md assets/.ergrc assets/erg-github
var embeddedAssets embed.FS

// assetMapping maps a deployed path under tickets/ to the embedded copy this
// binary ships. Membership here means "this binary knows what the shipped
// bytes are", NOT "erg installs this file": tickets/erg-github is embedded as
// a read-only reference for the vendored-drift compare (ticket 0282) and is
// deliberately absent from initAssetPaths, so no write path ever reaches it.
// What erg writes is decided by the path LISTS in init.go, never by this map.
var assetMapping = map[string]string{
	"tickets/AGENTS.md":      "assets/AGENTS.md",
	"tickets/spec-erg-v1.md": "assets/spec-erg-v1.md",
	"tickets/integration.md": "assets/integration.md",
	"tickets/.ergrc":         "assets/.ergrc",
	"tickets/erg-github":     "assets/erg-github",
}

func bootstrapAsset(path string) (string, bool) {
	embedPath, ok := assetMapping[path]
	if !ok {
		return "", false
	}
	data, err := embeddedAssets.ReadFile(embedPath)
	if err != nil {
		return "", false
	}
	return string(data), true
}
