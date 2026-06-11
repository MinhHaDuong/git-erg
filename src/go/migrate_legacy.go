package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// migrate_legacy.go implements ticket 0243: one-time cleanup of artifacts that
// early (pre spec 0.1) erg init bundles vendored into host repos, plus a
// repo-wide sweep for stale binary-path references. All entry points are
// called from migrateLayout, share its contract (idempotent, never commits,
// loud one-line notice per artifact), and are byte-level no-ops on a clean
// tree.

// legacySkillNames are the five skill directories the old bundle vendored
// into host repos under .claude/skills/. They duplicated `erg <verb>` in
// prose and drifted (described %erg v1 with a Status: header).
var legacySkillNames = []string{
	"ticket-new", "ticket-claim", "ticket-close", "ticket-ready", "ticket-release",
}

// legacySkillTelltales identify the vendored content. Those bundles predate
// provenance markers, so deletion keys on content: a skill dir is removed
// only when at least one of its files carries one of these substrings.
// Substrings (not hashes) tolerate minor local edits; a user-authored skill
// that merely reuses a ticket-* name carries none of them and is left alone.
var legacySkillTelltales = []string{"%erg v1", "tickets/tools/go"}

// legacyClaudeMDTelltales mark a stale managed CLAUDE.md block: claims from
// the old bundle that the 0.1 toolchain has since invalidated.
var legacyClaudeMDTelltales = []string{"no CLI needed", "tickets/tools/go", "%erg v1"}

// claudeMDBody is the refreshed content written between the managed markers
// when a stale block is found. It states the current contract: CLI verbs,
// %erg 0.1 format, docs in tickets/AGENTS.md.
const claudeMDBody = `git-erg tickets live in tickets/ (%erg 0.1 format).
Use the CLI: erg <verb> (binary at tickets/erg). See tickets/AGENTS.md.`

// legacyRefRewrites are the stale-reference rewrites applied by the repo-wide
// sweep, in order. The binary-path rewrite runs first so the resulting
// "tickets/erg validate tickets/" is then caught by the verb rewrite.
// The $erg_bin forms mirror migrateHook's pre-commit patterns so the sweep
// subsumes them in any other file (Makefiles, CI, scripts).
var legacyRefRewrites = [][2]string{
	{"tickets/tools/go/erg", "tickets/erg"},
	{"erg validate tickets/", "erg check tickets/"},
	{`"$erg_bin" validate tickets/`, `"$erg_bin" check tickets/`},
	{`$erg_bin validate tickets/`, `$erg_bin check tickets/`},
}

// pruneLegacySkills removes previously-vendored .claude/skills/ticket-* dirs
// from the project root. Content-gated: a dir is deleted only when one of its
// files contains a legacy tell-tale; anything else under the same name is
// user content and survives. Emits one notice per pruned dir. Idempotent.
func pruneLegacySkills(root string) {
	skillsDir := filepath.Join(root, ".claude", "skills")
	for _, name := range legacySkillNames {
		dir := filepath.Join(skillsDir, name)
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		if !dirContainsTelltale(dir, legacySkillTelltales) {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintf(os.Stderr, "migrate: remove %s: %v\n", dir, err)
			continue
		}
		fmt.Printf("migrate: removed legacy vendored skill %s\n", dir)
	}
}

// dirContainsTelltale reports whether any regular file under dir contains one
// of the tell-tale substrings. Read errors are treated as "no match" -- when
// in doubt, do not delete.
func dirContainsTelltale(dir string, telltales []string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || found {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		for _, t := range telltales {
			if bytes.Contains(data, []byte(t)) {
				found = true
				return nil
			}
		}
		return nil
	})
	return found
}

