package rdbsh

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func newTestShell(t testing.TB) *Shell {
	t.Helper()
	ldb, err := exec.LookPath("ldb")
	if err != nil {
		ldb, err = exec.LookPath("rocksdb_ldb")
	}
	if err != nil {
		t.Skip("RocksDB integration checks require ldb or rocksdb_ldb")
	}
	path := filepath.Join(t.TempDir(), "db")
	if output, err := exec.Command(ldb, "--db="+path, "--create_if_missing", "put", "seed", "value").CombinedOutput(); err != nil {
		t.Fatalf("create DB: %v: %s", err, output)
	}
	s, err := NewShell(Config{DBPath: path, Writable: true, Out: io.Discard, ErrOut: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err := s.delete([]byte("seed")); err != nil {
		t.Fatal(err)
	}
	return s
}

func BenchmarkReadCommands(b *testing.B) {
	for _, size := range []int{32, 4096} {
		s := newTestShell(b)
		value := []byte(strings.Repeat("v", size))
		for i := range 10000 {
			if err := s.put([]byte(fmt.Sprintf("key-%05d", i)), value); err != nil {
				b.Fatal(err)
			}
		}
		out, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			b.Fatal(err)
		}
		b.Cleanup(func() { out.Close() })
		s.out = out
		for _, command := range []string{"count", "count key-0", "keys key-0 10000", "export - json"} {
			b.Run(fmt.Sprintf("value=%d/%s", size, command), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if err := s.Exec(command); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func TestReadCommandsPreserveOutput(t *testing.T) {
	s := newTestShell(t)
	for key, value := range map[string][]byte{"alpha": []byte("one"), "alpine": {}, "beta": {0, 10}} {
		if err := s.put([]byte(key), value); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct{ command, want, diagnostic string }{
		{"count", "3 keys total\n", ""},
		{"count al", "2 keys (prefix: al)\n", ""},
		{"count z", "0 keys (prefix: z)\n", ""},
		{"keys al 1", "alpha\n... (limit 1 reached)\n(1 keys shown)\n", ""},
		{"keys al 2", "alpha\nalpine\n(2 keys shown)\n", ""},
		{"keys z", "(no keys found)\n", ""},
		{"scan b 1", "beta  0x000a\n", ""},
		{"export - csv al", "key,value\nalpha,one\nalpine,0x\n", "exported 2 entries to stdout (csv)\n"},
		{"export - json al", "[\n{\"key\":\"alpha\",\"value\":\"one\"}\n,\n{\"key\":\"alpine\",\"value\":\"0x\"}\n]\n", "exported 2 entries to stdout (json)\n"},
		{"export - json z", "[\n]\n", "exported 0 entries to stdout (json)\n"},
	} {
		var out, diagnostic bytes.Buffer
		s.out, s.errOut = &out, &diagnostic
		if err := s.Exec(tt.command); err != nil {
			t.Fatalf("%s: %v", tt.command, err)
		}
		if out.String() != tt.want || diagnostic.String() != tt.diagnostic {
			t.Fatalf("%s: stdout=%q, stderr=%q", tt.command, out.String(), diagnostic.String())
		}
	}
	failure := errors.New("callback failed")
	result, err := s.iterate([]byte("al"), 0, true, func(_, _ []byte) error { return failure })
	if !errors.Is(err, failure) || result.Count != 0 {
		t.Fatalf("iterate: result=%#v, err=%v", result, err)
	}
}

type failedExportWriter struct{ err error }

func (w failedExportWriter) Write([]byte) (int, error) { return 0, w.err }

func TestJSONExportPropagatesWriteAndFlushErrors(t *testing.T) {
	s := newTestShell(t)
	failure := errors.New("output failed")
	for _, size := range []int{32, 8192} {
		if err := s.put([]byte("key"), []byte(strings.Repeat("v", size))); err != nil {
			t.Fatal(err)
		}
		count, err := s.exportJSON(failedExportWriter{failure}, nil)
		if !errors.Is(err, failure) || count != 0 {
			t.Fatalf("size %d: count=%d, err=%v", size, count, err)
		}
	}
}
