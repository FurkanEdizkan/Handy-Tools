package main

import (
	"flag"
	"fmt"
	"strings"
)

// parseFlags parses args against fs and returns the positional (non-flag)
// arguments. Unlike fs.Parse, this allows flags and positionals to be
// intermixed in any order — `htools convert in.png --format jpeg --out o.jpg`
// works the same as `htools convert --format jpeg --out o.jpg in.png`. The
// terminator `--` stops flag parsing; everything after it is treated as a
// positional.
func parseFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	var positionals []string
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		// A bare "-" or anything that doesn't start with "-" is a positional.
		if a == "-" || !strings.HasPrefix(a, "-") {
			positionals = append(positionals, a)
			i++
			continue
		}
		name := strings.TrimLeft(a, "-")
		var (
			value     string
			hasInline bool
		)
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			value = name[eq+1:]
			name = name[:eq]
			hasInline = true
		}
		fl := fs.Lookup(name)
		if fl == nil {
			return nil, fmt.Errorf("unknown flag --%s", name)
		}
		if isBoolFlag(fl) {
			if !hasInline {
				value = "true"
			}
			if err := fl.Value.Set(value); err != nil {
				return nil, fmt.Errorf("--%s: %w", name, err)
			}
			i++
			continue
		}
		if !hasInline {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("flag --%s needs a value", name)
			}
			i++
			value = args[i]
		}
		if err := fl.Value.Set(value); err != nil {
			return nil, fmt.Errorf("--%s: %w", name, err)
		}
		i++
	}
	return positionals, nil
}

// isBoolFlag reports whether f's value implements the optional IsBoolFlag()
// method (which all bools registered with flag.Bool do). Used to decide
// whether a bare `--flag` (no value) is legal.
func isBoolFlag(f *flag.Flag) bool {
	type boolFlag interface {
		IsBoolFlag() bool
	}
	if b, ok := f.Value.(boolFlag); ok && b.IsBoolFlag() {
		return true
	}
	return false
}
