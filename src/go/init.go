package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var initAssetPaths = []string{
	"tickets/.ergrc",
	"tickets/AGENTS.md",
}

// migrateAssetPaths is the subset of assets that `erg migrate` refreshes during
// its layout sweep. It deliberately excludes tickets/.ergrc: configuration
// delivery is `erg init`'s job (the dpkg 3-state compare that preserves local
// edits). AGENTS.md keeps the charter's force-overwrite behaviour here because
// agent operating instructions must track the binary (ticket 0224).
var migrateAssetPaths = []string{
	"tickets/AGENTS.md",
}

// vendoredAssetPaths lists files that erg SHIPS a reference copy of but never
// writes: they are vendored into an adopter's repo as plain committed files the
// adopter owns outright (ticket 0282). Today that is the forge helper
// tickets/erg-github, which README calls out as travelling with the clone.
//
// This list is deliberately NOT reachable from any install path. Every writer
// (installAssets via initAssetPaths / migrateAssetPaths, the orphan sweep via
// orphanAssetPaths) is driven by one of the other lists; this one is consumed
// by exactly one read-only caller, manifest.go's vendoredDriftWarnings, which
// compares and reports. The reason is not caution about a fragile file: an
// adopter may have wired their copy into CI, and a binary that silently
// replaced it would be claiming an ownership the vendoring contract gives the
// adopter. It is also why these paths stay out of initAssetPaths, whose
// members buildManifest stamps -- stamping a file init never wrote would record
// an installation that did not happen (the defect ticket 0292 tracks for the
// locally-edited case).
//
// The forge layer stays optional: a repo with no erg-github at all has nothing
// to compare and is never reported. erg core remains offline and forge-blind --
// this is a byte comparison against an embedded blob, not a use of the helper.
var vendoredAssetPaths = []string{
	"tickets/erg-github",
}

// showableAssetPaths is every embedded file `erg init --show NAME` can print:
// the managed assets init writes, plus the vendored reference copy it only
// ships. Derived from the two lists rather than spelled out again, so a new
// asset becomes showable by being added where it belongs.
//
// The union is deliberate even though the two populations are governed
// differently. --show answers one question -- "what does this binary ship at
// this path" -- and that question is exactly as answerable, and exactly as
// useful, for a vendored file the adopter must re-vendor by hand as for a
// managed one erg would overwrite. Excluding the vendored copy would leave
// README's re-vendor recipe with no offline source (ticket 0292, defect 4).
func showableAssetPaths() []string {
	out := make([]string, 0, len(initAssetPaths)+len(vendoredAssetPaths))
	out = append(out, initAssetPaths...)
	out = append(out, vendoredAssetPaths...)
	return out
}

// orphanAssetPaths lists assets that older erg versions deposited during init
// but are now served on demand via erg spec / erg integration. If a file at
// one of these paths matches the current embedded content exactly, init
// removes it as an orphan.
var orphanAssetPaths = []string{
	"tickets/spec-erg-v1.md",
	"tickets/integration.md",
}

const summaryInit = "Unpack .ergrc and AGENTS.md into tickets/"

