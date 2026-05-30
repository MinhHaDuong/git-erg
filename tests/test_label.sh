#!/bin/sh
# Integration tests for: erg label, erg unlabel
set -eu

ERG="${ERG_BIN:-build/erg}"
FIXTURES=$(mktemp -d)
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

trap 'rm -rf "$FIXTURES"' EXIT

echo "=== erg label / unlabel ==="

# --- Label an open ticket ---
cat > "$FIXTURES/7001-labelable.erg" <<'EOF'
%erg 0.1
Title: Labelable ticket
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Test body.
EOF

OUT=$(ERG_AUTHOR=testuser $ERG label 7001 needs-human "$FIXTURES")
if [ "$OUT" = "LABELED" ]; then
    if grep -q "^Label: needs-human$" "$FIXTURES/7001-labelable.erg"; then
        if grep -q "testuser label needs-human" "$FIXTURES/7001-labelable.erg"; then
            pass "label open ticket"
        else
            fail "label open ticket (missing log line)"
        fi
    else
        fail "label open ticket (no Label: header)"
    fi
else
    fail "label open ticket (output: $OUT)"
fi

# --- Label header appears in preamble (before --- log ---) ---
label_line_no=$(grep -n "^Label: needs-human$" "$FIXTURES/7001-labelable.erg" | head -1 | cut -d: -f1)
log_line_no=$(grep -n "^--- log ---$" "$FIXTURES/7001-labelable.erg" | cut -d: -f1)
if [ -n "$label_line_no" ] && [ -n "$log_line_no" ] && [ "$label_line_no" -lt "$log_line_no" ]; then
    pass "label header in preamble before --- log ---"
else
    fail "label header in preamble before --- log ---"
fi

# --- Idempotent: label already present ---
OUT=$(ERG_AUTHOR=testuser $ERG label 7001 needs-human "$FIXTURES")
if [ "$OUT" = "LABELED (already)" ]; then
    pass "label idempotent returns LABELED (already)"
else
    fail "label idempotent returns LABELED (already) (output: $OUT)"
fi

# --- Label does not duplicate header on idempotent call ---
count=$(grep -c "^Label: needs-human$" "$FIXTURES/7001-labelable.erg" || true)
if [ "$count" -eq 1 ]; then
    pass "label idempotent does not duplicate header"
else
    fail "label idempotent does not duplicate header (count: $count)"
fi

# --- Unlabel removes the header ---
OUT=$(ERG_AUTHOR=testuser $ERG unlabel 7001 needs-human "$FIXTURES")
if [ "$OUT" = "UNLABELED" ]; then
    if grep -q "^Label: needs-human$" "$FIXTURES/7001-labelable.erg"; then
        fail "unlabel removes header (header still present)"
    else
        if grep -q "testuser unlabel needs-human" "$FIXTURES/7001-labelable.erg"; then
            pass "unlabel removes header and logs"
        else
            fail "unlabel removes header (missing log line)"
        fi
    fi
else
    fail "unlabel removes header (output: $OUT)"
fi

# --- Unlabel idempotent: label not present ---
OUT=$(ERG_AUTHOR=testuser $ERG unlabel 7001 needs-human "$FIXTURES")
if [ "$OUT" = "NOT LABELED" ]; then
    pass "unlabel idempotent returns NOT LABELED"
else
    fail "unlabel idempotent returns NOT LABELED (output: $OUT)"
fi

# --- Unknown label rejected (label) ---
cat > "$FIXTURES/7002-unknown.erg" <<'EOF'
%erg 0.1
Title: Unknown label test
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
if $ERG label 7002 invalid-label "$FIXTURES" 2>/dev/null; then
    fail "label rejects unknown label"
else
    pass "label rejects unknown label"
fi

# --- Unknown label rejected (unlabel) ---
if $ERG unlabel 7002 invalid-label "$FIXTURES" 2>/dev/null; then
    fail "unlabel rejects unknown label"
else
    pass "unlabel rejects unknown label"
fi

# --- Missing args (label) ---
if $ERG label 2>/dev/null; then
    fail "label missing args exits non-zero"
else
    pass "label missing args exits non-zero"
fi

# --- Missing args (unlabel) ---
if $ERG unlabel 2>/dev/null; then
    fail "unlabel missing args exits non-zero"
else
    pass "unlabel missing args exits non-zero"
