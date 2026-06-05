package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// makeCorpus creates n valid .erg files in dir, numbered 9001..9000+n
// (fixture range reserved for contract tests).
func makeCorpus(t *testing.T, dir string, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		id := 9000 + i
		name := fmt.Sprintf("%04d-synth-%04d.erg", id, i)
		writeErg(t, dir, name, validErgContent())
	}
}

// makeCorpusWithRefs creates n tickets where each ticket (except the first)
// has a Blocked-by ref to the previous ticket, forming a chain. This
// exercises the ref-resolution and cycle-detection paths in validateCorpus.
func makeCorpusWithRefs(t *testing.T, dir string, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		id := 9000 + i
		name := fmt.Sprintf("%04d-synth-%04d.erg", id, i)
		content := "%erg 0.1\nTitle: Synthetic ticket\nCreated: 2024-01-01\nAuthor: test\n"
		if i > 1 {
			content += fmt.Sprintf("Blocked-by: %04d\n", 9000+i-1)
		}
		content += "\n--- log ---\n--- body ---\n"
		writeErg(t, dir, name, content)
	}
}

// makeCorpusQuadraticRefs creates n tickets where ticket i references ALL
// prior tickets (i-1 refs), producing O(N^2) total refs. This is the data
// shape that would expose a linear scan inside the ref loop.
func makeCorpusQuadraticRefs(t *testing.T, dir string, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		id := 9000 + i
		name := fmt.Sprintf("%04d-synth-%04d.erg", id, i)
		var refs []string
		for j := 1; j < i; j++ {
			refs = append(refs, fmt.Sprintf("Blocked-by: %04d", 9000+j))
		}
		refBlock := ""
		if len(refs) > 0 {
			refBlock = strings.Join(refs, "\n") + "\n"
		}
		content := "%erg 0.1\nTitle: Synthetic ticket\nCreated: 2024-01-01\nAuthor: test\n" +
			refBlock + "\n--- log ---\n--- body ---\n"
		writeErg(t, dir, name, content)
	}
}

func TestParseOnce(t *testing.T) {
	t.Run("loadErgs parses each file exactly once", func(t *testing.T) {
		dir := t.TempDir()
		n := 20
		makeCorpus(t, dir, n)

		resetParseCount()
		tickets, _ := loadErgs(dir)
		if len(tickets) != n {
			t.Fatalf("loadErgs returned %d tickets, want %d", len(tickets), n)
		}
		if parseCount != n {
			t.Errorf("parseCount = %d, want %d (one parse per file)", parseCount, n)
		}
	})

	t.Run("negative control: double-parse trips the guard", func(t *testing.T) {
		dir := t.TempDir()
		n := 5
		makeCorpus(t, dir, n)

		resetParseCount()
		loadErgs(dir)
		loadErgs(dir)
		if parseCount <= n {
			t.Errorf("negative control failed: parseCount = %d after two loadErgs calls on %d files; expected > %d",
				parseCount, n, n)
		}
	})

	t.Run("check command parses each file once", func(t *testing.T) {
		dir := t.TempDir()
		n := 15
		makeCorpus(t, dir, n)

		resetParseCount()
		cmdCheck([]string{dir})
		if parseCount != n {
			t.Errorf("cmdCheck: parseCount = %d, want %d (one parse per file)", parseCount, n)
		}
	})

	t.Run("list command parses each file once", func(t *testing.T) {
		dir := t.TempDir()
		n := 15
		makeCorpus(t, dir, n)

		resetParseCount()
		cmdList([]string{dir})
		if parseCount != n {
			t.Errorf("cmdList: parseCount = %d, want %d (one parse per file)", parseCount, n)
		}
	})
}

