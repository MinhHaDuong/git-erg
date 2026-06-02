package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// manifestName is the provenance manifest written under tickets/. It records,
// for the current binary, the rev/date and the SHA-256 of each embedded asset
// init lays down. It is the "reference stamp" the dpkg-style 3-state compare
// (ticket 0211) reads to tell a clean upgrade from a local edit. It is
// committable durable state -- not gitignored -- and invisible to erg check
// (not a .erg file), so it never trips the pre-commit hook.
const manifestName = ".erg-assets"

// buildManifest returns the deterministic provenance manifest content for the
// embedded assets in initAssetPaths. The hashes are of the EMBEDDED content
// (what this binary ships), so the manifest is the reference a clean init
// would produce. Same binary + same embedded assets => byte-identical output.
// rev/date come from the build stamp (version.go), so the manifest is stable
// for a given binary (not wall-clock dependent).
func buildManifest() (string, error) {
	type entry struct{ name, sum string }
	var entries []entry
	for _, rel := range initAssetPaths {
		content, ok := bootstrapAsset(rel)
		if !ok {
			return "", fmt.Errorf("missing embedded asset: %s", rel)
		}
		sum := sha256.Sum256([]byte(content))
		entries = append(entries, entry{
			name: strings.TrimPrefix(rel, "tickets/"),
			sum:  hex.EncodeToString(sum[:]),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })

	rev := vcsRevision
	if rev == "" {
		rev = "unknown"
	}
	date := buildDate
	if date == "" {
		date = "unknown"
	}

	var b strings.Builder
	b.WriteString("# erg provenance manifest -- do not edit\n")
	fmt.Fprintf(&b, "rev: %s\n", rev)
	fmt.Fprintf(&b, "date: %s\n", date)
	b.WriteString("assets:\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "  %s sha256:%s\n", e.name, e.sum)
	}
	return b.String(), nil
}

// writeManifest writes the provenance manifest under root/tickets/. In dryRun
// it prints a preview line and writes nothing.
func writeManifest(root string, dryRun bool) error {
	content, err := buildManifest()
	if err != nil {
		return err
	}
	if dryRun {
		fmt.Printf("  would write provenance manifest %s\n", filepath.Join("tickets", manifestName))
		return nil
	}
	return atomicWriteFile(filepath.Join(root, "tickets", manifestName), []byte(content), 0644)
}