fi

# --- Non-existent ID (label) ---
if $ERG label 8888 needs-human "$FIXTURES" 2>/dev/null; then
    fail "label non-existent ID exits non-zero"
else
    pass "label non-existent ID exits non-zero"
fi

# --- Non-existent ID (unlabel) ---
if $ERG unlabel 8888 needs-human "$FIXTURES" 2>/dev/null; then
    fail "unlabel non-existent ID exits non-zero"
else
    pass "unlabel non-existent ID exits non-zero"
fi

# --- Custom .ergrc label vocabulary ---
cat > "$FIXTURES/.ergrc" <<'EOF'
[labels]
custom-label
EOF
cat > "$FIXTURES/7003-custom.erg" <<'EOF'
%erg 0.1
Title: Custom label vocab
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
OUT=$(ERG_AUTHOR=testuser $ERG label 7003 custom-label "$FIXTURES")
if [ "$OUT" = "LABELED" ]; then
    pass "label accepts custom .ergrc label"
else
    fail "label accepts custom .ergrc label (output: $OUT)"
fi
# Default label should be rejected when .ergrc overrides
if $ERG label 7003 needs-human "$FIXTURES" 2>/dev/null; then
    fail "label rejects default label when .ergrc overrides"
else
    pass "label rejects default label when .ergrc overrides"
fi
rm -f "$FIXTURES/.ergrc"

# --- Validate labeled ticket passes erg check ---
cat > "$FIXTURES/7004-validate.erg" <<'EOF'
%erg 0.1
Title: Validate after label
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
$ERG label 7004 needs-human "$FIXTURES" > /dev/null
if $ERG validate "$FIXTURES/7004-validate.erg" > /dev/null 2>&1; then
    pass "labeled ticket passes erg validate"
else
    fail "labeled ticket passes erg validate"
fi

# --- Round-trip: label then unlabel leaves ticket valid ---
$ERG unlabel 7004 needs-human "$FIXTURES" > /dev/null
if $ERG validate "$FIXTURES/7004-validate.erg" > /dev/null 2>&1; then
    pass "round-trip label/unlabel leaves valid ticket"
else
    fail "round-trip label/unlabel leaves valid ticket"
fi

# --- Autofix on write: label strips a pre-existing interior header blank ---
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
$ERG label 7010 needs-human "$FIXTURES" > /dev/null
# The blank between Author: and Blocked-by: must be gone after the write.
if awk '/^Author: claude$/{getline n; if(n==""){found=1}} END{exit !found}' "$FIXTURES/7010-interior-blank.erg"; then
    fail "label autofix removes interior header blank"
else
    pass "label autofix removes interior header blank"
fi
if $ERG check "$FIXTURES" 2>&1 | grep -q "7010-interior-blank.*blank line inside header block"; then
    fail "labeled file no longer warns on interior header blank"
else
    pass "labeled file no longer warns on interior header blank"
fi

# --- unlabel removes the parser-tolerated 'Label : value' (whitespace before colon) ---
# Detection (Erg.Labels) and removal (removeLabelLine) must agree on this spelling.
cat > "$FIXTURES/7011-ws-label.erg" <<'EOF'
%erg 0.1
Title: Whitespace before colon
Created: 2026-01-01
Author: claude
Label : needs-human

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
OUT=$($ERG unlabel 7011 needs-human "$FIXTURES")
if [ "$OUT" = "UNLABELED" ] && ! grep -Eq "^Label" "$FIXTURES/7011-ws-label.erg"; then
    pass "unlabel removes whitespace-before-colon Label line"
else
    fail "unlabel removes whitespace-before-colon Label line (out=$OUT, left: $(grep -E '^Label' "$FIXTURES/7011-ws-label.erg"))"
fi

# --- Hard rename: old 'tag'/'untag' commands are gone (no deprecated aliases) ---
if $ERG tag 7001 needs-human "$FIXTURES" >/dev/null 2>&1; then
    fail "erg tag is removed (hard rename, no alias)"
else
    pass "erg tag is removed (hard rename, no alias)"
fi
if $ERG untag 7001 needs-human "$FIXTURES" >/dev/null 2>&1; then
    fail "erg untag is removed (hard rename, no alias)"
else
    pass "erg untag is removed (hard rename, no alias)"
fi

echo "label: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