// refreshLegacyClaudeBlock rewrites a stale managed git-erg block in the root
// CLAUDE.md. Detection is two-stage: the block must be delimited by the
// legacy `# --- git-erg ...` (or canonical) markers AND its interior must
// carry a stale claim ("no CLI needed", tickets/tools/go/, %erg v1). A block
// without stale claims, or content outside the markers, is left byte-
// identical. Unbalanced markers refuse loudly (same policy as install).
// An absent CLAUDE.md is a silent no-op -- migrate never creates the file.
func refreshLegacyClaudeBlock(root string) {
	path := filepath.Join(root, "CLAUDE.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := splitLinesNoTrailingEmpty(string(data))

	begin, end := findLegacyClaudeBlock(lines)
	if begin < 0 {
		return
	}
	if end < 0 {
		fmt.Fprintf(os.Stderr, "migrate: %s: git-erg managed BEGIN marker without a matching END -- fix or remove the stray marker manually\n", path)
		return
	}
	interior := strings.Join(lines[begin+1:end], "\n")
	if !containsAny(interior, legacyClaudeMDTelltales) {
		return
	}

	block := []string{hookMarkerSets[0][0]}
	block = append(block, strings.Split(claudeMDBody, "\n")...)
	block = append(block, hookMarkerSets[0][1])

	out := make([]string, 0, len(lines))
	out = append(out, lines[:begin]...)
	out = append(out, block...)
	out = append(out, lines[end+1:]...)

	perm := os.FileMode(0644)
	if info, serr := os.Stat(path); serr == nil {
		perm = info.Mode().Perm()
	}
	if werr := os.WriteFile(path, []byte(strings.Join(out, "\n")+"\n"), perm); werr != nil {
		fmt.Fprintf(os.Stderr, "migrate: rewrite %s: %v\n", path, werr)
		return
	}
	fmt.Printf("migrate: refreshed stale git-erg block in %s\n", path)
}

// findLegacyClaudeBlock locates the first managed git-erg block. Begin/end
// are recognised in both the canonical install markers and the loose legacy
// `# --- git-erg ---` family (any line starting with "# --- git-erg").
// Returns (-1, -1) when no begin marker exists, (begin, -1) when unbalanced.
func findLegacyClaudeBlock(lines []string) (int, int) {
	isLegacy := func(line string) bool {
		return strings.HasPrefix(trimMarker(strings.TrimLeft(line, " \t")), "# --- git-erg")
	}
	begin := -1
	for i, line := range lines {
		if begin < 0 {
			if isBeginMarker(line, hookMarkerSets) || isLegacy(line) {
				begin = i
			}
			continue
		}
		if isEndMarker(line, hookMarkerSets) || isLegacy(line) {
			return begin, i
		}
	}
	return begin, -1
}

// containsAny reports whether s contains any of the given substrings.
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// sweepLegacyRefsMaxSize caps the files the sweep will read. The stale
// references live in hand-written config (hooks, settings, CI, Makefiles);
// anything bigger than this is build output or data, not wiring.
const sweepLegacyRefsMaxSize = 2 << 20 // 2 MiB

// sweepLegacyRefs walks the entire work tree and rewrites stale legacy
// references (tickets/tools/go/erg paths, `erg validate tickets/`
// invocations) wherever they appear: git hooks, .claude/settings.json, CI
// workflows of any forge, Makefiles, scripts. Pattern-driven, so it needs no
// per-file list. The walk is a plain filesystem walk, NOT git ls-files:
// hook-definition files like .claude/settings.local.json are routinely
// gitignored, and a tracked-only walk would miss exactly the files this
// sweep exists for. The .git directory is skipped except for its hooks
// (resolved via git, so worktrees and core.hooksPath are honoured).
//
// Exemptions: the ticket store itself (ticket text may legitimately quote old
// paths as history), binary files (NUL byte heuristic), non-regular files
// (symlinks etc.), and files over sweepLegacyRefsMaxSize. Files without a
// match are left byte-identical.
func sweepLegacyRefs(root, ticketDirName string) {
	storeDir := filepath.Join(root, ticketDirName)
	gitDir := filepath.Join(root, ".git")
	seen := make(map[string]bool)
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path == storeDir || path == gitDir {
				return filepath.SkipDir
			}
			return nil
		}
		if rel, rerr := filepath.Rel(root, path); rerr == nil {
			seen[rel] = true
			sweepFile(root, rel)
		}
		return nil
	})
	// Hooks live under the (skipped) .git dir -- or elsewhere entirely with
	// worktrees and core.hooksPath -- so they get their own pass.
	for _, rel := range hooksDirFiles(root) {
		if !seen[rel] {
			sweepFile(root, rel)
		}
	}
}

// sweepFile applies the legacy-reference rewrites to a single file (path
// relative to root), preserving its mode and reporting one notice naming the
// patterns rewritten. No-match, unreadable, binary, and non-regular files are
// all silent no-ops.
func sweepFile(root, rel string) {
	path := filepath.Join(root, rel)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > sweepLegacyRefsMaxSize {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil || bytes.IndexByte(data, 0) >= 0 {
		return
	}
	updated := string(data)
	var applied []string
	for _, rw := range legacyRefRewrites {
		if strings.Contains(updated, rw[0]) {
			updated = strings.ReplaceAll(updated, rw[0], rw[1])
			applied = append(applied, rw[0]+" -> "+rw[1])
		}
	}
	if len(applied) == 0 {
		return
	}
	if err := os.WriteFile(path, []byte(updated), info.Mode().Perm()); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: rewrite %s: %v\n", path, err)
		return
	}
	fmt.Printf("migrate: rewrote %s (%s)\n", rel, strings.Join(applied, "; "))
}

// hooksDirFiles returns the files in the repository hooks directory as
// root-relative paths. Hooks are untracked, so ls-files never lists them;
// they are the original home of the stale references this sweep targets.
// Resolution goes through git (worktrees, core.hooksPath); outside a git
// repository the result is nil.
func hooksDirFiles(root string) []string {
	cmd := exec.Command("git", "-C", root, "rev-parse", "--git-path", "hooks")
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	hooksDir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(hooksDir) {
		hooksDir = filepath.Join(root, hooksDir)
	}
	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if rel, rerr := filepath.Rel(root, filepath.Join(hooksDir, e.Name())); rerr == nil {
			files = append(files, rel)
		}
	}
	return files
}
