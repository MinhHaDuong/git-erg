package main

import (
	"fmt"
	"math"
	"testing"
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
		// Simulate a bug: re-parse every file
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

func TestLinearOpCount(t *testing.T) {
	t.Run("loadErgs scales linearly with corpus size", func(t *testing.T) {
		nSmall := 50
		nLarge := 100

		dirSmall := t.TempDir()
		makeCorpus(t, dirSmall, nSmall)
		resetParseCount()
		loadErgs(dirSmall)
		countSmall := parseCount

		dirLarge := t.TempDir()
		makeCorpus(t, dirLarge, nLarge)
		resetParseCount()
		loadErgs(dirLarge)
		countLarge := parseCount

		if countSmall == 0 {
			t.Fatal("countSmall is 0 — instrumentation broken")
		}

		ratio := float64(countLarge) / float64(countSmall)
		// Linear: ratio should be ~2.0. Quadratic would give ~4.0.
		// Allow 1.5–2.5 for linear; anything above 3.0 is clearly super-linear.
		if ratio < 1.5 || ratio > 2.5 {
			t.Errorf("op-count ratio = %.2f (N=%d→%d, 2N=%d→%d); want ~2.0 for linear scaling",
				ratio, nSmall, countSmall, nLarge, countLarge)
		}
	})

	t.Run("negative control: quadratic loader trips the guard", func(t *testing.T) {
		nSmall := 50
		nLarge := 100

		dirSmall := t.TempDir()
		makeCorpus(t, dirSmall, nSmall)

		dirLarge := t.TempDir()
		makeCorpus(t, dirLarge, nLarge)

		// Simulate O(N²): parse each file N times
		quadraticLoad := func(dir string) int {
			resetParseCount()
			tickets, _ := loadErgs(dir)
			n := len(tickets)
			for i := 0; i < n; i++ {
				loadErgs(dir)
			}
			return parseCount
		}

		countSmall := quadraticLoad(dirSmall)
		countLarge := quadraticLoad(dirLarge)

		ratio := float64(countLarge) / float64(countSmall)
		// With O(N²), ratio should be ~4.0 (or higher), well above the 2.5 ceiling
		if ratio < 3.0 {
			t.Errorf("negative control: ratio = %.2f, expected ≥3.0 for a quadratic loader", ratio)
		}
		_ = math.Abs(ratio) // use math to keep import
	})
}
