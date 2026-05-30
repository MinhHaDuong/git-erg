package main

import (
	"io"
	"os/exec"
	"strings"
)

// refReferencesID reports whether the git ref short name refName references
// the ticket whose 4-digit ID is id. Per spec-erg-v1.md, a ref references a
// ticket when the literal ID appears in the short name delimited by word
// boundaries -- start, end, or one of '/', '-', '_'. So "feat/0001-foo"
// references 0001, but "feat/00010-foo" does not.
func refReferencesID(refName, id string) bool {
	if id == "" || len(refName) < len(id) {
		return false
	}
	for i := 0; i+len(id) <= len(refName); i++ {
		if refName[i:i+len(id)] != id {
			continue
		}
		if isRefBoundary(refName, i-1) && isRefBoundary(refName, i+len(id)) {
			return true
		}
	}
	return false
}

func isRefBoundary(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return true
	}
	c := s[i]
	return c == '/' || c == '-' || c == '_'
}

// loadGitRefs returns short names of every local and remote-tracking branch
// in the repository containing dir, excluding remote HEAD symrefs. Returns
// nil on any git error (no repo, etc.) so callers degrade silently -- refs
// are an optional annotation, not a precondition.
func loadGitRefs(dir string) []string {
	// Skip symbolic refs (e.g. refs/remotes/origin/HEAD) structurally: the
	// %(if)%(symref) test emits an empty line for them, which the loop drops.
	// Matching on a "/HEAD" short-name suffix does not work -- a remote HEAD
	// symref's short name is just the remote name ("origin"), not "origin/HEAD".
	cmd := exec.Command("git", "-C", dir, "for-each-ref",
		"--format=%(if)%(symref)%(then)%(else)%(refname:short)%(end)",
		"refs/heads", "refs/remotes")
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var refs []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		refs = append(refs, line)
	}
	return refs
}

// worktreeRef pairs a worktree's filesystem path with its checked-out branch
// short name (empty for detached HEAD).
type worktreeRef struct {
	path   string
	branch string
}

// loadGitWorktrees enumerates worktrees of the repository containing dir via
// `git worktree list --porcelain`. Returns nil on git error.
func loadGitWorktrees(dir string) []worktreeRef {
	cmd := exec.Command("git", "-C", dir, "worktree", "list", "--porcelain")
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var list []worktreeRef
	var cur worktreeRef
	flush := func() {
		if cur.path != "" {
			list = append(list, cur)
		}
		cur = worktreeRef{}
	}
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			flush()
			continue
		}
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur.path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch "):
			cur.branch = strings.TrimPrefix(
				strings.TrimPrefix(line, "branch "), "refs/heads/")
		}
	}
	flush()
	return list
}

// worktreeTopFor returns the absolute path of the worktree containing dir, or
// "" if dir is not inside a git repository. Used to suppress the dir's own
// worktree from annotations -- its branch is already shown via the local-ref
// pass, and the absolute path is just noise.
func worktreeTopFor(dir string) string {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitBranch returns the short name of HEAD in the repository containing dir
// (e.g. "main", "t0194"). Returns "" on any git error or when git is
// unavailable, so callers degrade silently offline.
func gitBranch(dir string) string {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// loadRefMatches returns, for each ticket id, the list of git refs and
// worktree paths in the repository containing dir that reference it (per
// refReferencesID). Refs are listed before worktree paths and preserve the
// order returned by git. The worktree containing dir itself is omitted (its
// branch is already covered by the ref scan). Network-free: all data comes
// from git for-each-ref and git worktree list.
func loadRefMatches(dir string, ids []string) map[string][]string {
	if len(ids) == 0 {
		return nil
	}
	matches := make(map[string][]string, len(ids))

	for _, ref := range loadGitRefs(dir) {
		for _, id := range ids {
			if refReferencesID(ref, id) {
				matches[id] = append(matches[id], ref)
			}
		}
	}

	top := worktreeTopFor(dir)
	for _, wt := range loadGitWorktrees(dir) {
		if wt.branch == "" || wt.path == top {
			continue
		}
		for _, id := range ids {
			if refReferencesID(wt.branch, id) {
				matches[id] = append(matches[id], wt.path)
			}
		}
	}

	return matches
}
