package bf

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/jo-cube/toolbox/internal/prob"
)

const (
	Magic         = "BLM1"
	Version uint8 = 1
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
	Type              string  `json:"type"`
	Version           uint8   `json:"version"`
	ExpectedItems     uint64  `json:"expected_items"`
	InsertedItems     uint64  `json:"inserted_items"`
	FalsePositiveRate float64 `json:"false_positive_rate"`
	BitCount          uint64  `json:"bit_count"`
	HashCount         uint32  `json:"hash_count"`
	Hash              string  `json:"hash"`
}

func New(expected uint64, rate float64) (*Filter, error) {
	if expected == 0 {
		return nil, fmt.Errorf("expected-items must be greater than zero")
	}
	if rate <= 0 || rate >= 1 {
		return nil, fmt.Errorf("false-positive-rate must be greater than 0 and less than 1")
	}

	m := uint64(math.Ceil(-float64(expected) * math.Log(rate) / (math.Ln2 * math.Ln2)))
	k := uint32(math.Round(float64(m) / float64(expected) * math.Ln2))
	if k == 0 {
		k = 1
	}

	return &Filter{
		ExpectedItems:     expected,
		FalsePositiveRate: rate,
		BitCount:          m,
		HashCount:         k,
		Bits:              make([]byte, (m+7)/8),
	}, nil
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
	for i := range f.Bits {
		f.Bits[i] |= other.Bits[i]
	}
	f.InsertedItems += other.InsertedItems
	return nil
}

func (f *Filter) Metadata() Metadata {
	return Metadata{
		Type:              "bloom-filter",
		Version:           Version,
		ExpectedItems:     f.ExpectedItems,
		InsertedItems:     f.InsertedItems,
		FalsePositiveRate: f.FalsePositiveRate,
		BitCount:          f.BitCount,
		HashCount:         f.HashCount,
		Hash:              prob.HashName,
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
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}
	if string(magic[:]) != Magic {
		return nil, fmt.Errorf("invalid Bloom filter magic %q", string(magic[:]))
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
			return nil, err
		}
	}
	if version != Version {
		return nil, fmt.Errorf("unsupported Bloom filter version %d", version)
	}
	hashName, err := readString(r)
	if err != nil {
		return nil, err
	}
	if hashName != prob.HashName {
		return nil, fmt.Errorf("unsupported hash %q", hashName)
	}
	if f.BitCount == 0 {
		return nil, fmt.Errorf("invalid bit count 0")
	}
	if f.HashCount == 0 {
		return nil, fmt.Errorf("invalid hash count 0")
	}
	var byteCount uint64
	if err := binary.Read(r, binary.BigEndian, &byteCount); err != nil {
		return nil, err
	}
	if byteCount != (f.BitCount+7)/8 {
		return nil, fmt.Errorf("invalid bitset size %d for %d bits", byteCount, f.BitCount)
	}
	if byteCount > uint64(maxInt()) {
		return nil, fmt.Errorf("bitset too large: %d bytes", byteCount)
	}
	f.Bits = make([]byte, byteCount)
	if _, err := io.ReadFull(r, f.Bits); err != nil {
		return nil, fmt.Errorf("read bitset: %w", err)
	}
	return f, nil
}

func maxInt() int {
	return int(^uint(0) >> 1)
}

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
