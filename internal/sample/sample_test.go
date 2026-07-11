package sample

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"
)

const fourRecords = `a
b
c
d
`

const twoRecords = `a
b
`

func TestStableSampleIsRepeatable(t *testing.T) {
	t.Parallel()

	path := writeInput(t, fourRecords)
	cfg := Config{Rate: 0.5, Stable: true, Seed: 7}

	var a bytes.Buffer
	if err := Run([]string{path}, cfg, &a); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := Run([]string{path}, cfg, &b); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Fatalf("stable sample changed: %q != %q", a.String(), b.String())
	}
}

func TestReservoirCount(t *testing.T) {
	t.Parallel()

	path := writeInput(t, fourRecords)
	var out bytes.Buffer
	if err := Run([]string{path}, Config{Count: 2, Seed: 1}, &out); err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(out.Bytes(), []byte("\n")); got != 2 {
		t.Fatalf("reservoir wrote %d records, want 2\n%s", got, out.String())
	}
}

func TestRateZeroIsValidAndEmitsNothing(t *testing.T) {
	t.Parallel()

	path := writeInput(t, twoRecords)
	var out bytes.Buffer
	if err := Run([]string{path}, Config{Rate: 0, RateSet: true}, &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("Run() wrote %q, want no output", out.String())
	}
}

func TestValidateRejectsAmbiguousMode(t *testing.T) {
	t.Parallel()

	if err := Validate(Config{Rate: 0.1, Count: 10}); err == nil {
		t.Fatal("Validate() accepted rate and count together")
	}
}

func TestValidateRejectsNaNRate(t *testing.T) {
	t.Parallel()

	if err := Validate(Config{Rate: math.NaN(), RateSet: true}); err == nil {
		t.Fatal("Validate() accepted NaN rate")
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
