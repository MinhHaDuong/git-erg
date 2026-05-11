package main

import "embed"

//go:embed assets/AGENTS.md assets/spec-erg-v1.md assets/integration.md
var embeddedAssets embed.FS

var assetMapping = map[string]string{
	"tickets/AGENTS.md":      "assets/AGENTS.md",
	"tickets/spec-erg-v1.md": "assets/spec-erg-v1.md",
	"tickets/integration.md": "assets/integration.md",
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
