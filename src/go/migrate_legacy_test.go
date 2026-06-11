package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupLegacyHostRepo builds a scratch git repo that looks like a host repo
// carrying the old (pre spec 0.1) erg init bundle: vendored ticket-* skills,
// a stale managed CLAUDE.md block, and stale tickets/tools/go/erg references
// seeded across the sweep surface (.claude/settings.json, a GitHub workflow,
// a Makefile, .git/hooks/pre-commit), plus the negative controls (a ticket
// body quoting the old path, a clean file, a user-authored ticket-new skill
// without tell-tales). Returns the repo root and tickets dir.
func setupLegacyHostRepo(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
	run("config", "commit.gpgsign", "false")

	write := func(rel, content string, perm os.FileMode) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), perm); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	ticketsDir := filepath.Join(root, "tickets")
	write("tickets/erg", "stub", 0755) // skip the os.Executable self-copy

	// Vendored skills: tell-tale legacy content -- except ticket-new, which is
	// seeded as a user-authored skill under a legacy NAME but with no
	// tell-tales, to prove deletion keys on content rather than name.
	for _, name := range legacySkillNames {
		if name == "ticket-new" {
			continue
		}
		write(".claude/skills/"+name+"/SKILL.md",
			"Create a ticket in %erg v1 format with a Status: header.\n"+
				"Validator: tickets/tools/go\n", 0644)
	}
	write(".claude/skills/ticket-new/SKILL.md", "My own way of filing tickets.\n", 0644)
	// User-authored skill under a non-legacy name: never even considered.
	write(".claude/skills/ticket-triage/SKILL.md", "My own triage notes.\n", 0644)

	// Stale managed CLAUDE.md block, user content on both sides.
	write("CLAUDE.md", strings.Join([]string{
		"# My project",
		"",
		"# --- git-erg: begin managed block ---",
		"Tickets need no CLI needed -- edit .erg files directly (%erg v1).",
		"Validate with tickets/tools/go.",
		"# --- git-erg: end managed block ---",
		"",
		"User notes after the block.",
	}, "\n")+"\n", 0644)

	// Sweep surface: stale references in settings, CI, Make, and a git hook.
	// settings.local.json is gitignored -- the original AEDIST evidence was a
	// local settings file, so the sweep must reach untracked files too.
	write(".gitignore", ".claude/settings.local.json\n", 0644)
	write(".claude/settings.json",
		`{"hooks":{"PostToolUse":"tickets/tools/go/erg validate tickets/"}}`+"\n", 0644)
	write(".claude/settings.local.json",
		`{"hooks":{"PreToolUse":"tickets/tools/go/erg validate tickets/"}}`+"\n", 0644)
	write(".github/workflows/ci.yml",
		"steps:\n  - run: tickets/tools/go/erg validate tickets/\n", 0644)
	write("Makefile",
		"check:\n\ttickets/tools/go/erg validate tickets/\n", 0644)
	// Negative controls.
	write("tickets/0001-history.erg",
		"%erg 0.1\nTitle: History\nCreated: 2026-01-01\nAuthor: t\n\n--- log ---\n2026-01-01T00:00Z t created\n\n--- body ---\nWe used to run tickets/tools/go/erg validate tickets/ here.\n", 0644)
	write("README.md", "Nothing legacy here.\n", 0644)

	run("add", "-A")
	run("commit", "-q", "-m", "seed legacy host repo")

	// The stale hook is seeded after the commit (hooks are untracked, and a
	// live hook with the broken legacy path would fail the seeding commit).
	write(".git/hooks/pre-commit",
		"#!/bin/sh\ntickets/tools/go/erg validate tickets/\n", 0755)
	return root, ticketsDir
}

