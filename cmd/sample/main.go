package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jo-cube/toolbox/internal/buildinfo"
	"github.com/jo-cube/toolbox/internal/prob"
	"github.com/jo-cube/toolbox/internal/sample"
)

func main() {
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "print version information")
	flag.BoolVar(&showVersion, "V", false, "print version information")
	rate := flag.Float64("rate", 0, "sample probability, 0..1")
	count := flag.Int("count", 0, "reservoir sample size")
	stable := flag.Bool("stable", false, "use deterministic hash sampling with --rate")
	invert := flag.Bool("invert", false, "emit records excluded by the rate sample")
	seed := flag.Int64("seed", 0, "random or stable hash seed")
	nul := flag.Bool("nul", false, "read and write NUL-delimited records")
	flag.BoolVar(nul, "0", false, "read and write NUL-delimited records")
	var fields prob.FieldOptions
	prob.AddFieldFlags(flag.CommandLine, &fields)

	flag.Usage = func() {
		name := filepath.Base(os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), `Usage: %s (--rate <p> [--stable] [--invert] | --count <n>) [file...]

Emit a subset of input records while preserving emitted records exactly.
Set exactly one of --rate or --count.

Examples:
  sample --rate 0.01 events.jsonl
  sample --rate 0.01 --stable events.jsonl
  sample --rate 0.01 --stable -d $'\t' -f 2 events.tsv
  sample --count 10000 huge-file.txt

Notes:
  --rate samples each record independently unless --stable is set.
  --stable hashes the full record or a selected field without its trailing delimiter.
  --invert emits the complementary rate sample; use --stable or the same --seed to repeat a split.
  -0 and --nul preserve NUL-delimited records instead of newline-delimited records.
  --count uses reservoir sampling and writes selected records after reading input.

Options:
`, name)
		flag.PrintDefaults()
	}
	flag.Parse()

	if showVersion {
		if len(os.Args) != 2 || (os.Args[1] != "--version" && os.Args[1] != "-V") {
			flag.Usage()
			os.Exit(2)
		}
		fmt.Fprintf(os.Stdout, "sample %s\n", buildinfo.Version())
		return
	}

	cfg := sample.Config{
		Rate:     *rate,
		RateSet:  flagWasSet("rate"),
		Count:    *count,
		CountSet: flagWasSet("count"),
		Stable:   *stable,
		Invert:   *invert,
		Seed:     *seed,
		SeedSet:  flagWasSet("seed"),
		NUL:      *nul,
		Fields:   fields,
	}
	if err := sample.Validate(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "sample: %v\n", err)
		os.Exit(2)
	}
	if err := sample.Run(flag.Args(), cfg, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "sample: %v\n", err)
		os.Exit(1)
	}
}

func flagWasSet(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
