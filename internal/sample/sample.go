package sample

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"sort"
	"time"

	"github.com/jo-cube/toolbox/internal/prob"
)

type Config struct {
	Rate     float64
	RateSet  bool
	Count    int
	CountSet bool
	Stable   bool
	Seed     int64
	SeedSet  bool
	NUL      bool
	Fields   prob.FieldOptions
}

func Validate(cfg Config) error {
	if err := cfg.Fields.Validate(); err != nil {
		return err
	}
	hasRate := cfg.RateSet || cfg.Rate != 0
	hasCount := cfg.CountSet || cfg.Count != 0
	if hasRate == hasCount {
		return fmt.Errorf("set exactly one of --rate or --count")
	}
	if math.IsNaN(cfg.Rate) || math.IsInf(cfg.Rate, 0) || cfg.Rate < 0 || cfg.Rate > 1 {
		return fmt.Errorf("rate must be between 0 and 1")
	}
	if hasCount && cfg.Count <= 0 {
		return fmt.Errorf("count must be a positive integer")
	}
	if cfg.Stable && hasCount {
		return fmt.Errorf("--stable can only be used with --rate")
	}
	if cfg.Fields.Enabled() && !cfg.Stable {
		return fmt.Errorf("--delimiter and --field can only be used with --stable")
	}
	return nil
}

func Run(paths []string, cfg Config, out io.Writer) error {
	return RunFrom(paths, cfg, out, os.Stdin)
}

// RunFrom uses stdin for an empty path list or an explicit "-" path.
func RunFrom(paths []string, cfg Config, out io.Writer, stdin io.Reader) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	buffered := bufio.NewWriter(out)
	var err error
	if cfg.RateSet || cfg.Rate > 0 {
		if cfg.Stable {
			err = rateStable(paths, cfg, buffered, stdin)
		} else {
			err = rateRandom(paths, cfg, buffered, stdin)
		}
	} else {
		err = reservoir(paths, cfg, buffered, stdin)
	}
	if flushErr := buffered.Flush(); err == nil {
		return flushErr
	}
	return err
}

func rateRandom(paths []string, cfg Config, out io.Writer, stdin io.Reader) error {
	rng := rand.New(rand.NewSource(seed(cfg)))
	return eachRaw(paths, stdin, delimiter(cfg), func(record []byte) error {
		if rng.Float64() < cfg.Rate {
			_, err := out.Write(record)
			return err
		}
		return nil
	})
}

func rateStable(paths []string, cfg Config, out io.Writer, stdin io.Reader) error {
	threshold := uint64(cfg.Rate * float64(math.MaxUint64))
	delim := delimiter(cfg)
	return eachRaw(paths, stdin, delim, func(record []byte) error {
		key := record
		if len(key) > 0 && key[len(key)-1] == delim {
			key = key[:len(key)-1]
			if delim == '\n' && len(key) > 0 && key[len(key)-1] == '\r' {
				key = key[:len(key)-1]
			}
		}
		key, err := cfg.Fields.Select(key)
		if err != nil {
			return err
		}
		if cfg.Rate >= 1 || prob.Hash64(key, uint64(cfg.Seed)) < threshold {
			_, err := out.Write(record)
			return err
		}
		return nil
	})
}

func reservoir(paths []string, cfg Config, out io.Writer, stdin io.Reader) error {
	rng := rand.New(rand.NewSource(seed(cfg)))
	type selected struct {
		record []byte
		order  int64
	}
	var items []selected
	var seen int64

	if err := eachRaw(paths, stdin, delimiter(cfg), func(record []byte) error {
		seen++
		if len(items) < cfg.Count {
			items = append(items, selected{record: append([]byte(nil), record...), order: seen})
			return nil
		}
		j := rng.Int63n(seen)
		if j < int64(cfg.Count) {
			items[j].record = append(items[j].record[:0], record...)
			items[j].order = seen
		}
		return nil
	}); err != nil {
		return err
	}

	sort.Slice(items, func(i, j int) bool { return items[i].order < items[j].order })
	for _, item := range items {
		if _, err := out.Write(item.record); err != nil {
			return err
		}
	}
	return nil
}

func seed(cfg Config) int64 {
	if cfg.SeedSet || cfg.Seed != 0 {
		return cfg.Seed
	}
	return time.Now().UnixNano()
}

func delimiter(cfg Config) byte {
	if cfg.NUL {
		return 0
	}
	return '\n'
}

func eachRaw(paths []string, stdin io.Reader, delim byte, fn func([]byte) error) error {
	if len(paths) == 0 {
		return eachRawReader("<stdin>", stdin, delim, fn)
	}
	stdinUsed := false
	for _, path := range paths {
		if path == "-" {
			if stdinUsed {
				return fmt.Errorf("stdin may be read only once")
			}
			stdinUsed = true
			if err := eachRawReader("<stdin>", stdin, delim, fn); err != nil {
				return err
			}
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", path, err)
		}
		err = eachRawReader(path, f, delim, fn)
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

func eachRawReader(name string, r io.Reader, delim byte, fn func([]byte) error) error {
	br := bufio.NewReader(r)
	var continued []byte
	for {
		record, err := br.ReadSlice(delim)
		if err == bufio.ErrBufferFull {
			continued = append(continued, record...)
			continue
		}
		if len(continued) != 0 {
			record = append(continued, record...)
			continued = nil
		}
		if len(record) > 0 {
			if err := fn(record); err != nil {
				return err
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
