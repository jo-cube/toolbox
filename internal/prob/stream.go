package prob

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
)

type InputOptions struct {
	NUL         bool
	Trim        bool
	IgnoreEmpty bool
	Fields      FieldOptions
}

func EachInput(paths []string, opts InputOptions, fn func([]byte) error) error {
	return EachInputFrom(paths, os.Stdin, opts, fn)
}

// EachInputFrom reads stdin for an empty path list or an explicit "-" path.
func EachInputFrom(paths []string, stdin io.Reader, opts InputOptions, fn func([]byte) error) error {
	return EachSelectedInputFrom(paths, stdin, opts, func(item, _ []byte) error {
		return fn(item)
	})
}

// EachSelectedInputFrom supplies the selected value and the output record.
// With field selection, output retains the complete record without its delimiter.
func EachSelectedInputFrom(paths []string, stdin io.Reader, opts InputOptions, fn func(item, output []byte) error) error {
	if err := opts.Fields.Validate(); err != nil {
		return err
	}
	return EachRecordFrom(paths, stdin, opts.NUL, func(record []byte) error {
		delim := byte('\n')
		if opts.NUL {
			delim = 0
		}
		if record[len(record)-1] == delim {
			record = record[:len(record)-1]
			if !opts.NUL && len(record) > 0 && record[len(record)-1] == '\r' {
				record = record[:len(record)-1]
			}
		}
		item, err := opts.Fields.Select(record)
		if err != nil {
			return err
		}
		if opts.Trim {
			item = bytes.TrimSpace(item)
		}
		if opts.IgnoreEmpty && len(item) == 0 {
			return nil
		}
		output := item
		if opts.Fields.Enabled() {
			output = record
		}
		return fn(item, output)
	})
}

// EachFile visits files in order, using stdin for no paths or one "-" path.
func EachFile(paths []string, stdin io.Reader, fn func(string, io.Reader) error) error {
	if len(paths) == 0 {
		return fn("<stdin>", stdin)
	}
	stdinUsed := false
	for _, path := range paths {
		if path == "-" {
			if stdinUsed {
				return fmt.Errorf("stdin may be read only once")
			}
			stdinUsed = true
		}
	}
	for _, path := range paths {
		if path == "-" {
			if err := fn("<stdin>", stdin); err != nil {
				return err
			}
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", path, err)
		}
		err = fn(path, f)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return fmt.Errorf("close %s: %w", path, closeErr)
		}
	}
	return nil
}

// EachRecordFrom preserves record bytes, including any LF or NUL delimiter.
// Callback slices are valid only until the callback returns.
func EachRecordFrom(paths []string, stdin io.Reader, nul bool, fn func([]byte) error) error {
	delim := byte('\n')
	if nul {
		delim = 0
	}
	return EachFile(paths, stdin, func(name string, r io.Reader) error {
		br := bufio.NewReader(r)
		var continued []byte
		for {
			record, err := br.ReadSlice(delim)
			if err == bufio.ErrBufferFull {
				continued = append(continued, record...)
				continue
			}
			if len(continued) != 0 {
				continued = append(continued, record...)
				record = continued
			}
			if len(record) > 0 {
				if err := fn(record); err != nil {
					return err
				}
			}
			continued = continued[:0]
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("read %s: %w", name, err)
			}
		}
	})
}
