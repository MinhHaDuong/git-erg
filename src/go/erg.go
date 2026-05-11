package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Line is a single-line string: no embedded newlines.
// Enforced by the parser invariant (header values read up to LF).
type Line = string

// Erg is the schema-literal projection of a %erg v1 ticket file.
type Erg struct {
	Path     Line
	HasMagic bool

	// v1 headers — typed fields populated from first occurrence
	Title      Line   // required, non-empty (validator rule 2)
	Created    Line   // required, non-empty
	Author     Line   // required, non-empty
	Closed     Line   // optional; first non-empty Closed: value when present
	BlockedBys []Line // possibly empty; one entry per `Blocked-by:` line, verbatim
	Tags       []Line // possibly empty; one entry per `Tag:` line, trimmed; empties skipped

	LogLines []Line // one structured event per entry
	Body     string // multiline

	// Legacy fields retained during the 0116 dual-write migration.
	// Removed in commit 4 of the ticket.
	headers      map[string][]string
	LogSepCount  int
	BodySepCount int
	ClosedInLog  bool
	ClosedInBody bool
}

// ParseDiagnostics carries parser observations the validator consumes.
// In 0117 (parse+validate merge) this struct is replaced in-place by
// []ParseError; until then, validate.go walks these fields.
type ParseDiagnostics struct {
	Unknown            []Line // unknown header keys seen
	RepeatedSingletons []Line // singleton keys seen more than once
	ClosedEmpty        bool   // ANY `Closed:` line seen with empty value
	ClosedInLog        bool
	ClosedInBody       bool
	HasLogSep          bool // `--- log ---` seen at least once
	HasBodySep         bool // `--- body ---` seen at least once
}

// IsClosed reports whether the ticket is closed under the v1 criterion:
// either a path component test fires, or a `Closed:` preamble header is
// present with a non-empty value.
func (t *Erg) IsClosed() bool {
	if pathIsClosed(t.Path) {
		return true
	}
	return t.Closed != ""
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

// BlockedByRefs parses every Blocked-by header value and returns refs
// aligned with parse errors by index. A nil error means a successful
// parse; a non-nil error means the corresponding ref is RefInvalid and
// the validator will reject it. Downstream callers (ready) treat invalid
// refs as not-yet-known and skip them — by the time tickets are
// committed, the validator has already rejected any malformed ref.
func (t *Erg) BlockedByRefs() ([]Ref, []error) {
	raws := t.BlockedBys
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

// v1HeaderKeys is the closed set of header keys recognised by parseErg.
// Used to classify header lines as known/unknown without depending on
// validate.go data (which 0116 commit 4 removes anyway).
var v1HeaderKeys = map[string]bool{
	"Title": true, "Created": true, "Author": true,
	"Closed": true, "Blocked-by": true, "Tag": true,
}

// v1SingletonKeys is the subset of v1HeaderKeys that may appear at most
// once in the preamble. Repeats are reported via ParseDiagnostics.
var v1SingletonKeys = map[string]bool{
	"Title": true, "Created": true, "Author": true, "Closed": true,
}

// parseErg parses a single .erg file. On read error, returns an empty Erg with
// only Path set so callers can still report a filename.
//
// During the 0116 migration the parser dual-writes: both the new typed
// fields and the legacy `headers` map / `LogSepCount` / `BodySepCount` /
// `ClosedInLog` / `ClosedInBody` fields are populated, and a
// ParseDiagnostics value is returned alongside the Erg. The legacy
// fields are removed in commit 4 of the ticket.
func parseErg(path string) (Erg, ParseDiagnostics) {
	var diag ParseDiagnostics
	data, err := os.ReadFile(path)
	if err != nil {
		return Erg{Path: path, headers: make(map[string][]string)}, diag
	}
	lines := strings.Split(string(data), "\n")

	headers := make(map[string][]string)
	var logLines, bodyLines []string
	var title, created, author, closed Line
	var blockedBys, tags []Line
	section := "magic" // magic | headers | gap | log | body
	hasMagic := false
	logSepCount := 0
	bodySepCount := 0
	closedInLog := false
	closedInBody := false
	unknownSeen := make(map[string]bool)
	repeatedSeen := make(map[string]bool)
	headerCounts := make(map[string]int)

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
			diag.HasLogSep = true
			if bodySepCount == 0 {
				section = "log"
				continue
			}
		}
		if trimmed == "--- body ---" {
			bodySepCount++
			diag.HasBodySep = true
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
				headerCounts[key]++
				// Classify header for diagnostics.
				if v1HeaderKeys[key] {
					if v1SingletonKeys[key] && headerCounts[key] == 2 {
						if !repeatedSeen[key] {
							diag.RepeatedSingletons = append(diag.RepeatedSingletons, key)
							repeatedSeen[key] = true
						}
					}
				} else {
					if !unknownSeen[key] {
						diag.Unknown = append(diag.Unknown, key)
						unknownSeen[key] = true
					}
				}
				// Populate typed fields. Singletons keep first occurrence.
				switch key {
				case "Title":
					if title == "" {
						title = val
					}
				case "Created":
					if created == "" {
						created = val
					}
				case "Author":
					if author == "" {
						author = val
					}
				case "Closed":
					tv := strings.TrimSpace(val)
					if tv == "" {
						diag.ClosedEmpty = true
					} else if closed == "" {
						closed = val
					}
				case "Blocked-by":
					blockedBys = append(blockedBys, val)
				case "Tag":
					tv := strings.TrimSpace(val)
					if tv != "" {
						tags = append(tags, tv)
					}
				}
			}
		case "gap":
			// ignore lines between header block and log separator
		case "log":
			if trimmed != "" {
				logLines = append(logLines, line)
			}
			if isClosedHeaderLine(line) {
				closedInLog = true
				diag.ClosedInLog = true
			}
		case "body":
			bodyLines = append(bodyLines, line)
			if isClosedHeaderLine(line) {
				closedInBody = true
				diag.ClosedInBody = true
			}
		}
	}

	return Erg{
		Path:         path,
		HasMagic:     hasMagic,
		Title:        title,
		Created:      created,
		Author:       author,
		Closed:       closed,
		BlockedBys:   blockedBys,
		Tags:         tags,
		LogLines:     logLines,
		Body:         strings.Join(bodyLines, "\n"),
		headers:      headers,
		LogSepCount:  logSepCount,
		BodySepCount: bodySepCount,
		ClosedInLog:  closedInLog,
		ClosedInBody: closedInBody,
	}, diag
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
		t, _ := parseErg(path)
		tickets = append(tickets, t)
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
