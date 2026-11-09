package rdbsh

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"syscall"
	"testing"
)

func TestKeysPreserveBinaryAndLargeOutput(t *testing.T) {
	s := newTestShell(t)
	large := strings.Repeat("z", 8192)
	for _, key := range [][]byte{nil, {0, 255}, []byte("alpha"), []byte("left\tright"), []byte("line\nbreak"), []byte(large), []byte("é"), {255}} {
		if err := s.put(key, []byte("value")); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	s.out = &out
	if err := s.Exec("keys"); err != nil {
		t.Fatal(err)
	}
	want := "(empty)\n0x00ff\nalpha\nleft\tright\nline\nbreak\n" + large + "\n0xc3a9\n0xff\n(8 keys shown)\n"
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}

func TestKeysPropagateWriteAndFlushFailures(t *testing.T) {
	small, large := newTestShell(t), newTestShell(t)
	if err := small.put([]byte("alpha"), []byte("value")); err != nil {
		t.Fatal(err)
	}
	if err := large.put([]byte(strings.Repeat("a", 8192)), []byte("value")); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("output failed")
	for _, tt := range []struct {
		name      string
		shell     *Shell
		remaining int
		writeErr  error
		wantErr   error
	}{
		{"flush", small, 0, failure, failure},
		{"footer", small, len("alpha\n"), failure, failure},
		{"short flush", small, 3, nil, io.ErrShortWrite},
		{"broken pipe", small, 0, syscall.EPIPE, syscall.EPIPE},
		{"write", large, 0, failure, failure},
		{"delayed write", large, 4096, failure, failure},
		{"short write", large, 17, nil, io.ErrShortWrite},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.shell.out = &limitedWriter{remaining: tt.remaining, err: tt.writeErr}
			if err := tt.shell.Exec("keys"); !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

type recoveringKeysWriter struct {
	out     bytes.Buffer
	failure error
}

func (w *recoveringKeysWriter) Write(p []byte) (int, error) {
	if w.failure != nil && bytes.HasPrefix(p, []byte("alpha\n")) {
		err := w.failure
		w.failure = nil
		return 0, err
	}
	return w.out.Write(p)
}

func TestKeysInteractiveOutputErrorRecovery(t *testing.T) {
	s := newTestShell(t)
	if err := s.put([]byte("alpha"), []byte("value")); err != nil {
		t.Fatal(err)
	}
	out := &recoveringKeysWriter{failure: errors.New("output failed")}
	var diagnostic bytes.Buffer
	s.in = strings.NewReader("keys\nkeys\nquit\n")
	s.out, s.errOut = out, &diagnostic
	if err := s.Run(); err != nil {
		t.Fatal(err)
	}
	wantSuffix := "rdbsh> rdbsh> alpha\n(1 keys shown)\nrdbsh> Bye!\n"
	if !strings.HasSuffix(out.out.String(), wantSuffix) || diagnostic.String() != "error: output failed\n" {
		t.Fatalf("stdout = %q, stderr = %q", out.out.String(), diagnostic.String())
	}
}
