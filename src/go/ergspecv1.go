package main

import "regexp"

// magicLine is the required first non-empty line of every %erg 0.1 ticket.
//
// ABNF production:
//
//	magic-line := "%erg 0.1"
const magicLine = "%erg 0.1"

// separatorLog is the log section delimiter.
const separatorLog = "--- log ---"

// separatorBody is the body section delimiter.
const separatorBody = "--- body ---"

// Erg is the schema-literal projection of a %erg 0.1 ticket file.
// Lenient-parse invariant: parseErg always returns a usable Erg (at
// minimum with Path set) so callers can report a filename even when the
// file is unreadable or malformed. parseErg also returns a []string of
// per-file rule violations alongside the Erg; corpus-level rules
// (duplicate IDs, ref resolution, cycles) live in validateCorpus.
type Erg struct {
	Path string

	// v1 headers -- typed fields populated from first occurrence
	Title         string   // required, non-empty (validator rule 2)
	Created       string   // required, non-empty
	Author        string   // required, non-empty
	Closed        string   // optional; first non-empty Closed: value when present
	BlockedBys    []Ref    // possibly empty; one entry per `Blocked-by:` line, parsed at parse time
	SupersededBys []Ref    // possibly empty; one entry per `Superseded-by:` line; durable lineage carried by the CLOSED ticket
	Labels        []string // possibly empty; one entry per `Label:` line, trimmed; empties skipped
	LabelLines    []int    // 1-indexed line numbers for each Label entry

	LogLines []string // one structured event per entry
	Body     string   // multiline
}

// RefKind discriminates the two Blocked-by reference forms defined in
// tickets/spec-erg-v1.md.
type RefKind int

const (
	RefInvalid RefKind = iota
	RefLocal           // 0042 -- local ticket ID
	RefForge           // host/owner/repo#N -- forge issue
)

// Ref is a parsed Blocked-by value. Downstream code (validator, ready)
// must read these fields rather than re-parse Raw -- a single parser is
// the source of truth.
type Ref struct {
	Raw    string // original text as written in the .erg file
	Kind   RefKind
	ID     string // 4-digit ticket ID (RefLocal only)
	Host   string // hostname (RefForge only)
	Owner  string // owner/org (RefForge only)
	Repo   string // repo name (RefForge only)
	Number string // issue number (RefForge only)
}

// v1HeaderKeys is the closed set of header keys recognised by parseErg.
// Used to classify header lines as known/unknown during parsing.
var v1HeaderKeys = map[string]bool{
	"Title": true, "Created": true, "Author": true,
	"Closed": true, "Blocked-by": true, "Superseded-by": true, "Label": true,
}

// v1SingletonKeys is the subset of v1HeaderKeys that may appear at most
// once in the preamble. Repeats are reported as parse errors.
var v1SingletonKeys = map[string]bool{
	"Title": true, "Created": true, "Author": true, "Closed": true,
}

// isoDateRE matches a valid Created: date value (YYYY-MM-DD, rule 7).
//
// ABNF production:
//
//	iso-date := 4DIGIT "-" 2DIGIT "-" 2DIGIT
var isoDateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// filenameRE matches a valid .erg filename: 4-digit ID, dash, lowercase
// kebab slug (rule 8). Pattern: NNNN-word(-word)*.erg
//
// ABNF production:
//
//	filename := 4DIGIT "-" lc-word *("-" lc-word) ".erg"
//	lc-word   := 1*(ALPHA / DIGIT)   ; lowercase ASCII alphanumeric
var filenameRE = regexp.MustCompile(`^\d{4}-[a-z0-9]+(?:-[a-z0-9]+)*\.erg$`)

// logLineRE matches a valid log section line: ISO timestamp, actor, verb,
// optional detail (rule 11).
// Pattern: YYYY-MM-DDThh:mmZ ACTOR VERB [detail...]
//
// ABNF production:
//
//	log-line := iso-datetime SP actor SP verb [SP detail]
//	iso-datetime := 4DIGIT "-" 2DIGIT "-" 2DIGIT "T" 2DIGIT ":" 2DIGIT "Z"
var logLineRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}Z\s+\S+\s+\S+`)

// logEntryPrefixRE matches any log line that opens a new log entry:
// a line whose first 10 characters form a YYYY-MM-DD date. Used by
// foldLogLines to distinguish entry-openers from continuation lines.
//
// ABNF production:
//
//	log-entry-prefix := 4DIGIT "-" 2DIGIT "-" 2DIGIT
var logEntryPrefixRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)

// logDateOnlyRE matches a date-only log entry stamp (YYYY-MM-DD followed
// by a space, not a T). Used by foldLogLines to normalise legacy date-only
// stamps to YYYY-MM-DDT00:00Z form.
//
// ABNF production:
//
//	log-date-only := 4DIGIT "-" 2DIGIT "-" 2DIGIT SP
var logDateOnlyRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} `)

// hostRE matches the host component of a forge ref.
// Colons and underscores are excluded; must start and end with an
// alphanumeric character.
//
// ABNF production:
//
//	host := ALNUM *( ALNUM / "." / "-" ) ALNUM / ALNUM
//	ALNUM := ALPHA / DIGIT
var hostRE = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$`)

// identRE matches the owner/org or repository name component of a forge ref.
// Both use the same character set: alphanumeric, underscore, dot, dash.
//
// ABNF productions:
//
//	owner := 1*( ALNUM / "_" / "." / "-" )
//	repo  := 1*( ALNUM / "_" / "." / "-" )
var identRE = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
