package prob

const (
	HashName = "fnv1a64-avalanche-v1"

	fnvOffset64 = 14695981039346656037
	fnvPrime64  = 1099511628211
)

func Hash64(data []byte, seed uint64) uint64 {
	h := NewHasher64(seed)
	h.Write(data)
	return h.Sum64()
}

type Hasher64 struct {
	value uint64
}

func NewHasher64(seed uint64) Hasher64 {
	return Hasher64{value: fnvOffset64 ^ seed}
}

func (h *Hasher64) Write(data []byte) {
	for _, b := range data {
		h.value ^= uint64(b)
		h.value *= fnvPrime64
	}
}

func (h Hasher64) Sum64() uint64 {
	x := h.value
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	x *= 0xc4ceb9fe1a85ec53
	x ^= x >> 33
	return x
}
