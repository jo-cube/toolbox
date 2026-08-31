package bf

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/bits"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cespare/xxhash/v2"
)

var benchmarkFilterMatches uint64
var benchmarkHash uint64

func TestFilterRoundTrip(t *testing.T) {
	t.Parallel()

	f, err := New(100, 0.01)
	if err != nil {
		t.Fatal(err)
	}
	f.Add([]byte("alpha"))
	f.Add([]byte("beta"))

	if !f.Test([]byte("alpha")) || !f.Test([]byte("beta")) {
		t.Fatal("inserted items should test present")
	}

	var buf bytes.Buffer
	if err := Write(&buf, f); err != nil {
		t.Fatal(err)
	}
	got, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Test([]byte("alpha")) || got.InsertedItems != 2 {
		t.Fatal("round trip changed filter")
	}
}

func TestSplitBlockLayout(t *testing.T) {
	t.Parallel()

	f := &Filter{Bits: make([]byte, blockBytes)}
	f.Add([]byte("alpha"))
	want := []byte{
		0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 4, 8, 0, 0, 0,
		0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 16, 0, 0, 4, 0,
	}
	if !bytes.Equal(f.Bits, want) {
		t.Fatalf("bits = %x, want %x", f.Bits, want)
	}
	if !f.Test([]byte("alpha")) {
		t.Fatal("inserted item should test present")
	}
}

func TestXXHash64Vector(t *testing.T) {
	t.Parallel()

	if got, want := xxhash.Sum64(nil), uint64(0xef46db3751d8e999); got != want {
		t.Fatalf("XXHash64(nil) = %x, want %x", got, want)
	}
}

func TestAddTouchesOneBitPerLaneInOneBlock(t *testing.T) {
	t.Parallel()

	f := &Filter{Bits: make([]byte, 7*blockBytes)}
	f.Add([]byte("alpha"))
	touched := -1
	for block := range 7 {
		start := block * blockBytes
		if count := bitCount(f.Bits[start : start+blockBytes]); count != 0 {
			if touched != -1 {
				t.Fatalf("Add touched blocks %d and %d", touched, block)
			}
			touched = block
			if count != blockHashCount {
				t.Fatalf("block %d has %d set bits, want %d", block, count, blockHashCount)
			}
			for lane := range blockHashCount {
				laneStart := start + lane*4
				if count := bitCount(f.Bits[laneStart : laneStart+4]); count != 1 {
					t.Fatalf("lane %d has %d set bits, want 1", lane, count)
				}
			}
		}
	}
	if touched == -1 {
		t.Fatal("Add did not touch a block")
	}
}

func bitCount(data []byte) int {
	count := 0
	for _, b := range data {
		count += bits.OnesCount8(b)
	}
	return count
}

func TestUnionRejectsIncompatibleFilters(t *testing.T) {
	t.Parallel()

	a, err := New(100, 0.01)
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(200, 0.01)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Union(b); err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("Union() error = %v, want incompatible", err)
	}
}

func TestUnionAcceptsSameLayoutWithDifferentSizingMetadata(t *testing.T) {
	t.Parallel()

	a, err := New(100, 0.01)
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(120, 0.02)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Bits) != len(b.Bits) {
		t.Fatalf("test filters have different layouts: %d and %d bytes", len(a.Bits), len(b.Bits))
	}
	a.Add([]byte("alpha"))
	b.Add([]byte("beta"))
	if err := a.Union(b); err != nil {
		t.Fatal(err)
	}
	if !a.Test([]byte("alpha")) || !a.Test([]byte("beta")) {
		t.Fatal("union lost an inserted item")
	}
}

func TestUnionFromSerializedFilter(t *testing.T) {
	t.Parallel()

	a, err := New(100, 0.01)
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(100, 0.01)
	if err != nil {
		t.Fatal(err)
	}
	a.Add([]byte("alpha"))
	b.Add([]byte("beta"))
	var serialized bytes.Buffer
	if err := Write(&serialized, b); err != nil {
		t.Fatal(err)
	}
	if err := a.UnionFrom(&serialized, DefaultMaxBytes); err != nil {
		t.Fatal(err)
	}
	if !a.Test([]byte("alpha")) || !a.Test([]byte("beta")) || a.InsertedItems != 2 {
		t.Fatal("streaming union changed filter behavior")
	}
}

