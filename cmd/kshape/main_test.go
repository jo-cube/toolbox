package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jo-cube/toolbox/internal/kshape"
)

func TestFormatBuildInspectAndMergePipeline(t *testing.T) {
	t.Parallel()

	var formatOut, errOut bytes.Buffer
	if status := run([]string{"format"}, nil, &formatOut, &errOut); status != 0 {
		t.Fatalf("format status = %d, stderr = %s", status, errOut.String())
	}
	if got := formatOut.String(); got != kshape.CanonicalFormat+"\n" {
		t.Fatalf("format output = %q", got)
	}

	artifactA := runBuild(t, formattedRecord(0, 1, []byte{'a', '\n', 0xff}))
	artifactB := runBuild(t, formattedRecord(1, 5, nil))
	pathA := writeFile(t, "a.kshape", artifactA)
	pathB := writeFile(t, "b.kshape", artifactB)

	var merged bytes.Buffer
	errOut.Reset()
	if status := run([]string{"merge", pathA, pathB}, nil, &merged, &errOut); status != 0 {
		t.Fatalf("merge status = %d, stderr = %s", status, errOut.String())
	}
	mergedPath := writeFile(t, "merged.kshape", merged.Bytes())

	var inspectOut bytes.Buffer
	errOut.Reset()
	if status := run([]string{"inspect", "--json", mergedPath}, nil, &inspectOut, &errOut); status != 0 {
		t.Fatalf("inspect status = %d, stderr = %s", status, errOut.String())
	}
	var report kshape.Report
	if err := json.Unmarshal(inspectOut.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Topic != "events" || len(report.Partitions) != 2 {
		t.Fatalf("report = %#v", report)
	}
	if report.Version != kshape.Version || report.Checksum != kshape.ChecksumName || report.Partitions[0].Regions[0].ObservedRecords != 1 {
		t.Fatalf("report metadata = %#v", report)
	}

	inspectOut.Reset()
	errOut.Reset()
	if status := run([]string{"inspect", mergedPath}, nil, &inspectOut, &errOut); status != 0 {
		t.Fatalf("inspect status = %d, stderr = %s", status, errOut.String())
	}
	for _, want := range []string{"checksum=crc32c", "observed_records=1", "keyed_records=1"} {
		if !strings.Contains(inspectOut.String(), want) {
			t.Errorf("inspect output %q does not contain %q", inspectOut.String(), want)
		}
	}

	var renderOut bytes.Buffer
	errOut.Reset()
	if status := run([]string{"render", "--title", "events <shape>", "--metric", "rewrite", mergedPath}, nil, &renderOut, &errOut); status != 0 {
		t.Fatalf("render status = %d, stderr = %s", status, errOut.String())
	}
	for _, want := range []string{"<!doctype html>", "events &lt;shape&gt;", `"initialMetric":"rewrite"`, "Partition × offset-space shape"} {
		if !strings.Contains(renderOut.String(), want) {
			t.Errorf("render output does not contain %q", want)
		}
	}
}

func TestRunExitStatuses(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{nil, {"nope"}, {"build", "extra"}, {"build", "--precision", "99"}, {"build", "--bucket-width", "0"}} {
		var out, errOut bytes.Buffer
		if status := run(args, bytes.NewReader(nil), &out, &errOut); status != 2 {
			t.Errorf("run(%v) status = %d, want 2", args, status)
		}
	}
	var out, errOut bytes.Buffer
	if status := run([]string{"build"}, bytes.NewBufferString("bad"), &out, &errOut); status != 1 {
		t.Fatalf("malformed build status = %d, want 1", status)
	}
	out.Reset()
	errOut.Reset()
	if status := run([]string{"render", "--metric", "unknown", "missing.kshape"}, nil, &out, &errOut); status != 2 {
		t.Fatalf("invalid render metric status = %d, want 2", status)
	}
	badPath := writeFile(t, "bad.kshape", []byte("bad"))
	out.Reset()
	errOut.Reset()
	if status := run([]string{"render", badPath}, nil, &out, &errOut); status != 1 || out.Len() != 0 {
		t.Fatalf("corrupt render status = %d, stdout = %q, want status 1 and empty stdout", status, out.String())
	}
}

func runBuild(t *testing.T, input []byte) []byte {
	t.Helper()
	var out, errOut bytes.Buffer
	if status := run([]string{"build", "--bucket-width", "4", "--precision", "8"}, bytes.NewReader(input), &out, &errOut); status != 0 {
		t.Fatalf("build status = %d, stderr = %s", status, errOut.String())
	}
	return out.Bytes()
}

func formattedRecord(partition int, offset int64, key []byte) []byte {
	var out bytes.Buffer
	fmt.Fprintf(&out, "events\t%d\t%d\t100\t7\t%d\t", partition, offset, len(key))
	out.Write(key)
	out.WriteByte('\n')
	return out.Bytes()
}

func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
