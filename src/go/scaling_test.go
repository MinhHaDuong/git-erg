//go:build scaling

// Scaling regression guard (ticket 0159) for the corpus-heavy erg commands.
//
// Build-tagged `scaling` so it is excluded from `make test`, `make unit-test`,
// and plain `go test ./...` — none of which pass `-tags scaling`. It is slow
// (it builds stores up to ~1000 tickets and profiles six commands) and is a
// regression guard, not a per-merge check. Run it on demand: `make test-scaling`.
//
// 0154 named the real O(N^2) risk as "the dependent/ref scans in check/rm/close".
// This test measures heap-allocation *volume* (bytes) — deterministic for fixed
// input, so immune to CI-box speed — and wall-clock time across a 3-point 4x
// ladder {64, 256, 1024}, then asserts each command's allocated bytes grow
// linearly: the ratio between consecutive sizes must stay below 6.0 (linear is
// ~4.0; a quadratic regression would show ~16.0). Time is logged, not asserted
// (it is too CI-box-dependent to gate on).
//
// What this catches, and what it does not. Allocation *volume* is the assertion
// signal because the realistic O(N^2) regressions in this code — a nested scan
// that builds a per-pair slice/map, re-parsing the corpus inside a loop — move
// O(N^2) bytes and trip the ratio hard (verified by mutation: a per-ticket
// []string of all IDs pushes the top-step ratio to ~16). Two things slip past
// it by construction: allocation *count* (Mallocs) is a poor proxy because
// `append` amortises an O(N^2)-byte build into O(N log N) allocation events; and
// a purely compute-bound O(N^2) that allocates nothing (e.g. a tight comparison
// scan over already-parsed structs) leaves both bytes and a few-ms time delta
// indistinguishable from noise at practical N. That residual is owned by 0154's
// wall-clock backstop plus code review — the same defense-in-depth posture the
// author accepted for 0154 (#169). This guard is the deterministic layer for the
// common, allocation-heavy failure mode, not a proof of asymptotic linearity.
//
// The corpus is deliberately non-trivial — a realistic acyclic DAG (multiple
// Blocked-by edges per ticket, fan-in), a closed/ subdirectory (recursive walk
// + folder-closure path), tags from the default vocabulary, and varied bodies —
// so the scan, ref-resolution, cycle-detection, and tag-vocabulary paths are all
// exercised, not just a flat parse loop.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// scalingSizes is a geometric ladder with a 4x step. Wide spacing makes the
// asymptotic ratio dominate fixed per-invocation overhead, and 4x sharpens the
// linear-vs-quadratic signal (expect ~4.0 linear, ~16.0 quadratic).
var scalingSizes = []int{64, 256, 1024}

// ratioCeiling separates linear (~4.0) from quadratic (~16.0) allocation-volume
// growth. Generous headroom over 4.0 absorbs single-run measurement noise
// without admitting a genuine super-linear regression.
const ratioCeiling = 6.0

// corpusDeps returns the Blocked-by targets for ticket i: the immediate
// predecessor plus a second earlier edge (i/2) for fan-in. All targets are
// < i, so the graph is a DAG (acyclic) and every ref resolves.
func corpusDeps(i int) []int {
	if i <= 1 {
		return nil
	}
	deps := []int{i - 1}
	if i > 3 {
		if d := i / 2; d != i-1 {
			deps = append(deps, d)
		}
	}
	return deps
}

