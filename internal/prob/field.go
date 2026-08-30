package prob

import (
	"bytes"
	"fmt"
)

type FieldOptions struct {
	Delimiter string
	Field     int
}

func (opts FieldOptions) Validate() error {
	if opts.Delimiter == "" && opts.Field == 0 {
		return nil
	}
	if opts.Delimiter == "" {
		return fmt.Errorf("--delimiter is required with --field")
	}
	if opts.Field <= 0 {
		return fmt.Errorf("--field must be a positive integer and is required with --delimiter")
	}
	return nil
}

func (opts FieldOptions) Enabled() bool {
	return opts.Delimiter != "" && opts.Field > 0
}

func (opts FieldOptions) Select(record []byte) ([]byte, error) {
	if !opts.Enabled() {
		return record, nil
	}
	field := record
	delimiter := []byte(opts.Delimiter)
	for n := 1; n < opts.Field; n++ {
		_, rest, found := bytes.Cut(field, delimiter)
		if !found {
			return nil, fmt.Errorf("field %d is missing", opts.Field)
		}
		field = rest
	}
	if value, _, found := bytes.Cut(field, delimiter); found {
		field = value
	}
	return field, nil
}
