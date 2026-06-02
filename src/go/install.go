package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const summaryInstall = "Wire up git hooks and agent instructions (opt-in)"

const helpInstall = `## erg install [DIR] [--hooks] [--push-hook] [--inject-agents] [--create-agents-md]

Wire up integration hooks and agent instructions for a project that already
has a ticket store (created by erg init).

By default -- with no flags -- install does nothing outside tickets/. Each
piece of wiring requires an explicit opt-in flag:

  --hooks              Install (or upgrade) a pre-commit hook that runs
                       erg validate and erg check on every commit and rejects
                       commits that modify the traveling binary (tickets/erg)
                       outside main. The erg block is delimited by sentinel
                       markers and is inserted right after the shebang so it
                       runs before any third-party hook content. Existing
                       content outside the markers is preserved.

  --push-hook          Install (or upgrade) a pre-push hook that WARNS about
                       tickets that are closed but not yet archived, printing
                       the exact archive+commit+push recipe. It mutates
                       nothing and never blocks the push -- a pre-push hook
                       cannot get a file move into the push it gates, and a
                       mutating hook would leave a dirty tree that git reset
                       could resurrect into a duplicate ticket. The real
                       archival stays at merge time and via manual erg archive.

  --inject-agents      Add a one-line pointer to tickets/AGENTS.md inside a
                       sentinel-marked block in the project-root AGENTS.md.
                       If the root AGENTS.md does not exist, the flag is
                       refused unless --create-agents-md is also given.

  --create-agents-md   Permit --inject-agents to create a root AGENTS.md when
                       none exists. On its own it does nothing.

All wiring flags default to off. install never overwrites content outside its
managed block; on rerun or upgrade it replaces only the region between the
markers. All preconditions are checked before any file is written, so a refused
run changes nothing on disk.

Requires tickets/erg (the binary) to already exist in the project, same as
erg init.

Exit codes: 0 success; 1 a hard error (bad flag, missing binary, not a git
repository, unbalanced markers, refused AGENTS.md creation, or a write
failure). See "Exit codes" in erg --help --all.
`

// Canonical (new) and legacy sentinel markers for the managed pre-commit block.
// markerSet[0] is the begin marker, markerSet[1] the end marker. The first set
// is canonical (what install writes); the rest are legacy formats recognized
// for in-place upgrade. Adding a new format here keeps old installs upgradable.
var hookMarkerSets = [][2]string{
	{"# >>> erg managed >>>", "# <<< erg managed <<<"},
	{"# --- git-erg: begin managed block ---", "# --- git-erg: end managed block ---"},
}

// AGENTS.md markers are HTML comments so they are inert in rendered markdown.
var agentsMarkerSets = [][2]string{
	{"<!-- >>> erg managed >>> -->", "<!-- <<< erg managed <<< -->"},
}

// hookBody is the canonical content placed between the hook markers. It runs
// erg validate + check on staged .erg files and rejects committing tickets/erg
// outside main. It deliberately does NOT run erg archive (charter blocker #2:
// archive in pre-commit corrupts the commit; autoarchive belongs in pre-push).
// It contains no `exit 0`, so when it is prepended into a shared hook control
// falls through to any third-party content after the block on success.
const hookBody = `# Reject tickets/erg commit on non-main branches.
# CI rebuilds the binary after merge; feature PRs must not include it.
if git diff --cached --name-only | grep -q '^tickets/erg$'; then
    branch=$(git branch --show-current)
    if [ "$branch" != "main" ]; then
        echo "pre-commit: do not commit tickets/erg in feature branches." >&2
        echo " CI rebuilds the binary after merge. To override: git commit --no-verify" >&2
        exit 1
    fi
fi

# Validate staged .erg files and the corpus.
erg_files=$(git diff --cached --name-only | grep '\.erg$' || true)
if [ -n "$erg_files" ]; then
    if [ -x tickets/erg ]; then
        # shellcheck disable=SC2086
        if ! tickets/erg validate $erg_files; then
            echo "ERROR: ticket validation failed." >&2
            exit 1
        fi
        if ! tickets/erg check tickets/; then
            echo "ERROR: ticket corpus check failed." >&2
            exit 1
        fi
    else
        echo "ERROR: tickets/erg not found. Run 'make build' first." >&2
        exit 1
    fi
fi`

