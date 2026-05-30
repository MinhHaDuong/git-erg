//go:build scaling

// Scaling regression guard (ticket 0159) for the corpus-heavy erg commands.
//
// Build-tagged `scaling` so it is excluded from `make test`, `make unit-test`,
// and plain `go test ./...` — none of which pass `-tags scaling`. It is slow
// (it builds stores up to ~1000 tickets and profiles five commands) and is a
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
// O(N^2) bytes and trip the ratio hard. TestScalingLinearNegativeControl is the
// permanent proof of teeth: it runs a genuinely O(N^2) probe through the same
// harness and asserts the ratio exceeds the ceiling. Two things slip past the
// guard by construction: allocation *count* (Mallocs) is a poor proxy because
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
// + folder-closure path), labels from the default vocabulary, and varied bodies —
// so the scan, ref-resolution, cycle-detection, and label-vocabulary paths are all
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
			b.WriteString("Label: needs-human\n")
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

// measure runs fn once with stdout and stderr discarded and returns the
// heap-allocation volume (TotalAlloc bytes) it produced and its wall-clock
// duration. TotalAlloc is cumulative and monotonic, so the delta across the two
// reads already reflects only what fn allocated; the GC beforehand just lowers
// the odds of a collection cycle landing inside the timed region.
func measure(t *testing.T, fn func()) (bytes uint64, elapsed time.Duration) {
	t.Helper()
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer devnull.Close()
	savedOut, savedErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = devnull, devnull
	defer func() { os.Stdout, os.Stderr = savedOut, savedErr }()

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	start := time.Now()
	fn()
	elapsed = time.Since(start)
	runtime.ReadMemStats(&m2)
	return m2.TotalAlloc - m1.TotalAlloc, elapsed
}

// scalingReps is how many times each (command, size) point is measured; the
// minimum is kept. Allocation volume is deterministic for fixed input, but
// min-of-reps rejects the occasional run perturbed by a stray GC allocation or
// scheduling jitter in the timing. Each rep gets a freshly built corpus so the
// mutating probes (close, rm) measure a clean scan every time.
const scalingReps = 3

// profileLadder runs invoke against a freshly built corpus at each size in the
// ladder, keeping the min-of-reps allocation volume and time, logs the table,
// and returns the largest consecutive-size allocation-bytes ratio. A 4x size
// step means a linear command yields ~4.0 and a quadratic one ~16.0.
func profileLadder(t *testing.T, label string, invoke func(dir string) int) (maxRatio float64) {
	t.Helper()
	t.Logf("%-8s %6s %12s %12s %8s", "cmd", "N", "alloc_KB", "time_us", "ratio")
	var prevBytes uint64
	for _, n := range scalingSizes {
		var minBytes uint64
		var minNs int64
		for r := 0; r < scalingReps; r++ {
			dir := t.TempDir()
			buildCorpus(t, dir, n)
			b, elapsed := measure(t, func() { invoke(dir) })
			if r == 0 || b < minBytes {
				minBytes = b
			}
			if ns := elapsed.Nanoseconds(); r == 0 || ns < minNs {
				minNs = ns
			}
		}

		ratioStr := "--"
		if prevBytes > 0 {
			r := float64(minBytes) / float64(prevBytes)
			ratioStr = fmt.Sprintf("%.2f", r)
			if r > maxRatio {
				maxRatio = r
			}
		}
		t.Logf("%-8s %6d %12d %12d %8s", label, n, minBytes/1024, minNs/1000, ratioStr)
		prevBytes = minBytes
	}
	return maxRatio
}

