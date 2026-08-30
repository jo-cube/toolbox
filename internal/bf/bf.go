package bf

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/bits"

	"github.com/jo-cube/toolbox/internal/prob"
)

const (
	Magic                  = "BLM1"
	Version         uint8  = 1
	DefaultMaxBytes uint64 = 2 << 30
	maxHashCount    uint32 = 64
)

type Filter struct {
	ExpectedItems     uint64
	InsertedItems     uint64
	FalsePositiveRate float64
	BitCount          uint64
	HashCount         uint32
	Bits              []byte
}

type Metadata struct {
	Type                       string  `json:"type"`
	Version                    uint8   `json:"version"`
	ExpectedItems              uint64  `json:"expected_items"`
	InsertedItems              uint64  `json:"inserted_items"`
	FalsePositiveRate          float64 `json:"false_positive_rate"`
	BitCount                   uint64  `json:"bit_count"`
	BitsetBytes                uint64  `json:"bitset_bytes"`
	HashCount                  uint32  `json:"hash_count"`
	Hash                       string  `json:"hash"`
	SetBits                    uint64  `json:"set_bits"`
	FillRatio                  float64 `json:"fill_ratio"`
	EstimatedFalsePositiveRate float64 `json:"estimated_false_positive_rate"`
}

func New(expected uint64, rate float64) (*Filter, error) {
	return NewWithLimit(expected, rate, DefaultMaxBytes)
}

// NewWithLimit disables the allocation limit when maxBytes is zero.
func NewWithLimit(expected uint64, rate float64, maxBytes uint64) (*Filter, error) {
	m, k, byteCount, err := sizing(expected, rate, maxBytes)
	if err != nil {
		return nil, err
	}

	return &Filter{
		ExpectedItems:     expected,
		FalsePositiveRate: rate,
		BitCount:          m,
		HashCount:         k,
		Bits:              make([]byte, byteCount),
	}, nil
}

func sizing(expected uint64, rate float64, maxBytes uint64) (uint64, uint32, uint64, error) {
	if expected == 0 {
		return 0, 0, 0, fmt.Errorf("expected-items must be greater than zero")
	}
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0 || rate >= 1 {
		return 0, 0, 0, fmt.Errorf("false-positive-rate must be greater than 0 and less than 1")
	}

	bitCountFloat := math.Ceil(-float64(expected) * math.Log(rate) / (math.Ln2 * math.Ln2))
	if maxBytes != 0 && bitCountFloat > float64(maxBytes)*8 {
		return 0, 0, 0, fmt.Errorf("expected-items and false-positive-rate require more than the %d MiB allocation limit", maxBytes>>20)
	}
	if math.IsInf(bitCountFloat, 0) || bitCountFloat >= float64(math.MaxUint64) {
		return 0, 0, 0, fmt.Errorf("expected-items and false-positive-rate exceed the filter format")
	}
	bitCount := uint64(bitCountFloat)
	hashCount := uint32(math.Round(float64(bitCount) / float64(expected) * math.Ln2))
	if hashCount == 0 {
		hashCount = 1
	}
	if hashCount > maxHashCount {
		return 0, 0, 0, fmt.Errorf("false-positive-rate requires %d hashes; maximum supported is %d", hashCount, maxHashCount)
	}
	byteCount := bitCount / 8
	if bitCount%8 != 0 {
		byteCount++
	}
	if byteCount > uint64(maxInt()) {
		return 0, 0, 0, fmt.Errorf("filter requires %d bytes, exceeding this platform's allocation limit", byteCount)
	}
	return bitCount, hashCount, byteCount, nil
}

func (f *Filter) Add(item []byte) {
	f.eachPosition(item, func(bit uint64) bool {
		f.Bits[bit/8] |= 1 << (bit % 8)
		return true
	})
	f.InsertedItems++
}

func (f *Filter) Test(item []byte) bool {
	present := true
	f.eachPosition(item, func(bit uint64) bool {
		if f.Bits[bit/8]&(1<<(bit%8)) == 0 {
			present = false
			return false
		}
		return true
	})
	return present
}

func (f *Filter) eachPosition(item []byte, fn func(uint64) bool) {
	h1 := prob.Hash64(item, 0)
	h2 := prob.Hash64(item, 0x9e3779b97f4a7c15)
	if h2 == 0 {
		h2 = 1
	}

	for i := uint32(0); i < f.HashCount; i++ {
		if !fn((h1 + uint64(i)*h2) % f.BitCount) {
			return
		}
	}
}