// pushHookBody is the canonical content placed between the pre-push hook
// markers. It is WARN-ONLY: it lists closed-but-unarchived tickets (via the
// read-only erg archive --dry-run) and prints the archive+commit+push recipe,
// but it mutates nothing and always exits 0 (it must never block a push).
// A pre-push hook cannot get a file move into the push it gates, and a
// mutating hook would leave a dirty tree that git reset/stash could resurrect
// into a duplicate ticket -- so warn-only is the safe, faithful realization.
const pushHookBody = `# Warn about tickets that are closed but not yet archived.
# This hook never mutates the tree and never blocks the push (charter blocker
# #2 + design council 0209): archiving at push cannot enter the gated push and
# would risk a dirty-tree resurrection. Archive happens at merge time and via
# manual 'erg archive'.
if [ -x tickets/erg ]; then
    pending=$(tickets/erg archive --dry-run 2>/dev/null | grep '^WOULD ARCHIVE ' || true)
    if [ -n "$pending" ]; then
        echo "pre-push: closed tickets are not yet archived:" >&2
        echo "$pending" | sed 's/^WOULD ARCHIVE /  /' >&2
        echo "  run: tickets/erg archive && git commit -am 'archive closed tickets' && git push" >&2
    fi
fi
exit 0`

const agentsBody = "git-erg local tickets: see tickets/AGENTS.md"

// a planned write, computed during pre-flight and applied only if every
// precondition passed.
type writePlan struct {
	path    string
	content []byte
	perm    os.FileMode
	summary string
}

func cmdInstall(args []string) int {
	var positional []string
	hooks := false
	pushHook := false
	injectAgents := false
	createAgentsMd := false
	for _, a := range args {
		switch a {
		case "--hooks":
			hooks = true
		case "--push-hook":
			pushHook = true
		case "--inject-agents":
			injectAgents = true
		case "--create-agents-md":
			createAgentsMd = true
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintf(os.Stderr, "install: unknown flag %q\nUsage: erg install [DIR] [--hooks] [--push-hook] [--inject-agents] [--create-agents-md]\n", a)
				return 1
			}
			positional = append(positional, a)
		}
	}

	root := "."
	if len(positional) > 0 {
		root = positional[0]
	}

	// Precondition: the traveling binary must be present (parity with init).
	binaryPath := filepath.Join(root, "tickets", "erg")
	if _, err := os.Stat(binaryPath); err != nil {
		fmt.Fprintf(os.Stderr, "install: binary not found at %s\n", binaryPath)
		fmt.Fprintln(os.Stderr, "Place the erg binary in tickets/ before running install.")
		return 1
	}

	if !hooks && !pushHook && !injectAgents {
		fmt.Fprintln(os.Stderr, "install: nothing to do -- pass --hooks, --push-hook, and/or --inject-agents")
		return 0
	}

	// ---- Pre-flight: compute every planned write; mutate nothing yet. ----
	var plans []writePlan
	var perrs []string

	if hooks {
		if plan, err := planHook(root); err != nil {
			perrs = append(perrs, err.Error())
		} else {
			plans = append(plans, plan)
		}
	}

	if pushHook {
		if plan, err := planPushHook(root); err != nil {
			perrs = append(perrs, err.Error())
		} else {
			plans = append(plans, plan)
		}
	}

	if injectAgents {
		if plan, err := planAgents(root, createAgentsMd); err != nil {
			perrs = append(perrs, err.Error())
		} else {
			plans = append(plans, plan)
		}
	}

	if len(perrs) > 0 {
		for _, e := range perrs {
			fmt.Fprintf(os.Stderr, "install: %s\n", e)
		}
		fmt.Fprintln(os.Stderr, "install: nothing was written (preconditions failed).")
		return 1
	}

	// ---- Apply: write each plan atomically; report loudly. ----
	rc := 0
	for _, p := range plans {
		if err := atomicWriteFile(p.path, p.content, p.perm); err != nil {
			fmt.Fprintf(os.Stderr, "install: write failed for %s: %v\n", p.path, err)
			rc = 1
			continue
		}
		fmt.Fprintln(os.Stderr, p.summary)
	}
	return rc
}

// planHook resolves the hooks directory (via git, so worktrees and
// core.hooksPath are honoured), reads any existing pre-commit, and computes the
// new content with the managed block inserted right after the shebang. It never
// writes; it returns an error if the markers in the existing hook are
// unbalanced (which would make an in-place edit ambiguous).
func planHook(root string) (writePlan, error) {
	hooksDir, err := resolveHooksDir(root)
	if err != nil {
		return writePlan{}, err
	}
	hookPath := filepath.Join(hooksDir, "pre-commit")

	existing, perm, isNew, err := readHookFile(hookPath)
	if err != nil {
		return writePlan{}, fmt.Errorf("cannot read %s: %v", hookPath, err)
	}

	var lines []string
	if isNew {
		lines = []string{"#!/bin/sh"}
	} else {
		lines = splitLinesNoTrailingEmpty(existing)
	}

	cleaned, err := stripManagedRegions(lines, hookMarkerSets)
	if err != nil {
		return writePlan{}, fmt.Errorf("%s: %v -- fix or remove the stray marker manually", hookPath, err)
	}

	content := insertManagedBlockAfterShebang(cleaned, hookMarkerSets[0], hookBody)

	action := "managed block updated"
	if isNew {
		action = "created"
	}
	summary := fmt.Sprintf("install: pre-commit hook %s at %s (executable, runs on every commit; bypass with git commit --no-verify; uninstall: delete the lines between the erg managed markers)", action, hookPath)

	return writePlan{path: hookPath, content: []byte(content), perm: perm, summary: summary}, nil
}

