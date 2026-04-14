package rdbsh

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
)

func (s *Shell) registerCommands() {
	s.commands = map[string]command{
		"get": {
			help: "get <key>                      Read a key's value",
			fn:   s.cmdGet,
		},
		"put": {
			help: "put <key> <value>              Write a key-value pair (requires --writable)",
			fn:   s.cmdPut,
		},
		"delete": {
			help: "delete <key>                   Delete a key (requires --writable)",
			fn:   s.cmdDelete,
		},
		"scan": {
			help: "scan [prefix] [limit]          Scan key-value pairs (default limit 100)",
			fn:   s.cmdScan,
		},
		"keys": {
			help: "keys [prefix] [limit]          List keys only (default limit 100)",
			fn:   s.cmdKeys,
		},
		"count": {
			help: "count [prefix]                 Count keys (optional prefix filter)",
			fn:   s.cmdCount,
		},
		"stats": {
			help: "stats                          Show RocksDB statistics and properties",
			fn:   s.cmdStats,
		},
		"export": {
			help: "export <file|-> [csv|json] [prefix]  Export key-values to a file or stdout",
			fn:   s.cmdExport,
		},
		"cfs": {
			help: "cfs                            List available column families",
			fn:   s.cmdCFS,
		},
		"help": {
			help: "help                           Show this help message",
			fn:   s.cmdHelp,
		},
	}
}

func (s *Shell) cmdGet(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: get <key>")
	}

	key, err := parseInput(args[0])
	if err != nil {
		return err
	}

	value, found, err := s.get(key)
	if err != nil {
		return err
	}
	if !found {
		fmt.Fprintln(s.out, "(not found)")
		return nil
	}

	fmt.Fprintln(s.out, formatBytes(value))
	if !isPrintable(value) {
		fmt.Fprintf(s.out, "(%d bytes)\n", len(value))
	}
	return nil
}

func (s *Shell) cmdPut(args []string) error {
	if !s.config.Writable {
		return fmt.Errorf("database opened in read-only mode (use --writable)")
	}
	if len(args) < 2 {
		return fmt.Errorf("usage: put <key> <value>")
	}

	key, err := parseInput(args[0])
	if err != nil {
		return err
	}
	value, err := parseInput(strings.Join(args[1:], " "))
	if err != nil {
		return err
	}

	if err := s.put(key, value); err != nil {
		return err
	}

	fmt.Fprintln(s.out, "OK")
	return nil
}

func (s *Shell) cmdDelete(args []string) error {
	if !s.config.Writable {
		return fmt.Errorf("database opened in read-only mode (use --writable)")
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: delete <key>")
	}

	key, err := parseInput(args[0])
	if err != nil {
		return err
	}
	if err := s.delete(key); err != nil {
		return err
	}

	fmt.Fprintln(s.out, "OK")
	return nil
}

func (s *Shell) cmdScan(args []string) error {
	prefix, limit, err := parsePrefixAndLimit(args, "usage: scan [prefix] [limit]")
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(s.out, 0, 0, 2, ' ', 0)
	result, err := s.iterate(prefix, limit, func(key, value []byte) error {
		_, err := fmt.Fprintf(w, "%s\t%s\n", formatBytes(key), formatBytes(value))
		return err
	})
	if err != nil {
		return err
	}
	if result.Count == 0 {
		fmt.Fprintln(s.out, "(no results)")
		return nil
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if result.Limited {
		fmt.Fprintf(s.out, "... (limit %d reached)\n", limit)
	}
	return nil
}

func (s *Shell) cmdKeys(args []string) error {
	prefix, limit, err := parsePrefixAndLimit(args, "usage: keys [prefix] [limit]")
	if err != nil {
		return err
	}

	result, err := s.iterate(prefix, limit, func(key, _ []byte) error {
		_, err := fmt.Fprintln(s.out, formatBytes(key))
		return err
	})
	if err != nil {
		return err
	}
	if result.Count == 0 {
		fmt.Fprintln(s.out, "(no keys found)")
		return nil
	}
	if result.Limited {
		fmt.Fprintf(s.out, "... (limit %d reached)\n", limit)
	}
	fmt.Fprintf(s.out, "(%d keys shown)\n", result.Count)
	return nil
}

func (s *Shell) cmdCount(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: count [prefix]")
	}

	var prefix []byte
	if len(args) == 1 {
		var err error
		prefix, err = parseInput(args[0])
		if err != nil {
			return err
		}
	}

	result, err := s.iterate(prefix, 0, func(_, _ []byte) error { return nil })
	if err != nil {
		return err
	}

	if len(args) == 1 {
		fmt.Fprintf(s.out, "%d keys (prefix: %s)\n", result.Count, formatBytes(prefix))
	} else {
		fmt.Fprintf(s.out, "%d keys total\n", result.Count)
	}
	return nil
}

