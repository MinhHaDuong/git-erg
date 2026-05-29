package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// parseCount tracks how many times parseErgBytes is called. Test-visible
// only — used by contract tests to verify parse-once and linear-scaling
// invariants. The increment is a single integer add; no production cost.
var parseCount int

func resetParseCount() { parseCount = 0 }

// IsClosed reports whether the ticket is closed under the v1 criterion:
// either a path component test fires, or a `Closed:` preamble header is
// present with a non-empty value.
func (t *Erg) IsClosed() bool {
	if pathIsClosed(t.Path) {
		return true
	}
	return t.Closed != ""
}

// pathIsClosed implements the path component test from tickets/spec-erg-v1.md:
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

// titleStatusWords are the status-vocabulary words a Title may not begin or
// end with (rule 14, ticket 0145). They are pseudo-tags consumed by
// `erg list` filters and the lifecycle states agents reason about.
var titleStatusWords = map[string]bool{
	"ready":  true,
	"done":   true,
	"closed": true,
	"open":   true,
}

// titleStatusEdgeWord reports whether the first or last word of title (after
// ignoring surrounding punctuation and whitespace) is a status word. A "word"
// is a maximal run of ASCII letters; comparison is case-insensitive. When a
// violation is found it returns the offending word (lowercased), the position
// ("begins with" or "ends with"), and true. The start edge is reported in
// preference to the end edge when both match.
func titleStatusEdgeWord(title string) (word, position string, bad bool) {
	var first, last string
	start := -1
	for i := 0; i < len(title); i++ {
		if isLetter(title[i]) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			w := strings.ToLower(title[start:i])
			if first == "" {
				first = w
			}
			last = w
			start = -1
		}
	}
	if start >= 0 {
		w := strings.ToLower(title[start:])
		if first == "" {
			first = w
		}
		last = w
	}
	if titleStatusWords[first] {
		return first, "begins with", true
	}
	if titleStatusWords[last] {
		return last, "ends with", true
	}
	return "", "", false
}

// titleStatusWordMessage returns the rule-14 violation message for title, or
// ("", false) when the Title is acceptable. The message names the offending
// word and edge; callers prefix it with their own location/context so the
// wording stays identical across `erg validate`/`erg check` (parseErgBytes)
// and the `erg new` creation-time guard — one definition, no drift.
func titleStatusWordMessage(title string) (string, bool) {
	word, pos, bad := titleStatusEdgeWord(title)
	if !bad {
		return "", false
	}
	return fmt.Sprintf(
		"Title %s status word '%s' — reserved for ticket status; rephrase so the Title does not start or end with: closed, done, open, ready",
		pos, word), true
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
	parseCount++
	var errs []string
	// Strip UTF-8 BOM if present
	raw := data
	if len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF {
		raw = raw[3:]
	}
	// Normalize CRLF to LF
	s := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(s, "\n")
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
	legacyV1 := false
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
			// Detect legacy "%erg v1" magic line (exact match only —
			// must not match "%erg v2" or other unknown versions).
			if trimmed == "%erg v1" {
				legacyV1 = true
			}
			// No magic line — try to parse as old format
			section = "headers"
			// Fall through to header parsing
		}

		if trimmed == separatorLog {
			hasLogSep = true
			if !bodySepSeen {
				section = "log"
				continue
			}
		}
		if trimmed == separatorBody {
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
				// A blank line ends the header block only when it is not
				// followed (skipping further blanks) by another header line
				// before the log/body separator. An interior blank — one with
				// more header-shaped lines still below it — is tolerated:
				// headers under it are parsed normally instead of dropped into
				// the discarded gap (ticket 0141).
				if blankEndsHeaderBlock(lines, lineIdx) {
					section = "gap"
				}
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
								"%s:%d: 'Status:' header is no longer part of %%erg 0.1 — run `erg migrate` to convert",
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
		if legacyV1 {
			errs = append(errs,
				fmt.Sprintf("%s: legacy '%%erg v1' magic line — run `erg migrate` to convert to '%%erg 0.1'", name))
		} else {
			errs = append(errs,
				fmt.Sprintf("%s: missing magic first line '%%erg 0.1'", name))
		}
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

	// Rule 7: Created is ISO date.
	if created != "" && !isoDateRE.MatchString(created) {
		errs = append(errs, fmt.Sprintf(
			"%s:%d: Created '%s' is not a valid ISO date (YYYY-MM-DD)", name, createdLine, created))
	}

	// Rule 14: Title must not begin or end with a status word (ready, done,
	// closed, open). Those are status vocabulary in this system — pseudo-tags
	// consumed by `erg list` filters and the lifecycle states agents reason
	// about — so at a title's edge they read as a status assertion about the
	// ticket itself rather than as a reference to the command or concept being
	// changed (ticket 0145). Closed tickets are grandfathered: the rule is
	// enforced on open + new only, so existing closed history is never broken.
	if title != "" && !(pathIsClosed(path) || closed != "") {
		if msg, bad := titleStatusWordMessage(title); bad {
			errs = append(errs, fmt.Sprintf("%s:%d: %s", name, titleLine, msg))
		}
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
			fmt.Sprintf("%s: missing '%s' separator", name, separatorLog))
	}
	if !hasBodySep {
		errs = append(errs,
			fmt.Sprintf("%s: missing '%s' separator", name, separatorBody))
	}

	return Erg{
		Path:       path,
		Title:      title,
		Created:    created,
		Author:     author,
		Closed:     closed,
		BlockedBys: blockedBys,
		Tags:       tags,
		TagLines:   tagLines,
		LogLines:   logLines,
		Body:       strings.Join(bodyLines, "\n"),
	}, errs
}

