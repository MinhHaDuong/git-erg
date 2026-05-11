package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Erg is a parsed %erg v1 ticket file.
type Erg struct {
	Path     string
	headers  map[string][]string // repeatable headers (unexported; use accessor methods)
	LogLines []string
	Body     string
	HasMagic bool
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
	if vs, ok := t.headers["Title"]; ok && len(vs) > 0 {
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
	if vs, ok := t.headers["Closed"]; ok {
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
	if vs, ok := t.headers["Blocked-by"]; ok {
		return vs
	}
	return nil
}

// Tags returns all normalized Tags header values.
func (t *Erg) Tags() []string {
	vs, ok := t.headers["Tags"]
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

// ClosedHeader reports whether the ticket has a non-empty Closed: header.
// This is a pure header test — it does NOT check the ticket path.
// Use Closed() when path-based closure (tickets/closed/ directory) must
// also be considered.
func (t *Erg) ClosedHeader() bool {
	if vs, ok := t.headers["Closed"]; ok {
		for _, v := range vs {
			if strings.TrimSpace(v) != "" {
				return true
			}
		}
	}
	return false
}

// Created returns the first Created header value, or "" if absent.
func (t *Erg) Created() string {
	if vs, ok := t.headers["Created"]; ok && len(vs) > 0 {
		return vs[0]
	}
	return ""
}

// Author returns the first Author header value, or "" if absent.
func (t *Erg) Author() string {
	if vs, ok := t.headers["Author"]; ok && len(vs) > 0 {
		return vs[0]
	}
	return ""
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
		return Erg{Path: path, headers: make(map[string][]string)}
	}
	lines := strings.Split(string(data), "\n")

	headers := make(map[string][]string)
	var logLines, bodyLines []string
	section := "magic" // magic | headers | gap | log | body
	hasMagic := false
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
			if trimmed == MagicLine {
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
			if bodySepCount == 0 {
				section = "log"
				continue
			}
		}
		if trimmed == "--- body ---" {
			bodySepCount++
			if bodySepCount == 1 {
				section = "body"
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
		headers:      headers,
		LogLines:     logLines,
		Body:         strings.Join(bodyLines, "\n"),
		HasMagic:     hasMagic,
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
