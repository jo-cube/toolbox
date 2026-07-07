package prob

import (
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
