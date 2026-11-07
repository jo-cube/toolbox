package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/jo-cube/toolbox/internal/heavy"
)

func TestWriteResultsPropagatesOutputFailures(t *testing.T) {
	t.Parallel()
	results := []heavy.Result{{Rank: 1, Item: "a\fvalue", CountEstimate: 2, CountLowerBound: 1}}
	failure := errors.New("output failed")
	for _, format := range []struct {
		name string
		json bool
		tsv  bool
	}{{name: "table"}, {name: "tsv", tsv: true}, {name: "json", json: true}} {
		t.Run(format.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeResults(&out, results, format.json, format.tsv); err != nil {
				t.Fatal(err)
			}
			for _, remaining := range []int{0, out.Len() - 1} {
				writer := &failOnceWriter{remaining: remaining, err: failure}
				if err := writeResults(writer, results, format.json, format.tsv); !errors.Is(err, failure) {
					t.Fatalf("remaining=%d: error = %v, want %v", remaining, err, failure)
				}
			}
		})
	}
}

type failOnceWriter struct {
	remaining int
	err       error
}

func (w *failOnceWriter) Write(p []byte) (int, error) {
	n := min(len(p), w.remaining)
	w.remaining -= n
	if n < len(p) && w.err != nil {
		err := w.err
		w.err = nil
		return n, err
	}
	return len(p), nil
}