func (s *Shell) cmdStats(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: stats")
	}

	properties := []string{
		"rocksdb.stats",
		"rocksdb.sstables",
		"rocksdb.levelstats",
		"rocksdb.num-files-at-level0",
		"rocksdb.num-files-at-level1",
		"rocksdb.num-files-at-level2",
		"rocksdb.estimate-num-keys",
		"rocksdb.estimate-live-data-size",
		"rocksdb.total-sst-files-size",
		"rocksdb.live-sst-files-size",
		"rocksdb.estimate-table-readers-mem",
		"rocksdb.block-cache-usage",
		"rocksdb.cur-size-all-mem-tables",
	}

	fmt.Fprintf(s.out, "RocksDB properties (column family: %s)\n", s.selectedCF)
	w := tabwriter.NewWriter(s.out, 0, 0, 2, ' ', 0)
	for _, property := range properties {
		value := s.property(property)
		if value == "" {
			continue
		}
		if strings.Contains(value, "\n") {
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Fprintf(s.out, "\n[%s]\n%s\n", property, value)
			continue
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\n", property, value); err != nil {
			return err
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return nil
}

func (s *Shell) cmdCFS(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: cfs")
	}

	if len(s.availableCFs) == 0 {
		fmt.Fprintln(s.out, "(no column families found)")
		return nil
	}

	for _, name := range s.availableCFs {
		marker := " "
		if name == s.selectedCF {
			marker = "*"
		}
		fmt.Fprintf(s.out, "%s %s\n", marker, name)
	}
	return nil
}

func (s *Shell) cmdHelp(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: help")
	}

	fmt.Fprintln(s.out, "Available commands:")
	fmt.Fprintln(s.out)

	order := []string{"get", "put", "delete", "scan", "keys", "count", "stats", "export", "cfs", "help"}
	for _, name := range order {
		fmt.Fprintf(s.out, "  %s\n", s.commands[name].help)
	}

	fmt.Fprintln(s.out)
	fmt.Fprintln(s.out, "Input rules:")
	fmt.Fprintln(s.out, "  - Prefix keys, values, or prefixes with 0x for raw hex bytes")
	fmt.Fprintln(s.out, "  - Use double quotes for spaces inside arguments")
	fmt.Fprintln(s.out, "  - Use \\ to escape spaces or quotes")
	return nil
}

func parsePrefixAndLimit(args []string, usage string) ([]byte, int, error) {
	if len(args) > 2 {
		return nil, 0, errors.New(usage)
	}

	limit := 100
	if len(args) == 0 {
		return nil, limit, nil
	}

	prefix, err := parseInput(args[0])
	if err != nil {
		return nil, 0, err
	}
	if len(args) == 1 {
		return prefix, limit, nil
	}

	limit, err = strconv.Atoi(args[1])
	if err != nil || limit <= 0 {
		return nil, 0, fmt.Errorf("limit must be a positive integer")
	}
	return prefix, limit, nil
}
