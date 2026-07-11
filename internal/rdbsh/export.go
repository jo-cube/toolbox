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

	if filePath == "-" {
		count, err := s.export(s.out, format, prefix)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(s.errOut, "exported %d entries to stdout (%s)\n", count, format)
		return err
	}

	file, err := openExportFile(filePath, s.config.Force)
	if err != nil {
		return err
	}
	count, exportErr := s.export(file, format, prefix)
	closeErr := file.Close()
	if exportErr != nil {
		return exportErr
	}
	if closeErr != nil {
		return fmt.Errorf("close export file: %w", closeErr)
	}
	_, err = fmt.Fprintf(s.out, "exported %d entries to %s (%s)\n", count, filePath, format)
	return err
}

func openExportFile(filePath string, force bool) (*os.File, error) {
	flag := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if force {
		flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	return os.OpenFile(filePath, flag, 0o644)
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
		return csvWriter.Write([]string{formatExportBytes(key), formatExportBytes(value)})
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
			Key:   formatExportBytes(key),
			Value: formatExportBytes(value),
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
