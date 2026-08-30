package prob

import "flag"

func AddInputFlags(fs *flag.FlagSet, opts *InputOptions) {
	fs.BoolVar(&opts.NUL, "0", false, "read NUL-delimited input")
	fs.BoolVar(&opts.NUL, "nul", false, "read NUL-delimited input")
	fs.BoolVar(&opts.Trim, "trim", false, "trim surrounding whitespace")
	fs.BoolVar(&opts.IgnoreEmpty, "ignore-empty", false, "ignore empty input items")
}

func AddFieldFlags(fs *flag.FlagSet, opts *FieldOptions) {
	fs.StringVar(&opts.Delimiter, "d", "", "literal field delimiter")
	fs.StringVar(&opts.Delimiter, "delimiter", "", "literal field delimiter")
	fs.IntVar(&opts.Field, "f", 0, "1-based field used as the value")
	fs.IntVar(&opts.Field, "field", 0, "1-based field used as the value")
}