func TestMigrateLayoutLegacyCleanup(t *testing.T) {
	root, ticketsDir := setupLegacyHostRepo(t)

	readBack := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(data)
	}
	cleanBefore := readBack("README.md")
	ticketBefore := readBack("tickets/0001-history.erg")

	if code := migrateLayout(ticketsDir); code != 0 {
		t.Fatalf("migrateLayout returned %d, want 0", code)
	}

	// Vendored skills are gone; user-authored skills survive -- including
	// ticket-new, which carries a legacy NAME but no tell-tale content.
	for _, name := range legacySkillNames {
		if name == "ticket-new" {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, ".claude", "skills", name)); !os.IsNotExist(err) {
			t.Errorf("vendored skill %s still present, stat err = %v", name, err)
		}
	}
	for _, name := range []string{"ticket-new", "ticket-triage"} {
		if _, err := os.Stat(filepath.Join(root, ".claude", "skills", name)); err != nil {
			t.Errorf("user-authored %s skill was removed: %v", name, err)
		}
	}

	// CLAUDE.md block refreshed, user content preserved on both sides.
	claude := readBack("CLAUDE.md")
	for _, stale := range legacyClaudeMDTelltales {
		if strings.Contains(claude, stale) {
			t.Errorf("CLAUDE.md still contains stale claim %q:\n%s", stale, claude)
		}
	}
	for _, want := range []string{"# My project", "User notes after the block.", "erg <verb>", "%erg 0.1", "tickets/AGENTS.md"} {
		if !strings.Contains(claude, want) {
			t.Errorf("CLAUDE.md missing %q after refresh:\n%s", want, claude)
		}
	}

	// Every seeded sweep target now reads tickets/erg ... check tickets/.
	for _, rel := range []string{".claude/settings.json", ".claude/settings.local.json", ".github/workflows/ci.yml", "Makefile", ".git/hooks/pre-commit"} {
		got := readBack(rel)
		if strings.Contains(got, "tickets/tools/go/erg") || strings.Contains(got, "validate tickets/") {
			t.Errorf("%s still has stale references:\n%s", rel, got)
		}
		if !strings.Contains(got, "tickets/erg") || !strings.Contains(got, "check tickets/") {
			t.Errorf("%s not rewritten to tickets/erg ... check tickets/:\n%s", rel, got)
		}
	}

	// Negative controls are byte-identical: ticket-store history and clean files.
	if got := readBack("tickets/0001-history.erg"); got != ticketBefore {
		t.Errorf("ticket body quoting old path was modified:\n got: %q\nwant: %q", got, ticketBefore)
	}
	if got := readBack("README.md"); got != cleanBefore {
		t.Errorf("clean file was modified:\n got: %q\nwant: %q", got, cleanBefore)
	}

	// Idempotent: a second run is a byte-level no-op on every file checked.
	snapshot := map[string]string{}
	for _, rel := range []string{"CLAUDE.md", ".claude/settings.json", ".claude/settings.local.json", ".github/workflows/ci.yml", "Makefile", ".git/hooks/pre-commit", "README.md", "tickets/0001-history.erg"} {
		snapshot[rel] = readBack(rel)
	}
	if code := migrateLayout(ticketsDir); code != 0 {
		t.Fatalf("second migrateLayout returned %d, want 0", code)
	}
	for rel, want := range snapshot {
		if got := readBack(rel); got != want {
			t.Errorf("second run changed %s:\n got: %q\nwant: %q", rel, got, want)
		}
	}
}