// TestLinearOpCount guards against structural regression in corpus operations:
// extra loads, growing ref counts, additional traversal passes. The counter
// tracks per-ref and per-edge work. The idExists timing subtest guards
// against a linear scan replacing the O(1) map lookup -- a per-call counter
// is blind to that mutation (fang-audit 2026-06-05, gaze round-1 REROLL).
func TestLinearOpCount(t *testing.T) {
	t.Run("corpus validation scales linearly with refs", func(t *testing.T) {
		nSmall := 50
		nLarge := 100

		dirSmall := t.TempDir()
		makeCorpusWithRefs(t, dirSmall, nSmall)
		resetCorpusOpCount()
		cmdCheck([]string{dirSmall})
		countSmall := corpusOpCount

		dirLarge := t.TempDir()
		makeCorpusWithRefs(t, dirLarge, nLarge)
		resetCorpusOpCount()
		cmdCheck([]string{dirLarge})
		countLarge := corpusOpCount

		if countSmall == 0 {
			t.Fatal("countSmall is 0 -- corpusOpCount instrumentation broken")
		}

		ratio := float64(countLarge) / float64(countSmall)
		if ratio < 1.5 || ratio > 2.5 {
			t.Errorf("corpus op-count ratio = %.2f (N=%d->%d ops, 2N=%d->%d ops); want ~2.0 for linear scaling",
				ratio, nSmall, countSmall, nLarge, countLarge)
		}

		// Absolute floor: for a chained corpus of n tickets, corpusOpCount must
		// include contributions from every instrumented site:
		//   - 2*n per-ticket passes (label-vocab check + ID extraction)
		//   - (n-1) per-ref rule-10 lookups in validateCorpus
		//   - (n-1) per-ref adjacency builds in detectCycles
		//   - (n-1) per-edge DFS visits in detectCycles  <- the dropped site
		// Total = 5*n - 3. Removing any single counter site drops below this.
		floorSmall := 5*nSmall - 3
		if countSmall < floorSmall {
			t.Errorf("countSmall = %d, want >= %d (absolute floor for n=%d); a counter site was likely dropped",
				countSmall, floorSmall, nSmall)
		}
	})

	t.Run("idExists lookup time is O(1) in map size", func(t *testing.T) {
		// A per-call counter (refLookupComparisons++) is blind to a linear
		// scan injected inside idExists -- it increments once per call
		// regardless of internal complexity. This timing subtest catches
		// the mutation directly: build maps of 100 and 10_000 entries,
		// measure per-call lookup time, and assert the ratio stays below
		// 5.0. A linear scan produces ratio ~100x; even noisy CI sees >10x.
		//
		// Warm-up and repeated iterations absorb allocator jitter. The 5.0
		// ceiling is ~250x looser than the expected ~1.0 ratio, so this
		// never flakes on O(1) lookups but always catches O(N) scans.
		const nSmall = 100
		const nLarge = 10_000
		const iters = 100_000

		smallMap := make(map[string]bool, nSmall)
		for i := 0; i < nSmall; i++ {
			smallMap[fmt.Sprintf("%04d", i)] = true
		}
		largeMap := make(map[string]bool, nLarge)
		for i := 0; i < nLarge; i++ {
			largeMap[fmt.Sprintf("%04d", i)] = true
		}

		// Probe key that exists in both maps.
		probe := fmt.Sprintf("%04d", nSmall/2)

		// Warm up.
		for i := 0; i < 1000; i++ {
			idExists(smallMap, probe)
			idExists(largeMap, probe)
		}

		start := time.Now()
		for i := 0; i < iters; i++ {
			idExists(smallMap, probe)
		}
		durSmall := time.Since(start)

		start = time.Now()
		for i := 0; i < iters; i++ {
			idExists(largeMap, probe)
		}
		durLarge := time.Since(start)

		if durSmall == 0 {
			t.Fatal("durSmall is 0 -- timing resolution too low")
		}
		ratio := float64(durLarge) / float64(durSmall)
		if ratio > 5.0 {
			t.Errorf("idExists timing ratio (map %d vs %d) = %.2f; want <= 5.0 for O(1) lookup "+
				"(small=%v, large=%v)", nSmall, nLarge, ratio, durSmall, durLarge)
		}
	})

	t.Run("negative control: quadratic ref fan-out trips the guard", func(t *testing.T) {
		// A corpus where ticket i references all prior tickets has O(N^2)
		// total refs. The per-ref counters in validateCorpus and
		// detectCycles produce a quadratic operation count, which the
		// ratio test catches (ratio ~ 4.0, well above the 2.5 ceiling).
		nSmall := 30
		nLarge := 60

		dirSmall := t.TempDir()
		makeCorpusQuadraticRefs(t, dirSmall, nSmall)
		resetCorpusOpCount()
		cmdCheck([]string{dirSmall})
		countSmall := corpusOpCount

		dirLarge := t.TempDir()
		makeCorpusQuadraticRefs(t, dirLarge, nLarge)
		resetCorpusOpCount()
		cmdCheck([]string{dirLarge})
		countLarge := corpusOpCount

		ratio := float64(countLarge) / float64(countSmall)
		if ratio < 3.0 {
			t.Errorf("negative control: ratio = %.2f (N=%d->%d ops, 2N=%d->%d ops); expected >=3.0 for quadratic ref fan-out",
				ratio, nSmall, countSmall, nLarge, countLarge)
		}
	})
}
