package heavy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExactTop(t *testing.T) {
	t.Parallel()

	path := writeInput(t, "b\na\nb\nc\nb\na\n")
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

	path := writeInput(t, "x\nx\nx\nx\nx\nx\nx\nx\na\nb\n")
	got, err := Run([]string{path}, Config{Top: 1, Capacity: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Item != "x" {
		t.Fatalf("Run() = %#v, want x as top item", got)
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
