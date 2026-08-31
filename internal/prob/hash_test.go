package prob

import (
	"bytes"
	"testing"
)

var benchmarkHash64 uint64

func TestHasher64MatchesHash64AcrossChunks(t *testing.T) {
	t.Parallel()

	data := []byte("arbitrary\x00binary\xffkey")
	h := NewHasher64(42)
	h.Write(data[:5])
	h.Write(data[5:12])
	h.Write(data[12:])
	if got, want := h.Sum64(), Hash64(data, 42); got != want {
		t.Fatalf("chunked hash = %d, want %d", got, want)
	}
	if got, want := h.Sum64(), uint64(6722762412642110665); got != want {
		t.Fatalf("stable hash = %d, want %d", got, want)
	}
}

func BenchmarkHash64(b *testing.B) {
	for _, tt := range []struct {
		name string
		size int
	}{
		{name: "16B", size: 16},
		{name: "32B", size: 32},
		{name: "128B", size: 128},
	} {
		b.Run(tt.name, func(b *testing.B) {
			data := bytes.Repeat([]byte{'x'}, tt.size)
			var hash uint64
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			for b.Loop() {
				hash = Hash64(data, 0)
			}
			benchmarkHash64 = hash
		})
	}
}
