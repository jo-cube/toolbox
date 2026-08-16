package sample

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
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
}

func Validate(cfg Config) error {
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
	return nil
}

func Run(paths []string, cfg Config, out io.Writer) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	if cfg.RateSet || cfg.Rate > 0 {
		if cfg.Stable {
			return rateStable(paths, cfg, out)
		}
		return rateRandom(paths, cfg, out)
	}
	return reservoir(paths, cfg, out)
}

func rateRandom(paths []string, cfg Config, out io.Writer) error {
	rng := rand.New(rand.NewSource(seed(cfg)))
	return eachRaw(paths, func(record []byte) error {
		if rng.Float64() < cfg.Rate {
			_, err := out.Write(record)
			return err
		}
		return nil
	})
}

func rateStable(paths []string, cfg Config, out io.Writer) error {
	threshold := uint64(cfg.Rate * float64(math.MaxUint64))
	return eachRaw(paths, func(record []byte) error {
		key := record
		if len(key) > 0 && key[len(key)-1] == '\n' {
			key = key[:len(key)-1]
			if len(key) > 0 && key[len(key)-1] == '\r' {
				key = key[:len(key)-1]
			}
		}
		if cfg.Rate >= 1 || prob.Hash64(key, uint64(cfg.Seed)) < threshold {
			_, err := out.Write(record)
			return err
		}
		return nil
	})
}

func reservoir(paths []string, cfg Config, out io.Writer) error {
	rng := rand.New(rand.NewSource(seed(cfg)))
	var items [][]byte
	var seen int64

	if err := eachRaw(paths, func(record []byte) error {
		seen++
		if len(items) < cfg.Count {
			items = append(items, append([]byte(nil), record...))
			return nil
		}
		j := rng.Int63n(seen)
		if j < int64(cfg.Count) {
			items[j] = append(items[j][:0], record...)
		}
		return nil
	}); err != nil {
		return err
	}

	for _, item := range items {
		if _, err := out.Write(item); err != nil {
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

func eachRaw(paths []string, fn func([]byte) error) error {
	if len(paths) == 0 {
		return eachRawReader("<stdin>", os.Stdin, fn)
	}
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", path, err)
		}
		err = eachRawReader(path, f, fn)
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

func eachRawReader(name string, r io.Reader, fn func([]byte) error) error {
	br := bufio.NewReader(r)
	for {
		record, err := br.ReadBytes('\n')
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
