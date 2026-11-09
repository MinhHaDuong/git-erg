package main

import (
	"crypto/rand"
	"crypto/sha256"
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
// Every store claims every ticket number it uses (ticket 0307: claims are
// required, a store without .claims/ is noncompliant, never a mode): erg new
// writes <store>/.claims/NNNN, one line holding the ticket's slug, a
// space and a random nonce. Two branches that take the same number then add
// the same path with different content, and git reports an add/add conflict
// when the second one is merged or rebased -- offline, on any forge, with no
// CI. The nonce is what makes every same-number pair conflict, including two
// claimants that share a slug.
//
// A store without .claims/ fails erg check and makes erg new refuse, each
// naming the one repair: erg migrate --claims. Claims are never deleted
// (close, rm and archive leave them): the directory is the record of every number ever used, which
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

// errNoClaims is wrapped by requireClaims for a store without .claims/, so
// the one caller that creates it (ensureClaimsDir) can tell that case apart.
var errNoClaims = errors.New("ID claims are required")

// requireClaims returns nil when store holds its .claims/ directory, and an
// error naming the repair otherwise. A symlinked .claims is an error, never
// followed.
func requireClaims(store string) error {
	fi, err := os.Lstat(claimsDir(store))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s has no .claims/: %w -- run: erg migrate --claims %s (without the binary: mkdir %s/.claims && touch %s/.claims/.keep, then claim each ticket: echo \"SLUG $(date +%%s)\" > %s/.claims/NNNN)",
				store, errNoClaims, store, store, store, store)
		}
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return errClaimsSymlink
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", claimsDir(store))
	}
	return nil
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

// migrationNonce is the nonce erg migrate --claims writes for a ticket that
// already exists: the first 8 hex digits of sha256("NNNN slug Created"). It
// is derived, not random, so two clones or branches that migrate the same
// base write the same bytes and merge clean; a random one would make every
// claim an add/add conflict. The title is left out because branches edit it.
// erg new keeps the random nonce: a fresh ticket must conflict with another
// fresh ticket of the same number, even one sharing its slug and day.
func migrationNonce(id, slug, created string) string {
	sum := sha256.Sum256([]byte(id + " " + slug + " " + created))
	return hex.EncodeToString(sum[:4])
}

// claimContent is the one-line claim for slug.
func claimContent(slug, nonce string) string {
	return slug + " " + nonce + "\n"
}

// claimIgnoreRule is one git ignore rule that ignores claim paths, with the
// exact lines that un-ignore the claims when appended to its file.
type claimIgnoreRule struct {
	source, line, pattern string   // as git check-ignore -v reports them
	paths                 []string // the store paths it ignores
	allow                 []string // the lines to append to source
	store, storeRule      string   // the store, and the rule ignoring it, if any
}

// message is the refusal for one rule. Paths are capped: migrate probes
// every ticket number, and one rule usually ignores them all.
func (r claimIgnoreRule) message() string {
	shown := r.paths
	more := ""
	if len(shown) > 3 {
		more = fmt.Sprintf(" and %d more", len(shown)-3)
		shown = shown[:3]
	}
	first := ""
	if r.storeRule != "" {
		// git never re-includes a file under an excluded directory, so
		// the claim lines alone cannot work.
		first = fmt.Sprintf("git ignores the store directory %s (rule %s, on it or a directory above it): un-ignore it first, then ", r.store, r.storeRule)
	}
	return fmt.Sprintf("%sgit ignores %s%s by the rule %s:%s:%s, so the claims would never be committed -- append these lines to %s, then retry:\n  %s",
		first, strings.Join(shown, ", "), more, r.source, r.line, r.pattern, r.source, strings.Join(r.allow, "\n  "))
}

