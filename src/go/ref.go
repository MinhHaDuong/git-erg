package main

import (
	"fmt"
	"strings"
)

// IsForge reports whether the ref targets a forge issue (offline-unknown).
func (r Ref) IsForge() bool {
	return r.Kind == RefForge
}

// parseRef parses a Blocked-by value into a Ref, or returns a precise
// error naming the failure mode. Stays purely syntactic — no network,
// no ticket-existence check.
func parseRef(raw string) (Ref, error) {
	if raw == "" {
		return Ref{Raw: raw}, fmt.Errorf("empty ref")
	}
	// Local: exactly 4 ASCII digits.
	if len(raw) == 4 && allDigits(raw) {
		return Ref{Raw: raw, Kind: RefLocal, ID: raw}, nil
	}

	// Reject old gh: and gh# forms.
	if strings.HasPrefix(raw, "gh:") {
		return Ref{Raw: raw}, fmt.Errorf(
			"forge ref %q uses deprecated 'gh:' scheme; use 'host/owner/repo#N' instead", raw)
	}
	if strings.HasPrefix(raw, "gh#") {
		return Ref{Raw: raw}, fmt.Errorf(
			"forge ref %q uses deprecated 'gh#' scheme; same-repo refs are not supported", raw)
	}

	// Catch case-variant old schemes before forge parsing.
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "gh#") || strings.HasPrefix(lower, "gh:") {
		return Ref{Raw: raw}, fmt.Errorf(
			"forge ref %q: scheme is case-sensitive (use lowercase 'gh' if intentional, or 'host/owner/repo#N')", raw)
	}

	// Forge: host "/" owner "/" repo "#" number.
	// Parse host, owner, repo from the part before the # sign.
	hashIdx := strings.LastIndexByte(raw, '#')
	if hashIdx > 0 {
		hostOwnerRepo := raw[:hashIdx]
		num := raw[hashIdx+1:]

		// Split host / owner / repo.
		parts := strings.Split(hostOwnerRepo, "/")
		if len(parts) == 3 {
			host, owner, repo := parts[0], parts[1], parts[2]
			if hostRE.MatchString(host) && identRE.MatchString(owner) && identRE.MatchString(repo) {
				// Validate the number format.
				if err := validateIssueNumber(num); err != nil {
					return Ref{Raw: raw}, fmt.Errorf("malformed ref %q: %v", raw, err)
				}
				return Ref{Raw: raw, Kind: RefForge, Host: host, Owner: owner, Repo: repo, Number: num}, nil
			}
		}
	}

	return Ref{Raw: raw}, fmt.Errorf(
		"malformed ref %q: not a 4-digit local ID or host/owner/repo#N", raw)
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

// validateIssueNumber enforces the [1-9][0-9]* rule from rules/tickets.md
// (positive integer, no leading zero).
func validateIssueNumber(num string) error {
	if num == "" {
		return fmt.Errorf("missing issue number")
	}
	if !allDigits(num) {
		return fmt.Errorf("issue number %q is not a positive integer", num)
	}
	if num[0] == '0' {
		return fmt.Errorf("issue number %q has a leading zero", num)
	}
	return nil
}
