package rdbsh

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"testing"
)

type keyBenchmarkWriter struct {
	out   io.Writer
	calls uint64
}

func (w *keyBenchmarkWriter) Write(p []byte) (int, error) {
	w.calls++
	return w.out.Write(p)
}

func BenchmarkKeys(b *testing.B) {
	s := newTestShell(b)
	for i := range 100000 {
		if err := s.put([]byte(fmt.Sprintf("key-%08d", i)), []byte("value")); err != nil {
			b.Fatal(err)
		}
	}
	out, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { out.Close() })
	for _, limit := range []int{100, 10000, 100000} {
		b.Run(fmt.Sprintf("keys=%d", limit), func(b *testing.B) {
			for _, sink := range []string{"devnull", "discard", "buffered"} {
				b.Run(sink, func(b *testing.B) {
					var destination io.Writer = out
					if sink == "discard" {
						destination = io.Discard
					}
					counter := &keyBenchmarkWriter{out: destination}
					s.out = counter
					var buffered *bufio.Writer
					if sink == "buffered" {
						buffered = bufio.NewWriter(counter)
						s.out = buffered
					}
					command := fmt.Sprintf("keys key-0 %d", limit)
					b.ReportAllocs()
					for b.Loop() {
						if err := s.Exec(command); err != nil {
							b.Fatal(err)
						}
						if buffered != nil {
							if err := buffered.Flush(); err != nil {
								b.Fatal(err)
							}
						}
					}
					b.ReportMetric(float64(counter.calls)/float64(b.N), "writes/op")
				})
			}
			b.Run("iterate", func(b *testing.B) {
				prefix := []byte("key-0")
				b.ReportAllocs()
				for b.Loop() {
					result, err := s.iterate(prefix, limit, false, func(_, _ []byte) error { return nil })
					if err != nil || result.Count != limit {
						b.Fatalf("iterate: result=%#v, error=%v", result, err)
					}
				}
			})
		})
	}
}