// ignoredClaimRules returns the git ignore rules that ignore any of names
// (files inside <store>/.claims/), one per rule (source and line), in the
// order git first reports them; nil when none does. An ignored claim would be
// left out of every commit while erg check passes locally on the file on
// disk. Each caller probes exactly the files it is about to write: erg new
// its claim, init .keep, migrate .keep and every claim. One git process
// answers for all names (check-ignore -v -z --stdin). git runs from the
// store, which exists, so the probe works before .claims/ is created.
// Outside a git worktree, or without git, nothing is ignored.
//
// check-ignore -v also reports a path whose last matching pattern is a
// negation (exit 0, pattern starting with "!"): that path is not ignored, and
// is dropped. The allowlist lines are computed for the base the rule's
// patterns are relative to (claimAllowLines).
func ignoredClaimRules(store string, names ...string) []claimIgnoreRule {
	var in strings.Builder
	for _, n := range names {
		in.WriteString(filepath.ToSlash(filepath.Join(claimsDirName, n)))
		in.WriteByte(0)
	}
	cmd := exec.Command("git", "-C", store, "check-ignore", "-v", "-z", "--stdin")
	cmd.Stdin = strings.NewReader(in.String())
	out, err := cmd.Output()
	if err != nil {
		// Exit 1: nothing matched. 128 or no git: cannot tell, not ignored.
		return nil
	}
	fields := strings.Split(string(out), "\x00")
	var rules []claimIgnoreRule
	index := make(map[string]int)
	var top, real string // resolved once, on the first rule
	for i := 0; i+3 < len(fields); i += 4 {
		source, line, pattern, path := fields[i], fields[i+1], fields[i+2], fields[i+3]
		if strings.HasPrefix(pattern, "!") {
			continue
		}
		key := source + ":" + line
		j, ok := index[key]
		if !ok {
			if rules == nil {
				top, real = storePaths(store)
			}
			rules = append(rules, claimIgnoreRule{source: source, line: line, pattern: pattern, allow: claimAllowLines(store, top, real, source)})
			j = len(rules) - 1
			index[key] = j
		}
		rules[j].paths = append(rules[j].paths, filepath.Join(store, filepath.FromSlash(path)))
	}
	if len(rules) > 0 {
		// Only on the refusal path: is the store directory itself ignored?
		probe := exec.Command("git", "-C", store, "check-ignore", "-v", "-z", "--stdin")
		probe.Stdin = strings.NewReader(".\x00")
		if out, err := probe.Output(); err == nil {
			if f := strings.Split(string(out), "\x00"); len(f) >= 3 && !strings.HasPrefix(f[2], "!") {
				for i := range rules {
					rules[i].store, rules[i].storeRule = store, f[0]+":"+f[1]+":"+f[2]
				}
			}
		}
	}
	return rules
}

// storePaths returns the repository top and the store's resolved absolute
// path, "" for either one it cannot tell.
func storePaths(store string) (top, real string) {
	if out, err := exec.Command("git", "-C", store, "rev-parse", "--show-toplevel").Output(); err == nil {
		top = strings.TrimSpace(string(out))
	}
	if abs, err := filepath.Abs(store); err == nil {
		real = abs
		if r, err := filepath.EvalSymlinks(abs); err == nil {
			real = r
		}
	}
	return top, real
}

// claimAllowLines returns the two lines that un-ignore store's .claims/ and
// the files in it, written relative to the directory the ignoring file's
// patterns are relative to: its own for a .gitignore (git reports it
// repo-top-relative), the repository top for info/exclude and
// core.excludesFile. Those two are told apart by name, not by an absolute
// path: git reports info/exclude as ".git/info/exclude" in a plain checkout
// and as an absolute path in a linked worktree. top and real come from
// storePaths; without them, or for a store outside the base, the lines name
// the store as given.
func claimAllowLines(store, top, real, source string) []string {
	lines := func(prefix string) []string {
		return []string{"!" + prefix + claimsDirName + "/", "!" + prefix + claimsDirName + "/*"}
	}
	fallback := lines(filepath.ToSlash(filepath.Clean(store)) + "/")
	if top == "" || real == "" {
		return fallback
	}
	base := top
	if filepath.Base(source) == ".gitignore" && !filepath.IsAbs(source) {
		base = filepath.Join(top, filepath.Dir(filepath.FromSlash(source)))
	}
	rel, err := filepath.Rel(base, real)
	switch {
	case err != nil || rel == ".." || strings.HasPrefix(rel, "../"):
		return fallback
	case rel == ".":
		return lines("")
	}
	return lines(filepath.ToSlash(rel) + "/")
}

