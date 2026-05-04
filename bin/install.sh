#!/bin/sh
# git-erg install — set up git-erg in a target project.
#
# Usage:
#   ./bin/install.sh /path/to/project
#   make install DEST=/path/to/project
#
# Installs:
#   tickets/spec-erg-v1.md         Format spec (%erg v1)
#   tickets/integration/skills/    Agent verbs (ticket-new, ticket-close, ticket-ready)
#   tickets/integration/settings.json  PostToolUse validation hook (Claude)
#   tickets/                       Ticket directory + validator source
#   .git/hooks/pre-commit          Validation on commit
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
if [ -f "$DEST/tickets/tools/go/main.go" ] && diff -q "$SRC/tickets/tools/go/main.go" "$DEST/tickets/tools/go/main.go" >/dev/null 2>&1; then
    skip "tickets/tools/go/"
else
    cp "$SRC/tickets/tools/go/main.go" "$DEST/tickets/tools/go/"
    cp "$SRC/tickets/tools/go/go.mod"  "$DEST/tickets/tools/go/"
    ok "tickets/tools/go/ (validator source)"
fi

# --- Spec ---
mkdir -p "$DEST/tickets"
if [ -f "$DEST/tickets/spec-erg-v1.md" ] && diff -q "$SRC/tickets/spec-erg-v1.md" "$DEST/tickets/spec-erg-v1.md" >/dev/null 2>&1; then
    skip "tickets/spec-erg-v1.md"
else
    cp "$SRC/tickets/spec-erg-v1.md" "$DEST/tickets/"
    ok "tickets/spec-erg-v1.md (format spec)"
fi

# --- Skills ---
mkdir -p "$DEST/tickets/integration/skills"
if [ -d "$DEST/tickets/integration/skills/ticket-new" ] && diff -rq "$SRC/tickets/integration/skills/" "$DEST/tickets/integration/skills/" >/dev/null 2>&1; then
    skip "tickets/integration/skills/"
else
    cp -r "$SRC/tickets/integration/skills/"* "$DEST/tickets/integration/skills/"
    ok "tickets/integration/skills/ (ticket-new, ticket-close, ticket-ready)"
fi

# --- Settings (hooks) ---
mkdir -p "$DEST/tickets/integration"
if [ -f "$DEST/tickets/integration/settings.json" ] && grep -qF "erg validate" "$DEST/tickets/integration/settings.json" 2>/dev/null; then
    skip "tickets/integration/settings.json"
else
    if [ -f "$DEST/tickets/integration/settings.json" ]; then
        # Don't overwrite existing settings — warn user
        echo "  ! tickets/integration/settings.json exists — merge manually from tickets/integration/settings.json"
        echo "    Merge the PostToolUse validation hook from tickets/integration/settings.json"
    else
        cp "$SRC/tickets/integration/settings.json" "$DEST/tickets/integration/settings.json"
        ok "tickets/integration/settings.json (validation hook)"
    fi
fi

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
        tail -n +2 "$SRC/tickets/integration/hooks/pre-commit" >> "$HOOK_FILE"
        printf '%s end\n' "$MARKER" >> "$HOOK_FILE"
    else
        mkdir -p "$DEST/.git/hooks"
        printf '#!/bin/sh\n\n%s begin\n' "$MARKER" > "$HOOK_FILE"
        tail -n +2 "$SRC/tickets/integration/hooks/pre-commit" >> "$HOOK_FILE"
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
echo "  To put erg on PATH: make install-erg-binary"
echo ""
