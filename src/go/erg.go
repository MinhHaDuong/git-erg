package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Line is a single-line string: no embedded newlines.
// Enforced by the parser invariant (header values read up to LF).
type Line = string

// Erg is the schema-literal projection of a %erg v1 ticket file.
// Lenient-parse invariant: parseErg always returns a usable Erg (at
// minimum with Path set) so callers can report a filename even when the
// file is unreadable or malformed. The validator (validateErg) decides
// which structural defects are errors.
type Erg struct {
	Path     Line
	HasMagic bool

	// v1 headers — typed fields populated from first occurrence
	Title      Line   // required, non-empty (validator rule 2)
	Created    Line   // required, non-empty
	Author     Line   // required, non-empty
	Closed     Line   // optional; first non-empty Closed: value when present
	BlockedBys []Line // possibly empty; one entry per `Blocked-by:` line, trimmed by parseHeaderLine
	Tags       []Line // possibly empty; one entry per `Tag:` line, trimmed; empties skipped

	LogLines []Line // one structured event per entry
	Body     string // multiline
}

// ParseDiagnostics carries parser observations the validator consumes.
// In 0117 (parse+validate merge) this struct collapses to a plain
// []string of error messages. Shape (c) transitional: Errors is
// populated alongside the legacy trace fields, and each subsequent
// commit migrates one rule from validateErg into parseErgBytes
// (emitting into Errors and zeroing the corresponding trace field).
// The struct is deleted once every per-file rule has migrated.
type ParseDiagnostics struct {
	Errors             []string // parse-time error messages (filename: msg)
	Unknown            []Line   // unknown header keys seen
	RepeatedSingletons []Line   // singleton keys seen more than once
	ClosedEmpty        bool     // ANY `Closed:` line seen with empty value
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

// Filename returns the basename of the ticket path. Always non-empty when
// Path is set (parseErg guarantees Path is set even on read errors).
func (t *Erg) Filename() string {
	return filepath.Base(t.Path)
}

// FilenameID extracts the numeric prefix from the filename (e.g., "0042"
// from "0042-add-auth.erg"). Returns the full stem when no dash is present,
// which may be empty or non-numeric — callers (close, archive, check) must
// guard against empty-string returns.
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

// parseHeaderLine extracts "Key: value" from a line. Accepts both "Key:"
// and "Key :" (whitespace before colon is tolerated). Both key and value
// are trimmed of surrounding whitespace before return.
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
// Used to classify header lines as known/unknown during parsing.
var v1HeaderKeys = map[string]bool{
	"Title": true, "Created": true, "Author": true,
	"Closed": true, "Blocked-by": true, "Tag": true,
}

// v1SingletonKeys is the subset of v1HeaderKeys that may appear at most
// once in the preamble. Repeats are reported via ParseDiagnostics.
var v1SingletonKeys = map[string]bool{
	"Title": true, "Created": true, "Author": true, "Closed": true,
}

// parseErg parses a single .erg file into an Erg plus parser observations
// for the validator (ParseDiagnostics). On read error, returns an empty
// Erg with only Path set so callers can still report a filename.
func parseErg(path string) (Erg, ParseDiagnostics) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Erg{Path: path}, ParseDiagnostics{}
	}
	return parseErgBytes(data, path)
}