const helpInit = `## erg init [DIR] [-n|--dry-run] [--force] [--show NAME]

Unpack embedded bootstrap assets into the project.

Writes two files relative to DIR (default: current directory):

  - tickets/.ergrc -- project configuration (label vocabulary, update remote).
  - tickets/AGENTS.md -- agent operating instructions for the ticket workflow.

It also writes tickets/.erg-assets, a provenance manifest recording this
binary's rev/date and the SHA-256 of each embedded asset. The manifest is
committable durable state (not gitignored) and is invisible to erg check, so
it never trips the pre-commit hook. It is deterministic: the same binary and
assets always produce byte-identical content.

The format specification and setup guide are available on demand via
erg spec and erg integration respectively.

Requires tickets/erg (the binary) to already exist in the project; the command
refuses if it is absent. This requirement ensures that agents do not accidentally
initialize an empty directory that was never meant to be a ticket store.

Each asset is compared against the embedded version with a dpkg-style 3-state
rule. Byte-identical files are left unchanged. A differing file that still
matches the .erg-assets stamp (or, with no stamp, a known shipped hash) is a
clean upgrade -- erg never touched it, so it is overwritten and a
"git restore -- <path>" hint is printed. A differing file that matches neither
is a local edit: it is preserved and the command exits 2 (local edits are never
overwritten without --force).

A preserved file is not stamped. The manifest records what init INSTALLED, so
an asset init declined to touch keeps whatever the previous manifest said about
it and gains no new entry -- stamping it with the embedded hash would certify a
customised file as identical to the shipped default and silence every later
report about it. An asset with no entry is compared against the embedded copy
directly, and erg check says so without claiming a direction.

When no stamp attests the difference, init says exactly that rather than
"local edits": with no recorded provenance, nothing distinguishes your own edit
from an upgrade the store never stamped, and --show is how you find out.

The stamp also records which binary wrote it, and init compares that date with
its own. If this binary is the OLDER one -- an erg from before the last init --
then refreshing would revert the deployed assets, not upgrade them. Such a file
is preserved too, and init says so and points at 'erg update' rather than
claiming a local edit. A stamp with no date (written by an erg predating the
field) carries no direction and is treated exactly as before. This is what makes
'erg update && erg init' a pair the code enforces and not merely a convention.

Flags:

  -n, --dry-run   Preview what init would create, refresh, skip, or leave
                  unchanged without writing or removing any file.
  --force         Overwrite files that differ from the embedded version
                  instead of skipping them. Use with care: local edits are
                  replaced. On a rollback (the .erg-assets stamp is newer than
                  this binary) a forced overwrite of a file still matching that
                  stamp is reported as "downgraded", not "refreshed": nothing
                  there was locally edited, the file is being reverted to an
                  older release. Run 'erg update' first if that is not what you
                  want.
  --show NAME     Print this binary's embedded copy of NAME on stdout and exit,
                  writing nothing. NAME is .ergrc, AGENTS.md or erg-github
                  (with or without the "tickets/" prefix). The output is byte-
                  identical to what the asset compare uses, so it pipes into
                  diff or sha256sum -- which is how you answer, for yourself,
                  the question every asset report poses: does my copy differ
                  from the shipped one because I edited it, or because the
                  binary moved on?

                    erg init --show .ergrc | diff - tickets/.ergrc

                  Needs no project and no tickets/ directory.

If tickets/spec-erg-v1.md or tickets/integration.md exist from a previous init
and match the current embedded content, they are removed as orphaned assets.
Files that have been edited locally are preserved.

After a successful run (not in dry-run), init chains a read-only corpus check
and prints any warnings, but its exit code reflects the init outcome only --
the chained warnings never change it.

Canonical keep-current sequence: 'erg update && erg init'. erg update replaces the
binary; erg init delivers embedded-asset changes and refreshes the default label
vocabulary. The default vocabulary is frozen-by-copy into .ergrc at init time -- a
new default added later to the binary is shadowed by the existing file and never takes
effect until erg init overwrites the file (clean upgrade) or the user opts in with
--force (local edit). erg update alone cannot un-shadow a frozen vocabulary.

Exit codes: 0 success; 1 a hard error (bad flag, missing binary, write
failure); 2 a file was preserved and skipped -- either it has local edits, or
it is newer than this binary (run with --force to overwrite). See "Exit codes"
in erg --help --all.
`

