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

// assetDriftSignal is the stable substring of the asset-drift warning emitted by
// assetDriftWarnings. erg update greps the re-exec'd new binary's `erg check`
// output for it (ticket 0212), so producer and consumer share this one literal.
// Producer and consumer are DIFFERENT binaries (the new one prints, the old one
// greps), so this literal is a cross-version contract: it may be extended at the
// END only, never rewritten, and the printed line must keep containing the
// historical text verbatim. The remedy moved inside the constant (ticket 0279)
// precisely so the rollback signal below can name a DIFFERENT remedy while the
// printed line for this direction stays byte-identical to what it always was.
const assetDriftSignal = "differs from the .erg-assets stamp (binary upgraded since last init) -- run 'erg init' to refresh"

// assetRollbackSignal is the opposite direction: the stamp is NEWER than the
// running binary, so the deployed assets are ahead of what this binary embeds
// and `erg init` would REVERT them (ticket 0279). It has no cross-version
// consumer -- erg update only ever re-execs a strictly newer binary, which is by
// construction the assetDriftSignal direction -- so it is free to say what it
// means and to name its own remedy.
const assetRollbackSignal = "is older than the .erg-assets stamp (this binary predates the last init) -- run 'erg update' first, then 'erg init'"

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

// readManifest parses <root>/tickets/.erg-assets and returns a map of asset
// name -> stamped SHA-256 hex. A missing OR malformed manifest returns nil: the
// caller treats absence and corruption identically (fall back to known shipped
// hashes), so a bad stamp never fails init.
func readManifest(root string) map[string]string {
	return readManifestFile(filepath.Join(root, "tickets", manifestName))
}

// readManifestFile parses the manifest at the given path (the file itself, not
// its parent). Used directly by callers that already hold the tickets dir.
func readManifestFile(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parseManifest(data)
}

// parseManifest extracts the asset name -> SHA-256 map from manifest bytes.
// Returns nil when no asset line parses (so absence and corruption look alike).
func parseManifest(data []byte) map[string]string {
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

// parseManifestDate extracts the "date:" header line from manifest bytes -- the
// buildDate of the binary that last ran init here. Returns "" when the field is
// absent, which is what a manifest written by an erg predating the field looks
// like. Deliberately a SIBLING of parseManifest rather than an extension of it:
// parseManifest returns the asset map and has callers and tests of its own, and
// the date is needed by exactly two of them, so widening its return type would
// churn every call site to serve two.
func parseManifestDate(data []byte) string {
	const prefix = "date:"
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

// manifestDateFile reads the date: field of the manifest AT path (the file
// itself, not its parent). Sibling of readManifestFile, same absence contract:
// unreadable reads as absent.
func manifestDateFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return parseManifestDate(data)
}

// manifestDate reads the date: field of <root>/tickets/.erg-assets. Sibling of
// readManifest.
func manifestDate(root string) string {
	return manifestDateFile(filepath.Join(root, "tickets", manifestName))
}

// buildDateLayout is the exact fixed-width shape the Makefile stamps into the
// binary and buildManifest copies into .erg-assets: date -u +%Y-%m-%dT%H:%M:%SZ.
// Digits are written as '0' here; every other byte must match literally.
const buildDateLayout = "0000-00-00T00:00:00Z"

// looksLikeBuildDate reports whether s has exactly buildDateLayout's shape.
//
// This is the guard that makes lexical comparison legitimate rather than merely
// convenient: only same-shape, same-zone, fixed-width ISO-8601 sorts
// chronologically as a string. Anything else -- a date-only stamp, an offset
// instead of Z, a hand-edited manifest, the "unknown" literal, or the "y" that
// a test fixture happens to carry -- is not comparable, and a string compare
// against it would silently return an answer that means nothing. ("y" > any
// digit, so an unguarded compare reads every garbage stamp as "newer than the
// binary", i.e. as a rollback.) The ticket is explicit that an unparseable
// stamp must degrade to the prior behaviour, never to a refusal.
//
// So do not "simplify" isRollback back into a bare comparison. This guard is
// not defensive padding around the compare; it is the half of the compare that
// time.Parse would have supplied, and dropping it costs a whole capability
// silently -- no error, no warning, just a store whose assets can never be
// refreshed again. tests/test_check.sh's drift fixture stamps "date: y" and is
// the standing positive control for exactly that.
func looksLikeBuildDate(s string) bool {
	if len(s) != len(buildDateLayout) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if buildDateLayout[i] == '0' {
			if s[i] < '0' || s[i] > '9' {
				return false
			}
		} else if s[i] != buildDateLayout[i] {
			return false
		}
	}
	return true
}

