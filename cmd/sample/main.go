package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jo-cube/toolbox/internal/buildinfo"
	"github.com/jo-cube/toolbox/internal/sample"
)

func main() {
	showVersion := flag.Bool("version", false, "print version information")
	rate := flag.Float64("rate", 0, "sample probability, 0..1")
	count := flag.Int("count", 0, "reservoir sample size")
	stable := flag.Bool("stable", false, "use deterministic hash sampling with --rate")
	seed := flag.Int64("seed", 0, "random or stable hash seed")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s (--rate <p> [--stable] | --count <n>) [file...]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Fprintf(os.Stdout, "sample %s\n", buildinfo.Version())
		return
	}

	cfg := sample.Config{
		Rate:    *rate,
		RateSet: flagWasSet("rate"),
		Count:   *count,
		Stable:  *stable,
		Seed:    *seed,
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
