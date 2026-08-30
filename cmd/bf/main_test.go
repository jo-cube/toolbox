package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	internalbf "github.com/jo-cube/toolbox/internal/bf"
)

func TestNULTestPreservesDelimiter(t *testing.T) {
	t.Parallel()

	f := newFilter(t, "aa", "bb")
	path := writeFilter(t, "filter.bf", f)
	var out bytes.Buffer
	if err := test([]string{"-0", path}, bytes.NewReader([]byte("aa\x00cc\x00bb\x00")), &out); err != nil {
		t.Fatal(err)
	}
	if want := []byte("aa\x00bb\x00"); !bytes.Equal(out.Bytes(), want) {
		t.Fatalf("test output = %q, want %q", out.Bytes(), want)
	}
}

func TestDedupeEmitsFirstProbablyUnseenItem(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := dedupe([]string{"--expected-items", "100", "--false-positive-rate", "0.000001"}, bytes.NewBufferString("a\nb\na\n"), &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != "a\nb\n" {
		t.Fatalf("dedupe output = %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("dedupe stderr = %q", errOut.String())
	}
}

func TestBuildWarnsWhenExpectedItemsAreExceeded(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := build([]string{"--expected-items", "1", "--false-positive-rate", "0.01"}, bytes.NewBufferString("a\nb\n"), &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(errOut.Bytes(), []byte("inserted items (2) exceed expected items (1)")) {
		t.Fatalf("build stderr = %q", errOut.String())
	}
	if _, err := internalbf.Read(&out); err != nil {
		t.Fatalf("build output is not a filter: %v", err)
	}
}

func TestInspectReadsFilterFromStdin(t *testing.T) {
	t.Parallel()

	f := newFilter(t, "alpha")
	var serialized, out bytes.Buffer
	if err := internalbf.Write(&serialized, f); err != nil {
		t.Fatal(err)
	}
	if err := inspect([]string{"--json", "-"}, &serialized, &out); err != nil {
		t.Fatal(err)
	}
	var metadata internalbf.Metadata
	if err := json.Unmarshal(out.Bytes(), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.SetBits == 0 || metadata.BitsetBytes != uint64(len(f.Bits)) {
		t.Fatalf("metadata = %#v", metadata)
	}
}

func TestUnionReadsOneFilterFromStdin(t *testing.T) {
	t.Parallel()

	a := newFilter(t, "alpha")
	b := newFilter(t, "beta")
	path := writeFilter(t, "a.bf", a)
	var serializedB, combined bytes.Buffer
	if err := internalbf.Write(&serializedB, b); err != nil {
		t.Fatal(err)
	}
	if err := union([]string{path, "-"}, &serializedB, &combined); err != nil {
		t.Fatal(err)
	}
	got, err := internalbf.Read(&combined)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Test([]byte("alpha")) || !got.Test([]byte("beta")) {
		t.Fatal("union lost an inserted item")
	}
}

func newFilter(t *testing.T, items ...string) *internalbf.Filter {
	t.Helper()
	f, err := internalbf.New(100, 0.000001)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		f.Add([]byte(item))
	}
	return f
}

func writeFilter(t *testing.T, name string, f *internalbf.Filter) string {
	t.Helper()
	var data bytes.Buffer
	if err := internalbf.Write(&data, f); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
