package prob

import "flag"

func AddInputFlags(fs *flag.FlagSet, opts *InputOptions) {
	fs.BoolVar(&opts.NUL, "0", false, "read NUL-delimited input")
	fs.BoolVar(&opts.NUL, "nul", false, "read NUL-delimited input")
	fs.BoolVar(&opts.Trim, "trim", false, "trim surrounding whitespace")
	fs.BoolVar(&opts.IgnoreEmpty, "ignore-empty", false, "ignore empty input items")
}
