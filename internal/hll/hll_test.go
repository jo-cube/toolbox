package hll

import (
	"bytes"
	"encoding/binary"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestAddHashUpdatesExpectedRegister(t *testing.T) {
	t.Parallel()

	s, err := New(4)
	if err != nil {
		t.Fatal(err)
	}
	s.AddHash(3<<60 | 1<<59)
	if got := s.Registers[3]; got != 1 {
		t.Fatalf("rank = %d, want 1", got)
	}
	s.AddHash(3<<60 | 1<<57)
	if got := s.Registers[3]; got != 3 {
		t.Fatalf("rank = %d, want 3", got)
	}
	s.AddHash(3<<60 | 1<<59)
	if got := s.Registers[3]; got != 3 {
		t.Fatalf("lower rank reduced register to %d", got)
	}
	s.AddHash(7 << 60)
	if got := s.Registers[7]; got != 61 {
		t.Fatalf("all-zero suffix rank = %d, want 61", got)
	}
}

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
	if got.Precision != s.Precision || !slices.Equal(got.Registers, s.Registers) {
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

func TestMergeAlgebra(t *testing.T) {
	t.Parallel()

	newSketch := func(offset uint8) *Sketch {
		s, err := New(4)
		if err != nil {
			t.Fatal(err)
		}
		for i := range s.Registers {
			s.Registers[i] = uint8((i + int(offset)) % 6)
		}
		return s
	}
	clone := func(s *Sketch) *Sketch {
		return &Sketch{Precision: s.Precision, Registers: slices.Clone(s.Registers)}
	}

	a, b, c := newSketch(0), newSketch(2), newSketch(4)
	idempotent := clone(a)
	if err := idempotent.Merge(a); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(idempotent.Registers, a.Registers) {
		t.Fatal("merge is not idempotent")
	}

	ab, ba := clone(a), clone(b)
	if err := ab.Merge(b); err != nil {
		t.Fatal(err)
	}
	if err := ba.Merge(a); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ab.Registers, ba.Registers) {
		t.Fatal("merge is not commutative")
	}

	left, bc := clone(ab), clone(b)
	if err := left.Merge(c); err != nil {
		t.Fatal(err)
	}
	if err := bc.Merge(c); err != nil {
		t.Fatal(err)
	}
	right := clone(a)
	if err := right.Merge(bc); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(left.Registers, right.Registers) {
		t.Fatal("merge is not associative")
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

func TestEstimatorErrorTracksTheoreticalBound(t *testing.T) {
	t.Parallel()

	const (
		precision = 10
		trials    = 64
	)
	for _, count := range []int{100, 1_000, 2_500, 10_000, 100_000} {
		var bias, squaredError float64
		for trial := range trials {
			s, err := New(precision)
			if err != nil {
				t.Fatal(err)
			}
			base := uint64(trial) << 32
			for i := range count {
				s.AddHash(mix64(base | uint64(i)))
			}
			relativeError := (float64(s.Estimate()) - float64(count)) / float64(count)
			bias += relativeError
			squaredError += relativeError * relativeError
		}

		bias /= trials
		rmse := math.Sqrt(squaredError / trials)
		theoretical := 1.04 / math.Sqrt(1<<precision)
		t.Logf("n=%d bias=%.4f RMSE=%.4f theoretical RSE=%.4f", count, bias, rmse, theoretical)
		if math.Abs(bias) > 0.25*theoretical {
			t.Errorf("n=%d mean relative error %.4f exceeds 0.25x theoretical RSE %.4f", count, bias, theoretical)
		}
		if rmse > theoretical {
			t.Errorf("n=%d RMSE %.4f exceeds theoretical RSE %.4f", count, rmse, theoretical)
		}
	}
}

func TestEstimateNeverDecreases(t *testing.T) {
	t.Parallel()

	s, err := New(10)
	if err != nil {
		t.Fatal(err)
	}
	var previous uint64
	for i := range 100_000 {
		s.AddHash(mix64(uint64(i)))
		if i%100 == 0 {
			estimate := s.Estimate()
			if estimate < previous {
				t.Fatalf("estimate decreased from %d to %d after %d additions", previous, estimate, i+1)
			}
			previous = estimate
		}
	}
}

func TestEstimateRangeEdges(t *testing.T) {
	t.Parallel()

	s, err := New(4)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Estimate(); got != 0 {
		t.Fatalf("empty Estimate() = %d, want 0", got)
	}
	for i := range s.Registers {
		s.Registers[i] = 61
	}
	if got := s.Estimate(); got != math.MaxUint64 {
		t.Fatalf("Estimate() = %d, want %d", got, uint64(math.MaxUint64))
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
	headerSize := 4 + 1 + 1 + 1 + int(data[6]) + binary.Size(uint32(0))
	data[headerSize] = 62
	if _, err := Read(bytes.NewReader(data)); err == nil || !strings.Contains(err.Error(), "invalid register") {
		t.Fatalf("Read() error = %v, want invalid register", err)
	}
}

func TestReadRejectsInvalidState(t *testing.T) {
	t.Parallel()

	s, err := New(8)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, s); err != nil {
		t.Fatal(err)
	}
	valid := buf.Bytes()
	countOffset := 7 + int(valid[6])
	tests := []struct {
		name   string
		mutate func([]byte) []byte
		want   string
	}{
		{name: "version", mutate: func(data []byte) []byte { data[4]++; return data }, want: "unsupported HLL version"},
		{name: "hash", mutate: func(data []byte) []byte { data[7] ^= 0xff; return data }, want: "unsupported hash"},
		{name: "register count", mutate: func(data []byte) []byte {
			binary.BigEndian.PutUint32(data[countOffset:], 1)
			return data
		}, want: "invalid register count"},
		{name: "truncated registers", mutate: func(data []byte) []byte { return data[:len(data)-1] }, want: "read registers"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := tt.mutate(bytes.Clone(valid))
			if _, err := Read(bytes.NewReader(data)); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Read() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func mix64(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ x>>30) * 0xbf58476d1ce4e5b9
	x = (x ^ x>>27) * 0x94d049bb133111eb
	return x ^ x>>31
}