// planPushHook computes the pre-push hook content with the warn-only managed
// block inserted after the shebang. Same machinery as planHook; the block is
// non-mutating and never blocks the push.
func planPushHook(root string) (writePlan, error) {
	hooksDir, err := resolveHooksDir(root)
	if err != nil {
		return writePlan{}, err
	}
	hookPath := filepath.Join(hooksDir, "pre-push")

	existing, perm, isNew, err := readHookFile(hookPath)
	if err != nil {
		return writePlan{}, fmt.Errorf("cannot read %s: %v", hookPath, err)
	}

	var lines []string
	if isNew {
		lines = []string{"#!/bin/sh"}
	} else {
		lines = splitLinesNoTrailingEmpty(existing)
	}

	cleaned, err := stripManagedRegions(lines, hookMarkerSets)
	if err != nil {
		return writePlan{}, fmt.Errorf("%s: %v -- fix or remove the stray marker manually", hookPath, err)
	}

	content := insertManagedBlockAfterShebang(cleaned, hookMarkerSets[0], pushHookBody)

	action := "managed block updated"
	if isNew {
		action = "created"
	}
	summary := fmt.Sprintf("install: pre-push hook %s at %s (warn-only, never blocks the push; uninstall: delete the lines between the erg managed markers)", action, hookPath)

	return writePlan{path: hookPath, content: []byte(content), perm: perm, summary: summary}, nil
}

// planAgents computes the new root AGENTS.md content with the pointer line in a
// managed block. It refuses (error) when AGENTS.md is absent and createAgentsMd
// is false -- creating a root file the user did not ask for is a surprise.
func planAgents(root string, createAgentsMd bool) (writePlan, error) {
	agentsPath := filepath.Join(root, "AGENTS.md")
	existing, isNew, err := readTextFile(agentsPath)
	if err != nil {
		return writePlan{}, fmt.Errorf("cannot read %s: %v", agentsPath, err)
	}
	if isNew && !createAgentsMd {
		return writePlan{}, fmt.Errorf("%s not found -- pass --create-agents-md to create it", agentsPath)
	}

	var lines []string
	if !isNew {
		lines = splitLinesNoTrailingEmpty(existing)
	}
	cleaned, err := stripManagedRegions(lines, agentsMarkerSets)
	if err != nil {
		return writePlan{}, fmt.Errorf("%s: %v -- fix or remove the stray marker manually", agentsPath, err)
	}
	content := appendManagedBlock(cleaned, agentsMarkerSets[0], agentsBody)

	action := "managed block updated in"
	if isNew {
		action = "created"
	}
	summary := fmt.Sprintf("install: AGENTS.md %s %s (now user-owned; uninstall: delete the lines between the erg managed markers)", action, agentsPath)

	return writePlan{path: agentsPath, content: []byte(content), perm: 0644, summary: summary}, nil
}

// resolveHooksDir returns the absolute hooks directory for the repository at
// root. It shells out to git rev-parse --git-path hooks, which correctly
// handles plain repos, linked worktrees (returns the common .git/hooks), and
// core.hooksPath -- none of which a hand-rolled .git parse gets right.
func resolveHooksDir(root string) (string, error) {
	cmd := exec.Command("git", "-C", root, "rev-parse", "--git-path", "hooks")
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository at %s (cannot install hooks)", root)
	}
	p := strings.TrimSpace(string(out))
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	if mkErr := os.MkdirAll(p, 0755); mkErr != nil {
		return "", fmt.Errorf("cannot create hooks directory %s: %v", p, mkErr)
	}
	return p, nil
}