func TestReadRejectsBadMagic(t *testing.T) {
	t.Parallel()

	_, err := Read(strings.NewReader("NOPE"))
	if err == nil || !strings.Contains(err.Error(), "invalid Bloom filter magic") {
		t.Fatalf("Read() error = %v, want invalid magic", err)
	}
}

func TestReadRejectsVersion1(t *testing.T) {
	t.Parallel()

	_, err := Read(bytes.NewReader(append([]byte(Magic), 1)))
	if err == nil || !strings.Contains(err.Error(), "unsupported Bloom filter version 1") {
		t.Fatalf("Read() error = %v, want unsupported version 1", err)
	}
}

func TestReadRejectsInvalidBlockSize(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	buf.WriteString(Magic)
	for _, field := range []any{
		Version,
		uint64(1),
		uint64(0),
		0.01,
	} {
		if err := binary.Write(&buf, binary.BigEndian, field); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeString(&buf, HashName); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&buf, binary.BigEndian, uint64(1)); err != nil {
		t.Fatal(err)
	}

	_, err := Read(&buf)
	if err == nil || !strings.Contains(err.Error(), "invalid split-block bitset size") {
		t.Fatalf("Read() error = %v, want invalid split-block size", err)
	}
}

func TestFalsePositiveRateAndNoFalseNegatives(t *testing.T) {
	t.Parallel()

	const (
		expected = 200_000
		queries  = 1_000_000
	)
	for _, tt := range []struct {
		name string
		rate float64
	}{
		{name: "10_percent", rate: 0.1},
		{name: "1_percent", rate: 0.01},
		{name: "0.1_percent", rate: 0.001},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f, err := New(expected, tt.rate)
			if err != nil {
				t.Fatal(err)
			}
			var key [16]byte
			key[0] = 1
			for i := range uint64(expected) {
				binary.BigEndian.PutUint64(key[8:], i)
				f.Add(key[:])
			}
			for i := range uint64(expected) {
				binary.BigEndian.PutUint64(key[8:], i)
				if !f.Test(key[:]) {
					t.Fatalf("false negative for item %d", i)
				}
			}

			key[0] = 2
			falsePositives := 0
			for i := range uint64(queries) {
				binary.BigEndian.PutUint64(key[8:], i)
				if f.Test(key[:]) {
					falsePositives++
				}
			}
			mean := queries * tt.rate
			tolerance := 6*math.Sqrt(queries*tt.rate*(1-tt.rate)) + 1
			if deviation := math.Abs(float64(falsePositives) - mean); deviation > tolerance {
				t.Fatalf("false positives = %d, want %.0f ± %.0f", falsePositives, mean, tolerance)
			}
			t.Logf("false positives = %d/%d", falsePositives, queries)
		})
	}
}

func TestSplitBlockSizingModel(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		rate        float64
		bitsPerItem float64
	}{
		{rate: 0.1, bitsPerItem: 6.0},
		{rate: 0.01, bitsPerItem: 10.5},
		{rate: 0.001, bitsPerItem: 16.9},
		{rate: 0.0001, bitsPerItem: 26.4},
		{rate: 0.00001, bitsPerItem: 41.0},
	} {
		if got := splitBlockBitsPerItem(tt.rate); math.Abs(got-tt.bitsPerItem) > 0.1 {
			t.Errorf("splitBlockBitsPerItem(%g) = %g, want about %g", tt.rate, got, tt.bitsPerItem)
		}
	}
}

func TestSizingAcrossScales(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		expected uint64
		rate     float64
	}{
		{expected: 100, rate: 0.1},
		{expected: 10_000, rate: 0.01},
		{expected: 1_000_000, rate: 0.001},
	} {
		f, err := New(tt.expected, tt.rate)
		if err != nil {
			t.Fatal(err)
		}
		if len(f.Bits) == 0 || len(f.Bits)%blockBytes != 0 {
			t.Fatalf("New(%d, %g) returned invalid sizing: %#v", tt.expected, tt.rate, f)
		}
	}
}

func TestSizingMeetsRequestedRate(t *testing.T) {
	t.Parallel()

	const expected = uint64(1_000_000)
	previousBytes := 0
	for _, rate := range []float64{0.1, 0.01, 0.001, 0.0001, 0.00001} {
		f, err := New(expected, rate)
		if err != nil {
			t.Fatal(err)
		}
		if len(f.Bits) <= previousBytes {
			t.Fatalf("rate %g allocated %d bytes after %d", rate, len(f.Bits), previousBytes)
		}
		previousBytes = len(f.Bits)
		bitsPerItem := float64(len(f.Bits)*8) / float64(expected)
		if modeled := splitBlockFalsePositiveRate(bitsPerItem); modeled > rate {
			t.Fatalf("rate %g modeled as %g with %g bits/item", rate, modeled, bitsPerItem)
		}
	}
}