// parseErgBytes parses raw .erg file content into an Erg plus parser
// observations. Callers that already hold the file bytes (e.g. after
// os.ReadFile for rewriting) use this to avoid a second read.
//
// Edge case: diag.HasLogSep and diag.HasBodySep are set to true on ANY
// sighting of the literal "--- log ---" or "--- body ---", including
// occurrences inside the body section (which are body text, not actual
// separators). The validator treats these flags as "at least one separator
// present" and does not distinguish genuine separators from quoted literals.
func parseErgBytes(data []byte, path string) (Erg, ParseDiagnostics) {
	var diag ParseDiagnostics
	lines := strings.Split(string(data), "\n")

	var logLines, bodyLines []string
	var title, created, author, closed Line
	var blockedBys, tags []Line
	section := "magic" // magic | headers | gap | log | body
	hasMagic := false
	bodySepSeen := false
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
			diag.HasLogSep = true
			if !bodySepSeen {
				section = "log"
				continue
			}
		}
		if trimmed == "--- body ---" {
			diag.HasBodySep = true
			if !bodySepSeen {
				bodySepSeen = true
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
				headerCounts[key]++
				// Classify header for diagnostics.
				if v1HeaderKeys[key] {
					if v1SingletonKeys[key] && headerCounts[key] == 2 {
						if !repeatedSeen[key] {
							diag.RepeatedSingletons = append(diag.RepeatedSingletons, key)
							repeatedSeen[key] = true
							diag.Errors = append(diag.Errors, fmt.Sprintf(
								"%s: header '%s' is non-repeatable (appears more than once)",
								filepath.Base(path), key))
						}
					}
				} else {
					if !unknownSeen[key] {
						diag.Unknown = append(diag.Unknown, key)
						unknownSeen[key] = true
						switch key {
						case "Status":
							diag.Errors = append(diag.Errors, fmt.Sprintf(
								"%s: 'Status:' header is no longer part of %%erg v1 — run `erg migrate` to convert",
								filepath.Base(path)))
						case "Tags":
							diag.Errors = append(diag.Errors, fmt.Sprintf(
								"%s: 'Tags:' has been renamed to 'Tag:' — run `erg migrate` to convert",
								filepath.Base(path)))
						default:
							diag.Errors = append(diag.Errors, fmt.Sprintf(
								"%s: unknown header '%s' (not in v1 closed set) — remove it or run `erg migrate`",
								filepath.Base(path), key))
						}
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
					// parseHeaderLine already trims val; no re-trim needed.
					if val == "" {
						if !diag.ClosedEmpty {
							diag.ClosedEmpty = true
							diag.Errors = append(diag.Errors, fmt.Sprintf(
								"%s: 'Closed:' header requires a non-empty value (closure reason)",
								filepath.Base(path)))
						}
					} else if closed == "" {
						closed = val
					}
				case "Blocked-by":
					blockedBys = append(blockedBys, val)
				case "Tag":
					// parseHeaderLine already trims val; skip empties.
					if val != "" {
						tags = append(tags, val)
					}
				}
			}
		case "gap":
			// ignore lines between header block and log separator
		case "log":
			if trimmed != "" {
				logLines = append(logLines, line)
			}
			if isClosedHeaderLine(line) && !diag.ClosedInLog {
				diag.ClosedInLog = true
				diag.Errors = append(diag.Errors, fmt.Sprintf(
					"%s: 'Closed:' header found in log section — only allowed in header section",
					filepath.Base(path)))
			}
		case "body":
			bodyLines = append(bodyLines, line)
			if isClosedHeaderLine(line) && !diag.ClosedInBody {
				diag.ClosedInBody = true
				diag.Errors = append(diag.Errors, fmt.Sprintf(
					"%s: 'Closed:' header found in body section — only allowed in header section",
					filepath.Base(path)))
			}
		}
	}

	name := filepath.Base(path)

	// Rule 1: magic first line (migrated from validateErg in 0117 step 2).
	if !hasMagic {
		diag.Errors = append(diag.Errors,
			fmt.Sprintf("%s: missing magic first line '%%erg v1'", name))
	}

	// Rule 12 (separators): only the *missing* case is an error. The rule
	// 12 relaxation from ticket 0116 — quoted "--- log ---" / "--- body ---"
	// literals inside the body are legitimate body text — lives in the
	// per-line walk above (the second occurrence of either literal does not
	// re-transition section because `section != "log"`/`bodySepSeen` is
	// already true). diag.HasLogSep / diag.HasBodySep stay true on ANY
	// sighting, mirroring today's parser invariant.
	if !diag.HasLogSep {
		diag.Errors = append(diag.Errors,
			fmt.Sprintf("%s: missing '--- log ---' separator", name))
	}
	if !diag.HasBodySep {
		diag.Errors = append(diag.Errors,
			fmt.Sprintf("%s: missing '--- body ---' separator", name))
	}

	return Erg{
		Path:       path,
		HasMagic:   hasMagic,
		Title:      title,
		Created:    created,
		Author:     author,
		Closed:     closed,
		BlockedBys: blockedBys,
		Tags:       tags,
		LogLines:   logLines,
		Body:       strings.Join(bodyLines, "\n"),
	}, diag
}

// hasHeaderKey reports whether line begins with the literal header key
// prefix (e.g. "Closed:"). When foldCase is true, the comparison is
// case-insensitive on the key portion only. This is a header-key match,
// not a free substring match — indented examples and prose mentions
// never trigger.
func hasHeaderKey(line, key string, foldCase bool) bool {
	if len(line) < len(key) {
		return false
	}
	if foldCase {
		return strings.EqualFold(line[:len(key)], key)
	}
	return line[:len(key)] == key
}

// isClosedHeaderLine reports whether a line begins with the literal
// header key `Closed:` at the start of the line (no leading whitespace).
// Used to detect misplaced `Closed:` keys in the log or body sections.
// Case-sensitive: `disclosed`, indented examples, and quirky casing
// never trigger.
func isClosedHeaderLine(line string) bool {
	return hasHeaderKey(line, "Closed:", false)
}

// loadErgs parses every .erg file under dir recursively. Returns parallel
// slices: tickets[i] and diags[i] describe the same file. The pair is
// sorted by ticket path. On walk failure, returns (nil, nil).
func loadErgs(dir string) ([]Erg, []ParseDiagnostics) {
	type pair struct {
		t Erg
		d ParseDiagnostics
	}
	var pairs []pair
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(path, ".erg") {
			return nil
		}
		t, d := parseErg(path)
		pairs = append(pairs, pair{t, d})
		return nil
	})
	if err != nil {
		return nil, nil
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].t.Path < pairs[j].t.Path
	})
	tickets := make([]Erg, len(pairs))
	diags := make([]ParseDiagnostics, len(pairs))
	for i, p := range pairs {
		tickets[i] = p.t
		diags[i] = p.d
	}
	return tickets, diags
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
