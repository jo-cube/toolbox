package heavy

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
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
