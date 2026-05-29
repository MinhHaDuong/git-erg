//go:build scaling

// Resource-hygiene guard (ticket 0165), sibling to scaling_test.go.
//
// scaling_test.go gates allocation *volume* (TotalAlloc) — the signal for an
// O(N^2) churn regression. It is blind to OS-resource leaks. This file adds two
// hygiene assertions that run under the same `scaling` build tag and the same
// `make test-scaling` target, so the scaling guard and the hygiene guard are
// exercised together and neither can be forgotten:
//
//   - TestScalingFDHygiene: every corpus-heavy command returns with no more open file
//     descriptors than it started with (erg shells out to git and opens files;
//     a missing Close or unreaped child pipe leaks an fd).
//   - TestScalingHeapRetention: the read-only commands hold no live heap once they
//     return. HeapAlloc after a forced GC is the bytes still reachable — the
//     retention signal TotalAlloc cannot see.
//
// CPU / nice: erg deliberately does NOT renice itself — process priority is the
// invoker's job (nice/ionice/systemd), and erg has no goroutines so it never
// saturates cores. There is therefore nothing to assert about scheduling here;
// the rationale lives in tickets/0169.

package main

import (
	"os"
	"runtime"
	"testing"
)

// openFDCount returns the number of open file descriptors for this process via
// /proc/self/fd (Linux-only; the scaling suite already targets Linux CI, so
// elsewhere we skip rather than fail). ReadDir opens and closes its own
// directory fd before returning, so the count is stable across calls.
func openFDCount(t *testing.T) int {
	t.Helper()
	ents, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Skipf("/proc/self/fd unavailable (%v); fd-leak check is Linux-only", err)
	}
	return len(ents)
}

// liveHeapBytes returns bytes of still-reachable heap after a forced GC. Unlike
// TotalAlloc (cumulative churn, the scaling_test.go signal), HeapAlloc after GC
// is exactly what the program is still holding — the retention signal.
func liveHeapBytes() uint64 {
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// withDiscardedStdout runs fn with os.Stdout redirected to /dev/null so command
// output neither prints nor pollutes the measurement.
func withDiscardedStdout(t *testing.T, fn func()) {
	t.Helper()
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer devnull.Close()
	saved := os.Stdout
	os.Stdout = devnull
	defer func() { os.Stdout = saved }()
	fn()
}

// hygieneCommands mirrors scaling_test.go's corpus-heavy set. Mutators target
// ticket 0001 (always open in the fixture) so a single clean invocation runs.
var hygieneCommands = []struct {
	name   string
	invoke func(dir string) int
}{
	{"check", func(d string) int { return cmdCheck([]string{d}) }},
	{"list", func(d string) int { return cmdList([]string{d}) }},
	{"ready", func(d string) int { return cmdReady([]string{d}) }},
	{"close", func(d string) int { return cmdClose([]string{"0001", "hygiene", d}) }},
	{"rm", func(d string) int { return cmdRm([]string{"0001", d, "--force"}) }},
}

// TestScalingFDHygiene asserts every corpus-heavy command returns with no leaked file
// descriptors — no unclosed file, no dangling pipe to the git child.
func TestScalingFDHygiene(t *testing.T) {
	const n = 256 // non-trivial; fd hygiene is size-independent.
	for _, c := range hygieneCommands {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			buildCorpus(t, dir, n)

			var before, after int
			withDiscardedStdout(t, func() {
				before = openFDCount(t)
				c.invoke(dir)
				after = openFDCount(t)
			})
			if after > before {
				t.Errorf("%s leaked %d fd(s): %d -> %d", c.name, after-before, before, after)
			}
			t.Logf("%-6s fd %d -> %d", c.name, before, after)
		})
	}
}

// TestScalingHeapRetention asserts the read-only commands hold no live heap once they
// return: run each many times and require post-GC live heap to stay flat. A
// per-call retention leak (e.g. appending parsed tickets to a package global)
// accumulates linearly and trips the bound. Mutators are excluded — they consume
// ticket 0001 and cannot be looped on one fixture; their resource risk (git
// child + temp file) is the fd path, covered by TestScalingFDHygiene.
func TestScalingHeapRetention(t *testing.T) {
	const (
		n     = 512
		iters = 25
		// One parsed corpus is ~n KB; 24 leaked copies would be many MB. 1 MiB
		// slack absorbs runtime arena warmup without admitting a real leak.
		slackBytes = 1 << 20
	)
	readOnly := hygieneCommands[:3] // check, list, ready
	for _, c := range readOnly {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			buildCorpus(t, dir, n)

			var first, last uint64
			withDiscardedStdout(t, func() {
				for i := 0; i < iters; i++ {
					c.invoke(dir)
					if i == 0 {
						first = liveHeapBytes()
					}
				}
				last = liveHeapBytes()
			})
			if last > first+slackBytes {
				t.Errorf("%s retained %d bytes of live heap over %d calls: %d -> %d (slack %d)",
					c.name, last-first, iters, first, last, slackBytes)
			}
			t.Logf("%-6s liveHeap %dKB -> %dKB over %d calls", c.name, first/1024, last/1024, iters)
		})
	}
}
