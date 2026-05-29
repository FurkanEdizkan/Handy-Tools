package tools

import (
	"context"
	"math"
	"time"
)

// Operations that can't report real progress (we shell out to a binary, or
// encode a single image in one opaque call) still want a bar that moves. For
// those we model an expected duration from the input size and a rough
// throughput prior, then ease a fraction toward a cap until the real
// completion event arrives. The prior only needs to be order-of-magnitude
// right: the ease-out curve below is forgiving, and a wrong guess just makes
// the bar reach the cap sooner or later, never wrong-then-backward.

// estimateCap is the ceiling an estimate may approach. Estimates never claim
// 100% — only the real terminal event does — so the bar can't sit at "done"
// while work is still running.
const estimateCap = 0.95

// throughputPrior returns a rough bytes/second estimate for an opaque op,
// keyed by "<tool>/<action>". Unknown keys fall back to a conservative
// general-purpose rate. These are deliberately approximate (see file comment).
func throughputPrior(tool, action string) float64 {
	switch tool + "/" + action {
	case "archive/extract-unrar", "archive/extract-7z":
		return 60 << 20 // ~60 MB/s decode through unrar/7z
	case "archive/compress-7z":
		return 25 << 20 // compression is slower than extraction
	case "pdf/render":
		return 4 << 20 // pdftoppm is rasterisation-bound
	case "pdf/text":
		return 20 << 20
	case "pdf/merge", "pdf/split":
		return 80 << 20 // mostly I/O over the page tree
	case "image/convert":
		return 12 << 20 // decode + re-encode of one image
	default:
		return 30 << 20
	}
}

// EaseFraction maps elapsed/expected onto [0, estimateCap] with an exponential
// ease-out: fast at first, asymptotically approaching the cap (and reaching it
// once the exponential underflows, far past the expected duration). It is
// monotonic non-decreasing in elapsed and always stays at or below the cap, so
// the bar moves forward and never claims "done" (1.0) from an estimate alone.
func EaseFraction(elapsed, expected time.Duration) float64 {
	if expected <= 0 || elapsed <= 0 {
		return 0
	}
	frac := 1 - math.Exp(-float64(elapsed)/float64(expected))
	return frac * estimateCap
}

// Estimator computes an eased ETA fraction for one opaque operation. Now and
// Load are injectable for tests; zero values fall back to the real clock and
// the platform system-load reader.
type Estimator struct {
	Tool      string
	Action    string
	InputSize int64 // best-effort size of the work (bytes); 0 -> use a default
	Start     time.Time
	Now       func() time.Time // defaults to time.Now
	Load      func() float64   // defaults to systemLoad(); >=1 stretches the ETA
}

// expected returns the modeled duration for the whole operation: how long
// InputSize bytes take at the throughput prior, stretched by current system
// load (a busy machine runs slower, so the bar should climb slower).
func (e Estimator) expected() time.Duration {
	size := e.InputSize
	if size <= 0 {
		size = 8 << 20 // assume a modest job when size is unknown
	}
	rate := throughputPrior(e.Tool, e.Action)
	secs := float64(size) / rate
	var load float64
	if e.Load != nil {
		load = e.Load()
	} else {
		load = systemLoad()
	}
	if load < 1 {
		load = 1
	}
	secs *= load
	if secs < 0.25 {
		secs = 0.25 // floor so tiny jobs still animate rather than snapping
	}
	return time.Duration(secs * float64(time.Second))
}

// Fraction returns the current eased estimate in [0, estimateCap).
func (e Estimator) Fraction() float64 {
	now := time.Now
	if e.Now != nil {
		now = e.Now
	}
	return EaseFraction(now().Sub(e.Start), e.expected())
}

// RunEstimator starts a background Ticker that emits eased ETA progress
// (Estimated: true) for an opaque operation until the returned stop func is
// called. The caller defers stop() and then emits the real terminal event
// (Completed, Fraction: 1). tmpl supplies CurrentItem / Message / Level; Tool,
// Action and Estimated are filled in here.
func RunEstimator(ctx context.Context, emit func(Progress), est Estimator, tmpl Progress) (stop func()) {
	tmpl.Tool = est.Tool
	tmpl.Action = est.Action
	tmpl.Estimated = true
	return Ticker(ctx, emit, minTickInterval, func() (float64, int64, int64) {
		return est.Fraction(), 0, est.InputSize
	}, tmpl)
}
