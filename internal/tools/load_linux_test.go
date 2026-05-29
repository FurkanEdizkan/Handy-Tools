//go:build linux

package tools

import "testing"

func TestParseLoadAvg(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want float64
	}{
		{"typical", "0.52 0.58 0.59 1/823 12345", 0.52},
		{"zero", "0.00 0.01 0.05 1/100 42", 0.0},
		{"high", "12.34 8.00 4.00 9/999 1", 12.34},
		{"empty", "", 1},
		{"garbage", "not-a-number 0 0", 1},
		{"negative", "-1 0 0", 1},
		{"trailing newline", "1.50 0.10 0.10 1/1 1\n", 1.50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseLoadAvg(tc.in); got != tc.want {
				t.Fatalf("parseLoadAvg(%q)=%v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
