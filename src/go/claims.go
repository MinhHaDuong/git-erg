package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ID claims (ticket 0306).
//
// A store whose .claims/ directory exists claims every ticket number it uses:
// erg new writes <store>/.claims/NNNN, one line holding the ticket's slug, a
// space and a random nonce. Two branches that take the same number then add
// the same path with different content, and git reports an add/add conflict
// when the second one is merged or rebased -- offline, on any forge, with no
// CI. The nonce is what makes every same-number pair conflict, including two
// claimants that share a slug.
//
// The directory is the switch: claims are on exactly when it exists, so a
// binary update never fails an existing corpus, and the switch is a plain file
// a human can see and remove. Claims are never deleted (close, rm and archive
// leave them): the directory is the record of every number ever used, which
// is why next-id counts claim names and a claim without a ticket is legal.

// claimsDirName is the claims directory, directly under the store root.
const claimsDirName = ".claims"

// claimsKeepName keeps the directory in git, which carries no empty directory.
// Dot names inside .claims/ are never claims: check and next-id skip them.
const claimsKeepName = ".keep"

// errClaimsSymlink refuses a .claims that is a symlink: following it would
// write claims outside the store.
var errClaimsSymlink = errors.New(".claims is a symlink; claims must be a plain directory inside the store")

// claimsDir returns <store>/.claims.
func claimsDir(store string) string {
	return filepath.Join(store, claimsDirName)
}

// claimsActive reports whether claims are on for store: its .claims/ exists
// and is a directory. A symlinked .claims is an error, never followed.
func claimsActive(store string) (bool, error) {
	fi, err := os.Lstat(claimsDir(store))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return false, errClaimsSymlink
	}
	if !fi.IsDir() {
		return false, fmt.Errorf("%s is not a directory", claimsDir(store))
	}
	return true, nil
}

// parseClaimName returns the ticket number a claim file name carries
// (exactly four digits, 0001-9999), or 0 for any other name -- dot names
// (.keep, a temp file) included.
func parseClaimName(name string) int {
	if len(name) != 4 {
		return 0
	}
	return parseIDFromFilename(name + ".erg")
}

// newNonce returns 8 random hex digits from crypto/rand.
func newNonce() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// claimContent is the one-line claim for slug.
func claimContent(slug, nonce string) string {
	return slug + " " + nonce + "\n"
}

// claimsIgnored reports whether git ignores the claim <store>/.claims/<id>,
// which would leave it out of every commit while erg check passes locally on
// the file on disk. erg new probes the id it is about to claim; init and
// migrate probe one name for the whole directory. git runs from the store,
// which exists, so the probe works before .claims/ is created. Outside a git
// worktree, or without git, nothing is ignored.
func claimsIgnored(store, id string) bool {
	probe := filepath.ToSlash(filepath.Join(claimsDirName, id))
	cmd := exec.Command("git", "-C", store, "check-ignore", "-q", "--", probe)
	return cmd.Run() == nil
}

// claimsIgnoredMessage is the shared refusal for an ignored claims path.
func claimsIgnoredMessage(store string) string {
	return fmt.Sprintf("git ignores the claims in %s, so a claim would never be committed -- un-ignore that directory and the files in it (see 'git check-ignore -v'), then retry",
		claimsDir(store))
}

// linkClaim publishes a claim; a seam so a test can simulate a filesystem
// without hard links.
var linkClaim = os.Link

