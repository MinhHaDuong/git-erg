package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Data-safety write path (ticket 0149).
//
// erg's one job is custody of tickets; losing or corrupting one is the worst,
// irreversible failure. Every ticket-file mutation funnels through this file so
// the safety properties are implemented once and audited once, rather than
// re-derived in each command:
//
//   - atomic replace      — write a temp then rename over the target, so a
//                           concurrent reader never sees a partial file and a
//                           crash leaves the complete old file or the complete
//                           new one, never a truncated one (atomicWriteFile).
//   - validate-before-replace — never write content that no longer parses as a
//                           %erg file over a good ticket (writeTicketAtomic).
//   - write-confinement   — refuse a target that resolves outside the store
//                           root: a fail-safe against a fat-fingered DIR/ID or
//                           an overeager agent computing a path outside
//                           tickets/. Not a security boundary — a determined
//                           caller bypasses it (the adversarial path-traversal
//                           review is ticket 0151); here it stops the common
//                           mistake before it lands (writeTicketAtomic).

// errOutsideStore is returned by writeTicketAtomic when a mutation's target
// resolves outside the store root.
type errOutsideStore struct {
	Target string
	Store  string
}

func (e *errOutsideStore) Error() string {
	return fmt.Sprintf("refusing to write %s: target is outside the ticket store %s", e.Target, e.Store)
}

// withinStore reports whether target resolves inside storeRoot's subtree.
// Both paths are made absolute first so a relative target ("../etc/x") or an
// absolute one that escapes the store is caught. storeRoot itself counts as
// inside (rel == ".").
func withinStore(storeRoot, target string) (bool, error) {
	absStore, err := filepath.Abs(storeRoot)
	if err != nil {
		return false, err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(absStore, absTarget)
	if err != nil {
		return false, err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false, nil
	}
	return true, nil
}

// atomicWriteFile writes data to path via a temp-file-then-rename so a reader
// never sees a partial file and a killed process leaves either the complete old
// file or the complete new one, never a zero-length/truncated one. The temp is
// created in the same directory as the target (so the rename is a same-
// filesystem atomic operation) by os.CreateTemp, which uses O_EXCL — a stray
// temp from an earlier crash is never silently reused. On any failure the temp
// is removed and the original (if any) is left untouched.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// Clean the temp up on every error path; cleared to "" once the rename
	// succeeds so we do not remove the now-live target.
	defer func() {
		if tmpName != "" {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	// fsync the contents before the rename so the bytes are durable on disk,
	// not just in the page cache, when the rename publishes them.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// CreateTemp makes the file 0600; restore the intended mode before publish.
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	tmpName = "" // rename done — the temp is now the target; do not remove it
	return nil
}

// writeTicketAtomic is the single audited path every ticket-file mutation
// funnels through. In order it: (1) confines the write to the resolved store
// (fail-safe rail), (2) refuses to replace a good ticket with content that no
// longer parses as a %erg file (validate-before-replace), then (3) replaces the
// file atomically (atomicWriteFile). Any refusal leaves the original untouched.
func writeTicketAtomic(storeRoot, path string, content []byte) error {
	ok, err := withinStore(storeRoot, path)
	if err != nil {
		return err
	}
	if !ok {
		absStore, _ := filepath.Abs(storeRoot)
		return &errOutsideStore{Target: path, Store: absStore}
	}
	if _, errs := parseErgBytes(content, path); len(errs) > 0 {
		return fmt.Errorf("refusing to write invalid %%erg content to %s: %s", path, errs[0])
	}
	return atomicWriteFile(path, content, 0644)
}
