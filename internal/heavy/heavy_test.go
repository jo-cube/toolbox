package heavy

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jo-cube/toolbox/internal/prob"
)

const repeatedLetters = `b
a
b
c
b
a
`

const streamWithOneDominantItem = `x
x
x
x
x
x
x
x
a
b
`

func TestExactModeRanksByObservedCounts(t *testing.T) {
	t.Parallel()

	path := writeInput(t, repeatedLetters)
	got, err := Run([]string{path}, Config{Top: 2, Exact: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Item != "b" || got[0].CountEstimate != 3 || got[0].CountLowerBound != 3 || got[1].Item != "a" {
		t.Fatalf("Run() = %#v", got)
	}
}

func TestRankedMatchesExactOrdering(t *testing.T) {
	t.Parallel()
	for _, tied := range []bool{false, true} {
		counts := make(map[string]uint64, 256)
		special := []string{"", "A", "a", "é", "日本", "\xff"}
		want := make([]Result, 0, 256)
		for i := range 256 {
			key := fmt.Sprintf("key-%03d", i)
			if i < len(special) {
				key = special[i]
			}
			count := uint64(7)
			if !tied {
				count = uint64(i%11 + 1)
				if i == 255 {
					count = ^uint64(0)
				}
			}
			counts[key] = count
			want = append(want, Result{Item: key, CountEstimate: count, CountLowerBound: count})
		}
		sort.Slice(want, func(i, j int) bool {
			if want[i].CountEstimate != want[j].CountEstimate {
				return want[i].CountEstimate > want[j].CountEstimate
			}
			return want[i].Item < want[j].Item
		})
		for i := range want {
			want[i].Rank = i + 1
		}
		for _, top := range []int{0, 1, 10, 20, 127, 128, 129, 255, 256, 257, int(^uint(0) >> 1)} {
			for range 3 {
				got := ranked(counts, top)
				if !reflect.DeepEqual(got, want[:min(top, len(want))]) {
					t.Fatalf("tied=%v top=%d: got %#v, want %#v", tied, top, got, want[:min(top, len(want))])
				}
			}
		}
	}
}

func TestExactModeTopBoundaries(t *testing.T) {
	t.Parallel()
	path := writeInput(t, repeatedLetters)
	for _, top := range []int{-1, 0} {
		if _, err := Run([]string{path}, Config{Top: top, Exact: true}); err == nil || !strings.Contains(err.Error(), "top must be a positive integer") {
			t.Fatalf("top=%d: error = %v", top, err)
		}
	}
	want := []Result{
		{Rank: 1, Item: "b", CountEstimate: 3, CountLowerBound: 3},
		{Rank: 2, Item: "a", CountEstimate: 2, CountLowerBound: 2},
		{Rank: 3, Item: "c", CountEstimate: 1, CountLowerBound: 1},
	}
	for _, top := range []int{1, 10, int(^uint(0) >> 1)} {
		got, err := Run([]string{path}, Config{Top: top, Capacity: top, Exact: true})
		if err != nil || !reflect.DeepEqual(got, want[:min(top, len(want))]) {
			t.Fatalf("top=%d: results = %#v, error = %v", top, got, err)
		}
	}
	empty := writeInput(t, "")
	got, err := Run([]string{empty}, Config{Top: 10, Exact: true})
	if err != nil || !reflect.DeepEqual(got, []Result{}) {
		t.Fatalf("empty input: results = %#v, error = %v", got, err)
	}
}

func TestApproximateKeepsHeavyItem(t *testing.T) {
	t.Parallel()

	path := writeInput(t, streamWithOneDominantItem)
	got, err := Run([]string{path}, Config{Top: 1, Capacity: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Item != "x" || got[0].CountLowerBound > got[0].CountEstimate {
		t.Fatalf("Run() = %#v, want x as top item", got)
	}
}

func TestApproximateHandlesHighCardinalityInput(t *testing.T) {
	t.Parallel()

	var input strings.Builder
	for i := range 10_000 {
		fmt.Fprintln(&input, i)
	}
	for range 1_000 {
		fmt.Fprintln(&input, "heavy")
	}
	path := writeInput(t, input.String())
	got, err := Run([]string{path}, Config{Top: 1, Capacity: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Item != "heavy" {
		t.Fatalf("Run() = %#v, want heavy as top item", got)
	}
	if got[0].CountLowerBound > 1_000 || got[0].CountEstimate < 1_000 {
		t.Fatalf("heavy bounds = %d..%d, want to contain 1000", got[0].CountLowerBound, got[0].CountEstimate)
	}
}

func writeInput(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFieldRankingWorksInBothModes(t *testing.T) {
	t.Parallel()
	path := writeInput(t, "1:: a ::x\x002::b::y\x003:: a ::z\x004:: ::empty\x005::b\x006::a")
	for _, exact := range []bool{false, true} {
		got, err := Run([]string{path}, Config{
			Top: 2, Exact: exact,
			Input: prob.InputOptions{NUL: true, Trim: true, IgnoreEmpty: true, Fields: prob.FieldOptions{Delimiter: "::", Field: 2}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].Item != "a" || got[0].CountEstimate != 3 || got[0].CountLowerBound != 3 || got[1].Item != "b" || got[1].CountEstimate != 2 {
			t.Fatalf("exact=%v: results = %#v", exact, got)
		}
	}
}

func BenchmarkRepeatedValues(b *testing.B) {
	path := filepath.Join(b.TempDir(), "input")
	input := strings.Repeat("dominant-value\n", 10000)
	if err := os.WriteFile(path, []byte(input), 0600); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Run([]string{path}, Config{Top: 1}); err != nil {
			b.Fatal(err)
		}
	}
}

func TestApproximateMatchesSpaceSavingReference(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(1))
	input := make([]string, 2000)
	for i := range input {
		input[i] = fmt.Sprintf("key-%02d", rng.Intn(30))
	}
	path := writeInput(t, strings.Join(input, "\n"))
	for _, capacity := range []int{1, 2, 7, 40} {
		tracked := map[string]Result{}
		for _, item := range input {
			entry, exists := tracked[item]
			if !exists && len(tracked) == capacity {
				var least Result
				first := true
				for _, candidate := range tracked {
					if first || candidate.CountEstimate < least.CountEstimate || candidate.CountEstimate == least.CountEstimate && candidate.Item < least.Item {
						least, first = candidate, false
					}
				}
				delete(tracked, least.Item)
				entry.CountEstimate = least.CountEstimate
			}
			entry.Item = item
			entry.CountEstimate++
			entry.CountLowerBound++
			tracked[item] = entry
		}
		var want []Result
		for _, entry := range tracked {
			want = append(want, entry)
		}
		want = rank(want, capacity)
		got, err := Run([]string{path}, Config{Top: capacity, Capacity: capacity})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("capacity %d: got %#v, want %#v", capacity, got, want)
		}
	}
}
