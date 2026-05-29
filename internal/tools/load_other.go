//go:build !linux

package tools

// systemLoad is neutral on non-Linux platforms: reading a normalized load
// average portably (darwin/windows) needs cgo or extra dependencies the
// project avoids, so the ETA estimator just runs without a load adjustment.
func systemLoad() float64 { return 1 }
