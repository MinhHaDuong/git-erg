package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// updateRemote is the default git remote `erg update` fetches from when neither
// ERG_UPDATE_URL nor the .ergrc [update] url is set. "origin" makes update
// fork-kind: you update from where you cloned, so "from origin" is literally true.
const updateRemote = "origin"

// summaryUpdate is the one-liner printed by printUsage via the commands registry.
const summaryUpdate = "Fetch and replace binary from origin (via git)"

const helpUpdate = `## erg update

Fetch the committed binary from your git remote and replace this executable atomically.

Uses git (already a dependency of git-erg) -- never an embedded network client -- so
the binary carries no network code at all. It runs 'git fetch <remote> HEAD' in the
ticket store's repository, extracts the committed binary at that remote's default
branch, and compares its hash to the running binary.

The remote defaults to 'origin' (you update from where you cloned). Override it with
the ERG_UPDATE_URL environment variable or the .ergrc [update] url key -- the value is
a git remote name or URL, so a fork can point it at upstream to track upstream's binary.

If the fetched hash matches the running binary, prints "already up to date" and exits 0.
Otherwise replaces the binary via an atomic rename (write to .tmp, then rename over self).

Fetch errors exit 0 so that 'erg update && erg validate' chains do not fail in offline
or isolated environments (no remote configured, no network, not a git repo). If no
ticket store is found, update does nothing and exits 0 -- it never pulls the binary from
an unrelated repository you happen to be standing in.

After a successful update, checks whether any .erg files in the ticket store still carry
legacy Status: headers. If found, prints explicit migration guidance: 'erg migrate DIR',
'git diff tickets/', 'git commit'. The update command never mutates ticket files itself --
migration is a separate, reviewable step.
`

// resolveUpdateRemote applies the update-source precedence: the ERG_UPDATE_URL
// environment variable wins, then the .ergrc [update] url, then the compiled-in
// default ("origin"). Empty strings are treated as unset at each level. The
// resolved value is a git remote name or URL passed to `git fetch`.
func resolveUpdateRemote(envRemote, configRemote, defaultRemote string) string {
	if envRemote != "" {
		return envRemote
	}
	if configRemote != "" {
		return configRemote
	}
	return defaultRemote
}

// gitToplevel returns the absolute path of the git working tree containing dir,
// or "" if dir is not inside a git repository.
func gitToplevel(dir string) string {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// fetchRemoteBinary fetches the default branch of remote into FETCH_HEAD and
// returns the bytes of the committed binary at blobPath (a path relative to the
// repository root). All work happens via git run in gitDir; no network client is
// embedded. The git stderr is folded into the returned error for diagnostics.
func fetchRemoteBinary(gitDir, remote, blobPath string) ([]byte, error) {
	var stderr bytes.Buffer
	fetch := exec.Command("git", "-C", gitDir, "fetch", "--quiet", remote, "HEAD")
	fetch.Stderr = &stderr
	if err := fetch.Run(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}

	stderr.Reset()
	show := exec.Command("git", "-C", gitDir, "cat-file", "blob", "FETCH_HEAD:"+blobPath)
	var out bytes.Buffer
	show.Stdout = &out
	show.Stderr = &stderr
	if err := show.Run(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out.Bytes(), nil
}

// cmdUpdate implements `erg update`. See helpUpdate for the user-facing summary.
func cmdUpdate(_ []string) int {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "update: cannot resolve executable: %v\n", err)
		return 1
	}
	if resolved, rErr := filepath.EvalSymlinks(self); rErr == nil {
		self = resolved
	}

	localHash, err := selfHash(self)
	if err != nil {
		fmt.Fprintf(os.Stderr, "update: cannot hash self: %v\n", err)
		return 1
	}

	// Locate the ticket store and, through it, the repository that carries the
	// committed binary. The store dir is also where the post-update migration
	// scan runs, so resolve it once.
	ticketDir := os.Getenv("ERG_TICKET_DIR")
	if ticketDir == "" {
		if d, findErr := findTicketsDir(); findErr == nil {
			ticketDir = d
		}
	}

	// Anchor the fetch to a resolved ticket store. Without one we have no
	// trustworthy notion of "this project's repo", and falling back to the
	// current directory's git repo would let `erg update` silently pull the
	// binary from whatever unrelated repo you happen to be standing in. Refuse
	// and leave the binary untouched (exit 0, so update && validate still works).
	if ticketDir == "" {
		fmt.Fprintln(os.Stderr,
			"update: no git-erg ticket store found here -- run from inside your "+
				"git-erg repo, or set ERG_TICKET_DIR. Binary left unchanged.")
		return 0
	}

	var configRemote string
	if os.Getenv("ERG_UPDATE_URL") == "" {
		if cfg, cfgErr := loadConfig(ticketDir); cfgErr == nil && cfg != nil {
			configRemote = cfg.UpdateURL
		}
	}
	remote := resolveUpdateRemote(os.Getenv("ERG_UPDATE_URL"), configRemote, updateRemote)

	// The repo's committed binary lives at <ticket store>/erg. Translate that
	// to a repo-root-relative path for `git cat-file blob FETCH_HEAD:<path>`.
	// If the store is not inside a git repo, blobPath is moot -- the fetch below
	// fails (not a repo) and we exit 0, leaving the binary in place.
	blobPath := "tickets/erg"
	if top := gitToplevel(ticketDir); top != "" {
		if abs, absErr := filepath.Abs(ticketDir); absErr == nil {
			if rel, relErr := filepath.Rel(top, filepath.Join(abs, "erg")); relErr == nil {
				blobPath = filepath.ToSlash(rel)
			}
		}
	}

	body, err := fetchRemoteBinary(ticketDir, remote, blobPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "update: could not fetch -- %v\n", err)
		return 0
	}
	if len(body) == 0 {
		fmt.Fprintf(os.Stderr, "update: fetched an empty binary -- leaving current in place\n")
		return 0
	}

	sum := sha256.Sum256(body)
	remoteHash := hex.EncodeToString(sum[:])

	if localHash == remoteHash {
		fmt.Println("erg: already up to date")
		return 0
	}

	tmp := self + ".tmp"
	if err := os.WriteFile(tmp, body, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "update: cannot write temp file: %v\n", err)
		return 1
	}
	if err := os.Rename(tmp, self); err != nil {
		_ = os.Remove(tmp)
		fmt.Fprintf(os.Stderr, "update: cannot replace binary: %v\n", err)
		return 1
	}

	fmt.Printf("erg: updated (%s \u2192 %s)\n", localHash[:12], remoteHash[:12])

	// Detect tickets still carrying `Status:` headers and emit a hint.
	// Migration is explicit: the user runs `erg migrate`, reviews the diff,
	// and commits separately. erg update never mutates ticket files.
	if info, err := os.Stat(ticketDir); err == nil && info.IsDir() && hasStatusHeader(ticketDir) {
		fmt.Printf("erg: detected Status: headers in %s -- run:\n", ticketDir)
		fmt.Printf("  erg migrate %s\n", ticketDir)
		fmt.Println("  git diff tickets/")
		fmt.Println("  git commit -m 'chore: migrate to Closed: header'")
	}
	return 0
}