func TestNewRejectsUnsafeSizing(t *testing.T) {
	t.Parallel()

	for _, rate := range []float64{math.NaN(), math.Inf(1)} {
		if _, err := New(1, rate); err == nil {
			t.Fatalf("New(1, %v) succeeded", rate)
		}
	}
	if _, err := New(math.MaxUint64, 0.01); err == nil || !strings.Contains(err.Error(), "allocation limit") {
		t.Fatalf("New() error = %v, want allocation limit", err)
	}
	if _, err := New(1, 1e-100); err == nil {
		t.Fatal("New() accepted sizing beyond the allocation limit")
	}
}

func TestSizingLimit(t *testing.T) {
	t.Parallel()

	bytes, err := sizing(1_000_000_000, 0.001, DefaultMaxBytes)
	if err != nil || bytes <= 512<<20 {
		t.Fatalf("billion-item sizing = %d bytes, %v", bytes, err)
	}
	if _, err := sizing(2_000_000_000, 0.001, DefaultMaxBytes); err == nil {
		t.Fatal("sizing accepted more than the default limit")
	}
	if bytes, err = sizing(2_000_000_000, 0.001, 0); err != nil || bytes <= DefaultMaxBytes {
		t.Fatalf("unlimited sizing = %d bytes, %v", bytes, err)
	}
}

func TestReadAcceptsSingleBlock(t *testing.T) {
	t.Parallel()

	f := &Filter{ExpectedItems: 100, FalsePositiveRate: 0.01, Bits: make([]byte, blockBytes)}
	var buf bytes.Buffer
	if err := Write(&buf, f); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(&buf); err != nil {
		t.Fatalf("Read() rejected structurally valid sizing: %v", err)
	}
}

func TestReadWithLimit(t *testing.T) {
	t.Parallel()

	f := &Filter{ExpectedItems: 100, FalsePositiveRate: 0.01, Bits: make([]byte, blockBytes)}
	var buf bytes.Buffer
	if err := Write(&buf, f); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadWithLimit(bytes.NewReader(buf.Bytes()), blockBytes-1); err == nil || !strings.Contains(err.Error(), "allocation limit") {
		t.Fatalf("ReadWithLimit() error = %v, want allocation limit", err)
	}
	if _, err := ReadWithLimit(bytes.NewReader(buf.Bytes()), 0); err != nil {
		t.Fatalf("ReadWithLimit() with no limit: %v", err)
	}
}

func TestInspectStreamsHealthMetadata(t *testing.T) {
	t.Parallel()

	f, err := New(100, 0.01)
	if err != nil {
		t.Fatal(err)
	}
	f.Add([]byte("alpha"))
	var buf bytes.Buffer
	if err := Write(&buf, f); err != nil {
		t.Fatal(err)
	}
	m, err := Inspect(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if m.BitsetBytes != uint64(len(f.Bits)) || m.SetBits == 0 || m.FillRatio <= 0 || m.EstimatedFalsePositiveRate <= 0 {
		t.Fatalf("Inspect() metadata = %#v", m)
	}
}

func TestStreamingOperationsRejectTruncatedBitsets(t *testing.T) {
	t.Parallel()

	f, err := New(100, 0.01)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, f); err != nil {
		t.Fatal(err)
	}
	truncated := buf.Bytes()[:buf.Len()-1]
	if _, err := Inspect(bytes.NewReader(truncated)); err == nil {
		t.Fatal("Inspect accepted a truncated bitset")
	}
	if err := f.UnionFrom(bytes.NewReader(truncated), DefaultMaxBytes); err == nil {
		t.Fatal("UnionFrom accepted a truncated bitset")
	}
}

func BenchmarkHash(b *testing.B) {
	item := make([]byte, 16)
	b.SetBytes(int64(len(item)))
	b.ReportAllocs()
	for b.Loop() {
		benchmarkHash = xxhash.Sum64(item)
	}
}

