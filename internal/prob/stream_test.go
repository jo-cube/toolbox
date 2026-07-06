package prob

import (
	"reflect"
	"strings"
	"testing"
)

func TestEachReaderTrimIgnoreEmpty(t *testing.T) {
	t.Parallel()

	var got []string
	err := eachReader("test", strings.NewReader(" a \n\nb\n"), InputOptions{Trim: true, IgnoreEmpty: true}, func(item []byte) error {
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
	err := eachReader("test", strings.NewReader("a\x00b\x00"), InputOptions{NUL: true}, func(item []byte) error {
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
