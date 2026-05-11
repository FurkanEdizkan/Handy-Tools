package mascot

import (
	"strings"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/ui/theme"
)

func TestNewIsIdle(t *testing.T) {
	m := New(theme.Build(theme.Forge))
	if m.state != StateIdle {
		t.Fatalf("expected idle state, got %d", m.state)
	}
}

// TestArtIncludesWrench is the brand-defining check: every state must show a
// wrench shape — otherwise the mascot has accidentally been reverted to a
// plain animal silhouette.
func TestArtIncludesWrench(t *testing.T) {
	m := New(theme.Build(theme.Forge))
	for _, s := range []State{StateIdle, StateThinking, StateWorking, StateSuccess, StateError} {
		m.Set(s)
		got := m.art()
		if !strings.Contains(got, "[o]") {
			t.Errorf("state %d: art missing wrench marker [o]:\n%s", s, got)
		}
	}
}

func TestStatesProduceDistinctEyes(t *testing.T) {
	m := New(theme.Build(theme.Forge))
	seen := map[string]bool{}
	for _, s := range []State{StateIdle, StateThinking, StateSuccess, StateError} {
		m.Set(s)
		seen[m.art()] = true
	}
	if len(seen) < 4 {
		t.Fatalf("expected 4 distinct frames across states, got %d", len(seen))
	}
}

func TestViewRendersWithoutPanic(t *testing.T) {
	m := New(theme.Build(theme.Forge))
	m.SetWidth(36)
	if out := m.View(); out == "" {
		t.Fatal("View returned empty")
	}
}
