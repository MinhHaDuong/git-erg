package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// parseIDFromFilename extracts the leading numeric prefix from an .erg
// filename (e.g. "0042-some-title.erg" -> 42). Returns 0 if the file does not
// end in .erg, the prefix is not numeric, or the numeric ID is >= 10000 (outside
// the valid 4-digit range). Stray files with IDs >= 10000 are silently ignored
// so they do not poison next-id into returning a 5-digit result.
func parseIDFromFilename(name string) int {
	if !strings.HasSuffix(name, ".erg") {
		return 0
	}
	stem := strings.TrimSuffix(name, ".erg")
	if idx := strings.Index(stem, "-"); idx > 0 {
		stem = stem[:idx]
	}
	n, err := strconv.Atoi(stem)
	if err != nil || n >= 10000 {
		return 0
	}
	return n
}

// maxIDInDir walks dir (and subdirectories) and returns the largest numeric
// .erg filename prefix found, or 0 if dir does not exist or contains none.
// Used by nextID for the local pass and for each sibling worktree pass.
func maxIDInDir(dir string) int {
	maxID := 0
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if n := parseIDFromFilename(d.Name()); n > maxID {
			maxID = n
		}
		return nil
	})
	return maxID
}

// knownBranches returns the short names of every refs/heads/ branch and every
// refs/remotes/ tracking branch in the repository containing dir. Both come
// from the local refs cache via a single `git for-each-ref` call -- no
// network. Remote-tracking branches are included because they reflect what
// the user last fetched from origin; an ID burned on origin is taken, even
// if no local branch carries it. The origin/HEAD symref is skipped (it
// duplicates whichever branch HEAD points at). Returns nil on any git error.
func knownBranches(ctx context.Context, dir string) []string {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "for-each-ref",
		"--format=%(refname:short)", "refs/heads", "refs/remotes")
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var refs []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, "/HEAD") {
			continue
		}
		refs = append(refs, line)
	}
	return refs
}

// maxIDInRef returns the largest ticket ID found in <ref>'s tree under
// <relpath> via `git ls-tree -r -z --name-only`. The command runs with -C
// <repoTop> so the pathspec is anchored at the repo root (running it from a
// subdir would interpret <relpath> relative to cwd, missing the tree).
// core.quotePath=false makes non-ASCII filenames come back literal, and -z
// gives NUL-delimited output so embedded newlines are safe. Returns 0 on any
// git error.
func maxIDInRef(ctx context.Context, repoTop, ref, relpath string) int {
	cmd := exec.CommandContext(ctx, "git", "-C", repoTop,
		"-c", "core.quotePath=false",
		"ls-tree", "-r", "-z", "--name-only", ref, "--", relpath)
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	maxID := 0
	for _, name := range strings.Split(string(out), "\x00") {
		if name == "" {
			continue
		}
		if n := parseIDFromFilename(path.Base(name)); n > maxID {
			maxID = n
		}
	}
	return maxID
}

// branchScanDeadline bounds the Pass 3 (local-branch ls-tree) scan. On
// timeout, nextID falls back to the Pass 1+2 result with a stderr warning.
const branchScanDeadline = 200 * time.Millisecond

// errRangeExhausted is returned by nextID when all 4-digit IDs are used up.
var errRangeExhausted = errors.New("next-id: range exhausted (max 4-digit ID is 9999)")

