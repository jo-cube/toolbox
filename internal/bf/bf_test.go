package bf

import (
	"bytes"
	"encoding/binary"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/jo-cube/toolbox/internal/prob"
)

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

func TestReadRejectsZeroBitCount(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	buf.WriteString(Magic)
	for _, field := range []any{
		Version,
		uint64(1),
		uint64(0),
		0.01,
		uint64(0),
		uint32(1),
		uint8(len(prob.HashName)),
	} {
		if err := binary.Write(&buf, binary.BigEndian, field); err != nil {
			t.Fatal(err)
		}
	}
	buf.WriteString(prob.HashName)
	if err := binary.Write(&buf, binary.BigEndian, uint64(0)); err != nil {
		t.Fatal(err)
	}

	_, err := Read(&buf)
	if err == nil || !strings.Contains(err.Error(), "invalid bit count") {
		t.Fatalf("Read() error = %v, want invalid bit count", err)
	}
}

func TestSizingAndFalsePositiveRate(t *testing.T) {
	t.Parallel()

	f, err := New(10_000, 0.01)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 10_000 {
		f.Add([]byte("present-" + strconv.Itoa(i)))
	}
	for i := range 10_000 {
		if !f.Test([]byte("present-" + strconv.Itoa(i))) {
			t.Fatalf("false negative for item %d", i)
		}
	}

	falsePositives := 0
	for i := range 10_000 {
		if f.Test([]byte("absent-" + strconv.Itoa(i))) {
			falsePositives++
		}
	}
	if falsePositives > 200 {
		t.Fatalf("false positives = %d, want at most 200", falsePositives)
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
		if f.BitCount == 0 || f.HashCount == 0 || len(f.Bits) == 0 {
			t.Fatalf("New(%d, %g) returned invalid sizing: %#v", tt.expected, tt.rate, f)
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
	if _, err := New(1, 1e-100); err == nil || !strings.Contains(err.Error(), "maximum supported") {
		t.Fatalf("New() error = %v, want hash-count limit", err)
	}
}

func TestSizingLimit(t *testing.T) {
	t.Parallel()

	_, _, bytes, err := sizing(1_000_000_000, 0.001, DefaultMaxBytes)
	if err != nil || bytes <= 512<<20 {
		t.Fatalf("billion-item sizing = %d bytes, %v", bytes, err)
	}
	if _, _, _, err := sizing(2_000_000_000, 0.001, DefaultMaxBytes); err == nil {
		t.Fatal("sizing accepted more than the default limit")
	}
	if _, _, bytes, err = sizing(2_000_000_000, 0.001, 0); err != nil || bytes <= DefaultMaxBytes {
		t.Fatalf("unlimited sizing = %d bytes, %v", bytes, err)
	}
}

func TestReadRejectsUnsafeHashCount(t *testing.T) {
	t.Parallel()

	f, err := New(100, 0.01)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, f); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	binary.BigEndian.PutUint32(data[37:41], maxHashCount+1)
	if _, err := Read(bytes.NewReader(data)); err == nil {
		t.Fatal("Read() accepted unsafe hash count")
	}
}

func TestReadAcceptsBoundedAlternateSizing(t *testing.T) {
	t.Parallel()

	f := &Filter{ExpectedItems: 100, FalsePositiveRate: 0.01, BitCount: 8, HashCount: 1, Bits: []byte{1}}
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

	f := &Filter{ExpectedItems: 100, FalsePositiveRate: 0.01, BitCount: 16, HashCount: 1, Bits: []byte{1, 2}}
	var buf bytes.Buffer
	if err := Write(&buf, f); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadWithLimit(bytes.NewReader(buf.Bytes()), 1); err == nil || !strings.Contains(err.Error(), "allocation limit") {
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

func TestHealthMetadataIgnoresUnusedPaddingBits(t *testing.T) {
	t.Parallel()

	f := &Filter{ExpectedItems: 1, FalsePositiveRate: 0.1, BitCount: 9, HashCount: 1, Bits: []byte{0xff, 0xff}}
	if got := f.Metadata().SetBits; got != 9 {
		t.Fatalf("Metadata().SetBits = %d, want 9", got)
	}
	var serialized bytes.Buffer
	if err := Write(&serialized, f); err != nil {
		t.Fatal(err)
	}
	metadata, err := Inspect(&serialized)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.SetBits != 9 || metadata.FillRatio != 1 {
		t.Fatalf("Inspect() metadata = %#v", metadata)
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
