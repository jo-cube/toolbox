package rdbsh

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

func (s *Shell) cmdExport(args []string) error {
	if len(args) < 1 || len(args) > 3 {
		return fmt.Errorf("usage: export <file|-> [csv|json] [prefix]")
	}

	filePath := args[0]
	format := "csv"
	if len(args) >= 2 {
		format = strings.ToLower(args[1])
	}
	if format != "csv" && format != "json" {
		return fmt.Errorf("format must be 'csv' or 'json'")
	}

	var prefix []byte
	if len(args) == 3 {
		decoded, err := parseInput(args[2])
		if err != nil {
			return err
		}
		prefix = decoded
	}

	writer := io.Writer(s.out)
	closer := io.NopCloser(strings.NewReader(""))
	closeNeeded := false
	if filePath != "-" {
		file, err := os.Create(filePath)
		if err != nil {
			return err
		}
		writer = file
		closer = file
		closeNeeded = true
	}
	if closeNeeded {
		defer closer.Close()
	}

	count, err := s.export(writer, format, prefix)
	if err != nil {
		return err
	}

	if filePath == "-" {
		fmt.Fprintf(s.errOut, "exported %d entries to stdout (%s)\n", count, format)
		return nil
	}
	fmt.Fprintf(s.out, "exported %d entries to %s (%s)\n", count, filePath, format)
	return nil
}

func (s *Shell) export(writer io.Writer, format string, prefix []byte) (int, error) {
	switch format {
	case "csv":
		return s.exportCSV(writer, prefix)
	case "json":
		return s.exportJSON(writer, prefix)
	default:
		return 0, fmt.Errorf("unsupported export format %q", format)
	}
}

func (s *Shell) exportCSV(writer io.Writer, prefix []byte) (int, error) {
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write([]string{"key", "value"}); err != nil {
		return 0, err
	}

	result, err := s.iterate(prefix, 0, func(key, value []byte) error {
		return csvWriter.Write([]string{formatBytes(key), formatBytes(value)})
	})
	if err != nil {
		return 0, err
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return 0, err
	}
	return result.Count, nil
}

func (s *Shell) exportJSON(writer io.Writer, prefix []byte) (int, error) {
	if _, err := fmt.Fprintln(writer, "["); err != nil {
		return 0, err
	}

	first := true
	result, err := s.iterate(prefix, 0, func(key, value []byte) error {
		entry, err := json.Marshal(struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}{
			Key:   formatBytes(key),
			Value: formatBytes(value),
		})
		if err != nil {
			return err
		}
		if !first {
			if _, err := fmt.Fprintln(writer, ","); err != nil {
				return err
			}
		}
		first = false
		if _, err := writer.Write(entry); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(writer); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if _, err := fmt.Fprintln(writer, "]"); err != nil {
		return 0, err
	}
	return result.Count, nil
}
