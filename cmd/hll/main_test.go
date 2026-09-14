package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jo-cube/toolbox/internal/hll"
)

func TestCountAndBuildSelectTheSameValues(t *testing.T) {
	t.Parallel()
	for _, nul := range []bool{false, true} {
		args := []string{"--delimiter", "::", "--field", "2", "--trim", "--ignore-empty"}
		input := "1:: a ::x\n2::b::y\n3::a::z\n4:: ::empty\n5::c"
		if nul {
			args = append(args, "-0")
			input = strings.ReplaceAll(input, "\n", "\x00")
		}
		var direct, artifact, saved bytes.Buffer
		if err := count(args, strings.NewReader(input), &direct); err != nil {
			t.Fatal(err)
		}
		if err := build(args, strings.NewReader(input), &artifact); err != nil {
			t.Fatal(err)
		}
		sketch, err := hll.Read(&artifact)
		if err != nil {
			t.Fatal(err)
		}
		if err := writeEstimate(&saved, sketch, false); err != nil {
			t.Fatal(err)
		}
		if direct.String() != saved.String() || !strings.Contains(direct.String(), "approx_unique=3\n") {
			t.Fatalf("direct = %q, saved = %q", direct.String(), saved.String())
		}
	}
}

func TestInputErrorsDoNotProduceASketchOrCount(t *testing.T) {
	t.Parallel()
	for _, run := range []func([]string, io.Reader, io.Writer) error{count, build} {
		for _, args := range [][]string{{"--field", "2"}, {"-d", "::", "-f", "2"}} {
			var out bytes.Buffer
			err := run(args, strings.NewReader("missing-field\n"), &out)
			if err == nil || out.Len() != 0 {
				t.Fatalf("args = %q, error = %v, output bytes = %d", args, err, out.Len())
			}
			if len(args) == 2 && !strings.HasPrefix(err.Error(), "usage:") {
				t.Fatalf("invalid field options must be a usage error: %v", err)
			}
		}
	}
}

func TestCountPropagatesOutputFailure(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("output failed")
	if err := count(nil, strings.NewReader("a\n"), failingWriter{sentinel}); !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want %v", err, sentinel)
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }
