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
	if m.character != CharacterWrenly {
		t.Fatalf("expected default character wrenly, got %q", m.character)
	}
}

// TestArtRendersSprite asserts every state renders the dot-grid sprite (at
// least one fur ● glyph) — i.e. we never fall back to a blank or to the
// retired placeholder frame.
func TestArtRendersSprite(t *testing.T) {
	m := New(theme.Build(theme.Forge))
	states := []State{
		StateIdle, StateThinking, StateWatching, StateStressed,
		StateTired, StateHappy, StateWorried,
	}
	for _, s := range states {
		m.Set(s)
		got := m.art()
		if !strings.Contains(got, "●") {
			t.Errorf("state %d: art missing fur glyph ●:\n%s", s, got)
		}
	}
}

func TestStatesProduceDistinctFrames(t *testing.T) {
	m := New(theme.Build(theme.Forge))
	seen := map[string]bool{}
	for _, s := range []State{
		StateIdle, StateThinking, StateWatching, StateStressed,
		StateTired, StateHappy, StateWorried,
	} {
		m.Set(s)
		seen[m.art()] = true
	}
	if len(seen) < 5 {
		t.Fatalf("expected at least 5 distinct frames across states, got %d", len(seen))
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
		{StateWatching, "WATCHING"},
		{StateStressed, "STRESSED"},
		{StateTired, "FRUSTRATED"},
		{StateHappy, "HAPPY"},
		{StateWorried, "WORRIED"},
	}
	for _, c := range cases {
		m.Set(c.s)
		if got := m.StatusLabel(); got != c.want {
			t.Errorf("state %d: got label %q want %q", c.s, got, c.want)
		}
	}
}

// TestBackCompatStateAliases ensures the old StateWorking/Success/Error
// identifiers still resolve to the new vocabulary so external callers don't
// break.
func TestBackCompatStateAliases(t *testing.T) {
	if StateWorking != StateWatching {
		t.Errorf("StateWorking should alias StateWatching")
	}
	if StateSuccess != StateHappy {
		t.Errorf("StateSuccess should alias StateHappy")
	}
	if StateError != StateWorried {
		t.Errorf("StateError should alias StateWorried")
	}
}

// TestSpriteRendersDesignGlyphs locks the plain-text Sprite() output that
// brand assets (docs/brand/wrenly.txt, README hero, install.sh banner) are
// generated from. It guards against accidental glyph drift: every character
// must render ● fur, ○ cream, and ▪ mouth, and the eye row gets the simple
// • placeholder (the TUI overlays a richer eye via lipgloss).
func TestSpriteRendersDesignGlyphs(t *testing.T) {
	for _, c := range []string{CharacterWrenly, CharacterHopper} {
		out := Sprite(c)
		for _, glyph := range []string{"●", "○", "▪"} {
			if !strings.Contains(out, glyph) {
				t.Errorf("character %q sprite missing glyph %q:\n%s", c, glyph, out)
			}
		}
	}
	// Only Wrenly has the cheek-stripe + eye row that emits •; Hopper's eye
	// row uses a different sprite pattern.
	if !strings.Contains(Sprite(CharacterWrenly), "•") {
		t.Errorf("Wrenly sprite should render • eye placeholder")
	}
	// Unknown character falls back to Wrenly.
	if Sprite("garbage") != Sprite(CharacterWrenly) {
		t.Errorf("unknown character should fall back to Wrenly")
	}
}

// TestSpritePlainNoANSI confirms the brand-asset path never leaks lipgloss
// escape sequences. Surfaces that embed Sprite() output literally (the
// installer banner wraps it with its own shell-color helpers) would render
// raw ESC bytes if this regressed.
func TestSpritePlainNoANSI(t *testing.T) {
	if strings.Contains(Sprite(CharacterWrenly), "\x1b") {
		t.Fatal("Sprite() emitted ANSI escape sequences; should be plain glyphs only")
	}
}

func TestSetCharacterAndView(t *testing.T) {
	m := New(theme.Build(theme.Forge))
	m.SetWidth(36)
	for _, c := range []string{CharacterWrenly, CharacterHopper, "unknown"} {
		m.SetCharacter(c)
		if out := m.View(); out == "" {
			t.Fatalf("View returned empty for character %q", c)
		}
	}
	// Unknown character must fall back to wrenly.
	m.SetCharacter("garbage")
	if m.Character() != CharacterWrenly {
		t.Fatalf("expected fallback to wrenly, got %q", m.Character())
	}
}

func TestViewRendersWithoutPanic(t *testing.T) {
	m := New(theme.Build(theme.Forge))
	m.SetWidth(36)
	if out := m.View(); out == "" {
		t.Fatal("View returned empty")
	}
}
