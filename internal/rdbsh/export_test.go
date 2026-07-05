package rdbsh

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenExportFileRequiresForceToOverwrite(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "dump.csv")
	if err := os.WriteFile(path, []byte("existing"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if file, err := openExportFile(path, false); err == nil {
		file.Close()
		t.Fatal("openExportFile(force=false) succeeded for existing file, want error")
	}

	file, err := openExportFile(path, true)
	if err != nil {
		t.Fatalf("openExportFile(force=true) error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("openExportFile(force=true) left size %d, want 0", info.Size())
	}
}
