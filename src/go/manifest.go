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

// assetStamplessSignal is the third condition: no .erg-assets entry stamps this
// asset -- either the store has no manifest at all, or the manifest it has says
// nothing about this file -- yet the asset on disk differs from what this binary
// embeds (tickets 0283, 0292). Without a stamp there is no recorded provenance,
// so the difference is unattributable -- it may be a clean upgrade the store
// never stamped, or a deliberate local edit -- and the only honest report is to
// say so and name the command that finds out. Like assetDriftSignal it is a
// cross-version contract: erg update greps the re-exec'd NEW binary's
// `erg check` output for this literal (see update.go), so producer and consumer
// share it and it may be extended at the END only, never rewritten.
//
// The tail after "--" is that extension, and its shape is forced. Two claims in
// the original text were wrong, and both had to be corrected from the END
// rather than in place:
//
//   - "stamps it as if shipped" described the defect ticket 0292 fixed. init
//     now carries a preserved file's prior stamp forward and adds none where
//     there was none, so the condition survives the run instead of being
//     certified away.
//   - "its git history can" is not true in every store erg accepts. A plain
//     directory under no version control takes `erg check` fine and has no
//     history at all; a repository where the diverged asset is untracked has
//     none for that file. The route that works in every store is the one 0292
//     added: `erg init --show NAME` prints the embedded copy on stdout, so
//     diff and sha256sum can answer the question the note poses.
//
// Why not simply rewrite the sentence, which would be half the length: the
// literal was already shipped when this correction was written. PR #348's
// executor ran `strings` against the committed bootstrap binary and found the
// constant present, so every store that has run `erg update` since carries an
// OLD binary that greps for the OLD prefix. Reword the front and that old side
// stops recognising its own signal -- silently, with no error anywhere, which
// is the exact failure this whole family of constants exists to prevent. The
// window to reword closed at the commit that rebuilt the binary, and nothing in
// the code announced it closing (see vendoredDriftSignal, which records the
// same lesson pointed at what makes a literal safe: the absence of a grepping
// consumer, never the absence of a release).
//
// Note what it still does NOT say. No direction or version: establishing those
// would need a table of historically shipped hashes, which is ticket 0281's
// territory and closed wontfix -- this branch compares only against the single
// currently embedded copy.
const assetStamplessSignal = "no .erg-assets stamp -- cannot tell whether this is a clean upgrade or a local edit; its git history can, and 'erg init' preserves the file either way but stamps it as if shipped -- CORRECTED since erg 0292: init no longer stamps a file it preserved, and that git history exists only where the store is version-controlled and this file tracked; 'erg init --show NAME' prints the embedded copy in any store, for diff or sha256sum"

// vendoredDriftSignal is the fourth condition, and the only one about a file
// erg does not own: a VENDORED file (vendoredAssetPaths -- today just
// tickets/erg-github) whose bytes on disk differ from the copy this binary
// embeds (ticket 0282). Vendored means the adopter owns it outright: it was
// committed into their repo as a plain file, they may have wired it into CI,
// and erg has never written it and never will. So this reports, and stops.
//
// Note what it does NOT say, and why each omission is forced:
//
// No claim that the on-disk copy is OLDER. The comparison establishes only
// that the two differ; the adopter may equally have customised theirs. Naming
// it "stale" would be a direction this compare cannot support -- the same
// discipline assetStamplessSignal follows for the unstamped managed asset.
//
// No stamp-relative claim, in either direction. The .erg-assets manifest
// records what init WROTE, and init never wrote this file, so the stamp is
// silent about it by construction: this compare is disk-against-embedded and
// ignores the manifest entirely, which is also why it reads identically in a
// stamped and an unstamped store.
//
// No "run 'erg init'". That remedy is true for the managed assets and false
// here -- init does not touch a vendored path, so advising it would send an
// adopter to a command that reports success and changes nothing. The route
// named instead is the one that actually exists: re-vendor the file by hand,
// documented in README's forge-layer section.
//
// Unlike assetDriftSignal and assetStamplessSignal this literal has no
// cross-version consumer -- erg update greps for those two, not for this one
// (see update.go) -- so, like assetRollbackSignal, it is free to be reworded
// later. If a future erg update ever greps it, that freedom ends.
//
// Note precisely what the freedom rests on: the ABSENCE OF A GREPPING
// CONSUMER, not the literal being unreleased. Ticket 0292 recorded how that
// second reading fails -- assetStamplessSignal was safe to reword right up to
// the commit that rebuilt the committed binary, and nothing in the code
// announced the window closing. A consumer is visible in the source; a release
// boundary is not.
const vendoredDriftSignal = "differs from the erg-github this binary ships -- it is vendored, so erg never writes it; if the difference is not your own customisation, re-vendor it by hand (README, 'Forge layer: erg-github')"