// installAssets unpacks the embedded bootstrap assets under root, returning
// counts of how many files were newly created, refreshed (overwritten with
// different content), skipped (differed from embedded but refuseDiverged was
// set), or left unchanged (byte-identical to the embedded copy). Shared by
// `erg init` and by `erg migrate`'s layout sweep. The caller supplies the exact
// asset list via paths: `erg init` passes initAssetPaths (both assets); `erg
// migrate` passes migrateAssetPaths (AGENTS.md only -- .ergrc is configuration,
// delivered by init, ticket 0224). Error messages are unwrapped -- no "init:"
// prefix -- so each caller can label them with its own command name.
//
// When refuseDiverged is true, files that differ from the embedded asset go
// through the dpkg 3-state compare: a clean upgrade (on-disk matches the
// .erg-assets stamp, or a known shipped hash when no stamp) is overwritten
// (refresh); a local edit (matches neither) is preserved with a message on
// stderr and counted as skipped. When false (erg init --force, erg migrate),
// differing files are overwritten unconditionally (refresh).
//
// A stamp match is only an upgrade when the stamp is the OLDER side. When the
// running binary predates the stamp, overwriting would revert the deployed
// asset, so it is preserved instead and counted as skipped (ticket 0279) --
// and the provenance manifest is left as it was, since this run changed
// nothing it would be describing. Under --force the revert is performed as
// asked, but reported as "downgraded", never as "refreshed".
//
// When dryRun is true, no directory is created, no file is written, and the
// orphan sweep is not performed; instead a preview line is printed for each
// asset describing the action that would be taken. The returned counts are the
// same as a real run would produce.
func installAssets(root string, paths []string, refuseDiverged, dryRun bool) (created, refreshed, skipped, unchanged int, err error) {
	// The .erg-assets stamp from a previous init (nil if absent or malformed):
	// name -> recorded SHA-256. Read once; the dpkg compare consults it per asset.
	stamps := readManifest(root)
	// Which binary is newer, this one or the one that last ran init here. A
	// property of the manifest, so read once, not per asset. rollback is false
	// whenever the provenance is missing or unparseable -- see isRollback
	// (ticket 0279).
	stampDate := readManifestDate(root)
	rollback := isRollback(stampDate, buildDate)
	// Set when an asset was preserved BECAUSE of the rollback, which suppresses
	// the provenance rewrite at the end of the run (see there).
	rollbackPreserved := false
	// Every asset this run PRESERVED, mapped to what the previous manifest
	// recorded for it ("" when it recorded nothing). Handed to writeManifest so
	// the manifest never claims to have installed a file it declined to touch
	// (ticket 0292, defect 1); see buildManifest for why a prior entry is
	// carried rather than dropped.
	preserved := map[string]string{}
	for _, rel := range paths {
		content, ok := bootstrapAsset(rel)
		if !ok {
			return created, refreshed, skipped, unchanged, fmt.Errorf("missing embedded asset: %s", rel)
		}
		name := strings.TrimPrefix(rel, "tickets/")
		target := filepath.Join(root, filepath.FromSlash(rel))
		existing, readErr := os.ReadFile(target)
		exists := readErr == nil

		// Row 1: on-disk byte-identical to embedded -> nothing to do.
		// Loud per-file output names this skip outcome too (criterion 5:
		// each action prints its file + action), matching refresh/preserve.
		if exists && string(existing) == content {
			unchanged++
			if dryRun {
				fmt.Printf("  unchanged  %s\n", rel)
			} else {
				fmt.Fprintf(os.Stderr, "init: %s unchanged\n", rel)
			}
			continue
		}

		// Divergent (or absent). Decide overwrite vs preserve.
		// - !refuseDiverged (erg init --force, erg migrate): overwrite
		//   unconditionally -- exempt from the dpkg prompt.
		// - refuseDiverged (erg init default): dpkg 3-state compare. A clean
		//   upgrade (on-disk == stamp, or a known shipped hash when no stamp)
		//   is overwritten silently; a local edit is preserved (exit 2). An
		//   on-disk file matching a stamp NEWER than this binary is a rollback:
		//   also preserved, for a different reason and with its own message.
		preserve := false
		// Distinguishes the two reasons to preserve: a local edit (on-disk
		// matches nothing known) from a rollback (on-disk matches a stamp this
		// binary predates). Both preserve; they do not say the same thing.
		preserveRollback := false
		// Computed for every existing file, on BOTH legs. Scoping it inside the
		// refuseDiverged branch left the --force / migrate leg with no per-file
		// evidence at all, which is how the downgrade label below came to be
		// asserted for files no stamp ever ordered.
		diskHash := ""
		if exists {
			diskHash = sha256hex(existing)
		}
		// The one state in which overwriting this asset would be a genuine
		// version rollback: it is byte-identical to what the stamp records,
		// and the stamp was written by a binary newer than this one. An
		// ordinary local edit, or an asset with no stamp entry, has no
		// established ordering. Computed once because both the preserve
		// branch and the downgrade label below need exactly this fact, and a
		// comment is a weaker guarantee that they agree than one expression.
		stampedByNewer := rollback && stamps[name] != "" && diskHash == stamps[name]

		if exists && refuseDiverged {
			if !isCleanUpgrade(diskHash, stamps[name], knownAssetHashes(rel), stampDate, buildDate) {
				preserve = true
				preserveRollback = stampedByNewer
			}
		}

		if preserve {
			skipped++
			preserved[name] = stamps[name]
			// Name the actual reason, and only a reason this run OBSERVED.
			// Three states, not two:
			//
			//   - a stamp exists for this asset and the bytes differ from it:
			//     the file is not what init last wrote, so "local edits" is a
			//     verdict the stamp supports;
			//   - the stamp is NEWER than this binary: nothing was edited here
			//     at all, and the remedy is a different command (ticket 0279);
			//   - no stamp for this asset: the compare established only that
			//     the bytes differ from what this binary ships. Calling that a
			//     local edit is an attribution nothing here observed -- the
			//     same false-reason class 0279 fixed on the rollback leg, and
			//     precisely the attribution assetStamplessSignal exists to
			//     refuse (ticket 0292, defect 2).
			//
			// This per-file line is also how `erg init` and `erg init -n`
			// report a stampless store's condition at all: the chained corpus
			// check below cannot, because a preserved asset means skipped > 0
			// and the run returns 2 several lines above it.
			reason := "has local edits -- preserving (run with --force to overwrite)"
			short := "local edits"
			if stamps[name] == "" {
				reason = "differs from the copy this binary ships and has no .erg-assets stamp -- preserving; nothing here records whether that is your edit or an unstamped upgrade (run 'erg init --show " + name + "' to see the shipped copy, --force to overwrite)"
				short = "differs, no stamp, reason unknown"
			}
			if preserveRollback {
				rollbackPreserved = true
				reason = "is newer than this binary -- preserving (run 'erg update' first, then 'erg init')"
				short = "newer than this binary"
			}
			if dryRun {
				fmt.Printf("  would preserve (%s)  %s\n", short, rel)
			} else {
				fmt.Fprintf(os.Stderr, "init: %s %s\n", rel, reason)
			}
			continue
		}

		if exists {
			refreshed++
		} else {
			created++
		}
		// A --force overwrite while the stamp is newer is a deliberate
		// downgrade. It is still performed -- --force means what it says -- but
		// the log must not call a revert a refresh (ticket 0279, defect 3).
		//
		// "This store is a rollback" is a property of the MANIFEST; "this file
		// is being reverted" is a property of the FILE. Only a file whose bytes
		// on disk match the newer stamp is demonstrably an older-for-newer
		// swap. A file with an ordinary local edit, or with no stamp entry at
		// all, has no established version ordering -- calling its overwrite a
		// downgrade asserts a history never observed, which is the same defect
		// this label exists to fix, pointed the other way.
		downgrade := exists && stampedByNewer
		if dryRun {
			verb := "would create "
			if exists {
				verb = "would refresh"
			}
			if downgrade {
				verb = "would downgrade"
			}
			fmt.Printf("  %s  %s\n", verb, rel)
			continue
		}
		if mkErr := os.MkdirAll(filepath.Dir(target), 0755); mkErr != nil {
			return created, refreshed, skipped, unchanged, fmt.Errorf("cannot create directory for %s: %w", rel, mkErr)
		}
		if wErr := os.WriteFile(target, []byte(content), 0644); wErr != nil {
			return created, refreshed, skipped, unchanged, fmt.Errorf("cannot write %s: %w", rel, wErr)
		}
		// Loud output: name each overwrite and give a reversibility hint, so a
		// refresh (even a safe clean upgrade) is never silent -- and so a
		// revert is never narrated as a refresh.
		if exists {
			verb := "refreshed"
			if downgrade {
				verb = "downgraded"
			}
			fmt.Fprintf(os.Stderr, "init: %s %s (git restore -- %s to undo)\n", verb, rel, rel)
		}
	}
	// Record provenance (ticket 0210): a deterministic manifest of the embedded
	// asset hashes for this binary. Written by both erg init and erg migrate
	// (the two callers of installAssets). Skipped in dry-run.
	//
	// Not written AT ALL when an asset was preserved because this binary
	// predates the stamp (ticket 0279): the run declined to touch the deployed
	// assets, so stamping them with this older binary's rev/date would record a
	// state that never happened -- and would destroy the very evidence that
	// said so, making the next run misreport the same rollback as a local edit.
	// The whole file is skipped here, not just the one entry, because the
	// evidence at stake is the manifest's own date: header, which is a property
	// of the file and not of any asset in it.
	//
	// For every OTHER preserved file the same rule applies per entry, which is
	// what `preserved` carries (ticket 0292, defect 1): init records what it
	// installed and stays silent about what it declined to touch, rather than
	// stamping a customised file with the embedded hash and certifying it as
	// shipped. That was the state corruption -- it silenced the drift report
	// and the stampless report for that file permanently, and following the
	// advice `erg check` printed is what triggered it.
	if rollbackPreserved {
		return created, refreshed, skipped, unchanged, nil
	}
	if err := writeManifest(root, dryRun, preserved); err != nil {
		return created, refreshed, skipped, unchanged, fmt.Errorf("cannot write provenance manifest: %w", err)
	}
	return created, refreshed, skipped, unchanged, nil
}