func TestRefreshLegacyClaudeBlock(t *testing.T) {
	t.Run("absent CLAUDE.md is not created", func(t *testing.T) {
		root := t.TempDir()
		refreshLegacyClaudeBlock(root)
		if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); !os.IsNotExist(err) {
			t.Errorf("CLAUDE.md should not exist, stat err = %v", err)
		}
	})

	t.Run("block without stale claims is byte-identical", func(t *testing.T) {
		root := t.TempDir()
		content := "# --- git-erg: begin managed block ---\nAll current.\n# --- git-erg: end managed block ---\n"
		path := filepath.Join(root, "CLAUDE.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		refreshLegacyClaudeBlock(root)
		got, _ := os.ReadFile(path)
		if string(got) != content {
			t.Errorf("clean block was modified:\n got: %q\nwant: %q", got, content)
		}
	})

	t.Run("symlinked CLAUDE.md is not written through", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "real.md")
		content := "# --- git-erg: begin managed block ---\nno CLI needed\n# --- git-erg: end managed block ---\n"
		if err := os.WriteFile(target, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(root, "CLAUDE.md")); err != nil {
			t.Skipf("symlink not supported: %v", err)
		}
		refreshLegacyClaudeBlock(root)
		got, _ := os.ReadFile(target)
		if string(got) != content {
			t.Errorf("symlink target was modified:\n got: %q\nwant: %q", got, content)
		}
	})

	t.Run("two adjacent labelled blocks both pair and refresh correctly", func(t *testing.T) {
		root := t.TempDir()
		content := strings.Join([]string{
			"# --- git-erg: begin managed block ---",
			"no CLI needed (%erg v1)",
			"# --- git-erg: end managed block ---",
			"between",
			"# --- git-erg: begin managed block ---",
			"clean current content",
			"# --- git-erg: end managed block ---",
		}, "\n") + "\n"
		path := filepath.Join(root, "CLAUDE.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		refreshLegacyClaudeBlock(root)
		got, _ := os.ReadFile(path)
		s := string(got)
		if strings.Contains(s, "no CLI needed") {
			t.Errorf("first (stale) block not refreshed:\n%s", s)
		}
		for _, want := range []string{"between", "clean current content", "erg <verb>"} {
			if !strings.Contains(s, want) {
				t.Errorf("missing %q after refresh:\n%s", want, s)
			}
		}
		// The refresh replaces the interior only -- the file keeps its own
		// marker style instead of gaining the hook-style canonical markers.
		if n := strings.Count(s, "# --- git-erg: begin managed block ---"); n != 2 {
			t.Errorf("original begin markers not preserved (found %d, want 2):\n%s", n, s)
		}
		if strings.Contains(s, ">>> erg managed >>>") {
			t.Errorf("hook-style canonical markers leaked into CLAUDE.md:\n%s", s)
		}
	})

	t.Run("unbalanced markers refuse and leave the file unchanged", func(t *testing.T) {
		root := t.TempDir()
		content := "# --- git-erg: begin managed block ---\nno CLI needed\n"
		path := filepath.Join(root, "CLAUDE.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		refreshLegacyClaudeBlock(root)
		got, _ := os.ReadFile(path)
		if string(got) != content {
			t.Errorf("unbalanced-marker file was modified:\n got: %q\nwant: %q", got, content)
		}
	})
}

func TestSweepLegacyRefsOutsideGitRepo(t *testing.T) {
	// The sweep is a plain filesystem walk, so it works even without git
	// (only the hooks pass needs a repository).
	root := t.TempDir()
	path := filepath.Join(root, "Makefile")
	if err := os.WriteFile(path, []byte("check:\n\ttickets/tools/go/erg validate tickets/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	sweepLegacyRefs(root, "tickets")
	got, _ := os.ReadFile(path)
	want := "check:\n\ttickets/erg check tickets/\n"
	if string(got) != want {
		t.Errorf("sweep outside a git repo:\n got: %q\nwant: %q", got, want)
	}
}

func TestSweepLegacyRefsSkipsNestedRepos(t *testing.T) {
	root := t.TempDir()
	stale := "tickets/tools/go/erg validate tickets/\n"
	// Nested repo with a .git directory.
	if err := os.MkdirAll(filepath.Join(root, "vendor", "dep", ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	nestedDir := filepath.Join(root, "vendor", "dep", "Makefile")
	if err := os.WriteFile(nestedDir, []byte(stale), 0644); err != nil {
		t.Fatal(err)
	}
	// Submodule-style work tree: .git is a gitfile, not a directory.
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", ".git"), []byte("gitdir: ../.git/modules/sub\n"), 0644); err != nil {
		t.Fatal(err)
	}
	nestedFile := filepath.Join(root, "sub", "Makefile")
	if err := os.WriteFile(nestedFile, []byte(stale), 0644); err != nil {
		t.Fatal(err)
	}
	// Control in the outer tree: must still be rewritten.
	outer := filepath.Join(root, "Makefile")
	if err := os.WriteFile(outer, []byte(stale), 0644); err != nil {
		t.Fatal(err)
	}
	sweepLegacyRefs(root, "tickets")
	for _, p := range []string{nestedDir, nestedFile} {
		got, _ := os.ReadFile(p)
		if string(got) != stale {
			t.Errorf("%s inside a nested work tree was modified:\n got: %q", p, got)
		}
	}
	got, _ := os.ReadFile(outer)
	if string(got) != "tickets/erg check tickets/\n" {
		t.Errorf("outer-tree file not rewritten:\n got: %q", got)
	}
}

func TestSweepLegacyRefsSkipsLargeAndBinary(t *testing.T) {
	root := t.TempDir()
	binary := append([]byte("tickets/tools/go/erg\x00"), make([]byte, 16)...)
	if err := os.WriteFile(filepath.Join(root, "blob.bin"), binary, 0644); err != nil {
		t.Fatal(err)
	}
	big := append(make([]byte, sweepLegacyRefsMaxSize), []byte("tickets/tools/go/erg")...)
	for i := range big[:sweepLegacyRefsMaxSize] {
		big[i] = 'a'
	}
	if err := os.WriteFile(filepath.Join(root, "big.txt"), big, 0644); err != nil {
		t.Fatal(err)
	}
	sweepLegacyRefs(root, "tickets")
	gotBin, _ := os.ReadFile(filepath.Join(root, "blob.bin"))
	if string(gotBin) != string(binary) {
		t.Error("binary file was modified by the sweep")
	}
	gotBig, _ := os.ReadFile(filepath.Join(root, "big.txt"))
	if string(gotBig) != string(big) {
		t.Error("oversized file was modified by the sweep")
	}
}
