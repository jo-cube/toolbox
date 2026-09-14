package prob

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEachReaderCanTrimAndIgnoreEmptyLines(t *testing.T) {
	t.Parallel()

	var got []string
	spacedLines := strings.Join([]string{" a ", "", "b"}, "\n") + "\n"
	err := EachInputFrom(nil, strings.NewReader(spacedLines), InputOptions{Trim: true, IgnoreEmpty: true}, func(item []byte) error {
		got = append(got, string(item))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("items = %#v, want %#v", got, want)
	}
}

func TestEachReaderNULDelimited(t *testing.T) {
	t.Parallel()

	var got []string
	input := string([]byte{'a', 0, 'b', 0})
	err := EachInputFrom(nil, strings.NewReader(input), InputOptions{NUL: true}, func(item []byte) error {
		got = append(got, string(item))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("items = %#v, want %#v", got, want)
	}
}

func TestEachReaderRemovesCRLFDelimiter(t *testing.T) {
	t.Parallel()

	var got []string
	err := EachInputFrom(nil, strings.NewReader("a\r\nb\r\n"), InputOptions{}, func(item []byte) error {
		got = append(got, string(item))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("items = %#v, want %#v", got, want)
	}
}

func TestEachInputAcceptsStdinMarker(t *testing.T) {
	t.Parallel()

	var got []string
	err := EachInputFrom([]string{"-"}, strings.NewReader("a\nb\n"), InputOptions{}, func(item []byte) error {
		got = append(got, string(item))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("items = %#v, want %#v", got, want)
	}
	if err := EachInputFrom([]string{"-", "-"}, strings.NewReader("a\n"), InputOptions{}, func([]byte) error { return nil }); err == nil {
		t.Fatal("EachInputFrom accepted stdin twice")
	}
}

func TestEachReaderAcceptsLargeItems(t *testing.T) {
	t.Parallel()

	want := strings.Repeat("x", 8<<10)
	var got string
	if err := EachInputFrom(nil, strings.NewReader(want+"\n"), InputOptions{}, func(item []byte) error {
		got = string(item)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("item length = %d, want %d", len(got), len(want))
	}
}

func BenchmarkEachReader(b *testing.B) {
	input := bytes.Repeat([]byte("0123456789abcdef0123456789abcdef\n"), 10_000)
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	for b.Loop() {
		if err := EachInputFrom(nil, bytes.NewReader(input), InputOptions{}, func([]byte) error { return nil }); err != nil {
			b.Fatal(err)
		}
	}
}

func TestSharedInputPreservesFileBoundariesAndSelection(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "records")
	if err := os.WriteFile(path, []byte("first:: a ::payload"), 0600); err != nil {
		t.Fatal(err)
	}
	opts := InputOptions{Trim: true, IgnoreEmpty: true, Fields: FieldOptions{Delimiter: "::", Field: 2}}
	var values, records []string
	err := EachSelectedInputFrom([]string{path, "-"}, strings.NewReader("second:: b ::payload\r\nskip:: ::payload\n"), opts, func(item, output []byte) error {
		values = append(values, string(item))
		records = append(records, string(output))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(values, []string{"a", "b"}) || !reflect.DeepEqual(records, []string{"first:: a ::payload", "second:: b ::payload"}) {
		t.Fatalf("values = %q, records = %q", values, records)
	}
	if err := EachInputFrom(nil, strings.NewReader("missing\n"), opts, func([]byte) error { return nil }); err == nil {
		t.Fatal("accepted a missing field")
	}
}

func TestRawInputPreservesLongBinaryRecords(t *testing.T) {
	t.Parallel()
	for _, nul := range []bool{false, true} {
		delim := "\n"
		if nul {
			delim = "\x00"
		}
		want := []string{delim, strings.Repeat("x", 16<<10) + "\r" + delim, strings.Repeat("y", 16<<10) + delim, "last\r"}
		var got []string
		err := EachRecordFrom(nil, strings.NewReader(strings.Join(want, "")), nul, func(record []byte) error {
			got = append(got, string(record))
			return nil
		})
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("NUL=%v: record preservation failed: %v", nul, err)
		}
	}
}

func TestSharedInputPropagatesErrorsAndRejectsRepeatedStdinBeforeReading(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("input failure")
	for _, run := range []func() error{
		func() error {
			return EachInputFrom(nil, strings.NewReader("value\n"), InputOptions{}, func([]byte) error { return sentinel })
		},
		func() error {
			return EachRecordFrom(nil, io.MultiReader(strings.NewReader("value\n"), failingReader{sentinel}), false, func([]byte) error { return nil })
		},
	} {
		if err := run(); !errors.Is(err, sentinel) {
			t.Fatalf("error = %v, want %v", err, sentinel)
		}
	}
	if err := EachFile([]string{"-", "-"}, strings.NewReader("value"), func(string, io.Reader) error {
		t.Fatal("consumed input before rejecting repeated stdin")
		return nil
	}); err == nil {
		t.Fatal("accepted repeated stdin")
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func BenchmarkLongRecords(b *testing.B) {
	input := bytes.Repeat(append(bytes.Repeat([]byte("x"), 16<<10), '\n'), 1000)
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	for b.Loop() {
		if err := EachInputFrom(nil, bytes.NewReader(input), InputOptions{}, func([]byte) error { return nil }); err != nil {
			b.Fatal(err)
		}
	}
}
