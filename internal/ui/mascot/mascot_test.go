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

// TestArtIncludesFace is the brand-defining check: every state must render
// the Wrenly red-panda face (ears + chin). The design moved away from the
// wrench-bearing silhouette to a face-only character on 2026-05-14; if these
// markers disappear the mascot has been reverted to something else.
func TestArtIncludesFace(t *testing.T) {
	m := New(theme.Build(theme.Forge))
	for _, s := range []State{StateIdle, StateThinking, StateWorking, StateSuccess, StateError} {
		m.Set(s)
		got := m.art()
		if !strings.Contains(got, "/\\___/\\") {
			t.Errorf("state %d: art missing ear marker /\\___/\\:\n%s", s, got)
		}
		if !strings.Contains(got, "`---`") {
			t.Errorf("state %d: art missing chin marker `---`:\n%s", s, got)
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

func TestStatusLabelMatchesState(t *testing.T) {
	m := New(theme.Build(theme.Forge))
	cases := []struct {
		s    State
		want string
	}{
		{StateIdle, "IDLE"},
		{StateThinking, "THINKING"},
		{StateWorking, "WORKING"},
		{StateSuccess, "DONE"},
		{StateError, "ERROR"},
	}
	for _, c := range cases {
		m.Set(c.s)
		if got := m.StatusLabel(); got != c.want {
			t.Errorf("state %d: got label %q want %q", c.s, got, c.want)
		}
	}
}

func TestViewRendersWithoutPanic(t *testing.T) {
	m := New(theme.Build(theme.Forge))
	m.SetWidth(36)
	if out := m.View(); out == "" {
		t.Fatal("View returned empty")
	}
}
