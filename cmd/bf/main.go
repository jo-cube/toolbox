package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
		usage(os.Stderr)
		if len(os.Args) < 2 {
			os.Exit(2)
		}
		return
	}

	var err error
	switch os.Args[1] {
	case "build":
		err = build(os.Args[2:], os.Stdin, os.Stdout, os.Stderr)
	case "test":
		err = test(os.Args[2:], os.Stdin, os.Stdout)
	case "dedupe":
		err = dedupe(os.Args[2:], os.Stdin, os.Stdout, os.Stderr)
	case "inspect":
		err = inspect(os.Args[2:], os.Stdin, os.Stdout)
	case "union":
		err = union(os.Args[2:], os.Stdin, os.Stdout)
	default:
		usage(os.Stderr)
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

func usage(out io.Writer) {
	fmt.Fprint(out, `Usage: bf <command> [options]

Build and query Bloom filters for approximate membership tests.
False positives are possible. False negatives should not occur unless a filter is corrupted or misused.

Commands:
  build      Read values and write a binary .bf filter to stdout.
  test       Emit values that are probably present, or definitely absent with --invert.
  dedupe     Emit the first probably unseen occurrence of each value.
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

func build(args []string, in io.Reader, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("bf build", flag.ExitOnError)
	fs.SetOutput(errOut)
	expected := fs.Uint64("expected-items", 0, "expected number of inserted items")
	rate := fs.Float64("false-positive-rate", 0, "target false-positive rate")
	noSizeLimit := addSizeLimitFlag(fs)
	var input prob.InputOptions
	prob.AddInputFlags(fs, &input)
	var fields prob.FieldOptions
	prob.AddFieldFlags(fs, &fields)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: bf build --expected-items <n> --false-positive-rate <p> [--delimiter value --field n] [--no-size-limit] [file...] > filter.bf

Read values from files or stdin and write a binary Bloom filter to stdout.
Sizing flags are required because they determine memory use and false-positive behavior.
With field selection, only that field is inserted.

Options:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := fields.Validate(); err != nil {
		return fmt.Errorf("usage: %w", err)
	}

	f, err := bf.NewWithLimit(*expected, *rate, filterSizeLimit(*noSizeLimit))
	if err != nil {
		return err
	}
	if err := eachSelectedInput(fs.Args(), in, input, fields, func(item, _ []byte) error {
		f.Add(item)
		return nil
	}); err != nil {
		return err
	}
	if err := warnIfOverfilled(errOut, f); err != nil {
		return err
	}
	return bf.Write(out, f)
}

func test(args []string, in io.Reader, out io.Writer) error {
	fs := flag.NewFlagSet("bf test", flag.ExitOnError)
	invert := fs.Bool("invert", false, "emit definitely absent items")
	noSizeLimit := addSizeLimitFlag(fs)
	var input prob.InputOptions
	prob.AddInputFlags(fs, &input)
	var fields prob.FieldOptions
	prob.AddFieldFlags(fs, &fields)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: bf test [--invert] [--delimiter value --field n] [--no-size-limit] <filter.bf> [file...]

Read candidates from files or stdin.
Default output is values probably present in the filter.
With --invert, output is values definitely absent from the filter.
With field selection, test that field and emit the complete record.

Options:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := fields.Validate(); err != nil {
		return fmt.Errorf("usage: %w", err)
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: bf test [--invert] [--no-size-limit] <filter> [file...]")
	}

	paths := fs.Args()[1:]
	stdinUses := countStdin(fs.Args())
	if len(paths) == 0 {
		stdinUses++
	}
	if stdinUses > 1 {
		return fmt.Errorf("usage: filter and candidates cannot both read stdin")
	}
	f, err := readFilter(fs.Arg(0), filterSizeLimit(*noSizeLimit), in)
	if err != nil {
		return err
	}
	buffered := bufio.NewWriter(out)
	err = eachSelectedInput(paths, in, input, fields, func(item, output []byte) error {
		present := f.Test(item)
		if present != *invert {
			return writeItem(buffered, output, input.NUL)
		}
		return nil
	})
	return flush(buffered, err)
}

func dedupe(args []string, in io.Reader, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("bf dedupe", flag.ExitOnError)
	fs.SetOutput(errOut)
	expected := fs.Uint64("expected-items", 0, "expected number of distinct items")
	rate := fs.Float64("false-positive-rate", 0, "target false-positive rate")
	noSizeLimit := addSizeLimitFlag(fs)
	var input prob.InputOptions
	prob.AddInputFlags(fs, &input)
	var fields prob.FieldOptions
	prob.AddFieldFlags(fs, &fields)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: bf dedupe --expected-items <n> --false-positive-rate <p> [--delimiter value --field n] [--no-size-limit] [file...]

Emit the first probably unseen occurrence of each value. False positives can
discard values that have not appeared before.
With field selection, deduplicate by that field and emit the complete record.

Options:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := fields.Validate(); err != nil {
		return fmt.Errorf("usage: %w", err)
	}
	f, err := bf.NewWithLimit(*expected, *rate, filterSizeLimit(*noSizeLimit))
	if err != nil {
		return err
	}
	buffered := bufio.NewWriter(out)
	err = eachSelectedInput(fs.Args(), in, input, fields, func(item, output []byte) error {
		if f.Test(item) {
			return nil
		}
		f.Add(item)
		return writeItem(buffered, output, input.NUL)
	})
	if err = flush(buffered, err); err != nil {
		return err
	}
	return warnIfOverfilled(errOut, f)
}

func inspect(args []string, in io.Reader, out io.Writer) error {
	fs := flag.NewFlagSet("bf inspect", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "write JSON output")
	noSizeLimit := addSizeLimitFlag(fs)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: bf inspect [--json] [--no-size-limit] <filter.bf>

Print Bloom filter sizing and health metadata.

Options:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: bf inspect [--json] [--no-size-limit] <filter>")
	}

	m, err := inspectFilter(fs.Arg(0), filterSizeLimit(*noSizeLimit), in)
	if err != nil {
		return err
	}
	if *jsonOut {
		return json.NewEncoder(out).Encode(m)
	}
	fmt.Fprintf(out, "type=%s\n", m.Type)
	fmt.Fprintf(out, "version=%d\n", m.Version)
	fmt.Fprintf(out, "expected_items=%d\n", m.ExpectedItems)
	fmt.Fprintf(out, "inserted_items=%d\n", m.InsertedItems)
	fmt.Fprintf(out, "false_positive_rate=%g\n", m.FalsePositiveRate)
	fmt.Fprintf(out, "estimated_false_positive_rate=%g\n", m.EstimatedFalsePositiveRate)
	fmt.Fprintf(out, "bit_count=%d\n", m.BitCount)
	fmt.Fprintf(out, "bitset_bytes=%d\n", m.BitsetBytes)
	fmt.Fprintf(out, "set_bits=%d\n", m.SetBits)
	fmt.Fprintf(out, "fill_ratio=%g\n", m.FillRatio)
	fmt.Fprintf(out, "hash_count=%d\n", m.HashCount)
	fmt.Fprintf(out, "hash=%s\n", m.Hash)
	return nil
}

func union(args []string, in io.Reader, out io.Writer) error {
	fs := flag.NewFlagSet("bf union", flag.ExitOnError)
	noSizeLimit := addSizeLimitFlag(fs)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: bf union [--no-size-limit] <filter.bf> <filter.bf>... > combined.bf

Union compatible Bloom filters and write a binary filter to stdout.
Filters must have compatible bit count, hash count, false-positive rate, version, and hash metadata.

Options:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return fmt.Errorf("usage: bf union [--no-size-limit] <filter> <filter>...")
	}
	if countStdin(fs.Args()) > 1 {
		return fmt.Errorf("usage: bf union accepts stdin only once")
	}

	limit := filterSizeLimit(*noSizeLimit)
	merged, err := readFilter(fs.Arg(0), limit, in)
	if err != nil {
		return err
	}
	for _, path := range fs.Args()[1:] {
		if err := unionFilter(merged, path, limit, in); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return bf.Write(out, merged)
}

func addSizeLimitFlag(fs *flag.FlagSet) *bool {
	return fs.Bool("no-size-limit", false, "allow filter bitsets larger than 2 GiB")
}

func filterSizeLimit(disabled bool) uint64 {
	if disabled {
		return 0
	}
	return bf.DefaultMaxBytes
}

func readFilter(path string, maxBytes uint64, stdin io.Reader) (*bf.Filter, error) {
	r, close, err := openInput(path, stdin)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer close()
	return bf.ReadWithLimit(r, maxBytes)
}

func inspectFilter(path string, maxBytes uint64, stdin io.Reader) (bf.Metadata, error) {
	r, close, err := openInput(path, stdin)
	if err != nil {
		return bf.Metadata{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer close()
	return bf.InspectWithLimit(r, maxBytes)
}

func unionFilter(dst *bf.Filter, path string, maxBytes uint64, stdin io.Reader) error {
	r, close, err := openInput(path, stdin)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer close()
	return dst.UnionFrom(r, maxBytes)
}

func openInput(path string, stdin io.Reader) (io.Reader, func(), error) {
	if path == "-" {
		return stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { _ = f.Close() }, nil
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

func eachSelectedInput(paths []string, in io.Reader, input prob.InputOptions, fields prob.FieldOptions, fn func(item, output []byte) error) error {
	rawInput := prob.InputOptions{NUL: input.NUL}
	return prob.EachInputFrom(paths, in, rawInput, func(record []byte) error {
		item, err := fields.Select(record)
		if err != nil {
			return err
		}
		if input.Trim {
			item = bytes.TrimSpace(item)
		}
		if input.IgnoreEmpty && len(item) == 0 {
			return nil
		}
		output := item
		if fields.Enabled() {
			output = record
		}
		return fn(item, output)
	})
}

func writeItem(out *bufio.Writer, item []byte, nul bool) error {
	if _, err := out.Write(item); err != nil {
		return err
	}
	if nul {
		return out.WriteByte(0)
	}
	return out.WriteByte('\n')
}

func flush(out *bufio.Writer, err error) error {
	if flushErr := out.Flush(); err == nil {
		return flushErr
	}
	return err
}

func warnIfOverfilled(out io.Writer, f *bf.Filter) error {
	if f.InsertedItems <= f.ExpectedItems {
		return nil
	}
	_, err := fmt.Fprintf(out, "bf: warning: inserted items (%d) exceed expected items (%d); false-positive rate may be higher than requested\n", f.InsertedItems, f.ExpectedItems)
	return err
}
