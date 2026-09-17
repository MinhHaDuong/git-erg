package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// syncRemote is the adopter project's default remote. The default mode keeps a
// clone aligned with the binary that project reviewed and committed.
const syncRemote = "origin"

// gitErgUpstream is deliberately reached only through `erg sync --upstream`.
// The explicit flag matters: unlike the default project-origin mode, this
// imports an executable that the adopter repository has not yet reviewed.
const gitErgUpstream = "https://github.com/MinhHaDuong/git-erg.git"

// summarySync is the one-liner printed by printUsage via the commands registry.
const summarySync = "Sync this project's vendored binary from its origin"

const helpSync = `## erg sync [--upstream]

Synchronize this project's vendored binary with a committed binary fetched through git.

By default, sync reads tickets/erg from the current PROJECT'S origin. It aligns clones
with the version vendored and reviewed by that project. It does NOT check whether the
git-erg project has published a newer binary.

Use --upstream to import tickets/erg explicitly from the git-erg project. Review and
commit the resulting binary in the adopter project so its other clones can use the
default project-origin mode. ERG_UPDATE_URL and the .ergrc [update] url key remain
custom-source overrides when --upstream is absent; the environment wins over config.
The explicit --upstream flag wins over both overrides.

Project-origin mode reads the binary at the adopter's local store-relative path.
Upstream and configured sources instead read canonical tickets/erg, because their
repository layout is independent of the adopter's local store name. Both fetch only
the source tip needed for that blob rather than importing the source's full history.

Sync uses git (already a dependency of git-erg) -- never an embedded network client --
so the binary carries no network code. The project-origin fetch runs in the ticket
store's repository: those objects are the adopter's own. Every other source (--upstream
or a configured one) is fetched shallowly in a private throwaway bare repository under
the ticket store, so all writes stay confined there and the adopter repository gains no
foreign objects, no FETCH_HEAD and no shallow marking. The source is resolved in the
adopter's repository first, so a repo-local url.<mirror>.insteadOf or a remote name is
honoured; the resolved URL is handed to git and never printed. Sync extracts the
committed binary at the source's default branch and compares its hash to the vendored
binary at <ticket store>/erg. The executable used to invoke sync is never replaced, so
a system-native erg remains intact.

Messages name the resolved source, sanitizing configured URLs and suppressing git's
raw diagnostics so credentials cannot be echoed. If the hash differs, sync replaces
the vendored binary atomically (exclusive temp file, fsync, then rename).

Transport and environmental errors exit 0 so that 'erg sync && erg validate' chains do
not fail in offline or isolated environments (no remote configured, no network, not a
git repo, ticket store not writable).
If the trusted project origin lacks its store-relative binary, sync warns and also exits
0: the file may simply be gitignored or not committed yet. A configured source or the
explicit upstream missing canonical tickets/erg is a hard error instead. If no ticket
store is found, sync does nothing and exits 0 -- it never pulls the binary from an
unrelated repository you happen to be standing in.

After a successful project-origin sync, checks whether any .erg files in the ticket store
still carry legacy Status: headers. If found, prints explicit migration guidance:
'erg migrate DIR', 'git diff tickets/', 'git commit'. The sync command never mutates
ticket files itself -- migration is a separate, reviewable step.

erg sync replaces the binary only -- it never writes or modifies any managed store file
(.ergrc, AGENTS.md, or tickets). Its private temporary git directory (.erg-sync-*) is
removed after the fetch; if that removal fails, sync warns and the next run sweeps any
such directory left directly under the store. erg check and erg validate ignore it.
Embedded-asset changes and new default label vocabulary are delivered by a follow-up
'erg init'. On Linux x86-64, invoke the vendored binary for that follow-up so init uses
the bytes just synchronized:

  erg sync
  tickets/erg init

The explicit import sequence keeps review ahead of first execution:

  erg sync --upstream
  git diff -- tickets/erg
  # review or verify the imported binary here
  tickets/erg init
  git diff -- tickets/
  git commit

An upstream or configured-source import is not executed automatically. Review it first,
then run '<ticket store>/erg check' before init. Project-origin sync may run that check
automatically because those bytes are the project's already-reviewed vendored version.

The vendored binary is always Linux x86-64. On macOS, Windows, or another architecture,
'erg sync' still updates that project/CI artifact but you must update or rebuild your
native system erg from the same reviewed git-erg revision before running 'erg init'.

erg init applies the dpkg-style 3-state rule: byte-identical files are left untouched;
a file that matches the previously recorded stock hash is a clean upgrade and is
overwritten; a locally-edited file is preserved (exit 2). A file the stamp says a
NEWER erg wrote is preserved too, so an init run from a stale binary reports the
situation instead of reverting the store. Running erg sync alone is never
sufficient to absorb new defaults.

Compatibility spellings: 'erg update' is an alias for 'erg sync' (it prints a notice),
and ERG_UPDATE_URL and the .ergrc [update] section keep the old verb. All three remain
accepted and will be removed together in a future major version; the environment
variable and the config key have no new spelling yet, so keep using them.
`

