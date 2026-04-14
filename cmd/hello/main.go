package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jo-cube/toolbox/internal/buildinfo"
	"github.com/jo-cube/toolbox/internal/hello"
)

func main() {
	showVersion := flag.Bool("version", false, "print version information")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [--version]\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output())
		fmt.Fprintln(flag.CommandLine.Output(), "Prints a friendly greeting.")
		fmt.Fprintln(flag.CommandLine.Output())
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
