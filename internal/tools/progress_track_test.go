package tools

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCountingReaderCountsBytes(t *testing.T) {
	src := strings.NewReader("hello, world")
	cr := NewCountingReader(src)
	n, err := io.Copy(io.Discard, cr)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if n != int64(len("hello, world")) {
		t.Fatalf("copied %d bytes, want %d", n, len("hello, world"))
	}
	if cr.Count() != n {
		t.Fatalf("Count()=%d, want %d", cr.Count(), n)
	}
}

func TestCountingReaderPartialReads(t *testing.T) {
	cr := NewCountingReader(bytes.NewReader(make([]byte, 100)))
	buf := make([]byte, 30)
	_, _ = cr.Read(buf)
	if cr.Count() != 30 {
		t.Fatalf("after one 30-byte read Count()=%d, want 30", cr.Count())
	}
	_, _ = cr.Read(buf)
	if cr.Count() != 60 {
		t.Fatalf("after two reads Count()=%d, want 60", cr.Count())
	}
}

// TestTickerEmitsAndStops verifies the Ticker emits monotonically increasing
// fractions while running and stops cleanly (no emits after stop()).
func TestTickerEmitsAndStops(t *testing.T) {
	var mu sync.Mutex
	var got []float64
	emit := func(p Progress) {
		mu.Lock()
		got = append(got, p.Fraction)
		mu.Unlock()
	}

	var cur float64
	setCur := func(v float64) { mu.Lock(); cur = v; mu.Unlock() }
	sample := func() (float64, int64, int64) {
		mu.Lock()
		defer mu.Unlock()
		return cur, 0, 100
	}

	// Ticker clamps sub-minTickInterval requests up to minTickInterval, so each
	// step must wait longer than that for a tick to land.
	stop := Ticker(context.Background(), emit, minTickInterval, sample, Progress{})
	step := minTickInterval + 60*time.Millisecond
	for _, v := range []float64{0.2, 0.5, 0.9} {
		setCur(v)
		time.Sleep(step)
	}
	stop()

	mu.Lock()
	snapshot := append([]float64(nil), got...)
	mu.Unlock()

	if len(snapshot) == 0 {
		t.Fatal("Ticker emitted nothing")
	}
	for i := 1; i < len(snapshot); i++ {
		if snapshot[i] <= snapshot[i-1] {
			t.Fatalf("fractions not strictly increasing: %v", snapshot)
		}
	}

	// After stop(), no further emits.
	countAtStop := len(snapshot)
	setCur(1.0)
	time.Sleep(minTickInterval + 60*time.Millisecond)
	mu.Lock()
	final := len(got)
	mu.Unlock()
	if final != countAtStop {
		t.Fatalf("Ticker emitted %d events after stop(); want 0", final-countAtStop)
	}
}

func TestTickerStopIsIdempotent(t *testing.T) {
	stop := Ticker(context.Background(), func(Progress) {}, 10*time.Millisecond,
		func() (float64, int64, int64) { return 0.5, 0, 1 }, Progress{})
	stop()
	stop() // must not panic or block
}