// sha256hex returns the hex-encoded SHA-256 of b.
func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// looksLikeAssetHash reports whether s has the shape sha256hex produces: 64
// lowercase hex digits. It answers "is this stamp usable as evidence", which is
// a different question from parseManifest's "is this line syntactically an
// entry" -- and keeping them different is deliberate, since parseManifest's
// looser rule is what makes a hand-written or truncated manifest parse at all
// rather than fail init.
//
// Two call sites need the stricter question, and they need the SAME answer
// (ticket 0292, PR #360 round 1). buildManifest carries a preserved asset's
// prior stamp forward, and installAssets reads a stamp's presence as the
// evidence for "has local edits". Before this predicate, a stamp that was
// non-empty but not a hash -- a manifest truncated mid-line, or edited by hand
// -- was carried forward verbatim FOREVER and made every subsequent run assert
// an edit nothing had observed: the exact false-reason class this ticket exists
// to remove, reached through a different door. The pre-0292 code self-healed
// that by accident, because it restamped every entry with the embedded hash on
// every run; dropping that overwrite is what made a validation step necessary.
//
// Anything that fails here is treated as no stamp at all, which is the honest
// reading and matches what parseManifest already does with an empty hash.
func looksLikeAssetHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// buildManifest returns the deterministic provenance manifest content for the
// embedded assets in initAssetPaths. The hashes are of the EMBEDDED content
// (what this binary ships), so the manifest is the reference a clean init
// would produce. Same binary + same embedded assets => byte-identical output.
// rev/date come from the build stamp (version.go), so the manifest is stable
// for a given binary (not wall-clock dependent).
//
// carry names the assets installAssets PRESERVED on this run, mapped to what
// the PREVIOUS manifest recorded for each ("" when it recorded nothing). Pass
// nil when every asset was installed. The manifest records what init wrote, and
// a preserved file is precisely what init did not write, so stamping it with
// the embedded hash would certify a customised file as byte-identical to the
// shipped default -- silencing both the drift report and the stampless report
// for it, permanently, with the divergence still on disk (ticket 0292,
// defect 1). That is 0279's rollback exemption generalised: don't stamp what
// you didn't touch.
//
// A prior entry is carried forward verbatim rather than dropped, and the
// difference is not cosmetic. That entry is evidence about a PAST install: it
// is what lets the next run say "has local edits" and mean it. Dropping it
// would swap a false record for a lost one, and the run after a re-init would
// downgrade its own verdict to "provenance unrecorded" on a store whose
// provenance was in fact recorded.
func buildManifest(carry map[string]string) (string, error) {
	type entry struct{ name, sum string }
	var entries []entry
	for _, rel := range initAssetPaths {
		name := strings.TrimPrefix(rel, "tickets/")
		if prev, preserved := carry[name]; preserved {
			// A stamp that is not a hash is not evidence, and carrying it
			// forward would preserve it past every future run. Dropping it
			// restores the self-healing the pre-0292 restamp gave for free.
			if !looksLikeAssetHash(prev) {
				continue
			}
			entries = append(entries, entry{name: name, sum: prev})
			continue
		}
		content, ok := bootstrapAsset(rel)
		if !ok {
			return "", fmt.Errorf("missing embedded asset: %s", rel)
		}
		entries = append(entries, entry{
			name: name,
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
// it prints a preview line and writes nothing. carry is buildManifest's
// preserved-asset map; see there.
func writeManifest(root string, dryRun bool, carry map[string]string) error {
	content, err := buildManifest(carry)
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

// readManifestDateFile reads the date: field of the manifest AT path (the file
// itself, not its parent). Sibling of readManifestFile, same absence contract:
// unreadable reads as absent.
func readManifestDateFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return parseManifestDate(data)
}

// readManifestDate reads the date: field of <root>/tickets/.erg-assets. Sibling of
// readManifest.
func readManifestDate(root string) string {
	return readManifestDateFile(filepath.Join(root, "tickets", manifestName))
}

// buildDateLayout is the exact fixed-width shape the Makefile stamps into the
// binary and buildManifest copies into .erg-assets: date -u +%Y-%m-%dT%H:%M:%SZ.
// Digits are written as '0' here; every other byte must match literally.
const buildDateLayout = "0000-00-00T00:00:00Z"

// looksLikeBuildDate reports whether s has exactly buildDateLayout's shape AND
// denotes a possible calendar instant.
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
	// Byte shape alone is not enough. "9999-99-99T99:99:99Z" has the right
	// shape, and sorts above every real stamp, so a shape-only guard hands the
	// lexical compare a value that means nothing and it answers "rollback" --
	// freezing the store exactly as an unguarded compare would. Range-check the
	// groups so an impossible calendar value lands in the same not-comparable
	// bucket as "y": no direction, fall through to pre-0279 behaviour.
	//
	// Ranges only -- no month-length or leap-year arithmetic. Lexical ordering
	// is unaffected by whether February had 30 days, and a calendar library is
	// exactly the dependency this compare avoids. Second 60 is admitted: it is
	// a legal leap second the stamper can emit.
	return inRange(s[5:7], 1, 12) && // month
		inRange(s[8:10], 1, 31) && // day
		inRange(s[11:13], 0, 23) && // hour
		inRange(s[14:16], 0, 59) && // minute
		inRange(s[17:19], 0, 60) // second (60 = leap second)
}

// inRange reports whether the two-digit group g, known to be digits already,
// denotes a value within [lo, hi].
func inRange(g string, lo, hi int) bool {
	v := int(g[0]-'0')*10 + int(g[1]-'0')
	return v >= lo && v <= hi
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

// managedAssetWarnings is assetDriftWarnings' managed-asset half: it reports
// assets erg INSTALLS whose .erg-assets stamp differs from the
// current binary's embedded version -- i.e. the binary was upgraded since the
// last init, so the deployed assets are behind and a re-init would refresh
// them. Comparing the stamp (not the on-disk bytes) means a deliberate local
// edit does not raise a drift warning; only a binary upgrade past the recorded
// stamp does.
//
// With no manifest it hands off to stamplessWarnings instead of returning nil
// (ticket 0283). "No stamp" used to be read as "nothing to compare", and that
// reading is what left a store with no provenance silent on every channel:
// there IS something to compare, the on-disk bytes against the embedded copy.
// What the absence of a stamp really costs is the ability to say WHICH side
// moved, so the stampless report claims no direction.
//
// Be precise about what survives of the derisque property, because it is
// narrower than "never nagged": a store with no managed asset on disk at all is
// still never nagged, and so is one whose assets are byte-identical to what
// this binary ships. A store that CUSTOMISED .ergrc and never stamped it now
// gets a NOTE on every erg check -- and customising .ergrc is the documented,
// encouraged case (AGENTS.md sends readers there to define Label: values), not
// an anomaly. That population is the deliberate cost of the fix: its silence
// was the bug. The gate is real divergence, not the absence of a manifest.
//
// The hash comparison establishes THAT the two differ, never which is newer, so
// the message is chosen by the stamp's recorded date instead (ticket 0279): the
// "binary upgraded since last init" wording was previously printed even when the
// running binary was months OLDER than the stamp, advising an erg init that
// would have reverted the asset.
// Two populations, compared differently, reported together (ticket 0282).
// managedAssetWarnings covers what erg INSTALLS, where the stamp is meaningful
// and the remedy is a command. vendoredDriftWarnings covers what erg only
// SHIPS A REFERENCE FOR, where there is no stamp and no command.
//
// The append order matters exactly once: the vendored notes must be appended
// OUTSIDE the managed branch, not inside it. The managed side returns early
// when there is no manifest, and a store with no .erg-assets is precisely the
// long-unmaintained adopter most likely to be carrying a year-old erg-github --
// so an implementation that folded the vendored compare into either branch
// would go silent for the population the fix exists to serve. The test's noisy
// arm runs over both stamped and stampless stores for that reason.
func assetDriftWarnings(dir string) []string {
	return append(managedAssetWarnings(dir), vendoredDriftWarnings(dir)...)
}

// vendoredDriftWarnings reports each file in vendoredAssetPaths whose bytes on
// disk differ from the copy this binary embeds. It is the staleness channel
// tickets/erg-github never had: not being in any asset list, it was invisible
// to the stamp, to isCleanUpgrade and to the whole 3-state compare, so a
// year-old vendored copy produced no signal on any channel (ticket 0282).
//
// Deliberately NOT a fourth state of the dpkg compare. That machinery exists to
// decide whether erg may overwrite a file; here the answer is fixed at "no" --
// the adopter owns this file, may have wired it into CI, and gets a report
// rather than a write. Reusing the compare's plumbing (the embedded blob, the
// byte equality) without reusing its authority is the whole shape of the fix.
//
// Silent in the two cases that carry no information, and the second one is an
// invariant rather than an optimisation:
//
//   - The bytes match what this binary ships: nothing to say.
//   - The file is absent: the repo never adopted the forge layer. erg core is
//     forge-blind and erg-github is optional infrastructure, so absence is a
//     legitimate steady state, not a gap to nag about. A report here would
//     wedge every non-forge adopter into an adoption they declined.
//
// Unreadable folds into absent, following readManifestFile, installAssets and
// stamplessWarnings -- breaking ranks in one function would be the surprise.
//
// No manifest is read and none is written. Adding these paths to
// initAssetPaths would have been the smaller diff and the wrong one: that list
// drives installAssets (a write) and buildManifest (a stamp recording an
// install that never happened, ticket 0292's defect).
//
// The compare sees CONTENT and nothing else, which has two consequences a
// reader should know rather than rediscover (PR #353 review):
//
// It cannot see the executable bit. A copy with the right bytes and mode 0644
// is silent here, yet fails in CI with "permission denied". Reporting mode
// would be a second signal about a different property, on a bit that several
// filesystems do not carry, and no positive control was available for those --
// so this function stays about content, and the README recipe keeps its
// chmod +x. Widening it later is a deliberate choice, not a patch.
//
// It reports a CRLF checkout as drift, and that is correct rather than a false
// positive: erg-github runs under /bin/sh, and a CRLF copy is broken there.
// How it breaks depends on how far the conversion reached. A whole-file
// conversion corrupts the shebang too, so the kernel never finds the
// interpreter and nothing runs. Leave the shebang intact and CRLF only the
// body and it does start, reads twenty lines of comment, then dies on
// "set -eu\r". Either way the copy differs from the one that works, which is
// what the note claims -- the earlier wording said no CRLF copy executes at
// all, which is true only of the first case.
//
// There is deliberately no acknowledge path -- no key, no stamp, nothing that
// silences the note for a copy the adopter customised on purpose. Silencing
// would mean recording a per-store fact about a file erg does not own, which
// is the ownership claim this whole design declines to make; the note says
// "not your own customisation?" precisely so a customiser can read it once and
// move on. If that proves too noisy in practice, the fix is a decision about
// what erg may record, not a flag bolted on here.
func vendoredDriftWarnings(dir string) []string {
	var notes []string
	for _, rel := range vendoredAssetPaths {
		name := strings.TrimPrefix(rel, "tickets/")
		shipped, ok := bootstrapAsset(rel)
		if !ok {
			// This binary ships no reference copy, so there is nothing to
			// compare against. Silence is the only honest answer: reporting
			// would be asserting drift from a blob that does not exist.
			continue
		}
		onDisk, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if string(onDisk) == shipped {
			continue
		}
		notes = append(notes, fmt.Sprintf("NOTE %s: %s", name, vendoredDriftSignal))
	}
	return notes
}

func managedAssetWarnings(dir string) []string {
	// dir is the ticket store itself (the dir holding .erg-assets), so read the
	// manifest file directly rather than via readManifest (which joins tickets/).
	stamps := readManifestFile(filepath.Join(dir, manifestName))
	if stamps == nil {
		return stamplessWarnings(dir)
	}
	// Which side is newer. Read once: the date is a property of the manifest,
	// not of any one asset in it.
	rollback := isRollback(readManifestDateFile(filepath.Join(dir, manifestName)), buildDate)
	var warnings []string
	for _, rel := range initAssetPaths {
		name := strings.TrimPrefix(rel, "tickets/")
		// looksLikeAssetHash, not `!= ""`: this is the THIRD reader of a stamp
		// as evidence, after buildManifest's carry branch and installAssets'
		// preserve reason, and it has to give the same answer as both (PR #360
		// round 2 -- round 1 wired the predicate into two call sites and a
		// comment claimed that was all of them). On a manifest holding a
		// non-hash stamp, the looser gate sent this loop into the stamped
		// branch, where the stamp cannot equal the embedded hash, and `erg
		// check` then printed a confident directional "binary upgraded since
		// last init -- run 'erg init' to refresh" about a file `erg init`
		// refuses to attribute or refresh. Two commands, one store,
		// contradicting each other, on a claim neither could support.
		stamp := stamps[name]
		if !looksLikeAssetHash(stamp) {
			// The manifest stamps nothing usable for THIS asset, so there is
			// no stamp-relative claim to make about it -- and the stampless
			// compare is exactly the one that applies. Skipping
			// here instead reproduced 0283's silence one level down: not
			// drift-warned (no stamp for it), not stampless-warned (the
			// manifest is not nil, and parseManifest returns non-nil as soon
			// as ONE line parses). Ticket 0292, defect 3.
			//
			// The gate is per ASSET, never per store. This is also the shape
			// init itself now writes: a run that preserved one asset stamps
			// the others and leaves that one unstamped, so a partial manifest
			// is the normal product of the fix above, not a corruption.
			//
			// Known and unfixable from here (PR #360 round 1): an OLDER erg
			// reading such a manifest is SILENT about the unstamped asset,
			// because its own copy of this loop still has the bare `continue`.
			// That is the pre-0292 gate in the old binary, not a new defect,
			// but this change makes partial manifests routine, so a mixed
			// fleet loses the warning until every binary is updated. Nothing
			// in a new binary can repair an old one's gate; the only lever is
			// erg update, which the stampless report already names.
			if note, has := stamplessNote(dir, rel); has {
				warnings = append(warnings, note)
			}
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

// stamplessWarnings is managedAssetWarnings' no-manifest branch (ticket 0283).
// For each managed asset it compares the bytes on disk against the bytes this
// binary embeds -- the SAME bootstrapAsset lookup the stamped branch uses, and
// deliberately only against the single CURRENTLY embedded copy. Matching a
// table of historically shipped hashes would let such a store auto-upgrade;
// that is ticket 0281, closed wontfix, and a future edit adding a multi-rev
// lookup here is the one thing this function must not grow.
//
// The per-asset decision lives in stamplessNote, which the stamped branch also
// calls for an asset its manifest does not stamp; what stays here is only the
// whole-store sweep.
func stamplessWarnings(dir string) []string {
	var notes []string
	for _, rel := range initAssetPaths {
		if note, has := stamplessNote(dir, rel); has {
			notes = append(notes, note)
		}
	}
	return notes
}

// stamplessNote is the per-asset half of stamplessWarnings, split out because
// managedAssetWarnings needs exactly this decision for an asset its manifest
// does not stamp (ticket 0292, defect 3). One function, so the two callers
// cannot drift into disagreeing about what "unstamped and diverged" means --
// a comment saying they agree would be a weaker guarantee than one expression.
//
// Reports (note, true) only for a real, unattributable divergence. Silent in
// both directions that carry no information: an asset that is absent (the store
// never adopted erg's asset management) and one that matches the embedded copy
// exactly (nothing to report). It reports a condition rather than prescribing
// an overwrite -- hence NOTE, not WARN. Nothing here changes what erg init does
// about the file; this makes the condition visible, no more.
func stamplessNote(dir, rel string) (string, bool) {
	name := strings.TrimPrefix(rel, "tickets/")
	content, ok := bootstrapAsset(rel)
	if !ok {
		return "", false
	}
	onDisk, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		// Absent, or present but unreadable. The two are not the same
		// thing, and folding them together means a chmod-000 asset with
		// real divergence stays quiet -- in a function whose whole subject
		// is not staying quiet. It is nonetheless the convention every
		// sibling here already follows (readManifestFile, installAssets'
		// `exists := readErr == nil`), so the fold is deliberate: breaking
		// ranks in one function would be the surprising move.
		return "", false
	}
	// Direct comparison, not sha256 of both sides: this is an equality
	// test, both operands are already in memory, and the sibling that asks
	// the same question next door (installAssets) spells it this way.
	if string(onDisk) == content {
		return "", false
	}
	return fmt.Sprintf("NOTE %s: %s", name, assetStamplessSignal), true
}
