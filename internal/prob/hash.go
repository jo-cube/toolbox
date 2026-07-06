package prob

const (
	HashName = "fnv1a64-avalanche-v1"

	fnvOffset64 = 14695981039346656037
	fnvPrime64  = 1099511628211
)

func Hash64(data []byte, seed uint64) uint64 {
	h := fnvOffset64 ^ seed
	for _, b := range data {
		h ^= uint64(b)
		h *= fnvPrime64
	}
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33
	return h
}
