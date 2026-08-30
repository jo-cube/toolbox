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
}

func EachInput(paths []string, opts InputOptions, fn func([]byte) error) error {
	return EachInputFrom(paths, os.Stdin, opts, fn)
}

// EachInputFrom reads stdin for an empty path list or an explicit "-" path.
func EachInputFrom(paths []string, stdin io.Reader, opts InputOptions, fn func([]byte) error) error {
	if len(paths) == 0 {
		return eachReader("<stdin>", stdin, opts, fn)
	}

	stdinUsed := false
	for _, path := range paths {
		if path == "-" {
			if stdinUsed {
				return fmt.Errorf("stdin may be read only once")
			}
			stdinUsed = true
			if err := eachReader("<stdin>", stdin, opts, fn); err != nil {
				return err
			}
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", path, err)
		}
		err = eachReader(path, f, opts, fn)
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

func eachReader(name string, r io.Reader, opts InputOptions, fn func([]byte) error) error {
	delim := byte('\n')
	if opts.NUL {
		delim = 0
	}

	br := bufio.NewReader(r)
	var continued []byte
	for {
		item, err := br.ReadSlice(delim)
		if err == bufio.ErrBufferFull {
			continued = append(continued, item...)
			continue
		}
		if len(continued) != 0 {
			item = append(continued, item...)
			continued = nil
		}
		if len(item) > 0 {
			if item[len(item)-1] == delim {
				item = item[:len(item)-1]
				if !opts.NUL && len(item) > 0 && item[len(item)-1] == '\r' {
					item = item[:len(item)-1]
				}
			}
			if opts.Trim {
				item = bytes.TrimSpace(item)
			}
			if !opts.IgnoreEmpty || len(item) > 0 {
				if err := fn(item); err != nil {
					return err
				}
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
	}
}
