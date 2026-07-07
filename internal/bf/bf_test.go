package bf

import (
	"bytes"
	"encoding/binary"
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
