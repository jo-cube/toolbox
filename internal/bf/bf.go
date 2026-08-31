package bf

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/bits"

	"github.com/cespare/xxhash/v2"
)

const (
	Magic                  = "BLM1"
	Version         uint8  = 2
	HashName               = "xxhash64-v1"
	DefaultMaxBytes uint64 = 2 << 30
	blockBytes             = 32
	blockBits              = blockBytes * 8
	blockHashCount         = 8
	maxBlockCount   uint64 = 1<<32 - 1
	maxFormatBytes         = maxBlockCount * blockBytes
)

var blockSalts = [...]uint32{
	0x47b6137b, 0x44974d91, 0x8824ad5b, 0xa2b7289d,
	0x705495c7, 0x2df1424b, 0x9efc4947, 0x5c6bfb31,
}

type Filter struct {
	ExpectedItems     uint64
	InsertedItems     uint64
	FalsePositiveRate float64
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
	byteCount, err := sizing(expected, rate, maxBytes)
	if err != nil {
		return nil, err
	}

	return &Filter{
		ExpectedItems:     expected,
		FalsePositiveRate: rate,
		Bits:              make([]byte, byteCount),
	}, nil
}

func sizing(expected uint64, rate float64, maxBytes uint64) (uint64, error) {
	if expected == 0 {
		return 0, fmt.Errorf("expected-items must be greater than zero")
	}
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0 || rate >= 1 {
		return 0, fmt.Errorf("false-positive-rate must be greater than 0 and less than 1")
	}

	bitCountFloat := float64(expected) * splitBlockBitsPerItem(rate)
	byteCountFloat := math.Ceil(bitCountFloat/blockBits) * blockBytes
	if maxBytes != 0 && byteCountFloat > float64(maxBytes) {
		return 0, fmt.Errorf("expected-items and false-positive-rate require more than the %d MiB allocation limit", maxBytes>>20)
	}
	if math.IsInf(byteCountFloat, 0) || byteCountFloat > float64(maxFormatBytes) {
		return 0, fmt.Errorf("expected-items and false-positive-rate exceed the filter format")
	}
	byteCount := uint64(byteCountFloat)
	if byteCount > uint64(maxInt()) {
		return 0, fmt.Errorf("filter requires %d bytes, exceeding this platform's allocation limit", byteCount)
	}
	return byteCount, nil
}

func splitBlockBitsPerItem(rate float64) float64 {
	low, high := 0.0, 1.0
	for splitBlockFalsePositiveRate(high) > rate {
		high *= 2
	}
	for range 64 {
		mid := (low + high) / 2
		if splitBlockFalsePositiveRate(mid) > rate {
			low = mid
		} else {
			high = mid
		}
	}
	return high
}

func splitBlockFalsePositiveRate(bitsPerItem float64) float64 {
	lambda := blockBits / bitsPerItem
	if lambda > 700 {
		return 1
	}
	probability := math.Exp(-lambda)
	laneUnset := 1.0
	var rate float64
	limit := int(math.Ceil(lambda + 12*math.Sqrt(lambda) + 64))
	for inserted := 0; inserted <= limit; inserted++ {
		rate += probability * math.Pow(1-laneUnset, float64(blockHashCount))
		probability *= lambda / float64(inserted+1)
		laneUnset *= 31.0 / 32
	}
	return rate
}

func (f *Filter) Add(item []byte) {
	hash := xxhash.Sum64(item)
	blocks := uint64(len(f.Bits) / blockBytes)
	offset := int((uint64(uint32(hash>>32))*blocks)>>32) * blockBytes
	block := f.Bits[offset : offset+blockBytes]
	x := uint32(hash)
	for i, salt := range blockSalts {
		bit := (x * salt) >> 27
		index := i*4 + int(bit>>3)
		block[index] |= 1 << (bit & 7)
	}
	f.InsertedItems++
}

func (f *Filter) Test(item []byte) bool {
	hash := xxhash.Sum64(item)
	blocks := uint64(len(f.Bits) / blockBytes)
	offset := int((uint64(uint32(hash>>32))*blocks)>>32) * blockBytes
	block := f.Bits[offset : offset+blockBytes]
	x := uint32(hash)
	for i, salt := range blockSalts {
		bit := (x * salt) >> 27
		index := i*4 + int(bit>>3)
		if block[index]&(1<<(bit&7)) == 0 {
			return false
		}
	}
	return true
}

func (f *Filter) Union(other *Filter) error {
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
	return f.metadata(countSetBits(f.Bits), uint64(len(f.Bits)))
}

func (f *Filter) metadata(setBits, bitsetBytes uint64) Metadata {
	bitCount := bitsetBytes * 8
	fillRatio := float64(setBits) / float64(bitCount)
	var estimatedRate float64
	if f.InsertedItems != 0 {
		estimatedRate = splitBlockFalsePositiveRate(float64(bitCount) / float64(f.InsertedItems))
	}
	return Metadata{
		Type:                       "bloom-filter",
		Version:                    Version,
		ExpectedItems:              f.ExpectedItems,
		InsertedItems:              f.InsertedItems,
		FalsePositiveRate:          f.FalsePositiveRate,
		BitCount:                   bitCount,
		BitsetBytes:                bitsetBytes,
		Hash:                       HashName,
		SetBits:                    setBits,
		FillRatio:                  fillRatio,
		EstimatedFalsePositiveRate: estimatedRate,
	}
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
	}
	for _, field := range fields {
		if err := binary.Write(w, binary.BigEndian, field); err != nil {
			return err
		}
	}
	if err := writeString(w, HashName); err != nil {
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
	err = eachBitsetChunk(r, byteCount, func(chunk []byte) {
		setBits += countSetBits(chunk)
	})
	if err != nil {
		return Metadata{}, err
	}
	return f.metadata(setBits, byteCount), nil
}

// UnionFrom merges a serialized compatible filter without allocating its bitset.
func (f *Filter) UnionFrom(r io.Reader, maxBytes uint64) error {
	other, byteCount, err := readHeader(r, maxBytes)
	if err != nil {
		return err
	}
	if uint64(len(f.Bits)) != byteCount {
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

	var version uint8
	if err := binary.Read(r, binary.BigEndian, &version); err != nil {
		return nil, 0, err
	}
	if version != Version {
		return nil, 0, fmt.Errorf("unsupported Bloom filter version %d", version)
	}

	f := &Filter{}
	fields := []any{
		&f.ExpectedItems,
		&f.InsertedItems,
		&f.FalsePositiveRate,
	}
	for _, field := range fields {
		if err := binary.Read(r, binary.BigEndian, field); err != nil {
			return nil, 0, err
		}
	}
	hashName, err := readString(r)
	if err != nil {
		return nil, 0, err
	}
	if hashName != HashName {
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
	if byteCount == 0 || byteCount%blockBytes != 0 || byteCount > maxFormatBytes {
		return nil, 0, fmt.Errorf("invalid split-block bitset size %d", byteCount)
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

func countSetBits(data []byte) uint64 {
	var count uint64
	for _, b := range data {
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
