package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jo-cube/toolbox/internal/bf"
	"github.com/jo-cube/toolbox/internal/buildinfo"
	"github.com/jo-cube/toolbox/internal/prob"
)

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-V") {
		fmt.Fprintf(os.Stdout, "bf %s\n", buildinfo.Version())
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
	case "build":
		err = build(os.Args[2:])
	case "test":
		err = test(os.Args[2:])
	case "inspect":
		err = inspect(os.Args[2:])
	case "union":
		err = union(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "bf: %v\n", err)
		if strings.HasPrefix(err.Error(), "usage:") ||
			strings.Contains(err.Error(), "expected-items") ||
			strings.Contains(err.Error(), "false-positive-rate") {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage: bf <command> [options]

Build and query Bloom filters for approximate membership tests.
False positives are possible. False negatives should not occur unless a filter is corrupted or misused.

Commands:
  build      Read values and write a binary .bf filter to stdout.
  test       Emit values that are probably present, or definitely absent with --invert.
  inspect    Print .bf metadata.
  union      Combine compatible .bf filters and write a filter to stdout.

Global options:
  -V, --version  Print version information.

Examples:
  cat known.txt | bf build --expected-items 1000000 --false-positive-rate 0.001 > known.bf
  cat candidates.txt | bf test known.bf
  cat candidates.txt | bf test --invert known.bf

Run "bf <command> -h" for command-specific flags.
`)
}

func build(args []string) error {
	fs := flag.NewFlagSet("bf build", flag.ExitOnError)
	expected := fs.Uint64("expected-items", 0, "expected number of inserted items")
	rate := fs.Float64("false-positive-rate", 0, "target false-positive rate")
	var input prob.InputOptions
	prob.AddInputFlags(fs, &input)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: bf build --expected-items <n> --false-positive-rate <p> [file...] > filter.bf

Read values from files or stdin and write a binary Bloom filter to stdout.
Sizing flags are required because they determine memory use and false-positive behavior.

Options:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	f, err := bf.New(*expected, *rate)
	if err != nil {
		return err
	}
	if err := prob.EachInput(fs.Args(), input, func(item []byte) error {
		f.Add(item)
		return nil
	}); err != nil {
		return err
	}
	return bf.Write(os.Stdout, f)
}

func test(args []string) error {
	fs := flag.NewFlagSet("bf test", flag.ExitOnError)
	invert := fs.Bool("invert", false, "emit definitely absent items")
	var input prob.InputOptions
	prob.AddInputFlags(fs, &input)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: bf test [--invert] <filter.bf> [file...]

Read candidates from files or stdin.
Default output is values probably present in the filter.
With --invert, output is values definitely absent from the filter.

Options:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: bf test [--invert] <filter> [file...]")
	}

	f, err := readFilter(fs.Arg(0))
	if err != nil {
		return err
	}
	return prob.EachInput(fs.Args()[1:], input, func(item []byte) error {
		present := f.Test(item)
		if present != *invert {
			_, err := os.Stdout.Write(append(append([]byte(nil), item...), '\n'))
			return err
		}
		return nil
	})
}

func inspect(args []string) error {
	fs := flag.NewFlagSet("bf inspect", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "write JSON output")
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: bf inspect [--json] <filter.bf>

Print Bloom filter metadata, including expected items, inserted items, bit count, and hash count.

Options:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: bf inspect <filter>")
	}

	f, err := readFilter(fs.Arg(0))
	if err != nil {
		return err
	}
	m := f.Metadata()
	if *jsonOut {
		return json.NewEncoder(os.Stdout).Encode(m)
	}
	fmt.Fprintf(os.Stdout, "type=%s\n", m.Type)
	fmt.Fprintf(os.Stdout, "version=%d\n", m.Version)
	fmt.Fprintf(os.Stdout, "expected_items=%d\n", m.ExpectedItems)
	fmt.Fprintf(os.Stdout, "inserted_items=%d\n", m.InsertedItems)
	fmt.Fprintf(os.Stdout, "false_positive_rate=%g\n", m.FalsePositiveRate)
	fmt.Fprintf(os.Stdout, "bit_count=%d\n", m.BitCount)
	fmt.Fprintf(os.Stdout, "hash_count=%d\n", m.HashCount)
	fmt.Fprintf(os.Stdout, "hash=%s\n", m.Hash)
	return nil
}

func union(args []string) error {
	fs := flag.NewFlagSet("bf union", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: bf union <filter.bf> <filter.bf>... > combined.bf

Union compatible Bloom filters and write a binary filter to stdout.
Filters must have compatible bit count, hash count, false-positive rate, version, and hash metadata.
`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return fmt.Errorf("usage: bf union <filter> <filter>...")
	}

	merged, err := readFilter(fs.Arg(0))
	if err != nil {
		return err
	}
	for _, path := range fs.Args()[1:] {
		f, err := readFilter(path)
		if err != nil {
			return err
		}
		if err := merged.Union(f); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return bf.Write(os.Stdout, merged)
}

func readFilter(path string) (*bf.Filter, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	return bf.Read(f)
}
