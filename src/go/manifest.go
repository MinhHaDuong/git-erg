package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
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

// sha256hex returns the hex-encoded SHA-256 of b.
func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

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
		entries = append(entries, entry{
			name: strings.TrimPrefix(rel, "tickets/"),
			sum:  sha256hex([]byte(content)),
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

// readManifest parses tickets/.erg-assets and returns a map of asset name ->
// stamped SHA-256 hex. A missing OR malformed manifest returns nil: the caller
// treats absence and corruption identically (fall back to known shipped
// hashes), so a bad stamp never fails init.
func readManifest(root string) map[string]string {
	data, err := os.ReadFile(filepath.Join(root, "tickets", manifestName))
	if err != nil {
		return nil
	}
	const sep = " sha256:"
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		i := strings.Index(line, sep)
		if i <= 0 {
			continue
		}
		name := strings.TrimSpace(line[:i])
		hash := strings.TrimSpace(line[i+len(sep):])
		if name != "" && hash != "" {
			out[name] = hash
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// extraHistoricalHashes maps an asset name to SHA-256 hex digests of PAST
// shipped versions, beyond the current embedded one. It is the offline fallback
// the dpkg compare consults when no .erg-assets stamp is present.
//
// BOOTSTRAP / DORMANCY NOTE: this is intentionally empty today. knownAssetHashes
// always includes the CURRENT embedded hash, and installAssets checks
// onDisk==embedded ("unchanged") BEFORE consulting this table -- so for the
// running binary the table is never the deciding factor (dormant). Its
// historical-match branch is exercised by the Go unit test (which injects a
// distinct hash). It becomes load-bearing only as future releases append the
// hashes they supersede. ONLY add SHA-256 of genuinely shipped content: a false
// entry is the single way this table could cause data loss (a real local edit
// mistaken for a pristine old asset). An unrecognized old asset degrades safely
// to "preserve" (exit 2, rerunnable with --force) -- never a silent clobber.
var extraHistoricalHashes = map[string][]string{
	// "AGENTS.md": {"<sha256 of a past shipped AGENTS.md>"},
}

// knownAssetHashes returns every SHA-256 hex this binary recognizes as a
// genuinely shipped version of the asset at rel: the current embedded hash plus
// any entries in extraHistoricalHashes.
func knownAssetHashes(rel string) []string {
	var hashes []string
	if content, ok := bootstrapAsset(rel); ok {
		hashes = append(hashes, sha256hex([]byte(content)))
	}
	hashes = append(hashes, extraHistoricalHashes[strings.TrimPrefix(rel, "tickets/")]...)
	return hashes
}

// isCleanUpgrade reports whether an on-disk asset that differs from the current
// embedded content is nonetheless a pristine prior release (safe to overwrite),
// as opposed to a local edit (must be preserved). It is a clean upgrade when
// the on-disk hash matches the recorded stamp (the file is exactly what the
// last init wrote), or, when no stamp is recorded, a known shipped hash.
func isCleanUpgrade(diskHash, stampedHash string, known []string) bool {
	if stampedHash != "" {
		return diskHash == stampedHash
	}
	for _, h := range known {
		if diskHash == h {
			return true
		}
	}
	return false
}
