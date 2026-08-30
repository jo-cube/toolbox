package rdbsh

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteExportFileIsAtomicAndRequiresForce(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "dump.csv")
	write := func(content string, writeErr error) func(io.Writer) (int, error) {
		return func(w io.Writer) (int, error) {
			if _, err := io.WriteString(w, content); err != nil {
				return 0, err
			}
			return 1, writeErr
		}
	}

	if _, err := writeExportFile(path, false, write("existing", nil)); err != nil {
		t.Fatalf("writeExportFile(force=false) error = %v", err)
	}
	if _, err := writeExportFile(path, false, write("new", nil)); err == nil {
		t.Fatal("writeExportFile(force=false) succeeded for existing file, want error")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "existing" {
		t.Fatalf("failed export changed destination: content=%q error=%v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("export mode = %v, want 0600", info.Mode().Perm())
	}

	if _, err := writeExportFile(path, true, write("partial", errors.New("write failed"))); err == nil {
		t.Fatal("writeExportFile() succeeded after writer error")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "existing" {
		t.Fatalf("failed forced export changed destination: content=%q error=%v", got, err)
	}

	count, err := writeExportFile(path, true, write("replacement", nil))
	if err != nil {
		t.Fatalf("writeExportFile(force=true) error = %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || count != 1 || string(got) != "replacement" {
		t.Fatalf("successful export: count=%d content=%q error=%v", count, got, err)
	}
}
