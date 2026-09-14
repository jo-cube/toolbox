package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jo-cube/toolbox/internal/buildinfo"
	"github.com/jo-cube/toolbox/internal/hll"
	"github.com/jo-cube/toolbox/internal/prob"
)

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-V") {
		fmt.Fprintf(os.Stdout, "hll %s\n", buildinfo.Version())
		return
	}
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		usage()
		if len(os.Args) < 2 {
			os.Exit(2)
		}
		return
	}

	var err error
	switch os.Args[1] {
	case "count":
		err = count(os.Args[2:], os.Stdin, os.Stdout)
	case "build":
		err = build(os.Args[2:], os.Stdin, os.Stdout)
	case "estimate":
		err = estimate(os.Args[2:])
	case "merge":
		err = merge(os.Args[2:])
	case "inspect":
		err = inspect(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "hll: %v\n", err)
		if strings.HasPrefix(err.Error(), "usage:") ||
			strings.Contains(err.Error(), "precision must be between") {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage: hll <command> [options]

Estimate distinct values with HyperLogLog.

Commands:
  count       Read values and print an approximate unique count.
  build       Read values and write a binary .hll sketch to stdout.
  estimate    Read a .hll sketch and print its approximate unique count.
  merge       Merge compatible .hll sketches and write a sketch to stdout.
  inspect     Print .hll metadata and estimate.

Global options:
  -V, --version  Print version information.

Examples:
  jq -r .user_id events.jsonl | hll count
  jq -r .user_id events.jsonl | hll build > users.hll
  hll merge monday.hll tuesday.hll > week.hll

Run "hll <command> -h" for command-specific flags.
`)
}

func count(args []string, in io.Reader, out io.Writer) error {
	fs := flag.NewFlagSet("hll count", flag.ExitOnError)
	precision := fs.Uint("precision", uint(hll.DefaultP), "HLL precision, 4..20")
	jsonOut := fs.Bool("json", false, "write JSON output")
	var input prob.InputOptions
	prob.AddInputFlags(fs, &input)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: hll count [options] [file...]

Read newline-delimited values from files or stdin and print an approximate unique count.
Use --delimiter and --field to count one field per record.
Empty values and surrounding whitespace are significant unless input flags change that.

Example:
  awk '{print $1}' access.log | hll count --ignore-empty

Options:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := input.Fields.Validate(); err != nil {
		return fmt.Errorf("usage: %w", err)
	}
	p, err := hll.Precision(*precision)
	if err != nil {
		return err
	}
	s, err := hll.New(p)
	if err != nil {
		return err
	}
	if err := prob.EachInputFrom(fs.Args(), in, input, func(item []byte) error {
		s.Add(item)
		return nil
	}); err != nil {
		return err
	}
	return writeEstimate(out, s, *jsonOut)
}

func build(args []string, in io.Reader, out io.Writer) error {
	fs := flag.NewFlagSet("hll build", flag.ExitOnError)
	precision := fs.Uint("precision", uint(hll.DefaultP), "HLL precision, 4..20")
	var input prob.InputOptions
	prob.AddInputFlags(fs, &input)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: hll build [options] [file...] > file.hll

Read values from files or stdin and write a binary HyperLogLog sketch to stdout.
Use --delimiter and --field to hash one field per record.
Redirect stdout to save the sketch.

Options:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := input.Fields.Validate(); err != nil {
		return fmt.Errorf("usage: %w", err)
	}
	p, err := hll.Precision(*precision)
	if err != nil {
		return err
	}
	s, err := hll.New(p)
	if err != nil {
		return err
	}
	if err := prob.EachInputFrom(fs.Args(), in, input, func(item []byte) error {
		s.Add(item)
		return nil
	}); err != nil {
		return err
	}
	return hll.Write(out, s)
}

func estimate(args []string) error {
	fs := flag.NewFlagSet("hll estimate", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "write JSON output")
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: hll estimate [--json] <file.hll>

Read a binary HLL sketch and print its approximate unique count.

Options:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: hll estimate <file>")
	}

	s, err := readSketch(fs.Arg(0))
	if err != nil {
		return err
	}
	return writeEstimate(os.Stdout, s, *jsonOut)
}

func merge(args []string) error {
	fs := flag.NewFlagSet("hll merge", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: hll merge <file.hll> <file.hll>... > merged.hll

Merge compatible HLL sketches and write a binary sketch to stdout.
Sketches must use compatible precision, register count, version, and hash metadata.
`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return fmt.Errorf("usage: hll merge <file> <file>...")
	}
	if countStdin(fs.Args()) > 1 {
		return fmt.Errorf("usage: hll merge accepts stdin only once")
	}

	merged, err := readSketch(fs.Arg(0))
	if err != nil {
		return err
	}
	for _, path := range fs.Args()[1:] {
		s, err := readSketch(path)
		if err != nil {
			return err
		}
		if err := merged.Merge(s); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return hll.Write(os.Stdout, merged)
}

func inspect(args []string) error {
	fs := flag.NewFlagSet("hll inspect", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "write JSON output")
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: hll inspect [--json] <file.hll>

Print HLL state-file metadata, including precision, hash name, and estimate.

Options:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: hll inspect <file>")
	}

	s, err := readSketch(fs.Arg(0))
	if err != nil {
		return err
	}
	m := s.Metadata()
	if *jsonOut {
		return json.NewEncoder(os.Stdout).Encode(m)
	}
	fmt.Fprintf(os.Stdout, "type=%s\n", m.Type)
	fmt.Fprintf(os.Stdout, "version=%d\n", m.Version)
	fmt.Fprintf(os.Stdout, "precision=%d\n", m.Precision)
	fmt.Fprintf(os.Stdout, "registers=%d\n", m.Registers)
	fmt.Fprintf(os.Stdout, "hash=%s\n", m.Hash)
	fmt.Fprintf(os.Stdout, "approx_unique=%d\n", m.ApproxUnique)
	fmt.Fprintf(os.Stdout, "relative_error=%.2f%%\n", m.RelativeError*100)
	return nil
}

func readSketch(path string) (*hll.Sketch, error) {
	if path == "-" {
		return hll.Read(os.Stdin)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	return hll.Read(f)
}

func countStdin(paths []string) int {
	count := 0
	for _, path := range paths {
		if path == "-" {
			count++
		}
	}
	return count
}

func writeEstimate(out io.Writer, s *hll.Sketch, jsonOut bool) error {
	m := s.Metadata()
	if jsonOut {
		return json.NewEncoder(out).Encode(struct {
			ApproxUnique  uint64  `json:"approx_unique"`
			RelativeError float64 `json:"relative_error"`
		}{m.ApproxUnique, m.RelativeError})
	}
	_, err := fmt.Fprintf(out, "approx_unique=%d\nrelative_error=%.2f%%\n", m.ApproxUnique, m.RelativeError*100)
	return err
}
