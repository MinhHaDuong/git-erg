package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const magicLine = "%erg v1"

// RefKind discriminates the two Blocked-by reference forms defined in
// rules/tickets.md.
type RefKind int

const (
	RefInvalid RefKind = iota
	RefLocal           // 0042 — local ticket ID
	RefForge           // host/owner/repo#N — forge issue
)

// Ref is a parsed Blocked-by value. Downstream code (validator, ready)
// must read these fields rather than re-parse Raw — a single parser is
// the source of truth.
type Ref struct {
	Raw    string  // original text as written in the .erg file
	Kind   RefKind
	ID     string  // 4-digit ticket ID (RefLocal only)
	Host   string  // hostname (RefForge only)
	Owner  string  // owner/org (RefForge only)
	Repo   string  // repo name (RefForge only)
	Number string  // issue number (RefForge only)
}

// IsForge reports whether the ref targets a forge issue (offline-unknown).
func (r Ref) IsForge() bool {
	return r.Kind == RefForge
}

// parseRef parses a Blocked-by value into a Ref, or returns a precise
// error naming the failure mode. Stays purely syntactic — no network,
// no ticket-existence check.
func parseRef(raw string) (Ref, error) {
	if raw == "" {
		return Ref{Raw: raw}, fmt.Errorf("empty ref")
	}
	// Local: exactly 4 ASCII digits.
	if len(raw) == 4 && allDigits(raw) {
		return Ref{Raw: raw, Kind: RefLocal, ID: raw}, nil
	}

	// Reject old gh: and gh# forms.
	if strings.HasPrefix(raw, "gh:") {
		return Ref{Raw: raw}, fmt.Errorf(
			"forge ref %q uses deprecated 'gh:' scheme; use 'host/owner/repo#N' instead", raw)
	}
	if strings.HasPrefix(raw, "gh#") {
		return Ref{Raw: raw}, fmt.Errorf(
			"forge ref %q uses deprecated 'gh#' scheme; same-repo refs are not supported", raw)
	}

	// Catch case-variant old schemes before forge parsing.
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "gh#") || strings.HasPrefix(lower, "gh:") {
		return Ref{Raw: raw}, fmt.Errorf(
			"forge ref %q: scheme is case-sensitive (use lowercase 'gh' if intentional, or 'host/owner/repo#N')", raw)
	}

	// Forge: host "/" owner "/" repo "#" number.
	// Parse host, owner, repo from the part before the # sign.
	hashIdx := strings.LastIndexByte(raw, '#')
	if hashIdx > 0 {
		hostOwnerRepo := raw[:hashIdx]
		num := raw[hashIdx+1:]

		// Split host / owner / repo.
		parts := strings.Split(hostOwnerRepo, "/")
		if len(parts) == 3 {
			host, owner, repo := parts[0], parts[1], parts[2]
			if host != "" && owner != "" && repo != "" && !strings.Contains(host, ":") {
				// Validate the number format.
				if err := validateIssueNumber(num); err != nil {
					return Ref{Raw: raw}, fmt.Errorf("malformed ref %q: %v", raw, err)
				}
				return Ref{Raw: raw, Kind: RefForge, Host: host, Owner: owner, Repo: repo, Number: num}, nil
			}
		}
	}

	return Ref{Raw: raw}, fmt.Errorf(
		"malformed ref %q: not a 4-digit local ID or host/owner/repo#N", raw)
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// validateIssueNumber enforces the [1-9][0-9]* rule from rules/tickets.md
// (positive integer, no leading zero).
func validateIssueNumber(num string) error {
	if num == "" {
		return fmt.Errorf("missing issue number")
	}
	if !allDigits(num) {
		return fmt.Errorf("issue number %q is not a positive integer", num)
	}
	if num[0] == '0' {
		return fmt.Errorf("issue number %q has a leading zero", num)
	}
	return nil
}

// Erg is a parsed %erg v1 ticket file.
type Erg struct {
	Path     string
	Headers  map[string][]string // repeatable headers
	LogLines []string
	Body     string
	HasMagic bool
	HasLog   bool
	HasBody  bool
	// Separator occurrence counts. A well-formed ticket has exactly 1 of each.
	LogSepCount  int
	BodySepCount int
	// ClosedInLog/ClosedInBody record whether a `Closed:` header-key line
	// was seen outside the preamble (validator rejects).
	ClosedInLog  bool
	ClosedInBody bool
}

// Title returns the first Title header value, or "" if absent.
func (t *Erg) Title() string {
	if vs, ok := t.Headers["Title"]; ok && len(vs) > 0 {
		return vs[0]
	}
	return ""
}

// Closed reports whether the ticket is closed under the v1 criterion:
// either a path component test fires, or a `Closed:` preamble header is
// present with a non-empty value.
func (t *Erg) Closed() bool {
	if pathIsClosed(t.Path) {
		return true
	}
	if vs, ok := t.Headers["Closed"]; ok {
		for _, v := range vs {
			if strings.TrimSpace(v) != "" {
				return true
			}
		}
	}
	return false
}

// pathIsClosed implements the path component test from rules/tickets.md:
// any path component (directory name or basename without extension) that
// equals "closed", starts with "closed-"/"closed.", or ends with "-closed"
// (case-insensitive) marks the ticket closed.
func pathIsClosed(path string) bool {
	if path == "" {
		return false
	}
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == len(parts)-1 {
			part = strings.TrimSuffix(part, filepath.Ext(part))
		}
		lower := strings.ToLower(part)
		if lower == "closed" ||
			strings.HasPrefix(lower, "closed-") ||
			strings.HasPrefix(lower, "closed.") ||
			strings.HasSuffix(lower, "-closed") {
			return true
		}
	}
	return false
}

