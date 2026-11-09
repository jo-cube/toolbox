package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/jo-cube/toolbox/internal/card"
)

func TestJSONSelectorDoesNotConsumeRelativePaths(t *testing.T) {
	t.Parallel()

	if !isJSONSelector(".user.id") {
		t.Fatal("JSON selector was rejected")
	}
	for _, path := range []string{"./events.jsonl", "../events.jsonl"} {
		if isJSONSelector(path) {
			t.Fatalf("relative path %q was treated as a JSON selector", path)
		}
	}
}

func TestWriteProfilesPropagatesOutputFailures(t *testing.T) {
	t.Parallel()
	profiles := []card.Profile{{Field: "field\fvalue", ApproxUnique: 2, Total: 3}}
	failure := errors.New("output failed")
	for _, jsonOut := range []bool{false, true} {
		var out bytes.Buffer
		if err := writeProfiles(&out, profiles, jsonOut); err != nil {
			t.Fatal(err)
		}
		for _, remaining := range []int{0, out.Len() - 1} {
			writer := &failOnceWriter{remaining: remaining, err: failure}
			if err := writeProfiles(writer, profiles, jsonOut); !errors.Is(err, failure) {
				t.Fatalf("json=%v remaining=%d: error = %v, want %v", jsonOut, remaining, err, failure)
			}
		}
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
