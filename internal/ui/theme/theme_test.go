package theme

import "testing"

func TestResolveKnownPalettes(t *testing.T) {
	cases := map[string]string{
		"forge": "forge",
		"snow":  "snow",
		"ember": "ember",
	}
	for in, want := range cases {
		if p := Resolve(in); p.Name != want {
			t.Errorf("Resolve(%q).Name = %q want %q", in, p.Name, want)
		}
	}
}

func TestResolveDefaultsToForge(t *testing.T) {
	for _, in := range []string{"", "unknown", "rainbow"} {
		if p := Resolve(in); p.Name != "forge" {
			t.Errorf("Resolve(%q) = %q want forge", in, p.Name)
		}
	}
}

func TestBuildPopulatesAllStyles(t *testing.T) {
	s := Build(Resolve("forge"))
	if s.P.Name != "forge" {
		t.Fatalf("Build dropped palette: %q", s.P.Name)
	}
	// A smoke check: Render must not return empty for a non-empty input.
	if got := s.Title.Render("hi"); got == "" {
		t.Fatal("Title.Render returned empty for non-empty input")
	}
	if got := s.MascotFur.Render("x"); got == "" {
		t.Fatal("MascotFur.Render returned empty for non-empty input")
	}
}
