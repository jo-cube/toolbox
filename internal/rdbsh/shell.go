package rdbsh

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/jo-cube/toolbox/internal/rdbsh/rocksdb"
)

type Shell struct {
	db             *rocksdb.DB
	ro             *rocksdb.ReadOptions
	wo             *rocksdb.WriteOptions
	config         Config
	commands       map[string]command
	in             io.Reader
	out            io.Writer
	errOut         io.Writer
	availableCFs   []string
	selectedCF     string
	selectedHandle *rocksdb.ColumnFamilyHandle
}

type command struct {
	help string
	fn   func(args []string) error
}

func NewShell(cfg Config) (*Shell, error) {
	if cfg.In == nil {
		cfg.In = os.Stdin
	}
	if cfg.Out == nil {
		cfg.Out = os.Stdout
	}
	if cfg.ErrOut == nil {
		cfg.ErrOut = os.Stderr
	}

	availableCFs, err := rocksdb.ListColumnFamilies(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("list column families: %w", err)
	}

	var db *rocksdb.DB
	var selectedHandle *rocksdb.ColumnFamilyHandle
	selectedCF := cfg.ColumnFamily
	if selectedCF == "" {
		selectedCF = "default"
	}

	if cfg.ColumnFamily == "" {
		if cfg.Writable {
			db, err = rocksdb.Open(cfg.DBPath)
		} else {
			db, err = rocksdb.OpenReadOnly(cfg.DBPath)
		}
	} else {
		db, err = rocksdb.OpenWithColumnFamilies(cfg.DBPath, availableCFs, !cfg.Writable)
		if err == nil {
			var ok bool
			selectedHandle, ok = db.ColumnFamily(cfg.ColumnFamily)
			if !ok {
				db.Close()
				return nil, fmt.Errorf("column family %q not found", cfg.ColumnFamily)
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("open rocksdb at %s: %w", cfg.DBPath, err)
	}

	s := &Shell{
		db:             db,
		ro:             rocksdb.NewDefaultReadOptions(),
		wo:             rocksdb.NewDefaultWriteOptions(),
		config:         cfg,
		in:             cfg.In,
		out:            cfg.Out,
		errOut:         cfg.ErrOut,
		availableCFs:   append([]string(nil), availableCFs...),
		selectedCF:     selectedCF,
		selectedHandle: selectedHandle,
	}
	s.registerCommands()
	return s, nil
}

func (s *Shell) Close() {
	if s.ro != nil {
		s.ro.Destroy()
	}
	if s.wo != nil {
		s.wo.Destroy()
	}
	if s.db != nil {
		s.db.Close()
	}
}

func (s *Shell) Run() error {
	mode := "read-only"
	if s.config.Writable {
		mode = "read-write"
	}

	fmt.Fprintf(s.out, "rdbsh connected to %s (%s, column family: %s)\n", s.config.DBPath, mode, s.selectedCF)
	fmt.Fprintln(s.out, "Type 'help' for available commands, 'exit' to quit.")
	fmt.Fprintln(s.out)

	scanner := bufio.NewScanner(s.in)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for {
		fmt.Fprint(s.out, "rdbsh> ")
		if !scanner.Scan() {
			fmt.Fprintln(s.out)
			return scanner.Err()
		}

		exit, err := s.executeLine(scanner.Text())
		if err != nil {
			fmt.Fprintf(s.errOut, "error: %v\n", err)
			continue
		}
		if exit {
			fmt.Fprintln(s.out, "Bye!")
			return nil
		}
	}
}

func (s *Shell) Exec(line string) error {
	_, err := s.executeLine(line)
	return err
}

func splitArgs(line string) ([]string, error) {
	var args []string
	var current strings.Builder
	var inQuote bool
	var escaped bool
	var tokenStarted bool

	for _, r := range line {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
			tokenStarted = true
		case r == '\\':
			escaped = true
			tokenStarted = true
		case r == '"':
			inQuote = !inQuote
			tokenStarted = true
		case unicode.IsSpace(r) && !inQuote:
			if tokenStarted {
				args = append(args, current.String())
				current.Reset()
				tokenStarted = false
			}
		default:
			current.WriteRune(r)
			tokenStarted = true
		}
	}

	if escaped {
		return nil, fmt.Errorf("unfinished escape sequence")
	}
	if inQuote {
		return nil, fmt.Errorf("unterminated quoted string")
	}
	if tokenStarted {
		args = append(args, current.String())
	}
	return args, nil
}

func (s *Shell) executeLine(line string) (bool, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return false, nil
	}

	parts, err := splitArgs(line)
	if err != nil {
		return false, err
	}
	if len(parts) == 0 {
		return false, nil
	}

	name := strings.ToLower(parts[0])
	if name == "exit" || name == "quit" {
		return true, nil
	}

	cmd, ok := s.commands[name]
	if !ok {
		return false, fmt.Errorf("unknown command: %s (type 'help' for available commands)", name)
	}

	return false, cmd.fn(parts[1:])
}

func (s *Shell) get(key []byte) ([]byte, bool, error) {
	if s.selectedHandle != nil {
		return s.db.GetCF(s.ro, s.selectedHandle, key)
	}
	return s.db.Get(s.ro, key)
}

func (s *Shell) put(key, value []byte) error {
	if s.selectedHandle != nil {
		return s.db.PutCF(s.wo, s.selectedHandle, key, value)
	}
	return s.db.Put(s.wo, key, value)
}

func (s *Shell) delete(key []byte) error {
	if s.selectedHandle != nil {
		return s.db.DeleteCF(s.wo, s.selectedHandle, key)
	}
	return s.db.Delete(s.wo, key)
}

func (s *Shell) property(name string) string {
	if s.selectedHandle != nil {
		return s.db.GetPropertyCF(s.selectedHandle, name)
	}
	return s.db.GetProperty(name)
}