// buildCorpus writes a non-trivial store of n tickets into dir. Open tickets
// live at the top level; roughly one in seven is closed and lives in closed/
// with a Closed: header (exercises the recursive walk and folder-closure path).
// Tickets 1-3 are always open so the close/rm probes can target ticket 0001.
func buildCorpus(t *testing.T, dir string, n int) {
	t.Helper()
	closedDir := filepath.Join(dir, "closed")
	if err := os.MkdirAll(closedDir, 0o755); err != nil {
		t.Fatalf("mkdir closed/: %v", err)
	}
	for i := 1; i <= n; i++ {
		id := fmt.Sprintf("%04d", i)
		isClosed := i%7 == 0 && i > 3

		var b strings.Builder
		b.WriteString("%erg 0.1\n")
		fmt.Fprintf(&b, "Title: Synthetic ticket %s\n", id)
		b.WriteString("Created: 2026-01-01\n")
		b.WriteString("Author: bench\n")
		if isClosed {
			b.WriteString("Closed: delivered in scaling fixture\n")
		}
		for _, dep := range corpusDeps(i) {
			fmt.Fprintf(&b, "Blocked-by: %04d\n", dep)
		}
		if i%5 == 0 {
			b.WriteString("Tag: needs-human\n")
		}
		b.WriteString("\n--- log ---\n")
		b.WriteString("2026-01-01T00:00Z bench created\n")
		if isClosed {
			b.WriteString("2026-01-02T00:00Z bench note delivered\n")
		}
		b.WriteString("\n--- body ---\n")
		fmt.Fprintf(&b, "## Context\nSynthetic ticket %s for the scaling fixture.\n\n", id)
		b.WriteString("## Notes\nA few lines of body text so files are not degenerate.\n")

		name := id + "-synth-" + id + ".erg"
		target := filepath.Join(dir, name)
		if isClosed {
			target = filepath.Join(closedDir, name)
		}
		if err := os.WriteFile(target, []byte(b.String()), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

// measure runs fn once with stdout discarded and returns the heap-allocation
// volume (TotalAlloc bytes) it produced and its wall-clock duration. The GC
// before sampling makes the TotalAlloc delta reflect only fn's own allocations.
func measure(t *testing.T, fn func()) (bytes uint64, elapsed time.Duration) {
	t.Helper()
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer devnull.Close()
	saved := os.Stdout
	os.Stdout = devnull
	defer func() { os.Stdout = saved }()

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	start := time.Now()
	fn()
	elapsed = time.Since(start)
	runtime.ReadMemStats(&m2)
	return m2.TotalAlloc - m1.TotalAlloc, elapsed
}

// scalingCommands are the corpus-heavy commands 0154 flagged. invoke runs the
// command against a freshly built corpus dir; mutating commands (close, rm)
// therefore measure a clean, repeatable scan each time.
var scalingCommands = []struct {
	name   string
	invoke func(dir string) int
}{
	{"check", func(d string) int { return cmdCheck([]string{d}) }},
	{"list", func(d string) int { return cmdList([]string{d}) }},
	{"ready", func(d string) int { return cmdReady([]string{d}) }},
	{"next-id", func(d string) int { return cmdNextID([]string{d}) }},
	// close/rm exercise the full-corpus dependent scan (clearBlockedByRefs):
	// ticket 0001 is Blocked-by of 0002, so both rewrite a dependent.
	{"close", func(d string) int { return cmdClose([]string{"0001", "scaling bench", d}) }},
	{"rm", func(d string) int { return cmdRm([]string{"0001", d, "--force"}) }},
}

func TestScalingLinear(t *testing.T) {
	for _, c := range scalingCommands {
		t.Run(c.name, func(t *testing.T) {
			t.Logf("%-8s %6s %12s %12s %8s", "cmd", "N", "alloc_KB", "time_us", "ratio")
			var prevBytes uint64
			var prevN int
			for _, n := range scalingSizes {
				dir := t.TempDir()
				buildCorpus(t, dir, n)

				allocBytes, elapsed := measure(t, func() { c.invoke(dir) })

				ratioStr := "--"
				if prevBytes > 0 {
					r := float64(allocBytes) / float64(prevBytes)
					ratioStr = fmt.Sprintf("%.2f", r)
					if r > ratioCeiling {
						t.Errorf("%s: alloc-bytes ratio %d->%d = %.2f exceeds %.1f — super-linear allocation (4x is linear, ~16x quadratic)",
							c.name, prevN, n, r, ratioCeiling)
					}
				}
				t.Logf("%-8s %6d %12d %12d %8s", c.name, n, allocBytes/1024, elapsed.Microseconds(), ratioStr)
				prevBytes = allocBytes
				prevN = n
			}
		})
	}
}
