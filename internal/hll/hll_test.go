package hll

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

func TestEstimateAndMerge(t *testing.T) {
	t.Parallel()

	a, err := New(10)
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(10)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 1000 {
		a.Add([]byte("a-" + strconv.Itoa(i)))
	}
	for i := range 1000 {
		b.Add([]byte("b-" + strconv.Itoa(i)))
	}
	if err := a.Merge(b); err != nil {
		t.Fatal(err)
	}
	got := a.Estimate()
	if got < 1500 || got > 2600 {
		t.Fatalf("Estimate() = %d, want near 2000", got)
	}
}

func TestReadWriteRoundTrip(t *testing.T) {
	t.Parallel()

	s, err := New(8)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []string{"a", "b", "c", "a"} {
		s.Add([]byte(item))
	}

	var buf bytes.Buffer
	if err := Write(&buf, s); err != nil {
		t.Fatal(err)
	}
	got, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Precision != s.Precision || got.Estimate() != s.Estimate() {
		t.Fatalf("round trip changed sketch")
	}
}

func TestMergeRejectsDifferentPrecision(t *testing.T) {
	t.Parallel()

	a, err := New(8)
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(9)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Merge(b); err == nil || !strings.Contains(err.Error(), "incompatible precision") {
		t.Fatalf("Merge() error = %v, want incompatible precision", err)
	}
}

func TestReadRejectsBadMagic(t *testing.T) {
	t.Parallel()

	_, err := Read(strings.NewReader("NOPE"))
	if err == nil || !strings.Contains(err.Error(), "invalid HLL file magic") {
		t.Fatalf("Read() error = %v, want invalid magic", err)
	}
}