func (f *Filter) Union(other *Filter) error {
	if err := f.compatible(other); err != nil {
		return err
	}
	if len(f.Bits) != len(other.Bits) {
		return fmt.Errorf("incompatible Bloom filters")
	}
	if other.InsertedItems > math.MaxUint64-f.InsertedItems {
		return fmt.Errorf("inserted item count overflow")
	}
	for i := range f.Bits {
		f.Bits[i] |= other.Bits[i]
	}
	f.InsertedItems += other.InsertedItems
	return nil
}

func (f *Filter) Metadata() Metadata {
	return f.metadata(countSetBits(f.Bits, 0, f.BitCount))
}

func (f *Filter) metadata(setBits uint64) Metadata {
	bitsetBytes := f.BitCount / 8
	if f.BitCount%8 != 0 {
		bitsetBytes++
	}
	fillRatio := float64(setBits) / float64(f.BitCount)
	return Metadata{
		Type:                       "bloom-filter",
		Version:                    Version,
		ExpectedItems:              f.ExpectedItems,
		InsertedItems:              f.InsertedItems,
		FalsePositiveRate:          f.FalsePositiveRate,
		BitCount:                   f.BitCount,
		BitsetBytes:                bitsetBytes,
		HashCount:                  f.HashCount,
		Hash:                       prob.HashName,
		SetBits:                    setBits,
		FillRatio:                  fillRatio,
		EstimatedFalsePositiveRate: math.Pow(fillRatio, float64(f.HashCount)),
	}
}

func (f *Filter) compatible(other *Filter) error {
	if f.BitCount != other.BitCount || f.HashCount != other.HashCount || f.FalsePositiveRate != other.FalsePositiveRate {
		return fmt.Errorf("incompatible Bloom filters")
	}
	return nil
}

func Write(w io.Writer, f *Filter) error {
	if _, err := io.WriteString(w, Magic); err != nil {
		return err
	}
	fields := []any{
		Version,
		f.ExpectedItems,
		f.InsertedItems,
		f.FalsePositiveRate,
		f.BitCount,
		f.HashCount,
	}
	for _, field := range fields {
		if err := binary.Write(w, binary.BigEndian, field); err != nil {
			return err
		}
	}
	if err := writeString(w, prob.HashName); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint64(len(f.Bits))); err != nil {
		return err
	}
	_, err := w.Write(f.Bits)
	return err
}

func Read(r io.Reader) (*Filter, error) {
	return ReadWithLimit(r, DefaultMaxBytes)
}

// ReadWithLimit disables the allocation limit when maxBytes is zero.
func ReadWithLimit(r io.Reader, maxBytes uint64) (*Filter, error) {
	f, byteCount, err := readHeader(r, maxBytes)
	if err != nil {
		return nil, err
	}
	if byteCount > uint64(maxInt()) {
		return nil, fmt.Errorf("bitset size %d exceeds this platform's allocation limit", byteCount)
	}
	f.Bits = make([]byte, byteCount)
	if _, err := io.ReadFull(r, f.Bits); err != nil {
		return nil, fmt.Errorf("read bitset: %w", err)
	}
	return f, nil
}

func Inspect(r io.Reader) (Metadata, error) {
	return InspectWithLimit(r, DefaultMaxBytes)
}

// InspectWithLimit disables the size limit when maxBytes is zero.
func InspectWithLimit(r io.Reader, maxBytes uint64) (Metadata, error) {
	f, byteCount, err := readHeader(r, maxBytes)
	if err != nil {
		return Metadata{}, err
	}
	var setBits uint64
	var offset uint64
	err = eachBitsetChunk(r, byteCount, func(chunk []byte) {
		setBits += countSetBits(chunk, offset, f.BitCount)
		offset += uint64(len(chunk))
	})
	if err != nil {
		return Metadata{}, err
	}
	return f.metadata(setBits), nil
}

