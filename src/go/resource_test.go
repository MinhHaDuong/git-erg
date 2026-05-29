//go:build scaling

// Resource-hygiene guard (ticket 0169), sibling to scaling_test.go.
//
// scaling_test.go gates allocation *volume* (TotalAlloc) — the signal for an
// O(N^2) churn regression. It is blind to OS-resource leaks. This file adds two
// hygiene assertions that run under the same `scaling` build tag and the same
// `make test-scaling` target, so the scaling guard and the hygiene guard are
// exercised together and neither can be forgotten:
//
//   - TestScalingFDHygiene: every corpus-heavy command returns with no more open
//     file descriptors than it started with (erg shells out to git and opens
//     files; a missing Close or unreaped child pipe leaks an fd).
//   - TestScalingHeapRetention: the read-only commands hold no live heap once
//     they return. HeapAlloc after a forced GC is the bytes still reachable —
//     the retention signal TotalAlloc cannot see.
//
// Each guard ships a falsifiable negative control (the 0146 / AGENTS.md
// convention, mirroring scaling_test.go's TestScalingLinearNegativeControl): a
// probe that deliberately leaks the resource and asserts the guard's predicate
// trips. If a control ever stops tripping, the matching guard has gone blind and
// its green is worthless.
//
// CPU / nice: erg deliberately does NOT renice itself — process priority is the
// invoker's job (nice/ionice/systemd), and erg has no goroutines so it never
// saturates cores. There is therefore nothing to assert about scheduling here;
// the rationale lives in tickets/0169 and PEP §10.

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

// withDiscardedStdout runs fn with os.Stdout and os.Stderr redirected to a
// write-only /dev/null so command output is genuinely discarded. Opening the
// sink read-only (os.Open) would give os.Stdout a read-only descriptor, so every
// fmt.Print* in the measured command fails with EBADF — exercising a broken
// output path instead of the success path. O_WRONLY mirrors the kernel's discard
// behaviour and matches scaling_test.go's measure(); redirecting stderr too keeps
// list/ready warnings out of the test log. Both streams are restored on return
// (defers run even when fn calls t.Fatalf, via runtime.Goexit).
func withDiscardedStdout(t *testing.T, fn func()) {
	t.Helper()
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer devnull.Close()
	savedOut, savedErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = devnull, devnull
	defer func() { os.Stdout, os.Stderr = savedOut, savedErr }()
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

// TestScalingFDHygiene asserts every corpus-heavy command returns with no leaked
// file descriptors — no unclosed file, no dangling pipe to the git child. A
// non-zero exit aborts the subtest: an early failure path could look fd-neutral
// and pass vacuously, so we only trust the count on the success path.
func TestScalingFDHygiene(t *testing.T) {
	const n = 256 // non-trivial; fd hygiene is size-independent.
	for _, c := range hygieneCommands {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			buildCorpus(t, dir, n)

			var before, after int
			withDiscardedStdout(t, func() {
				before = openFDCount(t)
				if ret := c.invoke(dir); ret != 0 {
					t.Fatalf("%s returned non-zero exit %d — fd-hygiene signal is untrustworthy", c.name, ret)
				}
				after = openFDCount(t)
			})
			if after > before {
				t.Errorf("%s leaked %d fd(s): %d -> %d", c.name, after-before, before, after)
			}
			t.Logf("%-6s fd %d -> %d", c.name, before, after)
		})
	}
}

// TestScalingFDHygiene_NegativeControl proves the fd guard has teeth: it holds a
// descriptor open across the measurement and asserts openFDCount sees the rise.
// The fd is closed before returning so the leak does not escape into other tests.
// If this ever stops tripping, TestScalingFDHygiene's green means nothing.
func TestScalingFDHygiene_NegativeControl(t *testing.T) {
	before := openFDCount(t)
	leak, err := os.Open(os.DevNull) // deliberately held open across the measurement
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	after := openFDCount(t)
	leak.Close() // clean up so the probe does not pollute other tests
	if after <= before {
		t.Errorf("fd-leak probe did not raise the count (%d -> %d) — the fd guard is vacuous", before, after)
	}
}

// heapRetentionSlackBytes is the live-heap growth budget shared by the retention
// guard and its negative control: the guard must stay under it, the control must
// blow past it.
const heapRetentionSlackBytes = 1 << 20 // 1 MiB

// TestScalingHeapRetention asserts the read-only commands hold no live heap once
// they return: run each many times and require post-GC live heap to stay flat. A
// per-call retention leak (e.g. appending parsed tickets to a package global)
// accumulates linearly and trips the bound. Mutators are excluded — they consume
// ticket 0001 and cannot be looped on one fixture; their resource risk (git
// child + temp file) is the fd path, covered by TestScalingFDHygiene. A non-zero
// return aborts the subtest: a failing command allocates less and could satisfy
// the flat-heap bound vacuously.
func TestScalingHeapRetention(t *testing.T) {
	const (
		n     = 512
		iters = 25
	)
	readOnly := hygieneCommands[:3] // check, list, ready
	for _, c := range readOnly {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			buildCorpus(t, dir, n)

			var first, last uint64
			withDiscardedStdout(t, func() {
				for i := 0; i < iters; i++ {
					if ret := c.invoke(dir); ret != 0 {
						t.Fatalf("%s returned non-zero exit %d — heap-retention signal is untrustworthy", c.name, ret)
					}
					if i == 0 {
						first = liveHeapBytes()
					}
				}
				last = liveHeapBytes()
			})
			if last > first+heapRetentionSlackBytes {
				t.Errorf("%s retained %d bytes of live heap over %d calls: %d -> %d (slack %d)",
					c.name, last-first, iters, first, last, heapRetentionSlackBytes)
			}
			t.Logf("%-6s liveHeap %dKB -> %dKB over %d calls", c.name, first/1024, last/1024, iters)
		})
	}
}

// TestScalingHeapRetention_NegativeControl proves the heap guard has teeth: it
// retains heap on purpose (well past the slack) and asserts liveHeapBytes climbs
// beyond the same bound the real guard uses. runtime.KeepAlive stops the sink
// from being collected before the final sample. If this ever stops tripping, the
// flat-heap assertion is vacuous.
func TestScalingHeapRetention_NegativeControl(t *testing.T) {
	const (
		iters     = 25
		chunkSize = 1 << 16 // 64 KiB/iter -> ~1.6 MiB retained, well over the 1 MiB slack.
	)
	first := liveHeapBytes()
	var sink [][]byte
	for i := 0; i < iters; i++ {
		sink = append(sink, make([]byte, chunkSize))
	}
	last := liveHeapBytes()
	runtime.KeepAlive(sink)
	if last <= first+heapRetentionSlackBytes {
		t.Errorf("heap-retain probe did not raise live heap past slack (%d -> %d, slack %d) — the heap guard is vacuous",
			first, last, heapRetentionSlackBytes)
	}
}