// blankEndsHeaderBlock reports whether the blank line at lines[idx] (the
// caller has already established it is blank) terminates the header block.
// Scanning forward past any further blank lines, the block ends unless the
// next non-blank line is another header-shaped line: a separator, a
// non-header line, or end-of-input ends the block; a header line means the
// blank is *interior* and the block continues (ticket 0141).
func blankEndsHeaderBlock(lines []string, idx int) bool {
	for j := idx + 1; j < len(lines); j++ {
		t := strings.TrimSpace(lines[j])
		if t == "" {
			continue
		}
		if t == separatorLog || t == separatorBody {
			return true
		}
		if _, _, ok := parseHeaderLine(lines[j]); ok {
			return false
		}
		return true
	}
	return true
}

// interiorHeaderBlanks returns the 0-based indices of blank lines that fall
// inside the header block — the blanks the parser tolerates rather than
// treating as the block terminator. The single blank that legitimately ends
// the header block (the one before `--- log ---`) is never included. This is
// the one definition of "interior blank", shared by the read path
// (parseErgBytes), the write-time autofix (collapseHeaderBlanks), the migrate
// sweep, and the validate/check warnings (ticket 0141).
func interiorHeaderBlanks(lines []string) []int {
	var idxs []int
	inHeaders := false
	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if !inHeaders {
			if t == "" {
				continue // leading blank before the magic / first header line
			}
			// First non-empty line is the magic line (or, in legacy files
			// with no magic, the first header). The header block proper begins
			// on the line below it.
			if t == separatorLog || t == separatorBody {
				return idxs // no header block at all
			}
			inHeaders = true
			continue
		}
		if t == separatorLog || t == separatorBody {
			break // header block region is over
		}
		if t == "" {
			if blankEndsHeaderBlock(lines, i) {
				break
			}
			idxs = append(idxs, i)
		}
	}
	return idxs
}

// splitLines splits content on "\n", dropping the empty trailing element a
// final newline produces. The bool reports whether that trailing newline was
// present so callers can restore it after rejoining.
func splitLines(content string) ([]string, bool) {
	hadTrailingNewline := strings.HasSuffix(content, "\n")
	lines := strings.Split(content, "\n")
	if hadTrailingNewline {
		lines = lines[:len(lines)-1]
	}
	return lines, hadTrailingNewline
}

// collapseHeaderBlanks removes interior blank lines from the header block,
// leaving the rest of the file — the terminating blank before `--- log ---`,
// the log, and the body — byte-for-byte unchanged. Returns data unmodified
// when there is no interior blank. Feeds the write-time autofix paths
// (close/tag/untag) and the migrate sweep (ticket 0141).
func collapseHeaderBlanks(data []byte) []byte {
	lines, hadTrailingNewline := splitLines(string(data))
	interior := interiorHeaderBlanks(lines)
	if len(interior) == 0 {
		return data
	}
	drop := make(map[int]bool, len(interior))
	for _, i := range interior {
		drop[i] = true
	}
	out := make([]string, 0, len(lines))
	for i, line := range lines {
		if drop[i] {
			continue
		}
		out = append(out, line)
	}
	rejoined := strings.Join(out, "\n")
	if hadTrailingNewline {
		rejoined += "\n"
	}
	return []byte(rejoined)
}

// hasInteriorHeaderBlank reports whether data has at least one blank line
// inside its header block. Feeds the non-fatal warnings emitted by
// `erg validate` (shout) and `erg check` (warn) (ticket 0141).
func hasInteriorHeaderBlank(data []byte) bool {
	lines, _ := splitLines(string(data))
	return len(interiorHeaderBlanks(lines)) > 0
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
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(path, ".erg") {
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
