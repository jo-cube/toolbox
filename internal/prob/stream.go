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
	if len(paths) == 0 {
		return eachReader("<stdin>", os.Stdin, opts, fn)
	}

	for _, path := range paths {
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
	for {
		item, err := br.ReadBytes(delim)
		if len(item) > 0 {
			if item[len(item)-1] == delim {
				item = item[:len(item)-1]
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
