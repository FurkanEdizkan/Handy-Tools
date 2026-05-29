//go:build linux

package tools

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// systemLoad returns a load ratio >= 0: the 1-minute load average divided by
// the CPU count. ~0 means idle, 1.0 means fully saturated, >1 oversubscribed.
// RunEstimator stretches the modeled duration by max(1, ratio) so the bar
// climbs slower on a busy machine. Any read/parse failure yields 1.0 (neutral).
func systemLoad() float64 {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 1
	}
	avg := parseLoadAvg(string(b))
	cpus := float64(runtime.NumCPU())
	if cpus < 1 {
		cpus = 1
	}
	return avg / cpus
}

// parseLoadAvg extracts the 1-minute load average (the first field) from the
// contents of /proc/loadavg, e.g. "0.52 0.58 0.59 1/823 12345". Split out for
// testability since /proc is not writable in tests. Returns 1.0 on any
// malformed input so a parse failure can't skew the estimate.
func parseLoadAvg(contents string) float64 {
	fields := strings.Fields(contents)
	if len(fields) == 0 {
		return 1
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || v < 0 {
		return 1
	}
	return v
}
