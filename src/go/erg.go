package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Erg is the schema-literal projection of a %erg v1 ticket file.
// Lenient-parse invariant: parseErg always returns a usable Erg (at
// minimum with Path set) so callers can report a filename even when the
// file is unreadable or malformed. parseErg also returns a []string of
// per-file rule violations alongside the Erg; corpus-level rules
// (duplicate IDs, ref resolution, cycles) live in validateCorpus.
type Erg struct {
	Path     string
	HasMagic bool

	// v1 headers — typed fields populated from first occurrence
	Title      string   // required, non-empty (validator rule 2)
	Created    string   // required, non-empty
	Author     string   // required, non-empty
	Closed     string   // optional; first non-empty Closed: value when present
	BlockedBys []Ref    // possibly empty; one entry per `Blocked-by:` line, parsed at parse time
	Tags       []string // possibly empty; one entry per `Tag:` line, trimmed; empties skipped

	LogLines []string // one structured event per entry
	Body     string   // multiline
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
// once in the preamble. Repeats are reported as parse errors.
var v1SingletonKeys = map[string]bool{
	"Title": true, "Created": true, "Author": true, "Closed": true,
}

// parseErg parses a single .erg file into an Erg plus parse-time errors
// (per-file rule violations: rules 1-9, 11, 12). On read error, returns
// an empty Erg with only Path set so callers can still report a filename.
// Corpus-level rules (rule 10 ref resolution, rule 13 cycles, duplicate
// IDs) live in validateCorpus.
func parseErg(path string) (Erg, []string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Erg{Path: path}, nil
	}
	return parseErgBytes(data, path)
}

