package hll

import (
	"bytes"
	"encoding/binary"
	"math"
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

func TestPrecisionRejectsOverflowBeforeUint8Cast(t *testing.T) {
	t.Parallel()

	if _, err := Precision(260); err == nil || !strings.Contains(err.Error(), "precision must be between") {
		t.Fatalf("Precision(260) error = %v, want out of range", err)
	}
}

func TestReadRejectsBadMagic(t *testing.T) {
	t.Parallel()

	_, err := Read(strings.NewReader("NOPE"))
	if err == nil || !strings.Contains(err.Error(), "invalid HLL file magic") {
		t.Fatalf("Read() error = %v, want invalid magic", err)
	}
}

func TestEstimateAcrossCardinalityAndPrecision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		precision uint8
		count     int
	}{
		{precision: 4, count: 10_000},
		{precision: DefaultP, count: 100},
		{precision: DefaultP, count: 10_000},
		{precision: DefaultP, count: 100_000},
		{precision: 20, count: 100_000},
	}
	for _, tt := range tests {
		current := tt
		t.Run("p="+strconv.Itoa(int(current.precision))+"/n="+strconv.Itoa(current.count), func(t *testing.T) {
			t.Parallel()
			s, err := New(current.precision)
			if err != nil {
				t.Fatal(err)
			}
			for i := range current.count {
				s.Add([]byte(strconv.Itoa(i)))
			}
			got := s.Estimate()
			tolerance := uint64(math.Ceil(float64(current.count) * s.RelativeError() * 5))
			if tolerance < 3 {
				tolerance = 3
			}
			delta := math.Abs(float64(got) - float64(current.count))
			if delta > float64(tolerance) {
				t.Fatalf("Estimate() = %d, want %d ± %d", got, current.count, tolerance)
			}
		})
	}
}

func TestReadRejectsImpossibleRegister(t *testing.T) {
	t.Parallel()

	s, err := New(4)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, s); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	headerSize := 4 + 1 + 1 + 1 + len("fnv1a64-avalanche-v1") + binary.Size(uint32(0))
	data[headerSize] = 62
	if _, err := Read(bytes.NewReader(data)); err == nil || !strings.Contains(err.Error(), "invalid register") {
		t.Fatalf("Read() error = %v, want invalid register", err)
	}
}
