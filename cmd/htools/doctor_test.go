package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
)

func TestMarker(t *testing.T) {
	if got := marker(true); got != "+" {
		t.Errorf("marker(true) = %q, want +", got)
	}
	if got := marker(false); got != "-" {
		t.Errorf("marker(false) = %q, want -", got)
	}
}

func TestWriteDoctorIncludesEveryKnownTool(t *testing.T) {
	var buf bytes.Buffer
	writeDoctor(&buf)
	out := buf.String()
	if !strings.Contains(out, "Handy Tools environment check") {
		t.Errorf("missing header in:\n%s", out)
	}
	for _, tool := range sysdep.Known {
		if !strings.Contains(out, tool.Name) {
			t.Errorf("doctor output missing tool %q", tool.Name)
		}
	}
}
