package main

// This file holds the %erg v1 parser and shared helpers used by every
// subcommand (Erg type, parseErg, loadErgs, jsonEscape, sortedKeys helpers).

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const magicLine = "%erg v1"

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
}

// Title returns the first Title header value, or "" if absent.
func (t *Erg) Title() string {
	if vs, ok := t.Headers["Title"]; ok && len(vs) > 0 {
		return vs[0]
	}
	return ""
}

// Status returns the first Status header value, or "" if absent.
func (t *Erg) Status() string {
	if vs, ok := t.Headers["Status"]; ok && len(vs) > 0 {
		return vs[0]
	}
	return ""
}

// BlockedBy returns all Blocked-by header values, or nil if absent.
func (t *Erg) BlockedBy() []string {
	if vs, ok := t.Headers["Blocked-by"]; ok {
		return vs
	}
	return nil
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
		case "body":
			bodyLines = append(bodyLines, line)
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
	}
}

// loadErgs parses every .erg file directly in dir, sorted by filename.
func loadErgs(dir string) []Erg {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var tickets []Erg
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".erg") {
			tickets = append(tickets, parseErg(filepath.Join(dir, e.Name())))
		}
	}
	sort.Slice(tickets, func(i, j int) bool {
		return tickets[i].Filename() < tickets[j].Filename()
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

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeys2[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