// parseErgBytes parses raw .erg file content into an Erg plus parse-time
// errors. Callers that already hold the file bytes (e.g. after
// os.ReadFile for rewriting) use this to avoid a second read.
//
// The parser is lenient: it never bails on a single error. It walks the
// whole file, accumulates every error it can find, and returns the
// best-effort Erg alongside the []string.
//
// Edge case: a literal "--- log ---" or "--- body ---" line is recognised
// as a separator only on first sighting; later occurrences inside the
// body are body text (rule 12 relaxation, ticket 0116).
func parseErgBytes(data []byte, path string) (Erg, []string) {
	var errs []string
	lines := strings.Split(string(data), "\n")
	name := filepath.Base(path)

	var logLines, bodyLines []string
	var logLineNums []int
	var title, created, author, closed string
	var titleLine, createdLine, authorLine int
	var tags []string
	var blockedBys []Ref
	var blockedByLines, tagLines []int
	section := "magic" // magic | headers | gap | log | body
	hasMagic := false
	hasLogSep := false
	hasBodySep := false
	bodySepSeen := false
	closedEmpty := false
	closedInLog := false
	closedInBody := false
	unknownSeen := make(map[string]bool)
	repeatedSeen := make(map[string]bool)
	headerCounts := make(map[string]int)

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
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
			hasLogSep = true
			if !bodySepSeen {
				section = "log"
				continue
			}
		}
		if trimmed == "--- body ---" {
			hasBodySep = true
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
				// Classify header.
				if v1HeaderKeys[key] {
					if v1SingletonKeys[key] && headerCounts[key] == 2 {
						if !repeatedSeen[key] {
							repeatedSeen[key] = true
							errs = append(errs, fmt.Sprintf(
								"%s:%d: header '%s' is non-repeatable (appears more than once)",
								name, lineNum, key))
						}
					}
				} else {
					if !unknownSeen[key] {
						unknownSeen[key] = true
						switch key {
						case "Status":
							errs = append(errs, fmt.Sprintf(
								"%s:%d: 'Status:' header is no longer part of %%erg v1 — run `erg migrate` to convert",
								name, lineNum))
						case "Tags":
							errs = append(errs, fmt.Sprintf(
								"%s:%d: 'Tags:' has been renamed to 'Tag:' — run `erg migrate` to convert",
								name, lineNum))
						default:
							errs = append(errs, fmt.Sprintf(
								"%s:%d: unknown header '%s' (not in v1 closed set) — remove it or run `erg migrate`",
								name, lineNum, key))
						}
					}
				}
				// Populate typed fields. Singletons keep first occurrence.
				// Track line numbers for end-of-walk emissions.
				switch key {
				case "Title":
					if titleLine == 0 {
						titleLine = lineNum
					}
					if title == "" {
						title = val
					}
				case "Created":
					if createdLine == 0 {
						createdLine = lineNum
					}
					if created == "" {
						created = val
					}
				case "Author":
					if authorLine == 0 {
						authorLine = lineNum
					}
					if author == "" {
						author = val
					}
				case "Closed":
					// parseHeaderLine already trims val; no re-trim needed.
					if val == "" {
						if !closedEmpty {
							closedEmpty = true
							errs = append(errs, fmt.Sprintf(
								"%s:%d: 'Closed:' header requires a non-empty value (closure reason)",
								name, lineNum))
						}
					} else if closed == "" {
						closed = val
					}
				case "Blocked-by":
					ref, refErr := parseRef(val)
					if refErr != nil {
						errs = append(errs, fmt.Sprintf("%s:%d: %v", name, lineNum, refErr))
					}
					blockedBys = append(blockedBys, ref)
					blockedByLines = append(blockedByLines, lineNum)
				case "Tag":
					// parseHeaderLine already trims val; skip empties.
					if val != "" {
						tags = append(tags, val)
						tagLines = append(tagLines, lineNum)
					}
				}
			}
		case "gap":
			// ignore lines between header block and log separator
		case "log":
			if trimmed != "" {
				logLines = append(logLines, line)
				logLineNums = append(logLineNums, lineNum)
			}
			if isClosedHeaderLine(line) && !closedInLog {
				closedInLog = true
				errs = append(errs, fmt.Sprintf(
					"%s:%d: 'Closed:' header found in log section — only allowed in header section",
					name, lineNum))
			}
		case "body":
			bodyLines = append(bodyLines, line)
			if isClosedHeaderLine(line) && !closedInBody {
				closedInBody = true
				errs = append(errs, fmt.Sprintf(
					"%s:%d: 'Closed:' header found in body section — only allowed in header section",
					name, lineNum))
			}
		}
	}

	// End-of-walk per-file rule emissions. Per-line emissions for rules
	// 3, 4, 6 already fired above; the remaining rules need the finalised
	// header values or end-of-walk knowledge.

	// Rule 1: magic first line.
	if !hasMagic {
		errs = append(errs,
			fmt.Sprintf("%s: missing magic first line '%%erg v1'", name))
	}

	// Rule 2: required headers must be present AND non-empty.
	// When the header was present (line number stored), report the line;
	// when completely absent (line number is 0), use filename-only format.
	if strings.TrimSpace(title) == "" {
		if titleLine > 0 {
			errs = append(errs, fmt.Sprintf(
				"%s:%d: missing or empty required header 'Title' — add 'Title: <text>' to the preamble", name, titleLine))
		} else {
			errs = append(errs, fmt.Sprintf(
				"%s: missing or empty required header 'Title' — add 'Title: <text>' to the preamble", name))
		}
	}
	if strings.TrimSpace(created) == "" {
		if createdLine > 0 {
			errs = append(errs, fmt.Sprintf(
				"%s:%d: missing or empty required header 'Created' — add 'Created: YYYY-MM-DD' to the preamble", name, createdLine))
		} else {
			errs = append(errs, fmt.Sprintf(
				"%s: missing or empty required header 'Created' — add 'Created: YYYY-MM-DD' to the preamble", name))
		}
	}
	if strings.TrimSpace(author) == "" {
		if authorLine > 0 {
			errs = append(errs, fmt.Sprintf(
				"%s:%d: missing or empty required header 'Author' — add 'Author: <name>' to the preamble", name, authorLine))
		} else {
			errs = append(errs, fmt.Sprintf(
				"%s: missing or empty required header 'Author' — add 'Author: <name>' to the preamble", name))
		}
	}

	// Rule 5: Tag: values must be from the closed value set.
	for i, v := range tags {
		if !validTagValues[v] {
			errs = append(errs, fmt.Sprintf(
				"%s:%d: unknown Tag value '%s' (not in v1 closed set: needs-human, deferred, post-talk, post-conference)",
				name, tagLines[i], v))
		}
	}

	// Rule 7: Created is ISO date.
	if created != "" && !isoDateRE.MatchString(created) {
		errs = append(errs, fmt.Sprintf(
			"%s:%d: Created '%s' is not a valid ISO date (YYYY-MM-DD)", name, createdLine, created))
	}

	// Rule 8: filename matches NNNN-slug.erg.
	if !filenameRE.MatchString(name) {
		errs = append(errs, fmt.Sprintf(
			"%s: filename does not match NNNN-slug.erg pattern", name))
	}

	// Rule 9: Blocked-by values are parsed inline (case "Blocked-by"
	// above) — errors emitted there. Rule 10 (local-ref resolution
	// against corpus IDs) is corpus-level and lives in validateCorpus.

	// Rule 11: log lines match format.
	for i, line := range logLines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !logLineRE.MatchString(trimmed) {
			errs = append(errs, fmt.Sprintf(
				"%s:%d: malformed log line: %s", name, logLineNums[i], trimmed))
		}
	}

	// Rule 12 (separators): only the *missing* case is an error. The rule
	// 12 relaxation from ticket 0116 — quoted "--- log ---" / "--- body ---"
	// literals inside the body are legitimate body text — lives in the
	// per-line walk above (the second occurrence of either literal does not
	// re-transition section because bodySepSeen is already true). hasLogSep
	// / hasBodySep go true on ANY sighting; emission fires only when
	// neither was ever seen.
	if !hasLogSep {
		errs = append(errs,
			fmt.Sprintf("%s: missing '--- log ---' separator", name))
	}
	if !hasBodySep {
		errs = append(errs,
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
	}, errs
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
// slices: tickets[i] and parseErrs[i] describe the same file. The pair is
// sorted by ticket path. On walk failure, returns (nil, nil).
func loadErgs(dir string) ([]Erg, [][]string) {
	type pair struct {
		t Erg
		e []string
	}
	var pairs []pair
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(path, ".erg") {
			return nil
		}
		t, e := parseErg(path)
		pairs = append(pairs, pair{t, e})
		return nil
	})
	if err != nil {
		return nil, nil
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].t.Path < pairs[j].t.Path
	})
	tickets := make([]Erg, len(pairs))
	parseErrs := make([][]string, len(pairs))
	for i, p := range pairs {
		tickets[i] = p.t
		parseErrs[i] = p.e
	}
	return tickets, parseErrs
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
