package heavy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if len(got) != 2 || got[0].Item != "b" || got[0].CountEstimate != 3 || got[1].Item != "a" {
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
	if len(got) != 1 || got[0].Item != "x" {
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
}

func writeInput(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