type syncSource struct {
	remote               string
	label                string
	trustedProjectOrigin bool
}

type remoteBinaryError struct {
	unavailable bool
	err         error
}

func (e *remoteBinaryError) Error() string { return e.err.Error() }
func (e *remoteBinaryError) Unwrap() error { return e.err }

// resolveSyncSource applies CLI > environment > config > default precedence.
// Labels identify the resolved source while sanitizing custom URLs, which may
// contain credentials in userinfo, query parameters, or fragments.
func resolveSyncSource(upstream bool, envRemote, configRemote string) syncSource {
	if upstream {
		return syncSource{remote: gitErgUpstream, label: "git-erg upstream"}
	}
	if envRemote != "" {
		return syncSource{remote: envRemote, label: "configured source (ERG_UPDATE_URL: " + safeRemoteIdentity(envRemote) + ")"}
	}
	if configRemote != "" {
		return syncSource{remote: configRemote, label: "configured source (tickets/.ergrc: " + safeRemoteIdentity(configRemote) + ")"}
	}
	return syncSource{remote: syncRemote, label: "project origin", trustedProjectOrigin: true}
}

// safeRemoteIdentity gives operators a stable fingerprint to distinguish
// sources without reproducing arbitrary user input. URLs additionally expose
// only their host; paths, remote names, userinfo, queries, and fragments may
// all contain secrets and are never echoed.
func safeRemoteIdentity(remote string) string {
	sum := sha256.Sum256([]byte(remote))
	id := hex.EncodeToString(sum[:])[:12]
	if scheme := strings.Index(remote, "://"); scheme >= 0 {
		authority := remote[scheme+3:]
		if end := strings.IndexAny(authority, "/?#"); end >= 0 {
			authority = authority[:end]
		}
		if at := strings.LastIndex(authority, "@"); at >= 0 {
			authority = authority[at+1:]
		}
		if authority != "" {
			return fmt.Sprintf("host %s, id %s", authority, id)
		}
	}
	if strings.Contains(remote, "/") || strings.Contains(remote, `\`) || strings.HasPrefix(remote, ".") {
		return "local path, id " + id
	}
	if at, colon := strings.LastIndex(remote, "@"), strings.Index(remote, ":"); at >= 0 && colon > at {
		return fmt.Sprintf("host %s, id %s", remote[at+1:colon], id)
	}
	return "named remote, id " + id
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

// syncTempPrefix names the throwaway bare repository an isolated fetch uses,
// directly under the ticket store. sweepStaleSyncDirs removes leftovers.
const syncTempPrefix = ".erg-sync-"

// fetchRemoteBinary fetches the default branch of remote into FETCH_HEAD and
// returns the bytes of the committed binary at blobPath (a path relative to the
// repository root). The trusted project origin fetches in the adopter's own
// repository: its objects are the adopter's own. Every other source fetches
// shallowly in a throwaway bare repository under the ticket store, so it can
// neither leave the adopter repository shallow nor import foreign objects.
// The remote is resolved in the adopter's repository first (ls-remote
// --get-url applies url.*.insteadOf and remote names without touching the
// network), because the throwaway repository has no repo-local config. That
// resolved value is user input that may carry credentials: it is passed to git
// only and never reaches a message.
// No network client is embedded. Raw git stderr is intentionally suppressed
// because git may echo a source URL containing credentials.
func fetchRemoteBinary(gitDir, remote, blobPath string, isolate bool) ([]byte, error) {
	fetchDir := gitDir
	args := []string{"fetch", "--quiet"}
	if isolate {
		remote = resolveRemoteURL(gitDir, remote)
		tmp, err := os.MkdirTemp(gitDir, syncTempPrefix)
		if err != nil {
			return nil, &remoteBinaryError{unavailable: true, err: fmt.Errorf("could not create temporary git directory: %w", err)}
		}
		defer func() {
			if err := os.RemoveAll(tmp); err != nil {
				fmt.Fprintf(os.Stderr, "sync: warning: could not remove temporary git repository %s: %v -- the next sync sweeps it\n", tmp, err)
			}
		}()
		if err := exec.Command("git", "-C", tmp, "init", "--quiet", "--bare").Run(); err != nil {
			return nil, &remoteBinaryError{unavailable: true, err: fmt.Errorf("could not initialize temporary git repository: %w", err)}
		}
		fetchDir = tmp
		args = append(args, "--depth=1")
	}

	fetch := exec.Command("git", append([]string{"-C", fetchDir}, append(args, remote, "HEAD")...)...)
	if err := fetch.Run(); err != nil {
		return nil, &remoteBinaryError{unavailable: true, err: fmt.Errorf("git fetch failed: %w", err)}
	}

	show := exec.Command("git", "-C", fetchDir, "cat-file", "blob", "FETCH_HEAD:"+blobPath)
	var out bytes.Buffer
	show.Stdout = &out
	if err := show.Run(); err != nil {
		return nil, &remoteBinaryError{err: fmt.Errorf("git could not read the committed vendored binary: %w", err)}
	}
	return out.Bytes(), nil
}

// resolveRemoteURL returns the URL git would fetch for remote when run in
// gitDir: a remote name becomes its URL and url.*.insteadOf rewrites apply.
// A relative local path is anchored where git would have resolved it in
// gitDir (the work tree root, else gitDir itself), so it keeps its meaning
// when the fetch runs elsewhere. The result is for git only; it may carry
// credentials and must not be echoed.
func resolveRemoteURL(gitDir, remote string) string {
	cmd := exec.Command("git", "-C", gitDir, "ls-remote", "--get-url", remote)
	cmd.Stderr = nil
	out, err := cmd.Output()
	resolved := strings.TrimSpace(string(out))
	if err != nil || resolved == "" {
		resolved = remote
	}
	if !strings.Contains(resolved, ":") && !filepath.IsAbs(resolved) {
		base := gitToplevel(gitDir)
		if base == "" {
			base = gitDir
		}
		if info, statErr := os.Stat(filepath.Join(base, resolved)); statErr == nil && info.IsDir() {
			resolved = filepath.Join(base, resolved)
		}
	}
	return resolved
}

// sweepStaleSyncDirs removes throwaway fetch repositories a killed sync left
// under the ticket store: only directories carrying syncTempPrefix, directly
// under dir. A leftover is a bare repository inside the adopter's work tree,
// which a careless `git add -A` would commit.
func sweepStaleSyncDirs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), syncTempPrefix) {
			_ = os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
}

// canRunTravelingBinary reports whether the host can execute the one committed
// traveler artifact. Other platforms use a native system erg and must never
// attempt to execute the freshly installed Linux binary.
func canRunTravelingBinary(goos, goarch string) bool {
	return goos == "linux" && goarch == "amd64"
}

// cmdSync implements `erg sync`. See helpSync for the user-facing summary.
func cmdSync(args []string) int {
	upstream := false
	for _, a := range args {
		switch a {
		case "--upstream":
			upstream = true
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintf(os.Stderr, "sync: unknown flag %q\nUsage: erg sync [--upstream]\n", a)
			} else {
				fmt.Fprintf(os.Stderr, "sync: unexpected argument %q\nUsage: erg sync [--upstream]\n", a)
			}
			return 1
		}
	}
	// Locate the ticket store and, through it, the repository that carries the
	// committed binary. The store dir is also where the post-sync migration
	// scan runs, so resolve it once.
	ticketDir := os.Getenv("ERG_TICKET_DIR")
	if ticketDir == "" {
		if d, findErr := findTicketsDir(); findErr == nil {
			ticketDir = d
		}
	}
	// Anchor the fetch to a resolved ticket store. Without one we have no
	// trustworthy notion of "this project's repo", and falling back to the
	// current directory's git repo would let `erg sync` silently pull the
	// binary from whatever unrelated repo you happen to be standing in. Refuse
	// and leave the binary untouched (exit 0, so sync && validate still works).
	if ticketDir == "" {
		fmt.Fprintln(os.Stderr,
			"sync: no git-erg ticket store found here -- run from inside your "+
				"git-erg repo, or set ERG_TICKET_DIR. Binary left unchanged.")
		return 0
	}
	target := filepath.Join(ticketDir, "erg")
	localHash, err := selfHash(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync: cannot hash vendored binary %s: %v\n", target, err)
		return 1
	}

	var configRemote string
	if os.Getenv("ERG_UPDATE_URL") == "" {
		if cfg, cfgErr := loadConfig(ticketDir); cfgErr == nil && cfg != nil {
			configRemote = cfg.UpdateURL
		}
	}
	source := resolveSyncSource(upstream, os.Getenv("ERG_UPDATE_URL"), configRemote)

	// The repo's committed binary lives at <ticket store>/erg. Translate that
	// to a repo-root-relative path for `git cat-file blob FETCH_HEAD:<path>`.
	// If the store is not inside a git repo, blobPath is moot -- the fetch below
	// fails (not a repo) and we exit 0, leaving the binary in place.
	blobPath := "tickets/erg"
	if source.trustedProjectOrigin {
		if top := gitToplevel(ticketDir); top != "" {
			if abs, absErr := filepath.Abs(ticketDir); absErr == nil {
				if rel, relErr := filepath.Rel(top, filepath.Join(abs, "erg")); relErr == nil {
					blobPath = filepath.ToSlash(rel)
				}
			}
		}
	}

	sweepStaleSyncDirs(ticketDir)
	body, err := fetchRemoteBinary(ticketDir, source.remote, blobPath, !source.trustedProjectOrigin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync: could not synchronize from %s -- %v\n", source.label, err)
		var remoteErr *remoteBinaryError
		if errors.As(err, &remoteErr) && !remoteErr.unavailable && !source.trustedProjectOrigin {
			return 1
		}
		return 0
	}
	if len(body) == 0 {
		fmt.Fprintf(os.Stderr, "sync: %s supplied an empty binary -- leaving current in place\n", source.label)
		return 0
	}

	sum := sha256.Sum256(body)
	remoteHash := hex.EncodeToString(sum[:])

	if localHash == remoteHash {
		fmt.Printf("erg: already synchronized with %s\n", source.label)
		return 0
	}

	if err := atomicWriteFile(target, body, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "sync: cannot replace binary: %v\n", err)
		return 1
	}

	fmt.Printf("erg: synchronized with %s (%s \u2192 %s)\n", source.label, localHash[:12], remoteHash[:12])
	targetCommand := shellSingleQuote(target)

	if !source.trustedProjectOrigin {
		fmt.Printf("erg: imported %s without executing it; review it before first use\n", source.label)
		return 0
	}
	hostCanRunTraveler := canRunTravelingBinary(runtime.GOOS, runtime.GOARCH)
	if !hostCanRunTraveler {
		fmt.Printf("erg: synchronized the Linux x86-64 traveler without executing it on %s/%s; use an updated native erg for check/init\n", runtime.GOOS, runtime.GOARCH)
	}

	// Detect tickets still carrying `Status:` headers and emit a hint.
	// Migration is explicit: the user runs `erg migrate`, reviews the diff,
	// and commits separately. erg sync never mutates ticket files.
	if info, err := os.Stat(ticketDir); err == nil && info.IsDir() && hasStatusHeader(ticketDir) {
		fmt.Printf("erg: detected Status: headers in %s -- run:\n", ticketDir)
		if hostCanRunTraveler {
			fmt.Printf("  %s migrate %s\n", targetCommand, shellSingleQuote(ticketDir))
		} else {
			fmt.Printf("  # after updating native erg: erg migrate %s\n", shellSingleQuote(ticketDir))
		}
		fmt.Println("  git diff tickets/")
		fmt.Println("  git commit -m 'chore: migrate to Closed: header'")
	}

	// Post-swap asset-condition hint (tickets 0212, 0283). This still-running
	// process is the OLD binary, so it cannot read the NEW binary's embedded
	// assets directly; instead it re-execs the freshly-swapped binary's own
	// `erg check`, whose asset detection compares against the NEW embedded asset
	// (charter 4c, re-exec approach). Attempted whenever the ticket dir exists.
	//
	// It used to be gated on a manifest existing, on the premise that without
	// one there was nothing to compare -- and that premise is exactly what kept
	// a store with no provenance silent on every channel (ticket 0283). Without
	// a manifest there IS something to compare: the on-disk bytes against the
	// embedded copy. Do not re-derive the old gate from a stale comment; the
	// decision of what is comparable belongs to assetDriftWarnings, and this
	// site's job is only to relay what the new binary reports.
	if hostCanRunTraveler {
		out, _ := exec.Command(target, "check", ticketDir).CombinedOutput()
		// Two conditions, two remedies: drift is "refresh what is stale",
		// stampless is "record what is unrecorded". They are independent, so
		// both may fire in one run (different assets).
		if strings.Contains(string(out), assetDriftSignal) {
			fmt.Printf("erg: deployed assets are from an earlier rev -- run %s init to refresh them.\n", targetCommand)
		}
		if strings.Contains(string(out), assetStamplessSignal) {
			// Still does NOT say "run erg init to establish provenance". Init
			// no longer stamps a preserved edit as if shipped (ticket 0292
			// fixed that), but it still does not RESOLVE anything: it
			// preserves the file and leaves the divergence exactly where it
			// was. What resolves it is a human comparison, so this names the
			// two commands that make one possible.
			//
			// Safe to advise --show even though this line is printed by the
			// OLD binary: the swap has already happened, so the erg the reader
			// runs next is the new one. The grep above is the reverse
			// direction and is why assetStamplessSignal itself may only ever
			// be extended at its end.
			fmt.Printf("erg: deployed assets carry no .erg-assets stamp -- run %s check to see which, then %s init --show NAME to compare each against the shipped copy before %s init.\n", targetCommand, targetCommand, targetCommand)
		}
	}
	return 0
}