// cmdInit implements `erg init [dir] [-n|--dry-run] [--force]`. See helpInit
// for the user-facing summary. Exit codes: 0 success; 1 hard error; 2 a file
// was preserved and skipped, having either local edits or a stamp newer than
// this binary.
func cmdInit(args []string) int {
	var positional []string
	dryRun := false
	force := false
	show := ""
	showAsked := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-n" || a == "--dry-run":
			dryRun = true
		case a == "--force":
			force = true
		case a == "--show":
			// The asset name is the NEXT token, and consuming it here is what
			// keeps it out of positional: `erg init --show .ergrc` must never
			// be read as an init of ./.ergrc, whose only symptom would be a
			// "binary not found" that looks like an unrelated environment
			// problem.
			showAsked = true
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "init: --show needs an asset name -- this binary ships: %s\n", strings.Join(showableAssetNames(), ", "))
				return 1
			}
			i++
			show = args[i]
		case strings.HasPrefix(a, "--show="):
			showAsked = true
			show = strings.TrimPrefix(a, "--show=")
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "init: unknown flag %q\nUsage: erg init [DIR] [-n|--dry-run] [--force] [--show NAME]\n", a)
			return 1
		default:
			positional = append(positional, a)
		}
	}

	// --show is a read-only dump, handled before every other check: it does not
	// need a project, a tickets/ directory or the erg binary in place, and
	// refusing it for a missing store would withhold the one thing that answers
	// "what does this binary ship" from a reader who has no store yet.
	if showAsked {
		return showEmbeddedAsset(show)
	}
	root := "."
	if len(positional) > 0 {
		root = positional[0]
	}

	binaryPath := filepath.Join(root, "tickets", "erg")
	if _, err := os.Stat(binaryPath); err != nil {
		fmt.Fprintf(os.Stderr, "init: binary not found at %s\n", binaryPath)
		fmt.Fprintln(os.Stderr, "Place the erg binary in tickets/ before running init.")
		return 1
	}

	// --force overwrites divergent files; without it, they are preserved.
	refuseDiverged := !force
	created, refreshed, skipped, unchanged, err := installAssets(root, initAssetPaths, refuseDiverged, dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init: %v\n", err)
		return 1
	}

	cleanOrphanAssets(root, dryRun)

	if dryRun {
		// "preserved", not "local edits": a file is also skipped when it is
		// newer than this binary, and the per-file lines above already gave
		// each skip its own reason (ticket 0279).
		fmt.Printf("init (dry-run): %d to create, %d to refresh, %d to skip (preserved), %d unchanged\n", created, refreshed, skipped, unchanged)
		if skipped > 0 {
			return 2
		}
		return 0
	}

	fmt.Printf("init: %d created, %d refreshed, %d skipped (preserved), %d unchanged\n", created, refreshed, skipped, unchanged)
	if skipped > 0 {
		return 2
	}

	// Chain a read-only corpus check: print warnings and folder-closure
	// hints, but the exit code reflects init only -- none change it.
	ticketsDir := filepath.Join(root, "tickets")
	chained, _ := loadErgs(ticketsDir)
	for _, e := range folderClosure(chained) {
		fmt.Fprintln(os.Stderr, e)
	}
	for _, w := range corpusWarnings(chained, ticketsDir) {
		fmt.Fprintln(os.Stderr, w)
	}

	fmt.Println("Next: erg install --hooks to set up pre-commit and pre-push hooks.")
	return 0
}

