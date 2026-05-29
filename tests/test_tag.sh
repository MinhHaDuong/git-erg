#!/bin/sh
# Integration tests for: erg tag, erg untag
set -eu

ERG="${ERG_BIN:-build/erg}"
FIXTURES=$(mktemp -d)
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

trap 'rm -rf "$FIXTURES"' EXIT

echo "=== erg tag / untag ==="

# --- Tag an open ticket ---
cat > "$FIXTURES/7001-taggable.erg" <<'EOF'
%erg 0.1
Title: Taggable ticket
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Test body.
EOF

OUT=$(ERG_AUTHOR=testuser $ERG tag 7001 needs-human "$FIXTURES")
if [ "$OUT" = "TAGGED" ]; then
    if grep -q "^Tag: needs-human$" "$FIXTURES/7001-taggable.erg"; then
        if grep -q "testuser tag needs-human" "$FIXTURES/7001-taggable.erg"; then
            pass "tag open ticket"
        else
            fail "tag open ticket (missing log line)"
        fi
    else
        fail "tag open ticket (no Tag: header)"
    fi
else
    fail "tag open ticket (output: $OUT)"
fi

# --- Tag header appears in preamble (before --- log ---) ---
tag_line_no=$(grep -n "^Tag: needs-human$" "$FIXTURES/7001-taggable.erg" | head -1 | cut -d: -f1)
log_line_no=$(grep -n "^--- log ---$" "$FIXTURES/7001-taggable.erg" | cut -d: -f1)
if [ -n "$tag_line_no" ] && [ -n "$log_line_no" ] && [ "$tag_line_no" -lt "$log_line_no" ]; then
    pass "tag header in preamble before --- log ---"
else
    fail "tag header in preamble before --- log ---"
fi

# --- Idempotent: tag already present ---
OUT=$(ERG_AUTHOR=testuser $ERG tag 7001 needs-human "$FIXTURES")
if [ "$OUT" = "TAGGED (already)" ]; then
    pass "tag idempotent returns TAGGED (already)"
else
    fail "tag idempotent returns TAGGED (already) (output: $OUT)"
fi

# --- Tag does not duplicate header on idempotent call ---
count=$(grep -c "^Tag: needs-human$" "$FIXTURES/7001-taggable.erg" || true)
if [ "$count" -eq 1 ]; then
    pass "tag idempotent does not duplicate header"
else
    fail "tag idempotent does not duplicate header (count: $count)"
fi

# --- Untag removes the header ---
OUT=$(ERG_AUTHOR=testuser $ERG untag 7001 needs-human "$FIXTURES")
if [ "$OUT" = "UNTAGGED" ]; then
    if grep -q "^Tag: needs-human$" "$FIXTURES/7001-taggable.erg"; then
        fail "untag removes header (header still present)"
    else
        if grep -q "testuser untag needs-human" "$FIXTURES/7001-taggable.erg"; then
            pass "untag removes header and logs"
        else
            fail "untag removes header (missing log line)"
        fi
    fi
else
    fail "untag removes header (output: $OUT)"
fi

# --- Untag idempotent: tag not present ---
OUT=$(ERG_AUTHOR=testuser $ERG untag 7001 needs-human "$FIXTURES")
if [ "$OUT" = "NOT TAGGED" ]; then
    pass "untag idempotent returns NOT TAGGED"
else
    fail "untag idempotent returns NOT TAGGED (output: $OUT)"
fi

# --- Unknown tag rejected (tag) ---
cat > "$FIXTURES/7002-unknown.erg" <<'EOF'
%erg 0.1
Title: Unknown tag test
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
if $ERG tag 7002 invalid-tag "$FIXTURES" 2>/dev/null; then
    fail "tag rejects unknown tag"
else
    pass "tag rejects unknown tag"
fi

# --- Unknown tag rejected (untag) ---
if $ERG untag 7002 invalid-tag "$FIXTURES" 2>/dev/null; then
    fail "untag rejects unknown tag"
else
    pass "untag rejects unknown tag"
fi

# --- Missing args (tag) ---
if $ERG tag 2>/dev/null; then
    fail "tag missing args exits non-zero"
else
    pass "tag missing args exits non-zero"
