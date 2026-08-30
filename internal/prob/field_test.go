package prob

import (
	"bytes"
	"testing"
)

func TestFieldOptionsSelectsOneBasedLiteralField(t *testing.T) {
	t.Parallel()

	opts := FieldOptions{Delimiter: "::", Field: 2}
	got, err := opts.Select([]byte("left::middle::right"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("middle")) {
		t.Fatalf("Select() = %q, want middle", got)
	}
	got, err = opts.Select([]byte("left::"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("Select() = %q, want empty field", got)
	}
	if _, err := opts.Select([]byte("left")); err == nil {
		t.Fatal("Select() accepted a missing field")
	}
}

func TestFieldOptionsRequiresDelimiterAndFieldTogether(t *testing.T) {
	t.Parallel()

	for _, opts := range []FieldOptions{{Delimiter: "\t"}, {Field: 2}, {Delimiter: "\t", Field: -1}} {
		if err := opts.Validate(); err == nil {
			t.Fatalf("Validate(%#v) succeeded", opts)
		}
	}
	if err := (FieldOptions{}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkFieldOptionsSelect(b *testing.B) {
	opts := FieldOptions{Delimiter: "\t", Field: 3}
	record := []byte("2026-08-31T00:00:00Z\ttenant-42\tuser-123\tpayload")
	b.ReportAllocs()
	for b.Loop() {
		field, err := opts.Select(record)
		if err != nil || len(field) != 8 {
			b.Fatalf("Select() = %q, %v", field, err)
		}
	}
}