// nextID returns the next available ticket ID as a zero-padded 4-digit string,
// or an error if the 4-digit range (0001-9999) is exhausted.
// The scan combines three sources:
//
//  1. The filesystem walk of dir itself.
//  2. The same-relative subdir of every sibling worktree (catches uncommitted
//     or unpushed tickets drafted in parallel agent worktrees).
//  3. The same-relative subtree of every refs/heads/ and refs/remotes/ tip
//     in the local refs cache (catches tickets committed on branches not
//     currently checked out anywhere, and IDs burned on origin that have
//     been fetched but not merged).
//
// Pass 1 always runs. Passes 2 and 3 run only when dir lies inside a git
// worktree; if dir is outside the repo or git is unavailable, they are
// silently skipped and behavior reduces to the original local-only scan.
// Pass 3 is bounded by branchScanDeadline; on timeout it falls back to the
// Pass 1+2 result and writes a single WARNING to stderr. No network calls:
// remote-tracking refs come from the local cache populated by the last fetch.
//
// Allocation is still optimistic: two concurrent invocations in different
// worktrees may return the same ID. The pre-commit hook on merge catches
// duplicates on the same branch; the cross-worktree window has narrowed but
// is not eliminated.
func nextID(dir string) (string, error) {
	maxID := maxIDInDir(dir)

	top := worktreeTopFor(dir)
	if top == "" {
		return formatNextID(maxID)
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return formatNextID(maxID)
	}
	rel, err := filepath.Rel(top, absDir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return formatNextID(maxID)
	}

	for _, wt := range loadGitWorktrees(dir) {
		if wt.path == top || wt.path == "" {
			continue
		}
		if n := maxIDInDir(filepath.Join(wt.path, rel)); n > maxID {
			maxID = n
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), branchScanDeadline)
	defer cancel()
	slashRel := filepath.ToSlash(rel)
	for _, ref := range knownBranches(ctx, top) {
		if ctx.Err() != nil {
			break
		}
		if n := maxIDInRef(ctx, top, ref, slashRel); n > maxID {
			maxID = n
		}
	}
	// Check after the loop so the warning fires whether the deadline tripped
	// during localBranches itself, during the last maxIDInRef call, or between
	// iterations. The Pass 1+2 result is preserved.
	if ctx.Err() != nil {
		fmt.Fprintln(os.Stderr,
			"WARNING: next-id branch-tip scan exceeded 200ms; result may miss IDs on un-checked-out branches")
	}

	return formatNextID(maxID)
}

// formatNextID returns the next ID string for maxID, or errRangeExhausted if
// maxID >= 9999 (meaning the next ID would be 10000 or more, violating the
// 4-digit NNNN-slug.erg filename contract).
func formatNextID(maxID int) (string, error) {
	if maxID >= 9999 {
		return "", errRangeExhausted
	}
	return fmt.Sprintf("%04d", maxID+1), nil
}

// summaryNextID is the one-liner printed by printUsage via the commands registry.
const summaryNextID = "Print the next available ticket ID"

const helpNextID = `## erg next-id [DIR]

Print the next available ticket ID.

Scans for the maximum ticket ID across three sources and returns max+1,
zero-padded to 4 digits. Prints "0001" if no numbered tickets exist anywhere.

  1. DIR (default: auto-discovered tickets/) and its subdirectories -- the
     local filesystem walk.
  2. The same-relative subdir of every sibling worktree, enumerated via
     'git worktree list'. Catches uncommitted tickets drafted in parallel
     agent worktrees of the same repository.
  3. The same-relative subtree of every refs/heads/ and refs/remotes/ tip
     in the local refs cache, enumerated via 'git for-each-ref' + 'git
     ls-tree'. Catches tickets committed on branches not currently checked
     out anywhere, and IDs already burned on origin that have been fetched
     but not yet merged locally. No network call -- remote-tracking refs
     come from the local cache populated by the last 'git fetch'. Bounded
     by a 200ms wall-clock deadline; on timeout, falls back to the Pass
     1+2 result and prints a WARNING to stderr.

When DIR is outside a git repository, or git is unavailable, behavior
reduces to the Pass 1 local walk alone.

Cache freshness: the remote-tracking scan is only as fresh as the last
'git fetch'. If parallel agents push tickets to origin between fetches,
their IDs are invisible to this scan and may be re-allocated. Run
'git fetch' before starting a parallel raid if you want the freshest
view; next-id itself never makes a network call.

ID allocation is still optimistic: two concurrent invocations in different
worktrees may return the same ID -- the cross-worktree window has narrowed
but is not eliminated. The pre-commit hook rejects duplicate IDs on merge;
the losing agent renames its ticket with a new ID from a fresh invocation.
`

// cmdNextID implements `erg next-id [dir]`. See helpNextID for the user-facing summary.
func cmdNextID(args []string) int {
	var ticketDir string
	if len(args) > 0 {
		ticketDir = args[0]
	} else {
		var err error
		ticketDir, err = findTicketsDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	id, err := nextID(ticketDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(id)
	return 0
}
