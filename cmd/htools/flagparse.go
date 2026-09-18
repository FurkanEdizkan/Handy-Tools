package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// parseFlags parses args against fs and returns the positional (non-flag)
// arguments. Unlike fs.Parse, this allows flags and positionals to be
// intermixed in any order — `htools convert in.png --format jpeg --out o.jpg`
// works the same as `htools convert --format jpeg --out o.jpg in.png`. The
// terminator `--` stops flag parsing; everything after it is treated as a
// positional. `-h` / `--help` (unless the verb defines a flag by that name)
// returns flag.ErrHelp; hand the error to flagErr.
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
			if name == "h" || name == "help" {
				return nil, flag.ErrHelp
			}
			return nil, fmt.Errorf("unknown flag --%s", name)
		}
		if isBoolFlag(fl) {
			if !hasInline {
				value = "true"
			}
			if err := fs.Set(name, value); err != nil {
				return nil, fmt.Errorf("--%s: %w", name, err)
			}
			i++
			continue
		}
		if !hasInline {
			if i+1 < len(args) {
				value = args[i+1]
				i++
			} else {
				return nil, fmt.Errorf("flag --%s needs a value", name)
			}
		}
		// fs.Set (not fl.Value.Set) so the flag is recorded as explicitly
		// given — flagWasSet relies on that to tell `--replace ''` from an
		// omitted flag.
		if err := fs.Set(name, value); err != nil {
			return nil, fmt.Errorf("--%s: %w", name, err)
		}
		i++
	}
	return positionals, nil
}

// flagWasSet reports whether the named flag appeared on the command line,
// even with an empty value.
func flagWasSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

// flagErr maps a parseFlags error to the verb's exit code. A help request
// prints the verb's flags to stdout and exits 0; anything else is a usage
// error (exit 1).
func flagErr(fs *flag.FlagSet, err error) int {
	if errors.Is(err, flag.ErrHelp) {
		printFlagHelp(os.Stdout, fs)
		return 0
	}
	return usageErr(os.Stderr, fs.Name(), err.Error())
}

// printFlagHelp lists fs's flags in the same `--name` spelling the top-level
// help uses (flag.PrintDefaults would print single-dash names).
func printFlagHelp(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprintf(w, "Usage: htools %s [flags] [args...]\n\nFlags:\n", fs.Name())
	fs.VisitAll(func(f *flag.Flag) {
		kind, usage := flag.UnquoteUsage(f)
		head := "  --" + f.Name
		if kind != "" && !isBoolFlag(f) {
			head += " " + kind
		}
		fmt.Fprintf(w, "%s\n      %s", head, usage)
		if f.DefValue != "" && f.DefValue != "0" && f.DefValue != "false" {
			fmt.Fprintf(w, " (default %s)", f.DefValue)
		}
		fmt.Fprintln(w)
	})
	fmt.Fprintln(w, "\nRun `htools help` for every command.")
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
