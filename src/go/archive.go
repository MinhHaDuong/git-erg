package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// errMoveCollision is returned by moveTicketToClosed when the destination
// under closed/ already exists. It is a distinct type so callers can choose a
// policy: archive skips (preserving its historical behaviour), close errors.
type errMoveCollision struct{ Dst string }

func (e *errMoveCollision) Error() string {
	return fmt.Sprintf("destination already exists: %s", e.Dst)
}

// syncDir best-effort fsyncs a directory so a rename into/out of it is durable
// across a crash (mirrors the parent-dir sync in atomicWriteFile). A platform
// or filesystem that cannot sync a directory must not fail the operation.
func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
}

// moveTicketToClosed moves ticket t into ticketDir/closed/, durably and
// confined to the store. It is the single move primitive shared by `archive`
// and `close`:
//
//   - Refuses a symlinked closed/ -- os.Rename would follow the symlinked
//     parent component and relocate the ticket OUTSIDE the store (silent data
//     loss); withinStore is re-asserted on the destination as a second rail.
//   - A pre-existing destination is an errMoveCollision, never a silent skip.
//   - fsyncs both parent directories so the rename survives a crash.
func moveTicketToClosed(ticketDir string, t *Erg) error {
	closedDir := filepath.Join(ticketDir, "closed")
	// Refuse a symlinked closed/: os.Rename follows the symlinked parent and
	// would move the ticket out of the store, which `erg check` then cannot see.
	if fi, err := os.Lstat(closedDir); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to move into %s: closed/ is a symlink (would escape the store)", closedDir)
	}
	if err := os.MkdirAll(closedDir, 0755); err != nil {
		return fmt.Errorf("cannot create %s: %w", closedDir, err)
	}
	dst := filepath.Join(closedDir, t.Filename())
	if ok, err := withinStore(ticketDir, dst); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("refusing to move %s: destination resolves outside the store", t.Filename())
	}
	if _, err := os.Stat(dst); err == nil {
		return &errMoveCollision{Dst: dst}
	}
	if err := os.Rename(t.Path, dst); err != nil {
		return fmt.Errorf("cannot move %s: %w", t.Filename(), err)
	}
	syncDir(filepath.Dir(t.Path))
	syncDir(closedDir)
	return nil
}

// summaryArchive is the one-liner printed by printUsage via the commands registry.
const summaryArchive = "Move closed tickets to tickets/closed/"

const helpArchive = `## erg archive [ID...] [DIR] [-n|--dry-run]

Move closed tickets to DIR/closed/.

With no IDs, scans only the direct children of DIR (default: tickets/) -- not subdirectories -- for tickets that
have a non-empty Closed: header and are not already inside a closed/ directory,
then moves each eligible ticket to DIR/closed/. With IDs given, archives only
the named tickets.

A ticket is skipped (with a SKIPPED message) if any open ticket in DIR still
has a Blocked-by: pointing to its ID; archiving would silently break that ref.
Run 'erg close ID REASON' (which removes Blocked-by refs automatically) before
archiving, or manually delete the stale Blocked-by line.

The command creates DIR/closed/ if it does not exist. It will not overwrite
an existing file at the destination.

With -n / --dry-run, archive renames nothing: it prints "WOULD ARCHIVE <file>"
for each eligible ticket and "WOULD SKIP <file> (needed by ...)" for tickets
held open by a Blocked-by ref, then exits 0. This is the read-only listing the
pre-push hook (erg install --push-hook) uses to warn about closed-but-
unarchived tickets without mutating the working tree.
`

// cmdArchive implements `erg archive [id...] [dir]`. See helpArchive for the user-facing summary.
func cmdArchive(args []string) int {
	var ids []string
	var explicit string
	dryRun := false

	for _, a := range args {
		switch {
		case a == "-n" || a == "--dry-run":
			dryRun = true
		case len(a) == 4 && allDigits(a):
			ids = append(ids, a)
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "archive: unknown flag %q\nUsage: erg archive [ID...] [DIR] [-n|--dry-run]\n", a)
			return 1
		default:
			explicit = a
		}
	}

	ticketDir, err := resolveDir(explicit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "archive: %v\n", err)
		return 1
	}

	// Load all tickets recursively (includes closed/) for building the
	// reverse blocker index.
	allTickets, _ := loadErgs(ticketDir)

	// Build reverse index: for each open ticket X, for each local Blocked-by
	// ref R in X, record that R.ID is blocking X.FilenameID().
	// blockedBy[closedTicketID] = list of open ticket IDs that mention it.
	blockedBy := make(map[string][]string)
	for i := range allTickets {
		t := &allTickets[i]
		if t.IsClosed() {
			continue
		}
		for _, ref := range t.BlockedBys {
			if ref.Kind != RefLocal {
				continue
			}
			blockedBy[ref.ID] = append(blockedBy[ref.ID], t.FilenameID())
		}
	}

	// Build path-keyed index from allTickets to avoid re-parsing.
	ticketByPath := make(map[string]*Erg, len(allTickets))
	for i := range allTickets {
		ticketByPath[allTickets[i].Path] = &allTickets[i]
	}

	// Collect target tickets -- reuse allTickets, no second parse.
	var targets []Erg
	exitCode := 0

	if len(ids) > 0 {
		// ID mode: resolve each ID to a file in the top-level ticketDir only.
		for _, id := range ids {
			path, err := resolveTicketByID(ticketDir, id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "archive: %v\n", err)
				exitCode = 1
				continue
			}
			if t, ok := ticketByPath[path]; ok {
				targets = append(targets, *t)
			}
		}
	} else {
		// Default mode: filter allTickets to top-level entries (not in
		// subdirectories like closed/).
		for i := range allTickets {
			if filepath.Dir(allTickets[i].Path) == ticketDir {
				targets = append(targets, allTickets[i])
			}
		}
	}

	for _, t := range targets {
		// Skip tickets without a non-empty Closed: header.
		// We use header-only check (not t.IsClosed()) to avoid re-processing
		// tickets that are already path-closed.
		if t.Closed == "" {
			continue
		}

		tid := t.FilenameID()

		// Skip if any open ticket is blocking on this one.
		if blockers := blockedBy[tid]; len(blockers) > 0 {
			verb := "SKIPPED"
			if dryRun {
				verb = "WOULD SKIP"
			}
			fmt.Printf("%s %s (needed by %s)\n", verb, t.Filename(), strings.Join(blockers, ", "))
			continue
		}

		// Dry-run: report what would be archived without touching the disk,
		// including a destination-collision preview so -n matches the real run.
		if dryRun {
			if _, err := os.Stat(filepath.Join(ticketDir, "closed", t.Filename())); err == nil {
				fmt.Printf("WOULD SKIP %s (destination exists)\n", t.Filename())
			} else {
				fmt.Printf("WOULD ARCHIVE %s\n", t.Filename())
			}
			continue
		}

		if err := moveTicketToClosed(ticketDir, &t); err != nil {
			// Preserve archive's historical skip-on-collision behaviour; any
			// other move failure (symlinked/non-dir closed/, rename error) is
			// a hard error.
			var col *errMoveCollision
			if errors.As(err, &col) {
				fmt.Fprintf(os.Stderr, "archive: destination already exists, skipping: %s\n", col.Dst)
				continue
			}
			fmt.Fprintf(os.Stderr, "archive: %v\n", err)
			exitCode = 1
			continue
		}

		fmt.Printf("ARCHIVED %s\n", t.Filename())
	}

	return exitCode
}
