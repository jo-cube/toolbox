package main

import (
	"encoding/json"
	"flag"
	"fmt"
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
		err = count(os.Args[2:])
	case "build":
		err = build(os.Args[2:])
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
		if strings.HasPrefix(err.Error(), "usage:") {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: hll <count|build|estimate|merge|inspect> [options]\n")
}

func count(args []string) error {
	fs := flag.NewFlagSet("hll count", flag.ExitOnError)
	precision := fs.Uint("precision", uint(hll.DefaultP), "HLL precision, 4..20")
	jsonOut := fs.Bool("json", false, "write JSON output")
	var input prob.InputOptions
	prob.AddInputFlags(fs, &input)
	if err := fs.Parse(args); err != nil {
		return err
	}

	s, err := hll.New(uint8(*precision))
	if err != nil {
		return err
	}
	if err := prob.EachInput(fs.Args(), input, func(item []byte) error {
		s.Add(item)
		return nil
	}); err != nil {
		return err
	}
	return writeEstimate(os.Stdout, s, *jsonOut)
}

func build(args []string) error {
	fs := flag.NewFlagSet("hll build", flag.ExitOnError)
	precision := fs.Uint("precision", uint(hll.DefaultP), "HLL precision, 4..20")
	var input prob.InputOptions
	prob.AddInputFlags(fs, &input)
	if err := fs.Parse(args); err != nil {
		return err
	}

	s, err := hll.New(uint8(*precision))
	if err != nil {
		return err
	}
	if err := prob.EachInput(fs.Args(), input, func(item []byte) error {
		s.Add(item)
		return nil
	}); err != nil {
		return err
	}
	return hll.Write(os.Stdout, s)
}

func estimate(args []string) error {
	fs := flag.NewFlagSet("hll estimate", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "write JSON output")
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
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return fmt.Errorf("usage: hll merge <file> <file>...")
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
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	return hll.Read(f)
}

func writeEstimate(out *os.File, s *hll.Sketch, jsonOut bool) error {
	m := s.Metadata()
	if jsonOut {
		return json.NewEncoder(out).Encode(struct {
			ApproxUnique  uint64  `json:"approx_unique"`
			RelativeError float64 `json:"relative_error"`
		}{m.ApproxUnique, m.RelativeError})
	}
	fmt.Fprintf(out, "approx_unique=%d\n", m.ApproxUnique)
	fmt.Fprintf(out, "relative_error=%.2f%%\n", m.RelativeError*100)
	return nil
}