// ticketPathsByID maps each ticket number in store (open and closed, any
// depth) to the paths of the ticket files that carry it.
func ticketPathsByID(store string) (map[string][]string, error) {
	byID := make(map[string][]string)
	err := filepath.WalkDir(store, func(path string, d fs.DirEntry, werr error) error {
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
	return byID, err
}

// linkClaim publishes a claim; a seam so a test can simulate a filesystem
// without hard links.
var linkClaim = os.Link

// writeClaim creates <store>/.claims/<id> holding slug and nonce,
// atomically and without clobbering: the content goes to a dot-prefixed temp
// in .claims/, then os.Link publishes it under the final name. Link fails
// with EEXIST when the number is already claimed -- the collision signal,
// returned as is (os.IsExist) -- and a killed erg leaves at most a dot-named
// temp (never read as a claim), never a partial claim. On a filesystem
// without hard links (some FUSE, 9p or exFAT mounts) it falls back to an
// O_EXCL create of the final name: still no clobber, and EEXIST still the
// signal, at the price of a partial claim if killed mid-write.
func writeClaim(store, id, slug, nonce string) error {
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
	if err := requireClaims(store); err != nil && !errors.Is(err, errNoClaims) {
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

// readClaims reads every claim in dir's .claims/ (which the caller has
// checked exists): the slug each well-formed claim names, the numbers whose
// claim is malformed or a symlink, and one message per bad claim. Shared by
// check (claimViolations) and the migrate precheck (migrateBlockers).
func readClaims(dir string) (claims map[string]string, malformed map[string]bool, errs []string) {
	entries, err := os.ReadDir(claimsDir(dir))
	if err != nil {
		return nil, nil, []string{fmt.Sprintf("%s: cannot read: %v", claimsDir(dir), err)}
	}
	claims = make(map[string]string, len(entries))
	malformed = make(map[string]bool)
	for _, e := range entries {
		if parseClaimName(e.Name()) == 0 {
			continue
		}
		path := filepath.Join(claimsDir(dir), e.Name())
		if e.IsDir() {
			// Named like a claim, so it holds the number's place: nothing
			// can write the claim, and check must not call it missing.
			errs = append(errs, fmt.Sprintf("%s: is a directory, not a claim file; remove it", path))
			malformed[e.Name()] = true
			continue
		}
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
	return claims, malformed, errs
}

// claimViolations returns the claim rules' violations for the store. The
// rules (spec-erg-v1.md "Collision handling"):
//
//   - the store has its .claims/ directory;
//   - a claim is exactly one line;
//   - its first field is the slug of the ticket that carries its number (the
//     second field is not checked: a hand-made claim may carry any);
//   - every ticket has a claim, except a number two tickets share, which
//     the duplicate-ID rule reports with its fix (renumber).
//
// A claim without a ticket passes: claims outlive close, rm and archive.
// These catch the silenced conflicts: `-X ours` lands two tickets on one
// number (a duplicate ID), a merge=union attribute writes a two-line claim.
func claimViolations(dir string, tickets []Erg) []string {
	if err := requireClaims(dir); err != nil {
		if errors.Is(err, errNoClaims) {
			return []string{err.Error()}
		}
		return []string{fmt.Sprintf("%s: %v", claimsDir(dir), err)}
	}
	claims, malformed, errs := readClaims(dir)
	// A number two tickets share is reported once, by the duplicate-ID rule,
	// with its fix (renumber). A claim line for each ticket of the pair would
	// prescribe the echo fix for both, which creates a fresh conflict.
	held := make(map[string]int, len(tickets))
	for i := range tickets {
		held[tickets[i].FilenameID()]++
	}
	for i := range tickets {
		name := tickets[i].Filename()
		id := tickets[i].FilenameID()
		if parseClaimName(id) == 0 {
			continue
		}
		if held[id] > 1 {
			continue
		}
		slug := ticketSlug(name)
		claimed, ok := claims[id]
		if !ok {
			if malformed[id] {
				continue
			}
			errs = append(errs, fmt.Sprintf(
				"%s: ticket has no claim %s -- fix: erg migrate --claims %s, or by hand: %s",
				name, filepath.Join(claimsDir(dir), id), dir, claimFix(dir, id, slug)))
			continue
		}
		if claimed != slug {
			errs = append(errs, slugMismatchMessage(name, filepath.Join(claimsDir(dir), id), claimed, slug))
		}
	}
	return errs
}

// slugMismatchMessage is the violation for a claim naming another slug,
// shared by check and the migrate precheck.
func slugMismatchMessage(ticket, claim, claimed, slug string) string {
	return fmt.Sprintf("%s: claim %s names slug %q, not %q -- another ticket holds this number (renumber this one clear of the frontier), or the slug was renamed without its claim",
		ticket, claim, claimed, slug)
}

// claimUntracked reports whether git says the claim at path (inside store)
// is not committed. Outside a git worktree, or without git, it says no.
func claimUntracked(store, name string) bool {
	rel := filepath.ToSlash(filepath.Join(claimsDirName, name))
	err := exec.Command("git", "-C", store, "ls-files", "--error-unmatch", "--", rel).Run()
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 1
}

// migrateBlockers returns every reason erg migrate --claims must write
// nothing, all of them, so one run lists the whole repair:
//
//   - a number two or more tickets share (one claim cannot hold two slugs);
//   - a claim migrate cannot fix: malformed, a symlink, or naming another
//     ticket's slug -- with the safe recovery for a partial claim an
//     interrupted write left uncommitted;
//   - a .claims that is not a plain directory;
//   - a claim path git ignores.
//
// byID is ticketPathsByID(store), walked once by the caller.
func migrateBlockers(store string, byID map[string][]string) []string {
	ids := sortedKeys(byID)
	var blockers []string
	for _, id := range ids {
		if paths := byID[id]; len(paths) > 1 {
			sort.Strings(paths)
			blockers = append(blockers, fmt.Sprintf("%s is held by %d tickets: %s -- renumber all but one clear of the frontier ('erg integration')",
				id, len(paths), strings.Join(paths, ", ")))
		}
	}
	switch err := requireClaims(store); {
	case err == nil:
		claims, malformed, errs := readClaims(store)
		blockers = append(blockers, errs...)
		for _, name := range sortedKeys(malformed) {
			path := filepath.Join(claimsDir(store), name)
			if fi, lerr := os.Lstat(path); lerr == nil && fi.Mode().IsRegular() && claimUntracked(store, name) {
				blockers = append(blockers, fmt.Sprintf("%s is not committed: a partial write from an interrupted erg? rm %s, then re-run", path, path))
			}
		}
		for _, id := range ids {
			paths := byID[id]
			if len(paths) != 1 {
				continue
			}
			slug := ticketSlug(filepath.Base(paths[0]))
			if claimed, ok := claims[id]; ok && claimed != slug {
				blockers = append(blockers, slugMismatchMessage(paths[0], filepath.Join(claimsDir(store), id), claimed, slug))
			}
		}
	case !errors.Is(err, errNoClaims):
		blockers = append(blockers, fmt.Sprintf("%s: %v", claimsDir(store), err))
	}
	probe := append([]string{claimsKeepName}, ids...)
	for _, rule := range ignoredClaimRules(store, probe...) {
		blockers = append(blockers, rule.message())
	}
	return blockers
}

// migrateClaims brings store to the required claims: it creates .claims/
// (with .keep) and writes a claim, with a nonce derived from the ticket
// (migrationNonce), for every ticket number, open and closed, that has none.
// It first collects every blocker (migrateBlockers) and, on any, writes
// nothing and returns them all. Idempotent: an existing claim is never
// rewritten, and one naming the ticket's slug counts as kept. So the same
// call repairs a compliant store whose tickets lack claims.
//
// byID is ticketPathsByID(store), walked once by the caller.
func migrateClaims(store string, byID map[string][]string) (written, kept int, blockers []string, err error) {
	if blockers := migrateBlockers(store, byID); len(blockers) > 0 {
		return 0, 0, blockers, nil
	}
	if err := ensureClaimsDir(store); err != nil {
		return 0, 0, nil, err
	}
	ids := sortedKeys(byID)
	for _, id := range ids {
		path := byID[id][0]
		slug := ticketSlug(filepath.Base(path))
		// Parse errors are erg check's to report, not migrate's: a ticket
		// without a Created header hashes an empty date, which is still the
		// same bytes in every clone, so independent migrations still agree.
		t, _ := parseErg(path)
		werr := writeClaim(store, id, slug, migrationNonce(id, slug, t.Created))
		switch {
		case werr == nil:
			written++
		case os.IsExist(werr):
			// The precheck read this claim and found the slug; a claim that
			// appeared since is checked again rather than counted blindly.
			data, rerr := os.ReadFile(filepath.Join(claimsDir(store), id))
			if f := strings.Fields(string(data)); rerr != nil || len(f) == 0 || f[0] != slug {
				return written, kept, nil, fmt.Errorf("claim %s appeared during the run and does not name %q", id, slug)
			}
			kept++
		default:
			return written, kept, nil, fmt.Errorf("claim %s: %w", id, werr)
		}
	}
	return written, kept, nil, nil
}

// unclaimedTickets returns the paths of the tickets in dir (which holds its
// .claims/) whose number has no claim file, sorted. A number two tickets
// share is left out: the duplicate-ID rule owns it. Names only: one walk of
// the store and one read of .claims/, no ticket parsed and no claim read, so
// a write verb pays almost nothing for it. A malformed claim still exists,
// so its ticket is not listed here; erg check reports it.
func unclaimedTickets(dir string) []string {
	byID, err := ticketPathsByID(dir)
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(claimsDir(dir))
	if err != nil {
		return nil
	}
	claimed := make(map[string]bool, len(entries))
	for _, e := range entries {
		claimed[e.Name()] = true
	}
	var out []string
	for id, paths := range byID {
		if len(paths) == 1 && !claimed[id] {
			out = append(out, paths[0])
		}
	}
	sort.Strings(out)
	return out
}

// warnUnclaimed prints on stderr, for a write verb, the store's tickets that
// have no claim (at most five, then a count), or, in a store without
// .claims/, the migrate command. It never changes the verb's outcome:
// refusing close, rm or archive right after an 'erg sync' would break their
// callers (autoclose), and the warning reaches every clone that uses erg,
// with or without hooks, offline. Silent in a compliant store.
func warnUnclaimed(verb, dir string) {
	// close and rm given a FILE inside closed/ resolve closed/ as their
	// directory; the store, and its .claims/, is its parent.
	// A store whose own directory is named closed keeps its name: only a
	// closed/ without .claims/ is taken for a store's subdirectory.
	if c := filepath.Clean(dir); filepath.Base(c) == "closed" && requireClaims(c) != nil {
		dir = filepath.Dir(c)
	}
	if err := requireClaims(dir); err != nil {
		fmt.Fprintf(os.Stderr, "%s: warning: %v\n", verb, err)
		return
	}
	paths := unclaimedTickets(dir)
	if len(paths) == 0 {
		return
	}
	names := make([]string, 0, 5)
	for i, p := range paths {
		if i == 5 {
			break
		}
		names = append(names, filepath.Base(p))
	}
	more := ""
	if len(paths) > 5 {
		more = fmt.Sprintf(" and %d more", len(paths)-5)
	}
	word := "tickets"
	if len(paths) == 1 {
		word = "ticket"
	}
	fmt.Fprintf(os.Stderr, "%s: warning: %d %s without a claim: %s%s -- run: erg migrate --claims %s\n",
		verb, len(paths), word, strings.Join(names, ", "), more, dir)
}
