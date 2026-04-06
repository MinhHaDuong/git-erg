#!/bin/sh
# git-erg install — set up git-erg in a target project.
#
# Usage:
#   ./bin/install.sh /path/to/project
#   make install DEST=/path/to/project
#
# Idempotent. Safe to re-run.

set -e

MARKER="# --- git-erg ---"

usage() {
    echo "Usage: $0 DEST" >&2
    echo "  DEST  Path to a git repository to install git-erg into" >&2
    exit 1
}

ok()   { printf "  \033[32m+\033[0m %s\n" "$1"; }
skip() { printf "  \033[33m~\033[0m %s (already present)\n" "$1"; }

DEST="$1"
[ -z "$DEST" ] && usage

# Validate destination
if [ ! -e "$DEST/.git" ]; then
    echo "ERROR: $DEST is not a git repository (.git not found)" >&2
    exit 1
fi

SRC="$(cd "$(dirname "$0")/.." && pwd)"

echo ""
echo "  git-erg — installing into $(cd "$DEST" && pwd)"
echo ""

# --- Tool source ---
mkdir -p "$DEST/tickets/tools/go"
cp "$SRC/tickets/tools/go/main.go" "$DEST/tickets/tools/go/"
cp "$SRC/tickets/tools/go/go.mod"  "$DEST/tickets/tools/go/"
ok "tickets/tools/go/ (validator source)"

# --- Rules ---
mkdir -p "$DEST/.claude/rules"
cp "$SRC/rules/tickets.md" "$DEST/.claude/rules/"
ok ".claude/rules/tickets.md (format spec)"

# --- Skills ---
mkdir -p "$DEST/.claude/skills"
cp -r "$SRC/claude/skills/"* "$DEST/.claude/skills/"
ok ".claude/skills/ (ticket-new, claim, close, release, ready)"

# --- CLAUDE.md ---
PLUGIN_SECTION="$SRC/claude/CLAUDE-PLUGIN.md"
if [ -f "$DEST/CLAUDE.md" ]; then
    if grep -qF "$MARKER" "$DEST/CLAUDE.md"; then
        skip "CLAUDE.md"
    else
        printf '\n%s begin\n' "$MARKER" >> "$DEST/CLAUDE.md"
        cat "$PLUGIN_SECTION" >> "$DEST/CLAUDE.md"
        printf '\n%s end\n' "$MARKER" >> "$DEST/CLAUDE.md"
        ok "CLAUDE.md (appended ticket system section)"
    fi
else
    printf '%s begin\n' "$MARKER" > "$DEST/CLAUDE.md"
    cat "$PLUGIN_SECTION" >> "$DEST/CLAUDE.md"
    printf '\n%s end\n' "$MARKER" >> "$DEST/CLAUDE.md"
    ok "CLAUDE.md (created with ticket system section)"
fi

# --- Ticket directories ---
mkdir -p "$DEST/tickets/archive"
if [ ! -f "$DEST/tickets/archive/.gitkeep" ]; then
    touch "$DEST/tickets/archive/.gitkeep"
fi
ok "tickets/ and tickets/archive/"

# --- .gitignore ---
GITIGNORE_LINE="tickets/tools/go/erg"
if [ -f "$DEST/.gitignore" ] && grep -qxF "$GITIGNORE_LINE" "$DEST/.gitignore"; then
    skip ".gitignore"
else
    if [ -f "$DEST/.gitignore" ]; then
        printf '\n# git-erg compiled binary\n%s\n' "$GITIGNORE_LINE" >> "$DEST/.gitignore"
    else
        printf '# git-erg compiled binary\n%s\n' "$GITIGNORE_LINE" > "$DEST/.gitignore"
    fi
    ok ".gitignore (erg binary excluded)"
fi

# --- Pre-commit hook ---
HOOK_FILE="$DEST/.git/hooks/pre-commit"
if [ -f "$HOOK_FILE" ] && grep -qF "$MARKER" "$HOOK_FILE"; then
    skip "pre-commit hook"
else
    if [ -f "$HOOK_FILE" ]; then
        printf '\n%s begin\n' "$MARKER" >> "$HOOK_FILE"
        tail -n +2 "$SRC/hooks/pre-commit" >> "$HOOK_FILE"
        printf '%s end\n' "$MARKER" >> "$HOOK_FILE"
    else
        mkdir -p "$DEST/.git/hooks"
        printf '#!/bin/sh\n\n%s begin\n' "$MARKER" > "$HOOK_FILE"
        tail -n +2 "$SRC/hooks/pre-commit" >> "$HOOK_FILE"
        printf '%s end\n' "$MARKER" >> "$HOOK_FILE"
    fi
    chmod +x "$HOOK_FILE"
    ok "pre-commit hook (ticket validation)"
fi

# --- Build ---
(cd "$DEST/tickets/tools/go" && go build -o erg . 2>/dev/null) && \
    ok "erg binary built" || \
    echo "  ! go not found — build later with: cd tickets/tools/go && go build -o erg ."

echo ""
echo "  Done. Try /ticket-new or /ticket-ready to get started."
echo ""
