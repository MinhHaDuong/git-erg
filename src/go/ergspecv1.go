package main

import "regexp"

// MagicLine is the required first non-empty line of every %erg v1 ticket.
//
// ABNF production:
//
//	magic-line := "%erg v1"
const MagicLine = "%erg v1"

// RequiredHeaders lists the three mandatory preamble headers for every
// %erg v1 ticket. A missing header is a validation error (rule 2).
//
// ABNF production:
//
//	required-header := "Title" / "Created" / "Author"
var RequiredHeaders = []string{"Title", "Created", "Author"}

// SingletonHeaders names headers that must appear at most once in the
// preamble. Repeating any of these is a validation error (rule 4).
//
// ABNF production:
//
//	singleton-header := "Title" / "Created" / "Author" / "Closed"
var SingletonHeaders = map[string]bool{
	"Title": true, "Created": true, "Author": true, "Closed": true,
}

// ValidHeaders is the closed set of header keys for %erg v1.
// No X- extensions are allowed; unknown keys are rejected (rule 3).
//
// ABNF production:
//
//	header-key := "Title" / "Created" / "Author" / "Closed" /
//	              "Blocked-by" / "Tag"
var ValidHeaders = map[string]bool{
	"Title": true, "Created": true, "Author": true,
	"Closed": true, "Blocked-by": true, "Tag": true,
}

// ValidTagValues is the closed value set for the Tag: header (%erg v1).
// Allowed values: needs-human, deferred, post-talk, post-conference.
// Tag: needs-human or deferred suppresses a ticket from erg ready output.
//
// ABNF production:
//
//	tag-value := "needs-human" / "deferred" / "post-talk" / "post-conference"
var ValidTagValues = map[string]bool{
	"needs-human":     true,
	"deferred":        true,
	"post-talk":       true,
	"post-conference": true,
}

// IsoDateRE matches a valid Created: date value (YYYY-MM-DD, rule 7).
//
// ABNF production:
//
//	iso-date := 4DIGIT "-" 2DIGIT "-" 2DIGIT
var IsoDateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// FilenameRE matches a valid .erg filename: 4-digit ID, dash, lowercase
// kebab slug (rule 8). Pattern: NNNN-word(-word)*.erg
//
// ABNF production:
//
//	filename := 4DIGIT "-" lc-word *("-" lc-word) ".erg"
//	lc-word   := 1*(ALPHA / DIGIT)   ; lowercase ASCII alphanumeric
var FilenameRE = regexp.MustCompile(`^\d{4}-[a-z0-9]+(?:-[a-z0-9]+)*\.erg$`)

// LogLineRE matches a valid log section line: ISO timestamp, actor, verb,
// optional detail (rule 11).
// Pattern: YYYY-MM-DDThh:mmZ ACTOR VERB [detail...]
//
// ABNF production:
//
//	log-line := iso-datetime SP actor SP verb [SP detail]
//	iso-datetime := 4DIGIT "-" 2DIGIT "-" 2DIGIT "T" 2DIGIT ":" 2DIGIT "Z"
var LogLineRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}Z\s+\S+\s+\S+`)

// hostRE matches the host component of a forge ref.
// Colons and underscores are excluded; must start and end with an
// alphanumeric character.
//
// ABNF production:
//
//	host := ALNUM *( ALNUM / "." / "-" ) ALNUM / ALNUM
//	ALNUM := ALPHA / DIGIT
var hostRE = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$`)

// ownerRE matches the owner/org component of a forge ref.
//
// ABNF production:
//
//	owner := 1*( ALNUM / "_" / "." / "-" )
var ownerRE = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// repoRE matches the repository name component of a forge ref.
//
// ABNF production:
//
//	repo := 1*( ALNUM / "_" / "." / "-" )
var repoRE = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
