package main

import (
	"fmt"
	"strings"
)

// MatchesLocalID reports whether r is a local Blocked-by reference to the
// given 4-digit ticket ID. This is the single predicate shared by dependency
// detection (cmdRm's dependent scan, clearBlockedByRefs) and edge removal
// (removeBlockedByLine), so "this ticket is a dependent" and "strip this
// Blocked-by line" can never disagree -- the class of bug where a dependent is
// detected but its edge is left dangling.
func (r Ref) MatchesLocalID(id string) bool {
	return r.Kind == RefLocal && r.ID == id
}

// parseRef parses a Blocked-by/Superseded-by value, which is a URI-reference
// (RFC 3986). Purely syntactic -- no network, no ticket-existence check. Only a
// malformed URI-reference (a space or control character) is an error; a
// well-formed but unresolvable handle is valid (the agnostic invariant).
func parseRef(raw string) (Ref, error) {
	if raw == "" {
		return Ref{Raw: raw}, fmt.Errorf("empty ref")
	}
	// Local: exactly 4 ASCII digits -> a ticket in the current store.
	if len(raw) == 4 && allDigits(raw) {
		return Ref{Raw: raw, Kind: RefLocal, ID: raw}, nil
	}
	// Otherwise it must be a well-formed URI-reference. A raw space or control
	// character is the one malformed case.
	for i := 0; i < len(raw); i++ {
		if raw[i] == ' ' || raw[i] < 0x20 || raw[i] == 0x7f {
			return Ref{Raw: raw}, fmt.Errorf(
				"malformed ref %q: not a URI-reference (contains a space or control character)", raw)
		}
	}
	if hasScheme(raw) {
		// Absolute URI -- opaque to the core; a scheme resolver handles it (or
		// it stays unresolved). https://.../tickets/0042-x.erg, file:/abs/path, ...
		return Ref{Raw: raw, Kind: RefURI}, nil
	}
	// Relative reference. A path (no query/fragment) ending in /NNNN names a
	// sibling ticket resolved at the repo root; anything else is a well-formed
	// handle the core cannot resolve locally (-> unresolved).
	if !strings.ContainsAny(raw, "?#") {
		if mod, id, ok := splitPathRef(raw); ok {
			return Ref{Raw: raw, Kind: RefPath, Module: mod, ID: id}, nil
		}
	}
	return Ref{Raw: raw, Kind: RefURI}, nil
}

// hasScheme reports whether raw begins with an RFC 3986 scheme (ALPHA *(ALPHA /
// DIGIT / "+" / "-" / ".") ":") -- i.e. it is an absolute URI rather than a
// relative reference. Hand-rolled to keep the binary free of any net/* import
// (the offline invariant); does not pull in net/url.
func hasScheme(raw string) bool {
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch {
		case c == ':':
			return i > 0 // a non-empty run of scheme chars preceded the colon
		case c == '/' || c == '?' || c == '#':
			return false // a relative-ref component began before any colon
		case i == 0:
			if !isAlpha(c) {
				return false
			}
		default:
			if !isAlpha(c) && !(c >= '0' && c <= '9') && c != '+' && c != '-' && c != '.' {
				return false
			}
		}
	}
	return false
}

func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// splitPathRef splits a relative path such as "auth/0042" or "libs/auth/0042"
// into the module dir ("auth", "libs/auth") and the 4-digit ID, requiring at
// least one non-empty path component before a trailing "/NNNN".
func splitPathRef(p string) (module, id string, ok bool) {
	i := strings.LastIndexByte(p, '/')
	if i <= 0 { // no slash, or a leading slash (no module component)
		return "", "", false
	}
	module, id = p[:i], p[i+1:]
	if module != "" && len(id) == 4 && allDigits(id) {
		return module, id, true
	}
	return "", "", false
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