// showableAssetNames is showableAssetPaths as the user spells them: the bare
// file names, in list order, for the help text and the error messages.
func showableAssetNames() []string {
	var names []string
	for _, rel := range showableAssetPaths() {
		names = append(names, strings.TrimPrefix(rel, "tickets/"))
	}
	return names
}

// showEmbeddedAsset prints the embedded copy of the named asset on stdout and
// nothing else, so the output pipes into diff, sha256sum or a file. It is the
// answer to a question erg could not previously answer about itself (ticket
// 0292, defect 4): every asset report -- the stampless NOTE, the vendored NOTE,
// init's own preserve line -- tells a reader their file differs from the copy
// this binary ships, and no subcommand could show them that copy. `erg init -n`
// reports only THAT a file differs; erg spec and erg integration dump different
// embedded files entirely. The only route was a second store, a copy of the
// binary, an init and a manual diff.
//
// It is the erg-side answer that lets those messages stop pointing at the
// store's version-control history, which a directory under no version control
// does not have at all, and an untracked asset does not have for that file.
//
// Both spellings of the name are accepted -- ".ergrc" and "tickets/.ergrc" --
// because the messages that send a reader here print the bare name while the
// asset lists spell the path.
func showEmbeddedAsset(name string) int {
	want := strings.TrimPrefix(name, "tickets/")
	for _, rel := range showableAssetPaths() {
		if strings.TrimPrefix(rel, "tickets/") != want {
			continue
		}
		content, ok := bootstrapAsset(rel)
		if !ok {
			// Unreachable while the lists and the embed set agree; reported
			// rather than ignored so a future asset added to one and not the
			// other fails loudly instead of printing nothing and exiting 0.
			fmt.Fprintf(os.Stderr, "init: this binary ships no copy of %s\n", want)
			return 1
		}
		// Bare content, no banner and no added newline: the contract is byte
		// identity with what the compare uses, and a cosmetic trailer would
		// break every checksum while looking fine on screen.
		fmt.Print(content)
		return 0
	}
	fmt.Fprintf(os.Stderr, "init: unknown asset %q -- this binary ships: %s\n", name, strings.Join(showableAssetNames(), ", "))
	return 1
}

// cleanOrphanAssets removes assets that older erg versions deposited but are
// now served on demand (erg spec / erg integration), only when they match the
// current embedded content. Divergent files (possible user data) are always
// preserved. In dryRun mode it prints what it would remove without removing.
func cleanOrphanAssets(root string, dryRun bool) {
	for _, rel := range orphanAssetPaths {
		embedded, ok := bootstrapAsset(rel)
		if !ok {
			continue
		}
		target := filepath.Join(root, filepath.FromSlash(rel))
		existing, err := os.ReadFile(target)
		if err != nil {
			continue
		}
		if string(existing) == embedded {
			if dryRun {
				fmt.Printf("  would remove orphaned asset %s (now: erg %s)\n", rel, commandForOrphan(rel))
			} else {
				os.Remove(target)
				fmt.Fprintf(os.Stderr, "init: removed orphaned asset %s (now: erg %s)\n", rel, commandForOrphan(rel))
			}
		}
	}
}

func commandForOrphan(rel string) string {
	if strings.Contains(rel, "spec") {
		return "spec"
	}
	return "integration"
}