// writeClaim creates <store>/.claims/<id> holding slug and a fresh nonce,
// atomically and without clobbering: the content goes to a dot-prefixed temp
// in .claims/, then os.Link publishes it under the final name. Link fails
// with EEXIST when the number is already claimed -- the collision signal,
// returned as is (os.IsExist) -- and a killed erg leaves at most a dot-named
// temp (never read as a claim), never a partial claim. On a filesystem
// without hard links (some FUSE, 9p or exFAT mounts) it falls back to an
// O_EXCL create of the final name: still no clobber, and EEXIST still the
// signal, at the price of a partial claim if killed mid-write.
func writeClaim(store, id, slug string) error {
	nonce, err := newNonce()
	if err != nil {
		return err
	}
	dir := claimsDir(store)
	tmp, err := os.CreateTemp(dir, "."+id+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(claimContent(slug, nonce)); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	final := filepath.Join(dir, id)
	if err := linkClaim(tmpName, final); err != nil {
		if os.IsExist(err) {
			return err
		}
		return createExclusive(final, claimContent(slug, nonce))
	}
	return nil
}

// ensureClaimsDir creates <store>/.claims/ and its .keep file. Idempotent:
// an existing .keep is left untouched.
func ensureClaimsDir(store string) error {
	if _, err := claimsActive(store); err != nil {
		return err
	}
	if err := os.MkdirAll(claimsDir(store), 0o755); err != nil {
		return err
	}
	keep := filepath.Join(claimsDir(store), claimsKeepName)
	f, err := os.OpenFile(keep, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	return f.Close()
}

// ticketSlug returns the slug of a NNNN-slug.erg basename ("" without one).
func ticketSlug(basename string) string {
	stem := strings.TrimSuffix(basename, ".erg")
	if idx := strings.Index(stem, "-"); idx > 0 {
		return stem[idx+1:]
	}
	return ""
}

// claimFix is the plain-file remedy for a ticket without a claim: a hand-made
// ticket needs no binary to be claimed (Agnostic invariant).
func claimFix(store, id, slug string) string {
	return fmt.Sprintf(`echo "%s $(date +%%s)" > %s`, slug, filepath.Join(claimsDir(store), id))
}

// claimViolations returns the claim rules' violations for the store, or nil
// when claims are off. The rules (spec-erg-v1.md "Collision handling"):
//
//   - a claim is exactly one line;
//   - its first field is the slug of the ticket that carries its number (the
//     second field is not checked: a hand-made claim may carry any);
//   - every ticket has a claim.
//
// A claim without a ticket passes: claims outlive close, rm and archive.
// These catch the silenced conflicts: `-X ours` lands two tickets under one
// claim (a slug mismatch), a merge=union attribute writes a two-line claim.
func claimViolations(dir string, tickets []Erg) []string {
	active, err := claimsActive(dir)
	if err != nil {
		return []string{fmt.Sprintf("%s: %v", claimsDir(dir), err)}
	}
	if !active {
		return nil
	}
	entries, err := os.ReadDir(claimsDir(dir))
	if err != nil {
		return []string{fmt.Sprintf("%s: cannot read: %v", claimsDir(dir), err)}
	}
	claims := make(map[string]string, len(entries))
	malformed := make(map[string]bool)
	var errs []string
	for _, e := range entries {
		if e.IsDir() || parseClaimName(e.Name()) == 0 {
			continue
		}
		path := filepath.Join(claimsDir(dir), e.Name())
		if e.Type()&fs.ModeSymlink != 0 {
			// A claim is a plain committed file; a link would let check
			// read, and a merge carry, content from outside the store.
			errs = append(errs, fmt.Sprintf("%s: claim is a symlink; replace it with a plain file", path))
			malformed[e.Name()] = true
			continue
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			errs = append(errs, fmt.Sprintf("%s: cannot read claim: %v", path, rerr))
			continue
		}
		line := strings.TrimSuffix(string(data), "\n")
		fields := strings.Fields(line)
		if strings.Contains(line, "\n") || len(fields) < 2 {
			errs = append(errs, fmt.Sprintf(
				"%s: claim must be exactly one line: slug, a space, a nonce -- a merge that kept both sides of a claim conflict, or a hand-made claim without its nonce? two tickets may share number %s",
				path, e.Name()))
			malformed[e.Name()] = true
			continue
		}
		claims[e.Name()] = fields[0]
	}
	for i := range tickets {
		name := tickets[i].Filename()
		id := tickets[i].FilenameID()
		if parseClaimName(id) == 0 {
			continue
		}
		slug := ticketSlug(name)
		claimed, ok := claims[id]
		if !ok {
			if malformed[id] {
				continue
			}
			errs = append(errs, fmt.Sprintf(
				"%s: ticket has no claim %s -- claims are on in this store; fix: %s",
				name, filepath.Join(claimsDir(dir), id), claimFix(dir, id, slug)))
			continue
		}
		if claimed != slug {
			errs = append(errs, fmt.Sprintf(
				"%s: claim %s names slug %q, not %q -- another ticket holds this number (renumber this one clear of the frontier), or the slug was renamed without its claim",
				name, filepath.Join(claimsDir(dir), id), claimed, slug))
		}
	}
	return errs
}

// migrateClaims switches claims on for store and claims every ticket in it,
// open and closed: it creates .claims/ (with .keep) and writes a claim for
// each number that has none. Idempotent: an existing claim is never
// rewritten. A number two tickets share gets no claim (one claim cannot hold
// two slugs): both files are reported and the returned count is non-zero.
func migrateClaims(store string) (written, kept int, shared [][]string, err error) {
	if err := ensureClaimsDir(store); err != nil {
		return 0, 0, nil, err
	}
	byID := make(map[string][]string)
	walkErr := filepath.WalkDir(store, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			if path != store && d.Name() == claimsDirName {
				return filepath.SkipDir
			}
			return nil
		}
		if parseIDFromFilename(d.Name()) == 0 {
			return nil
		}
		id := filenameID(d.Name())
		byID[id] = append(byID[id], path)
		return nil
	})
	if walkErr != nil {
		return 0, 0, nil, walkErr
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		paths := byID[id]
		if len(paths) > 1 {
			sort.Strings(paths)
			shared = append(shared, paths)
			continue
		}
		werr := writeClaim(store, id, ticketSlug(filepath.Base(paths[0])))
		switch {
		case werr == nil:
			written++
		case os.IsExist(werr):
			kept++
		default:
			return written, kept, shared, fmt.Errorf("claim %s: %w", id, werr)
		}
	}
	return written, kept, shared, nil
}
