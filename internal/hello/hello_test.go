package hello

import (
	"bytes"
	"testing"
)

func TestMessage(t *testing.T) {
	t.Parallel()

	if got := Message(); got != "Hello, world!" {
		t.Fatalf("Message() = %q, want %q", got, "Hello, world!")
	}
}

func TestPrint(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := Print(&buf); err != nil {
		t.Fatalf("Print() error = %v", err)
	}

	if got := buf.String(); got != "Hello, world!\n" {
		t.Fatalf("Print() wrote %q, want %q", got, "Hello, world!\n")
	}
}