// BlockedBy returns all Blocked-by header values verbatim.
func (t *Erg) BlockedBy() []string {
	if vs, ok := t.Headers["Blocked-by"]; ok {
		return vs
	}
	return nil
}

// Tags returns all normalized Tags header values.
func (t *Erg) Tags() []string {
	vs, ok := t.Headers["Tags"]
	if !ok || len(vs) == 0 {
		return nil
	}
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		t := strings.TrimSpace(v)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// BlockedByRefs parses every Blocked-by header value and returns refs
// aligned with parse errors by index. A nil error means a successful
// parse; a non-nil error means the corresponding ref is RefInvalid and
// the validator will reject it. Downstream callers (ready) treat invalid
// refs as not-yet-known and skip them — by the time tickets are
// committed, the validator has already rejected any malformed ref.
func (t *Erg) BlockedByRefs() ([]Ref, []error) {
	raws := t.BlockedBy()
	if len(raws) == 0 {
		return nil, nil
	}
	refs := make([]Ref, len(raws))
	errs := make([]error, len(raws))
	for i, raw := range raws {
		refs[i], errs[i] = parseRef(raw)
	}
	return refs, errs
}

// Filename returns the basename of the ticket path.
func (t *Erg) Filename() string {
	return filepath.Base(t.Path)
}

// FilenameID extracts the numeric prefix from the filename (e.g., "0042" from "0042-add-auth.erg").
func (t *Erg) FilenameID() string {
	stem := strings.TrimSuffix(t.Filename(), ".erg")
	if idx := strings.Index(stem, "-"); idx > 0 {
		return stem[:idx]
	}
	return stem
}

func isLetter(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func isAlphanumeric(c byte) bool {
	return isLetter(c) || (c >= '0' && c <= '9')
}

// parseHeaderLine extracts "Key: value" from a line.
func parseHeaderLine(line string) (string, string, bool) {
	if len(line) == 0 || !isLetter(line[0]) {
		return "", "", false
	}
	colonPos := -1
	for i := 1; i < len(line); i++ {
		c := line[i]
		if c == ':' {
			colonPos = i
			break
		}
		if isAlphanumeric(c) || c == '_' || c == '-' {
			continue
		}
		if c == ' ' || c == '\t' {
			for j := i; j < len(line); j++ {
				if line[j] == ':' {
					colonPos = j
					break
				}
				if line[j] != ' ' && line[j] != '\t' {
					return "", "", false
				}
			}
			break
		}
		return "", "", false
	}
	if colonPos < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:colonPos])
	val := strings.TrimSpace(line[colonPos+1:])
	return key, val, true
}

// parseErg parses a single .erg file. On read error, returns an empty Erg with
// only Path and Headers set so callers can still report a filename.
func parseErg(path string) Erg {
	data, err := os.ReadFile(path)
	if err != nil {
		return Erg{Path: path, Headers: make(map[string][]string)}
	}
	lines := strings.Split(string(data), "\n")

	headers := make(map[string][]string)
	var logLines, bodyLines []string
	section := "magic" // magic | headers | gap | log | body
	hasMagic := false
	hasLog := false
	hasBody := false
	logSepCount := 0
	bodySepCount := 0
	closedInLog := false
	closedInBody := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// First non-empty line must be the magic line
		if section == "magic" {
			if trimmed == "" {
				continue
			}
			if trimmed == magicLine {
				hasMagic = true
				section = "headers"
				continue
			}
			// No magic line — try to parse as old format
			section = "headers"
			// Fall through to header parsing
		}

		if trimmed == "--- log ---" {
			logSepCount++
			if !hasBody {
				section = "log"
				hasLog = true
				continue
			}
		}
		if trimmed == "--- body ---" {
			bodySepCount++
			if !hasBody {
				section = "body"
				hasBody = true
				continue
			}
		}

		switch section {
		case "headers":
			if trimmed == "" {
				section = "gap"
				continue
			}
			if key, val, ok := parseHeaderLine(line); ok {
				headers[key] = append(headers[key], val)
			}
		case "gap":
			// ignore lines between header block and log separator
		case "log":
			if trimmed != "" {
				logLines = append(logLines, line)
			}
			if isClosedHeaderLine(line) {
				closedInLog = true
			}
		case "body":
			bodyLines = append(bodyLines, line)
			if isClosedHeaderLine(line) {
				closedInBody = true
			}
		}
	}

	return Erg{
		Path:         path,
		Headers:      headers,
		LogLines:     logLines,
		Body:         strings.Join(bodyLines, "\n"),
		HasMagic:     hasMagic,
		HasLog:       hasLog,
		HasBody:      hasBody,
		LogSepCount:  logSepCount,
		BodySepCount: bodySepCount,
		ClosedInLog:  closedInLog,
		ClosedInBody: closedInBody,
	}
}

// isClosedHeaderLine reports whether a line begins with the literal
// header key `Closed:` at the start of the line (no leading whitespace).
// Used to detect misplaced `Closed:` keys in the log or body sections.
// This is a header-key match, not a free substring match — `disclosed`,
// indented examples, and prose mentions never trigger.
func isClosedHeaderLine(line string) bool {
	const key = "Closed:"
	return len(line) >= len(key) && line[:len(key)] == key
}

// loadErgs parses every .erg file under dir recursively, sorted by path.
func loadErgs(dir string) []Erg {
	var tickets []Erg
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(path, ".erg") {
			return nil
		}
		tickets = append(tickets, parseErg(path))
		return nil
	})
	if err != nil {
		return nil
	}
	sort.Slice(tickets, func(i, j int) bool {
		return tickets[i].Path < tickets[j].Path
	})
	return tickets
}

// jsonEscape escapes a string for inclusion in a double-quoted JSON value.
func jsonEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

func sortedKeys2[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
