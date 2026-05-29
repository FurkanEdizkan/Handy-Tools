package tools

import (
	"context"
	"io"
	"sync/atomic"
	"time"
)

// CountingReader wraps an io.Reader and atomically accumulates the number of
// bytes read through it. It is the shared byte-meter used to drive determinate
// progress: place one below a decompressor (to count compressed input
// consumed) or around a plain copy (to count payload bytes), then read Count()
// from a background Ticker to compute a fraction.
//
// Count() is safe to call concurrently with Read (the Ticker goroutine reads it
// while the worker goroutine reads bytes), which is why the counter is atomic.
type CountingReader struct {
	r io.Reader
	n atomic.Int64
}

// NewCountingReader returns a CountingReader over r.
func NewCountingReader(r io.Reader) *CountingReader {
	return &CountingReader{r: r}
}

// Read implements io.Reader, tallying bytes as they flow through.
func (c *CountingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		c.n.Add(int64(n))
	}
	return n, err
}

// Count returns the total number of bytes read so far.
func (c *CountingReader) Count() int64 { return c.n.Load() }

// minTickInterval bounds how often a Ticker emits, so a fast stream can't flood
// the SSE/gRPC fan-out. ~7 events/sec is smooth to the eye and cheap on the wire.
const minTickInterval = 140 * time.Millisecond

// Ticker drives time-based progress emission for a long single operation. Every
// `every` interval it calls sample(), which returns the current fraction
// (0..1) and optional bytes-done / bytes-total, and emits a Progress built from
// tmpl (so the caller's Tool/Action/CurrentItem/Level/Estimated ride along).
//
// It returns a stop func the caller must invoke (typically via defer) once the
// operation finishes; stop() halts the goroutine and waits for it to exit so no
// stray progress event races the caller's terminal Completed event. The caller
// remains responsible for emitting that terminal event.
//
// Emissions are de-duplicated: if the fraction hasn't advanced since the last
// tick, nothing is sent — a stalled stream stays quiet rather than spamming the
// same value.
func Ticker(ctx context.Context, emit func(Progress), every time.Duration, sample func() (frac float64, done, total int64), tmpl Progress) (stop func()) {
	if every < minTickInterval {
		every = minTickInterval
	}
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		t := time.NewTicker(every)
		defer t.Stop()
		var last float64 = -1
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-t.C:
				frac, bd, bt := sample()
				if frac <= last {
					continue
				}
				last = frac
				p := tmpl
				p.Fraction = frac
				p.BytesDone = bd
				p.BytesTotal = bt
				emit(p)
			}
		}
	}()
	var once int32
	return func() {
		if atomic.CompareAndSwapInt32(&once, 0, 1) {
			close(done)
			<-stopped
		}
	}
}
