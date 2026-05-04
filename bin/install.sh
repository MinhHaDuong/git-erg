#!/bin/sh
# git-erg install — set up git-erg in a target project.
#
# Usage:
#   ./bin/install.sh /path/to/project
#   make install DEST=/path/to/project
#
# Installs:
#   tickets/                       Ticket directory (README, spec, tools, integration)
#   AGENTS.md                      One-line pointer for agents (appended if exists)
#   .git/hooks/pre-commit          Validation on commit (appended if exists)
#   .gitignore                     Excludes compiled erg binary
#
# Idempotent. Safe to re-run.

set -e

MARKER="# --- git-erg ---"
AGENTS_LINE="This repo stores issues in a local folder in the \`erg v1\` format, see \`tickets/README.md\` when you need to use it."

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

# --- tickets/ ---
mkdir -p "$DEST/tickets"
cp -r "$SRC/tickets/tools" "$DEST/tickets/"
cp -r "$SRC/tickets/integration" "$DEST/tickets/"
cp "$SRC/tickets/README.md" "$DEST/tickets/"
cp "$SRC/tickets/spec-erg-v1.md" "$DEST/tickets/"
ok "tickets/ (README, spec, tools, integration)"

# --- AGENTS.md ---
if [ -f "$DEST/AGENTS.md" ] && grep -qF "erg v1" "$DEST/AGENTS.md"; then
    skip "AGENTS.md"
elif [ -f "$DEST/AGENTS.md" ]; then
    printf '\n%s\n' "$AGENTS_LINE" >> "$DEST/AGENTS.md"
    ok "AGENTS.md (appended git-erg pointer)"
else
    printf '%s\n' "$AGENTS_LINE" > "$DEST/AGENTS.md"
    ok "AGENTS.md"
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
echo "  Done. Create tickets/0001-*.erg to get started."
echo ""
