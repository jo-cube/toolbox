package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jo-cube/toolbox/internal/buildinfo"
	"github.com/jo-cube/toolbox/internal/hello"
)

func main() {
	showVersion := flag.Bool("version", false, "print version information")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `Usage: %s [--version]

Print a friendly greeting. This is the smallest reference CLI in toolbox.

Examples:
  hello
  hello --version

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Fprintf(os.Stdout, "hello %s\n", buildinfo.Version())
		return
	}

	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}

	if err := hello.Print(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "hello: %v\n", err)
		os.Exit(1)
	}
}
