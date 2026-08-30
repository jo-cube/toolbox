package prob

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestEachReaderCanTrimAndIgnoreEmptyLines(t *testing.T) {
	t.Parallel()

	var got []string
	spacedLines := strings.Join([]string{" a ", "", "b"}, "\n") + "\n"
	err := eachReader("test", strings.NewReader(spacedLines), InputOptions{Trim: true, IgnoreEmpty: true}, func(item []byte) error {
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
	err := eachReader("test", strings.NewReader(input), InputOptions{NUL: true}, func(item []byte) error {
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
	err := eachReader("test", strings.NewReader("a\r\nb\r\n"), InputOptions{}, func(item []byte) error {
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
	if err := eachReader("test", strings.NewReader(want+"\n"), InputOptions{}, func(item []byte) error {
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
		if err := eachReader("bench", bytes.NewReader(input), InputOptions{}, func([]byte) error { return nil }); err != nil {
			b.Fatal(err)
		}
	}
}
