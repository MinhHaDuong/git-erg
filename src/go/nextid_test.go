package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseIDFromFilename(t *testing.T) {
	cases := []struct {
		name string
		want int
	}{
		{"0001-foo.erg", 1},
		{"0042-some-title.erg", 42},
		{"0100.erg", 100},
		{"9999-max.erg", 9999},
		{"0001-foo.txt", 0},
		{"readme.erg", 0},
		{"not-a-number-foo.erg", 0},
		{"", 0},
		{"0005.txt", 0},
	}
	for _, c := range cases {
		got := parseIDFromFilename(c.name)
		if got != c.want {
			t.Errorf("parseIDFromFilename(%q) = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestNextID(t *testing.T) {
	cases := []struct {
		desc  string
		files map[string]string // relative path -> content (content is irrelevant)
		want  string
	}{
		{
			desc:  "empty directory",
			files: nil,
			want:  "0001",
		},
		{
			desc:  "one ticket",
			files: map[string]string{"0001-foo.erg": ""},
			want:  "0002",
		},
		{
			desc:  "gap takes max+1 not first gap",
			files: map[string]string{"0001-a.erg": "", "0003-b.erg": ""},
			want:  "0004",
		},
		{
			desc:  "max existing 0099",
			files: map[string]string{"0099-high.erg": ""},
			want:  "0100",
		},
		{
			desc:  "non-erg files ignored",
			files: map[string]string{"0050-notes.txt": "", "0060-data.md": ""},
			want:  "0001",
		},
		{
			desc:  "non-numeric prefix ignored",
			files: map[string]string{"readme.erg": "", "abc-def.erg": ""},
			want:  "0001",
		},
		{
			desc: "mix of erg and non-erg with padded IDs",
			files: map[string]string{
				"0010-ticket.erg": "",
				"0020-other.txt":  "",
				"0005-also.erg":   "",
				"notes.md":        "",
			},
			want: "0011",
		},
		{
			desc: "tickets in closed/ subdir are counted",
			files: map[string]string{
				"closed/0010-old.erg": "",
				"0003-active.erg":     "",
			},
			want: "0011",
		},
		{
			desc: "tickets in archive/ subdir are counted",
			files: map[string]string{
				"archive/0050-ancient.erg": "",
				"0002-current.erg":         "",
			},
			want: "0051",
		},
		{
			desc: "subdirs combined with top-level",
			files: map[string]string{
				"0005-active.erg":           "",
				"closed/0020-done.erg":      "",
				"archive/0015-archived.erg": "",
			},
			want: "0021",
		},
		{
			desc: "nonexistent directory returns 0001",
			// handled specially below
			want: "0001",
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			if c.desc == "nonexistent directory returns 0001" {
				got := nextID("/nonexistent/path/that/does/not/exist")
				if got != c.want {
					t.Errorf("nextID(nonexistent) = %q, want %q", got, c.want)
				}
				return
			}

			tmp := t.TempDir()
			for relPath, content := range c.files {
				full := filepath.Join(tmp, relPath)
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			got := nextID(tmp)
			if got != c.want {
				t.Errorf("nextID() = %q, want %q", got, c.want)
			}
		})
	}
}

// gitOrSkip runs `git --version`; if git is unavailable the caller skips.
// All cross-worktree tests need git in PATH.
func gitOrSkip(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

// initRepoWithCommit creates a git repo at root with one initial commit so
// branches can be created. Sets local user.email/user.name so subsequent
// commits do not depend on global config.
func initRepoWithCommit(t *testing.T, root string) {
	t.Helper()
	mustGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	mustGit("init", "-q", "-b", "main")
	mustGit("config", "user.email", "test@example.com")
	mustGit("config", "user.name", "Test")
	mustGit("config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(root, "README"), []byte("init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit("add", "README")
	mustGit("commit", "-q", "-m", "init")
}

func TestNextID_SkipsSiblingWorktreeUncommittedTicket(t *testing.T) {
	gitOrSkip(t)
	root := t.TempDir()
	initRepoWithCommit(t, root)
	if err := os.MkdirAll(filepath.Join(root, "tickets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tickets", "0050-local.erg"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	wt2 := filepath.Join(t.TempDir(), "wt2")
	cmd := exec.Command("git", "-C", root, "worktree", "add", "-b", "wt2-branch", wt2)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %v\n%s", err, out)
	}
	if err := os.MkdirAll(filepath.Join(wt2, "tickets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt2, "tickets", "0123-uncommitted-in-sibling.erg"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	got := nextID(filepath.Join(root, "tickets"))
	if got != "0124" {
		t.Errorf("nextID from primary worktree = %q, want 0124 (must skip past 0123 in sibling)", got)
	}
}

func TestNextID_SkipsTicketCommittedOnUncheckedOutBranch(t *testing.T) {
	gitOrSkip(t)
	root := t.TempDir()
	initRepoWithCommit(t, root)
	mustGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	mustGit("checkout", "-q", "-b", "other-branch")
	if err := os.MkdirAll(filepath.Join(root, "tickets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tickets", "0200-on-branch.erg"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit("add", "tickets/0200-on-branch.erg")
	mustGit("commit", "-q", "-m", "add 0200")
	mustGit("checkout", "-q", "main")
	if err := os.RemoveAll(filepath.Join(root, "tickets")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "tickets"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := nextID(filepath.Join(root, "tickets"))
	if got != "0201" {
		t.Errorf("nextID = %q, want 0201 (must see 0200 on other-branch via ls-tree)", got)
	}
}

func TestNextID_SiblingWithoutSubdirIsHarmless(t *testing.T) {
	gitOrSkip(t)
	root := t.TempDir()
	initRepoWithCommit(t, root)
	if err := os.MkdirAll(filepath.Join(root, "tickets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tickets", "0010-local.erg"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	wt2 := filepath.Join(t.TempDir(), "wt2")
	cmd := exec.Command("git", "-C", root, "worktree", "add", "-b", "wt2-branch", wt2)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %v\n%s", err, out)
	}
	// Sibling worktree has no tickets/ subdir at all.

	got := nextID(filepath.Join(root, "tickets"))
	if got != "0011" {
		t.Errorf("nextID = %q, want 0011 (sibling without tickets/ must be a no-op)", got)
	}
}

func TestNextID_NotAGitRepoFallsBackToLocal(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "0007-only-local.erg"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got := nextID(tmp)
	if got != "0008" {
		t.Errorf("nextID = %q, want 0008 (no-repo path must still work)", got)
	}
}

func TestMaxIDInDir(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "closed"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"0003-a.erg", "0042-b.erg", "closed/0007-c.erg", "readme.md"} {
		if err := os.WriteFile(filepath.Join(tmp, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := maxIDInDir(tmp); got != 42 {
		t.Errorf("maxIDInDir = %d, want 42", got)
	}
	if got := maxIDInDir(filepath.Join(tmp, "does-not-exist")); got != 0 {
		t.Errorf("maxIDInDir(missing) = %d, want 0", got)
	}
}