// scalingCommands are the corpus-heavy commands whose in-process work scales
// with corpus size. invoke runs the command against a freshly built corpus dir;
// mutating commands (close, rm) therefore measure a clean, repeatable scan each
// time.
//
// next-id is deliberately absent. Its dominant work in production is a
// `git for-each-ref` / `ls-tree` subprocess (nextid.go) that ReadMemStats — a
// parent-heap probe — cannot see; and against this non-git temp corpus those
// passes no-op entirely, leaving only the trivial Pass-1 WalkDir, which barely
// allocates. Either way it is not a meaningful allocation-scaling target, so
// measuring it would report a hollow curve.
var scalingCommands = []struct {
	name   string
	invoke func(dir string) int
}{
	{"check", func(d string) int { return cmdCheck([]string{d}) }},
	{"list", func(d string) int { return cmdList([]string{d}) }},
	{"ready", func(d string) int { return cmdReady([]string{d}) }},
	// close/rm exercise the full-corpus dependent scan (clearBlockedByRefs):
	// ticket 0001 is Blocked-by of 0002, so both rewrite a dependent. The
	// rewrite itself is in-process (atomic file writes, no git), so the
	// measured cost is the loadErgs scan — a genuine O(N) in-process target.
	{"close", func(d string) int { return cmdClose([]string{"0001", "scaling bench", d}) }},
	{"rm", func(d string) int { return cmdRm([]string{"0001", d, "--force"}) }},
}

func TestScalingLinear(t *testing.T) {
	for _, c := range scalingCommands {
		t.Run(c.name, func(t *testing.T) {
			// Guard against a command silently starting to fail: a non-zero
			// exit would taint the scaling signal (an error path allocates
			// differently). Run once on a small corpus and assert success
			// before trusting the ladder. Checked here, not in profileLadder,
			// because the negative control's probe returns a count, not a status.
			dir := t.TempDir()
			buildCorpus(t, dir, scalingSizes[0])
			if ret := c.invoke(dir); ret != 0 {
				t.Fatalf("%s returned non-zero exit %d — scaling signal is untrustworthy", c.name, ret)
			}
			if maxRatio := profileLadder(t, c.name, c.invoke); maxRatio > ratioCeiling {
				t.Errorf("%s: worst alloc-bytes ratio %.2f exceeds %.1f — super-linear allocation (4x is linear, ~16x quadratic)",
					c.name, maxRatio, ratioCeiling)
			}
		})
	}
}

// quadraticProbe deliberately allocates O(N²) bytes: for each ticket it builds
// a slice of every ticket's filename. It is not a command — it exists only to
// drive the negative control below.
func quadraticProbe(dir string) int {
	tickets, _ := loadErgs(dir)
	var sink [][]string
	for range tickets {
		row := make([]string, 0, len(tickets))
		for j := range tickets {
			row = append(row, tickets[j].Filename())
		}
		sink = append(sink, row)
	}
	return len(sink)
}

// TestScalingLinearNegativeControl is the falsifiable companion to
// TestScalingLinear (the AGENTS.md / 0146 convention: every guard ships a
// control that proves it trips). It runs quadraticProbe — genuinely O(N²) in
// allocated bytes — through the same ladder and asserts the ratio exceeds the
// ceiling. If this ever passes, the guard has gone blind and TestScalingLinear's
// green is worthless.
func TestScalingLinearNegativeControl(t *testing.T) {
	maxRatio := profileLadder(t, "quad-ctl", quadraticProbe)
	if maxRatio <= ratioCeiling {
		t.Errorf("negative control: O(N²) probe ratio %.2f did not exceed %.1f — the scaling guard is vacuous",
			maxRatio, ratioCeiling)
	}
}

// TestScalingCorpusValid pins the "validates clean" invariant in code rather
// than prose: a malformed fixture would silently make the whole ladder measure
// an error path instead of the success path. It builds the corpus and asserts
// validateCorpus reports zero errors. Warnings (e.g. an open ticket whose
// Blocked-by points at a closed one) are not errors and are allowed.
func TestScalingCorpusValid(t *testing.T) {
	dir := t.TempDir()
	buildCorpus(t, dir, scalingSizes[len(scalingSizes)-1])

	tickets, parseErrs := loadErgs(dir)
	if len(tickets) == 0 {
		t.Fatal("buildCorpus produced no tickets")
	}
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if errs := validateCorpus(tickets, parseErrs, cfg); len(errs) > 0 {
		t.Fatalf("fixture is not clean: %d corpus error(s); first: %s", len(errs), errs[0])
	}
}
