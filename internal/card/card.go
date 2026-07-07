package card

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/jo-cube/toolbox/internal/hll"
)

type Config struct {
	Mode      string
	Columns   []string
	JSONPaths []string
	Delimiter string
	Precision uint8
}

type Profile struct {
	Field        string `json:"field"`
	ApproxUnique uint64 `json:"approx_unique"`
	Nulls        uint64 `json:"nulls,omitempty"`
	Missing      uint64 `json:"missing,omitempty"`
	Empty        uint64 `json:"empty,omitempty"`
	Total        uint64 `json:"total"`
}

type counter struct {
	field   string
	sketch  *hll.Sketch
	nulls   uint64
	missing uint64
	empty   uint64
	total   uint64
}

func Run(paths []string, cfg Config) ([]Profile, error) {
	if cfg.Precision == 0 {
		cfg.Precision = hll.DefaultP
	}
	switch cfg.Mode {
	case "csv":
		return runCSV(paths, cfg)
	case "json":
		return runJSON(paths, cfg)
	case "delimiter":
		return runDelimited(paths, cfg)
	default:
		return nil, fmt.Errorf("choose one of --csv, --json, or --delimiter")
	}
}

func runCSV(paths []string, cfg Config) ([]Profile, error) {
	if len(cfg.Columns) == 0 {
		return nil, fmt.Errorf("--columns is required in CSV mode")
	}
	counters, err := newCounters(cfg.Columns, cfg.Precision)
	if err != nil {
		return nil, err
	}
	return finish(counters, eachPath(paths, func(name string, r io.Reader) error {
		cr := csv.NewReader(r)
		header, err := cr.Read()
		if err != nil {
			return fmt.Errorf("%s: read header: %w", name, err)
		}
		indexes := make([]int, len(cfg.Columns))
		for i, col := range cfg.Columns {
			indexes[i] = -1
			for j, headerCol := range header {
				if headerCol == col {
					indexes[i] = j
					break
				}
			}
			if indexes[i] == -1 {
				return fmt.Errorf("%s: column %q not found", name, col)
			}
		}
		for {
			record, err := cr.Read()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			for i, idx := range indexes {
				value, ok := recordValue(record, idx)
				addValue(counters[i], value, ok)
			}
		}
	}))
}

func runDelimited(paths []string, cfg Config) ([]Profile, error) {
	if cfg.Delimiter == "" {
		return nil, fmt.Errorf("--delimiter cannot be empty")
	}
	indexes, fields, err := parseColumnIndexes(cfg.Columns)
	if err != nil {
		return nil, err
	}
	counters, err := newCounters(fields, cfg.Precision)
	if err != nil {
		return nil, err
	}
	return finish(counters, eachLine(paths, func(line string) error {
		parts := strings.Split(line, cfg.Delimiter)
		for i, idx := range indexes {
			value, ok := recordValue(parts, idx)
			addValue(counters[i], value, ok)
		}
		return nil
	}))
}

func runJSON(paths []string, cfg Config) ([]Profile, error) {
	if len(cfg.JSONPaths) == 0 {
		return nil, fmt.Errorf("at least one JSON path is required")
	}
	counters, err := newCounters(cfg.JSONPaths, cfg.Precision)
	if err != nil {
		return nil, err
	}
	segments := make([][]string, len(cfg.JSONPaths))
	for i, path := range cfg.JSONPaths {
		segments[i] = parsePath(path)
		if len(segments[i]) == 0 {
			return nil, fmt.Errorf("invalid JSON path %q", path)
		}
	}
	return finish(counters, eachLine(paths, func(line string) error {
		var obj any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			return err
		}
		for i, path := range segments {
			value, ok := lookup(obj, path)
			if !ok {
				counters[i].missing++
				counters[i].total++
				continue
			}
			addAny(counters[i], value)
		}
		return nil
	}))
}

func newCounters(fields []string, precision uint8) ([]*counter, error) {
	counters := make([]*counter, len(fields))
	for i, field := range fields {
		s, err := hll.New(precision)
		if err != nil {
			return nil, err
		}
		counters[i] = &counter{field: field, sketch: s}
	}
	return counters, nil
}

func finish(counters []*counter, err error) ([]Profile, error) {
	if err != nil {
		return nil, err
	}
	profiles := make([]Profile, len(counters))
	for i, c := range counters {
		profiles[i] = Profile{
			Field:        c.field,
			ApproxUnique: c.sketch.Estimate(),
			Nulls:        c.nulls,
			Missing:      c.missing,
			Empty:        c.empty,
			Total:        c.total,
		}
	}
	return profiles, nil
}

func addValue(c *counter, value string, ok bool) {
	c.total++
	if !ok {
		c.missing++
		return
	}
	if value == "" {
		c.empty++
		return
	}
	c.sketch.Add([]byte(value))
}

func addAny(c *counter, value any) {
	c.total++
	if value == nil {
		c.nulls++
		return
	}
	if s, ok := value.(string); ok {
		if s == "" {
			c.empty++
			return
		}
		c.sketch.Add([]byte(s))
		return
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		c.sketch.Add([]byte(fmt.Sprint(value)))
		return
	}
	c.sketch.Add(encoded)
}

func recordValue(record []string, idx int) (string, bool) {
	if idx < 0 || idx >= len(record) {
		return "", false
	}
	return record[idx], true
}

func parseColumnIndexes(columns []string) ([]int, []string, error) {
	if len(columns) == 0 {
		return nil, nil, fmt.Errorf("--columns is required in delimiter mode")
	}
	indexes := make([]int, len(columns))
	fields := make([]string, len(columns))
	for i, col := range columns {
		n, err := strconv.Atoi(col)
		if err != nil || n <= 0 {
			return nil, nil, fmt.Errorf("column %q must be a 1-based integer", col)
		}
		indexes[i] = n - 1
		fields[i] = col
	}
	return indexes, fields, nil
}

func parsePath(path string) []string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return nil
	}
	return strings.Split(path, ".")
}

func lookup(value any, path []string) (any, bool) {
	current := value
	for _, part := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = obj[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func eachPath(paths []string, fn func(string, io.Reader) error) error {
	if len(paths) == 0 {
		return fn("<stdin>", os.Stdin)
	}
	for _, path := range paths {
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

func eachLine(paths []string, fn func(string) error) error {
	return eachPath(paths, func(name string, r io.Reader) error {
		br := bufio.NewReader(r)
		line := 0
		for {
			text, err := br.ReadString('\n')
			if len(text) > 0 {
				text = strings.TrimSuffix(text, "\n")
				text = strings.TrimSuffix(text, "\r")
			}
			if len(text) == 0 && err == io.EOF {
				return nil
			}
			line++
			if err := fn(text); err != nil {
				return fmt.Errorf("%s:%d: %w", name, line, err)
			}
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
	})
}
