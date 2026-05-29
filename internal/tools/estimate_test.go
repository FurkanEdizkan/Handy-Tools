package tools

import (
	"testing"
	"time"
)

func TestEaseFractionBoundsAndMonotonic(t *testing.T) {
	expected := 10 * time.Second
	if got := EaseFraction(0, expected); got != 0 {
		t.Fatalf("EaseFraction(0)=%v, want 0", got)
	}
	if got := EaseFraction(-1, expected); got != 0 {
		t.Fatalf("EaseFraction(negative)=%v, want 0", got)
	}
	// Strictly increasing while meaningfully below the cap (up to ~6x the
	// expected duration); always strictly below the cap everywhere.
	var prev float64
	for s := 1; s <= 60; s++ {
		f := EaseFraction(time.Duration(s)*time.Second, expected)
		if f <= prev {
			t.Fatalf("not increasing at %ds: %v <= %v", s, f, prev)
		}
		prev = f
	}
	for s := 1; s <= 600; s++ {
		f := EaseFraction(time.Duration(s)*time.Second, expected)
		if f > estimateCap {
			t.Fatalf("exceeded cap at %ds: %v > %v", s, f, estimateCap)
		}
		if f >= 1 {
			t.Fatalf("estimate reached 1.0 at %ds: %v — must never claim done", s, f)
		}
	}
	// Far past the expected duration it should be close to (but under) the cap.
	if f := EaseFraction(100*expected, expected); f < estimateCap*0.99 {
		t.Fatalf("EaseFraction at 100x expected=%v, want near cap %v", f, estimateCap)
	}
}

func TestEaseFractionZeroExpected(t *testing.T) {
	if got := EaseFraction(time.Second, 0); got != 0 {
		t.Fatalf("EaseFraction with zero expected=%v, want 0", got)
	}
}

func TestEstimatorFractionAdvancesWithClock(t *testing.T) {
	now := time.Unix(1000, 0)
	est := Estimator{
		Tool:      "archive",
		Action:    "extract-7z",
		InputSize: 100 << 20,
		Start:     now,
		Now:       func() time.Time { return now },
		Load:      func() float64 { return 1 },
	}
	if f := est.Fraction(); f != 0 {
		t.Fatalf("at t=start fraction=%v, want 0", f)
	}
	now = now.Add(2 * time.Second)
	f2 := est.Fraction()
	now = now.Add(2 * time.Second)
	f4 := est.Fraction()
	if !(f2 > 0 && f4 > f2 && f4 < 1) {
		t.Fatalf("fraction should rise toward but under 1: f2=%v f4=%v", f2, f4)
	}
}

func TestEstimatorLoadStretchesETA(t *testing.T) {
	mk := func(load float64) Estimator {
		start := time.Unix(0, 0)
		return Estimator{
			Tool: "archive", Action: "extract-7z", InputSize: 100 << 20,
			Start: start,
			Now:   func() time.Time { return start.Add(3 * time.Second) },
			Load:  func() float64 { return load },
		}
	}
	idle := mk(1).Fraction()
	busy := mk(4).Fraction()
	// Under heavier load the modeled duration stretches, so at the same elapsed
	// time the estimated fraction is lower.
	if !(busy < idle) {
		t.Fatalf("busy fraction %v should be < idle fraction %v", busy, idle)
	}
}
