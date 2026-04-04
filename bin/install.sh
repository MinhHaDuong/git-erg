#!/bin/sh
# git-erg install — set up git-erg in a target project.
#
# Usage:
#   ./bin/install.sh /path/to/project
#   make install DEST=/path/to/project

set -e

usage() {
    echo "Usage: $0 DEST" >&2
    echo "  DEST  Path to a git repository to install git-erg into" >&2
    exit 1
}

DEST="${1:?$(usage)}"

# Validate destination
if [ ! -d "$DEST/.git" ]; then
    echo "ERROR: $DEST is not a git repository (.git/ not found)" >&2
    exit 1
fi

SRC="$(cd "$(dirname "$0")/.." && pwd)"

echo "Installing git-erg into $DEST ..."

# --- Build the binary ---
echo "  Building erg binary..."
(cd "$SRC/tickets/tools/go" && go build -o erg .)

# --- Copy tool source (not the binary) ---
echo "  Copying tool source..."
mkdir -p "$DEST/tickets/tools/go"
cp "$SRC/tickets/tools/go/main.go" "$DEST/tickets/tools/go/"
cp "$SRC/tickets/tools/go/go.mod"  "$DEST/tickets/tools/go/"

# --- Copy rules ---
echo "  Copying rules..."
mkdir -p "$DEST/.claude/rules"
cp "$SRC/rules/tickets.md" "$DEST/.claude/rules/"

# --- Copy skills ---
echo "  Copying skills..."
mkdir -p "$DEST/.claude/skills"
cp -r "$SRC/claude/skills/"* "$DEST/.claude/skills/"

# --- Create tickets directory (without sample tickets) ---
echo "  Creating tickets directories..."
mkdir -p "$DEST/tickets/archive"

# --- .gitkeep for archive ---
if [ ! -f "$DEST/tickets/archive/.gitkeep" ]; then
    touch "$DEST/tickets/archive/.gitkeep"
fi

# --- .gitignore: append binary if not already present ---
GITIGNORE_LINE="tickets/tools/go/erg"
if [ -f "$DEST/.gitignore" ]; then
    if ! grep -qxF "$GITIGNORE_LINE" "$DEST/.gitignore"; then
        echo "  Appending erg binary to .gitignore..."
        printf '\n# git-erg compiled binary\n%s\n' "$GITIGNORE_LINE" >> "$DEST/.gitignore"
    fi
else
    echo "  Creating .gitignore with erg binary..."
    printf '# git-erg compiled binary\n%s\n' "$GITIGNORE_LINE" > "$DEST/.gitignore"
fi

# --- Pre-commit hook: append with markers if not already present ---
HOOK_FILE="$DEST/.git/hooks/pre-commit"
MARKER="# --- git-erg ---"

if [ -f "$HOOK_FILE" ] && grep -qF "$MARKER" "$HOOK_FILE"; then
    echo "  Pre-commit hook already contains git-erg section, skipping."
else
    echo "  Installing pre-commit hook fragment..."
    if [ -f "$HOOK_FILE" ]; then
        # Append to existing hook
        printf '\n%s begin\n' "$MARKER" >> "$HOOK_FILE"
        # Append the hook body (skip the shebang line)
        tail -n +2 "$SRC/hooks/pre-commit" >> "$HOOK_FILE"
        printf '%s end\n' "$MARKER" >> "$HOOK_FILE"
    else
        # Create new hook with shebang + markers
        mkdir -p "$DEST/.git/hooks"
        printf '#!/bin/sh\n\n%s begin\n' "$MARKER" > "$HOOK_FILE"
        tail -n +2 "$SRC/hooks/pre-commit" >> "$HOOK_FILE"
        printf '%s end\n' "$MARKER" >> "$HOOK_FILE"
    fi
    chmod +x "$HOOK_FILE"
fi

# --- Build binary in the target project ---
echo "  Building erg binary in target project..."
(cd "$DEST/tickets/tools/go" && go build -o erg .)

echo ""
echo "Done. git-erg installed into $DEST"
echo ""
echo "Next steps:"
echo "  cd $DEST"
echo "  Add the Makefile targets from git-erg (see README.md)"
echo "  Create your first ticket: /ticket-new or write a .erg file"
