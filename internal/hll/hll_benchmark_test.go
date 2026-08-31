package hll

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"testing"

	"github.com/jo-cube/toolbox/internal/prob"
)

var (
	benchmarkEstimate uint64
	benchmarkSketch   *Sketch
)

func BenchmarkAddHash(b *testing.B) {
	for _, precision := range []uint8{10, DefaultP, 20} {
		b.Run("p="+strconv.Itoa(int(precision)), func(b *testing.B) {
			s, err := New(precision)
			if err != nil {
				b.Fatal(err)
			}
			var hash uint64
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				hash += 0x9e3779b97f4a7c15
				s.AddHash(hash)
			}
			benchmarkSketch = s
		})
	}
}

func BenchmarkAdd(b *testing.B) {
	for _, size := range []int{16, 32, 128} {
		keys := benchmarkKeys(size, 1<<16)
		b.Run(strconv.Itoa(size)+"B", func(b *testing.B) {
			s, err := New(DefaultP)
			if err != nil {
				b.Fatal(err)
			}
			var i int
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				offset := (i & ((1 << 16) - 1)) * size
				s.Add(keys[offset : offset+size])
				i++
			}
			benchmarkSketch = s
		})
	}
}

func BenchmarkEstimate(b *testing.B) {
	for _, precision := range []uint8{10, DefaultP, 20} {
		b.Run("p="+strconv.Itoa(int(precision)), func(b *testing.B) {
			s, err := New(precision)
			if err != nil {
				b.Fatal(err)
			}
			for i := range s.Registers {
				s.AddHash(mix64(uint64(i)))
			}
			b.SetBytes(int64(len(s.Registers)))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				benchmarkEstimate = s.Estimate()
			}
		})
	}
}

func BenchmarkMerge(b *testing.B) {
	for _, precision := range []uint8{10, DefaultP, 20} {
		b.Run("p="+strconv.Itoa(int(precision)), func(b *testing.B) {
			dst, err := New(precision)
			if err != nil {
				b.Fatal(err)
			}
			src, err := New(precision)
			if err != nil {
				b.Fatal(err)
			}
			for i := range src.Registers {
				src.Registers[i] = uint8(1 + i%5)
			}
			b.SetBytes(int64(2 * len(dst.Registers)))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := dst.Merge(src); err != nil {
					b.Fatal(err)
				}
			}
			benchmarkSketch = dst
		})
	}
}

func BenchmarkCountStream(b *testing.B) {
	const keyCount = 1_000_000
	input := benchmarkLines(keyCount)
	b.Run("noop", func(b *testing.B) {
		b.SetBytes(int64(len(input)))
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			if err := prob.EachInputFrom(nil, bytes.NewReader(input), prob.InputOptions{}, func([]byte) error { return nil }); err != nil {
				b.Fatal(err)
			}
		}
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*keyCount), "ns/key")
	})
	b.Run("count", func(b *testing.B) {
		b.SetBytes(int64(len(input)))
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			s, err := New(DefaultP)
			if err != nil {
				b.Fatal(err)
			}
			if err := prob.EachInputFrom(nil, bytes.NewReader(input), prob.InputOptions{}, func(key []byte) error {
				s.Add(key)
				return nil
			}); err != nil {
				b.Fatal(err)
			}
			benchmarkSketch = s
		}
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*keyCount), "ns/key")
	})
}

func benchmarkKeys(size, count int) []byte {
	keys := make([]byte, size*count)
	for i := range count {
		binary.LittleEndian.PutUint64(keys[i*size:], mix64(uint64(i)))
	}
	return keys
}

func benchmarkLines(count int) []byte {
	lines := make([]byte, count*17)
	var raw [8]byte
	for i := range count {
		binary.LittleEndian.PutUint64(raw[:], mix64(uint64(i)))
		offset := i * 17
		hex.Encode(lines[offset:offset+16], raw[:])
		lines[offset+16] = '\n'
	}
	return lines
}