// isRollback reports whether stampDate is strictly newer than runningDate: the
// binary running now PREDATES the one that last ran init here, so overwriting
// the deployed assets with this binary's embedded copies would be a revert, not
// a refresh (ticket 0279).
//
// Plain lexical comparison, no time.Parse, guarded by a shape check. Both sides
// are the same fixed-width ISO-8601 UTC stamp the Makefile writes, which sorts
// chronologically as a string; version.go's outdated-sibling check already
// orders two buildDates exactly this way. The shape check (looksLikeBuildDate)
// is what replaces the parse: it answers "is this comparable", which is the only
// question parsing would have answered here.
//
// Either side missing, "unknown" (what buildManifest writes for an unstamped
// build), or not of that shape means there is no provenance to compare -- and
// absent or unreadable provenance is NOT evidence of a rollback. Those cases
// return false, so every caller falls through to the pre-0279 behaviour. This
// is the invariant that keeps stores written by an erg predating the date:
// field working.
func isRollback(stampDate, runningDate string) bool {
	if !looksLikeBuildDate(stampDate) || !looksLikeBuildDate(runningDate) {
		return false
	}
	return stampDate > runningDate
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
//
// "Prior" is the load-bearing word, and until ticket 0279 nothing checked it:
// hash equality alone was read as a licence to overwrite, in either direction,
// so a binary OLDER than the stamp cheerfully reverted a newer asset and
// reported a refresh. stampDate/runningDate supply the direction (see
// isRollback); when the stamp is the newer side this is a rollback, not an
// upgrade, and the file is preserved instead (exit 2, overridable with --force).
//
// The direction check guards the STAMP branch only. The history branch below
// requires exact equality to a hash this binary already ships in its own
// knownAssetHashes table -- i.e. a version this binary knows it superseded --
// so it cannot express "newer than me" in the first place.
func isCleanUpgrade(diskHash, stampedHash string, known []string, stampDate, runningDate string) bool {
	if stampedHash != "" {
		if diskHash != stampedHash {
			return false
		}
		return !isRollback(stampDate, runningDate)
	}
	for _, h := range known {
		if diskHash == h {
			return true
		}
	}
	return false
}

// assetDriftWarnings reports assets whose .erg-assets stamp differs from the
// current binary's embedded version -- i.e. the binary was upgraded since the
// last init, so the deployed assets are behind and a re-init would refresh
// them. It REQUIRES a manifest: without one (readManifest returns nil) the
// comparison is impossible and we invent no fallback (charter 4c derisque), so
// a hand-maintained store that never ran the asset-managed init is never
// nagged. Comparing the stamp (not the on-disk bytes) means a deliberate local
// edit does not raise a drift warning; only a binary upgrade past the recorded
// stamp does.
//
// The hash comparison establishes THAT the two differ, never which is newer, so
// the message is chosen by the stamp's recorded date instead (ticket 0279): the
// "binary upgraded since last init" wording was previously printed even when the
// running binary was months OLDER than the stamp, advising an erg init that
// would have reverted the asset.
func assetDriftWarnings(dir string) []string {
	// dir is the ticket store itself (the dir holding .erg-assets), so read the
	// manifest file directly rather than via readManifest (which joins tickets/).
	stamps := readManifestFile(filepath.Join(dir, manifestName))
	if stamps == nil {
		return nil
	}
	// Which side is newer. Read once: the date is a property of the manifest,
	// not of any one asset in it.
	rollback := isRollback(manifestDateFile(filepath.Join(dir, manifestName)), buildDate)
	var warnings []string
	for _, rel := range initAssetPaths {
		name := strings.TrimPrefix(rel, "tickets/")
		stamp, ok := stamps[name]
		if !ok || stamp == "" {
			continue
		}
		content, ok := bootstrapAsset(rel)
		if !ok {
			continue
		}
		if stamp != sha256hex([]byte(content)) {
			signal := assetDriftSignal
			if rollback {
				signal = assetRollbackSignal
			}
			warnings = append(warnings, fmt.Sprintf("WARN %s: embedded version %s", name, signal))
		}
	}
	return warnings
}
