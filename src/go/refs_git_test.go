package main

import (
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// gitRun runs a git command in repo, failing the test on error.
func gitRun(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// gitOut runs a git command in repo and returns trimmed stdout.
func gitOut(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

func TestLoadGitRefs(t *testing.T) {
	t.Run("non-repo returns nil", func(t *testing.T) {
		if got := loadGitRefs(t.TempDir()); got != nil {
			t.Errorf("loadGitRefs on non-repo = %v, want nil", got)
		}
	})

	t.Run("local and remote-tracking refs, HEAD symref excluded", func(t *testing.T) {
		gitOrSkip(t)
		repo := t.TempDir()
		initRepoWithCommit(t, repo)
		gitRun(t, repo, "branch", "feat/0001-foo")
		gitRun(t, repo, "branch", "0002-bar")
		sha := gitOut(t, repo, "rev-parse", "HEAD")
		gitRun(t, repo, "update-ref", "refs/remotes/origin/main", sha)
		gitRun(t, repo, "update-ref", "refs/remotes/origin/feat/0003-baz", sha)
		gitRun(t, repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

		refs := loadGitRefs(repo)

		for _, want := range []string{"main", "feat/0001-foo", "0002-bar", "origin/main", "origin/feat/0003-baz"} {
			if !slices.Contains(refs, want) {
				t.Errorf("refs %v missing %q", refs, want)
			}
		}
		// The remote HEAD symref's short name is the bare remote name
		// ("origin"), not "origin/HEAD". It must be excluded as a symbolic ref.
		if slices.Contains(refs, "origin") {
			t.Errorf("refs %v should exclude remote HEAD symref (short name 'origin')", refs)
		}
	})
}

func TestLoadRefMatches(t *testing.T) {
	t.Run("empty ids returns nil", func(t *testing.T) {
		if got := loadRefMatches(t.TempDir(), nil); got != nil {
			t.Errorf("loadRefMatches with no ids = %v, want nil", got)
		}
	})

	t.Run("branches and remote-tracking refs map to ids with boundary check", func(t *testing.T) {
		gitOrSkip(t)
		repo := t.TempDir()
		initRepoWithCommit(t, repo)
		gitRun(t, repo, "branch", "feat/0001-foo")
		gitRun(t, repo, "branch", "0002-bar")
		gitRun(t, repo, "branch", "feat/00010-x") // must NOT match 0001
		sha := gitOut(t, repo, "rev-parse", "HEAD")
		gitRun(t, repo, "update-ref", "refs/remotes/origin/feat/0003-baz", sha)

		matches := loadRefMatches(repo, []string{"0001", "0002", "0003", "0010"})

		if got := matches["0001"]; len(got) != 1 || got[0] != "feat/0001-foo" {
			t.Errorf("0001 -> %v, want [feat/0001-foo]", got)
		}
		if got := matches["0002"]; len(got) != 1 || got[0] != "0002-bar" {
			t.Errorf("0002 -> %v, want [0002-bar]", got)
		}
		if got := matches["0003"]; len(got) != 1 || got[0] != "origin/feat/0003-baz" {
			t.Errorf("0003 -> %v, want [origin/feat/0003-baz]", got)
		}
		if got := matches["0010"]; len(got) != 0 {
			t.Errorf("0010 -> %v, want no match (00010 fails word boundary)", got)
		}
	})

	t.Run("other worktree path included, host worktree excluded", func(t *testing.T) {
		gitOrSkip(t)
		repo := t.TempDir()
		initRepoWithCommit(t, repo)
		wtPath := filepath.Join(t.TempDir(), "wt")
		gitRun(t, repo, "worktree", "add", "-q", wtPath, "-b", "wt/0004-qux")

		matches := loadRefMatches(repo, []string{"0004"})

		got := matches["0004"]
		// The branch wt/0004-qux is reported via the ref scan first; the
		// worktree path follows. The host worktree (repo) is never listed.
		if len(got) != 2 {
			t.Fatalf("0004 -> %v, want 2 entries (branch + worktree path)", got)
		}
		if got[0] != "wt/0004-qux" {
			t.Errorf("0004 first entry = %q, want branch wt/0004-qux", got[0])
		}
		if !filepath.IsAbs(got[1]) {
			t.Errorf("0004 second entry = %q, want absolute worktree path", got[1])
		}
		if got[1] == repo {
			t.Errorf("host worktree %q should be excluded", repo)
		}
	})

	t.Run("host worktree path excluded even when host branch references id", func(t *testing.T) {
		// Distinguishing case for the `wt.path == top` guard: put the host repo
		// on a branch that references the same id as the secondary worktree.
		// Without the guard, loadRefMatches would append `repo` (the host path)
		// to matches["0004"], violating the docstring contract
		// "The worktree containing dir itself is omitted".
		//
		// Mutation: removing `wt.path == top` from the skip condition causes
		// the host repo path to appear in matches["0004"], failing this subtest.
		gitOrSkip(t)
		repo := t.TempDir()
		// Init with a branch that references 0004, so the host worktree IS a
		// candidate match via both the ref scan (branch name) and the worktree
		// walk (wt.path). The guard must suppress the worktree-path entry.
		initRepoWithCommit(t, repo)
		gitRun(t, repo, "checkout", "-b", "wt/0004-host")
		wtPath := filepath.Join(t.TempDir(), "wt")
		gitRun(t, repo, "worktree", "add", "-q", wtPath, "-b", "wt/0004-qux")

		matches := loadRefMatches(repo, []string{"0004"})

		got := matches["0004"]
		// Expected: [wt/0004-host (ref scan), wt/0004-qux (ref scan), wtPath (worktree walk)]
		// The host repo top-level path must NOT appear.
		for _, entry := range got {
			if entry == repo {
				t.Errorf("host worktree path %q should be excluded from matches; got %v", repo, got)
			}
		}
		// The secondary worktree's absolute path must be present.
		found := false
		for _, entry := range got {
			if entry == wtPath {
				found = true
			}
		}
		if !found {
			t.Errorf("secondary worktree path %q should be in matches; got %v", wtPath, got)
		}
	})
}
