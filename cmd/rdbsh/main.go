package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jo-cube/toolbox/internal/buildinfo"
	"github.com/jo-cube/toolbox/internal/rdbsh"
)

func main() {
	showVersion := flag.Bool("version", false, "print version information")
	dbPath := flag.String("db", "", "path to RocksDB directory [required]")
	writable := flag.Bool("writable", false, "open the database in read-write mode")
	columnFamily := flag.String("cf", "", "column family to operate on (default: default)")
	execCommand := flag.String("exec", "", "run a single command and exit")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s --db <path> [options]\n\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Inspect a local RocksDB database interactively or by running a single command.")
		fmt.Fprintln(flag.CommandLine.Output())
		fmt.Fprintln(flag.CommandLine.Output(), "Examples:")
		fmt.Fprintln(flag.CommandLine.Output(), "  rdbsh --db /tmp/offsetstorage")
		fmt.Fprintln(flag.CommandLine.Output(), "  rdbsh --db /tmp/offsetstorage --writable")
		fmt.Fprintln(flag.CommandLine.Output(), "  rdbsh --db /tmp/offsetstorage --cf offsets --exec \"count 0x00\"")
		fmt.Fprintln(flag.CommandLine.Output())
		flag.PrintDefaults()
	}

	flag.Parse()

	if *showVersion {
		fmt.Fprintf(os.Stdout, "rdbsh %s\n", buildinfo.Version())
		return
	}

	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}

	if *dbPath == "" {
		fmt.Fprintln(os.Stderr, "rdbsh: missing required flag: --db")
		fmt.Fprintln(os.Stderr)
		flag.Usage()
		os.Exit(2)
	}

	info, err := os.Stat(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rdbsh: cannot access %q: %v\n", *dbPath, err)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "rdbsh: %q is not a directory\n", *dbPath)
		os.Exit(1)
	}

	shell, err := rdbsh.NewShell(rdbsh.Config{
		DBPath:       *dbPath,
		Writable:     *writable,
		ColumnFamily: strings.TrimSpace(*columnFamily),
		ExecCommand:  *execCommand,
		In:           os.Stdin,
		Out:          os.Stdout,
		ErrOut:       os.Stderr,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "rdbsh: %v\n", err)
		os.Exit(1)
	}
	defer shell.Close()

	if strings.TrimSpace(*execCommand) != "" {
		if err := shell.Exec(*execCommand); err != nil {
			fmt.Fprintf(os.Stderr, "rdbsh: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := shell.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "rdbsh: %v\n", err)
		os.Exit(1)
	}
}
