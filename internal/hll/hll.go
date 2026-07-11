package hll

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/bits"

	"github.com/jo-cube/toolbox/internal/prob"
)

const (
	Magic          = "HLL1"
	Version  uint8 = 1
	DefaultP uint8 = 14
)

type Sketch struct {
	Precision uint8
	Registers []uint8
}

type Metadata struct {
	Type          string  `json:"type"`
	Version       uint8   `json:"version"`
	Precision     uint8   `json:"precision"`
	Registers     int     `json:"registers"`
	Hash          string  `json:"hash"`
	ApproxUnique  uint64  `json:"approx_unique"`
	RelativeError float64 `json:"relative_error"`
}

func New(precision uint8) (*Sketch, error) {
	if precision < 4 || precision > 20 {
		return nil, fmt.Errorf("precision must be between 4 and 20")
	}
	return &Sketch{
		Precision: precision,
		Registers: make([]uint8, 1<<precision),
	}, nil
}

func Precision(value uint) (uint8, error) {
	if value < 4 || value > 20 {
		return 0, fmt.Errorf("precision must be between 4 and 20")
	}
	return uint8(value), nil
}

func (s *Sketch) Add(item []byte) {
	x := prob.Hash64(item, 0)
	idx := x >> (64 - s.Precision)
	remaining := x << s.Precision

	rank := uint8(64 - s.Precision + 1)
	if remaining != 0 {
		rank = uint8(bits.LeadingZeros64(remaining) + 1)
	}
	if rank > s.Registers[idx] {
		s.Registers[idx] = rank
	}
}

func (s *Sketch) Merge(other *Sketch) error {
	if s.Precision != other.Precision {
		return fmt.Errorf("incompatible precision: %d != %d", s.Precision, other.Precision)
	}
	if len(s.Registers) != len(other.Registers) {
		return fmt.Errorf("incompatible register count: %d != %d", len(s.Registers), len(other.Registers))
	}
	for i, r := range other.Registers {
		if r > s.Registers[i] {
			s.Registers[i] = r
		}
	}
	return nil
}

func (s *Sketch) Estimate() uint64 {
	m := float64(len(s.Registers))
	var sum float64
	zeros := 0
	for _, r := range s.Registers {
		sum += math.Ldexp(1, -int(r))
		if r == 0 {
			zeros++
		}
	}

	raw := alpha(len(s.Registers)) * m * m / sum
	if raw <= 2.5*m && zeros > 0 {
		raw = m * math.Log(m/float64(zeros))
	}
	return uint64(math.Round(raw))
}

func (s *Sketch) RelativeError() float64 {
	return 1.04 / math.Sqrt(float64(len(s.Registers)))
}

func (s *Sketch) Metadata() Metadata {
	return Metadata{
		Type:          "hyperloglog",
		Version:       Version,
		Precision:     s.Precision,
		Registers:     len(s.Registers),
		Hash:          prob.HashName,
		ApproxUnique:  s.Estimate(),
		RelativeError: s.RelativeError(),
	}
}

func Write(w io.Writer, s *Sketch) error {
	if _, err := io.WriteString(w, Magic); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, Version); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, s.Precision); err != nil {
		return err
	}
	if err := writeString(w, prob.HashName); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint32(len(s.Registers))); err != nil {
		return err
	}
	_, err := w.Write(s.Registers)
	return err
}

func Read(r io.Reader) (*Sketch, error) {
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}
	if string(magic[:]) != Magic {
		return nil, fmt.Errorf("invalid HLL file magic %q", string(magic[:]))
	}

	var version uint8
	if err := binary.Read(r, binary.BigEndian, &version); err != nil {
		return nil, fmt.Errorf("read version: %w", err)
	}
	if version != Version {
		return nil, fmt.Errorf("unsupported HLL version %d", version)
	}

	var precision uint8
	if err := binary.Read(r, binary.BigEndian, &precision); err != nil {
		return nil, fmt.Errorf("read precision: %w", err)
	}
	hashName, err := readString(r)
	if err != nil {
		return nil, err
	}
	if hashName != prob.HashName {
		return nil, fmt.Errorf("unsupported hash %q", hashName)
	}

	var registerCount uint32
	if err := binary.Read(r, binary.BigEndian, &registerCount); err != nil {
		return nil, fmt.Errorf("read register count: %w", err)
	}
	if registerCount != 1<<precision {
		return nil, fmt.Errorf("invalid register count %d for precision %d", registerCount, precision)
	}

	s, err := New(precision)
	if err != nil {
		return nil, err
	}
	if _, err := io.ReadFull(r, s.Registers); err != nil {
		return nil, fmt.Errorf("read registers: %w", err)
	}
	maxRank := uint8(65 - precision)
	for i, rank := range s.Registers {
		if rank > maxRank {
			return nil, fmt.Errorf("invalid register %d value %d for precision %d", i, rank, precision)
		}
	}
	return s, nil
}

func alpha(m int) float64 {
	switch m {
	case 16:
		return 0.673
	case 32:
		return 0.697
	case 64:
		return 0.709
	default:
		return 0.7213 / (1 + 1.079/float64(m))
	}
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
		return "", fmt.Errorf("read string length: %w", err)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", fmt.Errorf("read string: %w", err)
	}
	return string(buf), nil
}