fi

# --- Missing args (untag) ---
if $ERG untag 2>/dev/null; then
    fail "untag missing args exits non-zero"
else
    pass "untag missing args exits non-zero"
fi

# --- Non-existent ID (tag) ---
if $ERG tag 8888 needs-human "$FIXTURES" 2>/dev/null; then
    fail "tag non-existent ID exits non-zero"
else
    pass "tag non-existent ID exits non-zero"
fi

# --- Non-existent ID (untag) ---
if $ERG untag 8888 needs-human "$FIXTURES" 2>/dev/null; then
    fail "untag non-existent ID exits non-zero"
else
    pass "untag non-existent ID exits non-zero"
fi

# --- Custom .ergrc tag vocabulary ---
cat > "$FIXTURES/.ergrc" <<'EOF'
[tags]
custom-tag
EOF
cat > "$FIXTURES/7003-custom.erg" <<'EOF'
%erg 0.1
Title: Custom tag vocab
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
OUT=$(ERG_AUTHOR=testuser $ERG tag 7003 custom-tag "$FIXTURES")
if [ "$OUT" = "TAGGED" ]; then
    pass "tag accepts custom .ergrc tag"
else
    fail "tag accepts custom .ergrc tag (output: $OUT)"
fi
# Default tag should be rejected when .ergrc overrides
if $ERG tag 7003 needs-human "$FIXTURES" 2>/dev/null; then
    fail "tag rejects default tag when .ergrc overrides"
else
    pass "tag rejects default tag when .ergrc overrides"
fi
rm -f "$FIXTURES/.ergrc"

# --- Validate tagged ticket passes erg check ---
cat > "$FIXTURES/7004-validate.erg" <<'EOF'
%erg 0.1
Title: Validate after tag
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
$ERG tag 7004 needs-human "$FIXTURES" > /dev/null
if $ERG validate "$FIXTURES/7004-validate.erg" > /dev/null 2>&1; then
    pass "tagged ticket passes erg validate"
else
    fail "tagged ticket passes erg validate"
fi

# --- Round-trip: tag then untag leaves ticket valid ---
$ERG untag 7004 needs-human "$FIXTURES" > /dev/null
if $ERG validate "$FIXTURES/7004-validate.erg" > /dev/null 2>&1; then
    pass "round-trip tag/untag leaves valid ticket"
else
    fail "round-trip tag/untag leaves valid ticket"
fi

# --- Autofix on write: tag strips a pre-existing interior header blank ---
cat > "$FIXTURES/7010-interior-blank.erg" <<'EOF'
%erg 0.1
Title: Interior blank
Created: 2026-01-01
Author: claude

Blocked-by: 0001

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
$ERG tag 7010 needs-human "$FIXTURES" > /dev/null
# The blank between Author: and Blocked-by: must be gone after the write.
if awk '/^Author: claude$/{getline n; if(n==""){found=1}} END{exit !found}' "$FIXTURES/7010-interior-blank.erg"; then
    fail "tag autofix removes interior header blank"
else
    pass "tag autofix removes interior header blank"
fi
if $ERG check "$FIXTURES" 2>&1 | grep -q "7010-interior-blank.*blank line inside header block"; then
    fail "tagged file no longer warns on interior header blank"
else
    pass "tagged file no longer warns on interior header blank"
fi

# --- untag removes the parser-tolerated 'Tag : value' (whitespace before colon) ---
# Detection (Erg.Tags) and removal (removeTagLine) must agree on this spelling.
cat > "$FIXTURES/7011-ws-tag.erg" <<'EOF'
%erg 0.1
Title: Whitespace before colon
Created: 2026-01-01
Author: claude
Tag : needs-human

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
OUT=$($ERG untag 7011 needs-human "$FIXTURES")
if [ "$OUT" = "UNTAGGED" ] && ! grep -Eq "^Tag" "$FIXTURES/7011-ws-tag.erg"; then
    pass "untag removes whitespace-before-colon Tag line"
else
    fail "untag removes whitespace-before-colon Tag line (out=$OUT, left: $(grep -E '^Tag' "$FIXTURES/7011-ws-tag.erg"))"
fi

echo "tag: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
