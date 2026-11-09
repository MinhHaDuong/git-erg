# Sourced by the shell suites, never run on its own (no test_ prefix, so it
# is not a suite): claims are required in every store (ticket 0307).
#
# claim_store DIR makes DIR a compliant store with plain files, as a hand
# would, never with the binary under test: DIR/.claims/.keep and, for every
# ticket under DIR that has no claim yet, DIR/.claims/NNNN holding
# "SLUG 00000000". Call it after the fixture's tickets are written (or before,
# for an empty store) so a test fails for its own defect, not for the store's
# missing .claims/. An existing claim is left as it is.
claim_store() {
    mkdir -p "$1/.claims"
    [ -e "$1/.claims/.keep" ] || : > "$1/.claims/.keep"
    find "$1" -path "$1/.claims" -prune -o -type f -name '[0-9][0-9][0-9][0-9]-*.erg' -print |
        while IFS= read -r f; do
            b=${f##*/}
            id=${b%%-*}
            slug=${b#*-}
            slug=${slug%.erg}
            [ -e "$1/.claims/$id" ] || printf '%s 00000000\n' "$slug" > "$1/.claims/$id"
        done
}
