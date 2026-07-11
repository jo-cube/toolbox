package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jo-cube/toolbox/internal/buildinfo"
	"github.com/jo-cube/toolbox/internal/rdbsh"
)

func main() {
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "print version information")
	flag.BoolVar(&showVersion, "V", false, "print version information")
	dbPath := flag.String("db", "", "path to RocksDB directory [required]")
	writable := flag.Bool("writable", false, "open the database in read-write mode")
	columnFamily := flag.String("cf", "", "column family to operate on (default: default)")
	execCommand := flag.String("exec", "", "run a single command and exit")
	force := flag.Bool("force", false, "allow export to overwrite an existing file")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `Usage: %s --db <path> [options]

Inspect a local RocksDB database interactively or by running a single command.
Databases are opened read-only unless --writable is set.

Examples:
  rdbsh --db /tmp/offsetstorage
  rdbsh --db /tmp/offsetstorage --exec "get 0x00000001"
  rdbsh --db /tmp/offsetstorage --cf offsets --exec "count 0x00"
  rdbsh --db /tmp/offsetstorage --exec "export - json 0x00"

Shell commands:
  get, put, delete, scan, keys, count, stats, export, cfs, help, exit

Notes:
  put and delete require --writable.
  export <file> refuses to overwrite unless --force is set.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	flag.Parse()

	if showVersion {
		if len(os.Args) != 2 || (os.Args[1] != "--version" && os.Args[1] != "-V") {
			flag.Usage()
			os.Exit(2)
		}
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
		Force:        *force,
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
