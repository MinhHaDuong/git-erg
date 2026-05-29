#!/bin/sh
# docs/demo.sh — the README "10-second, zero-install" demo, recordable.
#
# Runs the zero-install ticket demo in a throwaway temp dir (never touches your
# repo) and cleans up after. It "types" each command, pauses briefly, then runs
# it — so a screen recorder captures a readable session.
#
# Record it:
#   asciinema rec -c "sh docs/demo.sh" demo.cast      # produces an asciinema cast
#   agg demo.cast docs/demo.gif                        # cast -> gif (asciinema/agg)
#
# Or just run it to sanity-check the demo still works:
#   sh docs/demo.sh
#
# Pure POSIX sh; the only "dependency" is the tools the demo is selling
# (mkdir, cat, ls, grep). No erg binary, no network, no Go.

set -eu

# Pacing — override to speed up/slow down a recording (e.g. SPEED=0 for tests).
SPEED="${SPEED:-1}"
pause() { [ "$SPEED" = "0" ] || sleep "$1"; }

PROMPT='$ '

# "Type" a command, then run it.
run() {
	printf '%s%s\n' "$PROMPT" "$1"
	pause 0.6
	# Demo commands are illustrative; a non-zero exit (e.g. grep -L finds no
	# matching *line* yet prints the file) must not abort the recording.
	# shellcheck disable=SC2086
	eval "$1" || true
	pause 0.8
	printf '\n'
}

# Comment line (shown, not executed).
say() {
	printf '%s# %s\n' "$PROMPT" "$1"
	pause 0.7
}

# Work in a throwaway dir so the demo never pollutes the caller's tree.
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
cd "$WORK"

say "A ticket is just a text file. Nothing to install."
pause 0.4

run "mkdir tickets"

run "cat > tickets/0001-add-auth.erg <<'EOF'
%erg 0.1
Title: Add authentication flow
Created: 2026-05-29
Author: me

--- log ---
2026-05-29T10:00Z me created

--- body ---
Need auth before shipping the API.
EOF"

say "There's your backlog:"
run "ls tickets/"

say "There's your open list (grep is the query language):"
run "grep -L '^Closed:' tickets/*.erg"

say "It's text in git. Delete it and it never existed."
run "rm -rf tickets/"

say "That's it. Offline, no server, no account, no lock-in."
pause 1