func BenchmarkFilterAdd(b *testing.B) {
	f, err := New(max(uint64(b.N), 1), 0.001)
	if err != nil {
		b.Fatal(err)
	}
	var key [16]byte
	b.SetBytes(int64(len(key)))
	b.ReportAllocs()
	b.ResetTimer()
	var i uint64
	for b.Loop() {
		binary.BigEndian.PutUint64(key[8:], i)
		f.Add(key[:])
		i++
	}
	benchmarkFilterMatches = f.InsertedItems
}

func BenchmarkFilterTestHot(b *testing.B) {
	f, err := New(1, 0.001)
	if err != nil {
		b.Fatal(err)
	}
	var present, absent [16]byte
	present[0] = 1
	absent[0] = 2
	f.Add(present[:])
	if f.Test(absent[:]) {
		b.Fatal("benchmark miss unexpectedly tests present")
	}
	for _, tt := range []struct {
		name string
		key  []byte
	}{
		{name: "Hit", key: present[:]},
		{name: "Miss", key: absent[:]},
	} {
		b.Run(tt.name, func(b *testing.B) {
			var matches uint64
			b.SetBytes(int64(len(tt.key)))
			b.ReportAllocs()
			for b.Loop() {
				if f.Test(tt.key) {
					matches++
				}
			}
			benchmarkFilterMatches = matches
		})
	}
}

func BenchmarkFilterTestMostlyNegative(b *testing.B) {
	f, err := New(1_000_000, 0.001)
	if err != nil {
		b.Fatal(err)
	}
	keys := make([][16]byte, 1<<16)
	for i := range keys {
		binary.BigEndian.PutUint64(keys[i][8:], uint64(i))
		if i%20 == 0 {
			f.Add(keys[i][:])
		}
	}
	var filler [16]byte
	filler[0] = 0xff
	for i := uint64(0); f.InsertedItems < f.ExpectedItems; i++ {
		binary.BigEndian.PutUint64(filler[8:], i)
		f.Add(filler[:])
	}

	var matches uint64
	b.SetBytes(16)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		if f.Test(keys[i&(len(keys)-1)][:]) {
			matches++
		}
	}
	benchmarkFilterMatches = matches
	b.ReportMetric(float64(matches)/float64(b.N), "matches/op")
}

func BenchmarkFilterTestMostlyNegative100M(b *testing.B) {
	const expected = uint64(100_000_000)
	benchmarkFilterTest(b, benchmarkPopulatedFilter(b, expected), expected)
}

func BenchmarkFilterTestMostlyNegative100MParallel(b *testing.B) {
	const expected = uint64(100_000_000)
	f := benchmarkPopulatedFilter(b, expected)
	var workers, matches atomic.Uint64
	b.SetBytes(16)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := (workers.Add(1) - 1) * 1_000_003 % expected
		var key [16]byte
		var localMatches uint64
		for pb.Next() {
			binary.BigEndian.PutUint64(key[8:], i)
			if f.Test(key[:]) {
				localMatches++
			}
			i++
			if i == expected {
				i = 0
			}
		}
		matches.Add(localMatches)
	})
	benchmarkFilterMatches = matches.Load()
	b.ReportMetric(float64(len(f.Bits))/(1<<20), "filter-MiB")
	b.ReportMetric(float64(matches.Load())/float64(b.N), "matches/op")
}

func benchmarkPopulatedFilter(b *testing.B, expected uint64) *Filter {
	b.Helper()
	f, err := New(expected, 0.001)
	if err != nil {
		b.Fatal(err)
	}
	var key [16]byte
	for i := uint64(0); i < expected; i++ {
		if i%20 == 0 {
			key[0] = 0
		} else {
			key[0] = 0xff
		}
		binary.BigEndian.PutUint64(key[8:], i)
		f.Add(key[:])
	}
	return f
}

func benchmarkFilterTest(b *testing.B, f *Filter, expected uint64) {
	b.Helper()
	var matches uint64
	var i uint64
	var key [16]byte
	b.SetBytes(int64(len(key)))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		key[0] = 0
		binary.BigEndian.PutUint64(key[8:], i)
		if f.Test(key[:]) {
			matches++
		}
		i++
		if i == expected {
			i = 0
		}
	}
	benchmarkFilterMatches = matches
	b.ReportMetric(float64(len(f.Bits))/(1<<20), "filter-MiB")
	b.ReportMetric(float64(matches)/float64(b.N), "matches/op")
}
