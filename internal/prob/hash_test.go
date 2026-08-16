package prob

import "testing"

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