// UnionFrom merges a serialized compatible filter without allocating its bitset.
func (f *Filter) UnionFrom(r io.Reader, maxBytes uint64) error {
	other, byteCount, err := readHeader(r, maxBytes)
	if err != nil {
		return err
	}
	if err := f.compatible(other); err != nil || uint64(len(f.Bits)) != byteCount {
		return fmt.Errorf("incompatible Bloom filters")
	}
	if other.InsertedItems > math.MaxUint64-f.InsertedItems {
		return fmt.Errorf("inserted item count overflow")
	}
	offset := 0
	if err := eachBitsetChunk(r, byteCount, func(chunk []byte) {
		for i, b := range chunk {
			f.Bits[offset+i] |= b
		}
		offset += len(chunk)
	}); err != nil {
		return err
	}
	f.InsertedItems += other.InsertedItems
	return nil
}

func readHeader(r io.Reader, maxBytes uint64) (*Filter, uint64, error) {
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return nil, 0, fmt.Errorf("read magic: %w", err)
	}
	if string(magic[:]) != Magic {
		return nil, 0, fmt.Errorf("invalid Bloom filter magic %q", string(magic[:]))
	}

	f := &Filter{}
	var version uint8
	fields := []any{
		&version,
		&f.ExpectedItems,
		&f.InsertedItems,
		&f.FalsePositiveRate,
		&f.BitCount,
		&f.HashCount,
	}
	for _, field := range fields {
		if err := binary.Read(r, binary.BigEndian, field); err != nil {
			return nil, 0, err
		}
	}
	if version != Version {
		return nil, 0, fmt.Errorf("unsupported Bloom filter version %d", version)
	}
	hashName, err := readString(r)
	if err != nil {
		return nil, 0, err
	}
	if hashName != prob.HashName {
		return nil, 0, fmt.Errorf("unsupported hash %q", hashName)
	}
	var byteCount uint64
	if err := binary.Read(r, binary.BigEndian, &byteCount); err != nil {
		return nil, 0, err
	}
	if f.ExpectedItems == 0 {
		return nil, 0, fmt.Errorf("invalid expected item count 0")
	}
	if math.IsNaN(f.FalsePositiveRate) || math.IsInf(f.FalsePositiveRate, 0) || f.FalsePositiveRate <= 0 || f.FalsePositiveRate >= 1 {
		return nil, 0, fmt.Errorf("invalid false-positive rate %g", f.FalsePositiveRate)
	}
	if f.BitCount == 0 {
		return nil, 0, fmt.Errorf("invalid bit count %d", f.BitCount)
	}
	if f.HashCount == 0 || f.HashCount > maxHashCount {
		return nil, 0, fmt.Errorf("invalid hash count %d", f.HashCount)
	}
	wantBytes := f.BitCount / 8
	if f.BitCount%8 != 0 {
		wantBytes++
	}
	if byteCount != wantBytes {
		return nil, 0, fmt.Errorf("invalid bitset size %d for %d bits", byteCount, f.BitCount)
	}
	if maxBytes != 0 && byteCount > maxBytes {
		return nil, 0, fmt.Errorf("bitset size %d exceeds the %d MiB allocation limit", byteCount, maxBytes>>20)
	}
	return f, byteCount, nil
}

func eachBitsetChunk(r io.Reader, byteCount uint64, fn func([]byte)) error {
	buf := make([]byte, 64<<10)
	for remaining := byteCount; remaining != 0; {
		n := min(remaining, uint64(len(buf)))
		chunk := buf[:n]
		if _, err := io.ReadFull(r, chunk); err != nil {
			return fmt.Errorf("read bitset: %w", err)
		}
		fn(chunk)
		remaining -= n
	}
	return nil
}

func countSetBits(data []byte, byteOffset, bitCount uint64) uint64 {
	var count uint64
	lastByte := bitCount / 8
	remainingBits := bitCount % 8
	for i, b := range data {
		if remainingBits != 0 && byteOffset+uint64(i) == lastByte {
			b &= byte(1<<remainingBits) - 1
		}
		count += uint64(bits.OnesCount8(b))
	}
	return count
}

func maxInt() int { return int(^uint(0) >> 1) }

func writeString(w io.Writer, value string) error {
	if len(value) > 255 {
		return fmt.Errorf("string too long")
	}
	if err := binary.Write(w, binary.BigEndian, uint8(len(value))); err != nil {
		return err
	}
	_, err := io.WriteString(w, value)
	return err
}

func readString(r io.Reader) (string, error) {
	var n uint8
	if err := binary.Read(r, binary.BigEndian, &n); err != nil {
		return "", err
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}