// readHookFile returns the existing hook content, the permission to write with
// (existing mode with the exec bits forced on, or 0755 for a new file), and
// whether the file is new. A non-IsNotExist error is surfaced so install never
// clobbers an existing-but-unreadable hook.
func readHookFile(path string) (content string, perm os.FileMode, isNew bool, err error) {
	info, serr := os.Stat(path)
	if serr != nil {
		if os.IsNotExist(serr) {
			return "", 0755, true, nil
		}
		return "", 0, false, serr
	}
	data, rerr := os.ReadFile(path)
	if rerr != nil {
		return "", 0, false, rerr
	}
	// Force the exec bits: git silently skips a non-executable hook, so a
	// 0644 third-party hook we append to would never run our checks.
	return string(data), info.Mode().Perm() | 0111, false, nil
}

// readTextFile returns file content and whether it is new. Only IsNotExist is
// treated as "new"; other errors are surfaced.
func readTextFile(path string) (content string, isNew bool, err error) {
	data, rerr := os.ReadFile(path)
	if rerr != nil {
		if os.IsNotExist(rerr) {
			return "", true, nil
		}
		return "", false, rerr
	}
	return string(data), false, nil
}

// splitLinesNoTrailingEmpty splits on \n and drops a single trailing empty
// element (the artifact of a final newline), so callers can rejoin with a
// single trailing newline without fusing lines.
func splitLinesNoTrailingEmpty(s string) []string {
	lines := strings.Split(s, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}

// trimMarker normalises a line for marker comparison: strips a trailing CR
// (CRLF files) and surrounding horizontal whitespace.
func trimMarker(s string) string {
	return strings.TrimRight(strings.TrimRight(s, "\r"), " \t")
}

func isBeginMarker(line string, sets [][2]string) bool {
	t := trimMarker(line)
	for _, m := range sets {
		if t == m[0] {
			return true
		}
	}
	return false
}

func isEndMarker(line string, sets [][2]string) bool {
	t := trimMarker(line)
	for _, m := range sets {
		if t == m[1] {
			return true
		}
	}
	return false
}

// stripManagedRegions removes every managed region (across all marker sets)
// from lines, pairing each BEGIN with the nearest following END. It refuses
// (error) on unbalanced markers -- a BEGIN with no following END, an END before
// any BEGIN, or a nested BEGIN before the current region closes -- rather than
// risk deleting un-managed content between mismatched markers.
func stripManagedRegions(lines []string, sets [][2]string) ([]string, error) {
	var out []string
	i := 0
	for i < len(lines) {
		switch {
		case isBeginMarker(lines[i], sets):
			j := i + 1
			for j < len(lines) {
				if isBeginMarker(lines[j], sets) {
					return nil, fmt.Errorf("nested erg managed BEGIN marker before the previous block closed")
				}
				if isEndMarker(lines[j], sets) {
					break
				}
				j++
			}
			if j >= len(lines) {
				return nil, fmt.Errorf("erg managed BEGIN marker without a matching END")
			}
			i = j + 1 // skip [i..j] inclusive
		case isEndMarker(lines[i], sets):
			return nil, fmt.Errorf("erg managed END marker before any BEGIN")
		default:
			out = append(out, lines[i])
			i++
		}
	}
	return out, nil
}

// insertManagedBlockAfterShebang inserts the canonical managed block right
// after a leading shebang (or at the very top if there is none), so the block
// runs before any third-party hook content. Returns content with a single
// trailing newline.
func insertManagedBlockAfterShebang(lines []string, marker [2]string, body string) string {
	block := []string{marker[0]}
	block = append(block, strings.Split(body, "\n")...)
	block = append(block, marker[1])

	var out []string
	rest := lines
	if len(lines) > 0 && strings.HasPrefix(lines[0], "#!") {
		out = append(out, lines[0])
		rest = lines[1:]
	}
	out = append(out, block...)
	if len(rest) > 0 {
		// One blank separator before existing content. Guard against an
		// already-blank boundary so reruns stay byte-stable: stripManagedRegions
		// leaves the separator that preceded a removed block, and adding another
		// unconditionally would grow a blank line on every install.
		if rest[0] != "" {
			out = append(out, "")
		}
		out = append(out, rest...)
	}
	return strings.Join(out, "\n") + "\n"
}

// appendManagedBlock appends the canonical managed block after existing content
// (used for AGENTS.md, where order does not matter). Returns content with a
// single trailing newline.
func appendManagedBlock(lines []string, marker [2]string, body string) string {
	block := []string{marker[0]}
	block = append(block, strings.Split(body, "\n")...)
	block = append(block, marker[1])

	var out []string
	out = append(out, lines...)
	// One blank separator after existing content. Guard against an
	// already-blank boundary so reruns stay byte-stable (see the matching
	// note in insertManagedBlockAfterShebang).
	if len(lines) > 0 && lines[len(lines)-1] != "" {
		out = append(out, "")
	}
	out = append(out, block...)
	return strings.Join(out, "\n") + "\n"
}
