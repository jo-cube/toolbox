package sample

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jo-cube/toolbox/internal/prob"
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

func TestStableSampleCanHashOneFieldAndPreserveRecords(t *testing.T) {
	t.Parallel()

	records := []struct {
		record string
		key    string
	}{
		{"1\tgroup-a\tfirst\n", "group-a"},
		{"2\tgroup-a\tsecond\n", "group-a"},
		{"3\tgroup-b\tthird\n", "group-b"},
		{"4\tgroup-c\tfourth\n", "group-c"},
	}
	cfg := Config{Rate: 0.5, Stable: true, Seed: 7, Fields: prob.FieldOptions{Delimiter: "\t", Field: 2}}
	threshold := uint64(cfg.Rate * float64(math.MaxUint64))
	var input, want strings.Builder
	for _, record := range records {
		input.WriteString(record.record)
		if prob.Hash64([]byte(record.key), uint64(cfg.Seed)) < threshold {
			want.WriteString(record.record)
		}
	}
	if want.Len() == 0 || want.Len() == input.Len() {
		t.Fatal("test keys do not exercise both sampling decisions")
	}

	var out bytes.Buffer
	if err := RunFrom(nil, cfg, &out, strings.NewReader(input.String())); err != nil {
		t.Fatal(err)
	}
	if out.String() != want.String() {
		t.Fatalf("RunFrom() wrote %q, want %q", out.String(), want.String())
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

func TestReservoirPreservesInputOrder(t *testing.T) {
	t.Parallel()

	var input strings.Builder
	for i := range 100 {
		fmt.Fprintln(&input, i)
	}
	path := writeInput(t, input.String())
	var out bytes.Buffer
	if err := Run([]string{path}, Config{Count: 20, Seed: 1}, &out); err != nil {
		t.Fatal(err)
	}
	previous := -1
	for _, line := range strings.Fields(out.String()) {
		value, err := strconv.Atoi(line)
		if err != nil {
			t.Fatal(err)
		}
		if value <= previous {
			t.Fatalf("reservoir output is not in input order: %q", out.String())
		}
		previous = value
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

func TestExplicitZeroSeedIsRepeatable(t *testing.T) {
	t.Parallel()

	var input strings.Builder
	for i := range 100 {
		input.WriteString(strconv.Itoa(i))
		input.WriteByte('\n')
	}
	path := writeInput(t, input.String())
	cfg := Config{Rate: 0.5, RateSet: true, SeedSet: true}

	var a, b bytes.Buffer
	if err := Run([]string{path}, cfg, &a); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{path}, cfg, &b); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Fatal("explicit --seed 0 produced different samples")
	}
}

func TestValidateRejectsAmbiguousMode(t *testing.T) {
	t.Parallel()

	if err := Validate(Config{Rate: 0.1, Count: 10}); err == nil {
		t.Fatal("Validate() accepted rate and count together")
	}
	if err := Validate(Config{Rate: 0.1, RateSet: true, CountSet: true}); err == nil {
		t.Fatal("Validate() accepted explicitly set rate and zero count together")
	}
	if err := Validate(Config{Count: -1, CountSet: true}); err == nil || !strings.Contains(err.Error(), "positive") {
		t.Fatalf("Validate() error = %v, want positive count error", err)
	}
}

func TestValidateRestrictsFieldSelectionToStableRate(t *testing.T) {
	t.Parallel()

	fields := prob.FieldOptions{Delimiter: "\t", Field: 2}
	if err := Validate(Config{Rate: 0.5, Fields: fields}); err == nil || !strings.Contains(err.Error(), "--stable") {
		t.Fatalf("Validate() error = %v, want stable-mode error", err)
	}
	if err := Validate(Config{Rate: 0.5, Stable: true, Fields: prob.FieldOptions{Field: 2}}); err == nil || !strings.Contains(err.Error(), "--delimiter") {
		t.Fatalf("Validate() error = %v, want delimiter error", err)
	}
}

func TestValidateRejectsNaNRate(t *testing.T) {
	t.Parallel()

	if err := Validate(Config{Rate: math.NaN(), RateSet: true}); err == nil {
		t.Fatal("Validate() accepted NaN rate")
	}
}

func TestReservoirDoesNotPreallocateUnseenRecords(t *testing.T) {
	t.Parallel()

	path := writeInput(t, twoRecords)
	var out bytes.Buffer
	if err := Run([]string{path}, Config{Count: int(^uint(0) >> 1), Seed: 1}, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != twoRecords {
		t.Fatalf("Run() wrote %q, want %q", out.String(), twoRecords)
	}
}

func TestStableSampleTreatsLFAndCRLFAsTheSameRecords(t *testing.T) {
	t.Parallel()

	var input strings.Builder
	for i := range 100 {
		fmt.Fprintln(&input, i)
	}
	lf := writeInput(t, input.String())
	crlf := writeInput(t, strings.ReplaceAll(input.String(), "\n", "\r\n"))
	cfg := Config{Rate: 0.5, Stable: true, Seed: 7}

	var lfOut, crlfOut bytes.Buffer
	if err := Run([]string{lf}, cfg, &lfOut); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{crlf}, cfg, &crlfOut); err != nil {
		t.Fatal(err)
	}
	if got := strings.ReplaceAll(crlfOut.String(), "\r\n", "\n"); got != lfOut.String() {
		t.Fatal("stable selection changed between LF and CRLF input")
	}
}

func TestNULSamplingPreservesDelimiter(t *testing.T) {
	t.Parallel()

	input := []byte("a\x00b\x00")
	var out bytes.Buffer
	if err := RunFrom([]string{"-"}, Config{Rate: 1, NUL: true}, &out, bytes.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), input) {
		t.Fatalf("RunFrom() wrote %q, want %q", out.Bytes(), input)
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
