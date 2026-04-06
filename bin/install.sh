#!/bin/sh
# git-erg install — set up git-erg in a target project.
#
# Usage:
#   ./bin/install.sh /path/to/project
#   make install DEST=/path/to/project
#
# Installs:
#   .claude/CLAUDE.md         Project instructions with @import
#   .claude/rules/tickets.md  Format spec (%erg v1)
#   .claude/skills/           Slash commands (ticket-new, claim, close, release, ready)
#   .claude/settings.json     PostToolUse validation hook
#   tickets/                  Ticket directory + archive + validator source
#   .git/hooks/pre-commit     Validation on commit
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

# --- Settings (hooks) ---
if [ -f "$DEST/.claude/settings.json" ] && grep -qF "git-erg" "$DEST/.claude/settings.json" 2>/dev/null; then
    skip ".claude/settings.json"
else
    if [ -f "$DEST/.claude/settings.json" ]; then
        # Don't overwrite existing settings — warn user
        echo "  ! .claude/settings.json exists — merge manually from claude/settings.json"
    else
        cp "$SRC/claude/settings.json" "$DEST/.claude/settings.json"
        ok ".claude/settings.json (validation hook)"
    fi
fi

# --- CLAUDE.md (inside .claude/, not project root) ---
CLAUDE_MD="$DEST/.claude/CLAUDE.md"
if [ -f "$CLAUDE_MD" ] && grep -qF "$MARKER" "$CLAUDE_MD"; then
    skip ".claude/CLAUDE.md"
elif [ -f "$CLAUDE_MD" ]; then
    printf '\n%s begin\n' "$MARKER" >> "$CLAUDE_MD"
    cat "$SRC/claude/CLAUDE-PLUGIN.md" >> "$CLAUDE_MD"
    printf '\n%s end\n' "$MARKER" >> "$CLAUDE_MD"
    ok ".claude/CLAUDE.md (appended ticket system section)"
else
    { printf '%s begin\n' "$MARKER"; cat "$SRC/claude/CLAUDE-PLUGIN.md"; printf '\n%s end\n' "$MARKER"; } > "$CLAUDE_MD"
    ok ".claude/CLAUDE.md (project instructions)"
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
