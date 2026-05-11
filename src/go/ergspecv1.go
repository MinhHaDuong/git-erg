package main

import "regexp"

// magicLine is the required first non-empty line of every %erg v1 ticket.
//
// ABNF production:
//
//	magic-line := "%erg v1"
const magicLine = "%erg v1"

// validTagValues is the closed value set for the Tag: header (%erg v1).
// Allowed values: needs-human, deferred, post-talk, post-conference.
// Any Tag: value suppresses the ticket from `erg ready` output (see
// skipReadyTags in ready.go).
//
// ABNF production:
//
//	tag-value := "needs-human" / "deferred" / "post-talk" / "post-conference"
var validTagValues = map[string]bool{
	"needs-human":     true,
	"deferred":        true,
	"post-talk":       true,
	"post-conference": true,
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
